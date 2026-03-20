package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	inputMinHeight = 1
	inputMaxHeight = 8
	maxHistory     = 50
)

// InputModel wraps a textarea for multi-line user input with a bordered box
// and prompt history.
type InputModel struct {
	textarea textarea.Model
	focused  bool
	width    int
	history  []string // Previous submitted prompts
	histIdx  int      // Current position in history (-1 = composing new)
	draft    string   // Saved draft when navigating history
}

// NewInputModel creates a new input model with a bordered, Claude Code-style design.
func NewInputModel() InputModel {
	ta := textarea.New()
	ta.Placeholder = "Ask anything..."
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.SetHeight(inputMinHeight)
	ta.CharLimit = 0 // No limit

	// Rebind InsertNewline to shift+enter so plain enter is free for submit
	km := ta.KeyMap
	km.InsertNewline = key.NewBinding(
		key.WithKeys("shift+enter"),
		key.WithHelp("shift+enter", "newline"),
	)
	ta.KeyMap = km

	// Remove all textarea styling — we render the border ourselves
	styles := ta.Styles()
	styles.Focused.Base = lipgloss.NewStyle()
	styles.Blurred.Base = lipgloss.NewStyle()
	styles.Focused.Prompt = lipgloss.NewStyle()
	styles.Blurred.Prompt = lipgloss.NewStyle()
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(dimTextColor)
	styles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(dimTextColor)
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Cursor.Blink = false // Steady cursor, no blink
	ta.SetStyles(styles)

	ta.Focus()

	return InputModel{
		textarea: ta,
		focused:  true,
		width:    80,
		histIdx:  -1,
	}
}

// Update handles textarea messages and auto-resizes height for multi-line input.
func (i InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	if !i.focused {
		return i, nil
	}
	var cmd tea.Cmd
	i.textarea, cmd = i.textarea.Update(msg)
	i.autoResize()
	return i, cmd
}

// HistoryUp navigates to the previous prompt in history.
// Returns true if history was navigated (caller should not forward key to textarea).
func (i *InputModel) HistoryUp() bool {
	if len(i.history) == 0 {
		return false
	}
	// Only navigate history when input is single-line (no newlines)
	if strings.Contains(i.textarea.Value(), "\n") {
		return false
	}

	if i.histIdx == -1 {
		// Save current draft before navigating
		i.draft = i.textarea.Value()
		i.histIdx = len(i.history) - 1
	} else if i.histIdx > 0 {
		i.histIdx--
	} else {
		return true // Already at oldest, consume the key
	}

	i.textarea.Reset()
	i.textarea.InsertString(i.history[i.histIdx])
	i.autoResize()
	return true
}

// HistoryDown navigates to the next prompt in history or back to draft.
// Returns true if history was navigated.
func (i *InputModel) HistoryDown() bool {
	if i.histIdx == -1 {
		return false // Not in history mode
	}

	if i.histIdx < len(i.history)-1 {
		i.histIdx++
		i.textarea.Reset()
		i.textarea.InsertString(i.history[i.histIdx])
	} else {
		// Back to draft
		i.histIdx = -1
		i.textarea.Reset()
		if i.draft != "" {
			i.textarea.InsertString(i.draft)
		}
		i.draft = ""
	}
	i.autoResize()
	return true
}

// PushHistory records a submitted prompt in history.
func (i *InputModel) PushHistory(prompt string) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return
	}
	// Deduplicate against last entry
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

// autoResize adjusts the textarea height to match the number of content lines.
// When the height grows, it resets the internal viewport so all lines are visible.
func (i *InputModel) autoResize() {
	lines := strings.Count(i.textarea.Value(), "\n") + 1
	h := lines
	if h < inputMinHeight {
		h = inputMinHeight
	}
	if h > inputMaxHeight {
		h = inputMaxHeight
	}

	oldH := i.textarea.Height()
	i.textarea.SetHeight(h)

	// When height grows, the textarea's internal viewport may be scrolled past
	// the top (it followed the cursor while height was still small). Reset by
	// visiting line 0 and navigating back to the cursor's original position.
	if h > oldH {
		row := i.textarea.Line()
		col := i.textarea.LineInfo().ColumnOffset
		i.textarea.MoveToBegin()
		for j := 0; j < row; j++ {
			i.textarea.CursorDown()
		}
		i.textarea.SetCursorColumn(col)
	}
}

// Height returns the total rendered height of the input box (borders + content + hints).
func (i *InputModel) Height() int {
	lines := strings.Count(i.textarea.Value(), "\n") + 1
	h := lines
	if h < inputMinHeight {
		h = inputMinHeight
	}
	if h > inputMaxHeight {
		h = inputMaxHeight
	}
	// top border (1) + content lines (h) + hint line (1) + bottom border (1)
	return h + 3
}

// View renders the input area with a rounded border box.
func (i InputModel) View() string {
	// Content width between the two border characters
	boxWidth := i.width - 2
	if boxWidth < 20 {
		boxWidth = 20
	}

	borderColor := inputBorderColor
	if !i.focused {
		borderColor = inputBorderDimColor
	}

	// Input content lines: " > " + textarea content
	taView := i.textarea.View()
	taLines := strings.Split(taView, "\n")

	var lines []string
	for idx, line := range taLines {
		if idx == 0 {
			lines = append(lines, " "+UserPromptStyle.Render("> ")+line)
		} else {
			lines = append(lines, "   "+line) // align with first line after "> "
		}
	}

	// Hint line
	if i.focused {
		hints := strings.Join([]string{
			hintKeyStyle.Render("enter") + hintTextStyle.Render(" send"),
			hintKeyStyle.Render("shift+enter") + hintTextStyle.Render(" newline"),
			hintKeyStyle.Render("↑↓") + hintTextStyle.Render(" history"),
		}, hintSepStyle.Render("  ·  "))
		lines = append(lines, " "+hints)
	}

	content := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(boxWidth)

	return style.Render(content)
}

// Value returns the current input text.
func (i *InputModel) Value() string {
	return i.textarea.Value()
}

// Reset clears the input text and resets height.
func (i *InputModel) Reset() {
	i.textarea.Reset()
	i.textarea.SetHeight(inputMinHeight)
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
	i.width = width
	// Textarea width = total width minus: 2 border ││ + 1 pad + 2 prompt "> "
	taWidth := width - 5
	if taWidth < 20 {
		taWidth = 20
	}
	i.textarea.SetWidth(taWidth)
}

// Focused returns whether the input is currently focused.
func (i *InputModel) Focused() bool {
	return i.focused
}
