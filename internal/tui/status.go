package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/permission"
)

// StatusModel renders the status bar at the bottom of the TUI.
type StatusModel struct {
	modelName        string
	mode             permission.Mode
	toolName         string    // Non-empty when a tool is executing
	width            int
	message          string    // Transient status message
	messageSetAt     time.Time // When the message was set (for auto-expire)
	gitBranch        string
	cwd              string
	cost             float64
	dailyBudget      float64 // Daily budget for warning display
	pendingApproval  bool    // True when awaiting user approval
	promptTokens     int32
	completionTokens int32
	totalTokens      int32
	lastPromptTokens int32 // Most recent prompt tokens = current context usage
}

// friendlyModelName converts raw model IDs to human-readable names.
// e.g. "gemini-3-flash-preview" → "Gemini 3 (Flash)"
//      "claude-opus-4-6"        → "Opus 4.6"
//      "gpt-4o-mini"            → "GPT-4o Mini"
func friendlyModelName(raw string) string {
	s := strings.ToLower(raw)
	// Strip common prefixes/suffixes
	s = strings.TrimPrefix(s, "models/")

	switch {
	// Gemini models
	case strings.Contains(s, "gemini"):
		return parseGemini(s)
	// Claude models
	case strings.Contains(s, "claude") || strings.Contains(s, "opus") ||
		strings.Contains(s, "sonnet") || strings.Contains(s, "haiku"):
		return parseClaude(s)
	// GPT models
	case strings.Contains(s, "gpt"):
		return parseGPT(s)
	default:
		return raw
	}
}

func parseGemini(s string) string {
	// Extract version: gemini-3, gemini-2.5, gemini-1.5
	var version, variant string
	for _, v := range []string{"3.1", "3", "2.5", "2.0", "1.5", "1.0"} {
		if strings.Contains(s, "gemini-"+v) || strings.Contains(s, "gemini_"+v) {
			version = v
			break
		}
	}
	if version == "" {
		return "Gemini"
	}

	// Detect variant
	switch {
	case strings.Contains(s, "ultra"):
		variant = "Ultra"
	case strings.Contains(s, "pro"):
		variant = "Pro"
	case strings.Contains(s, "flash-lite") || strings.Contains(s, "flash_lite"):
		variant = "Flash Lite"
	case strings.Contains(s, "flash"):
		variant = "Flash"
	case strings.Contains(s, "nano"):
		variant = "Nano"
	}

	name := "Gemini " + version
	if variant != "" {
		name += " (" + variant + ")"
	}
	return name
}

func parseClaude(s string) string {
	// claude-opus-4-6 → Opus 4.6, claude-sonnet-4-5 → Sonnet 4.5
	var family, version string
	for _, f := range []string{"opus", "sonnet", "haiku"} {
		if strings.Contains(s, f) {
			family = strings.Title(f)
			break
		}
	}
	if family == "" {
		return "Claude"
	}

	// Find version digits after the family name
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if p == strings.ToLower(family) && i+1 < len(parts) {
			major := parts[i+1]
			if i+2 < len(parts) {
				minor := parts[i+2]
				// Skip non-numeric suffixes like "20251001"
				if len(minor) <= 2 {
					version = major + "." + minor
				} else {
					version = major
				}
			} else {
				version = major
			}
			break
		}
	}

	if version != "" {
		return family + " " + version
	}
	return family
}

func parseGPT(s string) string {
	switch {
	case strings.Contains(s, "gpt-4o-mini"):
		return "GPT-4o Mini"
	case strings.Contains(s, "gpt-4o"):
		return "GPT-4o"
	case strings.Contains(s, "gpt-4-turbo"):
		return "GPT-4 Turbo"
	case strings.Contains(s, "gpt-4"):
		return "GPT-4"
	case strings.Contains(s, "gpt-3.5"):
		return "GPT-3.5"
	default:
		return strings.ToUpper(s[:3]) + s[3:]
	}
}

// contextWindowSize returns the context window size for known models.
func contextWindowSize(model string) int32 {
	s := strings.ToLower(model)
	switch {
	// Gemini
	case strings.Contains(s, "gemini-1.5-pro"):
		return 2_000_000
	case strings.Contains(s, "gemini"):
		return 1_000_000
	// Claude
	case strings.Contains(s, "claude"), strings.Contains(s, "opus"),
		strings.Contains(s, "sonnet"), strings.Contains(s, "haiku"):
		return 200_000
	// OpenAI
	case strings.Contains(s, "gpt-4o"), strings.Contains(s, "gpt-4-turbo"):
		return 128_000
	case strings.Contains(s, "o3"), strings.Contains(s, "o1"):
		return 200_000
	default:
		return 200_000 // safe default
	}
}

// NewStatusModel creates a new status bar.
func NewStatusModel(modelName string, mode permission.Mode) StatusModel {
	return StatusModel{
		modelName: modelName,
		mode:      mode,
	}
}

// SetMode updates the displayed permission mode.
func (s *StatusModel) SetMode(mode permission.Mode) {
	s.mode = mode
}

// SetModel updates the displayed model name.
func (s *StatusModel) SetModel(name string) {
	s.modelName = name
}

// SetToolName sets the currently executing tool name (empty string to clear).
func (s *StatusModel) SetToolName(name string) {
	s.toolName = name
}

// SetMessage sets a transient status message that auto-expires after 3 seconds.
func (s *StatusModel) SetMessage(msg string) {
	s.message = msg
	s.messageSetAt = time.Now()
}

// SetWidth updates the status bar width.
func (s *StatusModel) SetWidth(width int) {
	s.width = width
}

// SetGitBranch sets the git branch display.
func (s *StatusModel) SetGitBranch(branch string) {
	s.gitBranch = branch
}

// SetCwd sets the working directory display.
func (s *StatusModel) SetCwd(cwd string) {
	s.cwd = cwd
}

// SetCost sets the running cost display.
func (s *StatusModel) SetCost(cost float64) {
	s.cost = cost
}

// SetDailyBudget sets the daily budget for cost warning display.
func (s *StatusModel) SetDailyBudget(budget float64) {
	s.dailyBudget = budget
}

// SetPendingApproval sets whether an approval is pending.
func (s *StatusModel) SetPendingApproval(pending bool) {
	s.pendingApproval = pending
}

// AddTokens accumulates token usage from a stream completion.
// lastPrompt is used as-is for context bar (it reflects current context size).
func (s *StatusModel) AddTokens(prompt, completion, total int32) {
	s.promptTokens += prompt
	s.completionTokens += completion
	s.totalTokens += total
	s.lastPromptTokens = prompt // Current context window usage
}

// formatTokens returns a human-readable token count (e.g., "1.2k", "15.4k").
func formatTokens(n int32) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return fmt.Sprintf("%.1fk", float64(n)/1000)
}

// View renders the status bar.
// Layout: model  MODE  [status]  [tokens]  cwd  branch
func (s StatusModel) View() string {
	w := s.width
	if w <= 0 {
		w = 80
	}

	// Build segments
	model := StatusModelStyle.Render(friendlyModelName(s.modelName))
	mode := ModeBadge(s.mode)

	// Middle: approval, tool, or cost status
	var middle string
	if s.pendingApproval {
		middle = lipgloss.NewStyle().Foreground(warningColor).Render("● awaiting approval")
	} else if s.toolName != "" {
		middle = ToolBulletStyle.Render("~") + " " + s.toolName
	} else if s.message != "" && time.Since(s.messageSetAt) < 3*time.Second {
		middle = s.message
	} else if s.cost > 0 {
		costStr := fmt.Sprintf("$%.2f", s.cost)
		if s.dailyBudget > 0 && s.cost/s.dailyBudget >= 0.80 {
			// Budget warning: amber color with percentage
			pct := int(s.cost / s.dailyBudget * 100)
			middle = lipgloss.NewStyle().Foreground(warningColor).Render(
				fmt.Sprintf("%s (%d%% budget)", costStr, pct))
		} else {
			middle = StatusDimStyle.Render(costStr)
		}
	}

	// Context usage bar (no token counts — those are shown per-turn in spinner)
	var context string
	if s.totalTokens > 0 {
		ctxSize := contextWindowSize(s.modelName)
		pct := float64(s.lastPromptTokens) / float64(ctxSize) * 100
		bar := renderContextBar(pct, 5)
		pctStr := fmt.Sprintf("%d%%", int(pct))
		if pct > 0 && pct < 1 {
			pctStr = "<1%"
		}
		context = bar + " " + StatusDimStyle.Render(pctStr)
	}

	// Right side: cwd + git branch (responsive)
	var right string
	if w >= 80 && s.cwd != "" {
		right = StatusDimStyle.Render(s.cwd)
	}
	if w >= 60 && s.gitBranch != "" {
		if right != "" {
			right += "  "
		}
		right += StatusDimStyle.Render(" " + s.gitBranch)
	}

	// Assemble with spacing, 2-space left pad to align with input box
	parts := []string{model, mode}
	if middle != "" {
		parts = append(parts, middle)
	}
	if context != "" {
		parts = append(parts, context)
	}
	if right != "" {
		parts = append(parts, right)
	}

	content := "  " + strings.Join(parts, "   ")
	contentWidth := lipgloss.Width(content)

	// Pad to fill width
	if contentWidth < w {
		content += strings.Repeat(" ", w-contentWidth)
	}

	return StatusBarStyle.Width(w).Render(content)
}

// renderContextBar draws a compact progress bar using block characters.
// width is the number of cells for the bar (e.g., 5).
func renderContextBar(pct float64, width int) string {
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}

	// Color shifts: green → yellow → red as context fills
	var filledColor, emptyColor lipgloss.Style
	switch {
	case pct < 50:
		filledColor = lipgloss.NewStyle().Foreground(successColor)
	case pct < 80:
		filledColor = lipgloss.NewStyle().Foreground(warningColor)
	default:
		filledColor = lipgloss.NewStyle().Foreground(dangerColor)
	}
	emptyColor = lipgloss.NewStyle().Foreground(inputBorderColor) // Gray-700 — visible on dark bar bg

	bar := filledColor.Render(strings.Repeat("█", filled)) +
		emptyColor.Render(strings.Repeat("░", width-filled))
	return bar
}

// DetectGitBranch runs git to find the current branch name.
func DetectGitBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ShortenPath replaces $HOME with ~ and truncates the middle if too long.
func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + path[len(home):]
	}
	// Truncate if too long
	if len(path) > 30 {
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		if len(base) > 25 {
			return ".../" + base[:22] + "..."
		}
		remaining := 30 - len(base) - 4 // ".../" prefix
		if remaining > 0 && len(dir) > remaining {
			path = ".../" + dir[len(dir)-remaining:] + "/" + base
		}
	}
	return path
}
