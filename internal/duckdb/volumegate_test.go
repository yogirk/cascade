package duckdb

import (
	"context"
	"errors"
	"testing"
)

type fakeEstimator struct {
	bytes int64
	err   error
}

func (f *fakeEstimator) EstimateBytes(ctx context.Context, sql string) (int64, error) {
	return f.bytes, f.err
}

func TestVolumeGate_BelowWarn_Allows(t *testing.T) {
	g := &VolumeGate{
		WarnBytes:     1 << 30,
		HardStopBytes: 50 * (1 << 30),
		Estimator:     &fakeEstimator{bytes: 100 * (1 << 20)}, // 100 MiB
	}
	res, err := g.CheckBQExport(t.Context(), "SELECT 1", false)
	if err != nil {
		t.Fatalf("CheckBQExport: %v", err)
	}
	if res.Decision != VolumeAllow {
		t.Errorf("decision = %v, want VolumeAllow", res.Decision)
	}
}

func TestVolumeGate_BetweenThresholds_Warns(t *testing.T) {
	g := &VolumeGate{
		WarnBytes:     1 << 30,
		HardStopBytes: 50 * (1 << 30),
		Estimator:     &fakeEstimator{bytes: 5 * (1 << 30)}, // 5 GiB
	}
	res, err := g.CheckBQExport(t.Context(), "SELECT 1", false)
	if err != nil {
		t.Fatalf("CheckBQExport: %v", err)
	}
	if res.Decision != VolumeWarn {
		t.Errorf("decision = %v, want VolumeWarn", res.Decision)
	}
	if res.Reason == "" {
		t.Error("expected Reason to be populated for warn")
	}
}

func TestVolumeGate_AboveHardStop_Blocks(t *testing.T) {
	g := &VolumeGate{
		WarnBytes:     1 << 30,
		HardStopBytes: 50 * (1 << 30),
		Estimator:     &fakeEstimator{bytes: 100 * (1 << 30)}, // 100 GiB
	}
	res, err := g.CheckBQExport(t.Context(), "SELECT *", false)
	if err != nil {
		t.Fatalf("CheckBQExport: %v", err)
	}
	if res.Decision != VolumeBlock {
		t.Errorf("decision = %v, want VolumeBlock", res.Decision)
	}
}

func TestVolumeGate_AboveHardStop_ForceWarns(t *testing.T) {
	g := &VolumeGate{
		WarnBytes:     1 << 30,
		HardStopBytes: 50 * (1 << 30),
		Estimator:     &fakeEstimator{bytes: 100 * (1 << 30)},
	}
	res, err := g.CheckBQExport(t.Context(), "SELECT *", true)
	if err != nil {
		t.Fatalf("CheckBQExport: %v", err)
	}
	if res.Decision != VolumeWarn {
		t.Errorf("force=true above hard stop should warn (not block), got %v", res.Decision)
	}
}

func TestVolumeGate_DryRunError_AllowsWithReason(t *testing.T) {
	g := &VolumeGate{
		WarnBytes:     1 << 30,
		HardStopBytes: 50 * (1 << 30),
		Estimator:     &fakeEstimator{err: errors.New("permission denied")},
	}
	res, err := g.CheckBQExport(t.Context(), "SELECT 1", false)
	if err != nil {
		t.Fatalf("CheckBQExport returned err: %v", err)
	}
	if res.Decision != VolumeAllow {
		t.Errorf("dry-run failure should allow, got %v", res.Decision)
	}
	if res.Reason == "" {
		t.Error("expected Reason explaining dry-run failure")
	}
}

func TestVolumeGate_LocalHardStopIsTighter(t *testing.T) {
	// 5 GiB: safely allowed for GCS path, blocked for local stream.
	g := &VolumeGate{
		WarnBytes:          1 << 30,
		HardStopBytes:      50 * (1 << 30),
		LocalHardStopBytes: 1 << 30,
		Estimator:          &fakeEstimator{bytes: 5 * (1 << 30)},
	}

	gcs, err := g.CheckBQExport(t.Context(), "SELECT 1", false)
	if err != nil {
		t.Fatalf("CheckBQExport: %v", err)
	}
	if gcs.Decision != VolumeWarn {
		t.Errorf("GCS path on 5 GiB: decision = %v, want VolumeWarn", gcs.Decision)
	}

	local, err := g.CheckBQLocal(t.Context(), "SELECT 1", false)
	if err != nil {
		t.Fatalf("CheckBQLocal: %v", err)
	}
	if local.Decision != VolumeBlock {
		t.Errorf("local path on 5 GiB: decision = %v, want VolumeBlock", local.Decision)
	}
	if !contains(local.Reason, "local stream") {
		t.Errorf("local block message should mention 'local stream' label, got %q", local.Reason)
	}
	if !contains(local.Reason, "force=true") {
		t.Errorf("local block message should mention force=true escape hatch, got %q", local.Reason)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestVolumeGate_NilOrUnconfigured_Allows(t *testing.T) {
	cases := []*VolumeGate{
		nil,
		{},                     // no estimator
		{Estimator: nil},
	}
	for _, g := range cases {
		res, err := g.CheckBQExport(t.Context(), "SELECT 1", false)
		if err != nil {
			t.Fatalf("CheckBQExport: %v", err)
		}
		if res.Decision != VolumeAllow {
			t.Errorf("unconfigured gate should allow, got %v", res.Decision)
		}
	}
}
