// Package bigquery provides a BigQuery client wrapper, SQL risk classification,
// and session cost tracking for Cascade.
package bigquery

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
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

// QueryToCSV runs a SELECT and dumps every row to a freshly-created
// temp CSV file. The caller owns the file from the moment QueryToCSV
// returns — it is responsible for `os.Remove` on success and cleanup on
// error. The header row is written first so DuckDB's `read_csv_auto`
// picks up column names without further plumbing.
//
// Used by the DuckDB integration's `mode=local` path: small BQ pulls
// where the GCS round-trip is more friction than it's worth. Streaming
// is row-by-row through encoding/csv, so memory is bounded — but
// `mode=local` is still gated to ~1 GiB by VolumeGate because each
// cell becomes a string for CSV serialization (5x-10x storage blow-up
// vs. Parquet's columnar+compressed format).
//
// NULL convention: empty unquoted cell. Pairs with `read_csv_auto`'s
// default `nullstr=''` so NULLs survive the round-trip cleanly.
func (c *Client) QueryToCSV(ctx context.Context, sql string) (path string, rows int64, err error) {
	q := c.bq.Query(sql)
	it, err := q.Read(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("query read: %w", err)
	}

	f, err := os.CreateTemp("", "cascade-bq-*.csv")
	if err != nil {
		return "", 0, fmt.Errorf("create temp csv: %w", err)
	}
	// On any failure past this point we delete the half-written file —
	// returning a useless path to the caller would be worse than
	// returning an error.
	cleanup := func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}

	w := csv.NewWriter(f)

	// Pull the first row to populate it.Schema (BQ doesn't fill the
	// schema until iteration has started).
	var firstRow []bigquery.Value
	first := it.Next(&firstRow)
	if first != nil && first != iterator.Done {
		cleanup()
		return "", 0, fmt.Errorf("first row: %w", first)
	}

	if it.Schema == nil {
		cleanup()
		return "", 0, fmt.Errorf("no schema returned from query")
	}

	headers := make([]string, len(it.Schema))
	for i, field := range it.Schema {
		headers[i] = field.Name
	}
	if err := w.Write(headers); err != nil {
		cleanup()
		return "", 0, fmt.Errorf("write header: %w", err)
	}

	if first != iterator.Done {
		if err := w.Write(rowToCSV(firstRow)); err != nil {
			cleanup()
			return "", 0, fmt.Errorf("write first row: %w", err)
		}
		rows++
	}

	for {
		var row []bigquery.Value
		switch err := it.Next(&row); err {
		case nil:
			if err := w.Write(rowToCSV(row)); err != nil {
				cleanup()
				return "", 0, fmt.Errorf("write row %d: %w", rows, err)
			}
			rows++
		case iterator.Done:
			w.Flush()
			if err := w.Error(); err != nil {
				cleanup()
				return "", 0, fmt.Errorf("flush csv: %w", err)
			}
			if err := f.Close(); err != nil {
				_ = os.Remove(f.Name())
				return "", 0, fmt.Errorf("close csv: %w", err)
			}
			return f.Name(), rows, nil
		default:
			cleanup()
			return "", 0, fmt.Errorf("row iteration: %w", err)
		}
	}
}

// rowToCSV converts a single BQ row to the [] string form encoding/csv
// expects. NULLs become empty strings (CSV-standard) so DuckDB's
// read_csv_auto restores them as NULLs without configuration.
func rowToCSV(row []bigquery.Value) []string {
	out := make([]string, len(row))
	for i, v := range row {
		if v == nil {
			out[i] = ""
			continue
		}
		out[i] = valueToString(v)
	}
	return out
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
