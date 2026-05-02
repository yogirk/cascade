// Package duckdb provides DuckDB tool implementations for Cascade.
package duckdb

import (
	"fmt"
	"strings"

	"github.com/slokam-ai/cascade/internal/duckdb"
	"github.com/slokam-ai/cascade/internal/render"
)

// renderQueryResult renders a duckdb.QueryResult as both a styled
// display string for the TUI and a plain-text content string for the
// LLM. Mirrors the shape of internal/tools/bigquery's renderer minus
// the cost footer — DuckDB's value-prop is no per-query cost.
func renderQueryResult(res *duckdb.QueryResult, durationMs int64) (display, content string) {
	if res == nil || len(res.Columns) == 0 {
		msg := "Query returned no columns."
		return msg, msg
	}

	headers := make([]string, len(res.Columns))
	for i, c := range res.Columns {
		headers[i] = c.Name
	}

	var sb strings.Builder

	// Same wide-column escape hatch BQ uses — over 10 cols becomes
	// gibberish at 120 chars wide.
	if len(headers) > 10 {
		sb.WriteString(render.DimStyle().Render(
			fmt.Sprintf("  %d columns × %d rows (table too wide for display)",
				len(headers), res.TotalRows)))
	} else {
		t := render.CascadeTable(headers)
		for _, row := range res.Rows {
			t.Row(row...)
		}
		sb.WriteString(t.Render())

		if res.Truncated {
			remaining := res.TotalRows - uint64(len(res.Rows))
			sb.WriteString("\n" + render.DimStyle().Render(
				fmt.Sprintf("  -- truncated: showing %d of %d rows --",
					len(res.Rows), res.TotalRows)))
			_ = remaining
		}
	}

	footer := fmt.Sprintf("  %s · %s rows", render.FormatDuration(durationMs),
		render.FormatRowCount(int64(res.TotalRows)))
	sb.WriteString("\n" + render.DimStyle().Render(footer))

	if len(res.Warnings) > 0 {
		sb.WriteString("\n" + render.DimStyle().Render(
			fmt.Sprintf("  %d warning(s): %s", len(res.Warnings), strings.Join(res.Warnings, "; "))))
	}

	display = sb.String()
	content = renderPlain(headers, res, durationMs)
	return display, content
}

// renderPlain builds an ANSI-free LLM-friendly representation. Includes
// the truncation marker the design names: the agent should see it and
// react by asking for an aggregation or sample.
func renderPlain(headers []string, res *duckdb.QueryResult, durationMs int64) string {
	var sb strings.Builder

	rows := res.Rows
	if len(rows) > 50 {
		rows = rows[:50]
	}

	sb.WriteString(strings.Join(headers, "\t") + "\n")
	for _, r := range rows {
		sb.WriteString(strings.Join(r, "\t") + "\n")
	}

	if res.Truncated {
		sb.WriteString(fmt.Sprintf("-- truncated: showing %d of %d rows --\n",
			len(res.Rows), res.TotalRows))
	} else if uint64(len(rows)) < res.TotalRows {
		sb.WriteString(fmt.Sprintf("(%d more rows omitted from LLM context)\n",
			res.TotalRows-uint64(len(rows))))
	}

	sb.WriteString(fmt.Sprintf("%s | %s rows\n",
		render.FormatDuration(durationMs),
		render.FormatRowCount(int64(res.TotalRows))))

	for _, w := range res.Warnings {
		sb.WriteString("warning: " + w + "\n")
	}

	return sb.String()
}

// renderTableList renders the output of `SHOW TABLES` as a styled list.
func renderTableList(rows [][]string) (display, content string) {
	if len(rows) == 0 {
		msg := "No tables in the local DuckDB session."
		return msg, msg
	}

	var sb strings.Builder
	sb.WriteString("  " + render.HeaderStyle().Render(
		fmt.Sprintf("Tables (%d)", len(rows))) + "\n")
	t := render.CascadeTable([]string{"Name"})
	for _, r := range rows {
		t.Row(r...)
	}
	sb.WriteString(t.Render())
	display = sb.String()

	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("Tables (%d):\n", len(rows)))
	for _, r := range rows {
		if len(r) > 0 {
			cb.WriteString("  " + r[0] + "\n")
		}
	}
	content = cb.String()
	return display, content
}

// renderColumnList renders DESCRIBE output for one table.
func renderColumnList(table string, cols []duckdb.Column) (display, content string) {
	var sb strings.Builder
	sb.WriteString("  " + render.BrightStyle().Render(table) + "\n")
	sb.WriteString(render.DimStyle().Render(
		fmt.Sprintf("  Columns (%d)", len(cols))) + "\n")

	t := render.CascadeTable([]string{"Column", "Type"})
	for _, c := range cols {
		t.Row(c.Name, c.Type)
	}
	sb.WriteString(t.Render())
	display = sb.String()

	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("%s — %d columns:\n", table, len(cols)))
	for _, c := range cols {
		cb.WriteString(fmt.Sprintf("  %-30s %s\n", c.Name, c.Type))
	}
	content = cb.String()
	return display, content
}
