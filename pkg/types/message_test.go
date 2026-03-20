package types

import (
	"encoding/json"
	"testing"
)

func TestUserMessage(t *testing.T) {
	msg := UserMessage("hello")
	if msg.Role != RoleUser {
		t.Errorf("expected role %q, got %q", RoleUser, msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("expected content %q, got %q", "hello", msg.Content)
	}
	if len(msg.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(msg.ToolCalls))
	}
	if msg.ToolResult != nil {
		t.Error("expected nil tool result")
	}
}

func TestSystemMessage(t *testing.T) {
	msg := SystemMessage("you are a helpful assistant")
	if msg.Role != RoleSystem {
		t.Errorf("expected role %q, got %q", RoleSystem, msg.Role)
	}
	if msg.Content != "you are a helpful assistant" {
		t.Errorf("expected content %q, got %q", "you are a helpful assistant", msg.Content)
	}
}

func TestAssistantMessage(t *testing.T) {
	toolCalls := []ToolCall{
		{
			ID:    "call_123",
			Name:  "read_file",
			Input: json.RawMessage(`{"path": "/tmp/test.go"}`),
		},
		{
			ID:    "call_456",
			Name:  "grep",
			Input: json.RawMessage(`{"pattern": "TODO"}`),
		},
	}
	msg := AssistantMessage("Let me check that file.", toolCalls)

	if msg.Role != RoleAssistant {
		t.Errorf("expected role %q, got %q", RoleAssistant, msg.Role)
	}
	if msg.Content != "Let me check that file." {
		t.Errorf("expected content %q, got %q", "Let me check that file.", msg.Content)
	}
	if len(msg.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].ID != "call_123" {
		t.Errorf("expected tool call ID %q, got %q", "call_123", msg.ToolCalls[0].ID)
	}
	if msg.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected tool call name %q, got %q", "read_file", msg.ToolCalls[0].Name)
	}
	if string(msg.ToolCalls[0].Input) != `{"path": "/tmp/test.go"}` {
		t.Errorf("unexpected tool call input: %s", msg.ToolCalls[0].Input)
	}
	if msg.ToolCalls[1].ID != "call_456" {
		t.Errorf("expected tool call ID %q, got %q", "call_456", msg.ToolCalls[1].ID)
	}
}

func TestAssistantMessageNoToolCalls(t *testing.T) {
	msg := AssistantMessage("Just text response.", nil)
	if msg.Role != RoleAssistant {
		t.Errorf("expected role %q, got %q", RoleAssistant, msg.Role)
	}
	if len(msg.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(msg.ToolCalls))
	}
}

func TestToolResultMessage(t *testing.T) {
	msg := ToolResultMessage("call_123", "read_file", "file contents here", false)
	if msg.Role != RoleTool {
		t.Errorf("expected role %q, got %q", RoleTool, msg.Role)
	}
	if msg.ToolResult == nil {
		t.Fatal("expected non-nil tool result")
	}
	if msg.ToolResult.CallID != "call_123" {
		t.Errorf("expected call ID %q, got %q", "call_123", msg.ToolResult.CallID)
	}
	if msg.ToolResult.Content != "file contents here" {
		t.Errorf("expected content %q, got %q", "file contents here", msg.ToolResult.Content)
	}
	if msg.ToolResult.IsError {
		t.Error("expected IsError to be false")
	}
}

func TestToolResultMessageError(t *testing.T) {
	msg := ToolResultMessage("call_789", "bash", "permission denied", true)
	if msg.ToolResult == nil {
		t.Fatal("expected non-nil tool result")
	}
	if !msg.ToolResult.IsError {
		t.Error("expected IsError to be true")
	}
	if msg.ToolResult.CallID != "call_789" {
		t.Errorf("expected call ID %q, got %q", "call_789", msg.ToolResult.CallID)
	}
}

func TestRoleConstants(t *testing.T) {
	if RoleUser != "user" {
		t.Errorf("RoleUser = %q, want %q", RoleUser, "user")
	}
	if RoleAssistant != "assistant" {
		t.Errorf("RoleAssistant = %q, want %q", RoleAssistant, "assistant")
	}
	if RoleSystem != "system" {
		t.Errorf("RoleSystem = %q, want %q", RoleSystem, "system")
	}
	if RoleTool != "tool" {
		t.Errorf("RoleTool = %q, want %q", RoleTool, "tool")
	}
}

// Compile-time check that all event types implement Event interface.
// These will fail to compile if the types don't implement Event.
var (
	_ Event = (*TokenEvent)(nil)
	_ Event = (*ToolStartEvent)(nil)
	_ Event = (*ToolEndEvent)(nil)
	_ Event = (*PermissionRequestEvent)(nil)
	_ Event = (*ErrorEvent)(nil)
	_ Event = (*DoneEvent)(nil)
)

func TestEventTypes(t *testing.T) {
	// Verify each event type satisfies the Event interface at runtime too.
	events := []Event{
		&TokenEvent{Token: "hello"},
		&ToolStartEvent{Name: "read_file", Input: json.RawMessage(`{}`)},
		&ToolEndEvent{Name: "read_file", Content: "data", IsError: false},
		&PermissionRequestEvent{ToolName: "bash", RiskLevel: "destructive"},
		&ErrorEvent{Err: nil},
		&DoneEvent{},
	}
	if len(events) != 6 {
		t.Errorf("expected 6 event types, got %d", len(events))
	}
}
