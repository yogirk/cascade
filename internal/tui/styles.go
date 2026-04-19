// Package tui implements the Bubble Tea terminal UI for Cascade.
package tui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/tui/themes"
)

// isDarkBg indicates whether the terminal has a dark background.
// Set by auto-detection at init; overridable via SetTheme("light"|"dark").
var isDarkBg bool

// currentTheme is the active named theme. Changes via SetTheme(<theme-name>)
// or the /theme slash command. Defaults to the registry's default theme.
var currentTheme themes.Theme = themes.Default()

func init() {
	isDarkBg = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	initPalette()
}

// SetTheme updates the active theme and/or lightness and re-applies the
// palette. Accepted values:
//
//   - "light" / "dark" — force lightness, keep current theme (back-compat)
//   - "auto" / ""      — use auto-detected lightness, keep current theme
//   - any registered theme name (e.g. "verse-in-code") — switch theme,
//     keep auto-detected lightness
//
// Unknown values are ignored (no change). Must be called before any
// rendering (typically from app startup) — calls initPalette() which
// rebuilds all module-level styles.
func SetTheme(theme string) {
	switch theme {
	case "light":
		isDarkBg = false
	case "dark":
		isDarkBg = true
	case "auto", "":
		// Keep auto-detected lightness value.
	default:
		if t, ok := themes.Get(theme); ok {
			currentTheme = t
		} else {
			return // Unknown — skip re-init.
		}
	}
	initPalette()
}

// CurrentTheme returns the currently active theme. Used by the TUI /theme
// picker to highlight the selected entry.
func CurrentTheme() themes.Theme {
	return currentTheme
}

// IsDarkBg reports whether the rendered variant is the dark palette.
// Used by the TUI /theme picker to show which lightness is in effect.
func IsDarkBg() bool {
	return isDarkBg
}

// Color palette — adaptive for light and dark terminals.
// Dark variants (second arg) are the original colors; light variants are
// deeper/darker shades that read well on white backgrounds.
var (
	accentColor  color.Color
	dimTextColor color.Color
	textColor    color.Color
	brightColor  color.Color
	successColor color.Color
	warningColor color.Color
	dangerColor  color.Color
	toolColor    color.Color
	planColor    color.Color

	diffAddBg color.Color
	diffAddFg color.Color
	diffRemBg color.Color
	diffRemFg color.Color

	inputBorderColor    color.Color
	inputBorderDimColor color.Color
	inputBgColor        color.Color

	settledAccent color.Color
)

// Spinner palette — adaptive for light and dark terminals.
var (
	// Sweep palette — dim to bright for the character spotlight effect.
	sweepDim    color.Color
	sweepMid    color.Color
	sweepBright color.Color

	// Cascade tilde colors — ocean blue palette matching the logo.
	cascadeDim    color.Color
	cascadeTrail  color.Color
	cascadeBright color.Color
	cascadePeak   color.Color
)

// Message rendering styles.
var (
	UserPromptStyle      lipgloss.Style
	UserMessageBarStyle  lipgloss.Style
	AssistantBulletStyle lipgloss.Style
	ToolBulletStyle      lipgloss.Style
	ToolBulletReadStyle  lipgloss.Style
	ToolBulletWriteStyle lipgloss.Style
	ToolBulletExecStyle  lipgloss.Style
	ToolBulletQueryStyle lipgloss.Style
	ToolBulletDataStyle  lipgloss.Style
	ToolNameStyle        lipgloss.Style
	ToolOutputStyle      lipgloss.Style
	ToolErrorStyle       lipgloss.Style
	DiffAddStyle         lipgloss.Style
	DiffRemoveStyle      lipgloss.Style
	DiffHunkStyle        lipgloss.Style
	SystemMsgStyle       lipgloss.Style
	ErrorPrefixStyle     lipgloss.Style
	StatusBarStyle       lipgloss.Style
	StatusModelStyle     lipgloss.Style
	StatusDimStyle       lipgloss.Style
	WelcomeTitleStyle    lipgloss.Style
	WelcomeDetailStyle   lipgloss.Style
	SeparatorStyle       lipgloss.Style
	ConfirmBoxStyle      lipgloss.Style

	// BigQuery-specific styles
	QueryTableHeaderStyle lipgloss.Style
	QueryTableCellStyle   lipgloss.Style
	QueryTableNullStyle   lipgloss.Style
	CostSafeStyle         lipgloss.Style
	CostWarnStyle         lipgloss.Style
	CostDangerStyle       lipgloss.Style
	SchemaHeaderStyle     lipgloss.Style
	SchemaAnnotationStyle lipgloss.Style
)

// Model picker styles.
var (
	mpAccentStyle lipgloss.Style
	mpNameStyle   lipgloss.Style
	mpIDStyle     lipgloss.Style
	mpNoteStyle   lipgloss.Style
	mpDimStyle    lipgloss.Style
	mpCurStyle    lipgloss.Style
	mpProvStyle   lipgloss.Style
)

// Welcome banner styles.
var (
	gBright lipgloss.Style
	gDim    lipgloss.Style

	cascadeBg1 lipgloss.Style
	cascadeBg2 lipgloss.Style
	cascadeBg3 lipgloss.Style
)

// Pre-computed badge strings (avoid allocating styles on every render).
var (
	riskReadBadge        string
	riskDMLBadge         string
	riskDDLBadge         string
	riskDestructiveBadge string

	modeAskBadge        string
	modeReadOnlyBadge   string
	modeFullAccessBadge string
)

// initPalette initializes all adaptive colors, styles, and badges from
// currentTheme + isDarkBg. Called at init and after SetTheme().
//
// All hex values live in internal/tui/themes/<theme>.go. This function is
// deliberately just field-to-var wiring — do not reintroduce hex literals
// here without a very good reason.
func initPalette() {
	p := currentTheme.Pick(isDarkBg)

	// ── Core colors ──
	accentColor = p.Accent
	dimTextColor = p.DimText
	textColor = p.Text
	brightColor = p.Bright
	successColor = p.Success
	warningColor = p.Warning
	dangerColor = p.Danger
	toolColor = p.Tool
	planColor = p.Plan

	// ── Diff colors ──
	diffAddBg = p.DiffAddBg
	diffAddFg = p.DiffAddFg
	diffRemBg = p.DiffRemBg
	diffRemFg = p.DiffRemFg

	// ── Input colors ──
	inputBorderColor = p.InputBorder
	inputBorderDimColor = p.InputBorderDim
	inputBgColor = p.InputBg

	// Muted accent for submitted questions — distinct per theme, dialled back
	// so the active input box feels "live" and past questions feel "settled."
	settledAccent = p.SettledAccent

	// ── Spinner palette ──
	// Sweep: spotlight moves across status text.
	sweepDim = p.SweepDim
	sweepMid = p.SweepMid
	sweepBright = p.SweepBright

	// Cascade tilde: pulsing palette (ocean blue in Midnight Hydrology,
	// warm chestnut in Verse in Code).
	cascadeDim = p.CascadeDim
	cascadeTrail = p.CascadeTrail
	cascadeBright = p.CascadeBright
	cascadePeak = p.CascadePeak

	// ── Message styles ──

	// UserPromptStyle styles the "> " prefix for user messages.
	UserPromptStyle = lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true)

	// UserMessageBarStyle renders submitted user messages to match the input box styling.
	UserMessageBarStyle = lipgloss.NewStyle().
		Background(inputBgColor).
		Foreground(brightColor).
		PaddingTop(1).
		PaddingBottom(1).
		PaddingLeft(2).
		PaddingRight(2).
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeftForeground(settledAccent)

	// AssistantBulletStyle styles the "≋" prefix for assistant messages.
	AssistantBulletStyle = lipgloss.NewStyle().
		Foreground(accentColor)

	// ToolBulletStyle styles the default "≋" character for tool messages.
	ToolBulletStyle = lipgloss.NewStyle().
		Foreground(toolColor)

	// ToolBulletReadStyle styles the "≋" for read-only tools (green).
	ToolBulletReadStyle = lipgloss.NewStyle().
		Foreground(successColor)

	// ToolBulletWriteStyle styles the "≋" for write/edit tools (amber).
	ToolBulletWriteStyle = lipgloss.NewStyle().
		Foreground(warningColor)

	// ToolBulletExecStyle styles the "≋" for bash/exec tools (red).
	ToolBulletExecStyle = lipgloss.NewStyle().
		Foreground(dangerColor)

	// ToolBulletQueryStyle styles the "~" for query tools like bigquery_query (cyan).
	ToolBulletQueryStyle = lipgloss.NewStyle().
		Foreground(toolColor)

	// ToolBulletDataStyle styles the "~" for platform data tools like cloud_logging, gcs (indigo).
	ToolBulletDataStyle = lipgloss.NewStyle().
		Foreground(planColor)

	// ToolNameStyle styles the tool name (dim, not bold — subordinate to assistant narrative).
	ToolNameStyle = lipgloss.NewStyle().Foreground(dimTextColor)

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
		Foreground(textColor)

	// StatusModelStyle styles the model name in the status bar.
	StatusModelStyle = lipgloss.NewStyle().
		Foreground(brightColor)

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

	// ConfirmBoxStyle wraps the permission confirmation prompt with a left accent.
	ConfirmBoxStyle = lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(warningColor).
		PaddingLeft(1)

	// ── BigQuery styles ──

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

	// ── Model picker styles ──
	mpAccentStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	mpNameStyle = lipgloss.NewStyle().Foreground(brightColor)
	mpIDStyle = lipgloss.NewStyle().Foreground(dimTextColor)
	mpNoteStyle = lipgloss.NewStyle().Foreground(dimTextColor)
	mpDimStyle = lipgloss.NewStyle().Foreground(dimTextColor)
	mpCurStyle = lipgloss.NewStyle().Foreground(successColor)
	mpProvStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)

	// ── Welcome banner styles ──
	gBright = lipgloss.NewStyle().Foreground(brightColor)
	gDim = lipgloss.NewStyle().Foreground(dimTextColor)

	cascadeBg1 = lipgloss.NewStyle().Background(p.CascadeBg1)
	cascadeBg2 = lipgloss.NewStyle().Background(p.CascadeBg2)
	cascadeBg3 = lipgloss.NewStyle().Background(p.CascadeBg3)

	// ── Pre-computed badges ──
	riskReadBadge = lipgloss.NewStyle().Foreground(successColor).Render("[READ]")
	riskDMLBadge = lipgloss.NewStyle().Foreground(warningColor).Render("[DML]")
	riskDDLBadge = lipgloss.NewStyle().Foreground(warningColor).Render("[DDL]")
	riskDestructiveBadge = lipgloss.NewStyle().Foreground(dangerColor).Render("[DESTRUCTIVE]")

	modeAskBadge = lipgloss.NewStyle().Foreground(successColor).Bold(true).Render("ASK")
	modeReadOnlyBadge = lipgloss.NewStyle().Foreground(planColor).Bold(true).Render("READ ONLY")
	modeFullAccessBadge = lipgloss.NewStyle().Foreground(dangerColor).Bold(true).Render("FULL ACCESS")
}

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

// ToolBulletByRisk returns a colored, shape-differentiated bullet.
//
//	Green  ○ — read-only (grep, glob, read)
//	Amber  ◇ — write/modify (write_file, edit_file)
//	Red    ● — execute/risky (bash)
//	Cyan   △ — query/cost-bearing (bigquery_query)
//	Indigo □ — platform data (cloud_logging, gcs)
//
// Shape encodes the action category (accessible without color).
// Color encodes the risk level (redundant with shape for sighted users).
// When riskLevel is provided, it determines the glyph shape.
// Special tool names (bigquery_query, cloud_logging, gcs) override to
// category-specific shapes regardless of risk level.
func ToolBulletByRisk(toolName, riskLevel string) string {
	// Special categories override risk-based shape
	switch toolName {
	case "bigquery_query":
		return ToolBulletQueryStyle.Render("△")
	case "cloud_logging", "gcs":
		return ToolBulletDataStyle.Render("□")
	}

	// Risk-based shape + color
	switch riskLevel {
	case "READ_ONLY":
		return ToolBulletReadStyle.Render("○")
	case "DML":
		return ToolBulletWriteStyle.Render("◇")
	case "DDL":
		return ToolBulletWriteStyle.Render("◇")
	case "DESTRUCTIVE":
		return ToolBulletExecStyle.Render("●")
	case "ADMIN":
		return ToolBulletExecStyle.Render("●")
	default:
		// Fallback: name-based classification when risk level unavailable
		switch toolName {
		case "grep", "glob", "read_file", "bigquery_schema":
			return ToolBulletReadStyle.Render("○")
		case "write_file", "edit_file":
			return ToolBulletWriteStyle.Render("◇")
		case "bash":
			return ToolBulletExecStyle.Render("●")
		default:
			return ToolBulletStyle.Render("○")
		}
	}
}

// ModeBadge returns a styled mode string (foreground-only, no background).
func ModeBadge(mode permission.Mode) string {
	switch mode {
	case permission.ModeAsk:
		return modeAskBadge
	case permission.ModeReadOnly:
		return modeReadOnlyBadge
	case permission.ModeFullAccess:
		return modeFullAccessBadge
	default:
		return modeAskBadge
	}
}
