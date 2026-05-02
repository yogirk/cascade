package tui

import bq "github.com/slokam-ai/cascade/internal/bigquery"

// costTrackerAdapter adapts bigquery.CostTracker to the CostTrackerView interface.
// This avoids exposing bigquery types in the TUI layer.
type costTrackerAdapter struct {
	tracker *bq.CostTracker
}

func newCostTrackerAdapter(t *bq.CostTracker) CostTrackerView {
	return &costTrackerAdapter{tracker: t}
}

func (a *costTrackerAdapter) Entries() []CostEntry {
	bqEntries := a.tracker.Entries()
	entries := make([]CostEntry, len(bqEntries))
	for i, e := range bqEntries {
		entries[i] = CostEntry{
			SQL:          e.SQL,
			BytesScanned: e.BytesScanned,
			Cost:         e.Cost,
			DurationMs:   e.DurationMs,
			IsDML:        e.IsDML,
		}
	}
	return entries
}

func (a *costTrackerAdapter) SessionTotal() float64 {
	return a.tracker.SessionTotal()
}

func (a *costTrackerAdapter) BudgetPercent() float64 {
	return a.tracker.BudgetPercent()
}

func (a *costTrackerAdapter) IsOverBudgetWarning() bool {
	return a.tracker.IsOverBudgetWarning()
}
