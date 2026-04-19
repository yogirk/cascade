// Package themes defines named color palettes (themes) for the Cascade TUI.
//
// Each Theme bundles a Light and Dark Palette with metadata. The TUI picks
// between Light and Dark at runtime based on terminal background detection.
// Themes are registered in registry.go and selected via config, CLI flag,
// or the /theme slash command.
package themes

import "image/color"

// Palette is the full set of colors used across the TUI for a single
// lightness variant. Pair a Light and Dark Palette inside a Theme.
//
// Every color listed here has a corresponding module-level variable in
// internal/tui/styles.go — adding a field here requires wiring it through
// initPalette(). Keep the field order stable (changing it churns diffs
// across every registered theme).
type Palette struct {
	// ── Core text colors ──
	Accent        color.Color // Assistant bullet, input border, headings
	DimText       color.Color // Metadata, timestamps, secondary info
	Text          color.Color // Body text, normal weight content
	Bright        color.Color // Headings, key values, bold text
	Success       color.Color // Read tools, safe cost, ASK badge
	Warning       color.Color // Write tools, cost warning, approval
	Danger        color.Color // Exec tools, errors, destructive
	Tool          color.Color // Query tool bullets (cost-bearing)
	Plan          color.Color // Platform data tools (cloud_logging, gcs)
	SettledAccent color.Color // Submitted user message border (muted echo)

	// ── Diff colors ──
	DiffAddBg color.Color
	DiffAddFg color.Color
	DiffRemBg color.Color
	DiffRemFg color.Color

	// ── Input box ──
	InputBorder    color.Color
	InputBorderDim color.Color
	InputBg        color.Color

	// ── Sweep palette (spinner text spotlight) ──
	SweepDim    color.Color
	SweepMid    color.Color
	SweepBright color.Color

	// ── Cascade palette (tilde spinner pulse) ──
	CascadeDim    color.Color
	CascadeTrail  color.Color
	CascadeBright color.Color
	CascadePeak   color.Color

	// ── Welcome banner cascade bars ──
	CascadeBg1 color.Color
	CascadeBg2 color.Color
	CascadeBg3 color.Color
}

// Theme bundles a light-mode and dark-mode Palette with metadata.
// The TUI selects between Light and Dark based on terminal background
// detection; the user selects between themes via config or /theme.
type Theme struct {
	Name        string // Stable identifier, kebab-case (e.g. "verse-in-code")
	DisplayName string // User-facing name (e.g. "Verse in Code")
	Description string // Short rationale shown in the /theme picker
	Light       Palette
	Dark        Palette
}

// Pick returns the Palette for the given lightness.
func (t Theme) Pick(isDark bool) Palette {
	if isDark {
		return t.Dark
	}
	return t.Light
}
