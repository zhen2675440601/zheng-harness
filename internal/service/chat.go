package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"zheng-harness/internal/config"
	"zheng-harness/internal/domain"
	"zheng-harness/internal/runtime"
	"zheng-harness/internal/runtimebuilder"
	"zheng-harness/internal/store"
)

type StartConversationOptions struct {
	TaskType   string
	MaxSteps   int
	VerifyMode string
}

type ConversationStartResult struct {
	ConversationID string
	SessionID      string
	Status         string
	StreamURL      string
	Task           domain.Task
}

type TurnResult struct {
	ConversationID string
	SessionID      string
	Status         string
	StreamURL      string
	Submission     domain.TurnSubmission
}

type ResumeResult struct {
	ConversationID string
	SessionID      string
	Status         string
	StreamURL      string
	Task           domain.Task
}

type Transcript struct {
	Conversation domain.Conversation
	Chat         domain.ChatTranscript
	Inspect      store.InspectState
}

type ListConversationsParams struct {
	Page     int
	PageSize int
	Status   string
}

type ConversationSummary struct {
	ConversationID string
	SessionID      string
	Status         string
	Task           string
	TaskType       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type ListResult struct {
	Conversations []ConversationSummary
	Total         int
	Page          int
	PageSize      int
}

type StartChatRequest struct {
	Message    string
	TaskType   string
	MaxSteps   int
	VerifyMode string
}

type ChatResult struct {
	ConversationID string
	SessionID      string
	Status         string
	StreamURL      string
	TurnIndex      int
	Task           domain.Task
}

type ChatListParams struct {
	Page     int
	PageSize int
	Status   string
}

type ChatList struct {
	Conversations []ConversationSummary
	Total         int
	Page          int
	PageSize      int
}

func (s *ChatService) StartConversation(ctx context.Context, taskText string, opts StartConversationOptions) (*ConversationStartResult, error) {
	if s == nil {
		return nil, errors.New("chat service is nil")
	}
	taskText = strings.TrimSpace(taskText)
	if taskText == "" {
		return nil, &ValidationError{Message: "task is required"}
	}
	if opts.MaxSteps < 0 {
		return nil, &ValidationError{Message: "max_steps must be greater than or equal to zero"}
	}
	verifyMode := strings.TrimSpace(opts.VerifyMode)
	if verifyMode != "" && !isSupportedVerifyMode(verifyMode) {
		return nil, &ValidationError{Message: "verify_mode must be one of off, standard, strict"}
	}
	taskType := strings.TrimSpace(opts.TaskType)
	if taskType != "" && !isSupportedTaskType(taskType) {
		return nil, &ValidationError{Message: "task_type must be one of general, coding, research, file_workflow"}
	}

	now := s.Now().UTC()
	sessionID := fmt.Sprintf("session-%d", now.UnixNano())
	task := domain.Task{
		ID:          sessionID,
		Description: taskText,
		Goal:        taskText,
		Category:    domain.TaskCategory(strings.TrimSpace(opts.TaskType)).Normalize(),
		CreatedAt:   now,
	}.Normalize()
	initial := domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusPending, CreatedAt: now, UpdatedAt: now}

	if err := s.sessionStore.SaveSession(ctx, initial); err != nil {
		return nil, fmt.Errorf("save initial session: %w", err)
	}
	if err := s.sessionStore.SaveTask(ctx, sessionID, task); err != nil {
		return nil, fmt.Errorf("save task metadata: %w", err)
	}
	if err := s.persistSessionDiagnostics(ctx, sessionID, "", false); err != nil {
		return nil, fmt.Errorf("save session diagnostics: %w", err)
	}
	if _, err := s.manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: sessionID,
		Task:      task,
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return s.newEngine(events, task, opts.MaxSteps, verifyMode)
		},
		PersistFinal: s.persistFatalOnRunnerError(now),
	}); err != nil {
		return nil, err
	}

	return &ConversationStartResult{
		ConversationID: sessionID,
		SessionID:      sessionID,
		Status:         "created",
		StreamURL:      s.StreamURL(sessionID),
		Task:           task,
	}, nil
}

func (s *ChatService) StartChat(ctx context.Context, req StartChatRequest) (*ChatResult, error) {
	if s == nil {
		return nil, errors.New("chat service is nil")
	}
	taskText := strings.TrimSpace(req.Message)
	if taskText == "" {
		return nil, &ValidationError{Message: "task is required"}
	}
	if req.MaxSteps < 0 {
		return nil, &ValidationError{Message: "max_steps must be greater than or equal to zero"}
	}
	verifyMode := strings.TrimSpace(req.VerifyMode)
	if verifyMode != "" && !isSupportedVerifyMode(verifyMode) {
		return nil, &ValidationError{Message: "verify_mode must be one of off, standard, strict"}
	}
	taskType := strings.TrimSpace(req.TaskType)
	if taskType != "" && !isSupportedTaskType(taskType) {
		return nil, &ValidationError{Message: "task_type must be one of general, coding, research, file_workflow"}
	}

	now := s.Now().UTC()
	conversationID := fmt.Sprintf("conversation-%d", now.UnixNano())
	sessionID := fmt.Sprintf("session-%d", now.UnixNano())
	task := domain.Task{
		ID:          sessionID,
		Description: taskText,
		Goal:        taskText,
		Category:    domain.TaskCategory(strings.TrimSpace(req.TaskType)).Normalize(),
		CreatedAt:   now,
	}.Normalize()
	initial := domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusPending, CreatedAt: now, UpdatedAt: now}

	if err := s.sessionStore.SaveSession(ctx, initial); err != nil {
		return nil, fmt.Errorf("save initial session: %w", err)
	}
	if err := s.sessionStore.SaveTask(ctx, sessionID, task); err != nil {
		return nil, fmt.Errorf("save task metadata: %w", err)
	}
	if err := s.persistSessionDiagnostics(ctx, sessionID, "", false); err != nil {
		return nil, fmt.Errorf("save session diagnostics: %w", err)
	}
	if err := s.sessionStore.SaveConversationState(ctx, sessionID, conversationID, "", 0); err != nil {
		return nil, fmt.Errorf("save conversation metadata: %w", err)
	}
	if _, err := s.manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: sessionID,
		Task:      task,
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return s.newEngine(events, task, req.MaxSteps, verifyMode)
		},
		PersistFinal: s.persistFatalOnRunnerError(now),
	}); err != nil {
		return nil, err
	}

	return &ChatResult{
		ConversationID: conversationID,
		SessionID:      sessionID,
		Status:         "created",
		StreamURL:      s.StreamURL(sessionID),
		TurnIndex:      0,
		Task:           task,
	}, nil
}

func (s *ChatService) SubmitTurn(ctx context.Context, submission domain.TurnSubmission) (*TurnResult, error) {
	result, err := s.SubmitReply(ctx, firstNonEmpty(strings.TrimSpace(submission.ConversationID), strings.TrimSpace(submission.SessionID)), submission.Input.Content)
	if err != nil {
		return nil, err
	}
	if submission.SubmittedAt.IsZero() {
		submission.SubmittedAt = s.Now().UTC()
	}
	submission.ConversationID = result.ConversationID
	submission.SessionID = result.SessionID

	return &TurnResult{
		ConversationID: result.ConversationID,
		SessionID:      result.SessionID,
		Status:         result.Status,
		StreamURL:      result.StreamURL,
		Submission:     submission,
	}, nil
}

func (s *ChatService) SubmitReply(ctx context.Context, conversationIDOrSessionID, message string) (*ChatResult, error) {
	if s == nil {
		return nil, errors.New("chat service is nil")
	}
	id := strings.TrimSpace(conversationIDOrSessionID)
	if id == "" {
		return nil, &ValidationError{Message: "conversation id is required"}
	}
	content := strings.TrimSpace(message)
	if content == "" {
		return nil, &ValidationError{Message: "input content is required"}
	}

	// 首先尝试获取transcript，这会处理session_id -> conversation_id的转换
	// GetTranscript 已经在内部处理了所有回退逻辑
	transcript, err := s.GetTranscript(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取对话历史失败: %w", err)
	}
	
	conversationID := transcript.Chat.ConversationID
	if conversationID == "" {
		conversationID = id
	}

	records, err := s.sessionStore.ListConversationSessions(ctx, conversationID)
	if err != nil {
		return nil, mapStoreLookupError("conversation", conversationID, err)
	}
	if len(records) == 0 {
		return nil, &ValidationError{Message: "conversation not found: " + conversationID}
	}
	// Find the last terminal (completed/failed) session to chain from.
	// If the very latest session is still running, we chain from the previous
	// terminal one instead of rejecting — this is chat, not single-task submission.
	parentIdx := -1
	for i := len(records) - 1; i >= 0; i-- {
		if isTerminalStatus(records[i].Session.Status) {
			parentIdx = i
			break
		}
	}
	if parentIdx < 0 {
		return nil, &ConflictError{Message: "conversation is already running"}
	}
	parent := records[parentIdx]
	taskDescription := content
	if historyContext := buildConversationContext(transcript.Chat.Messages); historyContext != "" {
		taskDescription = historyContext + "\n\nCurrent user message:\n" + content
	}

	now := s.Now().UTC()
	sessionID := fmt.Sprintf("session-%d", now.UnixNano())
	task := domain.Task{
		ID:          sessionID,
		Description: taskDescription,
		Goal:        content,
		Category:    parent.Task.CategoryOrDefault().Normalize(),
		CreatedAt:   now,
	}.Normalize()
	initial := domain.Session{ID: sessionID, TaskID: sessionID, Status: domain.SessionStatusPending, CreatedAt: now, UpdatedAt: now}
	if err := s.sessionStore.SaveSession(ctx, initial); err != nil {
		return nil, fmt.Errorf("save initial session: %w", err)
	}
	if err := s.sessionStore.SaveTask(ctx, sessionID, task); err != nil {
		return nil, fmt.Errorf("save task metadata: %w", err)
	}
	if err := s.persistSessionDiagnostics(ctx, sessionID, "", false); err != nil {
		return nil, fmt.Errorf("save session diagnostics: %w", err)
	}
	if err := s.sessionStore.SaveConversationState(ctx, sessionID, conversationID, parent.Session.ID, parent.TurnIndex+1); err != nil {
		return nil, fmt.Errorf("save conversation metadata: %w", err)
	}
	if _, err := s.manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: sessionID,
		Task:      task,
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return s.newEngine(events, task, 0, "")
		},
		PersistFinal: s.persistFatalOnRunnerError(now),
	}); err != nil {
		return nil, err
	}

	return &ChatResult{
		ConversationID: conversationID,
		SessionID:      sessionID,
		Status:         "created",
		StreamURL:      s.StreamURL(sessionID),
		TurnIndex:      parent.TurnIndex + 1,
		Task:           task,
	}, nil
}

func (s *ChatService) ResumeConversation(ctx context.Context, conversationOrSessionID string) (*ResumeResult, error) {
	if s == nil {
		return nil, errors.New("chat service is nil")
	}
	if s.manager == nil {
		return nil, errors.New("chat service session manager is not initialized")
	}
	sessionID := strings.TrimSpace(conversationOrSessionID)
	if sessionID == "" {
		return nil, &ValidationError{Message: "session_id is required"}
	}
	if actor, ok := s.manager.Lookup(sessionID); ok {
		snapshot := actor.Snapshot()
		if snapshot.State == runtime.ActorStatePending || snapshot.State == runtime.ActorStateRunning {
			return nil, &ConflictError{Message: "session already running"}
		}
	}

	inspected, err := s.sessionStore.InspectSession(ctx, sessionID)
	if err != nil {
		return nil, mapStoreLookupError("session", sessionID, err)
	}
	if isTerminalStatus(inspected.Session.Status) {
		return nil, &ConflictError{Message: "session is not resumable"}
	}
	if inspErrors := s.validateProvenanceForResume(inspected.Session.Provenance); len(inspErrors) > 0 {
		return nil, &ConflictError{Message: "session cannot be resumed: " + strings.Join(inspErrors, "; ")}
	}

	continuedTask := inspected.Task
	if continuedTask.ID == "" {
		continuedTask.ID = sessionID
	}
	if continuedTask.CreatedAt.IsZero() {
		continuedTask.CreatedAt = inspected.Session.CreatedAt
	}
	if _, err := s.manager.Start(context.Background(), runtime.SessionStartRequest{
		SessionID: sessionID,
		Task:      continuedTask,
		NewRunner: func(events *runtime.EventChannel) (runtime.SessionRunner, error) {
			return s.newEngine(events, continuedTask, 0, "")
		},
		PersistFinal: s.persistFatalOnRunnerError(inspected.Session.CreatedAt),
	}); err != nil {
		return nil, err
	}

	return &ResumeResult{
		ConversationID: sessionID,
		SessionID:      sessionID,
		Status:         "running",
		StreamURL:      s.StreamURL(sessionID),
		Task:           continuedTask,
	}, nil
}

func (s *ChatService) GetTranscript(ctx context.Context, conversationID string) (*Transcript, error) {
	if s == nil {
		return nil, errors.New("chat service is nil")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, &ValidationError{Message: "conversation id is required"}
	}
	records, err := s.sessionStore.ListConversationSessions(ctx, conversationID)
	if err != nil {
		// 数据库查询失败，尝试直接查询session
		inspected, inspectErr := s.sessionStore.InspectSession(ctx, conversationID)
		if inspectErr != nil {
			return nil, mapStoreLookupError("conversation", conversationID, err)
		}
		conversation := buildConversation(inspected)
		chat := domain.BuildChatTranscript(conversation)
		return &Transcript{Conversation: conversation, Chat: chat, Inspect: inspected}, nil
	}
	
	// 如果没有找到记录，尝试用InspectSession作为回退
	if len(records) == 0 {
		inspected, inspectErr := s.sessionStore.InspectSession(ctx, conversationID)
		if inspectErr == nil {
			conversation := buildConversation(inspected)
			chat := domain.BuildChatTranscript(conversation)
			return &Transcript{Conversation: conversation, Chat: chat, Inspect: inspected}, nil
		}
		// 如果session不存在，返回not found
		return nil, &ValidationError{Message: "conversation not found: " + conversationID}
	}
	
	conversation, inspected, err := s.buildTranscriptFromRecords(ctx, conversationID, records)
	if err != nil {
		return nil, err
	}
	chat := domain.BuildChatTranscript(conversation)
	return &Transcript{Conversation: conversation, Chat: chat, Inspect: inspected}, nil
}

func (s *ChatService) ListConversations(ctx context.Context, params ListConversationsParams) (*ListResult, error) {
	if s == nil {
		return nil, errors.New("chat service is nil")
	}
	result, err := s.sessionStore.ListSessions(ctx, store.ListSessionsParams{Page: params.Page, PageSize: params.PageSize, Status: params.Status})
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	summaries := make([]ConversationSummary, 0, len(result.Sessions))
	for _, item := range result.Sessions {
		summaries = append(summaries, ConversationSummary{
			ConversationID: item.SessionID,
			SessionID:      item.SessionID,
			Status:         apiSessionStatus(domain.SessionStatus(item.Status)),
			Task:           item.Task,
			TaskType:       item.TaskType,
			CreatedAt:      item.CreatedAt.UTC(),
			UpdatedAt:      item.UpdatedAt.UTC(),
		})
	}
	return &ListResult{Conversations: summaries, Total: result.Total, Page: result.Page, PageSize: result.PageSize}, nil
}

func (s *ChatService) ListChats(ctx context.Context, params ChatListParams) (*ChatList, error) {
	if s == nil {
		return nil, errors.New("chat service is nil")
	}
	result, err := s.sessionStore.ListSessions(ctx, store.ListSessionsParams{Page: params.Page, PageSize: params.PageSize, Status: params.Status})
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	if len(result.Sessions) == 0 {
		return &ChatList{Conversations: []ConversationSummary{}, Total: 0, Page: result.Page, PageSize: result.PageSize}, nil
	}
	byConversation := make(map[string]ConversationSummary, len(result.Sessions))
	countByConversation := make(map[string]struct{}, len(result.Sessions))
	for _, item := range result.Sessions {
		conversationID := firstNonEmpty(item.ConversationID, item.SessionID)
		countByConversation[conversationID] = struct{}{}
		summary := ConversationSummary{
			ConversationID: conversationID,
			SessionID:      item.SessionID,
			Status:         apiSessionStatus(domain.SessionStatus(item.Status)),
			Task:           item.Task,
			TaskType:       item.TaskType,
			CreatedAt:      item.CreatedAt.UTC(),
			UpdatedAt:      item.UpdatedAt.UTC(),
		}
		if existing, ok := byConversation[conversationID]; !ok || summary.CreatedAt.After(existing.CreatedAt) || (summary.CreatedAt.Equal(existing.CreatedAt) && summary.SessionID > existing.SessionID) {
			byConversation[conversationID] = summary
		}
	}
	summaries := make([]ConversationSummary, 0, len(byConversation))
	for _, item := range byConversation {
		summaries = append(summaries, item)
	}
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].CreatedAt.Equal(summaries[j].CreatedAt) {
			return summaries[i].ConversationID > summaries[j].ConversationID
		}
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})
	return &ChatList{Conversations: summaries, Total: len(countByConversation), Page: result.Page, PageSize: result.PageSize}, nil
}

func (s *ChatService) buildTranscriptFromRecords(ctx context.Context, conversationID string, records []store.ConversationSessionRecord) (domain.Conversation, store.InspectState, error) {
	orderedMessages := make([]domain.ChatMessage, 0, len(records)*4)
	var latestInspect store.InspectState
	var latestRecord store.ConversationSessionRecord
	for idx, record := range records {
		inspected, err := s.sessionStore.InspectSession(ctx, record.Session.ID)
		if err != nil {
			return domain.Conversation{}, store.InspectState{}, mapStoreLookupError("session", record.Session.ID, err)
		}
		latestInspect = inspected
		latestRecord = record
		segment := buildConversation(inspected)
		for _, msg := range segment.Messages {
			msg.TurnIndex += record.TurnIndex
			msg.ID = fmt.Sprintf("%s-%d", msg.ID, idx)
			orderedMessages = append(orderedMessages, msg)
		}
	}
	conversation := domain.Conversation{
		ID:        conversationID,
		SessionID: latestRecord.Session.ID,
		Status:    conversationStatusForSession(latestRecord.Session.Status),
		Messages:  orderedMessages,
		Checkpoint: domain.ConversationCheckpoint{
			LastTurnIndex: latestRecord.TurnIndex,
			Resumable:     !isTerminalStatus(latestRecord.Session.Status),
		},
		CreatedAt: records[0].Session.CreatedAt.UTC(),
		UpdatedAt: latestRecord.Session.UpdatedAt.UTC(),
	}
	if len(orderedMessages) > 0 {
		last := orderedMessages[len(orderedMessages)-1]
		conversation.Checkpoint.LastMessageID = last.ID
		conversation.Checkpoint.LastEventTime = last.Timestamp
	}
	return conversation, latestInspect, nil
}

func (s *ChatService) newEngine(events *runtime.EventChannel, task domain.Task, maxSteps int, verifyMode string) (runtime.SessionRunner, error) {
	if s.engineFactory != nil {
		return s.engineFactory(events, task, maxSteps, verifyMode)
	}
	if s.builder == nil {
		return nil, errors.New("server runtime builder is not initialized")
	}
	executor, err := s.builder.NewExecutor(runtimebuilder.ExecutorOptions{})
	if err != nil {
		return nil, err
	}
	model := s.builder.NewModel()
	if model == nil {
		return nil, errors.New("server runtime model is not configured")
	}
	cfg := s.config
	if strings.TrimSpace(verifyMode) != "" {
		cfg.Runtime.VerifyMode = verifyMode
	}
	if maxSteps <= 0 {
		maxSteps = s.builder.DefaultMaxSteps()
	}
	return s.builder.BuildEngine(runtimebuilder.EngineOptions{
		Model:         model,
		Tools:         executor,
		Memory:        s.memoryStore,
		Sessions:      runtimebuilder.NewSessionAliasStore(s.sessionStore, context.Background(), task.ID),
		Verifier:      runtimebuilder.NewVerifierFromConfig(cfg, executor),
		MaxSteps:      maxSteps,
		EventChannel:  events,
		PersistentCtx: context.Background(),
	}), nil
}

func (s *ChatService) persistFatalOnRunnerError(createdAt time.Time) runtime.SessionActorFinalizer {
	return func(ctx context.Context, result runtime.SessionActorResult) error {
		failureReason := ""
		if result.Err != nil {
			failureReason = strings.TrimSpace(result.Err.Error())
		}
		_ = s.sessionStore.SaveDiagnostics(ctx, result.SessionID, store.SessionDiagnostics{FailureReason: failureReason, Finalized: true})
		if result.Err != nil && result.Session.Status == "" {
			failed := domain.Session{
				ID:        result.SessionID,
				TaskID:    result.SessionID,
				Status:    domain.SessionStatusFatalError,
				CreatedAt: createdAt,
				UpdatedAt: s.Now().UTC(),
			}
			_ = s.sessionStore.SaveSession(ctx, failed)
		}
		return nil
	}
}

func (s *ChatService) persistSessionDiagnostics(ctx context.Context, sessionID, failureReason string, finalized bool) error {
	if s == nil || s.sessionStore == nil {
		return nil
	}
	requestID := strings.TrimSpace(middleware.GetReqID(ctx))
	if requestID == "" && strings.TrimSpace(failureReason) == "" && !finalized {
		return nil
	}
	return s.sessionStore.SaveDiagnostics(ctx, sessionID, store.SessionDiagnostics{
		RequestID:     requestID,
		FailureReason: strings.TrimSpace(failureReason),
		Finalized:     finalized,
	})
}

func (s *ChatService) validateProvenanceForResume(p *domain.Provenance) []string {
	if p == nil || s.builder == nil {
		return nil
	}
	var errs []string
	apiCfg := s.config
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

func buildConversation(inspected store.InspectState) domain.Conversation {
	messages := make([]domain.ChatMessage, 0, len(inspected.Steps)*2+1)
	if description := strings.TrimSpace(firstNonEmpty(inspected.Task.Goal, inspected.Task.Description)); description != "" {
		messages = append(messages, domain.ChatMessage{
			ID:        inspected.Session.ID + "-user-0",
			TurnIndex: 0,
			Role:      domain.ChatRoleUser,
			Content:   description,
			Timestamp: inspected.Session.CreatedAt.UTC(),
		})
	}
	for _, step := range inspected.Steps {
		baseTime := inspected.Session.UpdatedAt.UTC()
		if baseTime.IsZero() {
			baseTime = inspected.Session.CreatedAt.UTC()
		}
		if summary := strings.TrimSpace(step.Action.Summary); summary != "" {
			messages = append(messages, domain.ChatMessage{
				ID:        fmt.Sprintf("%s-system-%d", inspected.Session.ID, step.Index),
				TurnIndex: step.Index,
				Role:      domain.ChatRoleSystem,
				Content:   summary,
				Timestamp: baseTime,
			})
		}
		if toolName := toolNameForStep(step); toolName != "" {
			messages = append(messages, domain.ChatMessage{
				ID:        fmt.Sprintf("%s-tool-%d", inspected.Session.ID, step.Index),
				TurnIndex: step.Index,
				Role:      domain.ChatRoleTool,
				Content:   toolName,
				Timestamp: baseTime.Add(time.Nanosecond),
			})
		}
		assistant := firstNonEmpty(strings.TrimSpace(step.Observation.FinalResponse), strings.TrimSpace(step.Observation.Summary), strings.TrimSpace(step.Verification.Reason))
		if assistant != "" {
			messages = append(messages, domain.ChatMessage{
				ID:        fmt.Sprintf("%s-assistant-%d", inspected.Session.ID, step.Index),
				TurnIndex: step.Index,
				Role:      domain.ChatRoleAssistant,
				Content:   assistant,
				Timestamp: baseTime.Add(2 * time.Nanosecond),
			})
		}
	}
	conversation := domain.Conversation{
		ID:        inspected.Session.ID,
		SessionID: inspected.Session.ID,
		Status:    conversationStatusForSession(inspected.Session.Status),
		Messages:  messages,
		Checkpoint: domain.ConversationCheckpoint{
			LastTurnIndex: len(inspected.Steps),
			Resumable:     !isTerminalStatus(inspected.Session.Status),
		},
		CreatedAt: inspected.Session.CreatedAt.UTC(),
		UpdatedAt: inspected.Session.UpdatedAt.UTC(),
	}
	if len(messages) > 0 {
		last := messages[len(messages)-1]
		conversation.Checkpoint.LastMessageID = last.ID
		conversation.Checkpoint.LastEventTime = last.Timestamp
	}
	return conversation
}

func conversationStatusForSession(status domain.SessionStatus) domain.ConversationStatus {
	switch status {
	case domain.SessionStatusPending, domain.SessionStatusRunning, domain.SessionStatusBlockedInput:
		return domain.ConversationStatusActive
	case domain.SessionStatusInterrupted:
		return domain.ConversationStatusBlocked
	case domain.SessionStatusSuccess:
		return domain.ConversationStatusCompleted
	default:
		return domain.ConversationStatusFailed
	}
}

func buildConversationContext(messages []domain.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("Conversation history:\n")
	wroteMessage := false
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		role := ""
		switch message.Role {
		case domain.ChatRoleUser:
			role = "User"
		case domain.ChatRoleAssistant:
			role = "Assistant"
		default:
			continue
		}
		if wroteMessage {
			builder.WriteString("\n")
		}
		builder.WriteString(role)
		builder.WriteString(":\n")
		builder.WriteString(content)
		builder.WriteString("\n")
		wroteMessage = true
	}
	if !wroteMessage {
		return ""
	}
	return strings.TrimSpace(builder.String())
}

func isSupportedVerifyMode(value string) bool {
	switch strings.TrimSpace(value) {
	case "", config.VerifyModeOff, config.VerifyModeStandard, config.VerifyModeStrict:
		return true
	default:
		return false
	}
}

func isSupportedTaskType(value string) bool {
	switch strings.TrimSpace(value) {
	case "general", "coding", "research", "file_workflow":
		return true
	default:
		return false
	}
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

func mapStoreLookupError(kind, id string, err error) error {
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "not found") {
		return &NotFoundError{Kind: kind, ID: id, Err: err}
	}
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func toolNameForStep(step domain.Step) string {
	if step.Action.ToolCall != nil {
		return strings.TrimSpace(step.Action.ToolCall.Name)
	}
	if step.Observation.ToolResult != nil {
		return strings.TrimSpace(step.Observation.ToolResult.ToolName)
	}
	return ""
}
