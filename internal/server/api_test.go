package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/store"
)

//go:embed testdata/web/index.html
var testWebMountFS embed.FS

func TestRunRequiresAuthentication(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(`{"task":"demo"}`))
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("POST /api/v1/run status = %d, want 401", rec.Code)
	}
}

func TestWebUIRoutesMountedWithoutBreakingAPI(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarnessWithOptions(t, testAPIHarnessOptions{webUIEnabled: true})
	h.api.EngineFactory = func(_ *runtime.EventChannel, _ domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		return fakeRunner{}, nil
	}

	rootRec := httptest.NewRecorder()
	h.router.ServeHTTP(rootRec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rootRec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want 200, body=%s", rootRec.Code, rootRec.Body.String())
	}
	if got := rootRec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("GET / content-type = %q, want text/html; charset=utf-8", got)
	}

	apiRec := httptest.NewRecorder()
	apiReq := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(`{"task":"demo"}`))
	apiReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(apiRec, apiReq)
	if apiRec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/run status = %d, want 202, body=%s", apiRec.Code, apiRec.Body.String())
	}
	_ = h.manager.Shutdown(context.Background())
}

func TestRunAcceptedWithAuthenticatedRequest(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	block := make(chan struct{})
	h.api.EngineFactory = func(_ *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
		if task.ID == "" || task.Description != "demo task" {
			t.Fatalf("unexpected task = %#v", task)
		}
		if maxSteps != 3 {
			t.Fatalf("maxSteps = %d, want 3", maxSteps)
		}
		if verifyMode != "strict" {
			t.Fatalf("verifyMode = %q, want strict", verifyMode)
		}
		return blockingRunner{done: block}, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(`{"task":"demo task","task_type":"coding","max_steps":3,"verify_mode":"strict"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/run status = %d, want 202", rec.Code)
	}
	var payload sessionAcceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.SessionID == "" {
		t.Fatal("session_id empty")
	}
	if payload.Status != "created" {
		t.Fatalf("status = %q, want created", payload.Status)
	}
	wantStreamURL := "/api/v1/sessions/" + payload.SessionID + "/stream"
	if payload.StreamURL != wantStreamURL {
		t.Fatalf("stream_url = %q, want %q", payload.StreamURL, wantStreamURL)
	}
	inspected, err := h.sessionStore.InspectSession(context.Background(), payload.SessionID)
	if err != nil {
		t.Fatalf("InspectSession() error = %v", err)
	}
	if inspected.Task.Description != "demo task" {
		t.Fatalf("persisted task description = %q, want demo task", inspected.Task.Description)
	}
	if inspected.Session.Status != domain.SessionStatusPending {
		t.Fatalf("persisted session status = %q, want %q", inspected.Session.Status, domain.SessionStatusPending)
	}
	if h.manager.ActiveCount() != 1 {
		t.Fatalf("active count = %d, want 1", h.manager.ActiveCount())
	}
	close(block)
	_ = h.manager.Shutdown(context.Background())
}

func TestChatStartReplyTranscriptAndList(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	firstBlock := make(chan struct{})
	secondBlock := make(chan struct{})
	callCount := 0
	h.api.EngineFactory = func(_ *runtime.EventChannel, task domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		callCount++
		switch callCount {
		case 1:
			if task.Description != "hello" {
				t.Fatalf("first task = %#v", task)
			}
			return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				select {
				case <-ctx.Done():
					return domain.Session{}, domain.Plan{}, nil, ctx.Err()
				case <-firstBlock:
				}
				now := time.Now().UTC()
				if err := h.sessionStore.SaveSession(ctx, domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}); err != nil {
					return domain.Session{}, domain.Plan{}, nil, err
				}
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description, CreatedAt: now}, nil, nil
			}), nil
		case 2:
			if task.Description != "follow-up" {
				t.Fatalf("second task = %#v", task)
			}
			return blockingRunner{done: secondBlock}, nil
		default:
			t.Fatalf("unexpected engine start count %d", callCount)
			return blockingRunner{done: secondBlock}, nil
		}
	}

	startRec := httptest.NewRecorder()
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/chat/start", bytes.NewBufferString(`{"message":"hello","task_type":"coding"}`))
	startReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/chat/start status = %d, want 202 body=%s", startRec.Code, startRec.Body.String())
	}
	var startPayload chatStartResponse
	if err := json.Unmarshal(startRec.Body.Bytes(), &startPayload); err != nil {
		t.Fatalf("unmarshal start payload: %v", err)
	}
	if startPayload.ConversationID == "" || startPayload.SessionID == "" || startPayload.ConversationID == startPayload.SessionID {
		t.Fatalf("unexpected start payload = %#v", startPayload)
	}

	close(firstBlock)
	time.Sleep(50 * time.Millisecond)

	replyRec := httptest.NewRecorder()
	replyReq := httptest.NewRequest(http.MethodPost, "/api/v1/chat/"+startPayload.ConversationID+"/reply", bytes.NewBufferString(`{"message":"follow-up"}`))
	replyReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(replyRec, replyReq)
	if replyRec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/chat/{conversation_id}/reply status = %d, want 202 body=%s", replyRec.Code, replyRec.Body.String())
	}
	var replyPayload chatReplyResponse
	if err := json.Unmarshal(replyRec.Body.Bytes(), &replyPayload); err != nil {
		t.Fatalf("unmarshal reply payload: %v", err)
	}
	if replyPayload.ConversationID != startPayload.ConversationID || replyPayload.TurnIndex != 1 {
		t.Fatalf("unexpected reply payload = %#v", replyPayload)
	}

	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/chat/conversations", nil)
	listReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/chat/conversations status = %d, want 200 body=%s", listRec.Code, listRec.Body.String())
	}
	var listPayload struct {
		Conversations []map[string]any `json:"conversations"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	if len(listPayload.Conversations) == 0 {
		t.Fatal("expected at least one conversation")
	}

	transcriptRec := httptest.NewRecorder()
	transcriptReq := httptest.NewRequest(http.MethodGet, "/api/v1/chat/"+startPayload.ConversationID+"/transcript", nil)
	transcriptReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(transcriptRec, transcriptReq)
	if transcriptRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/chat/{conversation_id}/transcript status = %d, want 200 body=%s", transcriptRec.Code, transcriptRec.Body.String())
	}
	var transcriptPayload chatTranscriptResponse
	if err := json.Unmarshal(transcriptRec.Body.Bytes(), &transcriptPayload); err != nil {
		t.Fatalf("unmarshal transcript payload: %v", err)
	}
	if transcriptPayload.ConversationID != startPayload.ConversationID || len(transcriptPayload.Messages) == 0 {
		t.Fatalf("unexpected transcript payload = %#v", transcriptPayload)
	}
	close(secondBlock)
	_ = h.manager.Shutdown(context.Background())
}

func TestChatEndpointsRequireAuthentication(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "start", method: http.MethodPost, path: "/api/v1/chat/start", body: `{"message":"hello"}`},
		{name: "reply", method: http.MethodPost, path: "/api/v1/chat/missing/reply", body: `{"message":"hello"}`},
		{name: "transcript", method: http.MethodGet, path: "/api/v1/chat/missing/transcript"},
		{name: "list", method: http.MethodGet, path: "/api/v1/chat/conversations"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			var body io.Reader
			if tc.body != "" {
				body = bytes.NewBufferString(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			h.router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("%s %s status = %d, want 401", tc.method, tc.path, rec.Code)
			}
		})
	}
}

func TestChatReplyReturns404ForMissingConversation(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/missing/reply", bytes.NewBufferString(`{"message":"follow-up"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /api/v1/chat/{conversation_id}/reply status = %d, want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestRunReturnsStructuredValidationEnvelopeForUnsupportedTaskType(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(`{"task":"demo","task_type":"unsupported-mode"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/run status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	var payload errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal validation payload: %v", err)
	}
	if payload.Error.Code != "invalid_request" || payload.Error.Message != "task_type must be one of general, coding, research, file_workflow" {
		t.Fatalf("validation payload = %#v, want structured unsupported task_type envelope", payload)
	}
}

func TestChatStartReturnsStructuredValidationEnvelopeForUnsupportedTaskType(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/start", bytes.NewBufferString(`{"message":"hello","task_type":"unsupported-mode"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/chat/start status = %d, want 400 body=%s", rec.Code, rec.Body.String())
	}
	var payload errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal validation payload: %v", err)
	}
	if payload.Error.Code != "invalid_request" || payload.Error.Message != "task_type must be one of general, coding, research, file_workflow" {
		t.Fatalf("validation payload = %#v, want structured unsupported task_type envelope", payload)
	}
}

func TestChatTranscriptReturns404ForMissingConversation(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/chat/missing/transcript", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/chat/{conversation_id}/transcript status = %d, want 404 body=%s", rec.Code, rec.Body.String())
	}
}

func TestChatReplyReturns409ForRunningConversation(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	block := make(chan struct{})
	h.api.EngineFactory = func(_ *runtime.EventChannel, _ domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		return blockingRunner{done: block}, nil
	}

	startRec := httptest.NewRecorder()
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/chat/start", bytes.NewBufferString(`{"message":"hello"}`))
	startReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(startRec, startReq)
	if startRec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/chat/start status = %d, want 202 body=%s", startRec.Code, startRec.Body.String())
	}
	var startPayload chatStartResponse
	if err := json.Unmarshal(startRec.Body.Bytes(), &startPayload); err != nil {
		t.Fatalf("unmarshal start payload: %v", err)
	}

	replyRec := httptest.NewRecorder()
	replyReq := httptest.NewRequest(http.MethodPost, "/api/v1/chat/"+startPayload.ConversationID+"/reply", bytes.NewBufferString(`{"message":"follow-up"}`))
	replyReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(replyRec, replyReq)
	if replyRec.Code != http.StatusConflict {
		t.Fatalf("POST /api/v1/chat/{conversation_id}/reply status = %d, want 409 body=%s", replyRec.Code, replyRec.Body.String())
	}
	close(block)
	_ = h.manager.Shutdown(context.Background())
}

func TestSessionIDContractUsesSessionIDFieldOnly(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)

	// 1) run response contract
	runRec := httptest.NewRecorder()
	runReq := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(`{"task":"contract test","task_type":"general"}`))
	runReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/run status = %d, want 202", runRec.Code)
	}
	var runPayload map[string]any
	if err := json.Unmarshal(runRec.Body.Bytes(), &runPayload); err != nil {
		t.Fatalf("unmarshal run payload: %v", err)
	}
	if _, ok := runPayload["session_id"]; !ok {
		t.Fatalf("run payload missing session_id: %#v", runPayload)
	}
	if _, hasLegacyID := runPayload["id"]; hasLegacyID {
		t.Fatalf("run payload must not include legacy id field: %#v", runPayload)
	}
	sessionID, _ := runPayload["session_id"].(string)
	if strings.TrimSpace(sessionID) == "" {
		t.Fatalf("run payload session_id empty: %#v", runPayload)
	}

	// 2) inspect response contract
	inspectRec := httptest.NewRecorder()
	inspectReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sessionID+"/inspect", nil)
	inspectReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(inspectRec, inspectReq)
	if inspectRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/sessions/{id}/inspect status = %d, want 200", inspectRec.Code)
	}
	var inspectPayload map[string]any
	if err := json.Unmarshal(inspectRec.Body.Bytes(), &inspectPayload); err != nil {
		t.Fatalf("unmarshal inspect payload: %v", err)
	}
	if _, ok := inspectPayload["session_id"]; !ok {
		t.Fatalf("inspect payload missing session_id: %#v", inspectPayload)
	}
	if _, hasLegacyID := inspectPayload["id"]; hasLegacyID {
		t.Fatalf("inspect payload must not include legacy id field: %#v", inspectPayload)
	}

	// 3) list response contract
	listRec := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=1&page_size=20", nil)
	listReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/sessions status = %d, want 200", listRec.Code)
	}
	var listPayload map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("unmarshal list payload: %v", err)
	}
	sessionsRaw, ok := listPayload["sessions"].([]any)
	if !ok || len(sessionsRaw) == 0 {
		t.Fatalf("list payload sessions missing/empty: %#v", listPayload)
	}
	for i, itemRaw := range sessionsRaw {
		item, ok := itemRaw.(map[string]any)
		if !ok {
			t.Fatalf("session item[%d] invalid shape: %#v", i, itemRaw)
		}
		if _, ok := item["session_id"]; !ok {
			t.Fatalf("session item[%d] missing session_id: %#v", i, item)
		}
		if _, hasLegacyID := item["id"]; hasLegacyID {
			t.Fatalf("session item[%d] must not include legacy id field: %#v", i, item)
		}
	}

	_ = h.manager.Shutdown(context.Background())
}

func TestAuthenticatedRunStreamInspectFlow(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	emit := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	h.api.EngineFactory = func(events *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
		return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
			select {
			case <-ctx.Done():
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusInterrupted, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{}, nil, ctx.Err()
			case <-emit:
			}
			tokenEvent, tokenErr := domain.TokenDelta(1, "hello world")
			stepEvent, stepErr := domain.StepComplete(1, "streamed summary")
			completeEvent, completeErr := domain.SessionComplete(task.ID, string(domain.SessionStatusSuccess))
			for _, event := range []domain.StreamingEvent{
				mustEvent(t, tokenEvent, tokenErr),
				mustEvent(t, stepEvent, stepErr),
				mustEvent(t, completeEvent, completeErr),
			} {
				if err := events.Emit(event); err != nil {
					return domain.Session{}, domain.Plan{}, nil, err
				}
			}
			select {
			case <-ctx.Done():
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusInterrupted, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{}, nil, ctx.Err()
			case <-release:
			}
			step := domain.Step{Index: 1, Action: domain.Action{Type: domain.ActionTypeRespond, Summary: "streamed summary", Response: "hello world"}, Observation: domain.Observation{Summary: "streamed summary", FinalResponse: "hello world"}, Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "streamed summary"}}
			now := time.Now().UTC()
			if err := h.sessionStore.SaveSession(ctx, domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			if err := h.sessionStore.SavePlan(ctx, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: "integration plan", CreatedAt: now}); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			if err := h.sessionStore.AppendStep(ctx, task.ID, step); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			close(finished)
			return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: "integration plan", CreatedAt: now}, []domain.Step{step}, nil
		}), nil
	}

	accepted := startSession(t, h, `{"task":"authenticated integration","task_type":"coding"}`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := newStreamingResponseWriter()
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		streamReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+accepted.SessionID+"/stream", nil).WithContext(ctx)
		streamReq.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(writer, streamReq)
	}()

	waitForBodyContains(t, writer, ": stream opened; no replay")
	close(emit)
	waitForBodyContains(t, writer, "event: session_complete")
	close(release)
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for persisted completion")
	}
	cancel()
	select {
	case <-streamDone:
	case <-time.After(2 * time.Second):
		t.Fatal("stream handler did not exit")
	}

	inspectRec := httptest.NewRecorder()
	inspectReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+accepted.SessionID+"/inspect", nil)
	inspectReq.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(inspectRec, inspectReq)
	if inspectRec.Code != http.StatusOK {
		t.Fatalf("GET /inspect status = %d, want 200, body=%s", inspectRec.Code, inspectRec.Body.String())
	}
	var payload inspectResponse
	if err := json.Unmarshal(inspectRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal inspect response: %v", err)
	}
	if payload.SessionID != accepted.SessionID {
		t.Fatalf("inspect session_id = %q, want %q", payload.SessionID, accepted.SessionID)
	}
	if payload.Status != "completed" {
		t.Fatalf("inspect status = %q, want completed", payload.Status)
	}
	if payload.Plan != "integration plan" {
		t.Fatalf("inspect plan = %q, want integration plan", payload.Plan)
	}
	if len(payload.Steps) != 1 || payload.Steps[0].Observation != "streamed summary" {
		t.Fatalf("inspect steps = %#v, want one streamed summary step", payload.Steps)
	}
	body := writer.BodyString()
	for _, marker := range []string{"event: token_delta", "event: step_complete", "event: session_complete"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("stream body missing %q: %s", marker, body)
		}
	}
}

func TestResumeReturns404WhenSessionMissing(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resume", bytes.NewBufferString(`{"session_id":"missing"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST /api/v1/resume status = %d, want 404", rec.Code)
	}
}

func TestResumeReturns409WhenSessionAlreadyRunning(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	const sessionID = "session-running"
	block := make(chan struct{})
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	if err := h.sessionStore.SaveSession(context.Background(), domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusRunning, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := h.sessionStore.SaveTask(context.Background(), sessionID, domain.Task{ID: sessionID, Description: "task", Goal: "task", CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	_, err := h.manager.Start(context.Background(), runtime.SessionStartRequest{SessionID: sessionID, Task: domain.Task{ID: sessionID, Description: "task", Goal: "task", CreatedAt: now}, NewRunner: func(*runtime.EventChannel) (runtime.SessionRunner, error) { return blockingRunner{done: block}, nil }})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resume", bytes.NewBufferString(`{"session_id":"session-running"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /api/v1/resume status = %d, want 409", rec.Code)
	}
	close(block)
	_ = h.manager.Shutdown(context.Background())
}

func TestResumeAcceptedForInterruptedSession(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	const sessionID = "session-interrupted"
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	if err := h.sessionStore.SaveSession(context.Background(), domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusInterrupted, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := h.sessionStore.SaveTask(context.Background(), sessionID, domain.Task{ID: sessionID, Description: "resume me", Goal: "resume me", Category: domain.TaskCategoryCoding, CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	h.api.EngineFactory = func(_ *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
		if task.ID != sessionID {
			t.Fatalf("task.ID = %q, want %q", task.ID, sessionID)
		}
		if maxSteps != 0 {
			t.Fatalf("maxSteps = %d, want 0", maxSteps)
		}
		if verifyMode != "" {
			t.Fatalf("verifyMode = %q, want empty", verifyMode)
		}
		return fakeRunner{}, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/resume", bytes.NewBufferString(`{"session_id":"session-interrupted"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/v1/resume status = %d, want 202", rec.Code)
	}
	var payload sessionAcceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Status != "running" {
		t.Fatalf("status = %q, want running", payload.Status)
	}
	_ = h.manager.Shutdown(context.Background())
}

func TestInspectReturnsPersistedState(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	const sessionID = "session-inspect"
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	if err := h.sessionStore.SaveSession(context.Background(), domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now.Add(time.Minute)}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := h.sessionStore.SaveTask(context.Background(), sessionID, domain.Task{ID: sessionID, Description: "inspect repository", Goal: "inspect repository", Category: domain.TaskCategoryCoding, CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := h.sessionStore.SavePlan(context.Background(), domain.Plan{ID: "plan-1", TaskID: sessionID, Summary: "inspect plan", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	if err := h.sessionStore.AppendStep(context.Background(), sessionID, domain.Step{Index: 1, Action: domain.Action{Type: domain.ActionTypeToolCall, Summary: "search", ToolCall: &domain.ToolCall{Name: "code_search", Input: `{"query":"main"}`}}, Observation: domain.Observation{Summary: "Found 3 matches"}, Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: "ok"}}); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-inspect/inspect", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /inspect status = %d, want 200", rec.Code)
	}
	var payload inspectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.SessionID != sessionID {
		t.Fatalf("session_id = %q, want %q", payload.SessionID, sessionID)
	}
	if payload.Task != "inspect repository" {
		t.Fatalf("task = %q, want inspect repository", payload.Task)
	}
	if payload.Status != "completed" {
		t.Fatalf("status = %q, want completed", payload.Status)
	}
	if len(payload.Steps) != 1 || payload.Steps[0].ToolName != "code_search" {
		t.Fatalf("steps = %#v, want one code_search step", payload.Steps)
	}
	if payload.Steps[0].ToolInput["query"] != "main" {
		t.Fatalf("tool input query = %#v, want main", payload.Steps[0].ToolInput["query"])
	}
}

func TestInspectExposesDiagnosticCorrelationAndFailureReason(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	const sessionID = "session-diagnostics"
	now := time.Date(2026, 5, 6, 11, 0, 0, 0, time.UTC)
	if err := h.sessionStore.SaveSession(context.Background(), domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusFatalError, CreatedAt: now, UpdatedAt: now.Add(time.Minute)}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := h.sessionStore.SaveTask(context.Background(), sessionID, domain.Task{ID: sessionID, Description: "broken task", Goal: "broken task", Category: domain.TaskCategoryCoding, CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := h.sessionStore.SavePlan(context.Background(), domain.Plan{ID: "plan-" + sessionID, TaskID: sessionID, Summary: "recover stream", CreatedAt: now}); err != nil {
		t.Fatalf("SavePlan() error = %v", err)
	}
	step := domain.Step{
		Index: 1,
		Observation: domain.Observation{
			Summary:       "stream disconnected upstream",
			FinalResponse: "stream disconnected upstream",
		},
		Verification: domain.VerificationResult{
			Reason: "stream disconnected upstream",
		},
	}
	if err := h.sessionStore.AppendStep(context.Background(), sessionID, step); err != nil {
		t.Fatalf("AppendStep() error = %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sessionID+"/inspect", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /inspect status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload inspectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload.Plan != "recover stream" {
		t.Fatalf("plan = %q, want recover stream", payload.Plan)
	}
	if payload.Termination != "stream disconnected upstream" {
		t.Fatalf("termination_reason = %q, want stream disconnected upstream", payload.Termination)
	}
}

func TestListSessionsReturnsPaginatedSummaries(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	base := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	seedSessionForListTest(t, h, "session-created", domain.SessionStatusPending, "created task", domain.TaskCategoryCoding, base)
	seedSessionForListTest(t, h, "session-running", domain.SessionStatusRunning, "running task", domain.TaskCategoryResearch, base.Add(time.Minute))
	seedSessionForListTest(t, h, "session-blocked", domain.SessionStatusBlockedInput, "blocked task", domain.TaskCategoryGeneral, base.Add(2*time.Minute))
	seedSessionForListTest(t, h, "session-completed", domain.SessionStatusSuccess, "completed task", domain.TaskCategoryFileWorkflow, base.Add(3*time.Minute))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page=1&page_size=2", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /sessions status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload listSessionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if payload.Page != 1 || payload.PageSize != 2 || payload.Total != 4 {
		t.Fatalf("pagination = %#v, want page=1 page_size=2 total=4", payload)
	}
	if len(payload.Sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(payload.Sessions))
	}
	if payload.Sessions[0].SessionID != "session-completed" || payload.Sessions[1].SessionID != "session-blocked" {
		t.Fatalf("sessions order = %#v, want completed then blocked", payload.Sessions)
	}
	if payload.Sessions[0].Status != "completed" || payload.Sessions[0].Task != "completed task" || payload.Sessions[0].TaskType != "file_workflow" {
		t.Fatalf("first session = %#v, want completed summary", payload.Sessions[0])
	}

	filteredRec := httptest.NewRecorder()
	filteredReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?status=running&page=1&page_size=20", nil)
	filteredReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(filteredRec, filteredReq)
	if filteredRec.Code != http.StatusOK {
		t.Fatalf("GET /sessions?status=running status = %d, want 200, body=%s", filteredRec.Code, filteredRec.Body.String())
	}
	var filtered listSessionsResponse
	if err := json.Unmarshal(filteredRec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("unmarshal filtered response: %v", err)
	}
	if filtered.Total != 2 || len(filtered.Sessions) != 2 {
		t.Fatalf("filtered payload = %#v, want two running sessions", filtered)
	}
	if filtered.Sessions[0].SessionID != "session-blocked" || filtered.Sessions[1].SessionID != "session-running" {
		t.Fatalf("filtered sessions = %#v, want blocked then running", filtered.Sessions)
	}
	for _, session := range filtered.Sessions {
		if session.Status != "running" {
			t.Fatalf("filtered status = %q, want running", session.Status)
		}
	}

	maxRec := httptest.NewRecorder()
	maxReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions?page_size=999", nil)
	maxReq.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(maxRec, maxReq)
	if maxRec.Code != http.StatusOK {
		t.Fatalf("GET /sessions?page_size=999 status = %d, want 200", maxRec.Code)
	}
	var maxPayload listSessionsResponse
	if err := json.Unmarshal(maxRec.Body.Bytes(), &maxPayload); err != nil {
		t.Fatalf("unmarshal max response: %v", err)
	}
	if maxPayload.PageSize != 100 {
		t.Fatalf("page_size = %d, want 100", maxPayload.PageSize)
	}
}

func TestListSessionsReturnsEmptyResults(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /sessions status = %d, want 200", rec.Code)
	}
	var payload listSessionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal empty response: %v", err)
	}
	if payload.Total != 0 || payload.Page != 1 || payload.PageSize != 20 {
		t.Fatalf("empty payload = %#v, want defaults with zero total", payload)
	}
	if len(payload.Sessions) != 0 {
		t.Fatalf("sessions = %#v, want empty", payload.Sessions)
	}
}

func TestListSessionsRejectsInvalidQueryParams(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	for _, path := range []string{"/api/v1/sessions?page=0", "/api/v1/sessions?page_size=-1", "/api/v1/sessions?status=weird"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d, want 400, body=%s", path, rec.Code, rec.Body.String())
		}
	}
}

func TestListSessionsRequiresAuthentication(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /sessions status = %d, want 401", rec.Code)
	}
}

func TestInspectReturns404WhenMissing(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/missing/inspect", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /inspect status = %d, want 404", rec.Code)
	}
}

func TestInspectReturns401WithoutAuthentication(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/inspect", nil)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /inspect status = %d, want 401", rec.Code)
	}
}

func TestStreamRequiresAuthentication(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-1/stream", nil)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET /stream status = %d, want 401", rec.Code)
	}
}

func TestStreamEmitsOrderedSSEFrames(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	release := make(chan struct{})
	emitted := make(chan struct{})
	actor, err := h.manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-stream",
		Task:      domain.Task{ID: "session-stream", Description: "demo stream", Goal: "demo stream", CreatedAt: time.Now().UTC()},
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-release
				tokenEvent, tokenErr := domain.TokenDelta(1, "hello")
				toolStartEvent, toolStartErr := domain.ToolStart(1, "grep", "pattern")
				toolEndEvent, toolEndErr := domain.ToolEnd(1, "grep", "done", "")
				stepCompleteEvent, stepCompleteErr := domain.StepComplete(1, "step finished")
				sessionCompleteEvent, sessionCompleteErr := domain.SessionComplete(task.ID, "success")
				defer close(emitted)
				for _, event := range []domain.StreamingEvent{
					mustEvent(t, tokenEvent, tokenErr),
					mustEvent(t, toolStartEvent, toolStartErr),
					mustEvent(t, toolEndEvent, toolEndErr),
					mustEvent(t, stepCompleteEvent, stepCompleteErr),
					mustEvent(t, sessionCompleteEvent, sessionCompleteErr),
				} {
					if err := events.Emit(event); err != nil {
						return domain.Session{}, domain.Plan{}, nil, err
					}
				}
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer actor.Cancel(context.Canceled)
	ctx, cancel := context.WithCancel(context.Background())
	writer := newStreamingResponseWriter()
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-stream/stream", nil).WithContext(ctx)
		streamReq.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(writer, streamReq)
	}()
	waitForBodyContains(t, writer, ": stream opened; no replay")
	close(release)

	select {
	case <-emitted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for emitted events")
	}
	waitForBodyContains(t, writer, "event: session_complete")
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream handler did not exit after cancel")
	}

	body := writer.BodyString()
	ordered := []string{"event: token_delta", "event: tool_start", "event: tool_end", "event: step_complete", "event: session_complete"}
	prev := -1
	for _, marker := range ordered {
		idx := strings.Index(body, marker)
		if idx < 0 {
			t.Fatalf("missing %q in stream body: %s", marker, body)
		}
		if idx <= prev {
			t.Fatalf("events out of order in stream body: %s", body)
		}
		prev = idx
	}
	if got := writer.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", got)
	}
}

func TestStreamDisconnectDoesNotCancelSession(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	runnerStarted := make(chan struct{})
	allowFinish := make(chan struct{})
	h.api.EngineFactory = func(events *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
		return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
			tokenEvent, tokenErr := domain.TokenDelta(1, "hello")
			token := mustEvent(t, tokenEvent, tokenErr)
			for _, event := range []domain.StreamingEvent{token} {
				if err := events.Emit(event); err != nil {
					return domain.Session{}, domain.Plan{}, nil, err
				}
			}
			close(runnerStarted)
			select {
			case <-ctx.Done():
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusInterrupted, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{}, nil, ctx.Err()
			case <-allowFinish:
				sessionCompleteEvent, sessionCompleteErr := domain.SessionComplete(task.ID, "success")
				sessionComplete := mustEvent(t, sessionCompleteEvent, sessionCompleteErr)
				for _, event := range []domain.StreamingEvent{sessionComplete} {
					_ = events.Emit(event)
				}
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{}, nil, nil
			}
		}), nil
	}

	accepted := startSession(t, h, `{"task":"disconnect safety"}`)
	ctx, cancel := context.WithCancel(context.Background())
	writer := newStreamingResponseWriter()
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+accepted.SessionID+"/stream", nil).WithContext(ctx)
		streamReq.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(writer, streamReq)
	}()

	select {
	case <-runnerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream handler did not exit after disconnect")
	}
	if h.manager.ActiveCount() != 1 {
		t.Fatalf("active count after disconnect = %d, want 1", h.manager.ActiveCount())
	}
	close(allowFinish)
	waitForActiveCount(t, h.manager, 0)
}

func TestStreamTerminatedSessionReturnsNoSSEBody(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	now := time.Now().UTC()
	if err := h.sessionStore.SaveSession(context.Background(), domain.Session{ID: "session-closed", TaskID: "session-closed", Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	if err := h.sessionStore.SaveTask(context.Background(), "session-closed", domain.Task{ID: "session-closed", Description: "closed", Goal: "closed", CreatedAt: now}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	writer := newStreamingResponseWriter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-closed/stream", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(writer, req)

	if writer.status != 0 {
		t.Fatalf("GET /stream status = %d, want no SSE response for terminated session", writer.status)
	}
	if body := writer.BodyString(); body != "" {
		t.Fatalf("terminated stream body = %q, want empty", body)
	}
}

func TestStreamWriteFailureStopsHandlerWithoutPartialEventFrame(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	release := make(chan struct{})
	actor, err := h.manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-stream-write-failure",
		Task:      domain.Task{ID: "session-stream-write-failure", Description: "writer fail", Goal: "writer fail", CreatedAt: time.Now().UTC()},
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-release
				event, buildErr := domain.TokenDelta(1, "hello")
				if buildErr != nil {
					return domain.Session{}, domain.Plan{}, nil, buildErr
				}
				if emitErr := events.Emit(*event); emitErr != nil {
					return domain.Session{}, domain.Plan{}, nil, emitErr
				}
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer actor.Cancel(context.Canceled)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	writer := newFailingStreamingResponseWriter(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-stream-write-failure/stream", nil).WithContext(ctx)
		streamReq.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(writer, streamReq)
	}()

	waitForBodyContains(t, writer.streamingResponseWriter, ": stream opened; no replay")
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream handler did not exit after writer failure")
	}
	if body := writer.BodyString(); strings.Contains(body, "event: token_delta") {
		t.Fatalf("stream body contains partial event header after write failure: %s", body)
	}
}

func TestStreamSkipsMalformedEventsAndContinuesToTerminalEvent(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	release := make(chan struct{})
	actor, err := h.manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: "session-stream-malformed",
		Task:      domain.Task{ID: "session-stream-malformed", Description: "malformed", Goal: "malformed", CreatedAt: time.Now().UTC()},
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
				<-release
				bad := domain.StreamingEvent{Type: domain.EventTokenDelta, StepIndex: 1, Payload: domain.EventPayload([]byte("{")), Timestamp: time.Now().UTC()}
				complete, completeErr := domain.SessionComplete(task.ID, "success")
				for _, event := range []domain.StreamingEvent{bad, mustEvent(t, complete, completeErr)} {
					if emitErr := events.Emit(event); emitErr != nil {
						return domain.Session{}, domain.Plan{}, nil, emitErr
					}
				}
				return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{}, nil, nil
			}), nil
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer actor.Cancel(context.Canceled)

	ctx, cancel := context.WithCancel(context.Background())
	writer := newStreamingResponseWriter()
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamReq := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/session-stream-malformed/stream", nil).WithContext(ctx)
		streamReq.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(writer, streamReq)
	}()
	waitForBodyContains(t, writer, ": stream opened; no replay")
	close(release)
	waitForBodyContains(t, writer, "event: session_complete")
	cancel()
	<-done

	body := writer.BodyString()
	if strings.Contains(body, "event: token_delta") {
		t.Fatalf("malformed event should not be written: %s", body)
	}
	if !strings.Contains(body, "event: session_complete") {
		t.Fatalf("session complete missing after malformed event: %s", body)
	}
}

func TestStreamReturnsStructuredNotFoundEnvelopeForMissingSession(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/missing-session/stream", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/sessions/{id}/stream status = %d, want 404 body=%s", rec.Code, rec.Body.String())
	}
	var payload errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal stream error payload: %v", err)
	}
	if payload.Error.Code != "not_found" || !strings.Contains(payload.Error.Message, "missing-session") {
		t.Fatalf("stream error payload = %#v, want structured not_found envelope naming missing session", payload)
	}
}

func TestRunReturns429WhenActiveSessionCapExceeded(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	block := make(chan struct{})
	h.api.RetryAfterSecs = 7
	h.api.EngineFactory = func(_ *runtime.EventChannel, _ domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		return blockingRunner{done: block}, nil
	}

	first := startSession(t, h, `{"task":"first"}`)
	if first.SessionID == "" {
		t.Fatal("first session_id empty")
	}
	second := startSession(t, h, `{"task":"second"}`)
	if second.SessionID == "" {
		t.Fatal("second session_id empty")
	}
	waitForActiveCountAtLeast(t, h.manager, 2)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(`{"task":"third"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("POST /run status = %d, want 429, body=%s", rec.Code, rec.Body.String())
	}
	if retryAfter := rec.Header().Get("Retry-After"); retryAfter != "7" {
		t.Fatalf("Retry-After = %q, want 7", retryAfter)
	}

	var payload errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error envelope: %v", err)
	}
	if payload.Error.Code != "too_many_requests" {
		t.Fatalf("error code = %q, want too_many_requests", payload.Error.Code)
	}
	close(block)
	waitForActiveCount(t, h.manager, 0)
}

func TestConcurrentSessionsRemainIsolatedAcrossInspectAndStream(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	emitA := make(chan struct{})
	emitB := make(chan struct{})
	started := make(chan string, 2)
	h.api.EngineFactory = func(events *runtime.EventChannel, task domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
			now := time.Now().UTC()
			step := domain.Step{Index: 1, Action: domain.Action{Type: domain.ActionTypeRespond, Summary: task.Description + "-summary", Response: task.Description + "-final"}, Observation: domain.Observation{Summary: task.Description + "-summary", FinalResponse: task.Description + "-final"}, Verification: domain.VerificationResult{Passed: true, Status: domain.VerificationStatusPassed, Reason: task.Description + "-summary"}}
			if err := h.sessionStore.SaveSession(ctx, domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			if err := h.sessionStore.SavePlan(ctx, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description + "-plan", CreatedAt: now}); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			if err := h.sessionStore.AppendStep(ctx, task.ID, step); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			started <- task.Description
			switch task.Description {
			case "alpha":
				<-emitA
			case "beta":
				<-emitB
			default:
				return domain.Session{}, domain.Plan{}, nil, fmt.Errorf("unexpected task %q", task.Description)
			}
			tokenEvent, tokenErr := domain.TokenDelta(1, task.Description+"-token")
			stepEvent, stepErr := domain.StepComplete(1, task.Description+"-summary")
			completeEvent, completeErr := domain.SessionComplete(task.ID, string(domain.SessionStatusSuccess))
			for _, event := range []domain.StreamingEvent{mustEvent(t, tokenEvent, tokenErr), mustEvent(t, stepEvent, stepErr), mustEvent(t, completeEvent, completeErr)} {
				if err := events.Emit(event); err != nil {
					return domain.Session{}, domain.Plan{}, nil, err
				}
			}
			switch task.Description {
			case "alpha":
				<-releaseA
			case "beta":
				<-releaseB
			default:
				return domain.Session{}, domain.Plan{}, nil, fmt.Errorf("unexpected task %q", task.Description)
			}
			return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: now, UpdatedAt: now}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description + "-plan", CreatedAt: now}, []domain.Step{step}, nil
		}), nil
	}

	alpha := startSession(t, h, `{"task":"alpha"}`)
	beta := startSession(t, h, `{"task":"beta"}`)
	for range 2 {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent session startup")
		}
	}

	alphaWriter := newStreamingResponseWriter()
	betaWriter := newStreamingResponseWriter()
	alphaCtx, cancelAlpha := context.WithCancel(context.Background())
	betaCtx, cancelBeta := context.WithCancel(context.Background())
	defer cancelAlpha()
	defer cancelBeta()
	alphaDone := make(chan struct{})
	betaDone := make(chan struct{})
	go func() {
		defer close(alphaDone)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+alpha.SessionID+"/stream", nil).WithContext(alphaCtx)
		req.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(alphaWriter, req)
	}()
	go func() {
		defer close(betaDone)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+beta.SessionID+"/stream", nil).WithContext(betaCtx)
		req.Header.Set("Authorization", "Bearer "+h.jwt)
		h.router.ServeHTTP(betaWriter, req)
	}()

	time.Sleep(50 * time.Millisecond)
	close(emitA)
	close(emitB)
	waitForBodyContains(t, alphaWriter, "alpha-token")
	waitForBodyContains(t, betaWriter, "beta-token")
	close(releaseA)
	close(releaseB)
	cancelAlpha()
	cancelBeta()
	<-alphaDone
	<-betaDone

	assertInspectSessionSummary(t, h, alpha.SessionID, "alpha-plan", "alpha-summary")
	assertInspectSessionSummary(t, h, beta.SessionID, "beta-plan", "beta-summary")
	if strings.Contains(alphaWriter.BodyString(), "beta-token") {
		t.Fatalf("alpha stream leaked beta events: %s", alphaWriter.BodyString())
	}
	if strings.Contains(betaWriter.BodyString(), "alpha-token") {
		t.Fatalf("beta stream leaked alpha events: %s", betaWriter.BodyString())
	}
}

func TestShutdownRejectsNewRequestsAndCancelsInflightSessions(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarnessWithOptions(t, testAPIHarnessOptions{managerOptions: runtime.SessionManagerOptions{ActiveSessionCap: 2, ShutdownTimeout: 50 * time.Millisecond}})
	runnerStarted := make(chan struct{}, 1)
	h.api.EngineFactory = func(_ *runtime.EventChannel, task domain.Task, _ int, _ string) (runtime.SessionRunner, error) {
		return runnerFunc(func(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
			runnerStarted <- struct{}{}
			<-ctx.Done()
			now := time.Now().UTC()
			if err := h.sessionStore.SaveSession(context.Background(), domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusInterrupted, CreatedAt: now, UpdatedAt: now}); err != nil {
				return domain.Session{}, domain.Plan{}, nil, err
			}
			return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusInterrupted, CreatedAt: now, UpdatedAt: now}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: "cancelled by shutdown", CreatedAt: now}, nil, ctx.Err()
		}), nil
	}

	accepted := startSession(t, h, `{"task":"shutdown me"}`)
	select {
	case <-runnerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not start")
	}

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- h.manager.Shutdown(context.Background())
	}()
	waitForManagerClosed(t, h.manager)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(`{"task":"rejected during shutdown"}`))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST /run during shutdown status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	var payload errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal shutdown rejection: %v", err)
	}
	if payload.Error.Code != "conflict" || payload.Error.Message != "session manager is shutting down" {
		t.Fatalf("shutdown rejection payload = %#v, want conflict session manager is shutting down", payload)
	}

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("shutdown error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not complete")
	}
	waitForActiveCount(t, h.manager, 0)
	inspected, err := h.sessionStore.InspectSession(context.Background(), accepted.SessionID)
	if err != nil {
		t.Fatalf("InspectSession() after shutdown error = %v", err)
	}
	if inspected.Session.Status != domain.SessionStatusInterrupted {
		t.Fatalf("persisted shutdown status = %q, want %q", inspected.Session.Status, domain.SessionStatusInterrupted)
	}
	if h.manager.IsAccepting() {
		t.Fatal("manager should reject new requests after shutdown")
	}
}

func TestMapStartErrorSessionManagerClosedReturnsConflict(t *testing.T) {
	t.Parallel()
	a := &API{}
	err := a.mapStartError(runtime.ErrSessionManagerClosed)
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("mapStartError error type = %T, want *apiError", err)
	}
	if apiErr.status != http.StatusConflict {
		t.Fatalf("mapStartError status = %d, want 409", apiErr.status)
	}
	if apiErr.code != "conflict" {
		t.Fatalf("mapStartError code = %q, want conflict", apiErr.code)
	}
	if apiErr.message != "session manager is shutting down" {
		t.Fatalf("mapStartError message = %q, want session manager is shutting down", apiErr.message)
	}
}

func TestRecovererReturns500OnPanic(t *testing.T) {
	t.Parallel()
	h := newTestAPIHarness(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(h.api.Recoverer)
	router.Get("/panic", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET /panic status = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
	var payload errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if payload.Error.Code != "internal_error" || !strings.Contains(payload.Error.Message, "internal server error") {
		t.Fatalf("error payload = %#v, want internal server error message", payload)
	}
}

type testAPIHarness struct {
	router       http.Handler
	api          *API
	sessionStore *store.SQLiteSessionStore
	memoryStore  *store.SQLiteMemoryStore
	manager      *runtime.SessionManager
	jwt          string
}

func newTestAPIHarness(t *testing.T) testAPIHarness {
	t.Helper()
	return newTestAPIHarnessWithOptions(t, testAPIHarnessOptions{})
}

type testAPIHarnessOptions struct {
	managerOptions runtime.SessionManagerOptions
	webUIEnabled   bool
}

func newTestAPIHarnessWithOptions(t *testing.T, opts testAPIHarnessOptions) testAPIHarness {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "server.db")
	var clockMu sync.Mutex
	clockBase := time.Date(2026, 5, 6, 10, 30, 0, 0, time.UTC)
	clockTick := 0
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
	managerOptions := opts.managerOptions
	if managerOptions.ActiveSessionCap == 0 {
		managerOptions.ActiveSessionCap = 2
	}
	if managerOptions.ShutdownTimeout == 0 {
		managerOptions.ShutdownTimeout = time.Second
	}
	manager := runtime.NewSessionManager(managerOptions)
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	api := &API{SessionStore: sessionStore, MemoryStore: memoryStore, Manager: manager, JWTSecret: "test-secret", Clock: func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		current := clockBase.Add(time.Duration(clockTick) * time.Nanosecond)
		clockTick++
		return current
	}, Config: configForHarness(opts.webUIEnabled)}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(api.Recoverer)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(api.AuthMiddleware)
		r.Post("/run", api.JSON(api.HandleRun))
		r.Post("/resume", api.JSON(api.HandleResume))
		r.Post("/chat/start", api.JSON(api.HandleChatStart))
		r.Post("/chat/{conversation_id}/reply", api.JSON(api.HandleChatReply))
		r.Get("/chat/{conversation_id}/transcript", api.JSON(api.HandleChatTranscript))
		r.Get("/chat/conversations", api.JSON(api.HandleChatList))
		r.Get("/sessions", api.JSON(api.HandleListSessions))
		r.Get("/sessions/{id}/inspect", api.JSON(api.HandleInspect))
		r.Get("/sessions/{id}/stream", api.JSON(api.HandleStream))
	})
	RegisterWebRoutesWithFS(r, api, testWebMountFS)
	return testAPIHarness{router: r, api: api, sessionStore: sessionStore, memoryStore: memoryStore, manager: manager, jwt: makeJWT(t, "test-secret", time.Date(2026, 5, 6, 11, 0, 0, 0, time.UTC))}
}

func configForHarness(webUIEnabled bool) config.Config {
	cfg := config.Default()
	cfg.Server.WebUIEnabled = webUIEnabled
	cfg.Server.WebUI.Enabled = webUIEnabled
	cfg.Server.WebUI.IndexPath = "testdata/web/index.html"
	return cfg
}

type fakeRunner struct{}

func (fakeRunner) Run(_ context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description, CreatedAt: time.Now().UTC()}, nil, nil
}

type runnerFunc func(context.Context, domain.Task) (domain.Session, domain.Plan, []domain.Step, error)

func (f runnerFunc) Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	return f(ctx, task)
}

type blockingRunner struct{ done <-chan struct{} }

func (r blockingRunner) Run(ctx context.Context, task domain.Task) (domain.Session, domain.Plan, []domain.Step, error) {
	select {
	case <-ctx.Done():
		return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusInterrupted, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description, CreatedAt: time.Now().UTC()}, nil, ctx.Err()
	case <-r.done:
		return domain.Session{ID: task.ID, TaskID: task.ID, Status: domain.SessionStatusSuccess, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}, domain.Plan{ID: "plan-" + task.ID, TaskID: task.ID, Summary: task.Description, CreatedAt: time.Now().UTC()}, nil, nil
	}
}

func makeJWT(t *testing.T, secret string, exp time.Time) string {
	t.Helper()
	headerJSON, err := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payloadJSON, err := json.Marshal(map[string]any{"sub": "tester", "iat": exp.Add(-time.Hour).Unix(), "exp": exp.Unix()})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	header := base64.RawURLEncoding.EncodeToString(headerJSON)
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", signingInput, signature)
}


func startSession(t *testing.T, h testAPIHarness, body string) sessionAcceptedResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/run", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /run status = %d, want 202, body=%s", rec.Code, rec.Body.String())
	}
	var accepted sessionAcceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &accepted); err != nil {
		t.Fatalf("unmarshal accepted response: %v", err)
	}
	return accepted
}

func seedSessionForListTest(t *testing.T, h testAPIHarness, sessionID string, status domain.SessionStatus, task string, category domain.TaskCategory, createdAt time.Time) {
	t.Helper()
	updatedAt := createdAt.Add(30 * time.Second)
	if err := h.sessionStore.SaveSession(context.Background(), domain.Session{ID: sessionID, TaskID: sessionID, Status: status, CreatedAt: createdAt, UpdatedAt: updatedAt}); err != nil {
		t.Fatalf("SaveSession(%s) error = %v", sessionID, err)
	}
	if err := h.sessionStore.SaveTask(context.Background(), sessionID, domain.Task{ID: sessionID, Description: task, Goal: task, Category: category, CreatedAt: createdAt}); err != nil {
		t.Fatalf("SaveTask(%s) error = %v", sessionID, err)
	}
}

func mustEvent(t *testing.T, event *domain.StreamingEvent, err error) domain.StreamingEvent {
	t.Helper()
	if err != nil {
		t.Fatalf("build event: %v", err)
	}
	return *event
}

func waitForBodyContains(t *testing.T, writer *streamingResponseWriter, needle string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(writer.BodyString(), needle) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in stream body: %s", needle, writer.BodyString())
}

func waitForActiveCount(t *testing.T, manager *runtime.SessionManager, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if manager.ActiveCount() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("active count = %d, want %d", manager.ActiveCount(), want)
}

func waitForActiveCountAtLeast(t *testing.T, manager *runtime.SessionManager, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if manager.ActiveCount() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("active count = %d, want at least %d", manager.ActiveCount(), want)
}

func waitForManagerClosed(t *testing.T, manager *runtime.SessionManager) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !manager.IsAccepting() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("manager did not stop accepting new sessions")
}

func assertInspectSessionSummary(t *testing.T, h testAPIHarness, sessionID, wantPlan, wantSummary string) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sessionID+"/inspect", nil)
	req.Header.Set("Authorization", "Bearer "+h.jwt)
	h.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /inspect status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload inspectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal inspect payload: %v", err)
	}
	if payload.SessionID != sessionID {
		t.Fatalf("inspect session id = %q, want %q", payload.SessionID, sessionID)
	}
	if payload.Plan != wantPlan {
		t.Fatalf("inspect plan = %q, want %q", payload.Plan, wantPlan)
	}
	if len(payload.Steps) != 1 || payload.Steps[0].Observation != wantSummary {
		t.Fatalf("inspect steps = %#v, want one step summary %q", payload.Steps, wantSummary)
	}
}

type streamingResponseWriter struct {
	mu     sync.Mutex
	header http.Header
	body   bytes.Buffer
	status int
	flushes int
}

type failingStreamingResponseWriter struct {
	*streamingResponseWriter
	maxWrites int
	writes    int
}

type delayedStreamingResponseWriter struct {
	*streamingResponseWriter
	delay time.Duration
}

func newStreamingResponseWriter() *streamingResponseWriter {
	return &streamingResponseWriter{header: make(http.Header)}
}

func newFailingStreamingResponseWriter(maxWrites int) *failingStreamingResponseWriter {
	return &failingStreamingResponseWriter{streamingResponseWriter: newStreamingResponseWriter(), maxWrites: maxWrites}
}

func newDelayedStreamingResponseWriter(delay time.Duration) *delayedStreamingResponseWriter {
	return &delayedStreamingResponseWriter{streamingResponseWriter: newStreamingResponseWriter(), delay: delay}
}

func (w *streamingResponseWriter) Header() http.Header {
	return w.header
}

func (w *streamingResponseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *streamingResponseWriter) WriteHeader(statusCode int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = statusCode
}

func (w *streamingResponseWriter) Flush() {
	w.mu.Lock()
	w.flushes++
	w.mu.Unlock()
}

func (w *streamingResponseWriter) BodyString() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}

func (w *streamingResponseWriter) FlushCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushes
}

func (w *failingStreamingResponseWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.maxWrites >= 0 && w.writes >= w.maxWrites {
		return 0, io.ErrClosedPipe
	}
	w.writes++
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

func (w *delayedStreamingResponseWriter) Write(data []byte) (int, error) {
	if w.delay > 0 {
		time.Sleep(w.delay)
	}
	return w.streamingResponseWriter.Write(data)
}

var _ http.Flusher = (*streamingResponseWriter)(nil)
var _ http.ResponseWriter = (*streamingResponseWriter)(nil)
var _ io.Writer = (*streamingResponseWriter)(nil)
var _ http.Flusher = (*failingStreamingResponseWriter)(nil)
var _ http.ResponseWriter = (*failingStreamingResponseWriter)(nil)
var _ http.Flusher = (*delayedStreamingResponseWriter)(nil)
var _ http.ResponseWriter = (*delayedStreamingResponseWriter)(nil)
