package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/permission"
)

// WelcomeModel renders the welcome banner when the chat is empty.
type WelcomeModel struct {
	modelName string
	mode      permission.Mode
	cwd       string
	gitBranch string
	width     int
	height    int
}

// NewWelcomeModel creates a new welcome banner.
func NewWelcomeModel(modelName string, mode permission.Mode, cwd, gitBranch string) WelcomeModel {
	return WelcomeModel{
		modelName: modelName,
		mode:      mode,
		cwd:       cwd,
		gitBranch: gitBranch,
	}
}

// SetSize updates the welcome banner dimensions.
func (w *WelcomeModel) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// -- Styles --

var (
	gBlue   = lipgloss.NewStyle().Foreground(googleBlue)
	gRed    = lipgloss.NewStyle().Foreground(googleRed)
	gYellow = lipgloss.NewStyle().Foreground(googleYellow)
	gGreen  = lipgloss.NewStyle().Foreground(googleGreen)
	gBright = lipgloss.NewStyle().Foreground(brightColor)
	gDim    = lipgloss.NewStyle().Foreground(dimTextColor)
)

// -- Lighthouse Beacon Mascot --

// renderLighthouse renders the lighthouse beacon pixel art mascot.
func renderLighthouse() string {
	Y := gYellow.Render
	W := gBright.Render
	R := gRed.Render
	G := gGreen.Render

	lines := []string{
		Y("─ ─") + "  " + Y("▄") + "  " + Y("─ ─"),
		"     " + W("▄█▄"),
		Y(" ─") + " " + Y("▄▀") + W("███") + Y("▀▄") + " " + Y("─"),
		"    " + W("█████"),
		"    " + R("█") + W("███") + R("█"),
		"    " + W("█") + R("███") + W("█"),
		"    " + R("█") + W("███") + R("█"),
		"   " + W("██") + R("███") + W("██"),
		"  " + G("▀████████▀"),
	}
	return strings.Join(lines, "\n")
}

// -- View --

// View renders the welcome screen using lipgloss box composition.
func (w WelcomeModel) View() string {
	bright := gBright.Bold(true)
	dim := gDim

	// === Left panel: mascot + info ===
	mascot := renderLighthouse()

	modelInfo := gBright.Render(w.modelName) + dim.Render(" · 1M · ") + ModeBadge(w.mode)

	leftContent := mascot + "\n\n" +
		" " + modelInfo + "\n"
	if w.cwd != "" {
		leftContent += " " + dim.Render(w.cwd) + "\n"
	}

	// Panel widths adapt to terminal
	totalW := w.width - 6 // account for margins and border
	if totalW < 60 {
		totalW = 60
	}
	if totalW > 90 {
		totalW = 90
	}
	leftW := totalW * 45 / 100
	rightW := totalW - leftW

	leftStyle := lipgloss.NewStyle().
		Width(leftW).
		Padding(1, 2)

	leftPanel := leftStyle.Render(leftContent)

	// === Right panel: quick start + shortcuts ===
	rightContent := gBlue.Bold(true).Render("Quick start") + "\n" +
		dim.Render("Type a message and press Enter") + "\n\n" +
		gYellow.Bold(true).Render("Shortcuts") + "\n"

	shortcuts := []struct{ key, desc string }{
		{"Enter", "send message"},
		{"Shift+Tab", "cycle mode"},
		{"Ctrl+Y", "copy response"},
		{"/help", "all commands"},
	}
	for _, sc := range shortcuts {
		rightContent += bright.Render(padTo(sc.key, 14)) + dim.Render(sc.desc) + "\n"
	}

	rightStyle := lipgloss.NewStyle().
		Width(rightW).
		Padding(1, 2).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(inputBorderColor)

	rightPanel := rightStyle.Render(rightContent)

	// === Compose panels horizontally ===
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	bodyLines := strings.Split(body, "\n")
	bodyW := lipgloss.Width(body)

	// === Build frame manually so title is embedded in top border ===
	bc := lipgloss.NewStyle().Foreground(inputBorderColor)

	// Top border with "Cascade v0.1.0-dev" embedded
	titleLabel := " " + bright.Render("Cascade") + dim.Render(" v0.1.0-dev") + " "
	titleLabelW := lipgloss.Width(titleLabel)
	trailW := bodyW - titleLabelW - 1 // -1 for leading ─
	if trailW < 0 {
		trailW = 0
	}
	topBorder := bc.Render("╭─") + titleLabel + bc.Render(strings.Repeat("─", trailW)+"╮")

	// Body rows with side borders
	var framedLines []string
	framedLines = append(framedLines, topBorder)
	for _, line := range bodyLines {
		lineW := lipgloss.Width(line)
		pad := bodyW - lineW
		if pad < 0 {
			pad = 0
		}
		framedLines = append(framedLines, bc.Render("│")+line+strings.Repeat(" ", pad)+bc.Render("│"))
	}

	// Bottom border
	botBorder := bc.Render("╰" + strings.Repeat("─", bodyW) + "╯")
	framedLines = append(framedLines, botBorder)

	content := strings.Join(framedLines, "\n")

	// === Center vertically ===
	contentLines := strings.Count(content, "\n") + 1
	if w.height > contentLines+2 {
		topPad := (w.height - contentLines) / 3
		content = strings.Repeat("\n", topPad) + content
	}

	return content
}

// padTo pads a string with spaces to a target width.
func padTo(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
