package duckdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	duck "github.com/slokam-ai/cascade/internal/duckdb"
	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/internal/tools"
)

// BQClient is the BigQuery surface bq_to_duckdb depends on. Defining
// a small interface here (rather than importing the heavy internal
// BQ client struct) keeps the dependency graph clean and makes the
// tool testable without spinning up real BQ.
type BQClient interface {
	// EstimateBytes runs a dry-run query and returns totalBytesProcessed.
	// Implementations must use JobConfigurationQuery.DryRun=true on the
	// Go BigQuery client — never shell out to the `bq` CLI (a reviewer
	// concern from the design).
	EstimateBytes(ctx context.Context, sql string) (int64, error)

	// ExportToGCS runs an EXPORT DATA OPTIONS(...) AS <sql> job that
	// writes Parquet to the given gs:// URI prefix. Implementations
	// should block until the job is done and return any non-zero error
	// status.
	ExportToGCS(ctx context.Context, sql, gcsURI string) error

	// QueryToCSV runs a SELECT and dumps every row to a freshly-created
	// temp file as CSV. Returns the path; the caller is responsible
	// for deleting it. Used by the local-stream path for small pulls
	// that don't justify a GCS round-trip.
	QueryToCSV(ctx context.Context, sql string) (path string, rows int64, err error)
}

// GCSCleaner deletes staged Parquet objects on success or after a
// cancelled run. Same separation-of-concerns rationale as BQClient —
// the tool doesn't need full GCS surface area, just delete.
type GCSCleaner interface {
	DeletePrefix(ctx context.Context, bucket, prefix string) error
}

// BQToDuckDBTool implements the BQ→local round-trip at the heart of the
// design's Tract 2025 KB story. Two modes:
//
//   - mode="gcs":   BQ EXPORT (slice → gs://staging/{session}/…parquet)
//                   → DuckDB COPY (https://… via httpfs+OAuth bearer)
//                   → CREATE TABLE in the local session DB.
//                   Best for medium-to-large pulls; needs staging_bucket.
//
//   - mode="local": BQ Query API → temp CSV on this machine
//                   → DuckDB CREATE TABLE AS SELECT FROM read_csv_auto.
//                   Zero-config; capped by VolumeGate.LocalHardStopBytes
//                   (default 1 GiB). Right for small slices and for
//                   users who haven't set up GCS staging.
//
//   - mode="auto" (default): pick local when it is allowed and staging
//                   isn't configured; else pick gcs. Push back when
//                   the pick disagrees with the user's data size.
//
// No DuckDB→BQ direction in v1 (deferred to v1.5).
type BQToDuckDBTool struct {
	bq            BQClient
	cleaner       GCSCleaner
	runtime       duck.Runtime
	session       *duck.Session
	gcs           *duck.GCSAuth
	gate          *duck.VolumeGate
	stagingBucket string
}

// BQToDuckDBConfig collects construction args. All fields are required
// except `gate` — when nil, the tool runs without a volume guard
// (matches the unit-test path).
type BQToDuckDBConfig struct {
	BQ            BQClient
	Cleaner       GCSCleaner
	Runtime       duck.Runtime
	Session       *duck.Session
	GCS           *duck.GCSAuth
	Gate          *duck.VolumeGate
	StagingBucket string
}

// NewBQToDuckDBTool constructs the tool. Returns nil if a hard
// dependency is missing — the agent never sees a half-wired version.
//
// GCS auth is now optional: it's only required when mode=gcs. With
// no GCS auth and no staging_bucket, the tool still registers in
// local-only mode (modes default to "local"; gcs/auto-promotion-to-
// gcs paths return a clear directive error).
func NewBQToDuckDBTool(c BQToDuckDBConfig) *BQToDuckDBTool {
	if c.BQ == nil || c.Runtime == nil || c.Session == nil {
		return nil
	}
	return &BQToDuckDBTool{
		bq:            c.BQ,
		cleaner:       c.Cleaner,
		runtime:       c.Runtime,
		session:       c.Session,
		gcs:           c.GCS,
		gate:          c.Gate,
		stagingBucket: c.StagingBucket,
	}
}

func (t *BQToDuckDBTool) Name() string { return "bq_to_duckdb" }

func (t *BQToDuckDBTool) Description() string {
	return "Pull a BigQuery slice into the local DuckDB session. " +
		"mode='local' (default for small slices, no extra config): streams BQ rows to a temp CSV and creates the table; capped at ~1 GiB. " +
		"mode='gcs' (for big slices): EXPORTs to gs://<staging>/cascade-bq-export/{session}/...parquet then COPIES into the table; capped at ~50 GiB; requires [duckdb] staging_bucket. " +
		"mode='auto' (default): picks based on size and config. " +
		"Volume-gated; hard caps refusable with force=true."
}

func (t *BQToDuckDBTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sql": map[string]any{
				"type":        "string",
				"description": "BigQuery SELECT to export. Use SELECT * FROM `project.dataset.table` for a full pull.",
			},
			"target_table": map[string]any{
				"type":        "string",
				"description": "Name of the local DuckDB table to create or replace.",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"auto", "local", "gcs"},
				"description": "Which path to use. 'local' streams via temp CSV (small data, no staging needed). 'gcs' uses BQ EXPORT to a staging bucket (medium-to-large data, needs [duckdb] staging_bucket). 'auto' picks based on size + config (default).",
			},
			"force": map[string]any{
				"type":        "boolean",
				"description": "Bypass the volume-gate hard stop. The user still sees a confirmation prompt.",
			},
		},
		"required": []string{"sql", "target_table"},
	}
}

// RiskLevel is destructive: this tool both spins a BQ EXPORT job
// (irreversible compute spend, GCS object writes) and creates/replaces
// a local table. The agent should always confirm.
func (t *BQToDuckDBTool) RiskLevel() permission.RiskLevel { return permission.RiskDDL }

type bqToDuckInput struct {
	SQL         string `json:"sql"`
	TargetTable string `json:"target_table"`
	// Mode selects the path: "local" streams BQ rows to a temp CSV and
	// COPYs into DuckDB; "gcs" uses the EXPORT-to-Parquet staging path;
	// "auto" (default, or empty) picks based on staging_bucket
	// availability and the pre-flight size estimate.
	Mode  string `json:"mode,omitempty"`
	Force bool   `json:"force,omitempty"`
}

const (
	modeAuto  = "auto"
	modeLocal = "local"
	modeGCS   = "gcs"
)

func (t *BQToDuckDBTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var p bqToDuckInput
	if err := json.Unmarshal(input, &p); err != nil {
		return errResult(fmt.Sprintf("invalid input: %v", err)), nil
	}
	if strings.TrimSpace(p.SQL) == "" {
		return errResult("sql is required"), nil
	}
	if err := validateIdentifier(p.TargetTable); err != nil {
		return errResult(err.Error()), nil
	}

	mode, err := t.resolveMode(ctx, p)
	if err != nil {
		return errResult(err.Error()), nil
	}

	switch mode {
	case modeLocal:
		return t.executeLocal(ctx, p)
	case modeGCS:
		return t.executeGCS(ctx, p)
	default:
		return errResult(fmt.Sprintf("unknown mode %q", mode)), nil
	}
}

// resolveMode normalizes the user's requested mode (or auto) into a
// concrete local|gcs choice and validates the request against
// available config (staging_bucket, GCS auth) and pre-flight size.
//
// "Push back when intent disagrees with reality": local mode + a
// 100 GiB estimate refuses with a clear redirect to mode=gcs;
// gcs mode without staging_bucket refuses with a fix-it message.
func (t *BQToDuckDBTool) resolveMode(ctx context.Context, p bqToDuckInput) (string, error) {
	requested := strings.ToLower(strings.TrimSpace(p.Mode))
	if requested == "" {
		requested = modeAuto
	}

	canGCS := t.stagingBucket != "" && t.gcs != nil

	switch requested {
	case modeLocal:
		// Local is always available (no GCS dep). Volume gate runs
		// inside executeLocal — let that be the place that pushes back
		// on size, with a redirect to mode=gcs when applicable.
		return modeLocal, nil

	case modeGCS:
		if !canGCS {
			return "", errors.New(stagingMissingMessage())
		}
		return modeGCS, nil

	case modeAuto:
		// auto picks the path that won't refuse on size, given the
		// user's environment.
		bytes, _ := t.estimateBytes(ctx, p.SQL)
		localOK := t.gate == nil || t.gate.LocalHardStopBytes <= 0 || bytes <= t.gate.LocalHardStopBytes || p.Force
		switch {
		case localOK && !canGCS:
			return modeLocal, nil
		case localOK && canGCS:
			// Both work — prefer local because it's faster for small
			// data and leaves no GCS artifacts behind.
			return modeLocal, nil
		case canGCS:
			return modeGCS, nil
		default:
			return "", fmt.Errorf(
				"the slice is too big for the local stream path (%d bytes) and no [duckdb] staging_bucket is configured for GCS. "+
					"Either configure staging_bucket and use mode='gcs', or pass force=true to push the local cap.",
				bytes)
		}

	default:
		return "", fmt.Errorf("invalid mode %q (want auto, local, or gcs)", requested)
	}
}

// estimateBytes is a best-effort dry-run for mode resolution. We
// silently swallow estimator errors here — the gate methods will
// re-run the dry-run and produce the right user-visible message.
func (t *BQToDuckDBTool) estimateBytes(ctx context.Context, sql string) (int64, error) {
	if t.bq == nil {
		return 0, fmt.Errorf("no BQ client")
	}
	return t.bq.EstimateBytes(ctx, sql)
}

// executeGCS runs the BQ-EXPORT-to-GCS-Parquet → DuckDB-COPY ladder.
// This is the original (and primary) path; it scales to ~50 GiB.
func (t *BQToDuckDBTool) executeGCS(ctx context.Context, p bqToDuckInput) (*tools.Result, error) {
	if t.gate != nil {
		decision, err := t.gate.CheckBQExport(ctx, p.SQL, p.Force)
		if err != nil {
			return errResult(fmt.Sprintf("volume gate: %v", err)), nil
		}
		if decision.Decision == duck.VolumeBlock {
			return errResult(decision.Reason), nil
		}
	}

	stagingPrefix := fmt.Sprintf("cascade-bq-export/%s", t.session.ID)
	stagingURI := fmt.Sprintf("gs://%s/%s/data-*.parquet", t.stagingBucket, stagingPrefix)

	exportOK := false
	defer func() {
		if exportOK && t.cleaner != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = t.cleaner.DeletePrefix(ctx, t.stagingBucket, stagingPrefix+"/")
		}
	}()

	exportSQL := buildExportSQL(p.SQL, stagingURI)
	if err := t.bq.ExportToGCS(ctx, exportSQL, stagingURI); err != nil {
		return errResult(fmt.Sprintf("BQ EXPORT failed: %v", err)), nil
	}

	urls, err := t.gcs.ExpandGlob(ctx, fmt.Sprintf("gs://%s/%s/data-*.parquet",
		t.stagingBucket, stagingPrefix))
	if err != nil {
		return errResult(fmt.Sprintf("list staged Parquet: %v", err)), nil
	}

	init, err := t.gcs.BuildInitPrelude(ctx)
	if err != nil {
		return errResult(fmt.Sprintf("build GCS init: %v", err)), nil
	}
	copySQL := fmt.Sprintf(
		"CREATE OR REPLACE TABLE %s AS SELECT * FROM %s",
		quoteIdent(p.TargetTable), duck.FormatReadParquetCall(urls))

	t.session.Lock()
	defer t.session.Unlock()

	if _, err := t.runtime.Exec(ctx, duck.ExecOptions{
		Database: t.session.Path,
		Init:     init,
		SQL:      copySQL,
	}); err != nil {
		return errResult(fmt.Sprintf("DuckDB COPY failed: %v", err)), nil
	}

	exportOK = true
	rowSummary, _ := t.countRows(ctx, p.TargetTable)
	msg := fmt.Sprintf("Loaded %d Parquet shard(s) into local table %q via GCS.%s",
		len(urls), p.TargetTable, rowSummary)
	return &tools.Result{Content: msg, Display: msg}, nil
}

// executeLocal runs the small-data path: stream BQ rows through the
// Query API into a temp CSV, then have DuckDB ingest it via
// read_csv_auto. No GCS round-trip, no staging bucket required.
//
// Volume gate uses LocalHardStopBytes (~1 GiB by default) — well below
// the GCS path because CSV is 5-10x larger than Parquet on disk and
// the iteration is single-stream.
func (t *BQToDuckDBTool) executeLocal(ctx context.Context, p bqToDuckInput) (*tools.Result, error) {
	if t.gate != nil {
		decision, err := t.gate.CheckBQLocal(ctx, p.SQL, p.Force)
		if err != nil {
			return errResult(fmt.Sprintf("volume gate: %v", err)), nil
		}
		if decision.Decision == duck.VolumeBlock {
			// Add a redirect to mode=gcs when the env supports it —
			// the user shouldn't have to read the docs to find the
			// next step.
			reason := decision.Reason
			if t.stagingBucket != "" && t.gcs != nil {
				reason += " Try mode='gcs' instead — your staging bucket is already configured."
			}
			return errResult(reason), nil
		}
	}

	csvPath, rowCount, err := t.bq.QueryToCSV(ctx, p.SQL)
	if err != nil {
		return errResult(fmt.Sprintf("BQ stream-to-CSV failed: %v", err)), nil
	}
	defer func() { _ = os.Remove(csvPath) }()

	// DuckDB needs the path quoted. CSV file paths from os.CreateTemp
	// shouldn't contain single quotes on any supported platform, but
	// double-up just in case.
	quotedPath := strings.ReplaceAll(csvPath, "'", "''")
	copySQL := fmt.Sprintf(
		"CREATE OR REPLACE TABLE %s AS SELECT * FROM read_csv_auto('%s')",
		quoteIdent(p.TargetTable), quotedPath)

	t.session.Lock()
	defer t.session.Unlock()

	if _, err := t.runtime.Exec(ctx, duck.ExecOptions{
		Database: t.session.Path,
		SQL:      copySQL,
	}); err != nil {
		return errResult(fmt.Sprintf("DuckDB ingest failed: %v", err)), nil
	}

	msg := fmt.Sprintf("Loaded %d row(s) into local table %q via local stream.",
		rowCount, p.TargetTable)
	return &tools.Result{Content: msg, Display: msg}, nil
}

// countRows tries to report how many rows landed; failure is silent
// because we don't want a count-rows hiccup to mask a successful copy.
func (t *BQToDuckDBTool) countRows(ctx context.Context, table string) (string, error) {
	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database: t.session.Path,
		SQL:      fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdent(table)),
	})
	if err != nil || len(res.Rows) == 0 {
		return "", err
	}
	return fmt.Sprintf(" %s rows.", res.Rows[0][0]), nil
}

// buildExportSQL wraps a user SELECT in BigQuery's EXPORT DATA OPTIONS
// statement with a wildcard GCS URI that BQ shards across files.
//
// Defensive trim: the user SQL may end with a `;` we'd otherwise
// embed, which BQ rejects in the AS clause.
func buildExportSQL(userSQL, gcsURI string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(userSQL), ";")
	return fmt.Sprintf(
		"EXPORT DATA OPTIONS(uri='%s', format='PARQUET', overwrite=true) AS\n%s",
		strings.ReplaceAll(gcsURI, "'", "''"), trimmed,
	)
}

// stagingMissingMessage tells the user how to fix a missing
// staging_bucket. Mentions the gcs tool because that's how a Cascade
// session creates a new bucket without leaving the agent.
func stagingMissingMessage() string {
	return "bq_to_duckdb requires a staging bucket. Add\n" +
		"  [duckdb]\n  staging_bucket = \"<bucket-name>\"\n" +
		"to your cascade config (or ~/.cascade/config.toml). " +
		"To create a new bucket, ask Cascade to create one with the gcs tool."
}
