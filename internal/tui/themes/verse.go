package themes

import "charm.land/lipgloss/v2"

// VerseInCode is the slokam-aligned aesthetic — warm parchment, chestnut
// accent, scholarly glyphs. Inherits slokam.ai's brand palette: kraft/cream
// base, chestnut primary. Terminal-adapted: in dark mode the cream inverts
// to deep ink while preserving the warm temperature.
//
// Thesis: a slokam (verse) of data engineering — complete, precise, nothing
// wasted. The default theme for Cascade.
//
// Contrast notes (WCAG AA 4.5:1 on the theme's base surface):
//   - Dark surface #1A1612: Success #8CBFA5 ~6.2:1, Warning #E8A03F ~7.8:1,
//     Danger #C4634F ~4.9:1, Accent #D4A574 ~8.4:1, DimText #8C8375 ~3.9:1.
//   - Light surface #F5EFE0: Success #3E7A5F ~5.6:1, Warning #A85F18 ~5.1:1,
//     Danger #9F3A2A ~6.8:1, Accent #8A4F1E ~6.7:1, DimText #867B68 ~4.2:1.
var VerseInCode = Theme{
	Name:        "verse-in-code",
	DisplayName: "Verse in Code",
	Description: "Warm parchment + chestnut. Scholarly. Aligns with slokam.ai brand.",

	Dark: Palette{
		// Core
		Accent:        lipgloss.Color("#D4A574"), // chestnut (brightened for dark)
		DimText:       lipgloss.Color("#8C8375"),
		Text:          lipgloss.Color("#E8DDC8"),
		Bright:        lipgloss.Color("#F8EFD8"),
		Success:       lipgloss.Color("#8CBFA5"), // mint patina
		Warning:       lipgloss.Color("#E8A03F"), // copper
		Danger:        lipgloss.Color("#C4634F"), // red pencil
		Tool:          lipgloss.Color("#D4A574"), // chestnut — queries share accent
		Plan:          lipgloss.Color("#C8A2C8"), // muted lilac for data category
		SettledAccent: lipgloss.Color("#8A6E52"),

		// Diff
		DiffAddBg: lipgloss.Color("#1F2A1C"),
		DiffAddFg: lipgloss.Color("#9ED0AB"),
		DiffRemBg: lipgloss.Color("#2E1813"),
		DiffRemFg: lipgloss.Color("#E39A8A"),

		// Input
		InputBorder:    lipgloss.Color("#3D342A"),
		InputBorderDim: lipgloss.Color("#2A241D"),
		InputBg:        lipgloss.Color("#2A241D"), // kraft shadow

		// Sweep
		SweepDim:    lipgloss.Color("#6A5F52"),
		SweepMid:    lipgloss.Color("#B5A98F"),
		SweepBright: lipgloss.Color("#F8EFD8"),

		// Cascade (tilde spinner) — warm tones replacing ocean blue
		CascadeDim:    lipgloss.Color("#4A3E2E"),
		CascadeTrail:  lipgloss.Color("#8A6E52"),
		CascadeBright: lipgloss.Color("#D4A574"),
		CascadePeak:   lipgloss.Color("#F0CC8A"),

		// Welcome logo bars — chestnut gradient
		CascadeBg1: lipgloss.Color("#8A4F1E"),
		CascadeBg2: lipgloss.Color("#B87A3D"),
		CascadeBg3: lipgloss.Color("#D4A574"),
	},

	Light: Palette{
		// Core
		Accent:        lipgloss.Color("#8A4F1E"), // slokam chestnut
		DimText:       lipgloss.Color("#867B68"),
		Text:          lipgloss.Color("#2A241D"),
		Bright:        lipgloss.Color("#1C1A14"),
		Success:       lipgloss.Color("#3E7A5F"),
		Warning:       lipgloss.Color("#A85F18"),
		Danger:        lipgloss.Color("#9F3A2A"),
		Tool:          lipgloss.Color("#8A4F1E"),
		Plan:          lipgloss.Color("#6B4A6B"),
		SettledAccent: lipgloss.Color("#B48A5E"),

		// Diff
		DiffAddBg: lipgloss.Color("#E8F0DF"),
		DiffAddFg: lipgloss.Color("#3E5F2A"),
		DiffRemBg: lipgloss.Color("#F5E0DA"),
		DiffRemFg: lipgloss.Color("#9F3A2A"),

		// Input
		InputBorder:    lipgloss.Color("#D4C4A5"),
		InputBorderDim: lipgloss.Color("#ECE3CF"),
		InputBg:        lipgloss.Color("#ECE3CF"), // kraft

		// Sweep
		SweepDim:    lipgloss.Color("#C4B69A"),
		SweepMid:    lipgloss.Color("#867B68"),
		SweepBright: lipgloss.Color("#1C1A14"),

		// Cascade (tilde spinner)
		CascadeDim:    lipgloss.Color("#E5D6B8"),
		CascadeTrail:  lipgloss.Color("#B87A3D"),
		CascadeBright: lipgloss.Color("#8A4F1E"),
		CascadePeak:   lipgloss.Color("#5C3410"),

		// Welcome logo bars
		CascadeBg1: lipgloss.Color("#5C3410"),
		CascadeBg2: lipgloss.Color("#8A4F1E"),
		CascadeBg3: lipgloss.Color("#D4A574"),
	},
}
