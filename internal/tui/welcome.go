package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/slokam-ai/cascade/internal/permission"
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

// renderCascadeLogo renders the Cascade logo — three solid bars
// stepping diagonally down-right, each a brightness tier. The shape
// is what gave the project its name — water stepping down the
// cascade. Theme-reactive via cascadeBg1/2/3Color, so the gradient
// follows /theme.
//
// Visual:    █████
//             █████
//              █████
//
// Implementation note: each "block" is a SPACE character painted with
// a Background color, not a foreground full-block glyph. The two
// approaches look identical in well-behaved terminals, but full-block
// glyphs (█, U+2588) ship with subtle font-metric variation across
// terminals/fonts that shows up as alignment drift between rows. A
// background-painted space is monospaced by definition.
//
// Total footprint: 3 lines × 7 cells.
func renderCascadeLogo() string {
	const px = " " // 1-cell pixel unit
	bar := strings.Repeat(px, 5)

	bright := lipgloss.NewStyle().Background(cascadeBg3Color).Render(bar)
	mid := lipgloss.NewStyle().Background(cascadeBg2Color).Render(bar)
	dim := lipgloss.NewStyle().Background(cascadeBg1Color).Render(bar)

	return strings.Join([]string{
		bright,
		px + mid,
		px + px + dim,
	}, "\n")
}

// -- View --

// View renders the welcome screen using lipgloss box composition.
func (w WelcomeModel) View() string {
	bright := gBright.Bold(true)
	dim := gDim
	labelStyle := dim.Bold(true)

	// === Logo content (rendering happens after we know right-panel height) ===
	leftContent := renderCascadeLogo()

	// Panel widths adapt to terminal
	totalW := w.width - 6
	if totalW < 60 {
		totalW = 60
	}
	if totalW > 90 {
		totalW = 90
	}
	// Fixed-width left panel sized for the 14-cell rubik-cube grid logo
	// (4 tiles × 2 cells + 3 gaps × 2 cells) plus 3 cells of breathing
	// room on each side. Right panel takes the rest.
	leftW := 20
	rightW := totalW - leftW

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

	// === Left panel: match the right panel's height and center the logo
	// vertically inside it. This keeps the logo visually anchored at the
	// midpoint of the welcome banner regardless of how many rows the right
	// panel ends up needing (auth warning, dataset list, etc.).
	//
	// Note: AlignHorizontal is intentionally LEFT (the default). The
	// staircase logo's whole point is unequal row widths (5, 6, 7 cells);
	// centering each row independently would collapse those offsets and
	// destroy the diagonal. ===
	leftStyle := lipgloss.NewStyle().
		Width(leftW).
		Height(lipgloss.Height(rightPanel)).
		Padding(0, 3).
		AlignVertical(lipgloss.Center)

	leftPanel := leftStyle.Render(leftContent)

	// === Compose panels horizontally — Top is fine here because the left
	// panel was sized to match right; vertical centering happens inside
	// the left box, not at the join. ===
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
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
