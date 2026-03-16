package bigquery

import (
	"sync"
	"testing"
)

func TestNewCostTracker(t *testing.T) {
	ct := NewCostTracker(100.0)
	if ct.SessionTotal() != 0 {
		t.Errorf("new tracker total = %f, want 0", ct.SessionTotal())
	}
	if len(ct.Entries()) != 0 {
		t.Errorf("new tracker entries = %d, want 0", len(ct.Entries()))
	}
}

func TestCostTrackerRecordSingle(t *testing.T) {
	ct := NewCostTracker(100.0)
	ct.Record(QueryCostEntry{
		SQL:          "SELECT 1",
		BytesScanned: 1e9,
		Cost:         0.50,
		DurationMs:   120,
	})

	if ct.SessionTotal() != 0.50 {
		t.Errorf("total = %f, want 0.50", ct.SessionTotal())
	}
	if len(ct.Entries()) != 1 {
		t.Errorf("entries = %d, want 1", len(ct.Entries()))
	}
}

func TestCostTrackerRecordMultiple(t *testing.T) {
	ct := NewCostTracker(100.0)
	ct.Record(QueryCostEntry{Cost: 1.50})
	ct.Record(QueryCostEntry{Cost: 2.50})
	ct.Record(QueryCostEntry{Cost: 3.00})

	if ct.SessionTotal() != 7.00 {
		t.Errorf("total = %f, want 7.00", ct.SessionTotal())
	}
	if len(ct.Entries()) != 3 {
		t.Errorf("entries = %d, want 3", len(ct.Entries()))
	}
}

func TestCostTrackerDMLNegativeCost(t *testing.T) {
	ct := NewCostTracker(100.0)
	ct.Record(QueryCostEntry{
		SQL:   "INSERT INTO t VALUES (1)",
		Cost:  -1, // DML signals cannot estimate
		IsDML: true,
	})

	if ct.SessionTotal() != 0 {
		t.Errorf("DML with negative cost should not add to total, got %f", ct.SessionTotal())
	}
}

func TestCostTrackerBudgetPercent(t *testing.T) {
	tests := []struct {
		name        string
		budget      float64
		costs       []float64
		wantPercent float64
	}{
		{"zero budget", 0, []float64{50}, 0},
		{"50 percent", 100, []float64{50}, 50},
		{"80 percent", 100, []float64{80}, 80},
		{"100 percent", 100, []float64{100}, 100},
		{"over budget", 100, []float64{150}, 150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := NewCostTracker(tt.budget)
			for _, c := range tt.costs {
				ct.Record(QueryCostEntry{Cost: c})
			}
			got := ct.BudgetPercent()
			if got != tt.wantPercent {
				t.Errorf("BudgetPercent() = %f, want %f", got, tt.wantPercent)
			}
		})
	}
}

func TestCostTrackerIsOverBudgetWarning(t *testing.T) {
	tests := []struct {
		name string
		cost float64
		want bool
	}{
		{"79 percent", 79, false},
		{"80 percent", 80, true},
		{"100 percent", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := NewCostTracker(100.0)
			ct.Record(QueryCostEntry{Cost: tt.cost})
			if got := ct.IsOverBudgetWarning(); got != tt.want {
				t.Errorf("IsOverBudgetWarning() = %v, want %v (cost=%f)", got, tt.want, tt.cost)
			}
		})
	}
}

func TestCostTrackerConcurrency(t *testing.T) {
	ct := NewCostTracker(1000.0)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct.Record(QueryCostEntry{Cost: 1.0})
		}()
	}

	wg.Wait()

	if ct.SessionTotal() != 100.0 {
		t.Errorf("concurrent total = %f, want 100.0", ct.SessionTotal())
	}
	if len(ct.Entries()) != 100 {
		t.Errorf("concurrent entries = %d, want 100", len(ct.Entries()))
	}
}

func TestCostTrackerEntriesCopy(t *testing.T) {
	ct := NewCostTracker(100.0)
	ct.Record(QueryCostEntry{SQL: "SELECT 1", Cost: 1.0})

	entries := ct.Entries()
	entries[0].Cost = 999 // mutate the copy

	// Original should be unchanged.
	original := ct.Entries()
	if original[0].Cost != 1.0 {
		t.Errorf("Entries() returned a reference, not a copy: cost = %f", original[0].Cost)
	}
}
