package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
	"zheng-harness/internal/service"
	"zheng-harness/internal/store"
)

type EngineFactory func(events *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error)

type API struct {
	SessionStore   *store.SQLiteSessionStore
	MemoryStore    *store.SQLiteMemoryStore
	Manager        *runtime.SessionManager
	Builder        *runtimebuilder.Builder
	Service        *service.ChatService
	Config         config.Config
	JWTSecret      string
	Clock          func() time.Time
	EngineFactory  EngineFactory
	RetryAfterSecs int
}

type HandlerFunc func(http.ResponseWriter, *http.Request) error

type runRequest struct {
	Task       string `json:"task"`
	TaskType   string `json:"task_type"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	MaxSteps   int    `json:"max_steps"`
	VerifyMode string `json:"verify_mode"`
}

type resumeRequest struct {
	SessionID string `json:"session_id"`
}

type chatStartRequest struct {
	Message    string `json:"message"`
	TaskType   string `json:"task_type"`
	MaxSteps   int    `json:"max_steps"`
	VerifyMode string `json:"verify_mode"`
}

type chatReplyRequest struct {
	Message string `json:"message"`
}

type sessionAcceptedResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	StreamURL string `json:"stream_url"`
}

type chatStartResponse struct {
	ConversationID string `json:"conversation_id"`
	SessionID      string `json:"session_id"`
	Status         string `json:"status"`
	StreamURL      string `json:"stream_url"`
}

type chatReplyResponse struct {
	ConversationID string `json:"conversation_id"`
	TurnIndex      int    `json:"turn_index"`
	StreamURL      string `json:"stream_url"`
}

type chatTranscriptResponse struct {
	ConversationID string              `json:"conversation_id"`
	Status         string              `json:"status"`
	Messages       []transcriptMessage `json:"messages"`
}

type transcriptMessage struct {
	TurnIndex int    `json:"turn_index"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type inspectResponse struct {
	SessionID   string        `json:"session_id"`
	Status      string        `json:"status"`
	Task        string        `json:"task"`
	TaskType    string        `json:"task_type,omitempty"`
	RequestID   string        `json:"request_id,omitempty"`
	FailureReason string      `json:"failure_reason,omitempty"`
	Finalized   bool          `json:"finalized,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	Steps       []inspectStep `json:"steps"`
	Provenance  *provenance   `json:"provenance,omitempty"`
	Plan        string        `json:"plan,omitempty"`
	Termination string        `json:"termination_reason,omitempty"`
}

type listSessionsResponse struct {
	Sessions []listSessionItem `json:"sessions"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	Total    int               `json:"total"`
}

type listSessionItem struct {
	SessionID string    `json:"session_id"`
	Status    string    `json:"status"`
	Task      string    `json:"task"`
	TaskType  string    `json:"task_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type inspectStep struct {
	StepNumber  int            `json:"step_number"`
	ToolName    string         `json:"tool_name,omitempty"`
	ToolInput   map[string]any `json:"tool_input,omitempty"`
	Observation string         `json:"observation,omitempty"`
	Status      string         `json:"status"`
}

type provenance struct {
	ProviderPlugin      *domain.PluginMetadata `json:"provider_plugin"`
	VerifierPlugin      *domain.PluginMetadata `json:"verifier_plugin"`
	AgentStrategyPlugin *domain.PluginMetadata `json:"agent_strategy_plugin"`
}

type errorEnvelope struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type sseWriter struct {
	w http.ResponseWriter
	fl http.Flusher
}

var errMalformedStreamEvent = errors.New("malformed stream event")

type apiError struct {
	status  int
	code    string
	message string
	err     error
}

func (e *apiError) Error() string {
	if e == nil {
		return ""
	}
	if e.message != "" {
		return e.message
	}
	if e.err != nil {
		return e.err.Error()
	}
	return http.StatusText(e.status)
}

func (e *apiError) Unwrap() error { return e.err }

func (a *API) JSON(next HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := next(w, r); err != nil {
			a.writeError(w, r, err)
		}
	}
}

func (a *API) Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.writeStructuredError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *API) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			a.writeStructuredError(w, r, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		if err := validateJWT(token, a.JWTSecret, a.now()); err != nil {
			a.writeStructuredError(w, r, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) HandleRun(w http.ResponseWriter, r *http.Request) error {
	var req runRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	result, err := a.chatService().StartConversation(r.Context(), req.Task, service.StartConversationOptions{
		TaskType:   req.TaskType,
		MaxSteps:   req.MaxSteps,
		VerifyMode: req.VerifyMode,
	})
	if err != nil {
		return a.mapServiceError(err, "start session")
	}

	writeJSON(w, http.StatusAccepted, sessionAcceptedResponse{SessionID: result.SessionID, Status: result.Status, StreamURL: result.StreamURL})
	return nil
}

func (a *API) HandleResume(w http.ResponseWriter, r *http.Request) error {
	var req resumeRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	result, err := a.chatService().ResumeConversation(r.Context(), req.SessionID)
	if err != nil {
		return a.mapServiceError(err, "resume session")
	}

	writeJSON(w, http.StatusAccepted, sessionAcceptedResponse{SessionID: result.SessionID, Status: result.Status, StreamURL: result.StreamURL})
	return nil
}

func (a *API) HandleChatStart(w http.ResponseWriter, r *http.Request) error {
	var req chatStartRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	result, err := a.chatService().StartChat(r.Context(), service.StartChatRequest{
		Message:    req.Message,
		TaskType:   req.TaskType,
		MaxSteps:   req.MaxSteps,
		VerifyMode: req.VerifyMode,
	})
	if err != nil {
		return a.mapServiceError(err, "start chat")
	}
	writeJSON(w, http.StatusAccepted, chatStartResponse{
		ConversationID: result.ConversationID,
		SessionID:      result.SessionID,
		Status:         result.Status,
		StreamURL:      result.StreamURL,
	})
	return nil
}

func (a *API) HandleChatReply(w http.ResponseWriter, r *http.Request) error {
	conversationID := strings.TrimSpace(chi.URLParam(r, "conversation_id"))
	if conversationID == "" {
		return &apiError{status: http.StatusBadRequest, code: "invalid_request", message: "conversation id is required"}
	}
	var req chatReplyRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	result, err := a.chatService().SubmitReply(r.Context(), conversationID, req.Message)
	if err != nil {
		return a.mapServiceError(err, "reply to chat")
	}
	writeJSON(w, http.StatusAccepted, chatReplyResponse{
		ConversationID: result.ConversationID,
		TurnIndex:      result.TurnIndex,
		StreamURL:      result.StreamURL,
	})
	return nil
}

func (a *API) HandleChatTranscript(w http.ResponseWriter, r *http.Request) error {
	conversationID := strings.TrimSpace(chi.URLParam(r, "conversation_id"))
	if conversationID == "" {
		return &apiError{status: http.StatusBadRequest, code: "invalid_request", message: "conversation id is required"}
	}
	transcript, err := a.chatService().GetTranscript(r.Context(), conversationID)
	if err != nil {
		return a.mapServiceError(err, "load transcript")
	}
	writeJSON(w, http.StatusOK, chatTranscriptResponse{
		ConversationID: transcript.Chat.ConversationID,
		Status:         string(transcript.Chat.Status),
		Messages:       toTranscriptMessages(transcript.Chat.Messages),
	})
	return nil
}

func (a *API) HandleChatList(w http.ResponseWriter, r *http.Request) error {
	page, err := parsePositiveIntQuery(r, "page", 1)
	if err != nil {
		return err
	}
	pageSize, err := parsePositiveIntQuery(r, "page_size", 20)
	if err != nil {
		return err
	}
	if pageSize > 100 {
		pageSize = 100
	}
	status, err := parseListStatusFilter(r.URL.Query().Get("status"))
	if err != nil {
		return err
	}
	result, err := a.chatService().ListChats(r.Context(), service.ChatListParams{Page: page, PageSize: pageSize, Status: status})
	if err != nil {
		return &apiError{status: http.StatusInternalServerError, code: "internal_error", message: "list conversations", err: err}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"conversations": toConversationListItems(result.Conversations),
		"page":          result.Page,
		"page_size":     result.PageSize,
		"total":         result.Total,
	})
	return nil
}

func (a *API) HandleInspect(w http.ResponseWriter, r *http.Request) error {
	sessionID := strings.TrimSpace(chi.URLParam(r, "id"))
	if sessionID == "" {
		return &apiError{status: http.StatusBadRequest, code: "invalid_request", message: "session id is required"}
	}
	transcript, err := a.chatService().GetTranscript(r.Context(), sessionID)
	if err != nil {
		return a.mapServiceError(err, "load session state")
	}

	writeJSON(w, http.StatusOK, inspectResponse{
		SessionID:   transcript.Inspect.Session.ID,
		Status:      apiSessionStatus(transcript.Inspect.Session.Status),
		Task:        transcript.Inspect.Task.Description,
		TaskType:    string(transcript.Inspect.Task.CategoryOrDefault()),
		CreatedAt:   transcript.Inspect.Session.CreatedAt.UTC(),
		UpdatedAt:   transcript.Inspect.Session.UpdatedAt.UTC(),
		Steps:       toInspectSteps(transcript.Inspect.Steps),
		Provenance:  toInspectProvenance(transcript.Inspect.Session.Provenance),
		Plan:        transcript.Inspect.Plan.Summary,
		Termination: deriveTerminationReason(transcript.Inspect.Session, transcript.Inspect.Steps),
	})
	return nil
}

func (a *API) HandleListSessions(w http.ResponseWriter, r *http.Request) error {
	page, err := parsePositiveIntQuery(r, "page", 1)
	if err != nil {
		return err
	}
	pageSize, err := parsePositiveIntQuery(r, "page_size", 20)
	if err != nil {
		return err
	}
	if pageSize > 100 {
		pageSize = 100
	}
	status, err := parseListStatusFilter(r.URL.Query().Get("status"))
	if err != nil {
		return err
	}

	result, err := a.chatService().ListConversations(r.Context(), service.ListConversationsParams{Page: page, PageSize: pageSize, Status: status})
	if err != nil {
		return &apiError{status: http.StatusInternalServerError, code: "internal_error", message: "list sessions", err: err}
	}

	writeJSON(w, http.StatusOK, listSessionsResponse{
		Sessions: toConversationListItems(result.Conversations),
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	})
	return nil
}

func (a *API) HandleStream(w http.ResponseWriter, r *http.Request) error {
	sessionID := strings.TrimSpace(chi.URLParam(r, "id"))
	actor, err := a.chatService().ValidateStreamSession(r.Context(), sessionID)
	if err != nil {
		return a.mapServiceError(err, "load session state")
	}
	if actor == nil {
		return nil
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return &apiError{status: http.StatusInternalServerError, code: "internal_error", message: "streaming not supported by response writer"}
	}
	writer := sseWriter{w: w, fl: flusher}
	writer.writeHeaders()
	if err := writer.writeComment("stream opened; no replay"); err != nil {
		return nil
	}

	sub := actor.Subscribe(256)
	if sub == nil {
		return &apiError{status: http.StatusInternalServerError, code: "internal_error", message: "stream relay is not available"}
	}
	defer sub.Close()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return nil
		case <-heartbeat.C:
			if err := writer.writeComment("heartbeat"); err != nil {
				return nil
			}
		case event, ok := <-sub.Events():
			if !ok {
				if overflow, has := sub.OverflowEvent(); has {
					_ = writer.writeEvent(*overflow)
				}
				return nil
			}
			if err := writer.writeEvent(event); err != nil {
				if errors.Is(err, errMalformedStreamEvent) {
					continue
				}
				return nil
			}
		}
	}
}

func (a *API) newEngine(events *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
	if a.EngineFactory != nil {
		return a.EngineFactory(events, task, maxSteps, verifyMode)
	}
	if a.Builder == nil {
		return nil, errors.New("server runtime builder is not initialized")
	}
	executor, err := a.Builder.NewExecutor(runtimebuilder.ExecutorOptions{})
	if err != nil {
		return nil, err
	}
	model := a.Builder.NewModel()
	if model == nil {
		return nil, errors.New("server runtime model is not configured")
	}
	cfg := a.Config
	if strings.TrimSpace(verifyMode) != "" {
		cfg.Runtime.VerifyMode = verifyMode
	}
	if maxSteps <= 0 {
		maxSteps = a.Builder.DefaultMaxSteps()
	}
	return a.Builder.BuildEngine(runtimebuilder.EngineOptions{
		Model:         model,
		Tools:         executor,
		Memory:        a.MemoryStore,
		Sessions:      runtimebuilder.NewSessionAliasStore(a.SessionStore, context.Background(), task.ID),
		Verifier:      runtimebuilder.NewVerifierFromConfig(cfg, executor),
		MaxSteps:      maxSteps,
		EventChannel:  events,
		PersistentCtx: context.Background(),
	}), nil
}

func (a *API) mapStartError(err error) error {
	switch {
	case errors.Is(err, runtime.ErrSessionAlreadyActive):
		return &apiError{status: http.StatusConflict, code: "conflict", message: "session already running", err: err}
	case errors.Is(err, runtime.ErrActiveSessionLimit):
		return &apiError{status: http.StatusTooManyRequests, code: "too_many_requests", message: "active session cap exceeded", err: err}
	case errors.Is(err, runtime.ErrSessionManagerClosed):
		return &apiError{status: http.StatusConflict, code: "conflict", message: "session manager is shutting down", err: err}
	default:
		return &apiError{status: http.StatusInternalServerError, code: "internal_error", message: "start session", err: err}
	}
}

func (a *API) mapServiceError(err error, defaultMessage string) error {
	if validationErr, ok := errors.AsType[*service.ValidationError](err); ok {
		return &apiError{status: http.StatusBadRequest, code: "invalid_request", message: validationErr.Error(), err: err}
	}
	if conflictErr, ok := errors.AsType[*service.ConflictError](err); ok {
		code := "conflict"
		if strings.Contains(conflictErr.Error(), "session cannot be resumed:") {
			code = "provenance_mismatch"
		}
		return &apiError{status: http.StatusConflict, code: code, message: conflictErr.Error(), err: err}
	}
if notFoundErr, ok := errors.AsType[*service.NotFoundError](err); ok {
		msg := notFoundErr.Error()
		if notFoundErr.ID != "" {
			msg = fmt.Sprintf("%s %q not found", notFoundErr.Kind, notFoundErr.ID)
		}
		return &apiError{status: http.StatusNotFound, code: "not_found", message: msg, err: err}
	}
	switch {
	case errors.Is(err, runtime.ErrSessionAlreadyActive), errors.Is(err, runtime.ErrActiveSessionLimit), errors.Is(err, runtime.ErrSessionManagerClosed):
		return a.mapStartError(err)
	default:
		return &apiError{status: http.StatusInternalServerError, code: "internal_error", message: defaultMessage, err: err}
	}
}

func (a *API) mapStoreError(err error, sessionID string) error {
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
		return &apiError{status: http.StatusNotFound, code: "not_found", message: fmt.Sprintf("session %q not found", sessionID), err: err}
	}
	return &apiError{status: http.StatusInternalServerError, code: "internal_error", message: "load session state", err: err}
}

func (a *API) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *apiError
	if !errors.As(err, &apiErr) {
		apiErr = &apiError{status: http.StatusInternalServerError, code: "internal_error", message: "internal server error", err: err}
	}
	if apiErr.status == http.StatusTooManyRequests {
		retryAfter := a.RetryAfterSecs
		if retryAfter <= 0 {
			retryAfter = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	}
	a.writeStructuredError(w, r, apiErr.status, apiErr.code, apiErr.Error())
}

func (a *API) writeStructuredError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	requestID := middleware.GetReqID(r.Context())
	writeJSON(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message}, RequestID: requestID})
}

const maxRequestBodySize = 32 * 1024 // 32KB

func decodeJSON(r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxRequestBodySize)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return &apiError{status: http.StatusRequestEntityTooLarge, code: "request_too_large", message: fmt.Sprintf("request body must be less than %d bytes", maxRequestBodySize)}
		}
		return &apiError{status: http.StatusBadRequest, code: "invalid_request", message: "invalid JSON body", err: err}
	}
	if decoder.More() {
		return &apiError{status: http.StatusBadRequest, code: "invalid_request", message: "request body must contain a single JSON object"}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (w sseWriter) writeHeaders() {
	headers := w.w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	w.w.WriteHeader(http.StatusOK)
	w.fl.Flush()
}

func (w sseWriter) writeComment(comment string) error {
	if _, err := io.WriteString(w.w, ": "+strings.TrimSpace(comment)+"\n\n"); err != nil {
		return err
	}
	w.fl.Flush()
	return nil
}

func (w sseWriter) writeEvent(event domain.StreamingEvent) error {
	if !isSupportedStreamEventType(event.Type) {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("%w: %v", errMalformedStreamEvent, err)
	}
	if _, err := io.WriteString(w.w, "event: "+string(event.Type)+"\n"); err != nil {
		return err
	}
	if _, err := io.WriteString(w.w, "data: "+string(data)+"\n\n"); err != nil {
		return err
	}
	w.fl.Flush()
	return nil
}

func WriteJSONForServer(w http.ResponseWriter, status int, payload any) {
	writeJSON(w, status, payload)
}

func bearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", errors.New("missing bearer token")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("invalid bearer token")
	}
	return strings.TrimSpace(parts[1]), nil
}

func validateJWT(token, secret string, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid bearer token")
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signingInput))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return errors.New("invalid bearer token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("invalid bearer token")
	}
	var headerClaims map[string]any
	if err := json.Unmarshal(headerBytes, &headerClaims); err != nil {
		return errors.New("invalid bearer token")
	}
	if alg, _ := headerClaims["alg"].(string); alg != "HS256" {
		return errors.New("invalid bearer token")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("invalid bearer token")
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return errors.New("invalid bearer token")
	}
	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return errors.New("invalid bearer token")
	}
	iat, ok := numericClaim(claims, "iat")
	if !ok {
		return errors.New("invalid bearer token")
	}
	exp, ok := numericClaim(claims, "exp")
	if !ok {
		return errors.New("invalid bearer token")
	}
	issuedAt := time.Unix(iat, 0)
	expiresAt := time.Unix(exp, 0)
	if !expiresAt.After(now) {
		return errors.New("expired bearer token")
	}
	if expiresAt.Sub(issuedAt) > 24*time.Hour {
		return errors.New("invalid bearer token")
	}
	return nil
}

func numericClaim(claims map[string]any, key string) (int64, bool) {
	value, ok := claims[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case int64:
		return typed, true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func streamURL(sessionID string) string {
	return "/api/v1/sessions/" + sessionID + "/stream"
}

func isSupportedStreamEventType(value domain.StreamingEventType) bool {
	switch value {
	case domain.EventTokenDelta, domain.EventToolStart, domain.EventToolEnd, domain.EventStepComplete, domain.EventError, domain.EventSessionComplete:
		return true
	default:
		return false
	}
}

func isSupportedVerifyMode(value string) bool {
	switch strings.TrimSpace(value) {
	case "", config.VerifyModeOff, config.VerifyModeStandard, config.VerifyModeStrict:
		return true
	default:
		return false
	}
}

func parsePositiveIntQuery(r *http.Request, key string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, &apiError{status: http.StatusBadRequest, code: "invalid_request", message: key + " must be a positive integer"}
	}
	return value, nil
}

func parseListStatusFilter(raw string) (string, error) {
	status := strings.TrimSpace(raw)
	if status == "" {
		return "", nil
	}
	if _, ok := listStatusFilterMap()[status]; ok {
		return status, nil
	}
	return status, &apiError{status: http.StatusBadRequest, code: "invalid_request", message: "status must be one of created, running, completed, cancelled, failed"}
}

func isTerminalStatus(status domain.SessionStatus) bool {
	switch status {
	case domain.SessionStatusSuccess, domain.SessionStatusVerificationFailed, domain.SessionStatusBudgetExceeded, domain.SessionStatusFatalError:
		return true
	default:
		return false
	}
}

func apiSessionStatus(status domain.SessionStatus) string {
	switch status {
	case domain.SessionStatusPending:
		return "created"
	case domain.SessionStatusRunning, domain.SessionStatusBlockedInput:
		return "running"
	case domain.SessionStatusSuccess:
		return "completed"
	case domain.SessionStatusInterrupted:
		return "cancelled"
	default:
		return "failed"
	}
}

func toListSessionItems(summaries []store.SessionSummary) []listSessionItem {
	if len(summaries) == 0 {
		return []listSessionItem{}
	}
	items := make([]listSessionItem, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, listSessionItem{
			SessionID: summary.SessionID,
			Status:    apiSessionStatus(domain.SessionStatus(summary.Status)),
			Task:      summary.Task,
			TaskType:  summary.TaskType,
			CreatedAt: summary.CreatedAt.UTC(),
			UpdatedAt: summary.UpdatedAt.UTC(),
		})
	}
	return items
}

func toConversationListItems(summaries []service.ConversationSummary) []listSessionItem {
	if len(summaries) == 0 {
		return []listSessionItem{}
	}
	items := make([]listSessionItem, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, listSessionItem{
			SessionID: summary.SessionID,
			Status:    summary.Status,
			Task:      summary.Task,
			TaskType:  summary.TaskType,
			CreatedAt: summary.CreatedAt.UTC(),
			UpdatedAt: summary.UpdatedAt.UTC(),
		})
	}
	return items
}

func toTranscriptMessages(messages []domain.ChatMessage) []transcriptMessage {
	if len(messages) == 0 {
		return []transcriptMessage{}
	}
	items := make([]transcriptMessage, 0, len(messages))
	for _, message := range messages {
		items = append(items, transcriptMessage{
			TurnIndex: message.TurnIndex,
			Role:      string(message.Role),
			Content:   message.Content,
			Timestamp: message.Timestamp.UTC().Format(time.RFC3339Nano),
		})
	}
	return items
}

func listStatusFilterMap() map[string][]string {
	return map[string][]string{
		"created":   {string(domain.SessionStatusPending)},
		"running":   {string(domain.SessionStatusRunning), string(domain.SessionStatusBlockedInput)},
		"completed": {string(domain.SessionStatusSuccess)},
		"cancelled": {string(domain.SessionStatusInterrupted)},
		"failed":    {string(domain.SessionStatusVerificationFailed), string(domain.SessionStatusBudgetExceeded), string(domain.SessionStatusFatalError)},
	}
}

func toInspectSteps(steps []domain.Step) []inspectStep {
	if len(steps) == 0 {
		return []inspectStep{}
	}
	result := make([]inspectStep, 0, len(steps))
	for _, step := range steps {
		item := inspectStep{StepNumber: step.Index, Observation: strings.TrimSpace(step.Observation.Summary), Status: inspectStepStatus(step)}
		if step.Action.ToolCall != nil {
			item.ToolName = step.Action.ToolCall.Name
			if input := strings.TrimSpace(step.Action.ToolCall.Input); input != "" {
				var decoded map[string]any
				if err := json.Unmarshal([]byte(input), &decoded); err == nil {
					item.ToolInput = decoded
				}
			}
		}
		result = append(result, item)
	}
	return result
}

func inspectStepStatus(step domain.Step) string {
	if step.Verification.Passed {
		return "completed"
	}
	if strings.TrimSpace(step.Verification.Reason) != "" {
		return "failed"
	}
	return "running"
}

func toInspectProvenance(p *domain.Provenance) *provenance {
	if p == nil {
		return nil
	}
	out := &provenance{}
	plugins := p.Normalize().Plugins
	for i := range plugins {
		plugin := &plugins[i]
		switch plugin.Family {
		case domain.PluginFamilyProvider:
			out.ProviderPlugin = plugin
		case domain.PluginFamilyVerifier:
			out.VerifierPlugin = plugin
		case domain.PluginFamilyAgentStrategy:
			out.AgentStrategyPlugin = plugin
		}
	}
	if out.ProviderPlugin == nil && out.VerifierPlugin == nil && out.AgentStrategyPlugin == nil {
		return &provenance{}
	}
	return out
}

// validateProvenanceForResume checks persisted plugin provenance against the API's
// current plugin configuration. If the session was created with a specific plugin
// provider, we verify it's still available. This implements the fail-closed semantics
// required by the harness engineering principles.
func (a *API) validateProvenanceForResume(p *domain.Provenance) []string {
	if p == nil || a.Builder == nil {
		return nil
	}
	var errs []string
	apiCfg := a.Config
	for _, plugin := range p.Normalize().Plugins {
		switch plugin.Family {
		case domain.PluginFamilyProvider:
			pluginID := strings.TrimSpace(plugin.LogicalID)
			if pluginID == "" {
				continue
			}
			if apiCfg.PluginProvider != "" && apiCfg.PluginProvider != pluginID {
				errs = append(errs, fmt.Sprintf("provider plugin %q was used but current configuration uses %q", pluginID, apiCfg.PluginProvider))
			}
		}
	}
	return errs
}


func deriveTerminationReason(session domain.Session, steps []domain.Step) string {
	if len(steps) > 0 {
		last := steps[len(steps)-1]
		if strings.TrimSpace(last.Verification.Reason) != "" {
			return last.Verification.Reason
		}
		if strings.TrimSpace(last.Observation.FinalResponse) != "" {
			return last.Observation.FinalResponse
		}
		if strings.TrimSpace(last.Observation.Summary) != "" {
			return last.Observation.Summary
		}
	}
	return string(session.Status)
}

func (a *API) now() time.Time {
	if a != nil && a.Clock != nil {
		return a.Clock()
	}
	return time.Now()
}

func (a *API) chatService() *service.ChatService {
	if a.Service != nil {
		return a.Service
	}
	var engineFactory service.EngineFactory
	if a.EngineFactory != nil {
		engineFactory = func(events *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
			return a.EngineFactory(events, task, maxSteps, verifyMode)
		}
	}
	a.Service = service.NewChatService(service.Dependencies{
		SessionStore:   a.SessionStore,
		MemoryStore:    a.MemoryStore,
		SessionManager: a.Manager,
		Builder:        a.Builder,
		Config:         a.Config,
		EngineFactory:  engineFactory,
		Clock:          a.Clock,
	})
	return a.Service
}
