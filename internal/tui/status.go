package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/cascade-cli/cascade/internal/permission"
)

// StatusModel renders the status bar at the bottom of the TUI.
type StatusModel struct {
	modelName string
	mode      permission.Mode
	version   string
	toolName  string // Non-empty when a tool is executing
	width     int
	message   string // Transient status message
}

// NewStatusModel creates a new status bar.
func NewStatusModel(modelName, version string, mode permission.Mode) StatusModel {
	return StatusModel{
		modelName: modelName,
		mode:      mode,
		version:   version,
	}
}

// SetMode updates the displayed permission mode.
func (s *StatusModel) SetMode(mode permission.Mode) {
	s.mode = mode
}

// SetToolName sets the currently executing tool name (empty string to clear).
func (s *StatusModel) SetToolName(name string) {
	s.toolName = name
}

// SetMessage sets a transient status message.
func (s *StatusModel) SetMessage(msg string) {
	s.message = msg
}

// SetWidth updates the status bar width.
func (s *StatusModel) SetWidth(width int) {
	s.width = width
}

// View renders the status bar.
func (s StatusModel) View() string {
	w := s.width
	if w <= 0 {
		w = 80
	}

	// Left: model name
	left := StatusBarModelStyle.Render(s.modelName)

	// Center: permission mode badge
	center := modeBadge(s.mode)

	// Right: version or tool status
	var right string
	if s.toolName != "" {
		right = ToolStyle.Render(fmt.Sprintf("executing %s...", s.toolName))
	} else if s.message != "" {
		right = s.message
	} else {
		right = StatusBarVersionStyle.Render("cascade " + s.version)
	}

	// Calculate padding
	leftLen := lipgloss.Width(left)
	centerLen := lipgloss.Width(center)
	rightLen := lipgloss.Width(right)

	totalContent := leftLen + centerLen + rightLen
	if totalContent >= w {
		// Not enough space, just concatenate with minimal spacing
		return StatusBarStyle.Width(w).Render(left + " " + center + " " + right)
	}

	// Distribute padding evenly
	remaining := w - totalContent
	leftPad := remaining / 2
	rightPad := remaining - leftPad

	bar := left + strings.Repeat(" ", leftPad) + center + strings.Repeat(" ", rightPad) + right
	return StatusBarStyle.Width(w).Render(bar)
}

// modeBadge returns the styled badge for the current permission mode.
func modeBadge(mode permission.Mode) string {
	switch mode {
	case permission.ModeConfirm:
		return ConfirmModeBadge
	case permission.ModePlan:
		return PlanModeBadge
	case permission.ModeBypass:
		return BypassModeBadge
	default:
		return ConfirmModeBadge
	}
}
