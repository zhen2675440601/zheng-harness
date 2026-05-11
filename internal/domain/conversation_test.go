package domain

import (
	"testing"
	"time"
)

func TestConversationMessagesInOrder(t *testing.T) {
	t.Parallel()

	base := time.Unix(1710000000, 0).UTC()

	conversation := Conversation{
		ID:        "conv-1",
		SessionID: "sess-1",
		Messages: []ChatMessage{
			{ID: "m3", TurnIndex: 2, Role: ChatRoleAssistant, Content: "answer", Timestamp: base.Add(2 * time.Second)},
			{ID: "m1", TurnIndex: 1, Role: ChatRoleUser, Content: "hello", Timestamp: base},
			{ID: "m2", TurnIndex: 2, Role: ChatRoleUser, Content: "next", Timestamp: base.Add(2 * time.Second)},
		},
	}

	ordered := conversation.MessagesInOrder()
	if len(ordered) != 3 {
		t.Fatalf("ordered length = %d, want 3", len(ordered))
	}

	if ordered[0].ID != "m1" || ordered[1].ID != "m2" || ordered[2].ID != "m3" {
		t.Fatalf("ordered IDs = [%s %s %s], want [m1 m2 m3]", ordered[0].ID, ordered[1].ID, ordered[2].ID)
	}
}

func TestConversationBelongsToSession(t *testing.T) {
	t.Parallel()

	conversation := Conversation{ID: "conv-1", SessionID: "sess-abc"}
	if !conversation.BelongsToSession("sess-abc") {
		t.Fatalf("expected conversation to belong to sess-abc")
	}
	if conversation.BelongsToSession("sess-other") {
		t.Fatalf("did not expect conversation to belong to sess-other")
	}
}

func TestBuildChatTranscriptAssemblesTurns(t *testing.T) {
	t.Parallel()

	base := time.Unix(1710001000, 0).UTC()
	conversation := Conversation{
		ID:        "conv-2",
		SessionID: "sess-2",
		Status:    ConversationStatusActive,
		CreatedAt: base,
		UpdatedAt: base.Add(10 * time.Second),
		Messages: []ChatMessage{
			{ID: "u1", TurnIndex: 1, Role: ChatRoleUser, Content: "question one", Timestamp: base.Add(1 * time.Second)},
			{ID: "a1", TurnIndex: 1, Role: ChatRoleAssistant, Content: "answer one", Timestamp: base.Add(2 * time.Second)},
			{ID: "u2", TurnIndex: 2, Role: ChatRoleUser, Content: "question two", Timestamp: base.Add(3 * time.Second)},
			{ID: "t2", TurnIndex: 2, Role: ChatRoleTool, Content: "tool trace", Timestamp: base.Add(4 * time.Second)},
			{ID: "a2", TurnIndex: 2, Role: ChatRoleAssistant, Content: "answer two", Timestamp: base.Add(5 * time.Second)},
		},
	}

	transcript := BuildChatTranscript(conversation)

	if transcript.ConversationID != conversation.ID {
		t.Fatalf("ConversationID = %s, want %s", transcript.ConversationID, conversation.ID)
	}
	if transcript.SessionID != conversation.SessionID {
		t.Fatalf("SessionID = %s, want %s", transcript.SessionID, conversation.SessionID)
	}
	if len(transcript.Turns) != 2 {
		t.Fatalf("turn count = %d, want 2", len(transcript.Turns))
	}

	first := transcript.Turns[0]
	if first.TurnIndex != 1 || first.UserMessage == nil || first.UserMessage.ID != "u1" {
		t.Fatalf("first turn malformed: %#v", first)
	}
	if len(first.AssistantMessages) != 1 || first.AssistantMessages[0].ID != "a1" {
		t.Fatalf("first assistant messages malformed: %#v", first.AssistantMessages)
	}

	second := transcript.Turns[1]
	if second.TurnIndex != 2 || second.UserMessage == nil || second.UserMessage.ID != "u2" {
		t.Fatalf("second turn malformed: %#v", second)
	}
	if len(second.ToolMessages) != 1 || second.ToolMessages[0].ID != "t2" {
		t.Fatalf("second tool messages malformed: %#v", second.ToolMessages)
	}
	if len(second.AssistantMessages) != 1 || second.AssistantMessages[0].ID != "a2" {
		t.Fatalf("second assistant messages malformed: %#v", second.AssistantMessages)
	}
}
