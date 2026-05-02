package duckdb

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	duck "github.com/slokam-ai/cascade/internal/duckdb"
	"github.com/slokam-ai/cascade/internal/permission"
)

func TestQueryTool_LocalSelect(t *testing.T) {
	runtime, sess := newTestRuntime(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := runtime.Exec(ctx, duck.ExecOptions{
		Database: sess.Path,
		SQL:      "CREATE TABLE t (id INT, v VARCHAR); INSERT INTO t VALUES (1, 'a'), (2, 'b'), (3, 'c')",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tool := NewQueryTool(runtime, sess, nil)
	out, err := tool.Execute(ctx, json.RawMessage(`{"sql":"SELECT * FROM t ORDER BY id"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.IsError {
		t.Fatalf("unexpected error: %s", out.Content)
	}
	for _, want := range []string{"id", "v", "1", "2", "3"} {
		if !contains(out.Content, want) {
			t.Errorf("output missing %q: %s", want, out.Content)
		}
	}
}

func TestQueryTool_TruncationMarker(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewQueryTool(runtime, sess, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := tool.Execute(ctx, json.RawMessage(`{"sql":"SELECT i FROM range(0, 50) t(i)","max_rows":10}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.IsError {
		t.Fatalf("error: %s", out.Content)
	}
	if !contains(out.Content, "truncated") {
		t.Errorf("expected truncation marker, got: %s", out.Content)
	}
}

func TestQueryTool_PlanPermission_DowngradesRead(t *testing.T) {
	tool := &QueryTool{}
	plan, err := tool.PlanPermission(t.Context(), json.RawMessage(`{"sql":"SELECT 1"}`), permission.RiskDestructive)
	if err != nil {
		t.Fatalf("PlanPermission: %v", err)
	}
	if plan.RiskOverride == nil || *plan.RiskOverride != permission.RiskReadOnly {
		t.Errorf("expected RiskReadOnly override, got %+v", plan)
	}
}

func TestQueryTool_PlanPermission_EscalatesDDL(t *testing.T) {
	tool := &QueryTool{}
	plan, err := tool.PlanPermission(t.Context(), json.RawMessage(`{"sql":"CREATE TABLE x (i INT)"}`), permission.RiskDestructive)
	if err != nil {
		t.Fatalf("PlanPermission: %v", err)
	}
	if plan.RiskOverride == nil || *plan.RiskOverride != permission.RiskDDL {
		t.Errorf("expected RiskDDL, got %+v", plan)
	}
}

func TestQueryTool_DDL_Executes(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewQueryTool(runtime, sess, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// CREATE then SELECT verifies the Exec branch lands rows that the
	// Query branch can read back.
	out, err := tool.Execute(ctx, json.RawMessage(`{"sql":"CREATE TABLE seed (i INT); INSERT INTO seed VALUES (7)"}`))
	if err != nil {
		t.Fatalf("DDL: %v", err)
	}
	if out.IsError {
		t.Fatalf("DDL returned error: %s", out.Content)
	}

	out, err = tool.Execute(ctx, json.RawMessage(`{"sql":"SELECT i FROM seed"}`))
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if !contains(out.Content, "7") {
		t.Errorf("expected to read back 7, got: %s", out.Content)
	}
}

func TestQueryTool_Sources_RequireGCSAuth(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewQueryTool(runtime, sess, nil) // gcs == nil

	out, err := tool.Execute(t.Context(), json.RawMessage(`{
		"sql":"SELECT * FROM ${hn} LIMIT 1",
		"sources":[{"alias":"hn","gcs_paths":["gs://b/o.parquet"]}]
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Errorf("expected error when sources used without GCS auth")
	}
}

func TestQueryTool_PlaceholderMissing(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := &QueryTool{
		runtime: runtime,
		session: sess,
		// stub gcs that has nil token source still satisfies non-nil check
		gcs: duck.NewGCSAuth(nil, nil),
	}

	out, err := tool.Execute(t.Context(), json.RawMessage(`{
		"sql":"SELECT 1",
		"sources":[{"alias":"hn","gcs_paths":["gs://b/o.parquet"]}]
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Error("expected error when placeholder missing from SQL")
	}
}

func TestValidAlias(t *testing.T) {
	good := []string{"hn", "hn_2025", "Foo", "X1"}
	bad := []string{"", "foo bar", "foo-bar", "foo.bar", "foo$"}
	for _, s := range good {
		if !validAlias(s) {
			t.Errorf("validAlias(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if validAlias(s) {
			t.Errorf("validAlias(%q) = true, want false", s)
		}
	}
}
