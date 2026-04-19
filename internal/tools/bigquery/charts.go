package bigquery

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/tui/themes"
)

// sparkBlocks are the Unicode block characters for sparkline rendering (8 levels).
var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// chartPalette maps a normalized value [0,1] to a color from the active
// theme's spinner/cascade ramp. Re-reads on every call so theme switches
// take effect immediately.
func chartColor(normalized float64) color.Color {
	p := themes.ActivePalette()
	switch {
	case normalized < 0.2:
		return p.CascadeDim
	case normalized < 0.4:
		return p.CascadeTrail
	case normalized < 0.6:
		// No distinct "mid" step in the palette — blend trail/bright territory.
		return p.CascadeTrail
	case normalized < 0.8:
		return p.CascadeBright
	default:
		return p.CascadePeak
	}
}

// RenderSparkline renders a sparkline from a slice of float64 values.
// Each value maps to one of 8 Unicode block characters, colored by intensity.
// Returns an empty string if data is empty.
func RenderSparkline(data []float64) string {
	if len(data) == 0 {
		return ""
	}

	minVal, maxVal := data[0], data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	spread := maxVal - minVal
	if spread == 0 {
		// All values equal — render mid-height bars
		mid := string(sparkBlocks[3])
		style := lipgloss.NewStyle().Foreground(themes.ActivePalette().CascadeBright)
		return style.Render(strings.Repeat(mid, len(data)))
	}

	var sb strings.Builder
	for _, v := range data {
		normalized := (v - minVal) / spread
		idx := int(normalized * float64(len(sparkBlocks)-1))
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		c := chartColor(normalized)
		sb.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(sparkBlocks[idx])))
	}
	return sb.String()
}

// SparklineWithAnnotation renders a sparkline with summary stats below.
// Format: ▂▃▅▇█▆▃  $42.17 total  |  avg $6.02/day  |  peak $12.30
func SparklineWithAnnotation(data []float64, label string, formatValue func(float64) string) string {
	if len(data) == 0 {
		return dimStyle().Render("No data")
	}

	spark := RenderSparkline(data)

	var total, peak float64
	for _, v := range data {
		total += v
		if v > peak {
			peak = v
		}
	}
	avg := total / float64(len(data))

	annotation := fmt.Sprintf("%s total  |  avg %s/day  |  peak %s",
		formatValue(total), formatValue(avg), formatValue(peak))

	return spark + "  " + dimStyle().Render(annotation)
}

// BarChartItem represents a single row in a horizontal bar chart.
type BarChartItem struct {
	Label          string
	Value          float64
	FormattedValue string // Pre-formatted display value (e.g., "$4.20")
}

// RenderBarChart renders a horizontal bar chart with labels, proportional bars, and values.
// maxBarWidth controls the maximum width of the bar portion (in characters).
func RenderBarChart(items []BarChartItem, maxBarWidth int) string {
	if len(items) == 0 {
		return dimStyle().Render("No data")
	}

	if maxBarWidth <= 0 {
		maxBarWidth = 30
	}

	// Find max value for proportional scaling
	maxVal := items[0].Value
	for _, item := range items {
		if item.Value > maxVal {
			maxVal = item.Value
		}
	}

	// Find max label width for alignment
	maxLabelW := 0
	maxValueW := 0
	for _, item := range items {
		if len(item.Label) > maxLabelW {
			maxLabelW = len(item.Label)
		}
		if len(item.FormattedValue) > maxValueW {
			maxValueW = len(item.FormattedValue)
		}
	}
	// Cap label width
	if maxLabelW > 25 {
		maxLabelW = 25
	}

	var sb strings.Builder
	for i, item := range items {
		// Truncate long labels
		label := item.Label
		if len(label) > maxLabelW {
			label = label[:maxLabelW-1] + "…"
		}

		// Proportional bar width
		var barW int
		if maxVal > 0 {
			barW = int(math.Round(item.Value / maxVal * float64(maxBarWidth)))
		}
		if barW < 1 && item.Value > 0 {
			barW = 1
		}

		// Color the bar based on relative value
		normalized := 0.0
		if maxVal > 0 {
			normalized = item.Value / maxVal
		}
		barColor := chartColor(normalized)
		bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", barW))
		padding := strings.Repeat(" ", maxBarWidth-barW)

		// Assemble line
		sb.WriteString(fmt.Sprintf("  %-*s  %s%s  %s",
			maxLabelW, label,
			bar, padding,
			dimStyle().Render(item.FormattedValue)))

		if i < len(items)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// FormatDollars formats a float as a dollar amount.
func FormatDollars(v float64) string {
	if v < 0.01 && v > 0 {
		return "<$0.01"
	}
	return fmt.Sprintf("$%.2f", v)
}

// FormatGB formats bytes as GB with one decimal.
func FormatGB(bytes float64) string {
	gb := bytes / math.Pow(1024, 3)
	if gb < 0.1 {
		return fmt.Sprintf("%.2f GB", gb)
	}
	return fmt.Sprintf("%.1f GB", gb)
}
