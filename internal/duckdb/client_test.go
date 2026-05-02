package duckdb

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// findCLI returns the duckdb CLI path or skips the test. Integration tests
// in this file are best-effort: they run on dev machines that have duckdb
// installed and silently skip in CI environments that don't.
func findCLI(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("duckdb")
	if err != nil {
		t.Skip("duckdb CLI not on PATH; skipping integration test")
	}
	return path
}

func TestClient_Query_InMemorySelect(t *testing.T) {
	cli := findCLI(t)
	c := NewClient(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := c.Query(ctx, QueryOptions{
		SQL:     "SELECT 1 AS one, 'hi' AS msg",
		MaxRows: 100,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(res.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(res.Columns))
	}
	if res.Columns[0].Name != "one" || res.Columns[1].Name != "msg" {
		t.Errorf("unexpected column names: %+v", res.Columns)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "1" || res.Rows[0][1] != "hi" {
		t.Errorf("unexpected rows: %+v", res.Rows)
	}
}

func TestClient_Query_BigIntPreservesPrecision(t *testing.T) {
	cli := findCLI(t)
	c := NewClient(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Above 2^53. JSON-as-number would silently corrupt this; the DESCRIBE
	// + json.Number path keeps it intact as a string.
	res, err := c.Query(ctx, QueryOptions{
		SQL: "SELECT CAST(9007199254740993 AS BIGINT) AS big",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if got := res.Rows[0][0]; got != "9007199254740993" {
		t.Errorf("BIGINT lost precision: got %q, want 9007199254740993", got)
	}
}

func TestClient_Query_FileBackedDB(t *testing.T) {
	cli := findCLI(t)
	c := NewClient(cli)

	dir := t.TempDir()
	db := filepath.Join(dir, "test.db")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := c.Exec(ctx, ExecOptions{
		Database: db,
		SQL:      "CREATE TABLE t (id INT, name VARCHAR); INSERT INTO t VALUES (1, 'a'), (2, 'b')",
	}); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	res, err := c.Query(ctx, QueryOptions{
		Database: db,
		SQL:      "SELECT * FROM t ORDER BY id",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(res.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d: %+v", len(res.Rows), res.Rows)
	}
}

func TestClient_Query_Truncation(t *testing.T) {
	cli := findCLI(t)
	c := NewClient(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := c.Query(ctx, QueryOptions{
		SQL:     "SELECT i FROM range(0, 100) t(i)",
		MaxRows: 10,
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if !res.Truncated {
		t.Errorf("expected Truncated=true")
	}
	if len(res.Rows) != 10 {
		t.Errorf("expected 10 rows after truncation, got %d", len(res.Rows))
	}
	if res.TotalRows != 100 {
		t.Errorf("expected TotalRows=100, got %d", res.TotalRows)
	}
}

func TestClient_Query_BadSQLReturnsSubprocessError(t *testing.T) {
	cli := findCLI(t)
	c := NewClient(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.Query(ctx, QueryOptions{
		SQL: "SELEKT 1",
	})
	if err == nil {
		t.Fatalf("expected error for bad SQL")
	}
	subErr, ok := err.(*SubprocessError)
	if !ok {
		t.Fatalf("expected *SubprocessError, got %T: %v", err, err)
	}
	if subErr.ExitCode == 0 {
		t.Errorf("expected non-zero exit code, got %d", subErr.ExitCode)
	}
	if subErr.Stderr == "" {
		t.Errorf("expected stderr content")
	}
}

func TestClient_Query_InitFile(t *testing.T) {
	cli := findCLI(t)
	c := NewClient(cli)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := c.Query(ctx, QueryOptions{
		Init: []string{"CREATE TABLE init_t (v INT)", "INSERT INTO init_t VALUES (42)"},
		SQL:  "SELECT v FROM init_t",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "42" {
		t.Errorf("init prelude did not execute: %+v", res.Rows)
	}
}

func TestWriteInitFile_Cleanup(t *testing.T) {
	path, cleanup, err := writeInitFile([]string{"SELECT 1", "SELECT 2;"})
	if err != nil {
		t.Fatalf("writeInitFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("init file not present: %v", err)
	}
	contents, _ := os.ReadFile(path)
	if !strings.Contains(string(contents), "SELECT 1;\nSELECT 2;\n") {
		t.Errorf("unexpected init contents: %q", contents)
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("init file should be removed after cleanup, stat err = %v", err)
	}
}
