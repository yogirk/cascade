package types

import "encoding/json"

// ApprovalAction describes how a pending permission request was resolved.
type ApprovalAction string

const (
	ApprovalAllowOnce       ApprovalAction = "allow_once"
	ApprovalAllowToolSession ApprovalAction = "allow_tool_session"
	ApprovalDeny            ApprovalAction = "deny"
)

// ApprovalDecision is the response returned to the agent for a pending request.
type ApprovalDecision struct {
	Action ApprovalAction
}

// ApprovalRequest is a dedicated UI permission prompt, kept separate from the
// generic agent event stream so interactive approval behaves like a true modal.
type ApprovalRequest struct {
	ToolName  string
	Input     json.RawMessage
	RiskLevel string
	Response  chan<- ApprovalDecision
}
