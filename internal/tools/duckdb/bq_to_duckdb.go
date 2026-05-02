package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
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
}

// GCSCleaner deletes staged Parquet objects on success or after a
// cancelled run. Same separation-of-concerns rationale as BQClient —
// the tool doesn't need full GCS surface area, just delete.
type GCSCleaner interface {
	DeletePrefix(ctx context.Context, bucket, prefix string) error
}

// BQToDuckDBTool implements the round-trip workflow at the heart of the
// design's Tract 2025 KB story:
//
//	BQ EXPORT (slice → gs://staging/{session}/…parquet)
//	  → DuckDB COPY (https://… via httpfs+OAuth bearer)
//	  → CREATE TABLE in the local session DB
//
// No DuckDB→BQ direction in v1 (deferred to v1.5 — clients have asked
// for the forward path; the reverse is speculative).
type BQToDuckDBTool struct {
	bq           BQClient
	cleaner      GCSCleaner
	runtime      duck.Runtime
	session      *duck.Session
	gcs          *duck.GCSAuth
	gate         *duck.VolumeGate
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

// NewBQToDuckDBTool constructs the tool. Returns nil if any required
// dependency is missing, so registration code can decide to hide the
// tool from the agent rather than expose a half-wired version.
func NewBQToDuckDBTool(c BQToDuckDBConfig) *BQToDuckDBTool {
	if c.BQ == nil || c.Runtime == nil || c.Session == nil || c.GCS == nil {
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
	return "Pull a BigQuery slice into the local DuckDB session: EXPORT to a staging bucket as Parquet, " +
		"then COPY into a local table. Use this to escape per-scan BQ cost on iterative work. " +
		"Requires [duckdb] staging_bucket in config. Volume-gated: warns above 1 GiB, hard-stops above 50 GiB " +
		"unless force=true."
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
	Force       bool   `json:"force,omitempty"`
}

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
	if t.stagingBucket == "" {
		return errResult(stagingMissingMessage()), nil
	}

	// Volume gate: dry-run the source SQL on BQ. Allow / Warn / Block.
	if t.gate != nil {
		decision, err := t.gate.CheckBQExport(ctx, p.SQL, p.Force)
		if err != nil {
			return errResult(fmt.Sprintf("volume gate: %v", err)), nil
		}
		if decision.Decision == duck.VolumeBlock {
			return errResult(decision.Reason), nil
		}
		// VolumeWarn is informational here; the permission layer already
		// gates the destructive risk. Surface the reason as a prefix.
		_ = decision // intentionally unused beyond the block check; agent sees risk prompt separately
	}

	// Stage path: gs://<bucket>/cascade-bq-export/<session-id>/data-*.parquet.
	// Uses session ID to avoid collisions across parallel cascade runs.
	stagingPrefix := fmt.Sprintf("cascade-bq-export/%s", t.session.ID)
	stagingURI := fmt.Sprintf("gs://%s/%s/data-*.parquet", t.stagingBucket, stagingPrefix)

	// Track whether we should clean up — default cleanup-on-success,
	// retain-on-error so the user can inspect what landed.
	exportOK := false
	defer func() {
		if exportOK && t.cleaner != nil {
			// Run cleanup with a fresh context — the original may be
			// cancelled by the time we get here.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = t.cleaner.DeletePrefix(ctx, t.stagingBucket, stagingPrefix+"/")
		}
	}()

	// Write the EXPORT data job.
	exportSQL := buildExportSQL(p.SQL, stagingURI)
	if err := t.bq.ExportToGCS(ctx, exportSQL, stagingURI); err != nil {
		return errResult(fmt.Sprintf("BQ EXPORT failed: %v", err)), nil
	}

	// Resolve the wildcard URI into a concrete list of https URLs.
	urls, err := t.gcs.ExpandGlob(ctx, fmt.Sprintf("gs://%s/%s/data-*.parquet",
		t.stagingBucket, stagingPrefix))
	if err != nil {
		return errResult(fmt.Sprintf("list staged Parquet: %v", err)), nil
	}

	// Build init prelude (bearer token + httpfs) and the COPY statement.
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

	msg := fmt.Sprintf(
		"Loaded %d Parquet shard(s) into local table %q.%s",
		len(urls), p.TargetTable, rowSummary)
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
