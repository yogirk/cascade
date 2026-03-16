package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Governor enforces loop limits, detects duplicate tool calls,
// and triggers progress nudges.
type Governor struct {
	maxToolCalls int
	seen         map[string]int // hash -> count
	nudgeEvery   int            // default 5
}

// NewGovernor creates a governor with the given maximum tool calls per turn.
func NewGovernor(maxToolCalls int) *Governor {
	return &Governor{
		maxToolCalls: maxToolCalls,
		seen:         make(map[string]int),
		nudgeEvery:   5,
	}
}

// CheckLimit returns true if the tool call count has reached the limit.
func (g *Governor) CheckLimit(toolCallCount int) bool {
	return toolCallCount >= g.maxToolCalls
}

// IsDuplicate returns true if this exact tool+args combination was seen before.
func (g *Governor) IsDuplicate(name string, args json.RawMessage) bool {
	key := callHash(name, args)
	g.seen[key]++
	return g.seen[key] > 1
}

// ShouldNudge returns true every nudgeEvery tool calls (5, 10, 15...).
func (g *Governor) ShouldNudge(toolCallCount int) bool {
	return toolCallCount > 0 && toolCallCount%g.nudgeEvery == 0
}

// Reset clears duplicate tracking state for a new turn.
func (g *Governor) Reset() {
	g.seen = make(map[string]int)
}

func callHash(name string, args json.RawMessage) string {
	h := sha256.Sum256(append([]byte(name+":"), args...))
	return fmt.Sprintf("%x", h[:8])
}
