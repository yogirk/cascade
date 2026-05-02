package duckdb

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	duck "github.com/slokam-ai/cascade/internal/duckdb"
)

func newTestRuntime(t *testing.T) (duck.Runtime, *duck.Session) {
	t.Helper()
	cli, err := exec.LookPath("duckdb")
	if err != nil {
		t.Skip("duckdb CLI not on PATH; skipping integration test")
	}
	sess, err := duck.NewSession(false)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return duck.NewClient(cli), sess
}

func TestSchemaTool_ListAndDescribe(t *testing.T) {
	runtime, sess := newTestRuntime(t)

	// Seed the session DB with one table.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := runtime.Exec(ctx, duck.ExecOptions{
		Database: sess.Path,
		SQL:      "CREATE TABLE hn_2025 (id BIGINT, title VARCHAR); INSERT INTO hn_2025 VALUES (1, 'a'), (2, 'b')",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tool := NewSchemaTool(runtime, sess)

	// list_tables
	out, err := tool.Execute(ctx, json.RawMessage(`{"action":"list_tables"}`))
	if err != nil {
		t.Fatalf("list_tables: %v", err)
	}
	if out.IsError {
		t.Fatalf("list_tables returned error: %s", out.Content)
	}
	if !contains(out.Content, "hn_2025") {
		t.Errorf("list_tables missing hn_2025: %s", out.Content)
	}

	// describe
	out, err = tool.Execute(ctx, json.RawMessage(`{"action":"describe","table":"hn_2025"}`))
	if err != nil {
		t.Fatalf("describe: %v", err)
	}
	if out.IsError {
		t.Fatalf("describe error: %s", out.Content)
	}
	if !contains(out.Content, "id") || !contains(out.Content, "title") {
		t.Errorf("describe missing column names: %s", out.Content)
	}

	// sample
	out, err = tool.Execute(ctx, json.RawMessage(`{"action":"sample","table":"hn_2025","limit":1}`))
	if err != nil {
		t.Fatalf("sample: %v", err)
	}
	if out.IsError {
		t.Fatalf("sample error: %s", out.Content)
	}
	if !contains(out.Content, "id") {
		t.Errorf("sample header missing: %s", out.Content)
	}
}

// TestSchemaTool_ExternalDatabase covers the "user has an existing
// .duckdb file, point Cascade at it" workflow. Cascade does not own
// the external file's lifecycle and does not hold the session
// RWMutex while operating on it.
func TestSchemaTool_ExternalDatabase(t *testing.T) {
	runtime, sess := newTestRuntime(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Pretend the user already has a .duckdb file at this path.
	other, err := duck.NewSession(false)
	if err != nil {
		t.Fatalf("second session: %v", err)
	}
	defer other.Close()

	if _, err := runtime.Exec(ctx, duck.ExecOptions{
		Database: other.Path,
		SQL:      "CREATE TABLE my_data (id INT, name VARCHAR); INSERT INTO my_data VALUES (1, 'a'), (2, 'b')",
	}); err != nil {
		t.Fatalf("seed external: %v", err)
	}

	tool := NewSchemaTool(runtime, sess)

	// list_tables against the external file
	out, err := tool.Execute(ctx, json.RawMessage(`{"action":"list_tables","database":"`+other.Path+`"}`))
	if err != nil {
		t.Fatalf("list_tables: %v", err)
	}
	if out.IsError {
		t.Fatalf("list_tables on external db returned error: %s", out.Content)
	}
	if !contains(out.Content, "my_data") {
		t.Errorf("list_tables should include 'my_data': %s", out.Content)
	}

	// describe + sample on the external file
	out, _ = tool.Execute(ctx, json.RawMessage(`{"action":"sample","table":"my_data","limit":5,"database":"`+other.Path+`"}`))
	if out.IsError {
		t.Fatalf("sample on external db: %s", out.Content)
	}
	if !contains(out.Content, "id") {
		t.Errorf("sample missing 'id' column: %s", out.Content)
	}
}

func TestSchemaTool_ExternalDatabase_NotFound(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewSchemaTool(runtime, sess)

	out, _ := tool.Execute(t.Context(), json.RawMessage(`{"action":"list_tables","database":"/nonexistent/path/data.duckdb"}`))
	if !out.IsError {
		t.Errorf("expected error for nonexistent database file")
	}
	if !contains(out.Content, "does not exist") {
		t.Errorf("error should be clear about missing file: %s", out.Content)
	}
}

func TestSchemaTool_RejectsBadIdentifier(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewSchemaTool(runtime, sess)

	out, err := tool.Execute(t.Context(), json.RawMessage(`{"action":"describe","table":"foo; DROP TABLE x"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Errorf("expected IsError for SQL-injection-ish identifier")
	}
}

func TestValidateIdentifier(t *testing.T) {
	good := []string{"foo", "foo_bar", "main.foo", "T1", "_x"}
	bad := []string{"", "foo bar", `"foo"`, "foo;DROP", "foo/bar", "foo'x"}
	for _, s := range good {
		if err := validateIdentifier(s); err != nil {
			t.Errorf("validateIdentifier(%q) returned %v, want nil", s, err)
		}
	}
	for _, s := range bad {
		if err := validateIdentifier(s); err == nil {
			t.Errorf("validateIdentifier(%q) returned nil, want error", s)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
