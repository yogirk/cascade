// Package bigquery provides BigQuery tool implementations for Cascade.
package bigquery

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	bq "github.com/slokam-ai/cascade/internal/bigquery"
	"github.com/slokam-ai/cascade/internal/render"
	"github.com/slokam-ai/cascade/internal/schema"
)

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
		sb.WriteString(render.DimStyle().Render(fmt.Sprintf("  %d columns × %d rows (table too wide for display)", len(headers), totalRows)))
	} else {
		t := render.CascadeTable(headers)
		for _, row := range displayRows {
			t.Row(row...)
		}
		sb.WriteString(t.Render())

		// Overflow indicator
		if totalRows > uint64(len(displayRows)) {
			remaining := totalRows - uint64(len(displayRows))
			sb.WriteString("\n" + render.DimStyle().Render(fmt.Sprintf("  %d more rows", remaining)))
		}
	}

	// Cost/duration footer
	footer := fmt.Sprintf("  %s · %s · %s scanned",
		FormatCost(cost), render.FormatDuration(durationMs), render.FormatBytes(bytesScanned))
	sb.WriteString("\n" + render.DimStyle().Render(footer))

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
	sb.WriteString("  " + render.BrightStyle().Render(header))
	sb.WriteString("\n")

	// Metadata line
	meta := fmt.Sprintf("  Type: %s · Rows: %s · Size: %s",
		detail.TableType, render.FormatRowCount(detail.RowCount), render.FormatBytes(detail.SizeBytes))
	sb.WriteString(render.DimStyle().Render(meta))
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
		sb.WriteString(render.DimStyle().Render("  " + strings.Join(parts, " · ")))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Columns table
	if len(detail.Columns) > 0 {
		sb.WriteString(render.DimStyle().Render(fmt.Sprintf("  Columns (%d)", len(detail.Columns))))
		sb.WriteString("\n")

		t := render.CascadeTable([]string{"Column", "Type", "Nullable", "Description"})
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
		detail.TableType, render.FormatRowCount(detail.RowCount), render.FormatBytes(detail.SizeBytes)))
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
	sb.WriteString("  " + render.HeaderStyle().Render(header))
	sb.WriteString("\n\n")

	for _, ds := range datasets {
		name := lipgloss.NewStyle().Foreground(render.BrightColor()).Bold(true).Render(ds.DatasetID)
		meta := render.DimStyle().Render(fmt.Sprintf("%d tables · %s", ds.TableCount, render.FormatBytes(ds.TotalBytes)))
		sb.WriteString(fmt.Sprintf("    %s  %s\n", name, meta))
	}

	display = sb.String()

	// Plain text content
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("Datasets in project %s:\n", projectID))
	for _, ds := range datasets {
		cb.WriteString(fmt.Sprintf("  %s  %d tables  %s\n", ds.DatasetID, ds.TableCount, render.FormatBytes(ds.TotalBytes)))
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
	sb.WriteString("  " + render.HeaderStyle().Render(header))
	sb.WriteString("\n")

	t := render.CascadeTable([]string{"Column Path", "Type", "Description"})
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
	sb.WriteString("  " + render.HeaderStyle().Render(header))
	sb.WriteString("\n")

	t := render.CascadeTable([]string{"Table", "Type", "Rows", "Size", "Partition"})
	for _, ti := range tables {
		partition := ti.PartitionField
		if partition == "" {
			partition = "-"
		}
		t.Row(ti.TableID, ti.TableType, render.FormatRowCount(ti.RowCount), render.FormatBytes(ti.SizeBytes), partition)
	}
	sb.WriteString(t.Render())

	display = sb.String()

	// Plain text content
	var cb strings.Builder
	cb.WriteString(fmt.Sprintf("Tables in %s:\n", datasetID))
	for _, ti := range tables {
		cb.WriteString(fmt.Sprintf("  %-30s %-8s %s rows  %s\n", ti.TableID, ti.TableType, render.FormatRowCount(ti.RowCount), render.FormatBytes(ti.SizeBytes)))
	}
	content = cb.String()

	return display, content
}

// FormatBytes is re-exported so existing callers in the BQ tool surface
// keep compiling. Source of truth is internal/render.
func FormatBytes(bytes int64) string { return render.FormatBytes(bytes) }

// FormatCost formats dollar amount with BQ-specific DML semantics.
func FormatCost(cost float64) string {
	if cost < 0 {
		return "N/A (DML)"
	}
	if cost < 0.01 {
		return "$0.00"
	}
	return fmt.Sprintf("$%.2f", cost)
}

// FormatDuration is re-exported so existing callers keep compiling.
func FormatDuration(ms int64) string { return render.FormatDuration(ms) }

// RenderOptimizationHints formats optimization hints for display and LLM content.
func RenderOptimizationHints(hints []bq.OptimizationHint) (display string, content string) {
	if len(hints) == 0 {
		return "", ""
	}

	var displayBuf, contentBuf strings.Builder

	displayBuf.WriteString("\n")
	displayBuf.WriteString(render.WarningStyle().Render("  Optimization Suggestions"))
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
