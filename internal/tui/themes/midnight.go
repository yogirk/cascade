package themes

import "charm.land/lipgloss/v2"

// Midnight is the distinctive cool-water aesthetic — deep basins, channel
// blues, mineral mist. Dogfoods the "cascade" name via flow motifs and
// directional motion (cascade sweep on streaming).
//
// Thesis: a midnight hydrology control room — calm, cold, precise, with the
// quiet kinetic energy of water moving through pressure, gates, channels.
// A flow instrument, not a dashboard.
//
// Contrast notes (WCAG AA 4.5:1 on the theme's base surface):
//   - Dark surface #0D1B2A: Success #9FD3C7 ~7.4:1, Warning #D6C7A1 ~8.5:1,
//     Danger #C46A5A ~5.0:1, Accent #6EA8C7 ~5.3:1, DimText #7B8A9B ~3.8:1.
//   - Light surface #F0F4F8: Success #0E7C6A ~5.8:1, Warning #8A6E2F ~5.4:1,
//     Danger #9F3A2A ~6.4:1, Accent #1F6B9B ~5.9:1, DimText #6F7A87 ~4.3:1.
var Midnight = Theme{
	Name:        "midnight",
	DisplayName: "Midnight",
	Description: "Cool water mineral tones. Distinctive flow-instrument aesthetic.",

	Dark: Palette{
		// Core
		Accent:        lipgloss.Color("#6EA8C7"), // channel blue
		DimText:       lipgloss.Color("#7B8A9B"),
		Text:          lipgloss.Color("#D9E3EC"),
		Bright:        lipgloss.Color("#F5F8FB"),
		Success:       lipgloss.Color("#9FD3C7"), // mineral mist
		Warning:       lipgloss.Color("#D6C7A1"), // sediment gold
		Danger:        lipgloss.Color("#C46A5A"), // danger clay
		Tool:          lipgloss.Color("#6EA8C7"), // channel — queries share accent
		Plan:          lipgloss.Color("#B4A5D8"), // soft lavender for data category
		SettledAccent: lipgloss.Color("#4A6FA5"),

		// Diff
		DiffAddBg: lipgloss.Color("#0A2416"),
		DiffAddFg: lipgloss.Color("#86CFAA"),
		DiffRemBg: lipgloss.Color("#2A0E0A"),
		DiffRemFg: lipgloss.Color("#E5998C"),

		// Input
		InputBorder:    lipgloss.Color("#2A3C52"),
		InputBorderDim: lipgloss.Color("#1B263B"),
		InputBg:        lipgloss.Color("#1B263B"), // wet slate

		// Sweep
		SweepDim:    lipgloss.Color("#3D4A5C"),
		SweepMid:    lipgloss.Color("#8598AB"),
		SweepBright: lipgloss.Color("#F5F8FB"),

		// Cascade (tilde spinner) — cool ocean palette
		CascadeDim:    lipgloss.Color("#1E3A5F"),
		CascadeTrail:  lipgloss.Color("#3D6B8C"),
		CascadeBright: lipgloss.Color("#6EA8C7"),
		CascadePeak:   lipgloss.Color("#9FD3C7"),

		// Welcome logo bars — deep-to-shallow gradient
		CascadeBg1: lipgloss.Color("#0C2E4F"),
		CascadeBg2: lipgloss.Color("#1F5A8C"),
		CascadeBg3: lipgloss.Color("#6EA8C7"),
	},

	Light: Palette{
		// Core
		Accent:        lipgloss.Color("#1F6B9B"),
		DimText:       lipgloss.Color("#6F7A87"),
		Text:          lipgloss.Color("#1A2430"),
		Bright:        lipgloss.Color("#0B1420"),
		Success:       lipgloss.Color("#0E7C6A"),
		Warning:       lipgloss.Color("#8A6E2F"),
		Danger:        lipgloss.Color("#9F3A2A"),
		Tool:          lipgloss.Color("#1F6B9B"),
		Plan:          lipgloss.Color("#5C4A8A"),
		SettledAccent: lipgloss.Color("#6F8DB5"),

		// Diff
		DiffAddBg: lipgloss.Color("#D5E8D8"),
		DiffAddFg: lipgloss.Color("#0E5A3F"),
		DiffRemBg: lipgloss.Color("#F5D8D0"),
		DiffRemFg: lipgloss.Color("#9F3A2A"),

		// Input
		InputBorder:    lipgloss.Color("#BFCEDA"),
		InputBorderDim: lipgloss.Color("#E1E8EF"),
		InputBg:        lipgloss.Color("#E1E8EF"),

		// Sweep
		SweepDim:    lipgloss.Color("#B8C4D0"),
		SweepMid:    lipgloss.Color("#6F7A87"),
		SweepBright: lipgloss.Color("#0B1420"),

		// Cascade (tilde spinner)
		CascadeDim:    lipgloss.Color("#A8C8DC"),
		CascadeTrail:  lipgloss.Color("#5A8AB0"),
		CascadeBright: lipgloss.Color("#1F6B9B"),
		CascadePeak:   lipgloss.Color("#0D3D5F"),

		// Welcome logo bars
		CascadeBg1: lipgloss.Color("#0D3D5F"),
		CascadeBg2: lipgloss.Color("#1F6B9B"),
		CascadeBg3: lipgloss.Color("#6EA8C7"),
	},
}
