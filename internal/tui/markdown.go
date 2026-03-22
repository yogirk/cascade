package tui

import "charm.land/glamour/v2/ansi"

// cascadeMarkdownStyle is a custom Glamour theme optimized for conversational AI.
// Design principles:
//   - Minimal color: use typography (bold, dim, italic) for hierarchy, not rainbows
//   - Inline code: subtle background tint, not red — red means errors in a CLI
//   - Headings: accent blue (matches app palette), not magenta/yellow
//   - Code blocks: muted syntax highlighting, no heavy backgrounds
//   - Lists: clean bullets, no extra color
//   - Links: underline only, no color change
//
// Colors use the 256-color palette for broad terminal compatibility.
// These are tuned for dark backgrounds; the ld() adaptive pattern isn't
// available here since Glamour uses string color refs, not lipgloss.Color.
func cascadeMarkdownStyle() ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       sp("252"), // Light gray body text
			},
			Margin: up(0),
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: sp("245"), // Dimmed
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

		// Headings: accent blue, bold — clean hierarchy without shouting
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       sp("75"), // Muted blue (matches accentColor)
				Bold:        bp(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Bold:  bp(true),
				Color: sp("75"),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Bold:   bp(true),
				Color:  sp("75"),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Bold:   bp(true),
				Color:  sp("75"),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Bold:   bp(true),
				Color:  sp("243"),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  sp("243"),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  sp("243"),
			},
		},

		// Text styles: typography over color
		Text: ansi.StylePrimitive{
			Color: sp("252"),
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: bp(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: bp(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: bp(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  sp("240"),
			Format: "\n────────\n",
		},

		// Lists: clean bullets, no color
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},

		// Links: underline only, no color shouting
		Link: ansi.StylePrimitive{
			Underline: bp(true),
			Color:     sp("244"),
		},
		LinkText: ansi.StylePrimitive{
			Color: sp("252"),
		},
		Image: ansi.StylePrimitive{
			Underline: bp(true),
			Color:     sp("244"),
		},
		ImageText: ansi.StylePrimitive{
			Color:  sp("243"),
			Format: "Image: {{.text}} →",
		},

		// Inline code: subtle tint, NOT red. Uses a gentle cyan on dark bg.
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           sp("117"), // Soft cyan — stands out without alarming
				BackgroundColor: sp("236"), // Subtle dark background
			},
		},

		// Code blocks: muted syntax highlighting
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: sp("250"),
				},
				Margin: up(1),
			},
			Chroma: &ansi.Chroma{
				Text:    ansi.StylePrimitive{Color: sp("#C4C4C4")},
				Error:   ansi.StylePrimitive{Color: sp("#F1F1F1"), BackgroundColor: sp("#4A2020")},
				Comment: ansi.StylePrimitive{Color: sp("#676767")},
				Keyword: ansi.StylePrimitive{Color: sp("#7EB8DA")},
				// Muted keyword variants
				KeywordReserved:  ansi.StylePrimitive{Color: sp("#B39DDB")},
				KeywordNamespace: ansi.StylePrimitive{Color: sp("#90A4AE")},
				KeywordType:      ansi.StylePrimitive{Color: sp("#80CBC4")},
				Operator:         ansi.StylePrimitive{Color: sp("#B0BEC5")},
				Punctuation:      ansi.StylePrimitive{Color: sp("#B0BEC5")},
				Name:             ansi.StylePrimitive{Color: sp("#C4C4C4")},
				NameBuiltin:      ansi.StylePrimitive{Color: sp("#CE93D8")},
				NameTag:          ansi.StylePrimitive{Color: sp("#B39DDB")},
				NameAttribute:    ansi.StylePrimitive{Color: sp("#90A4AE")},
				NameClass:        ansi.StylePrimitive{Color: sp("#F1F1F1"), Bold: bp(true)},
				NameFunction:     ansi.StylePrimitive{Color: sp("#80CBC4")},
				LiteralNumber:    ansi.StylePrimitive{Color: sp("#A5D6A7")},
				LiteralString:    ansi.StylePrimitive{Color: sp("#BCAAA4")},
				GenericDeleted:   ansi.StylePrimitive{Color: sp("#EF9A9A")},
				GenericInserted:  ansi.StylePrimitive{Color: sp("#A5D6A7")},
				GenericEmph:      ansi.StylePrimitive{Italic: bp(true)},
				GenericStrong:    ansi.StylePrimitive{Bold: bp(true)},
				GenericSubheading: ansi.StylePrimitive{Color: sp("#90A4AE")},
			},
		},

		// Tables: clean separators
		Table: ansi.StyleTable{
			StyleBlock:      ansi.StyleBlock{},
			CenterSeparator: sp("┼"),
			ColumnSeparator: sp("│"),
			RowSeparator:    sp("─"),
		},

		DefinitionTerm: ansi.StylePrimitive{
			Bold: bp(true),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n",
		},
	}
}

// helpers to create pointers for Glamour style values
func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }
func up(u uint) *uint     { return &u }
