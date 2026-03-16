package platform

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// Styles for morning report rendering.
var (
	morningHeaderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B9FFF")).Bold(true)
	morningTitleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
	morningSepStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	morningDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	morningTextStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	morningAccentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8"))
	morningWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))
	morningCritStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)
	morningOKStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399")).Bold(true)
	morningLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
)

// RenderMorningReport produces a Lipgloss-styled display string and a plain-text
// content string for the LLM.
func RenderMorningReport(report *MorningReport) (display string, content string) {
	var db, cb strings.Builder

	// Header
	db.WriteString("\n")
	db.WriteString(morningHeaderStyle.Render("  ☀ Morning Briefing"))
	db.WriteString(morningDimStyle.Render(fmt.Sprintf("  (last %s)", formatDuration(report.Since))))
	db.WriteString("\n")
	db.WriteString(morningSepStyle.Render("  " + strings.Repeat("─", 60)))
	db.WriteString("\n\n")

	cb.WriteString(fmt.Sprintf("Morning Briefing (last %s)\n\n", formatDuration(report.Since)))

	// Source notes (warnings about unavailable sources)
	for _, note := range report.SourceNotes {
		db.WriteString(morningDimStyle.Render(fmt.Sprintf("  ⚠ %s", note)))
		db.WriteString("\n")
		cb.WriteString(fmt.Sprintf("  Note: %s\n", note))
	}
	if len(report.SourceNotes) > 0 {
		db.WriteString("\n")
		cb.WriteString("\n")
	}

	// All clear?
	if len(report.Incidents) == 0 {
		db.WriteString(morningOKStyle.Render("  ✓ All clear! No issues detected."))
		db.WriteString("\n\n")
		cb.WriteString("All clear! No issues detected.\n")
		return db.String(), cb.String()
	}

	// Summary line
	critCount, warnCount := countBySeverity(report.Incidents)
	summaryParts := []string{}
	if critCount > 0 {
		summaryParts = append(summaryParts, morningCritStyle.Render(fmt.Sprintf("%d critical", critCount)))
	}
	if warnCount > 0 {
		summaryParts = append(summaryParts, morningWarnStyle.Render(fmt.Sprintf("%d warning", warnCount)))
	}
	infoCount := len(report.Incidents) - critCount - warnCount
	if infoCount > 0 {
		summaryParts = append(summaryParts, morningDimStyle.Render(fmt.Sprintf("%d info", infoCount)))
	}
	db.WriteString(fmt.Sprintf("  %s need attention: %s\n\n",
		morningTitleStyle.Render(fmt.Sprintf("%d things", len(report.Incidents))),
		strings.Join(summaryParts, ", ")))

	cb.WriteString(fmt.Sprintf("%d incidents: %d critical, %d warning, %d info\n\n",
		len(report.Incidents), critCount, warnCount, infoCount))

	// Incidents
	for i, inc := range report.Incidents {
		if i >= 10 {
			remaining := len(report.Incidents) - 10
			db.WriteString(morningDimStyle.Render(fmt.Sprintf("  ... and %d more\n", remaining)))
			cb.WriteString(fmt.Sprintf("... and %d more incidents\n", remaining))
			break
		}

		renderIncident(&db, &cb, i+1, inc)
	}

	db.WriteString("\n")
	return db.String(), cb.String()
}

func renderIncident(db, cb *strings.Builder, num int, inc Incident) {
	// Severity badge
	var badge string
	switch inc.TopSignal.Severity {
	case SeverityCritical:
		badge = morningCritStyle.Render("CRITICAL")
	case SeverityWarning:
		badge = morningWarnStyle.Render("WARNING")
	default:
		badge = morningDimStyle.Render("INFO")
	}

	// Number + badge + summary
	db.WriteString(fmt.Sprintf("  %s  %s\n",
		morningAccentStyle.Render(fmt.Sprintf("#%d", num)),
		badge))
	db.WriteString(fmt.Sprintf("  %s\n", morningTitleStyle.Render(inc.TopSignal.Summary)))

	cb.WriteString(fmt.Sprintf("#%d [%s] %s\n", num, inc.TopSignal.Severity, inc.TopSignal.Summary))

	// Resources
	if len(inc.Resources) > 0 {
		resourceStr := strings.Join(inc.Resources, ", ")
		if len(resourceStr) > 80 {
			resourceStr = resourceStr[:77] + "..."
		}
		db.WriteString(fmt.Sprintf("  %s %s\n",
			morningLabelStyle.Render("Resources:"),
			morningTextStyle.Render(resourceStr)))
		cb.WriteString(fmt.Sprintf("  Resources: %s\n", resourceStr))
	}

	// Blast radius
	if inc.BlastRadius > 0 {
		db.WriteString(fmt.Sprintf("  %s %s\n",
			morningLabelStyle.Render("Blast radius:"),
			morningTextStyle.Render(fmt.Sprintf("%d downstream tables", inc.BlastRadius))))
		cb.WriteString(fmt.Sprintf("  Blast radius: %d downstream tables\n", inc.BlastRadius))
	}

	// Additional signals in this incident
	if len(inc.Signals) > 1 {
		db.WriteString(fmt.Sprintf("  %s\n",
			morningDimStyle.Render(fmt.Sprintf("  + %d related signals", len(inc.Signals)-1))))
		cb.WriteString(fmt.Sprintf("  + %d related signals\n", len(inc.Signals)-1))
	}

	// Suggested action
	db.WriteString(fmt.Sprintf("  %s %s\n",
		morningLabelStyle.Render("Action:"),
		morningAccentStyle.Render(inc.SuggestedAction)))
	cb.WriteString(fmt.Sprintf("  Action: %s\n", inc.SuggestedAction))

	db.WriteString("\n")
	cb.WriteString("\n")
}

func countBySeverity(incidents []Incident) (critical, warning int) {
	for _, inc := range incidents {
		switch inc.TopSignal.Severity {
		case SeverityCritical:
			critical++
		case SeverityWarning:
			warning++
		}
	}
	return
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 {
		days := hours / 24
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dh", hours)
}
