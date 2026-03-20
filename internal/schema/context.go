package schema

import (
	"fmt"
	"strings"

	bq "github.com/yogirk/cascade/internal/bigquery"
)

// BuildSchemaContext searches the cache for tables matching the query and
// returns a formatted string suitable for LLM system prompt injection.
// maxTables limits how many tables to include (default 10 if <= 0).
func BuildSchemaContext(cache *Cache, query string, maxTables int) (string, error) {
	if maxTables <= 0 {
		maxTables = 10
	}

	refs, err := cache.Search(query, maxTables)
	if err != nil {
		return "", fmt.Errorf("search: %w", err)
	}

	if len(refs) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("## Available Tables\n\n")

	for _, ref := range refs {
		detail, err := cache.GetTableDetail(ref.DatasetID, ref.TableID)
		if err != nil {
			continue // Skip tables that can't be retrieved.
		}

		// Header: dataset.table_name
		fmt.Fprintf(&b, "### %s.%s\n", ref.DatasetID, ref.TableID)

		// Metadata line.
		fmt.Fprintf(&b, "Type: %s", detail.TableType)
		if detail.RowCount > 0 {
			fmt.Fprintf(&b, " | Rows: %s", bq.FormatRowCount(detail.RowCount))
		}
		if detail.SizeBytes > 0 {
			fmt.Fprintf(&b, " | Size: %s", bq.FormatBytes(detail.SizeBytes))
		}
		b.WriteString("\n")

		// Partitioning and clustering.
		if detail.PartitionField != "" {
			fmt.Fprintf(&b, "Partitioned by: %s", detail.PartitionField)
			b.WriteString("\n")
		}
		if len(detail.ClusteringFields) > 0 {
			fmt.Fprintf(&b, "Clustered by: %s", strings.Join(detail.ClusteringFields, ", "))
			b.WriteString("\n")
		}

		// Description.
		if detail.Description != "" {
			fmt.Fprintf(&b, "Description: %s\n", detail.Description)
		}

		// Columns.
		b.WriteString("\nColumns:\n")
		for _, col := range detail.Columns {
			nullable := ""
			if !col.IsNullable {
				nullable = " NOT NULL"
			}

			extra := ""
			if col.IsPartitioning {
				extra += " (partition key)"
			}
			if col.ClusteringOrdinal > 0 {
				extra += fmt.Sprintf(" (cluster %d)", col.ClusteringOrdinal)
			}

			desc := ""
			if col.Description != "" {
				desc = " -- " + col.Description
			}

			fmt.Fprintf(&b, "- %s %s%s%s%s\n", col.Name, col.DataType, nullable, extra, desc)
		}

		b.WriteString("\n")
	}

	return b.String(), nil
}

// BuildDatasetSummary returns a compact summary of all cached datasets.
func BuildDatasetSummary(cache *Cache) (string, error) {
	datasets, err := cache.GetDatasets()
	if err != nil {
		return "", err
	}

	if len(datasets) == 0 {
		return "No datasets cached.", nil
	}

	var b strings.Builder
	if len(datasets) > 0 {
		fmt.Fprintf(&b, "Project: %s\n", datasets[0].ProjectID)
	}
	b.WriteString("Datasets: ")

	parts := make([]string, len(datasets))
	for i, d := range datasets {
		parts[i] = fmt.Sprintf("%s (%d tables, %s)", d.DatasetID, d.TableCount, bq.FormatBytes(d.TotalBytes))
	}
	b.WriteString(strings.Join(parts, ", "))

	return b.String(), nil
}
