// Package bigquery provides BigQuery tool implementations for Cascade.
package bigquery

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"

	"github.com/yogirk/cascade/internal/schema"
)

// Colors hardcoded here since render.go is not in the tui package.
// These match the dark-terminal palette from internal/tui/styles.go.
var (
	borderColor = lipgloss.Color("#374151")
	accentColor = lipgloss.Color("#6B9FFF")
	brightColor = lipgloss.Color("#F3F4F6")
	dimColor    = lipgloss.Color("#4B5563")
	textColor   = lipgloss.Color("#D1D5DB")
)

// RenderQueryResults renders query results as a styled Lipgloss table.
// Returns both a Display string (styled for TUI) and a Content string (plain text for LLM).
func RenderQueryResults(headers []string, rows [][]string, totalRows uint64, maxDisplayRows int, cost float64, durationMs int64, bytesScanned int64) (display string, content string) {
	if len(headers) == 0 {
		msg := "Query returned no columns."
		return msg, msg
	}

	// Determine how many rows to display.
	displayRows := rows
	if len(displayRows) > maxDisplayRows {
		displayRows = displayRows[:maxDisplayRows]
	}

	// Build styled display table.
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(borderColor)).
		Headers(headers...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Foreground(accentColor).
					Bold(true).
					Padding(0, 1)
			}
			return lipgloss.NewStyle().
				Foreground(textColor).
				Padding(0, 1)
		}).
		Wrap(false)

	for _, row := range displayRows {
		t.Row(row...)
	}

	// Cap table width at 120.
	t.Width(120)

	var sb strings.Builder
	sb.WriteString(t.Render())

	// Overflow indicator.
	if totalRows > uint64(len(displayRows)) {
		remaining := totalRows - uint64(len(displayRows))
		sb.WriteString(fmt.Sprintf("\n  %d more rows", remaining))
	}

	// Cost/duration footer.
	footer := fmt.Sprintf("  %s | %s | %s scanned",
		FormatCost(cost), FormatDuration(durationMs), FormatBytes(bytesScanned))
	sb.WriteString("\n" + footer)

	display = sb.String()

	// Build plain-text content for LLM (no ANSI).
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
	header := fmt.Sprintf("  %s.%s", detail.DatasetID, detail.TableID)
	sb.WriteString(lipgloss.NewStyle().Foreground(brightColor).Bold(true).Render(header))
	sb.WriteString("\n")

	// Metadata line.
	meta := fmt.Sprintf("  Type: %s | Rows: %s | Size: %s",
		detail.TableType, formatRowCount(detail.RowCount), FormatBytes(detail.SizeBytes))
	sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(meta))
	sb.WriteString("\n")

	// Partition/Clustering info.
	if detail.PartitionField != "" || len(detail.ClusteringFields) > 0 {
		var parts []string
		if detail.PartitionField != "" {
			parts = append(parts, fmt.Sprintf("Partitioned by: %s", detail.PartitionField))
		}
		if len(detail.ClusteringFields) > 0 {
			parts = append(parts, fmt.Sprintf("Clustered by: %s", strings.Join(detail.ClusteringFields, ", ")))
		}
		partLine := "  " + strings.Join(parts, " | ")
		sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(partLine))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Columns table.
	if len(detail.Columns) > 0 {
		sb.WriteString(fmt.Sprintf("  Columns (%d):\n", len(detail.Columns)))

		colHeaders := []string{"Column Name", "Type", "Nullable", "Description"}
		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(borderColor)).
			Headers(colHeaders...).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return lipgloss.NewStyle().
						Foreground(accentColor).
						Bold(true).
						Padding(0, 1)
				}
				return lipgloss.NewStyle().
					Foreground(textColor).
					Padding(0, 1)
			}).
			Wrap(false)

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

		t.Width(120)
		sb.WriteString(t.Render())
	}

	display = sb.String()

	// Plain text content for LLM.
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

	header := fmt.Sprintf("  Datasets in project %s:", projectID)
	sb.WriteString(lipgloss.NewStyle().Foreground(brightColor).Bold(true).Render(header))
	sb.WriteString("\n")

	for _, ds := range datasets {
		line := fmt.Sprintf("    %s  %d tables  %s",
			lipgloss.NewStyle().Foreground(brightColor).Bold(true).Render(ds.DatasetID),
			ds.TableCount,
			FormatBytes(ds.TotalBytes))
		sb.WriteString(lipgloss.NewStyle().Foreground(dimColor).Render(line))
		sb.WriteString("\n")
	}

	display = sb.String()

	// Plain text content.
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

	header := fmt.Sprintf("  Columns matching %q:", query)
	sb.WriteString(lipgloss.NewStyle().Foreground(brightColor).Bold(true).Render(header))
	sb.WriteString("\n")

	for _, r := range results {
		path := fmt.Sprintf("%s.%s.%s", r.DatasetID, r.TableID, r.ColumnName)
		line := fmt.Sprintf("    %-45s %-10s %s", path, r.DataType, r.Description)
		sb.WriteString(lipgloss.NewStyle().Foreground(textColor).Render(line))
		sb.WriteString("\n")
	}

	display = sb.String()

	// Plain text content.
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

	header := fmt.Sprintf("  Tables in %s:", datasetID)
	sb.WriteString(lipgloss.NewStyle().Foreground(brightColor).Bold(true).Render(header))
	sb.WriteString("\n")

	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(borderColor)).
		Headers("Table", "Type", "Rows", "Size", "Partition").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Foreground(accentColor).
					Bold(true).
					Padding(0, 1)
			}
			return lipgloss.NewStyle().
				Foreground(textColor).
				Padding(0, 1)
		}).
		Wrap(false)

	for _, ti := range tables {
		partition := ti.PartitionField
		if partition == "" {
			partition = "-"
		}
		t.Row(ti.TableID, ti.TableType, formatRowCount(ti.RowCount), FormatBytes(ti.SizeBytes), partition)
	}

	t.Width(120)
	sb.WriteString(t.Render())

	display = sb.String()

	// Plain text content.
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

// renderPlainTable builds a plain-text table without ANSI for LLM consumption.
func renderPlainTable(headers []string, rows [][]string, totalRows uint64, maxDisplayRows int, cost float64, durationMs int64, bytesScanned int64) string {
	var sb strings.Builder

	// Plain text limited to 50 rows for LLM context.
	llmRows := rows
	if len(llmRows) > 50 {
		llmRows = llmRows[:50]
	}

	// Header line.
	sb.WriteString(strings.Join(headers, "\t"))
	sb.WriteString("\n")

	// Data rows.
	for _, row := range llmRows {
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteString("\n")
	}

	// Overflow.
	if totalRows > uint64(len(llmRows)) {
		sb.WriteString(fmt.Sprintf("(%d more rows)\n", totalRows-uint64(len(llmRows))))
	}

	// Footer.
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
