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
	width    int
	height   int
}

// NewWelcomeModel creates a new welcome banner.
func NewWelcomeModel(mode permission.Mode, project string, datasets []string) WelcomeModel {
	return WelcomeModel{
		mode:     mode,
		project:  project,
		datasets: datasets,
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

// -- Cascade Logo --

// Cascade ocean palette — four stages, deepest first.
var (
	cascadeBg1 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#0C4A6E"), lipgloss.Color("#0369A1")))
	cascadeBg2 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#0369A1"), lipgloss.Color("#0EA5E9")))
	cascadeBg3 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#0EA5E9"), lipgloss.Color("#38BDF8")))
	cascadeBg4 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#38BDF8"), lipgloss.Color("#7DD3FC")))
)

// renderCascadeLogo renders the Cascade logo — four bars stepping
// right, each a pipeline stage.
func renderCascadeLogo() string {
	const px = " " // 1-space pixel unit

	b1 := cascadeBg1.Render
	b2 := cascadeBg2.Render
	b3 := cascadeBg3.Render

	bar := func(render func(...string) string, n int) string {
		return render(strings.Repeat(px, n))
	}
	pad := func(n int) string { return strings.Repeat(px, n) }

	lines := []string{
		pad(0) + bar(b1, 5),
		pad(1) + bar(b2, 5),
		pad(2) + bar(b3, 5),
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
	leftW := totalW * 20 / 100
	rightW := totalW - leftW

	leftStyle := lipgloss.NewStyle().
		Width(leftW).
		Padding(1, 2, 1, 4)

	leftPanel := leftStyle.Render(leftContent)

	// === Right panel: connection status ===
	var rightLines []string

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
	titleLabel := " " + bright.Render("Cascade") + dim.Render(" v0.1.0-dev") + " "
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

// padTo pads a string with spaces to a target width.
func padTo(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
