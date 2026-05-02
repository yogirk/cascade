// Package render holds engine-agnostic rendering primitives shared across
// tool packages (BigQuery, DuckDB, …). Contains the cascade-styled table
// builder, palette accessors that follow the active theme, render-width
// state, and generic byte/duration/row-count formatters.
//
// Engine-specific renderers (cost footers, optimization hints, etc.)
// stay in their respective tool packages.
package render

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/slokam-ai/cascade/internal/tui/themes"
)

// Palette accessors. Built per-call from the live palette so renderers
// follow the active theme — package-level `var` would freeze on whatever
// theme was active at package-init time.
func AccentColor() color.Color    { return themes.ActivePalette().Accent }
func BrightColor() color.Color    { return themes.ActivePalette().Bright }
func DimColor() color.Color       { return themes.ActivePalette().DimText }
func TextColor() color.Color      { return themes.ActivePalette().Text }
func WarningColor() color.Color   { return themes.ActivePalette().Warning }
func SeparatorColor() color.Color { return themes.ActivePalette().InputBorderDim }

func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(AccentColor()).Bold(true)
}
func DimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(DimColor())
}
func BrightStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(BrightColor()).Bold(true)
}
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(WarningColor()).Bold(true)
}

// renderWidth is the available width in cells for tool-output tables.
// Set by the TUI on window resize via SetRenderWidth; falls back to 120
// for tests and one-shot mode where no terminal is attached.
var renderWidth = 120

// SetRenderWidth updates the width budget used by CascadeTable. Called
// from the TUI layout pass on WindowSizeMsg. Safe to call repeatedly.
// Width <= 0 is ignored.
func SetRenderWidth(w int) {
	if w > 0 {
		renderWidth = w
	}
}

// RenderWidth returns the current width budget.
func RenderWidth() int {
	return renderWidth
}

// CascadeTable builds a bordered table that shrink-wraps to its content,
// with column separators and a header rule. Rounded corners for a soft,
// modern look. Alternating row dimming preserved for scanability.
//
// We deliberately do not call .Width(renderWidth): forcing fill-to-width
// distributes slack across columns and produces a stretched, hard-to-scan
// table for short content.
func CascadeTable(headers []string) *table.Table {
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		BorderColumn(true).
		BorderRow(false).
		BorderHeader(true).
		BorderStyle(lipgloss.NewStyle().Foreground(SeparatorColor())).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Foreground(AccentColor()).
					Bold(true).
					Padding(0, 1)
			}
			s := lipgloss.NewStyle().Padding(0, 1)
			if row%2 == 0 {
				return s.Foreground(TextColor())
			}
			return s.Foreground(DimColor())
		}).
		Wrap(false)
}

// FormatBytes formats byte count as human-readable (KB, MB, GB, TB).
func FormatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case bytes < kb:
		return fmt.Sprintf("%d B", bytes)
	case bytes < mb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	case bytes < gb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes < tb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	default:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	}
}

// FormatDuration formats milliseconds as "Xms", "X.Xs", or "Xm Xs".
func FormatDuration(ms int64) string {
	switch {
	case ms < 1000:
		return fmt.Sprintf("%dms", ms)
	case ms < 60000:
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	default:
		m := ms / 60000
		s := (ms % 60000) / 1000
		return fmt.Sprintf("%dm %ds", m, s)
	}
}

// FormatRowCount formats a row count with comma separators.
func FormatRowCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var b strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		b.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
