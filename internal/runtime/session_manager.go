package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"zheng-harness/internal/domain"
)

const (
	defaultActiveSessionCap = 8
	defaultShutdownTimeout  = 30 * time.Second
	defaultEventBufferSize  = 64
	defaultSubscriberBuffer = 256
)

var (
	ErrSessionAlreadyActive = errors.New("runtime session already active")
	ErrActiveSessionLimit   = errors.New("runtime active session limit exceeded")
	ErrSessionManagerClosed = errors.New("runtime session manager is shutting down")
)

type ActorState string

const (
	ActorStatePending   ActorState = "pending"
	ActorStateRunning   ActorState = "running"
	ActorStateCompleted ActorState = "completed"
	ActorStateFailed    ActorState = "failed"
	ActorStateCancelled ActorState = "cancelled"
)

type SessionRunner interface {
	Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error)
}

type SessionRunnerFactory func(events *EventChannel) (SessionRunner, error)

type SessionActorFinalizer func(ctx context.Context, result SessionActorResult) error

type SessionStartRequest struct {
	SessionID   string
	Task        domain.Task
	NewRunner   SessionRunnerFactory
	PersistFinal SessionActorFinalizer
}

type SessionManagerOptions struct {
	ActiveSessionCap int
	ShutdownTimeout  time.Duration
	EventBufferSize  int
	Clock            func() time.Time
}

type SessionManager struct {
	mu              sync.RWMutex
	actors          map[string]*SessionActor
	activeCount     int
	accepting       bool
	activeSessionCap int
	shutdownTimeout time.Duration
	eventBufferSize int
	clock           func() time.Time
	wg              sync.WaitGroup
	shutdownOnce    sync.Once
}

type SessionActor struct {
	sessionID string
	task      domain.Task
	events    *EventChannel
	done      chan struct{}
	cancel    context.CancelFunc
	relay     *sessionEventRelay

	mu          sync.RWMutex
	state       ActorState
	createdAt   time.Time
	startedAt   time.Time
	finishedAt  time.Time
	err         error
	result      SessionActorResult
	cancelled   bool
	cancelCause error
	finalized   bool
}

type SessionActorSnapshot struct {
	SessionID   string
	State       ActorState
	CreatedAt   time.Time
	StartedAt   time.Time
	FinishedAt  time.Time
	Err         error
	Cancelled   bool
	CancelCause error
}

type EventSubscription struct {
	ch chan domain.StreamingEvent

	mu         sync.RWMutex
	closed     bool
	overflowed bool
	errorEvent *domain.StreamingEvent
	closeOnce  sync.Once
	onClose    func()
}

type sessionEventRelay struct {
	input       <-chan domain.StreamingEvent
	bufferSize  int
	subscribers map[*EventSubscription]struct{}
	mu          sync.Mutex
	closed      bool
}

type SessionActorResult struct {
	SessionID string
	Task      domain.Task
	State     ActorState
	Session   domain.Session
	Plan      domain.Plan
	Steps     []domain.Step
	Err       error

	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	Cancelled  bool
}

func NewSessionManager(opts SessionManagerOptions) *SessionManager {
	cap := opts.ActiveSessionCap
	if cap <= 0 {
		cap = defaultActiveSessionCap
	}
	shutdownTimeout := opts.ShutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = defaultShutdownTimeout
	}
	eventBufferSize := opts.EventBufferSize
	if eventBufferSize <= 0 {
		eventBufferSize = defaultEventBufferSize
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	return &SessionManager{
		actors:           make(map[string]*SessionActor),
		accepting:        true,
		activeSessionCap: cap,
		shutdownTimeout:  shutdownTimeout,
		eventBufferSize:  eventBufferSize,
		clock:            clock,
	}
}

func (m *SessionManager) Start(ctx context.Context, req SessionStartRequest) (*SessionActor, error) {
	if m == nil {
		return nil, errors.New("runtime session manager is nil")
	}
	if err := validateSessionStartRequest(req); err != nil {
		return nil, err
	}

	now := m.clock()
	actor := &SessionActor{
		sessionID: req.SessionID,
		task:      req.Task,
		events:    NewEventChannel(m.eventBufferSize),
		done:      make(chan struct{}),
		state:     ActorStatePending,
		createdAt: now,
	}
	actor.relay = newSessionEventRelay(actor.events.Events(), actor.done, defaultSubscriberBuffer)

	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.accepting {
		return nil, ErrSessionManagerClosed
	}
	if _, exists := m.actors[req.SessionID]; exists {
		return nil, ErrSessionAlreadyActive
	}
	if m.activeCount >= m.activeSessionCap {
		return nil, ErrActiveSessionLimit
	}
	m.actors[req.SessionID] = actor
	m.activeCount++
	m.wg.Add(1)
	go m.runActor(ctx, actor, req)
	go actor.relay.run()
	return actor, nil
}

func (m *SessionManager) ActiveSessionCap() int {
	if m == nil {
		return 0
	}
	return m.activeSessionCap
}

func (m *SessionManager) ActiveCount() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeCount
}

func (m *SessionManager) IsAccepting() bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accepting
}

func (m *SessionManager) Lookup(sessionID string) (*SessionActor, bool) {
	if m == nil {
		return nil, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	actor, ok := m.actors[sessionID]
	return actor, ok
}

func (m *SessionManager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.shutdownOnce.Do(func() {
		m.mu.Lock()
		m.accepting = false
		m.mu.Unlock()
	})

	if ctx == nil {
		ctx = context.Background()
	}

	waitDone := make(chan struct{})
	go func() {
		defer close(waitDone)
		m.wg.Wait()
	}()

	timeout := time.NewTimer(m.shutdownTimeout)
	defer timeout.Stop()

	select {
	case <-waitDone:
		return nil
	case <-timeout.C:
		m.cancelAllActive(context.DeadlineExceeded)
	case <-ctx.Done():
		m.cancelAllActive(ctx.Err())
		<-waitDone
		return ctx.Err()
	}

	select {
	case <-waitDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *SessionManager) cancelAllActive(cause error) {
	m.mu.RLock()
	actors := make([]*SessionActor, 0, len(m.actors))
	for _, actor := range m.actors {
		actors = append(actors, actor)
	}
	m.mu.RUnlock()
	for _, actor := range actors {
		actor.markCancelled(cause)
	}
}

func (m *SessionManager) runActor(ctx context.Context, actor *SessionActor, req SessionStartRequest) {
	defer m.wg.Done()
	defer close(actor.done)
	defer actor.events.Close()

	runCtx := ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	runCtx, cancel := context.WithCancel(runCtx)
	actor.attachCancel(cancel)
	defer cancel()

	actor.transitionToRunning(m.clock())
	runner, buildErr := req.NewRunner(actor.events)
	if buildErr != nil {
		result := actor.finish(m.clock(), domain.Session{}, domain.Plan{}, nil, buildErr)
		m.completeActor(req, result)
		return
	}
	if runner == nil {
		result := actor.finish(m.clock(), domain.Session{}, domain.Plan{}, nil, errors.New("runtime session runner factory returned nil runner"))
		m.completeActor(req, result)
		return
	}

	session, plan, steps, runErr := runner.Run(runCtx, req.Task)
	result := actor.finish(m.clock(), session, plan, steps, runErr)
	m.completeActor(req, result)
}

func (m *SessionManager) completeActor(req SessionStartRequest, result SessionActorResult) {
	m.mu.Lock()
	delete(m.actors, req.SessionID)
	if m.activeCount > 0 {
		m.activeCount--
	}
	m.mu.Unlock()

	if req.PersistFinal != nil {
		_ = req.PersistFinal(context.Background(), result)
	}
}

func (a *SessionActor) Done() <-chan struct{} {
	if a == nil {
		return nil
	}
	return a.done
}

func (a *SessionActor) Events() <-chan domain.StreamingEvent {
	if a == nil || a.events == nil {
		return nil
	}
	return a.events.Events()
}

func (a *SessionActor) EventChannel() *EventChannel {
	if a == nil {
		return nil
	}
	return a.events
}

func (a *SessionActor) Subscribe(buffer int) *EventSubscription {
	if a == nil || a.relay == nil {
		return nil
	}
	a.mu.RLock()
	finalized := a.finalized
	a.mu.RUnlock()
	if finalized {
		return newClosedEventSubscription(buffer)
	}
	return a.relay.subscribe(buffer)
}

func (a *SessionActor) Snapshot() SessionActorSnapshot {
	if a == nil {
		return SessionActorSnapshot{}
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return SessionActorSnapshot{
		SessionID:   a.sessionID,
		State:       a.state,
		CreatedAt:   a.createdAt,
		StartedAt:   a.startedAt,
		FinishedAt:  a.finishedAt,
		Err:         a.err,
		Cancelled:   a.cancelled,
		CancelCause: a.cancelCause,
	}
}

func (a *SessionActor) Result() (SessionActorResult, bool) {
	if a == nil {
		return SessionActorResult{}, false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	if !a.finalized {
		return SessionActorResult{}, false
	}
	return a.result, true
}

func (a *SessionActor) Cancel(cause error) {
	if a == nil {
		return
	}
	a.markCancelled(cause)
}

func (a *SessionActor) attachCancel(cancel context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancel = cancel
}

func (a *SessionActor) transitionToRunning(startedAt time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = ActorStateRunning
	a.startedAt = startedAt
}

func (a *SessionActor) markCancelled(cause error) {
	a.mu.Lock()
	a.cancelled = true
	a.cancelCause = cause
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *SessionActor) finish(finishedAt time.Time, session domain.Session, plan domain.Plan, steps []domain.Step, runErr error) SessionActorResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	state := classifyActorState(session, runErr, a.cancelled)
	result := SessionActorResult{
		SessionID:   a.sessionID,
		Task:        a.task,
		State:       state,
		Session:     session,
		Plan:        plan,
		Steps:       steps,
		Err:         runErr,
		CreatedAt:   a.createdAt,
		StartedAt:   a.startedAt,
		FinishedAt:  finishedAt,
		Cancelled:   a.cancelled,
	}

	a.state = state
	a.finishedAt = finishedAt
	a.err = runErr
	a.result = result
	a.finalized = true
	return result
}

func validateSessionStartRequest(req SessionStartRequest) error {
	if req.NewRunner == nil {
		return errors.New("runtime session start requires runner factory")
	}
	if req.Task.ID == "" {
		return errors.New("runtime session start requires task id")
	}
	if req.SessionID == "" {
		return errors.New("runtime session start requires session id")
	}
	if req.Task.ID != req.SessionID {
		return fmt.Errorf("runtime session start requires task id %q to match session id", req.SessionID)
	}
	return nil
}

func classifyActorState(session domain.Session, runErr error, cancelled bool) ActorState {
	if cancelled || errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded) || session.Status == domain.SessionStatusInterrupted {
		return ActorStateCancelled
	}
	if runErr != nil {
		return ActorStateFailed
	}
	switch session.Status {
	case domain.SessionStatusFatalError, domain.SessionStatusVerificationFailed, domain.SessionStatusBudgetExceeded:
		return ActorStateFailed
	case domain.SessionStatusInterrupted:
		return ActorStateCancelled
	default:
		return ActorStateCompleted
	}
}

func newSessionEventRelay(input <-chan domain.StreamingEvent, _ <-chan struct{}, bufferSize int) *sessionEventRelay {
	if bufferSize <= 0 {
		bufferSize = defaultSubscriberBuffer
	}
	return &sessionEventRelay{
		input:       input,
		bufferSize:  bufferSize,
		subscribers: make(map[*EventSubscription]struct{}),
	}
}

func (r *sessionEventRelay) run() {
	if r == nil {
		return
	}
	defer r.closeAll()
	for event := range r.input {
		r.broadcast(event)
	}
}

func (r *sessionEventRelay) subscribe(buffer int) *EventSubscription {
	if r == nil {
		return nil
	}
	if buffer <= 0 {
		buffer = r.bufferSize
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return newClosedEventSubscription(buffer)
	}
	sub := &EventSubscription{ch: make(chan domain.StreamingEvent, buffer)}
	sub.onClose = func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.subscribers, sub)
	}
	r.subscribers[sub] = struct{}{}
	return sub
}

func (r *sessionEventRelay) broadcast(event domain.StreamingEvent) {
	r.mu.Lock()
	subs := make([]*EventSubscription, 0, len(r.subscribers))
	for sub := range r.subscribers {
		subs = append(subs, sub)
	}
	r.mu.Unlock()
	for _, sub := range subs {
		if !sub.trySend(event) {
			sub.markOverflow(overflowSignalForEvent(event))
		}
	}
}

func (r *sessionEventRelay) closeAll() {
	r.mu.Lock()
	r.closed = true
	subs := make([]*EventSubscription, 0, len(r.subscribers))
	for sub := range r.subscribers {
		subs = append(subs, sub)
	}
	r.subscribers = make(map[*EventSubscription]struct{})
	r.mu.Unlock()
	for _, sub := range subs {
		sub.Close()
	}
}

func newClosedEventSubscription(buffer int) *EventSubscription {
	if buffer <= 0 {
		buffer = defaultSubscriberBuffer
	}
	sub := &EventSubscription{ch: make(chan domain.StreamingEvent, buffer)}
	sub.Close()
	return sub
}

func (s *EventSubscription) Events() <-chan domain.StreamingEvent {
	if s == nil {
		return nil
	}
	return s.ch
}

func (s *EventSubscription) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		close(s.ch)
		s.mu.Unlock()
		if s.onClose != nil {
			s.onClose()
		}
	})
}

func (s *EventSubscription) OverflowEvent() (*domain.StreamingEvent, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.overflowed || s.errorEvent == nil {
		return nil, false
	}
	copyEvent := *s.errorEvent
	return &copyEvent, true
}

func (s *EventSubscription) trySend(event domain.StreamingEvent) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return false
	}
	select {
	case s.ch <- event:
		return true
	default:
		return false
	}
}

func (s *EventSubscription) markOverflow(event *domain.StreamingEvent) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.overflowed {
		s.mu.Unlock()
		return
	}
	s.overflowed = true
	s.errorEvent = event
	if event != nil {
		select {
		case <-s.ch:
		default:
		}
		select {
		case s.ch <- *event:
		default:
		}
	}
	s.mu.Unlock()
	s.Close()
}

func newSubscriberOverflowEvent() *domain.StreamingEvent {
	event, err := domain.Error(0, "stream subscriber fell behind and was disconnected")
	if err != nil {
		return &domain.StreamingEvent{Type: domain.EventError, Timestamp: time.Now().UTC()}
	}
	return event
}

func overflowSignalForEvent(event domain.StreamingEvent) *domain.StreamingEvent {
	if event.Type == domain.EventSessionComplete {
		copyEvent := event
		return &copyEvent
	}
	return newSubscriberOverflowEvent()
}
