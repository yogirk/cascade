package bigquery

import "sync"

// QueryCostEntry records the cost and metadata of a single query execution.
type QueryCostEntry struct {
	SQL          string
	BytesScanned int64
	Cost         float64
	DurationMs   int64
	IsDML        bool
}

// CostTracker accumulates per-query costs and provides session totals.
// It is safe for concurrent use.
type CostTracker struct {
	mu           sync.Mutex
	entries      []QueryCostEntry
	sessionTotal float64
	dailyBudget  float64
}

// NewCostTracker creates a cost tracker with the given daily budget.
func NewCostTracker(dailyBudget float64) *CostTracker {
	return &CostTracker{
		dailyBudget: dailyBudget,
	}
}

// Record adds a query cost entry and updates the session total.
func (t *CostTracker) Record(entry QueryCostEntry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, entry)
	if entry.Cost > 0 {
		t.sessionTotal += entry.Cost
	}
}

// SessionTotal returns the accumulated cost for this session.
func (t *CostTracker) SessionTotal() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sessionTotal
}

// Entries returns a copy of all recorded cost entries.
func (t *CostTracker) Entries() []QueryCostEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	cp := make([]QueryCostEntry, len(t.entries))
	copy(cp, t.entries)
	return cp
}

// BudgetPercent returns the percentage of the daily budget consumed.
// Returns 0 if no daily budget is set.
func (t *CostTracker) BudgetPercent() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.dailyBudget <= 0 {
		return 0
	}
	return t.sessionTotal / t.dailyBudget * 100
}

// IsOverBudgetWarning returns true if session cost is >= 80% of the daily budget.
func (t *CostTracker) IsOverBudgetWarning() bool {
	return t.BudgetPercent() >= 80
}
