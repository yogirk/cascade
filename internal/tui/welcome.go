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

// renderCascadeLogo renders the Cascade logo — a 4×3 grid (4 columns,
// 3 rows) of square tiles painted with three brightness tiers in the
// Slokam dialect, theme-reactive via cascadeBg1Color (dim),
// cascadeBg2Color (mid), and cascadeBg3Color (bright). The pattern
// favours brighter tiles on the upper-left and dimmer tiles on the
// lower-right, suggesting a cascade catching light at the lip and
// fading as the water disperses.
//
// Each tile is a 2-char-wide full-block pair (██) — terminal cells are
// roughly 1:2 (width:height), so two cells side-by-side approximate a
// square. A 2-cell horizontal gap matches the visual thickness of the
// 1-line vertical gap (since 1 line ≈ 2 cell-widths), giving the logo
// equal grooves in both axes. Total footprint: 5 lines × 14 cells →
// 14 × 10 cell-widths → landscape rectangle (wider than tall).
func renderCascadeLogo() string {
	// 4 cols × 3 rows scatter — values map to brightness tiers:
	//   1 = dim   (cascadeBg1Color)
	//   2 = mid   (cascadeBg2Color)
	//   3 = bright (cascadeBg3Color)
	// The diagonal bias (more 3s top-left, more 1s bottom-right) is what
	// gives the eye a sense of cascade direction.
	pattern := [3][4]int{
		{3, 3, 2, 1},
		{2, 3, 3, 2},
		{1, 2, 3, 3},
	}

	tier := [4]lipgloss.Style{
		{}, // index 0 unused — pattern values start at 1
		lipgloss.NewStyle().Foreground(cascadeBg1Color),
		lipgloss.NewStyle().Foreground(cascadeBg2Color),
		lipgloss.NewStyle().Foreground(cascadeBg3Color),
	}

	const tile = "██"
	const hgap = "  "

	rowLines := make([]string, 3)
	for r := 0; r < 3; r++ {
		var b strings.Builder
		for c := 0; c < 4; c++ {
			if c > 0 {
				b.WriteString(hgap)
			}
			b.WriteString(tier[pattern[r][c]].Render(tile))
		}
		rowLines[r] = b.String()
	}

	// Interleave a blank line between rows for the vertical "groove."
	out := make([]string, 0, 3*2-1)
	for i, line := range rowLines {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
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
	// panel ends up needing (auth warning, dataset list, etc.). ===
	leftStyle := lipgloss.NewStyle().
		Width(leftW).
		Height(lipgloss.Height(rightPanel)).
		Padding(0, 3).
		AlignVertical(lipgloss.Center).
		AlignHorizontal(lipgloss.Center)

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
