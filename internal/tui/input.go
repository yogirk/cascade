package tui

import (
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

// InputModel wraps a textarea for multi-line user input.
type InputModel struct {
	textarea textarea.Model
	focused  bool
}

// NewInputModel creates a new input model with default settings.
func NewInputModel() InputModel {
	ta := textarea.New()
	ta.Placeholder = "Ask Cascade..."
	ta.ShowLineNumbers = false
	ta.SetHeight(3)
	ta.CharLimit = 0 // No limit
	ta.Focus()

	return InputModel{
		textarea: ta,
		focused:  true,
	}
}

// Update handles textarea messages.
func (i InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	if !i.focused {
		return i, nil
	}
	var cmd tea.Cmd
	i.textarea, cmd = i.textarea.Update(msg)
	return i, cmd
}

// View renders the input area.
func (i InputModel) View() string {
	return i.textarea.View()
}

// Value returns the current input text.
func (i *InputModel) Value() string {
	return i.textarea.Value()
}

// Reset clears the input text.
func (i *InputModel) Reset() {
	i.textarea.Reset()
}

// Focus gives focus to the input area.
func (i *InputModel) Focus() tea.Cmd {
	i.focused = true
	return i.textarea.Focus()
}

// Blur removes focus from the input area.
func (i *InputModel) Blur() {
	i.focused = false
	i.textarea.Blur()
}

// SetWidth updates the input width.
func (i *InputModel) SetWidth(width int) {
	i.textarea.SetWidth(width)
}

// Focused returns whether the input is currently focused.
func (i *InputModel) Focused() bool {
	return i.focused
}
