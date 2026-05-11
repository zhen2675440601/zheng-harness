package domain

import (
	"sort"
	"time"
)

// ChatRole describes the speaker role of a chat message.
type ChatRole string

const (
	ChatRoleUser      ChatRole = "user"
	ChatRoleAssistant ChatRole = "assistant"
	ChatRoleSystem    ChatRole = "system"
	ChatRoleTool      ChatRole = "tool"
)

// ConversationStatus describes the lifecycle status of a conversation.
type ConversationStatus string

const (
	ConversationStatusActive    ConversationStatus = "active"
	ConversationStatusBlocked   ConversationStatus = "blocked"
	ConversationStatusCompleted ConversationStatus = "completed"
	ConversationStatusFailed    ConversationStatus = "failed"
	ConversationStatusArchived  ConversationStatus = "archived"
)

// ChatMessage is the domain primitive for one chat utterance.
type ChatMessage struct {
	ID        string
	TurnIndex int
	Role      ChatRole
	Content   string
	Timestamp time.Time
	Metadata  map[string]string
}

// Conversation tracks a message-centric interaction bound to one session.
type Conversation struct {
	ID               string
	SessionID        string
	Status           ConversationStatus
	Messages         []ChatMessage
	Checkpoint       ConversationCheckpoint
	CreatedAt        time.Time
	UpdatedAt        time.Time
	LastResumedAt    *time.Time
	LastSubmissionAt *time.Time
}

// ConversationCheckpoint captures resumable stream boundaries.
type ConversationCheckpoint struct {
	LastMessageID string
	LastTurnIndex int
	LastEventTime time.Time
	Resumable     bool
}

// TurnSubmission is the user-facing payload for submitting a new turn.
type TurnSubmission struct {
	ConversationID string
	SessionID      string
	Input          UserMessagePayload
	Resume         ResumeCursor
	SubmittedAt    time.Time
}

// UserMessagePayload is the user message body of a turn submission.
type UserMessagePayload struct {
	Content  string
	Metadata map[string]string
}

// ResumeCursor represents client resume intent and replay boundary.
type ResumeCursor struct {
	AfterMessageID  string
	AfterTurnIndex  int
	ReplayFromStart bool
}

// ChatStreamEvent wraps streaming domain events with conversation context.
type ChatStreamEvent struct {
	ConversationID string
	SessionID      string
	TurnIndex      int
	Sequence       int
	Resume         ResumeCursor
	Event          StreamingEvent
}

// TranscriptTurn is an inspectable view for one complete turn.
type TranscriptTurn struct {
	TurnIndex          int
	UserMessage        *ChatMessage
	AssistantMessages  []ChatMessage
	SystemMessages     []ChatMessage
	ToolMessages       []ChatMessage
	FirstMessageAt     time.Time
	LastMessageAt      time.Time
	AssistantCompleted bool
}

// ChatTranscript is the inspectable history of a conversation.
type ChatTranscript struct {
	ConversationID string
	SessionID      string
	Status         ConversationStatus
	Messages       []ChatMessage
	Turns          []TranscriptTurn
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// BelongsToSession reports whether this conversation is linked to sessionID.
func (c Conversation) BelongsToSession(sessionID string) bool {
	return c.SessionID != "" && c.SessionID == sessionID
}

// MessagesInOrder returns a timestamp-first deterministic message ordering.
func (c Conversation) MessagesInOrder() []ChatMessage {
	ordered := append([]ChatMessage(nil), c.Messages...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Timestamp.Equal(ordered[j].Timestamp) {
			if ordered[i].TurnIndex == ordered[j].TurnIndex {
				return ordered[i].ID < ordered[j].ID
			}
			return ordered[i].TurnIndex < ordered[j].TurnIndex
		}
		return ordered[i].Timestamp.Before(ordered[j].Timestamp)
	})
	return ordered
}

// BuildChatTranscript assembles an inspectable transcript from a conversation.
func BuildChatTranscript(c Conversation) ChatTranscript {
	ordered := c.MessagesInOrder()
	turnMap := make(map[int]*TranscriptTurn)
	turnOrder := make([]int, 0, len(ordered))

	for _, message := range ordered {
		turn, exists := turnMap[message.TurnIndex]
		if !exists {
			turn = &TranscriptTurn{TurnIndex: message.TurnIndex}
			turnMap[message.TurnIndex] = turn
			turnOrder = append(turnOrder, message.TurnIndex)
		}

		if turn.FirstMessageAt.IsZero() || message.Timestamp.Before(turn.FirstMessageAt) {
			turn.FirstMessageAt = message.Timestamp
		}
		if turn.LastMessageAt.IsZero() || message.Timestamp.After(turn.LastMessageAt) {
			turn.LastMessageAt = message.Timestamp
		}

		switch message.Role {
		case ChatRoleUser:
			msg := message
			if turn.UserMessage == nil {
				turn.UserMessage = &msg
			}
		case ChatRoleAssistant:
			turn.AssistantMessages = append(turn.AssistantMessages, message)
			turn.AssistantCompleted = true
		case ChatRoleSystem:
			turn.SystemMessages = append(turn.SystemMessages, message)
		case ChatRoleTool:
			turn.ToolMessages = append(turn.ToolMessages, message)
		}
	}

	sort.Ints(turnOrder)
	turns := make([]TranscriptTurn, 0, len(turnOrder))
	for _, idx := range turnOrder {
		turns = append(turns, *turnMap[idx])
	}

	return ChatTranscript{
		ConversationID: c.ID,
		SessionID:      c.SessionID,
		Status:         c.Status,
		Messages:       ordered,
		Turns:          turns,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}
