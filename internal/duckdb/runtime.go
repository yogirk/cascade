package duckdb

import (
	"context"
)

// Runtime is the engine abstraction Cascade speaks to. Two methods on
// purpose: Query returns a materialized read result; Exec runs DDL/DML
// where the only signal is success vs failure.
//
// The interface stays small deliberately. Today the only implementation
// is a subprocess wrapper around the duckdb CLI. A future MotherDuck
// HTTP client or embedded build tag could swap in here. The moment we
// reach for a third method (streaming, pagination, batch) is the moment
// to step back and ask whether we built the right abstraction.
type Runtime interface {
	Query(ctx context.Context, opts QueryOptions) (*QueryResult, error)
	Exec(ctx context.Context, opts ExecOptions) (*ExecResult, error)
}

// QueryOptions describes a read request.
type QueryOptions struct {
	// Database is the path to a .duckdb file. Empty selects an
	// in-memory database (":memory:").
	Database string

	// Init holds SQL statements written to a temporary -init file and
	// executed before SQL. Used for session-scoped state like
	// `SET extra_http_headers = '{"Authorization":"Bearer …"}'` for
	// httpfs GCS auth.
	Init []string

	// SQL is the read query. The classifier upstream is expected to have
	// confirmed it is read-only; the runtime does not re-classify.
	SQL string

	// MaxRows truncates the result to at most this many rows after
	// materialization. 0 disables truncation.
	MaxRows int
}

// ExecOptions describes a DDL/DML request.
type ExecOptions struct {
	Database string
	Init     []string
	SQL      string
}

// QueryResult is a materialized read result.
//
// Rows are stored as canonical string forms — DuckDB's -json mode
// already serializes BIGINT > 2^53, DECIMAL, and TIMESTAMP as strings,
// and Cascade's render and LLM-content paths both consume strings.
// The Columns slice carries DuckDB type names so callers that need
// typed values (rare today) can cast deliberately.
type QueryResult struct {
	Columns   []Column
	Rows      [][]string
	Truncated bool     // true if more rows existed than were returned
	TotalRows uint64   // total rows the query produced (or len(Rows) if unknown)
	Warnings  []string // NOTICE-level messages captured from stderr
}

// Column carries a column's name and DuckDB type as reported by DESCRIBE.
type Column struct {
	Name string
	Type string // e.g. "BIGINT", "VARCHAR", "TIMESTAMPTZ", "DECIMAL(38,9)"
}

// ExecResult is the success envelope for DDL/DML.
type ExecResult struct {
	Warnings []string
}

// SubprocessError is the structured error envelope returned when a duckdb
// CLI invocation exits non-zero. The agent can inspect ExitCode and
// StderrTail to decide between retrying, surfacing to the user, or
// asking the user for an alternate query.
type SubprocessError struct {
	ExitCode      int
	Stderr        string // tail of stderr (capped)
	PartialStdout int    // bytes of stdout we did capture before failure
	Cmd           string // command summary, redacted of init-file contents
}

func (e *SubprocessError) Error() string {
	return e.Stderr
}
