package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/yogirk/cascade/pkg/types"
)

// ConfirmModel handles inline permission confirmation prompts.
type ConfirmModel struct {
	active    bool
	toolName  string
	riskLevel string
	input     json.RawMessage
	response  chan<- types.ApprovalDecision
	cursor    int
}

// NewConfirmModel creates a new confirmation prompt model.
func NewConfirmModel() ConfirmModel {
	return ConfirmModel{}
}

// Show activates the confirmation prompt with the given tool details.
func (c *ConfirmModel) Show(toolName string, input json.RawMessage, riskLevel string, response chan<- types.ApprovalDecision) {
	c.active = true
	c.toolName = toolName
	c.input = input
	c.riskLevel = riskLevel
	c.response = response
	c.cursor = 0
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
	return lipgloss.Height(c.View())
}

// Update handles key presses for the confirmation prompt.
func (c ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !c.active {
		return c, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "1", "y", "Y":
			c.sendResponse(types.ApprovalAllowOnce)
			return c, nil
		case "2", "a", "A":
			c.sendResponse(types.ApprovalAllowToolSession)
			return c, nil
		case "3", "n", "N", "esc":
			c.sendResponse(types.ApprovalDeny)
			return c, nil
		case "enter":
			c.sendResponse(confirmOptions[c.cursor].action)
			return c, nil
		case "j", "down", "tab", "l", "right":
			if c.cursor < len(confirmOptions)-1 {
				c.cursor++
			}
			return c, nil
		case "k", "up", "shift+tab", "h", "left":
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

	sb.WriteString("\n")
	sb.WriteString(brightConfirmLabel().Render("Choose an action:"))
	sb.WriteString("\n")
	labelWidth := 0
	for idx, option := range confirmOptions {
		label := fmt.Sprintf("%d. %s", idx+1, option.label)
		if len(label) > labelWidth {
			labelWidth = len(label)
		}
	}
	for idx, option := range confirmOptions {
		label := fmt.Sprintf("%d. %s", idx+1, option.label)
		paddedLabel := fmt.Sprintf("%-*s", labelWidth, label)
		desc := StatusDimStyle.Render(option.description)
		if c.cursor == idx {
			sb.WriteString(confirmActiveStyle().Render("▸"))
			sb.WriteString(" ")
			sb.WriteString(confirmActiveStyle().Render(paddedLabel))
		} else {
			sb.WriteString("  ")
			sb.WriteString(StatusDimStyle.Render(paddedLabel))
		}
		sb.WriteString("  ")
		sb.WriteString(desc)
		if idx < len(confirmOptions)-1 {
			sb.WriteString("\n")
		}
	}

	return ConfirmBoxStyle.Render(sb.String())
}

// sendResponse sends the user's decision and deactivates the prompt.
func (c *ConfirmModel) sendResponse(action types.ApprovalAction) {
	if c.response != nil {
		c.response <- types.ApprovalDecision{Action: action}
	}
	c.active = false
	c.toolName = ""
	c.input = nil
	c.riskLevel = ""
	c.response = nil
	c.cursor = 0
}

type confirmOption struct {
	label       string
	description string
	action      types.ApprovalAction
}

var confirmOptions = []confirmOption{
	{label: "Allow once", description: "Run this exact action now", action: types.ApprovalAllowOnce},
	{label: "Allow tool for session", description: "Skip future prompts for this tool until you exit", action: types.ApprovalAllowToolSession},
	{label: "Deny", description: "Block this action", action: types.ApprovalDeny},
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
	case "bigquery_query":
		if sqlStr, ok := args["sql"].(string); ok {
			lines := strings.Split(sqlStr, "\n")
			if len(lines) <= 6 {
				var result []string
				for _, line := range lines {
					result = append(result, "  "+line)
				}
				return result
			}
			var result []string
			for _, line := range lines[:6] {
				result = append(result, "  "+line)
			}
			result = append(result, fmt.Sprintf("  ...%d more lines", len(lines)-6))
			return result
		}
	case "bigquery_schema":
		if action, ok := args["action"].(string); ok {
			if tableName, ok := args["table"].(string); ok {
				return []string{action + ": " + tableName}
			}
			return []string{action}
		}
	case "bigquery_cost":
		if sqlStr, ok := args["sql"].(string); ok {
			truncated := sqlStr
			if len(truncated) > 60 {
				truncated = truncated[:57] + "..."
			}
			return []string{"sql: " + truncated}
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
// Built per-call so theme switches take effect. Previously these were
// package-level `var` declarations — their right-hand sides ran at package
// init time before initPalette() populated brightColor/textColor, leaving
// both styles with nil foreground (rendered as terminal default).
func confirmActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(brightColor).Bold(true)
}

func brightConfirmLabel() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(textColor).Bold(true)
}
