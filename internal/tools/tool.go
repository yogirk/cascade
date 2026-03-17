package tools

import (
	"context"
	"encoding/json"

	"github.com/cascade-cli/cascade/internal/permission"
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
