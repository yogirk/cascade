package bigquery

import (
	"fmt"
	"regexp"
	"strings"
)

// OptimizationHint represents a suggestion for improving query performance.
type OptimizationHint struct {
	Category string // "partition_filter", "clustering_key", "expensive_join"
	Severity string // "warning", "info"
	Message  string // human-readable suggestion
	TableRef string // dataset.table that the hint applies to
}

// TableMeta holds the subset of table metadata needed for optimization analysis.
// This avoids importing the schema package (which already imports bigquery).
type TableMeta struct {
	DatasetID        string
	TableID          string
	SizeBytes        int64
	PartitionField   string
	ClusteringFields []string
}

// SchemaLookup provides table metadata for optimization analysis.
type SchemaLookup interface {
	GetTableMeta(datasetID, tableID string) (*TableMeta, error)
	ListTableMeta(datasetID string) ([]TableMeta, error)
}

// AnalyzeQuery inspects a SQL query against schema metadata and returns optimization hints.
// It checks for: missing partition filters, unused clustering keys, and expensive JOINs.
// Returns nil/empty slice if no issues found or if the query is not a SELECT.
func AnalyzeQuery(sql string, lookup SchemaLookup) []OptimizationHint {
	if lookup == nil {
		return nil
	}

	// Only analyze SELECT queries (including WITH/CTE).
	normalized := stripSQLComments(strings.TrimSpace(sql))
	upper := strings.ToUpper(strings.TrimSpace(normalized))
	if !hasAnyPrefix(upper, "SELECT", "WITH", "EXPLAIN") {
		return nil
	}

	var hints []OptimizationHint

	// Extract table references from SQL.
	tableRefs := extractTableRefs(sql)

	for _, ref := range tableRefs {
		meta, err := lookup.GetTableMeta(ref.dataset, ref.table)
		if err != nil || meta == nil {
			continue
		}

		// Check 1: Missing partition filter.
		if meta.PartitionField != "" {
			if !referencesColumn(sql, meta.PartitionField) {
				hints = append(hints, OptimizationHint{
					Category: "partition_filter",
					Severity: "warning",
					Message: fmt.Sprintf(
						"Table %s.%s is partitioned on '%s' but no filter on this column was detected. "+
							"Adding a WHERE clause on '%s' can significantly reduce bytes scanned and cost.",
						ref.dataset, ref.table, meta.PartitionField, meta.PartitionField),
					TableRef: ref.dataset + "." + ref.table,
				})
			}
		}

		// Check 2: Unused clustering keys.
		if len(meta.ClusteringFields) > 0 {
			var unused []string
			for _, cf := range meta.ClusteringFields {
				if !referencesColumn(sql, cf) {
					unused = append(unused, cf)
				}
			}
			if len(unused) > 0 {
				hints = append(hints, OptimizationHint{
					Category: "clustering_key",
					Severity: "info",
					Message: fmt.Sprintf(
						"Table %s.%s is clustered on [%s] but these columns are not in the query: %s. "+
							"Filtering on clustering keys can improve performance.",
						ref.dataset, ref.table,
						strings.Join(meta.ClusteringFields, ", "),
						strings.Join(unused, ", ")),
					TableRef: ref.dataset + "." + ref.table,
				})
			}
		}
	}

	// Check 3: Expensive JOINs (cross-dataset JOINs of large tables).
	if len(tableRefs) >= 2 && containsJoin(upper) {
		datasets := make(map[string]bool)
		var largeTables []analyzedTableRef
		for _, ref := range tableRefs {
			datasets[ref.dataset] = true
			tables, err := lookup.ListTableMeta(ref.dataset)
			if err != nil {
				continue
			}
			for _, t := range tables {
				if t.TableID == ref.table && t.SizeBytes > 1_000_000_000 { // > 1GB
					largeTables = append(largeTables, ref)
				}
			}
		}
		if len(datasets) > 1 && len(largeTables) >= 2 {
			hints = append(hints, OptimizationHint{
				Category: "expensive_join",
				Severity: "warning",
				Message: fmt.Sprintf(
					"Cross-dataset JOIN detected between large tables (%d tables >1GB). "+
						"Cross-dataset JOINs can be expensive. Consider materializing intermediate results or filtering early.",
					len(largeTables)),
				TableRef: "multiple",
			})
		}
	}

	return hints
}

// analyzedTableRef is a parsed dataset.table reference from SQL.
type analyzedTableRef struct {
	dataset string
	table   string
}

// tableRefPattern matches `project.dataset.table` or `dataset.table` patterns in SQL.
// Handles backtick-quoted and unquoted identifiers.
var tableRefPattern = regexp.MustCompile(
	`(?i)(?:FROM|JOIN)\s+` + "`?" + `(?:[\w-]+\.)?(\w+)\.(\w+)` + "`?")

// extractTableRefs finds all dataset.table references in FROM and JOIN clauses.
func extractTableRefs(sql string) []analyzedTableRef {
	matches := tableRefPattern.FindAllStringSubmatch(sql, -1)
	seen := make(map[string]bool)
	var refs []analyzedTableRef
	for _, m := range matches {
		if len(m) >= 3 {
			key := m[1] + "." + m[2]
			if !seen[key] {
				seen[key] = true
				refs = append(refs, analyzedTableRef{dataset: m[1], table: m[2]})
			}
		}
	}
	return refs
}

// referencesColumn checks if the SQL query references a column name
// (case-insensitive, as a whole word).
func referencesColumn(sql, column string) bool {
	pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(column) + `\b`)
	return pattern.MatchString(sql)
}

// containsJoin checks if the SQL contains a JOIN keyword.
func containsJoin(upperSQL string) bool {
	return strings.Contains(upperSQL, " JOIN ")
}
