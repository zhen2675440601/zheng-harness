package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
	"zheng-harness/internal/store"
)

type EngineFactory func(events *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error)

type ChatService struct {
	sessionStore  *store.SQLiteSessionStore
	memoryStore   *store.SQLiteMemoryStore
	manager       *runtime.SessionManager
	builder       *runtimebuilder.Builder
	config        config.Config
	engineFactory EngineFactory
	clock         func() time.Time
}

type Dependencies struct {
	SessionStore   *store.SQLiteSessionStore
	MemoryStore    *store.SQLiteMemoryStore
	SessionManager *runtime.SessionManager
	Builder        *runtimebuilder.Builder
	Config         config.Config
	EngineFactory  EngineFactory
	Clock          func() time.Time
}

type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

type NotFoundError struct {
	Kind string
	ID   string
	Err  error
}

func (e *NotFoundError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Kind) == "" {
		return "not found"
	}
	return e.Kind + " not found"
}

func (e *NotFoundError) Unwrap() error { return e.Err }

type ConflictError struct {
	Message string
	Err     error
}

func (e *ConflictError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "conflict"
}

func (e *ConflictError) Unwrap() error { return e.Err }

func NewChatService(deps Dependencies) *ChatService {
	return &ChatService{
		sessionStore:  deps.SessionStore,
		memoryStore:   deps.MemoryStore,
		manager:       deps.SessionManager,
		builder:       deps.Builder,
		config:        deps.Config,
		engineFactory: deps.EngineFactory,
		clock:         deps.Clock,
	}
}

func (s *ChatService) WithEngineFactory(factory EngineFactory) {
	if s == nil {
		return
	}
	s.engineFactory = factory
}

func (s *ChatService) ValidateProvenanceForResume(p *domain.Provenance) []string {
	return s.validateProvenanceForResume(p)
}

func (s *ChatService) Now() time.Time {
	if s != nil && s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

func (s *ChatService) StreamURL(sessionID string) string {
	return "/api/v1/sessions/" + strings.TrimSpace(sessionID) + "/stream"
}

func (s *ChatService) ValidateStreamSession(ctx context.Context, sessionID string) (*runtime.SessionActor, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, &ValidationError{Message: "session id is required"}
	}
	if s == nil || s.manager == nil {
		return nil, errors.New("chat service session manager is not initialized")
	}
	if actor, ok := s.manager.Lookup(sessionID); ok {
		return actor, nil
	}
	inspected, err := s.sessionStore.InspectSession(ctx, sessionID)
	if err != nil {
		return nil, mapStoreLookupError("session", sessionID, err)
	}
	if isTerminalStatus(inspected.Session.Status) {
		return nil, nil
	}
	return nil, &ConflictError{Message: "session stream is not active"}
}
