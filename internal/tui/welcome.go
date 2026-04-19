package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/permission"
)

// WelcomeModel renders the welcome banner when the chat is empty.
type WelcomeModel struct {
	mode     permission.Mode
	project  string   // GCP project ID
	datasets []string // configured BQ datasets
	version  string   // app version
	authOK   bool     // GCP resource auth succeeded
	width    int
	height   int
}

// NewWelcomeModel creates a new welcome banner.
func NewWelcomeModel(mode permission.Mode, project string, datasets []string, version string) WelcomeModel {
	return WelcomeModel{
		mode:     mode,
		project:  project,
		datasets: datasets,
		version:  version,
		authOK:   true, // assume OK unless explicitly set
	}
}

// SetAuthOK sets whether GCP resource auth succeeded.
func (w *WelcomeModel) SetAuthOK(ok bool) {
	w.authOK = ok
}

// SetSize updates the welcome banner dimensions.
func (w *WelcomeModel) SetSize(width, height int) {
	w.width = width
	w.height = height
}

// Welcome banner styles (gBlue, gRed, gYellow, gGreen, gBright, gDim,
// cascadeBg1-3) are defined in styles.go and initialized adaptively
// for light/dark terminals via initPalette().

// renderCascadeLogo renders the Cascade logo — a 4×4 scatter of
// square pixels in the Slokam dialect: bright highlights on a muted
// field, theme-reactive via CascadeBg1 (muted) and CascadeBg3 (bright).
//
// Uses the ■ (U+25A0) glyph as the pixel unit with a 1-char horizontal
// gap between columns. Vertical row separation comes from the cell's
// natural font padding above/below the glyph.
func renderCascadeLogo() string {
	// 4×4 scatter — 1 = muted (CascadeBg1), 3 = bright (CascadeBg3).
	pattern := [4][4]int{
		{3, 1, 3, 3},
		{3, 3, 1, 3},
		{1, 3, 3, 1},
		{3, 3, 1, 3},
	}

	bright := lipgloss.NewStyle().Foreground(cascadeBg3Color)
	muted := lipgloss.NewStyle().Foreground(cascadeBg1Color)

	const dot = "■"
	const gap = " "

	lines := make([]string, 4)
	for r := 0; r < 4; r++ {
		var b strings.Builder
		for c := 0; c < 4; c++ {
			if c > 0 {
				b.WriteString(gap)
			}
			if pattern[r][c] == 3 {
				b.WriteString(bright.Render(dot))
			} else {
				b.WriteString(muted.Render(dot))
			}
		}
		lines[r] = b.String()
	}
	return strings.Join(lines, "\n")
}

// -- View --

// View renders the welcome screen using lipgloss box composition.
func (w WelcomeModel) View() string {
	bright := gBright.Bold(true)
	dim := gDim
	labelStyle := dim.Bold(true)

	// === Left panel: logo vertically centered ===
	leftContent := renderCascadeLogo()

	// Panel widths adapt to terminal
	totalW := w.width - 6
	if totalW < 60 {
		totalW = 60
	}
	if totalW > 90 {
		totalW = 90
	}
	// Fixed-width left panel sized for the 9-char scatter logo (5 dots + 4 gaps)
	// plus symmetric breathing room. Right panel takes the rest.
	leftW := 15
	rightW := totalW - leftW

	leftStyle := lipgloss.NewStyle().
		Width(leftW).
		Padding(1, 3, 1, 3)

	leftPanel := leftStyle.Render(leftContent)

	// === Right panel: connection status ===
	warnStyle := lipgloss.NewStyle().Foreground(warningColor)
	var rightLines []string

	if !w.authOK {
		rightLines = append(rightLines,
			warnStyle.Render("⚠ Not authenticated"))
		rightLines = append(rightLines, "")
	}

	if w.project != "" {
		rightLines = append(rightLines,
			labelStyle.Render("Project   ")+bright.Render(w.project))
	}

	if len(w.datasets) > 0 {
		dsText := strings.Join(w.datasets, ", ")
		rightLines = append(rightLines,
			labelStyle.Render("Datasets  ")+bright.Render(dsText))
	}

	rightLines = append(rightLines,
		labelStyle.Render("Mode      ")+ModeBadge(w.mode))

	rightLines = append(rightLines, "")
	if w.project == "" {
		rightLines = append(rightLines,
			dim.Render("Run cascade --project <id> to connect"))
	}
	rightLines = append(rightLines,
		dim.Render("Type a message to get started"))
	rightLines = append(rightLines,
		dim.Render(fmt.Sprintf("%-10s%s", "/help", "all commands")))

	rightContent := strings.Join(rightLines, "\n")

	rightStyle := lipgloss.NewStyle().
		Width(rightW).
		Padding(1, 2)

	rightPanel := rightStyle.Render(rightContent)

	// === Compose panels horizontally (Center aligns logo vertically with right content) ===
	body := lipgloss.JoinHorizontal(lipgloss.Center, leftPanel, rightPanel)
	bodyW := lipgloss.Width(body)

	// === Title line ===
	ver := w.version
	if ver == "" {
		ver = "dev"
	}
	titleLabel := " " + bright.Render("Cascade") + dim.Render(" v"+ver) + " "
	titleLabelW := lipgloss.Width(titleLabel)
	trailW := bodyW - titleLabelW - 3
	if trailW < 0 {
		trailW = 0
	}

	bc := lipgloss.NewStyle().Foreground(inputBorderColor)
	topLine := "  " + bc.Render("──") + titleLabel + bc.Render(strings.Repeat("─", trailW))

	// === Separator below welcome ===
	sepLine := "  " + bc.Render(strings.Repeat("─", bodyW))

	// === Indent body to align with input box (2-space indent) ===
	bodyLines := strings.Split(body, "\n")
	var indentedLines []string
	for _, line := range bodyLines {
		indentedLines = append(indentedLines, "  "+line)
	}

	var all []string
	all = append(all, topLine)
	all = append(all, indentedLines...)
	all = append(all, sepLine)

	content := strings.Join(all, "\n")

	// === Center vertically ===
	contentLines := strings.Count(content, "\n") + 1
	if w.height > contentLines+2 {
		topPad := (w.height - contentLines) / 3
		content = strings.Repeat("\n", topPad) + content
	}

	return content
}
