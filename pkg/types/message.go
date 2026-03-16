// Package types defines provider-agnostic message and response types
// used throughout the Cascade agent system.
package types

import "encoding/json"

// Role represents the role of a message sender in a conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
	RoleTool      Role = "tool"
)

// Message represents a single message in a conversation.
type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolResult *ToolResult
}

// ToolCall represents an LLM request to invoke a tool.
type ToolCall struct {
	ID    string
	Name  string
	Input json.RawMessage

	// ThoughtSignature is an opaque token from thinking models (e.g. Gemini 3+).
	// Must be echoed back in the conversation to satisfy the API contract.
	ThoughtSignature []byte `json:"-"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	CallID  string
	Name    string // function name (e.g., "bash") — required by Vertex AI
	Content string
	IsError bool
}

// Response represents a complete LLM response after streaming finishes.
// Usage holds token consumption metadata from the LLM.
type Usage struct {
	PromptTokens     int32
	CompletionTokens int32
	TotalTokens      int32
}

type Response struct {
	Text      string
	ToolCalls []ToolCall
	Usage     *Usage // nil if provider doesn't report usage
}

// UserMessage creates a user message with the given content.
func UserMessage(content string) Message {
	return Message{
		Role:    RoleUser,
		Content: content,
	}
}

// AssistantMessage creates an assistant message with optional tool calls.
func AssistantMessage(text string, toolCalls []ToolCall) Message {
	return Message{
		Role:      RoleAssistant,
		Content:   text,
		ToolCalls: toolCalls,
	}
}

// SystemMessage creates a system message with the given content.
func SystemMessage(content string) Message {
	return Message{
		Role:    RoleSystem,
		Content: content,
	}
}

// ToolResultMessage creates a tool result message.
func ToolResultMessage(callID, name, content string, isError bool) Message {
	return Message{
		Role: RoleTool,
		ToolResult: &ToolResult{
			CallID:  callID,
			Name:    name,
			Content: content,
			IsError: isError,
		},
	}
}
