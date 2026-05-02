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
// Defaults: warn at 1 GiB, hard-stop at 50 GiB.
type VolumeGate struct {
	WarnBytes     int64
	HardStopBytes int64
	Estimator     VolumeEstimator
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
	case g.HardStopBytes > 0 && bytes > g.HardStopBytes && !force:
		return VolumeResult{
			Decision: VolumeBlock,
			Bytes:    bytes,
			Reason: fmt.Sprintf(
				"BQ export would scan %s, above the %s hard stop. Add a WHERE clause, narrow columns, or pass force=true to override.",
				render.FormatBytes(bytes), render.FormatBytes(g.HardStopBytes),
			),
		}, nil

	case g.WarnBytes > 0 && bytes > g.WarnBytes:
		reason := fmt.Sprintf("BQ export would scan %s (warn threshold %s)",
			render.FormatBytes(bytes), render.FormatBytes(g.WarnBytes))
		if force && g.HardStopBytes > 0 && bytes > g.HardStopBytes {
			reason += " — proceeding under force=true"
		}
		return VolumeResult{Decision: VolumeWarn, Bytes: bytes, Reason: reason}, nil

	default:
		return VolumeResult{Decision: VolumeAllow, Bytes: bytes}, nil
	}
}
