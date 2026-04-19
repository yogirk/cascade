package themes

import "charm.land/lipgloss/v2"

// Classic is the legacy main-branch palette — Google-adjacent blues with
// Tailwind-like neutrals. Originated from an early Google Cloud Console
// imitation that then drifted via ad-hoc decisions. Preserved here so the
// existing look remains available for A/B/C comparison against the two new
// theses (Verse in Code, Midnight Hydrology) during peer testing.
//
// All hex values and contrast measurements are carried forward verbatim from
// the pre-refactor `initPalette()` so visual equivalence with main is exact.
//
// Contrast notes (from the original WCAG AA audit, retained):
//   - Dark Success #34D399 ≈ 5.0:1, Warning #FBBF24 ≈ 6.5:1,
//     Danger #F87171 ≈ 5.4:1, Tool #22D3EE ≈ 4.8:1, Plan #818CF8 ≈ 5.7:1.
//   - Dark Dim raised from #4B5563 (2.35:1) to #64748B (~3.7:1).
var Classic = Theme{
	Name:        "classic",
	DisplayName: "Classic",
	Description: "Legacy main-branch palette — blue accent, cyan queries, indigo data.",

	Dark: Palette{
		// Core
		Accent:        lipgloss.Color("#6B9FFF"), // Blue
		DimText:       lipgloss.Color("#64748B"), // Gray (raised for readability)
		Text:          lipgloss.Color("#D1D5DB"),
		Bright:        lipgloss.Color("#F3F4F6"),
		Success:       lipgloss.Color("#34D399"), // Green
		Warning:       lipgloss.Color("#FBBF24"), // Amber
		Danger:        lipgloss.Color("#F87171"), // Red
		Tool:          lipgloss.Color("#22D3EE"), // Cyan — queries
		Plan:          lipgloss.Color("#818CF8"), // Indigo — data tools
		SettledAccent: lipgloss.Color("#4A6FA5"),

		// Diff
		DiffAddBg: lipgloss.Color("#022c22"),
		DiffAddFg: lipgloss.Color("#86efac"),
		DiffRemBg: lipgloss.Color("#2a0a0a"),
		DiffRemFg: lipgloss.Color("#fca5a5"),

		// Input
		InputBorder:    lipgloss.Color("#374151"),
		InputBorderDim: lipgloss.Color("#1F2937"),
		InputBg:        lipgloss.Color("#3A3B3F"),

		// Sweep
		SweepDim:    lipgloss.Color("#4B5563"),
		SweepMid:    lipgloss.Color("#9CA3AF"),
		SweepBright: lipgloss.Color("#F3F4F6"),

		// Cascade (tilde spinner) — ocean blue pulse
		CascadeDim:    lipgloss.Color("#1E3A5F"),
		CascadeTrail:  lipgloss.Color("#0369A1"),
		CascadeBright: lipgloss.Color("#38BDF8"),
		CascadePeak:   lipgloss.Color("#7DD3FC"),

		// Welcome logo bars
		CascadeBg1: lipgloss.Color("#0369A1"),
		CascadeBg2: lipgloss.Color("#0EA5E9"),
		CascadeBg3: lipgloss.Color("#38BDF8"),
	},

	Light: Palette{
		// Core
		Accent:        lipgloss.Color("#2563EB"),
		DimText:       lipgloss.Color("#6B7280"),
		Text:          lipgloss.Color("#374151"),
		Bright:        lipgloss.Color("#111827"),
		Success:       lipgloss.Color("#047857"),
		Warning:       lipgloss.Color("#92400E"),
		Danger:        lipgloss.Color("#B91C1C"),
		Tool:          lipgloss.Color("#0E7490"),
		Plan:          lipgloss.Color("#4F46E5"),
		SettledAccent: lipgloss.Color("#4A6FA5"),

		// Diff
		DiffAddBg: lipgloss.Color("#DCFCE7"),
		DiffAddFg: lipgloss.Color("#166534"),
		DiffRemBg: lipgloss.Color("#FEE2E2"),
		DiffRemFg: lipgloss.Color("#991B1B"),

		// Input
		InputBorder:    lipgloss.Color("#D1D5DB"),
		InputBorderDim: lipgloss.Color("#E5E7EB"),
		InputBg:        lipgloss.Color("#ECEEF2"),

		// Sweep
		SweepDim:    lipgloss.Color("#B0B8C4"),
		SweepMid:    lipgloss.Color("#6B7280"),
		SweepBright: lipgloss.Color("#111827"),

		// Cascade (tilde spinner)
		CascadeDim:    lipgloss.Color("#93C5FD"),
		CascadeTrail:  lipgloss.Color("#3B82F6"),
		CascadeBright: lipgloss.Color("#2563EB"),
		CascadePeak:   lipgloss.Color("#1D4ED8"),

		// Welcome logo bars
		CascadeBg1: lipgloss.Color("#0C4A6E"),
		CascadeBg2: lipgloss.Color("#0369A1"),
		CascadeBg3: lipgloss.Color("#0EA5E9"),
	},
}
