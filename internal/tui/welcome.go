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
// cascadeBg1-4) are defined in styles.go and initialized adaptively
// for light/dark terminals via initPalette().

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
