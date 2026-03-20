// Package tui implements the Bubble Tea terminal UI for Cascade.
package tui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/permission"
)

// ld chooses between light and dark color variants based on terminal background.
// Detection happens once at init; defaults to dark if detection fails.
var ld = lipgloss.LightDark(lipgloss.HasDarkBackground(os.Stdin, os.Stdout))

// Color palette — adaptive for light and dark terminals.
// Dark variants (second arg) are the original colors; light variants are
// deeper/darker shades that read well on white backgrounds.
var (
	accentColor  color.Color = ld(lipgloss.Color("#2563EB"), lipgloss.Color("#6B9FFF")) // Blue
	dimTextColor color.Color = ld(lipgloss.Color("#6B7280"), lipgloss.Color("#4B5563")) // Gray
	textColor    color.Color = ld(lipgloss.Color("#374151"), lipgloss.Color("#D1D5DB")) // Body text
	brightColor  color.Color = ld(lipgloss.Color("#111827"), lipgloss.Color("#F3F4F6")) // Headings
	successColor color.Color = ld(lipgloss.Color("#059669"), lipgloss.Color("#34D399")) // Green
	warningColor color.Color = ld(lipgloss.Color("#D97706"), lipgloss.Color("#FBBF24")) // Amber
	dangerColor  color.Color = ld(lipgloss.Color("#DC2626"), lipgloss.Color("#F87171")) // Red
	barBgColor   color.Color = ld(lipgloss.Color("#F3F4F6"), lipgloss.Color("#111827")) // Status bar bg
	toolColor    color.Color = ld(lipgloss.Color("#D97706"), lipgloss.Color("#FBBF24")) // Amber
	planColor    color.Color = ld(lipgloss.Color("#4F46E5"), lipgloss.Color("#818CF8")) // Indigo

	diffAddBg color.Color = ld(lipgloss.Color("#DCFCE7"), lipgloss.Color("#022c22")) // Diff + bg
	diffAddFg color.Color = ld(lipgloss.Color("#166534"), lipgloss.Color("#86efac")) // Diff + fg
	diffRemBg color.Color = ld(lipgloss.Color("#FEE2E2"), lipgloss.Color("#2a0a0a")) // Diff - bg
	diffRemFg color.Color = ld(lipgloss.Color("#991B1B"), lipgloss.Color("#fca5a5")) // Diff - fg

	inputBorderColor    color.Color = ld(lipgloss.Color("#D1D5DB"), lipgloss.Color("#374151")) // Border
	inputBorderDimColor color.Color = ld(lipgloss.Color("#E5E7EB"), lipgloss.Color("#1F2937")) // Border dim

	// Google brand colors — fixed, not theme-dependent.
	googleBlue   color.Color = lipgloss.Color("#4285F4")
	googleRed    color.Color = lipgloss.Color("#EA4335")
	googleYellow color.Color = lipgloss.Color("#FBBC05")
	googleGreen  color.Color = lipgloss.Color("#34A853")
)

// Message rendering styles.
var (
	// UserPromptStyle styles the "> " prefix for user messages.
	UserPromptStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	// AssistantBulletStyle styles the "∞" prefix for assistant messages.
	AssistantBulletStyle = lipgloss.NewStyle().
				Foreground(accentColor)

	// ToolBulletStyle styles the default "∞" character for tool messages.
	ToolBulletStyle = lipgloss.NewStyle().
			Foreground(toolColor)

	// ToolBulletReadStyle styles the "∞" for read-only tools (green).
	ToolBulletReadStyle = lipgloss.NewStyle().
				Foreground(successColor)

	// ToolBulletWriteStyle styles the "∞" for write/edit tools (amber).
	ToolBulletWriteStyle = lipgloss.NewStyle().
				Foreground(warningColor)

	// ToolBulletExecStyle styles the "∞" for bash/exec tools (red).
	ToolBulletExecStyle = lipgloss.NewStyle().
				Foreground(dangerColor)

	// ToolNameStyle styles the tool name (bold, default fg).
	ToolNameStyle = lipgloss.NewStyle().Bold(true)

	// ToolOutputStyle styles indented tool output (dim, 4-space left padding).
	ToolOutputStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			PaddingLeft(4)

	// ToolErrorStyle styles tool error output (red, 4-space left padding).
	ToolErrorStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			PaddingLeft(4)

	// DiffAddStyle styles added lines in diffs.
	DiffAddStyle = lipgloss.NewStyle().
			Foreground(diffAddFg).
			Background(diffAddBg)

	// DiffRemoveStyle styles removed lines in diffs.
	DiffRemoveStyle = lipgloss.NewStyle().
			Foreground(diffRemFg).
			Background(diffRemBg)

	// DiffHunkStyle styles @@ hunk headers.
	DiffHunkStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	// SystemMsgStyle styles system messages (dim, italic).
	SystemMsgStyle = lipgloss.NewStyle().
			Foreground(dimTextColor).
			Italic(true)

	// ErrorPrefixStyle styles the "!" error prefix.
	ErrorPrefixStyle = lipgloss.NewStyle().
				Foreground(dangerColor).
				Bold(true)

	// StatusBarStyle is the base style for the status bar.
	StatusBarStyle = lipgloss.NewStyle().
			Background(barBgColor).
			Foreground(textColor).
			Padding(0, 1)

	// StatusModelStyle styles the model name in the status bar.
	StatusModelStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	// StatusDimStyle styles dim text in the status bar.
	StatusDimStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	// WelcomeTitleStyle styles the welcome banner title.
	WelcomeTitleStyle = lipgloss.NewStyle().
				Foreground(brightColor).
				Bold(true)

	// WelcomeDetailStyle styles labels in the welcome banner.
	WelcomeDetailStyle = lipgloss.NewStyle().
				Foreground(dimTextColor)

	// SeparatorStyle styles the thin horizontal rule between turns.
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(inputBorderDimColor)

	// hintKeyStyle styles keyboard shortcut keys in the input hint.
	hintKeyStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	// hintTextStyle styles the description text in input hints.
	hintTextStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	// hintSepStyle styles the separator between hint items.
	hintSepStyle = lipgloss.NewStyle().
			Foreground(dimTextColor)

	// ConfirmBoxStyle wraps the permission confirmation prompt with a left accent.
	ConfirmBoxStyle = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(warningColor).
			PaddingLeft(1)

	// Phase 2: BigQuery-specific styles

	// QueryTableHeaderStyle styles column headers in query result tables.
	QueryTableHeaderStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	// QueryTableCellStyle styles data cells in query result tables.
	QueryTableCellStyle = lipgloss.NewStyle().
				Foreground(textColor)

	// QueryTableNullStyle styles NULL values in query result tables.
	QueryTableNullStyle = lipgloss.NewStyle().
				Foreground(dimTextColor).
				Italic(true)

	// CostSafeStyle styles cost amounts below the warn threshold.
	CostSafeStyle = lipgloss.NewStyle().
			Foreground(successColor)

	// CostWarnStyle styles cost amounts between warn and max thresholds.
	CostWarnStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	// CostDangerStyle styles cost amounts above the max threshold.
	CostDangerStyle = lipgloss.NewStyle().
			Foreground(dangerColor)

	// SchemaHeaderStyle styles table/dataset name headers in schema output.
	SchemaHeaderStyle = lipgloss.NewStyle().
				Foreground(brightColor).
				Bold(true)

	// SchemaAnnotationStyle styles partition/cluster annotations.
	SchemaAnnotationStyle = lipgloss.NewStyle().
				Foreground(accentColor)
)

// Pre-computed badge strings (avoid allocating styles on every render).
var (
	riskReadBadge        = lipgloss.NewStyle().Foreground(successColor).Render("[READ]")
	riskDMLBadge         = lipgloss.NewStyle().Foreground(warningColor).Render("[DML]")
	riskDDLBadge         = lipgloss.NewStyle().Foreground(warningColor).Render("[DDL]")
	riskDestructiveBadge = lipgloss.NewStyle().Foreground(dangerColor).Render("[DESTRUCTIVE]")

	modeConfirmBadge = lipgloss.NewStyle().Foreground(successColor).Bold(true).Render("CONFIRM")
	modePlanBadge    = lipgloss.NewStyle().Foreground(planColor).Bold(true).Render("PLAN")
	modeBypassBadge  = lipgloss.NewStyle().Foreground(dangerColor).Bold(true).Render("BYPASS")
)

// RiskBadge returns a styled inline risk badge like "[DML]" or "[DESTRUCTIVE]".
func RiskBadge(riskLevel string) string {
	switch riskLevel {
	case "READ_ONLY":
		return riskReadBadge
	case "DML":
		return riskDMLBadge
	case "DDL":
		return riskDDLBadge
	case "DESTRUCTIVE", "ADMIN":
		return riskDestructiveBadge
	default:
		return riskDMLBadge
	}
}

// ToolBullet returns a colored ∞ bullet based on the tool's category.
//
//	Green  — read-only (grep, glob, read)
//	Amber  — write/modify (write, edit)
//	Red    — execute/risky (bash)
func ToolBullet(toolName string) string {
	switch toolName {
	case "grep", "glob", "read":
		return ToolBulletReadStyle.Render("∞")
	case "write", "edit":
		return ToolBulletWriteStyle.Render("∞")
	case "bash":
		return ToolBulletExecStyle.Render("∞")
	case "bigquery_schema", "bigquery_cost":
		return ToolBulletReadStyle.Render("∞") // Green -- read-only
	case "bigquery_query":
		return ToolBulletStyle.Render("∞") // Amber -- default tool color (can write)
	default:
		return ToolBulletStyle.Render("∞")
	}
}

// ModeBadge returns a styled mode string (foreground-only, no background).
func ModeBadge(mode permission.Mode) string {
	switch mode {
	case permission.ModeConfirm:
		return modeConfirmBadge
	case permission.ModePlan:
		return modePlanBadge
	case permission.ModeBypass:
		return modeBypassBadge
	default:
		return modeConfirmBadge
	}
}
