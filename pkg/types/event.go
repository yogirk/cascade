package types

import "encoding/json"

// Event is the sealed interface for all agent events.
// The unexported method prevents external implementations.
type Event interface {
	agentEvent()
}

// TokenEvent is emitted when a streaming text token arrives from the LLM.
type TokenEvent struct {
	Token string
}

// StreamStartEvent is emitted when the LLM starts a new stream (either first or after tool use).
type StreamStartEvent struct{}

// StreamCompleteEvent is emitted when the LLM finishes streaming its response (before tool execution).
type StreamCompleteEvent struct {
	Content string
	Usage   *Usage // Token usage for this stream, nil if unavailable
}

// ToolStartEvent is emitted when a tool begins execution.
type ToolStartEvent struct {
	Name  string
	Input json.RawMessage
}

// ToolEndEvent is emitted when a tool finishes execution.
type ToolEndEvent struct {
	Name    string
	Content string
	Display string // formatted output for TUI (diffs, etc.)
	IsError bool
	Err     error
}

// PermissionRequestEvent is emitted when a tool call needs user permission.
type PermissionRequestEvent struct {
	ToolName  string
	Input     json.RawMessage
	RiskLevel string
	Response  chan<- bool
}

// ErrorEvent is emitted for non-fatal errors during agent execution.
type ErrorEvent struct {
	Err error
}

// DoneEvent is emitted when the agent turn is complete.
type DoneEvent struct{}

// TurnStartEvent is emitted when the agent begins processing a user message.
// Allows the TUI to track turn boundaries.
type TurnStartEvent struct {
	Input string // The user's input text
}

// Sealed interface implementations.
func (e *TokenEvent) agentEvent()             {}
func (e *StreamStartEvent) agentEvent()       {}
func (e *StreamCompleteEvent) agentEvent()    {}
func (e *ToolStartEvent) agentEvent()         {}
func (e *ToolEndEvent) agentEvent()           {}
func (e *PermissionRequestEvent) agentEvent() {}
func (e *ErrorEvent) agentEvent()             {}
func (e *DoneEvent) agentEvent()              {}
func (e *TurnStartEvent) agentEvent()         {}
