package tui

import (
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// BrailleSpinner is a smooth braille-dot spinner.
var BrailleSpinner = spinner.Spinner{
	Frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	FPS:    time.Second / 12,
}

// thinkingMessages are displayed during LLM streaming.
var thinkingMessages = []string{
	"Thinking...",
	"Reasoning...",
	"Analyzing...",
	"Working on it...",
	"Processing...",
	"Pondering...",
	"Crafting response...",
	"Connecting the dots...",
	"Almost there...",
}

// toolMessages maps tool names to descriptive action messages.
var toolMessages = map[string]string{
	"bash":             "Executing command...",
	"read":             "Reading file...",
	"write":            "Writing file...",
	"edit":             "Applying edits...",
	"grep":             "Searching code...",
	"glob":             "Scanning files...",
	"bigquery_query":   "Executing query...",
	"bigquery_schema":  "Looking up schema...",
	"bigquery_cost":    "Estimating cost...",
}

// SpinnerModel wraps a Bubbles spinner with rotating status messages.
type SpinnerModel struct {
	spinner  spinner.Model
	toolName string
	mode     spinnerMode
	active   bool
	msgIndex int       // Current message index for rotation
	lastSwap time.Time // When the message last rotated
}

type spinnerMode int

const (
	spinnerIdle     spinnerMode = iota
	spinnerThinking             // LLM is streaming
	spinnerTool                 // Tool is executing
)

const messageRotateInterval = 2 * time.Second

// NewSpinnerModel creates a new spinner with braille animation.
func NewSpinnerModel() SpinnerModel {
	s := spinner.New(
		spinner.WithSpinner(BrailleSpinner),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(dimTextColor)),
	)
	return SpinnerModel{
		spinner: s,
	}
}

// StartThinking activates the spinner in thinking mode (LLM streaming).
func (s *SpinnerModel) StartThinking() {
	s.mode = spinnerThinking
	s.active = true
	s.msgIndex = 0
	s.lastSwap = time.Now()
}

// StartTool activates the spinner for a tool execution.
func (s *SpinnerModel) StartTool(toolName string) {
	s.toolName = toolName
	s.mode = spinnerTool
	s.active = true
	s.msgIndex = 0
	s.lastSwap = time.Now()
}

// Stop deactivates the spinner.
func (s *SpinnerModel) Stop() {
	s.toolName = ""
	s.mode = spinnerIdle
	s.active = false
}

// Active returns whether the spinner is currently running.
func (s *SpinnerModel) Active() bool {
	return s.active
}

// Update handles spinner tick messages and rotates the status message.
func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if !s.active {
		return s, nil
	}

	// Rotate message periodically
	if time.Since(s.lastSwap) >= messageRotateInterval {
		if s.mode == spinnerThinking {
			s.msgIndex = (s.msgIndex + 1) % len(thinkingMessages)
		}
		s.lastSwap = time.Now()
	}

	var cmd tea.Cmd
	s.spinner, cmd = s.spinner.Update(msg)
	return s, cmd
}

// View renders the spinner with contextual status message.
func (s SpinnerModel) View() string {
	if !s.active {
		return ""
	}

	var msg string
	var bullet string

	switch s.mode {
	case spinnerThinking:
		msg = thinkingMessages[s.msgIndex]
		bullet = AssistantBulletStyle.Render("∞")
	case spinnerTool:
		if m, ok := toolMessages[s.toolName]; ok {
			msg = m
		} else {
			msg = "Running " + s.toolName + "..."
		}
		bullet = ToolBullet(s.toolName)
	default:
		msg = "Working..."
		bullet = ToolBulletStyle.Render("∞")
	}

	return bullet + " " + s.spinner.View() + " " + StatusDimStyle.Render(msg)
}

// Tick returns the spinner's tick command for animation.
func (s SpinnerModel) Tick() tea.Cmd {
	if !s.active {
		return nil
	}
	return s.spinner.Tick
}

