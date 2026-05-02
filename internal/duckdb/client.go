package duckdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Client is the subprocess-based Runtime implementation. It shells out
// to the `duckdb` CLI for every Query/Exec, writing per-call init files
// to a temp dir for any session-scoped state (e.g. EXTRA_HTTP_HEADERS
// for GCS auth).
//
// Why subprocess over CGo: keeping CGO_ENABLED=0 is a release-story
// invariant (single static binary, easy cross-compile, easy Homebrew).
// The duckdb CLI is everywhere users care about. The interface is small
// enough that swapping in MotherDuck-HTTP later is a contained change.
type Client struct {
	bin string // path to the duckdb CLI (resolved by Detect)

	// stderrTailBytes caps how much stderr we surface in error envelopes.
	// 4 KiB is enough to carry a typical duckdb error including a SQL
	// snippet, not so much that it floods the agent's context.
	stderrTailBytes int

	// stdoutCapBytes caps how much stdout we will read before treating the
	// response as oversized. The classifier-driven volume gate is meant to
	// catch this earlier; the cap here is a defensive backstop.
	stdoutCapBytes int64
}

// NewClient returns a Client wired to the given duckdb binary.
func NewClient(bin string) *Client {
	return &Client{
		bin:             bin,
		stderrTailBytes: 4 * 1024,
		stdoutCapBytes:  256 * 1024 * 1024, // 256 MiB hard ceiling
	}
}

// Compile-time interface check.
var _ Runtime = (*Client)(nil)

// Query runs a read-only statement and materializes the result.
//
// Implementation:
//  1. (optional) write Init SQL to a temp -init file.
//  2. Run `duckdb [-init …] <db> -json -c "DESCRIBE <SQL>"` to get the
//     authoritative column types — this is what saves us from JSON's
//     loss-of-fidelity for BIGINT > 2^53, DECIMAL, TIMESTAMP.
//  3. Run `duckdb [-init …] <db> -json -c "<SQL>"` for the data.
//  4. Decode JSON with UseNumber so wide ints survive as strings,
//     stringify each cell into the canonical form Cascade renders.
//  5. Truncate to MaxRows and report Truncated/TotalRows accordingly.
func (c *Client) Query(ctx context.Context, opts QueryOptions) (*QueryResult, error) {
	initPath, cleanup, err := writeInitFile(opts.Init)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	cols, descWarnings, err := c.runDescribe(ctx, opts.Database, initPath, opts.SQL)
	if err != nil {
		return nil, err
	}

	rows, dataWarnings, err := c.runJSONQuery(ctx, opts.Database, initPath, opts.SQL, cols)
	if err != nil {
		return nil, err
	}

	total := uint64(len(rows))
	truncated := false
	if opts.MaxRows > 0 && len(rows) > opts.MaxRows {
		rows = rows[:opts.MaxRows]
		truncated = true
	}

	return &QueryResult{
		Columns:   cols,
		Rows:      rows,
		Truncated: truncated,
		TotalRows: total,
		Warnings:  append(descWarnings, dataWarnings...),
	}, nil
}

// Exec runs a DDL/DML statement. We do not capture stdout in a typed
// form — duckdb prints "1 row affected"-style messages we surface as
// warnings if any, and the success signal is exit code 0.
func (c *Client) Exec(ctx context.Context, opts ExecOptions) (*ExecResult, error) {
	initPath, cleanup, err := writeInitFile(opts.Init)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	stdout, stderr, err := c.run(ctx, opts.Database, initPath, opts.SQL, false)
	if err != nil {
		return nil, err
	}
	return &ExecResult{Warnings: collectWarnings(stdout, stderr)}, nil
}

// runDescribe gets the column-name + column-type list for a SELECT.
// DuckDB's DESCRIBE on a query is a planning-only operation, so cost
// is negligible.
func (c *Client) runDescribe(ctx context.Context, db, initPath, sql string) ([]Column, []string, error) {
	descSQL := "DESCRIBE " + strings.TrimRight(strings.TrimSpace(sql), ";")
	stdout, stderr, err := c.run(ctx, db, initPath, descSQL, true)
	if err != nil {
		return nil, nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.UseNumber()
	var raw []map[string]any
	if err := dec.Decode(&raw); err != nil {
		// DuckDB sometimes emits a leading prelude line on stderr instead;
		// surface a helpful error rather than a cryptic decode failure.
		return nil, nil, fmt.Errorf("decode DESCRIBE output: %w (stderr: %s)", err, tail(string(stderr), 256))
	}

	cols := make([]Column, 0, len(raw))
	for _, row := range raw {
		name, _ := row["column_name"].(string)
		typ, _ := row["column_type"].(string)
		cols = append(cols, Column{Name: name, Type: typ})
	}
	return cols, collectWarnings(nil, stderr), nil
}

// runJSONQuery executes the actual SELECT and decodes -json output.
// Cell values are stringified using a canonical-form rule so the render
// and LLM-content paths can both consume them without further casting.
func (c *Client) runJSONQuery(ctx context.Context, db, initPath, sql string, cols []Column) ([][]string, []string, error) {
	stdout, stderr, err := c.run(ctx, db, initPath, sql, true)
	if err != nil {
		return nil, nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(stdout))
	dec.UseNumber()
	var raw []map[string]any
	if err := dec.Decode(&raw); err != nil {
		return nil, nil, fmt.Errorf("decode query output: %w (stderr: %s)", err, tail(string(stderr), 256))
	}

	rows := make([][]string, len(raw))
	for i, m := range raw {
		row := make([]string, len(cols))
		for j, col := range cols {
			row[j] = stringifyCell(m[col.Name])
		}
		rows[i] = row
	}
	return rows, collectWarnings(nil, stderr), nil
}

// run shells out to the duckdb CLI with the given SQL on -c.
//
// useJSON toggles `-json` mode. When db is "", duckdb opens an
// in-memory database. initPath, when non-empty, supplies SET-style
// session prelude.
//
// Cancellation: exec.CommandContext + applyProcessGroup so a Ctrl-C
// kills the duckdb subprocess and any child it spawned.
func (c *Client) run(ctx context.Context, db, initPath, sql string, useJSON bool) ([]byte, []byte, error) {
	args := []string{}
	if initPath != "" {
		args = append(args, "-init", initPath)
	}
	if useJSON {
		args = append(args, "-json")
	}
	args = append(args, "-c", sql)
	if db != "" {
		args = append(args, db)
	}

	cmd := exec.CommandContext(ctx, c.bin, args...)
	applyProcessGroup(cmd)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &cappedWriter{w: &stdoutBuf, max: c.stdoutCapBytes}
	cmd.Stderr = &cappedWriter{w: &stderrBuf, max: int64(c.stderrTailBytes) * 4} // give stderr more headroom

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start duckdb: %w", err)
	}
	runErr := cmd.Wait()

	if ctx.Err() != nil {
		if cmd.Process != nil {
			killProcessGroup(cmd.Process.Pid)
		}
		return nil, nil, ctx.Err()
	}

	stdout := stdoutBuf.Bytes()
	stderr := stderrBuf.Bytes()

	if runErr != nil {
		var exitErr *exec.ExitError
		exitCode := -1
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return nil, nil, &SubprocessError{
			ExitCode:      exitCode,
			Stderr:        tail(string(stderr), c.stderrTailBytes),
			PartialStdout: len(stdout),
			Cmd:           summarizeCmd(c.bin, args),
		}
	}

	return stdout, stderr, nil
}

// writeInitFile materializes the init prelude to a temp file and returns
// its path plus a cleanup func. If init is empty, returns ("", noop, nil).
//
// init files are written to os.TempDir with mode 0600 so other users on
// the box cannot read any bearer tokens we may have written.
func writeInitFile(init []string) (string, func(), error) {
	if len(init) == 0 {
		return "", func() {}, nil
	}
	f, err := os.CreateTemp("", "cascade-duckdb-init-*.sql")
	if err != nil {
		return "", func() {}, fmt.Errorf("create init file: %w", err)
	}
	for _, stmt := range init {
		s := strings.TrimRight(stmt, "; \n\r\t")
		if _, err := io.WriteString(f, s+";\n"); err != nil {
			_ = f.Close()
			_ = os.Remove(f.Name())
			return "", func() {}, fmt.Errorf("write init file: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", func() {}, fmt.Errorf("close init file: %w", err)
	}
	cleanup := func() { _ = os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// stringifyCell renders one JSON-decoded value into the canonical string
// form Cascade's render and LLM-content paths consume. UseNumber means
// numerics arrive as json.Number (not float64), preserving precision for
// wide ints. NULL → empty string is the BQ render convention.
func stringifyCell(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case json.Number:
		return x.String()
	default:
		// Fallback for the rare type we didn't anticipate. fmt %v gives a
		// stable, debuggable form rather than an error mid-render.
		return fmt.Sprintf("%v", v)
	}
}

// collectWarnings parses a stderr buffer for NOTICE-level messages.
// duckdb prefixes them with "Notice:" or "WARNING:" depending on
// severity; we surface both as warnings so the agent can decide.
func collectWarnings(_ /*stdout*/, stderr []byte) []string {
	if len(stderr) == 0 {
		return nil
	}
	var out []string
	for line := range strings.SplitSeq(string(stderr), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		upper := strings.ToUpper(trimmed)
		if strings.HasPrefix(upper, "NOTICE") || strings.HasPrefix(upper, "WARNING") {
			out = append(out, trimmed)
		}
	}
	return out
}

func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return "…" + s[len(s)-max:]
}

func summarizeCmd(bin string, args []string) string {
	// The init file's path leaks into args but its contents do not, which
	// is what we want for the redacted error envelope.
	return bin + " " + strings.Join(args, " ")
}

// cappedWriter forwards writes to its underlying writer until max bytes
// have been written, then silently drops further writes. Used to bound
// stdout/stderr capture so a runaway query cannot blow up Cascade's
// memory.
type cappedWriter struct {
	w       io.Writer
	max     int64
	written int64
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	if c.written >= c.max {
		return len(p), nil // pretend we accepted, drop the bytes
	}
	remaining := c.max - c.written
	if int64(len(p)) > remaining {
		n, err := c.w.Write(p[:remaining])
		c.written += int64(n)
		if err != nil {
			return n, err
		}
		// dropped tail bytes — surface as a no-error short-write that
		// callers don't see thanks to the lie above.
		return len(p), nil
	}
	n, err := c.w.Write(p)
	c.written += int64(n)
	return n, err
}
