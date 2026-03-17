package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// ConfirmModel handles inline permission confirmation prompts.
// It appears above the input area when a tool call needs user permission.
type ConfirmModel struct {
	active    bool
	toolName  string
	riskLevel string
	input     json.RawMessage
	response  chan<- bool
}

// NewConfirmModel creates a new confirmation prompt model.
func NewConfirmModel() ConfirmModel {
	return ConfirmModel{}
}

// Show activates the confirmation prompt with the given tool details.
// The Response channel is used to send the user's decision back to the agent.
func (c *ConfirmModel) Show(toolName string, input json.RawMessage, riskLevel string, response chan<- bool) {
	c.active = true
	c.toolName = toolName
	c.input = input
	c.riskLevel = riskLevel
	c.response = response
}

// Active returns whether the confirmation prompt is currently shown.
func (c *ConfirmModel) Active() bool {
	return c.active
}

// Update handles key presses for the confirmation prompt.
func (c ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	if !c.active {
		return c, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "y", "Y":
			c.sendResponse(true)
			return c, nil
		case "n", "N", "enter", "escape", "esc":
			c.sendResponse(false)
			return c, nil
		}
	}

	return c, nil
}

// View renders the confirmation prompt.
func (c ConfirmModel) View() string {
	if !c.active {
		return ""
	}

	badge := RiskBadge(c.riskLevel)
	args := abbreviateArgs(c.input)

	prompt := fmt.Sprintf("%s %s %s Execute? [y/N]", badge, c.toolName, args)
	return ConfirmStyle.Render(prompt)
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
}

// abbreviateArgs produces a shortened string representation of tool input args.
func abbreviateArgs(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return ""
	}

	var parts []string
	for k, v := range args {
		s := fmt.Sprintf("%v", v)
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
