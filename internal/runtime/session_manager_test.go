package runtime_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
)

func TestSessionManagerConcurrentSessions(t *testing.T) {
	t.Parallel()

	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 8})
	store := &managerTestSessionStore{}
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	start := make(chan struct{})
	results := make(chan string, 2)

	startSession := func(sessionID string) {
		actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
			SessionID: sessionID,
			Task: domain.Task{
				ID:          sessionID,
				Description: sessionID,
				Goal:        sessionID,
				CreatedAt:   time.Now().UTC(),
			},
			NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) {
				return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
					current := concurrent.Add(1)
					defer concurrent.Add(-1)
					for {
						previous := maxConcurrent.Load()
						if current <= previous || maxConcurrent.CompareAndSwap(previous, current) {
							break
						}
					}
					<-start
					return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{ID: task.ID + "-plan", TaskID: task.ID, Summary: task.Description}, []domain.Step{{Index: 1}}, nil
				}), nil
			},
			PersistFinal: persistFinalWithAlias(store, sessionID),
		})
		if err != nil {
			t.Errorf("start %s: %v", sessionID, err)
			return
		}
		<-actor.Done()
		result, ok := actor.Result()
		if !ok {
			t.Errorf("result missing for %s", sessionID)
			return
		}
		results <- fmt.Sprintf("%s:%s", sessionID, result.State)
	}

	go startSession("session-a")
	go startSession("session-b")

	deadline := time.After(2 * time.Second)
	for manager.ActiveCount() < 2 {
		select {
		case <-deadline:
			t.Fatalf("active count did not reach 2, got %d", manager.ActiveCount())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
	close(start)

	got := map[string]runtime.ActorState{}
	for range 2 {
		select {
		case value := <-results:
			parts := strings.SplitN(value, ":", 2)
			if len(parts) != 2 {
				t.Fatalf("invalid result payload %q", value)
			}
			got[parts[0]] = runtime.ActorState(parts[1])
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent session completion")
		}
	}

	if maxConcurrent.Load() < 2 {
		t.Fatalf("max concurrent actors = %d, want at least 2", maxConcurrent.Load())
	}
	if got["session-a"] != runtime.ActorStateCompleted {
		t.Fatalf("session-a state = %q, want %q", got["session-a"], runtime.ActorStateCompleted)
	}
	if got["session-b"] != runtime.ActorStateCompleted {
		t.Fatalf("session-b state = %q, want %q", got["session-b"], runtime.ActorStateCompleted)
	}
	if manager.ActiveCount() != 0 {
		t.Fatalf("active count = %d, want 0", manager.ActiveCount())
	}
	if len(store.saved) != 2 {
		t.Fatalf("persisted sessions = %d, want 2", len(store.saved))
	}
}

func TestSessionManagerRejectsDuplicateResume(t *testing.T) {
	t.Parallel()

	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 8})
	release := make(chan struct{})

	actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-dup",
		Task:      domain.Task{ID: "session-dup", Description: "dup", Goal: "dup", CreatedAt: time.Now().UTC()},
		NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-release
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("start original actor: %v", err)
	}

	_, err = manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-dup",
		Task:      domain.Task{ID: "session-dup", Description: "dup", Goal: "dup", CreatedAt: time.Now().UTC()},
		NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if !errors.Is(err, runtime.ErrSessionAlreadyActive) {
		t.Fatalf("duplicate start error = %v, want %v", err, runtime.ErrSessionAlreadyActive)
	}

	close(release)
	<-actor.Done()
	result, ok := actor.Result()
	if !ok {
		t.Fatal("original actor result missing")
	}
	if result.State != runtime.ActorStateCompleted {
		t.Fatalf("original actor state = %q, want %q", result.State, runtime.ActorStateCompleted)
	}
}

func TestSessionManagerGracefulShutdownCancelsRemainingActors(t *testing.T) {
	t.Parallel()

	store := &managerTestSessionStore{}
	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{
		ActiveSessionCap: 8,
		ShutdownTimeout:  50 * time.Millisecond,
	})

	actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-shutdown",
		Task:      domain.Task{ID: "session-shutdown", Description: "shutdown", Goal: "shutdown", CreatedAt: time.Now().UTC()},
		NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-ctx.Done()
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusInterrupted}, domain.Plan{ID: task.ID + "-plan", TaskID: task.ID}, nil, ctx.Err()
			}), nil
		},
		PersistFinal: persistFinalWithAlias(store, "session-shutdown"),
	})
	if err != nil {
		t.Fatalf("start actor: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	<-actor.Done()
	result, ok := actor.Result()
	if !ok {
		t.Fatal("actor result missing")
	}
	if result.State != runtime.ActorStateCancelled {
		t.Fatalf("actor state = %q, want %q", result.State, runtime.ActorStateCancelled)
	}
	if !result.Cancelled {
		t.Fatal("expected cancelled result")
	}
	persisted := store.last("session-shutdown")
	if persisted.Status != domain.SessionStatusInterrupted {
		t.Fatalf("persisted status = %q, want %q", persisted.Status, domain.SessionStatusInterrupted)
	}
	if manager.IsAccepting() {
		t.Fatal("manager should stop accepting new sessions after shutdown")
	}
	_, err = manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-after-shutdown",
		Task:      domain.Task{ID: "session-after-shutdown"},
		NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) { return nil, nil },
	})
	if !errors.Is(err, runtime.ErrSessionManagerClosed) {
		t.Fatalf("start after shutdown error = %v, want %v", err, runtime.ErrSessionManagerClosed)
	}
}

func TestSessionManagerEnforcesActiveSessionLimit(t *testing.T) {
	t.Parallel()

	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 1, EventBufferSize: 512})
	release := make(chan struct{})

	actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-1",
		Task:      domain.Task{ID: "session-1", Description: "one", Goal: "one", CreatedAt: time.Now().UTC()},
		NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-release
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("start first actor: %v", err)
	}

	_, err = manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-2",
		Task:      domain.Task{ID: "session-2", Description: "two", Goal: "two", CreatedAt: time.Now().UTC()},
		NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if !errors.Is(err, runtime.ErrActiveSessionLimit) {
		t.Fatalf("limit error = %v, want %v", err, runtime.ErrActiveSessionLimit)
	}

	close(release)
	<-actor.Done()
}

func TestSessionActorSubscriptionsReceiveOrderedEvents(t *testing.T) {
	t.Parallel()

	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 1, EventBufferSize: 512})
	release := make(chan struct{})
	emitted := make(chan struct{})
	actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-stream-order",
		Task:      domain.Task{ID: "session-stream-order", Description: "order", Goal: "order", CreatedAt: time.Now().UTC()},
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-release
				defer close(emitted)
				for i := 0; i < 3; i++ {
					event, buildErr := domain.StepComplete(i+1, fmt.Sprintf("step-%d", i+1))
					if buildErr != nil {
						return domain.Session{}, domain.Plan{}, nil, buildErr
					}
					if emitErr := events.Emit(*event); emitErr != nil {
						return domain.Session{}, domain.Plan{}, nil, emitErr
					}
				}
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	sub := actor.Subscribe(256)
	if sub == nil {
		t.Fatal("Subscribe() returned nil")
	}
	defer sub.Close()
	close(release)

	select {
	case <-emitted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for emitted events")
	}

	indices := make([]int, 0, 3)
	for event := range sub.Events() {
		indices = append(indices, event.StepIndex)
	}
	if len(indices) != 3 {
		t.Fatalf("received %d events, want 3", len(indices))
	}
	for i, want := range []int{1, 2, 3} {
		if indices[i] != want {
			t.Fatalf("event %d step index = %d, want %d", i, indices[i], want)
		}
	}
}

func TestSessionActorSlowSubscriberGetsOverflowWithoutCancellingActor(t *testing.T) {
	t.Parallel()

	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 1, EventBufferSize: 512})
	release := make(chan struct{})
	ready := make(chan struct{})
	actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-stream-overflow",
		Task:      domain.Task{ID: "session-stream-overflow", Description: "overflow", Goal: "overflow", CreatedAt: time.Now().UTC()},
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-ready
				for i := 0; i < 300; i++ {
					event, buildErr := domain.TokenDelta(i+1, fmt.Sprintf("chunk-%d", i))
					if buildErr != nil {
						return domain.Session{}, domain.Plan{}, nil, buildErr
					}
					if emitErr := events.Emit(*event); emitErr != nil {
						return domain.Session{}, domain.Plan{}, nil, emitErr
					}
				}
				<-release
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	sub := actor.Subscribe(256)
	if sub == nil {
		t.Fatal("Subscribe() returned nil")
	}
	close(ready)
	time.Sleep(100 * time.Millisecond)
	events := make([]domain.StreamingEvent, 0, 257)
	closed := false
	for !closed {
		select {
		case event, ok := <-sub.Events():
			if !ok {
				closed = true
				continue
			}
			events = append(events, event)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for overflowed subscription to close")
		}
	}
	if len(events) != 256 {
		t.Fatalf("received %d events before close, want 256 buffered events", len(events))
	}
	overflow, ok := sub.OverflowEvent()
	if !ok {
		t.Fatal("expected overflow event")
	}
	if overflow.Type != domain.EventError {
		t.Fatalf("overflow type = %q, want %q", overflow.Type, domain.EventError)
	}
	var payload domain.ErrorPayload
	if err := overflow.GetPayload(&payload); err != nil {
		t.Fatalf("decode overflow payload: %v", err)
	}
	if payload.Message != "stream subscriber fell behind and was disconnected" {
		t.Fatalf("overflow message = %q", payload.Message)
	}
	if manager.ActiveCount() != 1 {
		t.Fatalf("active count = %d, want 1", manager.ActiveCount())
	}
	close(release)
	<-actor.Done()
}

func TestSessionActorSubscribeAfterCompletionReturnsClosedSubscription(t *testing.T) {
	t.Parallel()

	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 1})
	actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-completed-subscribe",
		Task:      domain.Task{ID: "session-completed-subscribe", Description: "complete", Goal: "complete", CreatedAt: time.Now().UTC()},
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				event, buildErr := domain.SessionComplete(task.ID, "success")
				if buildErr != nil {
					return domain.Session{}, domain.Plan{}, nil, buildErr
				}
				if emitErr := events.Emit(*event); emitErr != nil {
					return domain.Session{}, domain.Plan{}, nil, emitErr
				}
				now := time.Now().UTC()
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() { _ = manager.Shutdown(context.Background()) }()

	<-actor.Done()
	sub := actor.Subscribe(4)
	if sub == nil {
		t.Fatal("Subscribe() returned nil")
	}
	defer sub.Close()

	select {
	case _, ok := <-sub.Events():
		if ok {
			t.Fatal("expected subscription created after completion to be closed")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("subscription created after completion remained open")
	}
}

func TestSessionActorRunnerFailurePropagatesToFinalState(t *testing.T) {
	t.Parallel()

	store := &managerTestSessionStore{}
	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 1})
	runErr := errors.New("runner exploded")

	actor, err := manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-runner-failure",
		Task:      domain.Task{ID: "session-runner-failure", Description: "fail", Goal: "fail", CreatedAt: time.Now().UTC()},
		NewRunner: func(_ *runtime.EventChannel) (runtime.SessionRunner, error) {
			return managerTestRunnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusFatalError}, domain.Plan{}, nil, runErr
			}), nil
		},
		PersistFinal: persistFinalWithAlias(store, "session-runner-failure"),
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	<-actor.Done()
	result, ok := actor.Result()
	if !ok {
		t.Fatal("actor result missing")
	}
	if result.State != runtime.ActorStateFailed {
		t.Fatalf("actor state = %q, want %q", result.State, runtime.ActorStateFailed)
	}
	if !errors.Is(result.Err, runErr) {
		t.Fatalf("result err = %v, want %v", result.Err, runErr)
	}
	persisted := store.last("session-runner-failure")
	if persisted.Status != domain.SessionStatusFatalError {
		t.Fatalf("persisted status = %q, want %q", persisted.Status, domain.SessionStatusFatalError)
	}
	if manager.ActiveCount() != 0 {
		t.Fatalf("active count = %d, want 0", manager.ActiveCount())
	}
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

type managerTestRunnerFunc func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error)

func (f managerTestRunnerFunc) Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	return f(ctx, task)
}

type managerTestSessionStore struct {
	mu    sync.Mutex
	saved []domain.Session
}

func (s *managerTestSessionStore) SaveSession(_ context.Context, session domain.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.saved = append(s.saved, session)
	return nil
}

func (s *managerTestSessionStore) SavePlan(_ context.Context, _ domain.Plan) error { return nil }

func (s *managerTestSessionStore) AppendStep(_ context.Context, _ string, _ domain.Step) error { return nil }

func (s *managerTestSessionStore) last(sessionID string) domain.Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.saved) - 1; i >= 0; i-- {
		if s.saved[i].ID == sessionID {
			return s.saved[i]
		}
	}
	return domain.Session{}
}

func persistFinalWithAlias(store *managerTestSessionStore, sessionID string) runtime.SessionActorFinalizer {
	alias := runtimebuilder.NewSessionAliasStore(store, context.Background(), sessionID)
	return func(ctx context.Context, result runtime.SessionActorResult) error {
		session := result.Session
		if session.TaskID == "" {
			session.TaskID = sessionID
		}
		if session.CreatedAt.IsZero() {
			session.CreatedAt = result.CreatedAt
		}
		if session.UpdatedAt.IsZero() {
			session.UpdatedAt = result.FinishedAt
		}
		if session.Status == "" {
			session.Status = mapActorStateToSessionStatus(result.State)
		}
		return alias.SaveSession(ctx, session)
	}
}

func mapActorStateToSessionStatus(state runtime.ActorState) domain.SessionStatus {
	switch state {
	case runtime.ActorStateCompleted:
		return domain.SessionStatusSuccess
	case runtime.ActorStateCancelled:
		return domain.SessionStatusInterrupted
	default:
		return domain.SessionStatusFatalError
	}
}
