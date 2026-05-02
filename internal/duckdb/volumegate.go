package duckdb

import (
	"context"
	"fmt"

	"github.com/slokam-ai/cascade/internal/render"
)

// VolumeEstimator measures how much data a candidate BigQuery export
// would move. Single-method interface so tests can mock it without
// pulling in the BQ client.
//
// The implementation in internal/bigquery wraps a Go-side
// JobConfigurationQuery.DryRun=true call (consistent with what
// internal/bigquery/client.go.EstimateCost does today). The reviewer
// concern that called out shelling out to the `bq` CLI is addressed by
// taking this interface here and binding it to the Go BQ client at
// app-wiring time.
type VolumeEstimator interface {
	EstimateBytes(ctx context.Context, sql string) (int64, error)
}

// VolumeDecision describes what the gate wants the caller to do.
type VolumeDecision int

const (
	// VolumeAllow means the export is comfortably under the warn line —
	// proceed without prompting.
	VolumeAllow VolumeDecision = iota

	// VolumeWarn means the export is between WarnBytes and HardStopBytes.
	// The agent should escalate the permission risk so the user sees a
	// confirmation prompt with the size in it.
	VolumeWarn

	// VolumeBlock means the export is over HardStopBytes and was not
	// forced. The caller must refuse with the included Reason.
	VolumeBlock
)

// VolumeResult is the structured outcome of a CheckBQExport call.
type VolumeResult struct {
	Decision VolumeDecision
	Bytes    int64
	// Reason is a human-readable string suitable for surfacing to the
	// agent or user. Always populated for Warn/Block; empty for Allow.
	Reason string
}

// VolumeGate guards bq_to_duckdb against accidentally pulling terabytes
// onto a laptop. Asymmetric concern by design: pulling too much remote
// data is real damage, while reading more than expected from local
// DuckDB self-corrects (the user notices and adjusts).
//
// Two hard stops because the two BQ→DuckDB paths have very different
// cost curves:
//   - GCS staging path: BQ shards EXPORT in parallel, DuckDB httpfs
//     reads efficiently. 50 GiB is uncomfortable but achievable.
//   - Local stream path: row-by-row through encoding/csv, single
//     stream. Above ~1 GiB the round-trip stings (CSV is 5-10x
//     larger than Parquet, and we hold the temp file on disk).
//
// Defaults: warn at 1 GiB, GCS hard-stop at 50 GiB, local-stream
// hard-stop at 1 GiB.
type VolumeGate struct {
	WarnBytes          int64
	HardStopBytes      int64 // GCS staging hard-stop
	LocalHardStopBytes int64 // Local stream hard-stop (much smaller)
	Estimator          VolumeEstimator
}

// CheckBQExport runs a dry-run against the source SQL and reports a
// decision based on configured thresholds. Force=true bypasses the
// hard-stop but still surfaces a Warn so the agent can include size
// in the confirmation prompt.
//
// On dry-run failure, the gate returns Allow with Reason set — the
// surrounding tool already needs to handle the same SQL failing for
// real, so blocking-on-estimation-failure would just chain false
// negatives.
func (g *VolumeGate) CheckBQExport(ctx context.Context, sql string, force bool) (VolumeResult, error) {
	if g == nil {
		return VolumeResult{Decision: VolumeAllow, Reason: "volume gate not configured"}, nil
	}
	return g.check(ctx, sql, force, g.HardStopBytes, "GCS export")
}

// CheckBQLocal applies the local-stream thresholds. Same Allow/Warn/
// Block contract as CheckBQExport, just a much smaller hard-stop
// because we're streaming row-by-row through CSV rather than letting
// BQ shard a parallel EXPORT.
func (g *VolumeGate) CheckBQLocal(ctx context.Context, sql string, force bool) (VolumeResult, error) {
	if g == nil {
		return VolumeResult{Decision: VolumeAllow, Reason: "volume gate not configured"}, nil
	}
	return g.check(ctx, sql, force, g.LocalHardStopBytes, "local stream")
}

func (g *VolumeGate) check(ctx context.Context, sql string, force bool, hardStop int64, label string) (VolumeResult, error) {
	if g == nil || g.Estimator == nil {
		return VolumeResult{Decision: VolumeAllow, Reason: "volume gate not configured"}, nil
	}
	bytes, err := g.Estimator.EstimateBytes(ctx, sql)
	if err != nil {
		return VolumeResult{
			Decision: VolumeAllow,
			Reason:   fmt.Sprintf("dry-run failed: %v (proceeding without size guard)", err),
		}, nil
	}

	switch {
	case hardStop > 0 && bytes > hardStop && !force:
		return VolumeResult{
			Decision: VolumeBlock,
			Bytes:    bytes,
			Reason: fmt.Sprintf(
				"BQ %s would move %s, above the %s hard stop. Add a WHERE clause, narrow columns, switch to a path with a higher cap, or pass force=true to override.",
				label, render.FormatBytes(bytes), render.FormatBytes(hardStop),
			),
		}, nil

	case g.WarnBytes > 0 && bytes > g.WarnBytes:
		reason := fmt.Sprintf("BQ %s would move %s (warn threshold %s)",
			label, render.FormatBytes(bytes), render.FormatBytes(g.WarnBytes))
		if force && hardStop > 0 && bytes > hardStop {
			reason += " — proceeding under force=true"
		}
		return VolumeResult{Decision: VolumeWarn, Bytes: bytes, Reason: reason}, nil

	default:
		return VolumeResult{Decision: VolumeAllow, Bytes: bytes}, nil
	}
}
