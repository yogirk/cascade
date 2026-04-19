package tui

import (
	"charm.land/glamour/v2/ansi"

	"github.com/yogirk/cascade/internal/tui/themes"
)

// cascadeMarkdownStyle builds a Glamour StyleConfig from the currently
// active palette. Called on every renderMarkdown invocation so theme
// switches take effect the next time a message renders.
//
// Design principles (preserved across themes):
//   - Minimal color. Typography (bold, dim, italic) carries hierarchy.
//   - Inline code: accent-tinted foreground on a subtle background — never red.
//   - Headings: the theme's accent color, bold.
//   - Code blocks: theme-driven Chroma syntax highlighting.
//   - Links: underlined, muted — no color shouting.
func cascadeMarkdownStyle() ansi.StyleConfig {
	p := themes.ActivePalette()
	isDark := themes.ActiveIsDark()

	// Hex strings Glamour can consume directly.
	accent := themes.Hex(p.Accent)
	text := themes.Hex(p.Text)
	dim := themes.Hex(p.DimText)
	bright := themes.Hex(p.Bright)
	tool := themes.Hex(p.Tool)
	success := themes.Hex(p.Success)
	warning := themes.Hex(p.Warning)
	danger := themes.Hex(p.Danger)
	plan := themes.Hex(p.Plan)
	border := themes.Hex(p.InputBorderDim)

	// Inline code background: a theme-aware tint. On dark themes we want a
	// slightly-lighter block; on light themes, a slightly-darker block.
	// Use InputBg (which IS the theme's "elevated surface") as the foundation.
	codeBg := themes.Hex(p.InputBg)
	// Error background used by Chroma — tinted danger, theme-aware.
	errorBg := themes.Hex(p.DiffRemBg)

	cfg := ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       sp(text),
			},
			Margin: up(0),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: sp(dim),
				Faint: bp(true),
			},
			Indent:      up(1),
			IndentToken: sp("│ "),
		},
		Paragraph: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{},
		},
		List: ansi.StyleList{
			LevelIndent: 2,
		},

		// Headings: accent, bold.
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       sp(accent),
				Bold:        bp(true),
			},
		},
		H1: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Bold: bp(true), Color: sp(accent)}},
		H2: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Prefix: "## ", Bold: bp(true), Color: sp(accent)}},
		H3: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Prefix: "### ", Bold: bp(true), Color: sp(accent)}},
		H4: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Prefix: "#### ", Bold: bp(true), Color: sp(dim)}},
		H5: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Prefix: "##### ", Color: sp(dim)}},
		H6: ansi.StyleBlock{StylePrimitive: ansi.StylePrimitive{Prefix: "###### ", Color: sp(dim)}},

		Text:          ansi.StylePrimitive{Color: sp(text)},
		Strikethrough: ansi.StylePrimitive{CrossedOut: bp(true)},
		Emph:          ansi.StylePrimitive{Italic: bp(true)},
		Strong:        ansi.StylePrimitive{Bold: bp(true)},
		HorizontalRule: ansi.StylePrimitive{
			Color:  sp(border),
			Format: "\n────────\n",
		},

		Item:        ansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: ansi.StylePrimitive{BlockPrefix: ". "},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},

		// Links: underlined, muted. No color shouting.
		Link:      ansi.StylePrimitive{Underline: bp(true), Color: sp(dim)},
		LinkText:  ansi.StylePrimitive{Color: sp(text)},
		Image:     ansi.StylePrimitive{Underline: bp(true), Color: sp(dim)},
		ImageText: ansi.StylePrimitive{Color: sp(dim), Format: "Image: {{.text}} →"},

		// Inline code: tool/accent foreground on theme's elevated surface.
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           sp(tool),
				BackgroundColor: sp(codeBg),
			},
		},

		// Code blocks: theme-driven Chroma. Token colors pulled from the
		// palette so cyan isn't mandatory — in Verse in Code, keywords
		// read chestnut; in Midnight Hydrology, channel blue; in Classic,
		// GCP blue.
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{Color: sp(text)},
				Margin:         up(1),
			},
			Chroma: &ansi.Chroma{
				Text:              ansi.StylePrimitive{Color: sp(text)},
				Error:             ansi.StylePrimitive{Color: sp(danger), BackgroundColor: sp(errorBg)},
				Comment:           ansi.StylePrimitive{Color: sp(dim), Italic: bp(true)},
				Keyword:           ansi.StylePrimitive{Color: sp(accent)},
				KeywordReserved:   ansi.StylePrimitive{Color: sp(plan)},
				KeywordNamespace:  ansi.StylePrimitive{Color: sp(dim)},
				KeywordType:       ansi.StylePrimitive{Color: sp(tool)},
				Operator:          ansi.StylePrimitive{Color: sp(text)},
				Punctuation:       ansi.StylePrimitive{Color: sp(text)},
				Name:              ansi.StylePrimitive{Color: sp(text)},
				NameBuiltin:       ansi.StylePrimitive{Color: sp(plan)},
				NameTag:           ansi.StylePrimitive{Color: sp(accent)},
				NameAttribute:     ansi.StylePrimitive{Color: sp(dim)},
				NameClass:         ansi.StylePrimitive{Color: sp(bright), Bold: bp(true)},
				NameFunction:      ansi.StylePrimitive{Color: sp(warning)},
				LiteralNumber:     ansi.StylePrimitive{Color: sp(plan)},
				LiteralString:     ansi.StylePrimitive{Color: sp(success)},
				GenericDeleted:    ansi.StylePrimitive{Color: sp(danger)},
				GenericInserted:   ansi.StylePrimitive{Color: sp(success)},
				GenericEmph:       ansi.StylePrimitive{Italic: bp(true)},
				GenericStrong:     ansi.StylePrimitive{Bold: bp(true)},
				GenericSubheading: ansi.StylePrimitive{Color: sp(dim)},
			},
		},

		// Tables: separators use the theme's border color.
		Table: ansi.StyleTable{
			StyleBlock:      ansi.StyleBlock{},
			CenterSeparator: sp("┼"),
			ColumnSeparator: sp("│"),
			RowSeparator:    sp("─"),
		},

		DefinitionTerm:        ansi.StylePrimitive{Bold: bp(true)},
		DefinitionDescription: ansi.StylePrimitive{BlockPrefix: "\n"},
	}

	// Silence unused-var warning for the (reserved for future variance
	// between light/dark treatments e.g. code block background strategies).
	_ = isDark

	return cfg
}

// helpers to create pointers for Glamour style values
func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }
func up(u uint) *uint     { return &u }
