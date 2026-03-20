package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ConfirmModel handles inline permission confirmation prompts.
type ConfirmModel struct {
	active    bool
	toolName  string
	riskLevel string
	input     json.RawMessage
	response  chan<- bool
	cursor    int // 0=Allow, 1=Deny
}

// NewConfirmModel creates a new confirmation prompt model.
func NewConfirmModel() ConfirmModel {
	return ConfirmModel{}
}

// Show activates the confirmation prompt with the given tool details.
func (c *ConfirmModel) Show(toolName string, input json.RawMessage, riskLevel string, response chan<- bool) {
	c.active = true
	c.toolName = toolName
	c.input = input
	c.riskLevel = riskLevel
	c.response = response
	c.cursor = 0 // default to Allow
}

// Active returns whether the confirmation prompt is currently shown.
func (c *ConfirmModel) Active() bool {
	return c.active
}

// Height returns the number of terminal lines the confirm prompt occupies.
func (c *ConfirmModel) Height() int {
	if !c.active {
		return 0
	}
	// Header line + arg summary lines + options line + padding
	lines := 1 // header: risk badge + tool name
	lines += len(formatArgsSummary(c.toolName, c.input))
	lines += 1 // Allow / Deny row
	lines += 1 // trailing padding
	return lines
}

// Update handles key presses for the confirmation prompt.
func (c ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !c.active {
		return c, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "y", "Y":
			c.sendResponse(true)
			return c, nil
		case "enter":
			c.sendResponse(c.cursor == 0)
			return c, nil
		case "n", "N", "esc":
			c.sendResponse(false)
			return c, nil
		case "j", "down", "tab":
			if c.cursor < 1 {
				c.cursor++
			}
			return c, nil
		case "k", "up", "shift+tab":
			if c.cursor > 0 {
				c.cursor--
			}
			return c, nil
		}
	}

	return c, nil
}

// View renders the confirmation prompt with context and readable args.
func (c ConfirmModel) View() string {
	if !c.active {
		return ""
	}

	var sb strings.Builder

	// Risk badge and action header
	sb.WriteString(RiskBadge(c.riskLevel))
	sb.WriteString(" ")
	sb.WriteString(ToolNameStyle.Render(c.toolName))
	sb.WriteString(" wants to execute:\n")

	// Human-readable argument summary
	summary := formatArgsSummary(c.toolName, c.input)
	for _, line := range summary {
		sb.WriteString("  ")
		sb.WriteString(StatusDimStyle.Render(line))
		sb.WriteString("\n")
	}

	// Action options
	allowLabel := "  Allow (y)"
	denyLabel := "  Deny  (n)"
	if c.cursor == 0 {
		allowLabel = confirmActiveStyle.Render("▸ Allow (y)")
		denyLabel = "  " + StatusDimStyle.Render("Deny  (n)")
	} else {
		allowLabel = "  " + StatusDimStyle.Render("Allow (y)")
		denyLabel = confirmActiveStyle.Render("▸ Deny  (n)")
	}
	sb.WriteString(allowLabel)
	sb.WriteString("   ")
	sb.WriteString(denyLabel)

	return ConfirmBoxStyle.Render(sb.String())
}

// sendResponse sends the user's decision and deactivates the prompt.
func (c *ConfirmModel) sendResponse(approved bool) {
	if c.response != nil {
		c.response <- approved
	}
	c.active = false
	c.toolName = ""
	c.input = nil
	c.riskLevel = ""
	c.response = nil
	c.cursor = 0
}

// formatArgsSummary produces human-readable lines summarizing tool args.
func formatArgsSummary(toolName string, input json.RawMessage) []string {
	if len(input) == 0 {
		return nil
	}

	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return []string{string(input)}
	}

	// Tool-specific formatting for common tools
	switch toolName {
	case "bash":
		if cmd, ok := args["command"].(string); ok {
			lines := strings.Split(cmd, "\n")
			if len(lines) == 1 {
				return []string{"$ " + cmd}
			}
			var result []string
			for _, line := range lines {
				result = append(result, "  "+line)
			}
			return result
		}
	case "write":
		if path, ok := args["file_path"].(string); ok {
			return []string{"file: " + path}
		}
	case "edit":
		if path, ok := args["file_path"].(string); ok {
			return []string{"file: " + path}
		}
	}

	// Generic: show key=value pairs, sorted for consistency
	var keys []string
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	for _, k := range keys {
		v := args[k]
		s := fmt.Sprintf("%v", v)
		if len(s) > 60 {
			s = s[:57] + "..."
		}
		lines = append(lines, fmt.Sprintf("%s: %s", k, s))
	}
	return lines
}

// abbreviateArgs produces a shortened string representation of tool input args.
// Used in chat transcript tool headers.
func abbreviateArgs(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return ""
	}

	var keys []string
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		s := fmt.Sprintf("%v", args[k])
		if len(s) > 40 {
			s = s[:37] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, s))
	}

	result := strings.Join(parts, " ")
	if len(result) > 80 {
		result = result[:77] + "..."
	}
	return result
}

// confirmActiveStyle styles the active option in the confirm prompt.
var confirmActiveStyle = lipgloss.NewStyle().Foreground(brightColor).Bold(true)
