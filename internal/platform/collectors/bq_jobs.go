package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/slokam-ai/cascade/internal/bigquery"
	"github.com/slokam-ai/cascade/internal/platform"
)

// BQJobCollector collects failed BigQuery job signals from INFORMATION_SCHEMA.
type BQJobCollector struct {
	client         *bigquery.Client
	projectID      string
	location       string
	criticalTables []string
}

// NewBQJobCollector creates a collector for failed BigQuery jobs.
// criticalTables is a list of fully-qualified table refs ("project.dataset.table")
// whose failures are escalated to Critical severity.
// If client is nil, Collect gracefully returns no signals.
func NewBQJobCollector(client *bigquery.Client, projectID, location string, criticalTables []string) *BQJobCollector {
	return &BQJobCollector{
		client:         client,
		projectID:      projectID,
		location:       location,
		criticalTables: criticalTables,
	}
}

// Source returns the signal source identifier.
func (c *BQJobCollector) Source() platform.SignalSource {
	return platform.SourceBigQuery
}

// Collect queries INFORMATION_SCHEMA.JOBS_BY_PROJECT for failed jobs within the
// given time window and converts each into a platform.Signal.
func (c *BQJobCollector) Collect(ctx context.Context, since time.Duration) ([]platform.Signal, error) {
	if c.client == nil {
		return nil, nil
	}

	hours := int(since.Hours())
	if hours < 1 {
		hours = 1
	}

	query := fmt.Sprintf(`
		SELECT
			job_id,
			error_result.reason AS error_reason,
			error_result.message AS error_message,
			creation_time,
			COALESCE(destination_table.project_id, '') AS dest_project,
			COALESCE(destination_table.dataset_id, '') AS dest_dataset,
			COALESCE(destination_table.table_id, '') AS dest_table,
			COALESCE(query, '') AS statement
		FROM %s.INFORMATION_SCHEMA.JOBS_BY_PROJECT
		WHERE error_result IS NOT NULL
		  AND creation_time > TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL %d HOUR)
		ORDER BY creation_time DESC
	`, fmt.Sprintf("`region-%s`", c.location), hours)

	headers, rows, _, _, err := c.client.ExecuteQuery(ctx, query, 500)
	if err != nil {
		return nil, fmt.Errorf("query failed jobs: %w", err)
	}

	// Build column index map for resilience against column order changes.
	colIdx := make(map[string]int, len(headers))
	for i, h := range headers {
		colIdx[h] = i
	}

	criticalSet := make(map[string]bool, len(c.criticalTables))
	for _, t := range c.criticalTables {
		criticalSet[t] = true
	}

	var signals []platform.Signal
	for _, row := range rows {
		jobID := colVal(row, colIdx, "job_id")
		errorReason := colVal(row, colIdx, "error_reason")
		errorMessage := colVal(row, colIdx, "error_message")
		creationTime := colVal(row, colIdx, "creation_time")
		destProject := colVal(row, colIdx, "dest_project")
		destDataset := colVal(row, colIdx, "dest_dataset")
		destTable := colVal(row, colIdx, "dest_table")
		statement := colVal(row, colIdx, "statement")

		ts, _ := time.Parse(time.RFC3339, creationTime)

		// Build destination table ref.
		var destRef string
		if destProject != "" && destDataset != "" && destTable != "" {
			destRef = fmt.Sprintf("%s.%s.%s", destProject, destDataset, destTable)
		}

		// Determine severity.
		severity := platform.SeverityWarning
		if destRef != "" && criticalSet[destRef] {
			severity = platform.SeverityCritical
		}

		// Truncate statement for summary.
		stmtSnippet := statement
		if len(stmtSnippet) > 80 {
			stmtSnippet = stmtSnippet[:80]
		}

		summary := fmt.Sprintf("[%s] on job %s: %s", errorReason, jobID, stmtSnippet)

		// Collect related resources.
		var related []string
		if destRef != "" {
			related = append(related, destRef)
		}

		sig := platform.Signal{
			Type:        platform.SignalJobFailed,
			Severity:    severity,
			Source:      platform.SourceBigQuery,
			Timestamp:   ts,
			Summary:     summary,
			Details: map[string]any{
				"job_id":        jobID,
				"error_reason":  errorReason,
				"error_message": errorMessage,
				"statement":     statement,
			},
			Related:     related,
			BlastRadius: 0,
		}
		signals = append(signals, sig)
	}

	return signals, nil
}

// colVal safely retrieves a column value from a row by header name.
func colVal(row []string, colIdx map[string]int, name string) string {
	idx, ok := colIdx[name]
	if !ok || idx >= len(row) {
		return ""
	}
	v := row[idx]
	if v == "NULL" {
		return ""
	}
	return v
}
