package logging

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/slokam-ai/cascade/internal/tui/themes"
)

// severityColor maps a log severity string to a theme-aware color. Severity
// semantics are stable across themes (info=accent, warning=warning,
// error/critical=danger); only the actual hex value changes with the
// active palette.
func severityColor(sev string) color.Color {
	p := themes.ActivePalette()
	switch strings.ToUpper(sev) {
	case "DEBUG", "DEFAULT":
		return p.DimText
	case "INFO":
		return p.Accent
	case "NOTICE":
		return p.Tool
	case "WARNING":
		return p.Warning
	case "ERROR":
		return p.Danger
	case "CRITICAL", "ALERT", "EMERGENCY":
		return p.Danger
	default:
		return p.DimText
	}
}

// Styles for log rendering. Built per-call from the live palette.
func logDimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(themes.ActivePalette().DimText)
}
func logTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(themes.ActivePalette().Text)
}
func logHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(themes.ActivePalette().Accent).Bold(true)
}
func logSepStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(themes.ActivePalette().InputBorderDim)
}

func severityBadge(sev string) string {
	c := severityColor(sev)
	short := sev
	if len(short) > 5 {
		short = short[:5]
	}
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(fmt.Sprintf("%-5s", short))
}

// RenderLogEntries renders log entries with severity coloring and metadata.
func RenderLogEntries(entries []LogEntry, filter string, duration time.Duration) (display string, content string) {
	var db, cb strings.Builder

	// Header
	db.WriteString("\n  " + logHeaderStyle().Render("Cloud Logging") + "  " +
		logDimStyle().Render(fmt.Sprintf("%d entries · last %s", len(entries), formatLogDuration(duration))) + "\n")
	db.WriteString("  " + logSepStyle().Render(strings.Repeat("─", 60)) + "\n\n")

	cb.WriteString(fmt.Sprintf("Cloud Logging — %d entries (last %s)\n", len(entries), formatLogDuration(duration)))
	if filter != "" {
		cb.WriteString(fmt.Sprintf("Filter: %s\n", filter))
	}
	cb.WriteString("\n")

	if len(entries) == 0 {
		db.WriteString("  " + logDimStyle().Render("No log entries found matching the filter.") + "\n")
		cb.WriteString("No log entries found.\n")
		return db.String(), cb.String()
	}

	for i, entry := range entries {
		// Timestamp
		ts := entry.Timestamp.Local().Format("15:04:05")

		// Styled display
		badge := severityBadge(entry.Severity)
		resource := ""
		if entry.Resource != "" {
			resource = logDimStyle().Render("[" + entry.Resource + "]") + " "
		}

		// Message — already truncated by extractMessage, cap display further
		displayMsg := entry.Message
		if len(displayMsg) > 80 {
			displayMsg = displayMsg[:77] + "..."
		}

		db.WriteString(fmt.Sprintf("  %s  %s  %s%s\n",
			logDimStyle().Render(ts),
			badge,
			resource,
			logTextStyle().Render(displayMsg)))

		// Plain text
		cb.WriteString(fmt.Sprintf("[%s] %s %s %s: %s\n",
			entry.Timestamp.Format(time.RFC3339),
			entry.Severity,
			entry.Resource,
			entry.LogName,
			entry.Message))

		// Subtle separator between entries (not after last)
		if i < len(entries)-1 && entry.Severity != entries[i+1].Severity {
			db.WriteString("\n")
		}
	}

	db.WriteString("\n")
	return db.String(), cb.String()
}

func formatLogDuration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
