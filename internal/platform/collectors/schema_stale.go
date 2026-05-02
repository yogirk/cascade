package collectors

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/slokam-ai/cascade/internal/platform"
	"github.com/slokam-ai/cascade/internal/schema"
)

// SchemaStaleCollector detects tables whose schema cache entries are stale.
type SchemaStaleCollector struct {
	cache          *schema.Cache
	staleHours     int
	criticalTables []string
}

// NewSchemaStaleCollector creates a collector that checks for stale table metadata.
// staleHours is the default threshold when insufficient refresh history exists.
// criticalTables is a list of fully-qualified refs ("project.dataset.table") to
// escalate to Critical severity.
func NewSchemaStaleCollector(cache *schema.Cache, staleHours int, criticalTables []string) *SchemaStaleCollector {
	if staleHours <= 0 {
		staleHours = 24
	}
	return &SchemaStaleCollector{
		cache:          cache,
		staleHours:     staleHours,
		criticalTables: criticalTables,
	}
}

// Source returns the signal source identifier.
func (c *SchemaStaleCollector) Source() platform.SignalSource {
	return platform.SourceSchema
}

// Collect scans the schema cache for tables whose last_refreshed timestamp
// exceeds the expected refresh interval.
func (c *SchemaStaleCollector) Collect(ctx context.Context, since time.Duration) ([]platform.Signal, error) {
	if c.cache == nil || !c.cache.IsPopulated() {
		return nil, nil
	}

	db := c.cache.DB()
	if db == nil {
		return nil, nil
	}

	datasets, err := c.cache.GetDatasets()
	if err != nil {
		return nil, fmt.Errorf("get datasets: %w", err)
	}

	criticalSet := make(map[string]bool, len(c.criticalTables))
	for _, t := range c.criticalTables {
		criticalSet[t] = true
	}

	now := time.Now()
	defaultThreshold := time.Duration(c.staleHours) * time.Hour
	var signals []platform.Signal

	for _, ds := range datasets {
		// Query all tables in this dataset with their last_refreshed timestamps.
		rows, err := db.QueryContext(ctx, `
			SELECT table_id, last_refreshed
			FROM tables
			WHERE project_id = ? AND dataset_id = ?
			ORDER BY table_id
		`, ds.ProjectID, ds.DatasetID)
		if err != nil {
			return nil, fmt.Errorf("query tables for %s.%s: %w", ds.ProjectID, ds.DatasetID, err)
		}

		var tables []tableRefresh
		for rows.Next() {
			var tid string
			var lr int64
			if err := rows.Scan(&tid, &lr); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan table row: %w", err)
			}
			tables = append(tables, tableRefresh{
				tableID:       tid,
				lastRefreshed: time.Unix(lr, 0),
			})
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}

		// Determine expected interval from the two most recent refresh timestamps.
		expectedInterval := defaultThreshold
		if interval, ok := inferRefreshInterval(tables); ok {
			expectedInterval = interval
		}

		// A table is stale if last_refreshed is older than 2x the expected interval.
		staleThreshold := 2 * expectedInterval

		for _, t := range tables {
			age := now.Sub(t.lastRefreshed)
			if age <= staleThreshold {
				continue
			}

			ref := fmt.Sprintf("%s.%s.%s", ds.ProjectID, ds.DatasetID, t.tableID)

			severity := platform.SeverityWarning
			if criticalSet[ref] {
				severity = platform.SeverityCritical
			}

			summary := fmt.Sprintf("Table %s last refreshed %s ago (expected every %s)",
				ref, formatDuration(age), formatDuration(expectedInterval))

			sig := platform.Signal{
				Type:      platform.SignalTableStale,
				Severity:  severity,
				Source:    platform.SourceSchema,
				Timestamp: now,
				Summary:   summary,
				Details: map[string]any{
					"last_refreshed":    t.lastRefreshed,
					"age":               age.String(),
					"expected_interval": expectedInterval.String(),
				},
				Related:     []string{ref},
				BlastRadius: 0,
			}
			signals = append(signals, sig)
		}
	}

	return signals, nil
}

// inferRefreshInterval calculates the interval between the two most recent
// distinct refresh timestamps across all tables in a dataset.
// Returns false if fewer than 2 distinct timestamps exist.
func inferRefreshInterval(tables []tableRefresh) (time.Duration, bool) {
	if len(tables) < 2 {
		return 0, false
	}

	// Collect unique timestamps.
	seen := make(map[int64]bool, len(tables))
	var unique []time.Time
	for _, t := range tables {
		unix := t.lastRefreshed.Unix()
		if !seen[unix] {
			seen[unix] = true
			unique = append(unique, t.lastRefreshed)
		}
	}

	if len(unique) < 2 {
		return 0, false
	}

	// Sort descending.
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].After(unique[j])
	})

	interval := unique[0].Sub(unique[1])
	if interval <= 0 {
		return 0, false
	}
	return interval, true
}

type tableRefresh struct {
	tableID       string
	lastRefreshed time.Time
}

// formatDuration returns a human-friendly duration string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := d.Hours()
	if hours < 48 {
		return fmt.Sprintf("%.0fh", hours)
	}
	return fmt.Sprintf("%.1fd", hours/24)
}
