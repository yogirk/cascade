package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/permission"
)

// StatusModel renders the status bar at the bottom of the TUI.
type StatusModel struct {
	modelName        string
	mode             permission.Mode
	toolName         string // Non-empty when a tool is executing
	width            int
	message          string // Transient status message
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

// contextWindowSize returns the context window size for known models.
func contextWindowSize(model string) int32 {
	// Gemini models context windows
	switch {
	case strings.Contains(model, "gemini-2.5"), strings.Contains(model, "gemini-2.0"):
		return 1_000_000
	case strings.Contains(model, "gemini-1.5-pro"):
		return 2_000_000
	case strings.Contains(model, "gemini-1.5-flash"):
		return 1_000_000
	default:
		return 1_000_000 // safe default
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

// SetMessage sets a transient status message.
func (s *StatusModel) SetMessage(msg string) {
	s.message = msg
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
	model := StatusModelStyle.Render(s.modelName)
	mode := ModeBadge(s.mode)

	// Middle: approval, tool, or cost status
	var middle string
	if s.pendingApproval {
		middle = lipgloss.NewStyle().Foreground(warningColor).Render("● awaiting approval")
	} else if s.toolName != "" {
		middle = ToolBulletStyle.Render("∞") + " " + s.toolName
	} else if s.message != "" {
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

	// Context usage bar + token counts
	var context string
	if s.totalTokens > 0 {
		ctxSize := contextWindowSize(s.modelName)
		pct := float64(s.lastPromptTokens) / float64(ctxSize) * 100
		bar := renderContextBar(pct, 5)
		pctStr := fmt.Sprintf("%d%%", int(pct))
		if pct > 0 && pct < 1 {
			pctStr = "<1%"
		}
		context = bar + " " + StatusDimStyle.Render(pctStr) +
			"  " + StatusDimStyle.Render(fmt.Sprintf("↑%s ↓%s",
			formatTokens(s.promptTokens), formatTokens(s.completionTokens)))
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
		right += StatusDimStyle.Render(s.gitBranch)
	}

	// Assemble with spacing
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

	content := strings.Join(parts, "   ")
	contentWidth := lipgloss.Width(content)

	// Pad to fill width
	if contentWidth < w-2 {
		content += strings.Repeat(" ", w-2-contentWidth)
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
