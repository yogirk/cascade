package tools

import (
	"context"
	"encoding/json"

	"github.com/slokam-ai/cascade/internal/permission"
)

// Tool defines the interface that all tools must implement.
// Each tool is self-describing: it provides its name, description,
// JSON Schema for input parameters, risk level, and an Execute method.
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	RiskLevel() permission.RiskLevel
	Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

// PermissionPlan allows a tool to refine how the agent should gate execution
// after it has inspected the concrete input.
type PermissionPlan struct {
	RiskOverride *permission.RiskLevel
	DenyMessage  string
}

// PermissionPlanner is an optional interface for tools that need input-aware
// gating before execution, for example cost-aware approval escalation.
type PermissionPlanner interface {
	PlanPermission(ctx context.Context, input json.RawMessage, baseRisk permission.RiskLevel) (*PermissionPlan, error)
}
