package permission

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Decision represents the permission engine's verdict on a tool call.
type Decision int

const (
	Allow   Decision = iota // Proceed without asking
	Confirm                 // Ask user for permission
	Deny                    // Block execution
)

func (d Decision) String() string {
	switch d {
	case Allow:
		return "ALLOW"
	case Confirm:
		return "ASK"
	case Deny:
		return "DENY"
	default:
		return "UNKNOWN"
	}
}

// ToolRiskProvider is the subset of Tool interface needed by the engine.
// This avoids a circular dependency: permission does NOT import tools.
type ToolRiskProvider interface {
	Name() string
	RiskLevel() RiskLevel
}

// Engine enforces permission policies based on tool risk levels and current mode.
type Engine struct {
	mode         Mode
	cache        map[string]Decision
	toolPolicies map[string]Decision
}

// NewEngine creates a permission engine with the given default mode.
func NewEngine(defaultMode Mode) *Engine {
	return &Engine{
		mode:  defaultMode,
		cache: make(map[string]Decision),
		toolPolicies: make(map[string]Decision),
	}
}

// Check evaluates whether a tool call should be allowed, needs confirmation, or is denied.
func (e *Engine) Check(tool ToolRiskProvider, input json.RawMessage) Decision {
	risk := tool.RiskLevel()

	switch e.mode {
	case ModeReadOnly:
		if risk > RiskReadOnly {
			return Deny
		}
		return Allow

	case ModeFullAccess:
		return Allow

	case ModeAsk:
		if d, ok := e.toolPolicies[tool.Name()]; ok {
			return d
		}
		if risk <= RiskReadOnly {
			return Allow
		}
		// Check session cache
		key := cacheKey(tool.Name(), input)
		if d, ok := e.cache[key]; ok {
			return d
		}
		return Confirm
	}

	return Deny
}

// CacheDecision stores a permission decision for a specific tool+input combination.
// This prevents repeated prompts for the same tool call within a session.
func (e *Engine) CacheDecision(toolName string, input json.RawMessage, decision Decision) {
	key := cacheKey(toolName, input)
	e.cache[key] = decision
}

// CacheToolDecision stores a session-scoped decision for all future invocations
// of a tool, regardless of input.
func (e *Engine) CacheToolDecision(toolName string, decision Decision) {
	e.toolPolicies[toolName] = decision
}

// SetMode sets the permission mode.
func (e *Engine) SetMode(mode Mode) { e.mode = mode }

// Mode returns the current permission mode.
func (e *Engine) Mode() Mode { return e.mode }

// CycleMode advances to the next permission mode in the cycle.
func (e *Engine) CycleMode() { e.mode = CycleMode(e.mode) }

// cacheKey generates a deterministic key from tool name and input for cache lookup.
func cacheKey(name string, input json.RawMessage) string {
	h := sha256.Sum256(append([]byte(name+":"), input...))
	return fmt.Sprintf("%x", h[:8])
}
