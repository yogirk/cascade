// Package bigquery provides BigQuery tool implementations for Cascade.
package bigquery

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	bq "github.com/yogirk/cascade/internal/bigquery"
	"github.com/yogirk/cascade/internal/schema"
	"github.com/yogirk/cascade/internal/tui/themes"
)

// Palette accessors. Built per-call from the live palette so BQ renderers
// follow the active theme — package-level `var` would freeze on whatever
// theme was active at package-init time.
func accentColor() color.Color   { return themes.ActivePalette().Accent }
func brightColor() color.Color   { return themes.ActivePalette().Bright }
func dimColor() color.Color      { return themes.ActivePalette().DimText }
func textColor() color.Color     { return themes.ActivePalette().Text }
func warningColor() color.Color  { return themes.ActivePalette().Warning }
func separatorClr() color.Color  { return themes.ActivePalette().InputBorderDim }

func headerStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(accentColor()).Bold(true)
}
func dimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(dimColor())
}

// cascadeTable builds a table with the Cascade conversational style:
// bold header row, thin separator, no borders, padding-separated columns,
// optional alternating row dimming.
func cascadeTable(headers []string) *table.Table {
	return table.New().
		Border(lipgloss.Border{
			Top:         "",
			Bottom:      "",
			Left:        "",
			Right:       "",
			TopLeft:     "",
			TopRight:    "",
			BottomLeft:  "",
			BottomRight: "",
			MiddleLeft:  "",
			MiddleRight: "",
			Middle:      "",
			MiddleTop:   "",
			MiddleBottom: "",
		}).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderColumn(false).
		BorderRow(false).
		BorderHeader(true).
		BorderStyle(lipgloss.NewStyle().Foreground(separatorClr())).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Foreground(accentColor()).
					Bold(true).
					PaddingRight(2)
			}
			// Subtle alternating rows for scanability
			s := lipgloss.NewStyle().PaddingRight(2)
			if row%2 == 0 {
				return s.Foreground(textColor())
			}
			return s.Foreground(dimColor()) // Alternating dim row
		}).
		Wrap(false).
		Width(120)
}

// RenderQueryResults renders query results as a styled table.
// Returns both a Display string (styled for TUI) and a Content string (plain text for LLM).
func RenderQueryResults(headers []string, rows [][]string, totalRows uint64, maxDisplayRows int, cost float64, durationMs int64, bytesScanned int64) (display string, content string) {
	if len(headers) == 0 {
		msg := "Query returned no columns."
		return msg, msg
	}

	displayRows := rows
	if len(displayRows) > maxDisplayRows {
		displayRows = displayRows[:maxDisplayRows]
	}

	var sb strings.Builder

	// If too many columns, the styled table becomes unreadable at 120 chars.
	// Show a compact summary instead of truncated gibberish.
	if len(headers) > 10 {
		sb.WriteString(dimStyle().Render(fmt.Sprintf("  %d columns × %d rows (table too wide for display)", len(headers), totalRows)))
	} else {
		t := cascadeTable(headers)
		for _, row := range displayRows {
			t.Row(row...)
		}
		sb.WriteString(t.Render())

		// Overflow indicator
		if totalRows > uint64(len(displayRows)) {
			remaining := totalRows - uint64(len(displayRows))
			sb.WriteString("\n" + dimStyle().Render(fmt.Sprintf("  %d more rows", remaining)))
		}
	}

	// Cost/duration footer
	footer := fmt.Sprintf("  %s · %s · %s scanned",
		FormatCost(cost), FormatDuration(durationMs), FormatBytes(bytesScanned))
	sb.WriteString("\n" + dimStyle().Render(footer))

	display = sb.String()
	content = renderPlainTable(headers, displayRows, totalRows, maxDisplayRows, cost, durationMs, bytesScanned)

	return display, content
}

// RenderTableDetail renders a table's schema as formatted text.
func RenderTableDetail(detail *schema.TableDetail) (display string, content string) {
	if detail == nil {
		return "No table detail available.", "No table detail available."
	}

	var sb strings.Builder

	// Header: dataset.table
	header := fmt.Sprintf("%s.%s", detail.DatasetID, detail.TableID)
	sb.WriteString("  " + lipgloss.NewStyle().Foreground(brightColor()).Bold(true).Render(header))
	sb.WriteString("\n")

	// Metadata line
	meta := fmt.Sprintf("  Type: %s · Rows: %s · Size: %s",
		detail.TableType, formatRowCount(detail.RowCount), FormatBytes(detail.SizeBytes))
	sb.WriteString(dimStyle().Render(meta))
	sb.WriteString("\n")

	// Partition/Clustering info
	if detail.PartitionField != "" || len(detail.ClusteringFields) > 0 {
		var parts []string
		if detail.PartitionField != "" {
			parts = append(parts, fmt.Sprintf("Partitioned by: %s", detail.PartitionField))
		}
		if len(detail.ClusteringFields) > 0 {
			parts = append(parts, fmt.Sprintf("Clustered by: %s", strings.Join(detail.ClusteringFields, ", ")))
		}
		sb.WriteString(dimStyle().Render("  " + strings.Join(parts, " · ")))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Columns table
	if len(detail.Columns) > 0 {
		sb.WriteString(dimStyle().Render(fmt.Sprintf("  Columns (%d)", len(detail.Columns))))
		sb.WriteString("\n")

		t := cascadeTable([]string{"Column", "Type", "Nullable", "Description"})
		for _, col := range detail.Columns {
			nullable := "NO"
			if col.IsNullable {
				nullable = "YES"
			}
			desc := col.Description
			if col.IsPartitioning {
				if desc != "" {
					desc += " "
				}
				desc += "(partition key)"
			}
			if col.ClusteringOrdinal > 0 {
				if desc != "" {
					desc += " "
				}
				desc += fmt.Sprintf("(cluster %d)", col.ClusteringOrdinal)
			}
			t.Row(col.Name, col.DataType, nullable, desc)
		}

		sb.WriteString(t.Render())
	}

	display = sb.String()

	// Plain text content for LLM
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("%s.%s\n", detail.DatasetID, detail.TableID))
	cb.WriteString(fmt.Sprintf("Type: %s | Rows: %s | Size: %s\n",
		detail.TableType, formatRowCount(detail.RowCount), FormatBytes(detail.SizeBytes)))
	if detail.PartitionField != "" {
		cb.WriteString(fmt.Sprintf("Partitioned by: %s\n", detail.PartitionField))
	}
	if len(detail.ClusteringFields) > 0 {
		cb.WriteString(fmt.Sprintf("Clustered by: %s\n", strings.Join(detail.ClusteringFields, ", ")))
	}
	cb.WriteString(fmt.Sprintf("\nColumns (%d):\n", len(detail.Columns)))
	for _, col := range detail.Columns {
		nullable := "NO"
		if col.IsNullable {
			nullable = "YES"
		}
		cb.WriteString(fmt.Sprintf("  %-30s %-15s %-8s %s\n", col.Name, col.DataType, nullable, col.Description))
	}
	content = cb.String()

	return display, content
}

// RenderDatasetList renders a list of datasets.
func RenderDatasetList(datasets []schema.DatasetInfo, projectID string) (display string, content string) {
	if len(datasets) == 0 {
		msg := "No datasets found."
		return msg, msg
	}

	var sb strings.Builder

	header := fmt.Sprintf("Datasets in %s", projectID)
	sb.WriteString("  " + headerStyle().Render(header))
	sb.WriteString("\n\n")

	for _, ds := range datasets {
		name := lipgloss.NewStyle().Foreground(brightColor()).Bold(true).Render(ds.DatasetID)
		meta := dimStyle().Render(fmt.Sprintf("%d tables · %s", ds.TableCount, FormatBytes(ds.TotalBytes)))
		sb.WriteString(fmt.Sprintf("    %s  %s\n", name, meta))
	}

	display = sb.String()

	// Plain text content
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("Datasets in project %s:\n", projectID))
	for _, ds := range datasets {
		cb.WriteString(fmt.Sprintf("  %s  %d tables  %s\n", ds.DatasetID, ds.TableCount, FormatBytes(ds.TotalBytes)))
	}
	content = cb.String()

	return display, content
}

// RenderColumnSearch renders column search results.
func RenderColumnSearch(results []schema.ColumnSearchResult, query string) (display string, content string) {
	if len(results) == 0 {
		msg := fmt.Sprintf("No columns matching %q.", query)
		return msg, msg
	}

	var sb strings.Builder

	header := fmt.Sprintf("Columns matching %q", query)
	sb.WriteString("  " + headerStyle().Render(header))
	sb.WriteString("\n")

	t := cascadeTable([]string{"Column Path", "Type", "Description"})
	for _, r := range results {
		path := fmt.Sprintf("%s.%s.%s", r.DatasetID, r.TableID, r.ColumnName)
		t.Row(path, r.DataType, r.Description)
	}
	sb.WriteString(t.Render())

	display = sb.String()

	// Plain text content
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("Columns matching %q:\n", query))
	for _, r := range results {
		path := fmt.Sprintf("%s.%s.%s", r.DatasetID, r.TableID, r.ColumnName)
		cb.WriteString(fmt.Sprintf("  %-45s %-10s %s\n", path, r.DataType, r.Description))
	}
	content = cb.String()

	return display, content
}

// RenderTableList renders a list of tables in a dataset.
func RenderTableList(tables []schema.TableInfo, datasetID string) (display string, content string) {
	if len(tables) == 0 {
		msg := fmt.Sprintf("No tables found in dataset %s.", datasetID)
		return msg, msg
	}

	var sb strings.Builder

	header := fmt.Sprintf("Tables in %s", datasetID)
	sb.WriteString("  " + headerStyle().Render(header))
	sb.WriteString("\n")

	t := cascadeTable([]string{"Table", "Type", "Rows", "Size", "Partition"})
	for _, ti := range tables {
		partition := ti.PartitionField
		if partition == "" {
			partition = "-"
		}
		t.Row(ti.TableID, ti.TableType, formatRowCount(ti.RowCount), FormatBytes(ti.SizeBytes), partition)
	}
	sb.WriteString(t.Render())

	display = sb.String()

	// Plain text content
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("Tables in %s:\n", datasetID))
	for _, ti := range tables {
		cb.WriteString(fmt.Sprintf("  %-30s %-8s %s rows  %s\n", ti.TableID, ti.TableType, formatRowCount(ti.RowCount), FormatBytes(ti.SizeBytes)))
	}
	content = cb.String()

	return display, content
}

// FormatBytes formats byte count as human-readable (KB, MB, GB, TB).
func FormatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case bytes < kb:
		return fmt.Sprintf("%d B", bytes)
	case bytes < mb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	case bytes < gb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes < tb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	default:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	}
}

// FormatCost formats dollar amount.
func FormatCost(cost float64) string {
	if cost < 0 {
		return "N/A (DML)"
	}
	if cost < 0.01 {
		return "$0.00"
	}
	return fmt.Sprintf("$%.2f", cost)
}

// FormatDuration formats milliseconds as "Xms", "X.Xs", or "Xm Xs".
func FormatDuration(ms int64) string {
	switch {
	case ms < 1000:
		return fmt.Sprintf("%dms", ms)
	case ms < 60000:
		return fmt.Sprintf("%.1fs", float64(ms)/1000)
	default:
		m := ms / 60000
		s := (ms % 60000) / 1000
		return fmt.Sprintf("%dm %ds", m, s)
	}
}

// RenderOptimizationHints formats optimization hints for display and LLM content.
func RenderOptimizationHints(hints []bq.OptimizationHint) (display string, content string) {
	if len(hints) == 0 {
		return "", ""
	}

	var displayBuf, contentBuf strings.Builder

	displayBuf.WriteString("\n")
	displayBuf.WriteString(lipgloss.NewStyle().Bold(true).Foreground(warningColor()).Render("  Optimization Suggestions"))
	displayBuf.WriteString("\n")

	contentBuf.WriteString("\n--- Optimization Suggestions ---\n")

	for i, h := range hints {
		icon := "INFO"
		if h.Severity == "warning" {
			icon = "WARN"
		}

		displayLine := fmt.Sprintf("  [%s] %s", icon, h.Message)
		displayBuf.WriteString(displayLine)

		contentBuf.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, strings.ToUpper(h.Category), h.Message))

		if i < len(hints)-1 {
			displayBuf.WriteString("\n")
		}
	}

	return displayBuf.String(), contentBuf.String()
}

// renderPlainTable builds a plain-text table without ANSI for LLM consumption.
func renderPlainTable(headers []string, rows [][]string, totalRows uint64, maxDisplayRows int, cost float64, durationMs int64, bytesScanned int64) string {
	var sb strings.Builder

	// Plain text limited to 50 rows for LLM context.
	llmRows := rows
	if len(llmRows) > 50 {
		llmRows = llmRows[:50]
	}

	sb.WriteString(strings.Join(headers, "\t"))
	sb.WriteString("\n")

	for _, row := range llmRows {
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteString("\n")
	}

	if totalRows > uint64(len(llmRows)) {
		sb.WriteString(fmt.Sprintf("(%d more rows)\n", totalRows-uint64(len(llmRows))))
	}

	sb.WriteString(fmt.Sprintf("%s | %s | %s scanned\n",
		FormatCost(cost), FormatDuration(durationMs), FormatBytes(bytesScanned)))

	return sb.String()
}

// formatRowCount formats a row count with comma separators.
func formatRowCount(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var b strings.Builder
	remainder := len(s) % 3
	if remainder > 0 {
		b.WriteString(s[:remainder])
	}
	for i := remainder; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
