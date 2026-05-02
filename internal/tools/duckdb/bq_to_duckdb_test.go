package duckdb

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	duck "github.com/slokam-ai/cascade/internal/duckdb"
)

type fakeBQ struct {
	bytes      int64
	exportErr  error
	exportSQL  string
	exportURI  string
}

func (f *fakeBQ) EstimateBytes(ctx context.Context, sql string) (int64, error) {
	return f.bytes, nil
}
func (f *fakeBQ) ExportToGCS(ctx context.Context, sql, gcsURI string) error {
	f.exportSQL = sql
	f.exportURI = gcsURI
	return f.exportErr
}

type fakeCleaner struct {
	deleted []string
}

func (f *fakeCleaner) DeletePrefix(ctx context.Context, bucket, prefix string) error {
	f.deleted = append(f.deleted, bucket+"/"+prefix)
	return nil
}

func TestBQToDuckDB_RequiresStaging(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:      &fakeBQ{},
		Runtime: runtime,
		Session: sess,
		// non-nil but token-less GCSAuth is fine for this code path —
		// we error out before any token fetch.
		GCS: stubGCSAuth(),
		// no StagingBucket on purpose
	})
	if tool == nil {
		t.Fatal("tool unexpectedly nil")
	}

	out, err := tool.Execute(t.Context(), json.RawMessage(`{"sql":"SELECT 1","target_table":"t"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Errorf("expected error when staging_bucket missing")
	}
	if !strings.Contains(out.Content, "staging_bucket") {
		t.Errorf("error message should reference staging_bucket: %s", out.Content)
	}
}

func TestBQToDuckDB_RejectsBadIdentifier(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:            &fakeBQ{},
		Runtime:       runtime,
		Session:       sess,
		GCS:           stubGCSAuth(),
		StagingBucket: "stage",
	})
	out, err := tool.Execute(t.Context(), json.RawMessage(`{"sql":"SELECT 1","target_table":"t; DROP TABLE x"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Errorf("expected error for malformed identifier")
	}
}

func TestBQToDuckDB_NewReturnsNilOnMissingDeps(t *testing.T) {
	if NewBQToDuckDBTool(BQToDuckDBConfig{}) != nil {
		t.Errorf("expected nil tool when no deps configured")
	}
}

func TestBuildExportSQL(t *testing.T) {
	out := buildExportSQL("SELECT id FROM `p.d.t`;", "gs://b/p/data-*.parquet")
	if !strings.Contains(out, "EXPORT DATA") {
		t.Errorf("missing EXPORT DATA: %s", out)
	}
	if !strings.Contains(out, "gs://b/p/data-*.parquet") {
		t.Errorf("missing URI: %s", out)
	}
	if strings.Contains(out, ";\n") || strings.HasSuffix(out, ";") {
		t.Errorf("trailing semicolon should be stripped: %q", out)
	}
}

func TestStagingMissingMessage(t *testing.T) {
	msg := stagingMissingMessage()
	if !strings.Contains(msg, "[duckdb]") || !strings.Contains(msg, "staging_bucket") {
		t.Errorf("message should reference [duckdb] staging_bucket, got %q", msg)
	}
}

// stubGCSAuth returns a non-nil GCSAuth with zero token source — good
// enough to satisfy NewBQToDuckDBTool's nil-check; tests that exercise
// the validation paths exit before any token fetch.
func stubGCSAuth() *duck.GCSAuth { return duck.NewGCSAuth(nil, nil) }
