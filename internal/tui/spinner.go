package tui

import (
	"fmt"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// SpinnerModel wraps a Bubbles spinner for tool execution indication.
type SpinnerModel struct {
	spinner  spinner.Model
	toolName string
	active   bool
}

// NewSpinnerModel creates a new spinner with default settings.
func NewSpinnerModel() SpinnerModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(accentColor)),
	)
	return SpinnerModel{
		spinner: s,
	}
}

// Start activates the spinner with the given tool name.
func (s *SpinnerModel) Start(toolName string) {
	s.toolName = toolName
	s.active = true
}

// Stop deactivates the spinner.
func (s *SpinnerModel) Stop() {
	s.toolName = ""
	s.active = false
}

// Active returns whether the spinner is currently running.
func (s *SpinnerModel) Active() bool {
	return s.active
}

// Update handles spinner tick messages.
func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if !s.active {
		return s, nil
	}
	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

// View renders the spinner with the tool name.
func (s SpinnerModel) View() string {
	if !s.active {
		return ""
	}
	return fmt.Sprintf("%s Executing %s...", s.spinner.View(), s.toolName)
}

// Tick returns the spinner's tick command for animation.
func (s SpinnerModel) Tick() tea.Cmd {
	if !s.active {
		return nil
	}
	return s.spinner.Tick
}
