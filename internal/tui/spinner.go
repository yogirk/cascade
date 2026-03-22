package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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
	"bash":            "Executing command...",
	"read":            "Reading file...",
	"write":           "Writing file...",
	"edit":            "Applying edits...",
	"grep":            "Searching code...",
	"glob":            "Scanning files...",
	"bigquery_query":  "Executing query...",
	"bigquery_schema": "Looking up schema...",
}

// Sweep palette — dim to bright for the character spotlight effect.
var sweepDim color.Color = lipgloss.Color("#4B5563")
var sweepMid color.Color = lipgloss.Color("#9CA3AF")
var sweepBright color.Color = lipgloss.Color("#F3F4F6")

// Cascade tilde colors — ocean blue palette matching the logo.
var (
	cascadeDim    color.Color = lipgloss.Color("#1E3A5F")
	cascadeTrail  color.Color = lipgloss.Color("#0369A1")
	cascadeBright color.Color = lipgloss.Color("#38BDF8")
	cascadePeak   color.Color = lipgloss.Color("#7DD3FC")
)

// tickMsg drives the cascade animation at ~12fps.
type cascadeTickMsg time.Time

// SpinnerModel renders an animated cascade tilde (~~~) with rotating status
// messages, elapsed timer, and sweep glow effect.
type SpinnerModel struct {
	toolName         string
	mode             spinnerMode
	active           bool
	msgIndex         int       // Current message index for rotation
	lastSwap         time.Time // When the message last rotated
	turnStart        time.Time // When the current turn started (for elapsed timer)
	turnPromptTokens int32     // Accumulated prompt tokens this turn
	turnCompTokens   int32     // Accumulated completion tokens this turn
}

type spinnerMode int

const (
	spinnerIdle     spinnerMode = iota
	spinnerThinking             // LLM is streaming
	spinnerTool                 // Tool is executing
)

const messageRotateInterval = 2 * time.Second

// NewSpinnerModel creates a new spinner with cascade tilde animation.
func NewSpinnerModel() SpinnerModel {
	return SpinnerModel{}
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

// StartTurn records the start of a new agent turn (for elapsed timer).
// Called once when the user submits a message.
func (s *SpinnerModel) StartTurn() {
	s.turnStart = time.Now()
}

// Stop deactivates the spinner but preserves turnStart for elapsed continuity.
func (s *SpinnerModel) Stop() {
	s.toolName = ""
	s.mode = spinnerIdle
	s.active = false
}

// AddTurnTokens accumulates token usage for the current turn.
func (s *SpinnerModel) AddTurnTokens(prompt, completion int32) {
	s.turnPromptTokens += prompt
	s.turnCompTokens += completion
}

// EndTurn fully resets the spinner including the turn timer and token counts.
// Called when the agent turn is complete (DoneEvent).
func (s *SpinnerModel) EndTurn() {
	s.Stop()
	s.turnStart = time.Time{}
	s.turnPromptTokens = 0
	s.turnCompTokens = 0
}

// Active returns whether the spinner is currently running.
func (s *SpinnerModel) Active() bool {
	return s.active
}

// Update handles tick messages and rotates the status message.
func (s SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	if !s.active {
		return s, nil
	}

	switch msg.(type) {
	case cascadeTickMsg:
		// Rotate message periodically
		if time.Since(s.lastSwap) >= messageRotateInterval {
			if s.mode == spinnerThinking {
				s.msgIndex = (s.msgIndex + 1) % len(thinkingMessages)
			}
			s.lastSwap = time.Now()
		}
	}

	return s, nil
}

// renderSweep renders text with a spotlight that sweeps across characters,
// similar to Claude Code's spinner effect. A bright "light" moves left-to-right
// across the text, with characters near the light being brighter.
func renderSweep(text string, elapsed time.Duration) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return ""
	}

	// Sweep position moves across text, wrapping around.
	// Complete one sweep every 1.5 seconds.
	speed := 1.5
	pos := math.Mod(elapsed.Seconds()/speed, 1.0) * float64(n+4) // +4 for trail overshoot

	var sb strings.Builder
	for i, r := range runes {
		dist := math.Abs(pos - float64(i))

		var c color.Color
		switch {
		case dist < 1.0:
			c = sweepBright
		case dist < 3.0:
			c = sweepMid
		default:
			c = sweepDim
		}
		sb.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(r)))
	}
	return sb.String()
}

// formatElapsed returns a human-readable elapsed time string.
func formatElapsed(d time.Duration) string {
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%dm%ds", secs/60, secs%60)
}

// renderCascadeTilde renders ≋ with a pulsing color cycle through the ocean blue palette.
func renderCascadeTilde(elapsed time.Duration) string {
	// Smooth sine pulse over 1.5 seconds
	phase := math.Sin(elapsed.Seconds() * math.Pi / 0.75)
	// Map [-1, 1] → palette index [0, 3]
	idx := int((phase + 1) / 2 * 3)
	palette := []color.Color{cascadeDim, cascadeTrail, cascadeBright, cascadePeak}
	if idx >= len(palette) {
		idx = len(palette) - 1
	}
	return lipgloss.NewStyle().Foreground(palette[idx]).Render("≋")
}

// renderToolTilde renders a single ~ with the same pulsing color for tool calls.
func renderToolTilde(elapsed time.Duration) string {
	phase := math.Sin(elapsed.Seconds() * math.Pi / 0.75)
	idx := int((phase + 1) / 2 * 3)
	palette := []color.Color{cascadeDim, cascadeTrail, cascadeBright, cascadePeak}
	if idx >= len(palette) {
		idx = len(palette) - 1
	}
	return lipgloss.NewStyle().Foreground(palette[idx]).Render("~")
}

// View renders the cascade tilde with status message, sweep glow, and elapsed timer.
func (s SpinnerModel) View() string {
	if !s.active {
		return ""
	}

	var msg string

	switch s.mode {
	case spinnerThinking:
		msg = thinkingMessages[s.msgIndex]
	case spinnerTool:
		if m, ok := toolMessages[s.toolName]; ok {
			msg = m
		} else {
			msg = "Running " + s.toolName + "..."
		}
	default:
		msg = "Working..."
	}

	elapsed := time.Since(s.turnStart)

	// ≋ for thinking/assistant, ~ for tool calls
	var tilde string
	if s.mode == spinnerTool {
		tilde = renderToolTilde(elapsed)
	} else {
		tilde = renderCascadeTilde(elapsed)
	}

	// Sweep glow effect on the message text
	swept := renderSweep(msg, elapsed)

	// Elapsed timer and token counts
	var meta []string
	if !s.turnStart.IsZero() && elapsed >= 1*time.Second {
		meta = append(meta, formatElapsed(elapsed))
	}
	if s.turnPromptTokens > 0 || s.turnCompTokens > 0 {
		meta = append(meta, fmt.Sprintf("↑%s ↓%s",
			formatTokens(s.turnPromptTokens), formatTokens(s.turnCompTokens)))
	}

	var suffix string
	if len(meta) > 0 {
		suffix = " " + StatusDimStyle.Render(strings.Join(meta, " · "))
	}

	return tilde + " " + swept + suffix
}

// Tick returns a command that triggers the next animation frame at ~12fps.
func (s SpinnerModel) Tick() tea.Cmd {
	if !s.active {
		return nil
	}
	return tea.Tick(time.Second/12, func(t time.Time) tea.Msg {
		return cascadeTickMsg(t)
	})
}
