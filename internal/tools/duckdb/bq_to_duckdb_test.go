package duckdb

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"

	duck "github.com/slokam-ai/cascade/internal/duckdb"
)

type fakeBQ struct {
	bytes     int64
	exportErr error
	exportSQL string
	exportURI string

	// Local-stream inputs.
	csvHeader [][]string // [["id","v"], ["1","a"], ...]
	csvErr    error
	csvCalls  int
}

func (f *fakeBQ) EstimateBytes(ctx context.Context, sql string) (int64, error) {
	return f.bytes, nil
}
func (f *fakeBQ) ExportToGCS(ctx context.Context, sql, gcsURI string) error {
	f.exportSQL = sql
	f.exportURI = gcsURI
	return f.exportErr
}
func (f *fakeBQ) QueryToCSV(ctx context.Context, sql string) (string, int64, error) {
	f.csvCalls++
	if f.csvErr != nil {
		return "", 0, f.csvErr
	}
	tmp, err := os.CreateTemp("", "fakebq-*.csv")
	if err != nil {
		return "", 0, err
	}
	w := csv.NewWriter(tmp)
	for _, row := range f.csvHeader {
		_ = w.Write(row)
	}
	w.Flush()
	_ = tmp.Close()
	return tmp.Name(), int64(len(f.csvHeader) - 1), nil // -1 for header
}

type fakeCleaner struct {
	deleted []string
}

func (f *fakeCleaner) DeletePrefix(ctx context.Context, bucket, prefix string) error {
	f.deleted = append(f.deleted, bucket+"/"+prefix)
	return nil
}

func TestBQToDuckDB_NewReturnsNilOnMissingDeps(t *testing.T) {
	if NewBQToDuckDBTool(BQToDuckDBConfig{}) != nil {
		t.Errorf("expected nil tool when no deps configured")
	}
}

func TestBQToDuckDB_RejectsBadIdentifier(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:      &fakeBQ{},
		Runtime: runtime,
		Session: sess,
	})
	out, err := tool.Execute(t.Context(), json.RawMessage(
		`{"sql":"SELECT 1","target_table":"t; DROP TABLE x","mode":"local"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Errorf("expected error for malformed identifier")
	}
}

func TestBQToDuckDB_GCSMode_RequiresStaging(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:      &fakeBQ{},
		Runtime: runtime,
		Session: sess,
		GCS:     stubGCSAuth(),
		// no StagingBucket on purpose
	})

	out, err := tool.Execute(t.Context(), json.RawMessage(
		`{"sql":"SELECT 1","target_table":"t","mode":"gcs"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Errorf("expected error when mode=gcs but staging_bucket missing")
	}
	if !strings.Contains(out.Content, "staging_bucket") {
		t.Errorf("error message should reference staging_bucket: %s", out.Content)
	}
}

func TestBQToDuckDB_LocalMode_NoStagingNeeded(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ: &fakeBQ{
			csvHeader: [][]string{
				{"id", "v"},
				{"1", "a"},
				{"2", "b"},
			},
		},
		Runtime: runtime,
		Session: sess,
		// No GCS, no staging — local mode shouldn't care.
	})
	out, err := tool.Execute(t.Context(), json.RawMessage(
		`{"sql":"SELECT * FROM t","target_table":"hn_2025","mode":"local"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.IsError {
		t.Fatalf("local mode should succeed without GCS, got: %s", out.Content)
	}
	if !strings.Contains(out.Content, "Loaded 2 row(s)") {
		t.Errorf("expected 'Loaded 2 row(s)' message, got: %s", out.Content)
	}
	// Verify the table actually landed.
	verify, _ := runtime.Query(t.Context(), duck.QueryOptions{
		Database: sess.Path,
		SQL:      "SELECT COUNT(*) FROM hn_2025",
	})
	if verify == nil || len(verify.Rows) == 0 || verify.Rows[0][0] != "2" {
		t.Errorf("expected 2 rows in hn_2025; got %+v", verify)
	}
}

func TestBQToDuckDB_LocalMode_BlocksOnSize(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:      &fakeBQ{bytes: 5 * (1 << 30)}, // 5 GiB > local cap
		Runtime: runtime,
		Session: sess,
		Gate: &duck.VolumeGate{
			WarnBytes:          1 << 30,
			HardStopBytes:      50 * (1 << 30),
			LocalHardStopBytes: 1 << 30,
			Estimator:          &fakeEstimator{bytes: 5 * (1 << 30)},
		},
	})
	out, err := tool.Execute(t.Context(), json.RawMessage(
		`{"sql":"SELECT *","target_table":"big","mode":"local"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !out.IsError {
		t.Errorf("expected block at 5 GiB on local mode")
	}
	if !strings.Contains(out.Content, "local stream") {
		t.Errorf("error should explain local-stream limit, got: %s", out.Content)
	}
}

func TestBQToDuckDB_LocalBlock_RedirectsToGCSWhenAvailable(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:      &fakeBQ{bytes: 5 * (1 << 30)},
		Runtime: runtime,
		Session: sess,
		GCS:     stubGCSAuth(),
		Gate: &duck.VolumeGate{
			WarnBytes:          1 << 30,
			HardStopBytes:      50 * (1 << 30),
			LocalHardStopBytes: 1 << 30,
			Estimator:          &fakeEstimator{bytes: 5 * (1 << 30)},
		},
		StagingBucket: "stage",
	})
	out, _ := tool.Execute(t.Context(), json.RawMessage(
		`{"sql":"SELECT *","target_table":"big","mode":"local"}`))
	if !out.IsError {
		t.Fatalf("expected block")
	}
	if !strings.Contains(out.Content, "mode='gcs'") {
		t.Errorf("expected redirect to mode='gcs' in error: %s", out.Content)
	}
}

func TestBQToDuckDB_AutoMode_PicksLocalForSmall(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	bq := &fakeBQ{
		bytes: 100 * (1 << 20), // 100 MiB
		csvHeader: [][]string{
			{"id"},
			{"1"},
		},
	}
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:      bq,
		Runtime: runtime,
		Session: sess,
		Gate: &duck.VolumeGate{
			WarnBytes:          1 << 30,
			LocalHardStopBytes: 1 << 30,
			HardStopBytes:      50 * (1 << 30),
			Estimator:          &fakeEstimator{bytes: 100 * (1 << 20)},
		},
	})
	out, err := tool.Execute(t.Context(), json.RawMessage(
		`{"sql":"SELECT 1","target_table":"small"}`)) // no mode = auto
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.IsError {
		t.Fatalf("auto-mode should succeed for 100 MiB, got: %s", out.Content)
	}
	if !strings.Contains(out.Content, "local stream") {
		t.Errorf("expected local-stream pick; got: %s", out.Content)
	}
	if bq.csvCalls != 1 {
		t.Errorf("expected QueryToCSV call; got %d calls", bq.csvCalls)
	}
}

func TestBQToDuckDB_AutoMode_NoGCSAndTooBig_Refuses(t *testing.T) {
	runtime, sess := newTestRuntime(t)
	tool := NewBQToDuckDBTool(BQToDuckDBConfig{
		BQ:      &fakeBQ{bytes: 5 * (1 << 30)}, // 5 GiB
		Runtime: runtime,
		Session: sess,
		Gate: &duck.VolumeGate{
			WarnBytes:          1 << 30,
			LocalHardStopBytes: 1 << 30,
			HardStopBytes:      50 * (1 << 30),
			Estimator:          &fakeEstimator{bytes: 5 * (1 << 30)},
		},
		// No GCS, no staging — auto can't pick gcs and shouldn't pick local.
	})
	out, _ := tool.Execute(t.Context(), json.RawMessage(
		`{"sql":"SELECT *","target_table":"big"}`))
	if !out.IsError {
		t.Fatalf("expected refusal when 5 GiB and no GCS available")
	}
	if !strings.Contains(out.Content, "staging_bucket") {
		t.Errorf("error should suggest staging_bucket: %s", out.Content)
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

// stubGCSAuth returns a non-nil GCSAuth with zero token source. Good
// enough to satisfy non-nil checks; tests that hit the validation
// path exit before any token fetch.
func stubGCSAuth() *duck.GCSAuth { return duck.NewGCSAuth(nil, nil) }

// fakeEstimator is shared with volumegate_test via the package; we
// redefine here to keep test-only types co-located.
type fakeEstimator struct {
	bytes int64
}

func (f *fakeEstimator) EstimateBytes(ctx context.Context, sql string) (int64, error) {
	return f.bytes, nil
}
