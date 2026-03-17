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

// ToolStartEvent is emitted when a tool begins execution.
type ToolStartEvent struct {
	Name  string
	Input json.RawMessage
}

// ToolEndEvent is emitted when a tool finishes execution.
type ToolEndEvent struct {
	Name    string
	Content string
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

// Sealed interface implementations.
func (e *TokenEvent) agentEvent()             {}
func (e *ToolStartEvent) agentEvent()         {}
func (e *ToolEndEvent) agentEvent()           {}
func (e *PermissionRequestEvent) agentEvent() {}
func (e *ErrorEvent) agentEvent()             {}
func (e *DoneEvent) agentEvent()              {}
