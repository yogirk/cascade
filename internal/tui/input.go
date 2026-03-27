package tui

import (
	"reflect"
	"strings"
	"unsafe"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const maxHistory = 50

// InputModel provides a chat-style input with:
//   - Soft wrapping while typing
//   - Shift+Enter inserts a hard newline
//   - Enter submits
//   - Up/Down navigates history (single-line mode only)
//
// Visual style: left accent border, elevated background, no full box border.
type InputModel struct {
	input          textarea.Model
	focused        bool
	width          int
	terminalHeight int
	history        []string
	histIdx        int
	draft          string
}

// NewInputModel creates a new input model.
func NewInputModel() InputModel {
	ti := textarea.New()
	ti.Placeholder = "Ask anything..."
	ti.Prompt = ""
	ti.ShowLineNumbers = false
	ti.CharLimit = 0
	ti.EndOfBufferCharacter = ' '

	styles := ti.Styles()
	styles.Focused.Base = lipgloss.NewStyle().Background(inputBgColor)
	styles.Blurred.Base = lipgloss.NewStyle().Background(inputBgColor)
	styles.Focused.Prompt = lipgloss.NewStyle().Background(inputBgColor)
	styles.Blurred.Prompt = lipgloss.NewStyle().Background(inputBgColor)
	styles.Focused.Text = lipgloss.NewStyle().
		Foreground(textColor).
		Background(inputBgColor)
	styles.Blurred.Text = lipgloss.NewStyle().
		Foreground(dimTextColor).
		Background(inputBgColor)
	styles.Focused.Placeholder = lipgloss.NewStyle().
		Foreground(dimTextColor).
		Background(inputBgColor)
	styles.Blurred.Placeholder = lipgloss.NewStyle().
		Foreground(dimTextColor).
		Background(inputBgColor)
	styles.Focused.CursorLine = lipgloss.NewStyle().Background(inputBgColor)
	styles.Blurred.CursorLine = lipgloss.NewStyle().Background(inputBgColor)
	styles.Focused.EndOfBuffer = lipgloss.NewStyle().Background(inputBgColor)
	styles.Blurred.EndOfBuffer = lipgloss.NewStyle().Background(inputBgColor)
	styles.Cursor.Blink = false
	ti.SetStyles(styles)
	ti.SetHeight(1)

	ti.Focus()

	m := InputModel{
		input:   ti,
		focused: true,
		width:   80,
		histIdx: -1,
	}
	m.syncInputSize()
	return m
}

// shiftEnterKey matches shift+enter for multiline insert.
var shiftEnterKey = key.NewBinding(key.WithKeys("shift+enter"))

// Update handles input messages.
func (i InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	if !i.focused {
		return i, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if key.Matches(keyMsg, shiftEnterKey) {
			i.input.InsertString("\n")
			i.syncInputSize()
			return i, nil
		}
	}

	var cmd tea.Cmd
	i.input, cmd = i.input.Update(msg)
	i.syncInputSize()
	return i, cmd
}

// HistoryUp navigates to the previous prompt in history.
func (i *InputModel) HistoryUp() bool {
	if len(i.history) == 0 || strings.Contains(i.input.Value(), "\n") {
		return false // no history, or in multiline mode
	}

	if i.histIdx == -1 {
		i.draft = i.input.Value()
		i.histIdx = len(i.history) - 1
	} else if i.histIdx > 0 {
		i.histIdx--
	} else {
		return true
	}

	i.input.SetValue(i.history[i.histIdx])
	i.input.CursorEnd()
	return true
}

// HistoryDown navigates to the next prompt in history or back to draft.
func (i *InputModel) HistoryDown() bool {
	if i.histIdx == -1 {
		return false
	}

	if i.histIdx < len(i.history)-1 {
		i.histIdx++
		i.input.SetValue(i.history[i.histIdx])
	} else {
		i.histIdx = -1
		i.input.SetValue(i.draft)
		i.draft = ""
	}
	i.input.CursorEnd()
	return true
}

// PushHistory records a submitted prompt in history.
func (i *InputModel) PushHistory(prompt string) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return
	}
	if len(i.history) > 0 && i.history[len(i.history)-1] == trimmed {
		return
	}
	i.history = append(i.history, trimmed)
	if len(i.history) > maxHistory {
		i.history = i.history[len(i.history)-maxHistory:]
	}
	i.histIdx = -1
	i.draft = ""
}

// Height returns the total rendered height of the input box.
func (i *InputModel) Height() int {
	return lipgloss.Height(i.View())
}

// View renders the input area with a left accent border and elevated background.
func (i InputModel) View() string {
	return i.renderBox()
}

// Value returns the full input text (all lines joined).
func (i *InputModel) Value() string {
	return i.input.Value()
}

// Reset clears the input text.
func (i *InputModel) Reset() {
	i.input.Reset()
	i.syncInputSize()
}

// Focus gives focus to the input area.
func (i *InputModel) Focus() tea.Cmd {
	i.focused = true
	return i.input.Focus()
}

// Blur removes focus from the input area.
func (i *InputModel) Blur() {
	i.focused = false
	i.input.Blur()
}

// SetTerminalHeight updates the terminal height for input max height calculation.
func (i *InputModel) SetTerminalHeight(h int) {
	i.terminalHeight = h
}

// SetWidth updates the input width.
func (i *InputModel) SetWidth(width int) {
	i.width = width
	// Match the rendered content width inside the bordered/padded box.
	tiWidth := width - 5
	if tiWidth < 20 {
		tiWidth = 20
	}
	i.input.SetWidth(tiWidth)
	i.syncInputSize()
}

// Focused returns whether the input is currently focused.
func (i *InputModel) Focused() bool {
	return i.focused
}

func (i InputModel) renderBox() string {
	accentClr := accentColor
	if !i.focused {
		accentClr = inputBorderDimColor
	}

	content := i.input.View()

	boxWidth := i.width
	if boxWidth < 20 {
		boxWidth = 20
	}

	return lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(accentClr).
		PaddingTop(1).
		PaddingBottom(1).
		PaddingLeft(2).
		PaddingRight(2).
		Width(boxWidth).
		Background(inputBgColor).
		Render(content)
}

func (i *InputModel) syncInputSize() {
	width := i.input.Width()
	if width <= 0 {
		i.input.SetHeight(1)
		i.resetViewport()
		return
	}

	value := i.input.Value()
	if value == "" {
		value = i.input.Placeholder
	}

	totalLines := 0
	for _, line := range strings.Split(value, "\n") {
		wrapped := lipgloss.Wrap(line, width, "")
		lineCount := len(strings.Split(wrapped, "\n"))
		if lineCount == 0 {
			lineCount = 1
		}
		totalLines += lineCount
	}
	if totalLines < 1 {
		totalLines = 1
	}

	// Cap at min(40% of terminal height, 10 lines) to prevent paste-bombing
	maxLines := 10
	if i.terminalHeight > 0 {
		fortyPct := i.terminalHeight * 2 / 5
		if fortyPct < maxLines {
			maxLines = fortyPct
		}
	}
	if maxLines < 3 {
		maxLines = 3
	}
	if totalLines > maxLines {
		totalLines = maxLines
	}

	i.input.SetHeight(totalLines)
	i.resetViewport()
}

func (i *InputModel) resetViewport() {
	// Bubble's textarea keeps its own private viewport offset. When the input
	// grows to accommodate soft wraps, that offset can remain pinned one row
	// down, which hides the first wrapped row. Snap it back to the top so the
	// full wrapped block stays visible.
	field := reflect.ValueOf(&i.input).Elem().FieldByName("viewport")
	if !field.IsValid() || field.IsNil() {
		return
	}
	vp := (*viewport.Model)(unsafe.Pointer(field.Pointer()))
	vp.GotoTop()
}
