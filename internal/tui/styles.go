// Package tui implements the Bubble Tea terminal UI for Cascade.
package tui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Color palette using lipgloss.Color which returns color.Color in v2.
var (
	subtleColor  color.Color = lipgloss.Color("#999999")
	accentColor  color.Color = lipgloss.Color("#AD8AFF")
	successColor color.Color = lipgloss.Color("#04B575")
	warningColor color.Color = lipgloss.Color("#FFA500")
	dangerColor  color.Color = lipgloss.Color("#FF4040")
	textColor    color.Color = lipgloss.Color("#FAFAFA")
	dimTextColor color.Color = lipgloss.Color("#666666")
	whiteColor   color.Color = lipgloss.Color("#FFFFFF")
	barBgColor   color.Color = lipgloss.Color("#333333")
	barFgColor   color.Color = lipgloss.Color("#E0E0E0")
	planBgColor  color.Color = lipgloss.Color("#5B9BD5")
)

// Message role styles.
var (
	UserStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	AssistantStyle = lipgloss.NewStyle().
			Foreground(textColor)

	ToolStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Italic(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			Bold(true)
)

// StatusBar styles.
var (
	StatusBarStyle = lipgloss.NewStyle().
			Background(barBgColor).
			Foreground(barFgColor).
			Padding(0, 1)

	StatusBarModelStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	StatusBarVersionStyle = lipgloss.NewStyle().
				Foreground(dimTextColor)
)

// Input styles.
var (
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtleColor).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor).
				Padding(0, 1)
)

// Confirm prompt styles.
var (
	ConfirmStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warningColor).
			Padding(0, 1)
)

// Risk level badges.
var (
	ReadBadge = lipgloss.NewStyle().
			Background(successColor).
			Foreground(whiteColor).
			Padding(0, 1).
			Bold(true).
			Render("[READ]")

	DMLBadge = lipgloss.NewStyle().
			Background(warningColor).
			Foreground(whiteColor).
			Padding(0, 1).
			Bold(true).
			Render("[DML]")

	DestructiveBadge = lipgloss.NewStyle().
				Background(dangerColor).
				Foreground(whiteColor).
				Padding(0, 1).
				Bold(true).
				Render("[DESTRUCTIVE]")
)

// Permission mode badges.
var (
	ConfirmModeBadge = lipgloss.NewStyle().
				Background(successColor).
				Foreground(whiteColor).
				Padding(0, 1).
				Bold(true).
				Render("CONFIRM")

	PlanModeBadge = lipgloss.NewStyle().
			Background(planBgColor).
			Foreground(whiteColor).
			Padding(0, 1).
			Bold(true).
			Render("PLAN")

	BypassModeBadge = lipgloss.NewStyle().
				Background(dangerColor).
				Foreground(whiteColor).
				Padding(0, 1).
				Bold(true).
				Render("BYPASS")
)

// RiskBadge returns the styled badge for a given risk level string.
func RiskBadge(riskLevel string) string {
	switch riskLevel {
	case "READ_ONLY":
		return ReadBadge
	case "DML":
		return DMLBadge
	case "DESTRUCTIVE", "DDL", "ADMIN":
		return DestructiveBadge
	default:
		return DMLBadge
	}
}
