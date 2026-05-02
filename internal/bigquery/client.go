// Package bigquery provides a BigQuery client wrapper, SQL risk classification,
// and session cost tracking for Cascade.
package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Client wraps the BigQuery client with Cascade-specific functionality.
type Client struct {
	bq         *bigquery.Client
	projectID  string
	location   string
	pricePerTB float64
}

// NewClient creates a new BigQuery client wrapper.
// pricePerTB is the on-demand cost per TB scanned (default 6.25 for US).
func NewClient(ctx context.Context, projectID, location string, ts oauth2.TokenSource, pricePerTB float64) (*Client, error) {
	bq, err := bigquery.NewClient(ctx, projectID, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("bigquery client: %w", err)
	}
	bq.Location = location

	if pricePerTB <= 0 {
		pricePerTB = 6.25
	}

	return &Client{
		bq:         bq,
		projectID:  projectID,
		location:   location,
		pricePerTB: pricePerTB,
	}, nil
}

// ProjectID returns the GCP project ID.
func (c *Client) ProjectID() string { return c.projectID }

// Close closes the underlying BigQuery client.
func (c *Client) Close() error { return c.bq.Close() }

// ExecuteQuery runs a SQL query and returns headers, string rows, total row count, and schema.
// maxRows limits how many rows are fetched and converted to strings.
func (c *Client) ExecuteQuery(ctx context.Context, sql string, maxRows int) (headers []string, rows [][]string, totalRows uint64, schema bigquery.Schema, err error) {
	q := c.bq.Query(sql)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, nil, 0, nil, fmt.Errorf("query read: %w", err)
	}

	// Read rows first — schema may not be populated until after the first Next() call.
	for i := 0; i < maxRows; i++ {
		var row []bigquery.Value
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, rows, it.TotalRows, it.Schema, fmt.Errorf("row iteration: %w", err)
		}
		rows = append(rows, valuesToStrings(row))
	}

	// Extract headers from schema (now populated after iteration).
	for _, field := range it.Schema {
		headers = append(headers, field.Name)
	}

	return headers, rows, it.TotalRows, it.Schema, nil
}

// EstimateCost performs a dry-run query and returns bytes processed and estimated cost.
// For DML statements where bytes processed is 0, returns -1 for cost to signal
// "cannot estimate".
func (c *Client) EstimateCost(ctx context.Context, sql string) (bytesProcessed int64, estimatedCost float64, err error) {
	q := c.bq.Query(sql)
	q.DryRun = true

	job, err := q.Run(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("dry-run failed: %w", err)
	}

	stats := job.LastStatus().Statistics
	bytes := stats.TotalBytesProcessed

	if bytes == 0 {
		// DML and some DDL statements return 0 bytes; cost cannot be estimated.
		return 0, -1, nil
	}

	cost := float64(bytes) / 1e12 * c.pricePerTB
	return bytes, cost, nil
}

// RunQuery executes a SQL query and returns the raw RowIterator.
// Used for INFORMATION_SCHEMA queries in schema population.
func (c *Client) RunQuery(ctx context.Context, sql string) (*bigquery.RowIterator, error) {
	q := c.bq.Query(sql)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("query read: %w", err)
	}
	return it, nil
}

// RunStatement runs a non-SELECT statement (EXPORT, DDL, DML) and waits
// for the job to complete. Returns the job's terminal status error if
// the job failed, or nil on success. Used by the DuckDB integration to
// drive `EXPORT DATA OPTIONS(...) AS <sql>` jobs without iterating an
// empty result set the way Read() would.
func (c *Client) RunStatement(ctx context.Context, sql string) error {
	q := c.bq.Query(sql)
	job, err := q.Run(ctx)
	if err != nil {
		return fmt.Errorf("submit job: %w", err)
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return fmt.Errorf("wait job: %w", err)
	}
	if status.Err() != nil {
		return fmt.Errorf("job failed: %w", status.Err())
	}
	return nil
}

// valuesToStrings converts a row of BigQuery values to string representations.
func valuesToStrings(row []bigquery.Value) []string {
	result := make([]string, len(row))
	for i, v := range row {
		result[i] = valueToString(v)
	}
	return result
}

// valueToString defensively converts a bigquery.Value to string.
func valueToString(v bigquery.Value) string {
	if v == nil {
		return "NULL"
	}

	switch val := v.(type) {
	case string:
		return val
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case time.Time:
		return val.Format(time.RFC3339)
	case []bigquery.Value:
		// Array or struct fields.
		parts := make([]string, len(val))
		for i, elem := range val {
			parts[i] = valueToString(elem)
		}
		data, err := json.Marshal(parts)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	default:
		// Fallback for any other types (civil.Date, civil.Time, etc.).
		return fmt.Sprintf("%v", val)
	}
}

// FormatBytes returns a human-readable byte size string.
func FormatBytes(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)

	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatRowCount returns a human-readable row count with comma separators.
func FormatRowCount(n int64) string {
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
