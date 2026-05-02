package app

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/storage"
	"github.com/slokam-ai/cascade/internal/auth"
	bq "github.com/slokam-ai/cascade/internal/bigquery"
	"github.com/slokam-ai/cascade/internal/config"
	duck "github.com/slokam-ai/cascade/internal/duckdb"
	"github.com/slokam-ai/cascade/internal/tools"
	dducktools "github.com/slokam-ai/cascade/internal/tools/duckdb"
	"google.golang.org/api/iterator"
)

// DuckDBComponents holds the runtime objects for the DuckDB feature.
// All fields are nil-safe at the call site — `nil` means the duckdb
// CLI was missing, GCP auth was unavailable, or staging_bucket was
// unset.
type DuckDBComponents struct {
	Detection duck.Detection
	Runtime   duck.Runtime
	Session   *duck.Session
	GCSAuth   *duck.GCSAuth

	QueryTool      *dducktools.QueryTool
	SchemaTool     *dducktools.SchemaTool
	BQToDuckDBTool *dducktools.BQToDuckDBTool
}

// initDuckDB resolves the duckdb CLI, builds the per-invocation session
// DB, and constructs DuckDB-aware tools. Returns nil + a printed warning
// when the CLI is missing or below the pinned floor — the agent never
// sees a half-wired duckdb_query in that state.
//
// Wiring order matters:
//  1. Detect the CLI (no point doing anything else if it's not there).
//  2. Build the Client and Session (works without GCP auth).
//  3. Build GCSAuth from ResourceAuth + Platform's storage client (only
//     if GCP auth is available — without it, gs:// sources error and
//     bq_to_duckdb is hidden).
//  4. Build the BQ adapter (volume gate + EXPORT) only if the BQ
//     component is up.
//  5. Build bq_to_duckdb only if staging_bucket is set, BQ is up, and
//     GCS auth is present.
func initDuckDB(ctx context.Context, cfg *config.Config, resource *auth.ResourceAuth, bqComp *BigQueryComponents, platform *PlatformComponents) *DuckDBComponents {
	det := duck.Detect(ctx, cfg.DuckDB.CLIPath)
	if !det.Available {
		// Print a single line so the user knows DuckDB tools are off and
		// can fix it. Detection.Reason already carries the install hint.
		if det.Reason != "" {
			fmt.Fprintf(os.Stderr, "  ✗ DuckDB tools disabled: %s\n", firstLine(det.Reason))
		}
		return nil
	}

	session, err := duck.NewSession(cfg.DuckDB.KeepSessionDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ DuckDB session init failed: %v\n", err)
		return nil
	}

	runtime := duck.NewClient(det.Path)

	comp := &DuckDBComponents{
		Detection: det,
		Runtime:   runtime,
		Session:   session,
	}

	// GCS auth: only if Resource Plane credentials are available.
	// Without it, query_tool can still run against the local DB and
	// schema_tool works fully — but gs:// sources and bq_to_duckdb
	// are hidden.
	if resource != nil && resource.Available && platform != nil {
		comp.GCSAuth = duck.NewGCSAuth(
			resource.TokenSource,
			platform.GetStorageClient,
		)
	}

	// Tools that always work when we got this far:
	comp.QueryTool = dducktools.NewQueryTool(runtime, session, comp.GCSAuth)
	comp.SchemaTool = dducktools.NewSchemaTool(runtime, session)

	// bq_to_duckdb requires BQ + GCS + staging_bucket.
	if bqComp != nil && bqComp.Client != nil && comp.GCSAuth != nil && cfg.DuckDB.StagingBucket != "" {
		gate := &duck.VolumeGate{
			WarnBytes:     cfg.DuckDB.VolumeGate.WarnBytes,
			HardStopBytes: cfg.DuckDB.VolumeGate.HardStopBytes,
			Estimator:     &bqEstimatorAdapter{client: bqComp.Client},
		}
		comp.BQToDuckDBTool = dducktools.NewBQToDuckDBTool(dducktools.BQToDuckDBConfig{
			BQ:            &bqAdapter{client: bqComp.Client},
			Cleaner:       &gcsCleanerAdapter{getClient: platform.GetStorageClient},
			Runtime:       runtime,
			Session:       session,
			GCS:           comp.GCSAuth,
			Gate:          gate,
			StagingBucket: cfg.DuckDB.StagingBucket,
		})
	}

	fmt.Fprintf(os.Stderr, "  ✓ DuckDB: %s (%s)\n", det.Version, det.Path)
	return comp
}

// registerDuckDBTools registers each tool that got built. Lazy
// registration: if a tool is nil because its deps were missing, it is
// silently skipped — the agent never sees a tool it can't actually use.
func registerDuckDBTools(reg *tools.Registry, comp *DuckDBComponents) {
	if comp == nil {
		return
	}
	dducktools.RegisterAll(reg, comp.QueryTool, comp.SchemaTool, comp.BQToDuckDBTool)
}

// Close removes the session DB unless KeepSessionDB was set, and
// reports any cleanup error. Best-effort.
func (c *DuckDBComponents) Close() {
	if c == nil {
		return
	}
	if c.Session != nil {
		if err := c.Session.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ DuckDB session cleanup: %v\n", err)
		}
	}
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}

// ──────────────────────────── adapters ────────────────────────────

// bqEstimatorAdapter wraps the existing BQ client's EstimateCost as a
// VolumeEstimator (bytes-only). Stays in app/ to avoid leaking
// internal/bigquery internals into the duckdb package.
type bqEstimatorAdapter struct{ client *bq.Client }

func (a *bqEstimatorAdapter) EstimateBytes(ctx context.Context, sql string) (int64, error) {
	bytes, _, err := a.client.EstimateCost(ctx, sql)
	return bytes, err
}

// bqAdapter implements the BQ surface bq_to_duckdb needs (estimate +
// EXPORT). Both methods are thin shims over the existing BQ client
// methods — the heavy lifting (RunStatement waits for the EXPORT job
// to terminate) lives in internal/bigquery/client.go.
type bqAdapter struct{ client *bq.Client }

func (a *bqAdapter) EstimateBytes(ctx context.Context, sql string) (int64, error) {
	bytes, _, err := a.client.EstimateCost(ctx, sql)
	return bytes, err
}
func (a *bqAdapter) ExportToGCS(ctx context.Context, sql, _ string) error {
	// gcsURI is already embedded in the EXPORT DATA OPTIONS(uri=...) AS
	// statement that bq_to_duckdb built; we just submit the job and wait.
	return a.client.RunStatement(ctx, sql)
}

// gcsCleanerAdapter implements the GCSCleaner interface bq_to_duckdb
// uses for cleanup-on-success. Iterates the prefix and deletes each
// object — no batch endpoint in cloud.google.com/go/storage today.
type gcsCleanerAdapter struct {
	getClient func() *storage.Client
}

func (a *gcsCleanerAdapter) DeletePrefix(ctx context.Context, bucket, prefix string) error {
	client := a.getClient()
	if client == nil {
		return fmt.Errorf("storage client not initialized")
	}
	it := client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return err
		}
		// Best-effort delete — first error wins, but we keep going so
		// most of the prefix is cleaned up before we return.
		if delErr := client.Bucket(bucket).Object(attrs.Name).Delete(ctx); delErr != nil {
			return delErr
		}
	}
}
