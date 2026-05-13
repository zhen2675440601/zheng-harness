package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/store"
)

func TestStartConversationPersistsAndStartsSession(t *testing.T) {
	t.Parallel()
	svc, manager, sessionStore := newTestChatService(t)
	block := make(chan struct{})
	svc.WithEngineFactory(func(_ *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
		if task.Description != "demo task" {
			t.Fatalf("task.Description = %q, want demo task", task.Description)
		}
		if maxSteps != 3 {
			t.Fatalf("maxSteps = %d, want 3", maxSteps)
		}
		if verifyMode != "strict" {
			t.Fatalf("verifyMode = %q, want strict", verifyMode)
		}
		return blockingRunner{done: block}, nil
	})

	result, err := svc.StartConversation(context.Background(), "demo task", StartConversationOptions{TaskType: "coding", MaxSteps: 3, VerifyMode: "strict"})
	if err != nil {
		t.Fatalf("StartConversation() error = %v", err)
	}
	if result.SessionID == "" || result.ConversationID != result.SessionID {
		t.Fatalf("unexpected start result = %#v", result)
	}
	if result.Status != "created" {
		t.Fatalf("status = %q, want created", result.Status)
	}
	if result.StreamURL != svc.StreamURL(result.SessionID) {
		t.Fatalf("streamURL = %q, want %q", result.StreamURL, svc.StreamURL(result.SessionID))
	}

	inspected, err := sessionStore.InspectSession(context.Background(), result.SessionID)
	if err != nil {
		t.Fatalf("InspectSession() error = %v", err)
	}
	if inspected.Session.Status != domain.SessionStatusPending {
		t.Fatalf("persisted session status = %q, want %q", inspected.Session.Status, domain.SessionStatusPending)
	}
	records, err := sessionStore.ListConversationSessions(context.Background(), result.ConversationID)
	if err != nil {
		t.Fatalf("ListConversationSessions() error = %v", err)
	}
	if len(records) != 1 || records[0].ConversationID != result.ConversationID || records[0].TurnIndex != 0 {
		t.Fatalf("unexpected conversation records = %#v", records)
	}
	if manager.ActiveCount() != 1 {
		t.Fatalf("active count = %d, want 1", manager.ActiveCount())
	}
	close(block)
	_ = manager.Shutdown(context.Background())
}

func TestStartConversationRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestChatService(t)

	_, err := svc.StartConversation(context.Background(), " ", StartConversationOptions{})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Error() != "task is required" {
		t.Fatalf("StartConversation() error = %v, want task is required validation error", err)
	}
}

func TestStartConversationRejectsUnsupportedTaskType(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestChatService(t)

	_, err := svc.StartConversation(context.Background(), "demo task", StartConversationOptions{TaskType: "unsupported-mode"})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Error() != "task_type must be one of general, coding, research, file_workflow" {
		t.Fatalf("StartConversation() error = %v, want unsupported task_type validation error", err)
	}
}

func TestStartConversationReturnsProviderError(t *testing.T) {
	t.Parallel()
	svc, manager, sessionStore := newTestChatService(t)
	wantErr := errors.New("provider unavailable")
	svc.WithEngineFactory(func(_ *runtime.EventChannel, _ domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		return nil, wantErr
	})

	result, err := svc.StartConversation(context.Background(), "demo task", StartConversationOptions{})
	if err != nil {
		t.Fatalf("StartConversation() error = %v", err)
	}
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	time.Sleep(50 * time.Millisecond)
	inspected, inspectErr := sessionStore.InspectSession(context.Background(), result.SessionID)
	if inspectErr != nil {
		t.Fatalf("InspectSession() error = %v", inspectErr)
	}
	if inspected.Session.Status != domain.SessionStatusFatalError {
		t.Fatalf("persisted session status = %q, want %q", inspected.Session.Status, domain.SessionStatusFatalError)
	}
}

func TestSubmitReplyCreatesFollowupSession(t *testing.T) {
	t.Parallel()
	svc, manager, sessionStore := newTestChatService(t)
	firstBlock := make(chan struct{})
	secondBlock := make(chan struct{})
	callCount := 0
	svc.WithEngineFactory(func(_ *runtime.EventChannel, task domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		callCount++
		switch callCount {
		case 1:
			return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				select {
				case <-ctx.Done():
					return domain.Session{}, domain.Plan{}, nil, ctx.Err()
				case <-firstBlock:
				}
				now := time.Now().UTC()
				if err := sessionStore.SaveSession(ctx, domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}); err != nil {
					return domain.Session{}, domain.Plan{}, nil, err
				}
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description, CreatedAt: now}, nil, nil
			}), nil
		case 2:
			if !strings.Contains(task.Description, "Conversation history:\nUser:\nfirst message") {
				t.Fatalf("follow-up task.Description missing history context: %q", task.Description)
			}
			if !strings.Contains(task.Description, "Current user message:\nfollow-up") {
				t.Fatalf("follow-up task.Description missing current message: %q", task.Description)
			}
			if task.Goal != "follow-up" {
				t.Fatalf("follow-up task.Goal = %q, want follow-up", task.Goal)
			}
			return blockingRunner{done: secondBlock}, nil
		default:
			t.Fatalf("unexpected engine start count %d", callCount)
			return blockingRunner{done: secondBlock}, nil
		}
	})

	start, err := svc.StartChat(context.Background(), StartChatRequest{Message: "first message", TaskType: "coding"})
	if err != nil {
		t.Fatalf("StartChat() error = %v", err)
	}
	close(firstBlock)
	time.Sleep(50 * time.Millisecond)

	reply, err := svc.SubmitReply(context.Background(), start.ConversationID, "follow-up")
	if err != nil {
		t.Fatalf("SubmitReply() error = %v", err)
	}
	if reply.ConversationID != start.ConversationID || reply.TurnIndex != 1 || reply.SessionID == start.SessionID {
		t.Fatalf("unexpected reply result = %#v", reply)
	}
	records, err := sessionStore.ListConversationSessions(context.Background(), start.ConversationID)
	if err != nil {
		t.Fatalf("ListConversationSessions() error = %v", err)
	}
	if len(records) != 2 || records[1].ParentSessionID != start.SessionID || records[1].TurnIndex != 1 {
		t.Fatalf("unexpected conversation chain = %#v", records)
	}
	close(secondBlock)
	_ = manager.Shutdown(context.Background())
}

func TestSubmitReplyRejectsRunningConversation(t *testing.T) {
	t.Parallel()
	svc, manager, _ := newTestChatService(t)
	block := make(chan struct{})
	svc.WithEngineFactory(func(_ *runtime.EventChannel, _ domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		return blockingRunner{done: block}, nil
	})
	start, err := svc.StartChat(context.Background(), StartChatRequest{Message: "still running"})
	if err != nil {
		t.Fatalf("StartChat() error = %v", err)
	}
	_, err = svc.SubmitReply(context.Background(), start.ConversationID, "follow-up")
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) || conflictErr.Error() != "conversation is already running" {
		t.Fatalf("SubmitReply() error = %v, want running conflict", err)
	}
	close(block)
	_ = manager.Shutdown(context.Background())
}

func TestStartChatRejectsUnsupportedTaskType(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestChatService(t)

	_, err := svc.StartChat(context.Background(), StartChatRequest{Message: "hello", TaskType: "unsupported-mode"})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Error() != "task_type must be one of general, coding, research, file_workflow" {
		t.Fatalf("StartChat() error = %v, want unsupported task_type validation error", err)
	}
}

func TestSubmitReplyReturnsNotFoundForMissingConversation(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestChatService(t)

	_, err := svc.SubmitReply(context.Background(), "missing-conversation", "follow-up")
	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) || notFoundErr.Kind != "conversation" || notFoundErr.ID != "missing-conversation" {
		t.Fatalf("SubmitReply() error = %v, want conversation not found", err)
	}
}

func TestSubmitReplyAcceptsSessionIDForConversationLookup(t *testing.T) {
	t.Parallel()
	svc, manager, _ := newTestChatService(t)
	firstBlock := make(chan struct{})
	secondBlock := make(chan struct{})
	callCount := 0
	svc.WithEngineFactory(func(_ *runtime.EventChannel, task domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		callCount++
		switch callCount {
		case 1:
			return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				select {
				case <-ctx.Done():
					return domain.Session{}, domain.Plan{}, nil, ctx.Err()
				case <-firstBlock:
				}
				now := time.Now().UTC()
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description, CreatedAt: now}, nil, nil
			}), nil
		case 2:
			return blockingRunner{done: secondBlock}, nil
		default:
			t.Fatalf("unexpected engine start count %d", callCount)
			return blockingRunner{done: secondBlock}, nil
		}
	})

	start, err := svc.StartChat(context.Background(), StartChatRequest{Message: "first message", TaskType: "coding"})
	if err != nil {
		t.Fatalf("StartChat() error = %v", err)
	}
	close(firstBlock)
	time.Sleep(50 * time.Millisecond)

	reply, err := svc.SubmitReply(context.Background(), start.SessionID, "follow-up")
	if err != nil {
		t.Fatalf("SubmitReply() error = %v", err)
	}
	if reply.ConversationID != start.ConversationID {
		t.Fatalf("reply.ConversationID = %q, want %q", reply.ConversationID, start.ConversationID)
	}
	close(secondBlock)
	_ = manager.Shutdown(context.Background())
}

func TestResumeConversationRejectsTerminalSession(t *testing.T) {
	t.Parallel()
	svc, _, sessionStore := newTestChatService(t)
	now := svc.Now().UTC()
	if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: "sess-1", TaskID: "sess-1", Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(context.Background(), "sess-1", domain.Task{ID: "sess-1", Description: "done", Goal: "done", CreatedAt: now}.Normalize()); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	_, err := svc.ResumeConversation(context.Background(), "sess-1")
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) || conflictErr.Error() != "session is not resumable" {
		t.Fatalf("ResumeConversation() error = %v, want session is not resumable conflict", err)
	}
}

func TestResumeConversationStartsResumableSession(t *testing.T) {
	t.Parallel()
	svc, manager, sessionStore := newTestChatService(t)
	block := make(chan struct{})
	now := svc.Now().UTC()
	if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: "sess-resume", TaskID: "sess-resume", Status: domain.SessionStatusBlockedInput, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(context.Background(), "sess-resume", domain.Task{ID: "sess-resume", Description: "resume me", Goal: "resume me", CreatedAt: now}.Normalize()); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	svc.WithEngineFactory(func(_ *runtime.EventChannel, task domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		if task.ID != "sess-resume" {
			t.Fatalf("unexpected resumed task = %#v", task)
		}
		return blockingRunner{done: block}, nil
	})

	result, err := svc.ResumeConversation(context.Background(), "sess-resume")
	if err != nil {
		t.Fatalf("ResumeConversation() error = %v", err)
	}
	if result.SessionID != "sess-resume" || result.ConversationID != "sess-resume" || result.Status != "running" {
		t.Fatalf("unexpected resume result = %#v", result)
	}
	if manager.ActiveCount() != 1 {
		t.Fatalf("active count = %d, want 1", manager.ActiveCount())
	}
	close(block)
	_ = manager.Shutdown(context.Background())
}

func TestGetTranscriptBuildsConversationProjection(t *testing.T) {
	t.Parallel()
	svc, _, sessionStore := newTestChatService(t)
	now := svc.Now().UTC()
	if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: "sess-2", TaskID: "sess-2", Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now.Add(2 * time.Second)}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(context.Background(), "sess-2", domain.Task{ID: "sess-2", Description: "inspect me", Goal: "inspect me", CreatedAt: now, Category: domain.TaskCategoryCoding}.Normalize()); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	step := domain.Step{Index: 1, Action: domain.Action{Type: domain.ActionTypeToolCall, ToolCall: &domain.ToolCall{Name: "bash", Input: "echo hi"}}, Observation: domain.Observation{Summary: "ran command", FinalResponse: "done"}, Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "ok"}}
	if err := sessionStore.AppendStep(context.Background(), "sess-2", step); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	transcript, err := svc.GetTranscript(context.Background(), "sess-2")
	if err != nil {
		t.Fatalf("GetTranscript() error = %v", err)
	}
	if transcript.Chat.ConversationID != "sess-2" {
		t.Fatalf("conversation id = %q, want sess-2", transcript.Chat.ConversationID)
	}
	if len(transcript.Chat.Messages) < 3 {
		t.Fatalf("message count = %d, want at least 3", len(transcript.Chat.Messages))
	}
	if transcript.Conversation.Status != domain.ConversationStatusActive {
		t.Fatalf("conversation status = %q, want %q", transcript.Conversation.Status, domain.ConversationStatusActive)
	}
}

func TestGetTranscriptReturnsNotFoundWhenConversationMissing(t *testing.T) {
	t.Parallel()
	svc, _, _ := newTestChatService(t)

	_, err := svc.GetTranscript(context.Background(), "missing-conversation")
	var notFoundErr *NotFoundError
	if !errors.As(err, &notFoundErr) || notFoundErr.Kind != "conversation" || notFoundErr.ID != "missing-conversation" {
		t.Fatalf("GetTranscript() error = %v, want conversation not found", err)
	}
}

func TestGetTranscriptBuildsEmptyStateFromStandaloneSession(t *testing.T) {
	t.Parallel()
	svc, _, sessionStore := newTestChatService(t)
	now := svc.Now().UTC()
	if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: "sess-empty", TaskID: "sess-empty", Status: domain.SessionStatusPending, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(context.Background(), "sess-empty", domain.Task{ID: "sess-empty", Description: "empty", Goal: "empty", CreatedAt: now}.Normalize()); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	transcript, err := svc.GetTranscript(context.Background(), "sess-empty")
	if err != nil {
		t.Fatalf("GetTranscript() error = %v", err)
	}
	if transcript.Chat.ConversationID != "sess-empty" {
		t.Fatalf("conversation id = %q, want sess-empty", transcript.Chat.ConversationID)
	}
	if transcript.Conversation.ID != "sess-empty" {
		t.Fatalf("conversation projection id = %q, want sess-empty", transcript.Conversation.ID)
	}
	if len(transcript.Chat.Messages) != 0 {
		t.Fatalf("message count = %d, want 0 for empty state", len(transcript.Chat.Messages))
	}
}

func TestGetTranscriptExposesDiagnosticCorrelationAndFailureReason(t *testing.T) {
	t.Parallel()
	svc, _, sessionStore := newTestChatService(t)
	now := svc.Now().UTC()
	if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: "sess-failed", TaskID: "sess-failed", Status: domain.SessionStatusFatalError, CreatedAt: now, UpdatedAt: now.Add(time.Second)}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(context.Background(), "sess-failed", domain.Task{ID: "sess-failed", Description: "failed run", Goal: "failed run", CreatedAt: now, Category: domain.TaskCategoryCoding}.Normalize()); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := sessionStore.SaveDiagnostics(context.Background(), "sess-failed", store.SessionDiagnostics{RequestID: "req-service-123", FailureReason: "runner initialization failed", Finalized: true}); err != nil {
		t.Fatalf("SaveDiagnostics() error = %v", err)
	}

	transcript, err := svc.GetTranscript(context.Background(), "sess-failed")
	if err != nil {
		t.Fatalf("GetTranscript() error = %v", err)
	}
	if transcript.Inspect.RequestID != "req-service-123" {
		t.Fatalf("inspect request id = %q, want req-service-123", transcript.Inspect.RequestID)
	}
	if transcript.Inspect.FailureReason != "runner initialization failed" {
		t.Fatalf("inspect failure reason = %q, want runner initialization failed", transcript.Inspect.FailureReason)
	}
	if !transcript.Inspect.Finalized {
		t.Fatal("inspect finalized = false, want true")
	}
}

func TestListConversationsMapsSessionSummaries(t *testing.T) {
	t.Parallel()
	svc, _, sessionStore := newTestChatService(t)
	now := svc.Now().UTC()
	for i, status := range []domain.SessionStatus{domain.SessionStatusPending, domain.SessionStatusSuccess} {
		id := string([]byte{'s', 'e', 's', 's', '-', 'l', 'i', 's', 't', '-', byte('a' + i)})
		if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: id, TaskID: id, Status: status, CreatedAt: now.Add(time.Duration(i) * time.Second), UpdatedAt: now.Add(time.Duration(i) * time.Second)}); err != nil {
			t.Fatalf("SaveSession(%s) error = %v", id, err)
		}
		if err := sessionStore.SaveTask(context.Background(), id, domain.Task{ID: id, Description: id, Goal: id, CreatedAt: now, Category: domain.TaskCategoryGeneral}.Normalize()); err != nil {
			t.Fatalf("SaveTask(%s) error = %v", id, err)
		}
	}

	result, err := svc.ListConversations(context.Background(), ListConversationsParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("total = %d, want 2", result.Total)
	}
	if len(result.Conversations) != 2 {
		t.Fatalf("conversations len = %d, want 2", len(result.Conversations))
	}
}

func TestListConversationsSupportsPaginationFilteringAndEmptyResults(t *testing.T) {
	t.Parallel()
	svc, _, sessionStore := newTestChatService(t)
	base := svc.Now().UTC()
	for i, tc := range []struct {
		id     string
		status domain.SessionStatus
	}{
		{id: "sess-page-a", status: domain.SessionStatusPending},
		{id: "sess-page-b", status: domain.SessionStatusRunning},
		{id: "sess-page-c", status: domain.SessionStatusSuccess},
	} {
		createdAt := base.Add(time.Duration(i) * time.Minute)
		if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: tc.id, TaskID: tc.id, Status: tc.status, CreatedAt: createdAt, UpdatedAt: createdAt}); err != nil {
			t.Fatalf("SaveSession(%s) error = %v", tc.id, err)
		}
		if err := sessionStore.SaveTask(context.Background(), tc.id, domain.Task{ID: tc.id, Description: tc.id, Goal: tc.id, CreatedAt: createdAt}.Normalize()); err != nil {
			t.Fatalf("SaveTask(%s) error = %v", tc.id, err)
		}
	}

	paged, err := svc.ListConversations(context.Background(), ListConversationsParams{Page: 2, PageSize: 1})
	if err != nil {
		t.Fatalf("ListConversations(page) error = %v", err)
	}
	if paged.Total != 3 || paged.Page != 2 || paged.PageSize != 1 {
		t.Fatalf("paged result = %#v, want total=3 page=2 page_size=1", paged)
	}
	if len(paged.Conversations) != 1 || paged.Conversations[0].SessionID != "sess-page-b" {
		t.Fatalf("paged conversations = %#v, want sess-page-b only", paged.Conversations)
	}

	filtered, err := svc.ListConversations(context.Background(), ListConversationsParams{Page: 1, PageSize: 10, Status: "completed"})
	if err != nil {
		t.Fatalf("ListConversations(filtered) error = %v", err)
	}
	if filtered.Total != 1 || len(filtered.Conversations) != 1 || filtered.Conversations[0].SessionID != "sess-page-c" {
		t.Fatalf("filtered conversations = %#v, want sess-page-c only", filtered)
	}

	empty, err := svc.ListConversations(context.Background(), ListConversationsParams{Page: 1, PageSize: 10, Status: "failed"})
	if err != nil {
		t.Fatalf("ListConversations(empty) error = %v", err)
	}
	if empty.Total != 0 || len(empty.Conversations) != 0 {
		t.Fatalf("empty conversations = %#v, want zero results", empty)
	}
}

func TestValidateStreamSessionReturnsNilForTerminalSession(t *testing.T) {
	t.Parallel()
	svc, _, sessionStore := newTestChatService(t)
	now := svc.Now().UTC()
	if err := sessionStore.SaveSession(context.Background(), domain.Session{ID: "sess-term", TaskID: "sess-term", Status: domain.SessionStatusFatalError, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := sessionStore.SaveTask(context.Background(), "sess-term", domain.Task{ID: "sess-term", Description: "failed", Goal: "failed", CreatedAt: now}.Normalize()); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	actor, err := svc.ValidateStreamSession(context.Background(), "sess-term")
	if err != nil {
		t.Fatalf("ValidateStreamSession() error = %v", err)
	}
	if actor != nil {
		t.Fatal("expected nil actor for terminal session")
	}
}

type blockingRunner struct{ done <-chan struct{} }

type runnerFunc func(context.Context, domain.Task) (domain.Session, domain.Plan, []domain.Step, error)

func (f runnerFunc) Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	return f(ctx, task)
}

func (r blockingRunner) Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	select {
	case <-ctx.Done():
		return domain.Session{}, domain.Plan{}, nil, ctx.Err()
	case <-r.done:
		now := time.Now().UTC()
		return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description, CreatedAt: now}, nil, nil
	}
}

func newTestChatService(t *testing.T) (*ChatService, *runtime.SessionManager, *store.SQLiteSessionStore) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "service.db")
	sessionStore, err := store.NewSQLiteSessionStoreWithOptions(dbPath, store.SQLiteOptions{EnableWAL: true})
	if err != nil {
		t.Fatalf("NewSQLiteSessionStoreWithOptions() error = %v", err)
	}
	memoryStore, err := store.NewMemoryStoreWithOptions(dbPath, store.SQLiteOptions{EnableWAL: true})
	if err != nil {
		_ = sessionStore.Close()
		t.Fatalf("NewMemoryStoreWithOptions() error = %v", err)
	}
	t.Cleanup(func() {
		_ = memoryStore.Close()
		_ = sessionStore.Close()
	})
	manager := runtime.NewSessionManager(runtime.SessionManagerOptions{ActiveSessionCap: 2, ShutdownTimeout: time.Second})
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	clockBase := time.Date(2026, 5, 6, 10, 30, 0, 0, time.UTC)
	clockTick := 0
	svc := NewChatService(Dependencies{
		SessionStore:   sessionStore,
		MemoryStore:    memoryStore,
		SessionManager: manager,
		Config:         config.Config{},
		Clock: func() time.Time {
			current := clockBase.Add(time.Duration(clockTick) * time.Nanosecond)
			clockTick++
			return current
		},
	})
	return svc, manager, sessionStore
}
