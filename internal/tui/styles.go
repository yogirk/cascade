// Package tui implements the Bubble Tea terminal UI for Cascade.
package tui

import (
	"image/color"
	"os"

	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/permission"
)

// isDarkBg indicates whether the terminal has a dark background.
// Set by auto-detection at init; overridable via SetTheme().
var isDarkBg bool

// ld chooses between light and dark color variants based on terminal background.
// Detection happens once at init; defaults to dark if detection fails.
var ld func(light, dark color.Color) color.Color

func init() {
	isDarkBg = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
	ld = lipgloss.LightDark(isDarkBg)
	initPalette()
}

// SetTheme overrides the auto-detected theme. Valid values: "light", "dark".
// "auto" (or any other value) keeps the detected value and is a no-op.
// Must be called before any rendering (typically from app startup).
func SetTheme(theme string) {
	switch theme {
	case "light":
		isDarkBg = false
	case "dark":
		isDarkBg = true
	default:
		return // "auto" — keep detected value
	}
	ld = lipgloss.LightDark(isDarkBg)
	initPalette()
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
	barBgColor   color.Color
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

	// Google brand colors — fixed, not theme-dependent.
	googleBlue   color.Color = lipgloss.Color("#4285F4")
	googleRed    color.Color = lipgloss.Color("#EA4335")
	googleYellow color.Color = lipgloss.Color("#FBBC05")
	googleGreen  color.Color = lipgloss.Color("#34A853")
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
	gBlue   lipgloss.Style
	gRed    lipgloss.Style
	gYellow lipgloss.Style
	gGreen  lipgloss.Style
	gBright lipgloss.Style
	gDim    lipgloss.Style

	cascadeBg1 lipgloss.Style
	cascadeBg2 lipgloss.Style
	cascadeBg3 lipgloss.Style
	cascadeBg4 lipgloss.Style
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

// initPalette initializes all adaptive colors, styles, and badges
// from the current ld() function. Called at init and after SetTheme().
func initPalette() {
	// ── Core colors ──
	accentColor = ld(lipgloss.Color("#2563EB"), lipgloss.Color("#6B9FFF"))   // Blue
	dimTextColor = ld(lipgloss.Color("#6B7280"), lipgloss.Color("#64748B"))   // Gray (dark raised for readability ~3.7:1)
	textColor = ld(lipgloss.Color("#374151"), lipgloss.Color("#D1D5DB"))      // Body text
	brightColor = ld(lipgloss.Color("#111827"), lipgloss.Color("#F3F4F6"))    // Headings
	successColor = ld(lipgloss.Color("#047857"), lipgloss.Color("#34D399"))   // Green (light darkened for WCAG AA ~5.0:1)
	warningColor = ld(lipgloss.Color("#92400E"), lipgloss.Color("#FBBF24"))   // Amber (light darkened for WCAG AA ~6.5:1)
	dangerColor = ld(lipgloss.Color("#B91C1C"), lipgloss.Color("#F87171"))    // Red (light darkened for WCAG AA ~5.4:1)
	barBgColor = ld(lipgloss.Color("#F3F4F6"), lipgloss.Color("#111827"))     // Status bar bg
	toolColor = ld(lipgloss.Color("#0E7490"), lipgloss.Color("#22D3EE"))      // Cyan — query tools (cost-bearing, distinct from write warnings)
	planColor = ld(lipgloss.Color("#4F46E5"), lipgloss.Color("#818CF8"))      // Indigo — also used as data tool color

	// ── Diff colors ──
	diffAddBg = ld(lipgloss.Color("#DCFCE7"), lipgloss.Color("#022c22"))
	diffAddFg = ld(lipgloss.Color("#166534"), lipgloss.Color("#86efac"))
	diffRemBg = ld(lipgloss.Color("#FEE2E2"), lipgloss.Color("#2a0a0a"))
	diffRemFg = ld(lipgloss.Color("#991B1B"), lipgloss.Color("#fca5a5"))

	// ── Input colors ──
	inputBorderColor = ld(lipgloss.Color("#D1D5DB"), lipgloss.Color("#374151"))
	inputBorderDimColor = ld(lipgloss.Color("#E5E7EB"), lipgloss.Color("#1F2937"))
	inputBgColor = ld(lipgloss.Color("#ECEEF2"), lipgloss.Color("#3A3B3F"))

	// Muted accent for submitted questions — same hue as accentColor
	// but dialed back so the active input box feels "live" and past questions feel "settled."
	settledAccent = ld(lipgloss.Color("#4A6FA5"), lipgloss.Color("#4A6FA5"))

	// ── Spinner palette ──
	// Sweep: spotlight moves across status text.
	// Light bg: dark=visible, bright=bold black. Dark bg: dim gray → bright white.
	sweepDim = ld(lipgloss.Color("#B0B8C4"), lipgloss.Color("#4B5563"))
	sweepMid = ld(lipgloss.Color("#6B7280"), lipgloss.Color("#9CA3AF"))
	sweepBright = ld(lipgloss.Color("#111827"), lipgloss.Color("#F3F4F6"))

	// Cascade tilde: pulsing ocean blue.
	// Light bg: pale→vivid dark blue. Dark bg: deep→bright cyan.
	cascadeDim = ld(lipgloss.Color("#93C5FD"), lipgloss.Color("#1E3A5F"))
	cascadeTrail = ld(lipgloss.Color("#3B82F6"), lipgloss.Color("#0369A1"))
	cascadeBright = ld(lipgloss.Color("#2563EB"), lipgloss.Color("#38BDF8"))
	cascadePeak = ld(lipgloss.Color("#1D4ED8"), lipgloss.Color("#7DD3FC"))

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
	gBlue = lipgloss.NewStyle().Foreground(googleBlue)
	gRed = lipgloss.NewStyle().Foreground(googleRed)
	gYellow = lipgloss.NewStyle().Foreground(googleYellow)
	gGreen = lipgloss.NewStyle().Foreground(googleGreen)
	gBright = lipgloss.NewStyle().Foreground(brightColor)
	gDim = lipgloss.NewStyle().Foreground(dimTextColor)

	cascadeBg1 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#0C4A6E"), lipgloss.Color("#0369A1")))
	cascadeBg2 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#0369A1"), lipgloss.Color("#0EA5E9")))
	cascadeBg3 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#0EA5E9"), lipgloss.Color("#38BDF8")))
	cascadeBg4 = lipgloss.NewStyle().Background(ld(lipgloss.Color("#38BDF8"), lipgloss.Color("#7DD3FC")))

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

// ToolBullet returns a colored ∞ bullet based on the tool's category.
//
//	Green  ○ — read-only (grep, glob, read)
//	Amber  ◇ — write/modify (write_file, edit_file)
//	Red    ● — execute/risky (bash)
//	Cyan   △ — query/cost-bearing (bigquery_query)
//	Indigo □ — platform data (cloud_logging, gcs)
//
// Shape encodes the action category (accessible without color).
// Color encodes the risk level (redundant with shape for sighted users).
func ToolBullet(toolName string) string {
	return ToolBulletByRisk(toolName, "")
}

// ToolBulletByRisk returns a colored, shape-differentiated bullet.
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
