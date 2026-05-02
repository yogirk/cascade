package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

// Markdown table rendering for assistant responses.
//
// Glamour v2 forces every markdown table to fill the terminal width
// (charm.land/glamour/v2 ansi/table.go:66 — `table.New().Width(width)` with
// no exposed knob). For a table-heavy product like Cascade that produces
// stretched, hard-to-scan tables in prose. None of Crush, Mods, or Glow have
// solved this; we're the first.
//
// Architecture: pre-pass the assistant content with a tiny line scanner,
// segment it into prose / table / prose / ..., render prose through Glamour
// as before, render each extracted table through our shrink-wrapping
// lipgloss table renderer (mirrors `cascadeTable` in tools/bigquery without
// the dep cycle). Concatenate and return.
//
// LLM tables in practice are well-formed pipe tables. We don't need a full
// markdown parser — a 30-line scanner handles the data shape we actually
// see. If we ever need richer cell semantics (inline bold/italic, escaped
// pipes, HTML cells), revisit with goldmark per the project memory note.

// mdSegment is one slice of assistant content: either prose to send through
// Glamour or a self-contained markdown table to render via lipgloss.
type mdSegment struct {
	text    string
	isTable bool
}

// splitMarkdownTables walks content line-by-line and emits ordered segments,
// preserving original ordering of prose and tables. A run of consecutive
// table rows immediately preceded by a header row + alignment separator is
// treated as one table segment; everything else stays as prose.
func splitMarkdownTables(content string) []mdSegment {
	lines := strings.Split(content, "\n")
	var out []mdSegment
	var prose []string

	flushProse := func() {
		if len(prose) == 0 {
			return
		}
		out = append(out, mdSegment{text: strings.Join(prose, "\n"), isTable: false})
		prose = prose[:0]
	}

	i := 0
	for i < len(lines) {
		// A table starts when a row is followed by an alignment separator.
		if i+1 < len(lines) && isTableRow(lines[i]) && isTableSeparator(lines[i+1]) {
			flushProse()
			start := i
			i += 2 // header + separator
			for i < len(lines) && isTableRow(lines[i]) {
				i++
			}
			out = append(out, mdSegment{text: strings.Join(lines[start:i], "\n"), isTable: true})
			continue
		}
		prose = append(prose, lines[i])
		i++
	}
	flushProse()
	return out
}

// isTableRow reports whether a line looks like a markdown pipe-table row:
// trimmed content starts with `|` and ends with `|`, with at least one cell.
func isTableRow(line string) bool {
	s := strings.TrimSpace(line)
	if len(s) < 2 {
		return false
	}
	return strings.HasPrefix(s, "|") && strings.HasSuffix(s, "|")
}

// isTableSeparator reports whether a line is a markdown alignment separator
// like `|---|:---:|---:|`. Each cell must contain at least one `-` and only
// `-`, `:`, or whitespace characters.
func isTableSeparator(line string) bool {
	if !isTableRow(line) {
		return false
	}
	cells := splitMarkdownRow(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		c := strings.TrimSpace(cell)
		if c == "" {
			return false
		}
		hasHyphen := false
		for _, r := range c {
			switch r {
			case '-':
				hasHyphen = true
			case ':', ' ', '\t':
				// allowed
			default:
				return false
			}
		}
		if !hasHyphen {
			return false
		}
	}
	return true
}

// splitMarkdownRow returns the cells of a `|a|b|c|` row, trimmed of
// surrounding whitespace. Leading and trailing pipe boundaries are dropped.
func splitMarkdownRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	parts := strings.Split(s, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// renderMarkdownTable converts a markdown pipe table into a styled,
// shrink-wrapping lipgloss table that matches the look of tool-output
// tables (rounded border, accent header, alternating dim rows). Falls back
// to the original markdown text if parsing yields fewer than the expected
// header + separator + zero-or-more rows shape.
func renderMarkdownTable(md string) string {
	lines := strings.Split(strings.TrimSpace(md), "\n")
	if len(lines) < 2 {
		return md
	}
	headers := splitMarkdownRow(lines[0])
	if len(headers) == 0 {
		return md
	}

	t := newCascadeMarkdownTable(headers)
	for _, raw := range lines[2:] { // skip header + separator
		cells := splitMarkdownRow(raw)
		// Pad / truncate to align with header column count.
		switch {
		case len(cells) < len(headers):
			padded := make([]string, len(headers))
			copy(padded, cells)
			cells = padded
		case len(cells) > len(headers):
			cells = cells[:len(headers)]
		}
		t.Row(cells...)
	}
	return t.Render()
}

// newCascadeMarkdownTable builds the lipgloss table used for assistant
// markdown tables. Mirrors the look of cascadeTable in tools/bigquery —
// we keep a parallel definition here to avoid a tui→tools/bigquery import
// cycle (tui already imports tools/bigquery the other way for chart/query
// rendering). If a third caller appears, lift this into a shared package.
func newCascadeMarkdownTable(headers []string) *table.Table {
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderTop(true).
		BorderBottom(true).
		BorderLeft(true).
		BorderRight(true).
		BorderColumn(true).
		BorderRow(false).
		BorderHeader(true).
		BorderStyle(lipgloss.NewStyle().Foreground(inputBorderDimColor)).
		Headers(headers...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Foreground(accentColor).
					Bold(true).
					Padding(0, 1)
			}
			s := lipgloss.NewStyle().Padding(0, 1)
			if row%2 == 0 {
				return s.Foreground(textColor)
			}
			return s.Foreground(dimTextColor)
		}).
		Wrap(false)
}
