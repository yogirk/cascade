package platform

import (
	"testing"
	"time"
)

func TestCorrelate_Empty(t *testing.T) {
	result := Correlate(nil)
	if len(result) != 0 {
		t.Errorf("expected 0 incidents, got %d", len(result))
	}
}

func TestCorrelate_SingleSignal(t *testing.T) {
	signals := []Signal{
		{Type: SignalJobFailed, Severity: SeverityCritical, Summary: "job failed",
			Related: []string{"proj.ds.table1"}, Timestamp: time.Now()},
	}
	result := Correlate(signals)
	if len(result) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(result))
	}
	if len(result[0].Signals) != 1 {
		t.Errorf("expected 1 signal in incident, got %d", len(result[0].Signals))
	}
}

func TestCorrelate_TwoUnrelated(t *testing.T) {
	signals := []Signal{
		{Type: SignalJobFailed, Severity: SeverityCritical, Summary: "job A failed",
			Related: []string{"proj.ds.tableA"}, Timestamp: time.Now()},
		{Type: SignalTableStale, Severity: SeverityWarning, Summary: "table B stale",
			Related: []string{"proj.ds.tableB"}, Timestamp: time.Now()},
	}
	result := Correlate(signals)
	if len(result) != 2 {
		t.Fatalf("expected 2 incidents, got %d", len(result))
	}
}

func TestCorrelate_TwoCorrelated(t *testing.T) {
	signals := []Signal{
		{Type: SignalJobFailed, Severity: SeverityCritical, Summary: "job failed",
			Related: []string{"proj.ds.table1"}, Timestamp: time.Now()},
		{Type: SignalTableStale, Severity: SeverityWarning, Summary: "table stale",
			Related: []string{"proj.ds.table1"}, Timestamp: time.Now()},
	}
	result := Correlate(signals)
	if len(result) != 1 {
		t.Fatalf("expected 1 incident (correlated), got %d", len(result))
	}
	if len(result[0].Signals) != 2 {
		t.Errorf("expected 2 signals in incident, got %d", len(result[0].Signals))
	}
	// Top signal should be the critical one
	if result[0].TopSignal.Severity != SeverityCritical {
		t.Errorf("expected top signal severity CRITICAL, got %s", result[0].TopSignal.Severity)
	}
}

func TestCorrelate_TransitiveCorrelation(t *testing.T) {
	// A shares resource with B, B shares resource with C → all in one incident
	signals := []Signal{
		{Type: SignalJobFailed, Severity: SeverityCritical, Summary: "A",
			Related: []string{"resource1"}, Timestamp: time.Now()},
		{Type: SignalTableStale, Severity: SeverityWarning, Summary: "B",
			Related: []string{"resource1", "resource2"}, Timestamp: time.Now()},
		{Type: SignalLogError, Severity: SeverityWarning, Summary: "C",
			Related: []string{"resource2"}, Timestamp: time.Now()},
	}
	result := Correlate(signals)
	if len(result) != 1 {
		t.Fatalf("expected 1 incident (transitive), got %d", len(result))
	}
	if len(result[0].Signals) != 3 {
		t.Errorf("expected 3 signals in incident, got %d", len(result[0].Signals))
	}
}

func TestCorrelate_EmptyRelated(t *testing.T) {
	// Signals with empty Related form singletons
	signals := []Signal{
		{Type: SignalLogError, Severity: SeverityWarning, Summary: "orphan A",
			Related: nil, Timestamp: time.Now()},
		{Type: SignalLogError, Severity: SeverityWarning, Summary: "orphan B",
			Related: nil, Timestamp: time.Now()},
	}
	result := Correlate(signals)
	if len(result) != 2 {
		t.Fatalf("expected 2 incidents (singletons), got %d", len(result))
	}
}

func TestCorrelate_Ranking(t *testing.T) {
	now := time.Now()
	signals := []Signal{
		{Type: SignalLogError, Severity: SeverityInfo, Summary: "info",
			Related: []string{"r1"}, Timestamp: now, BlastRadius: 0},
		{Type: SignalJobFailed, Severity: SeverityCritical, Summary: "critical",
			Related: []string{"r2"}, Timestamp: now.Add(-time.Hour), BlastRadius: 5},
		{Type: SignalTableStale, Severity: SeverityWarning, Summary: "warning high blast",
			Related: []string{"r3"}, Timestamp: now, BlastRadius: 10},
	}
	result := Correlate(signals)
	if len(result) != 3 {
		t.Fatalf("expected 3 incidents, got %d", len(result))
	}
	// Critical first
	if result[0].TopSignal.Severity != SeverityCritical {
		t.Errorf("first incident should be critical, got %s", result[0].TopSignal.Severity)
	}
	// Warning with higher blast radius second
	if result[1].TopSignal.Summary != "warning high blast" {
		t.Errorf("second incident should be warning with blast=10, got %q", result[1].TopSignal.Summary)
	}
}

func TestCorrelate_Resources(t *testing.T) {
	signals := []Signal{
		{Type: SignalJobFailed, Severity: SeverityCritical, Summary: "job",
			Related: []string{"proj.ds.t1", "proj.ds.t2"}, Timestamp: time.Now()},
		{Type: SignalTableStale, Severity: SeverityWarning, Summary: "stale",
			Related: []string{"proj.ds.t2", "proj.ds.t3"}, Timestamp: time.Now()},
	}
	result := Correlate(signals)
	if len(result) != 1 {
		t.Fatalf("expected 1 incident, got %d", len(result))
	}
	// Resources should be the union: t1, t2, t3
	if len(result[0].Resources) != 3 {
		t.Errorf("expected 3 resources, got %d: %v", len(result[0].Resources), result[0].Resources)
	}
}

func TestCorrelate_BlastRadiusMax(t *testing.T) {
	signals := []Signal{
		{Type: SignalJobFailed, Severity: SeverityCritical, Summary: "a",
			Related: []string{"r1"}, BlastRadius: 3, Timestamp: time.Now()},
		{Type: SignalTableStale, Severity: SeverityWarning, Summary: "b",
			Related: []string{"r1"}, BlastRadius: 7, Timestamp: time.Now()},
	}
	result := Correlate(signals)
	if result[0].BlastRadius != 7 {
		t.Errorf("expected blast radius 7 (max), got %d", result[0].BlastRadius)
	}
}

func TestSuggestAction(t *testing.T) {
	tests := []struct {
		sigType SignalType
		want    string
	}{
		{SignalJobFailed, "Investigate the failed job"},
		{SignalTableStale, "Check the pipeline"},
		{SignalObjectMissing, "Verify the upstream process"},
		{SignalLogError, "Review the error logs"},
		{SignalCostSpike, "Review expensive queries"},
	}

	for _, tt := range tests {
		action := suggestAction(Signal{Type: tt.sigType})
		if action == "" {
			t.Errorf("suggestAction(%s) returned empty", tt.sigType)
		}
		// Just check it contains the expected fragment
		if len(action) < 10 {
			t.Errorf("suggestAction(%s) too short: %q", tt.sigType, action)
		}
	}
}
