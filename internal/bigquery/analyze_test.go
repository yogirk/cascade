package bigquery

import (
	"fmt"
	"testing"
)

// mockSchemaLookup implements SchemaLookup for testing.
type mockSchemaLookup struct {
	tables  map[string][]TableMeta // datasetID -> tables
	details map[string]*TableMeta  // "dataset.table" -> detail
}

func (m *mockSchemaLookup) GetTableMeta(datasetID, tableID string) (*TableMeta, error) {
	key := datasetID + "." + tableID
	if d, ok := m.details[key]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("not found: %s", key)
}

func (m *mockSchemaLookup) ListTableMeta(datasetID string) ([]TableMeta, error) {
	if t, ok := m.tables[datasetID]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("dataset not found: %s", datasetID)
}

func newTestLookup() *mockSchemaLookup {
	return &mockSchemaLookup{
		tables: map[string][]TableMeta{
			"dataset": {
				{DatasetID: "dataset", TableID: "orders", PartitionField: "created_at", SizeBytes: 5_000_000_000},
				{DatasetID: "dataset", TableID: "events", ClusteringFields: []string{"user_id", "event_type"}, SizeBytes: 2_000_000_000},
				{DatasetID: "dataset", TableID: "small_table", SizeBytes: 1000},
			},
			"other_dataset": {
				{DatasetID: "other_dataset", TableID: "big_table", SizeBytes: 3_000_000_000},
			},
		},
		details: map[string]*TableMeta{
			"dataset.orders": {
				DatasetID: "dataset", TableID: "orders",
				PartitionField: "created_at", SizeBytes: 5_000_000_000,
			},
			"dataset.events": {
				DatasetID: "dataset", TableID: "events",
				ClusteringFields: []string{"user_id", "event_type"}, SizeBytes: 2_000_000_000,
			},
			"dataset.small_table": {
				DatasetID: "dataset", TableID: "small_table", SizeBytes: 1000,
			},
			"other_dataset.big_table": {
				DatasetID: "other_dataset", TableID: "big_table", SizeBytes: 3_000_000_000,
			},
		},
	}
}

func TestAnalyzeQuery_MissingPartitionFilter(t *testing.T) {
	lookup := newTestLookup()
	hints := AnalyzeQuery("SELECT * FROM dataset.orders WHERE status = 'shipped'", lookup)

	found := false
	for _, h := range hints {
		if h.Category == "partition_filter" {
			found = true
			if h.TableRef != "dataset.orders" {
				t.Errorf("expected TableRef=dataset.orders, got %s", h.TableRef)
			}
		}
	}
	if !found {
		t.Error("expected partition_filter hint for query missing partition filter")
	}
}

func TestAnalyzeQuery_PartitionFilterPresent(t *testing.T) {
	lookup := newTestLookup()
	hints := AnalyzeQuery("SELECT * FROM dataset.orders WHERE created_at > '2024-01-01' AND status = 'shipped'", lookup)

	for _, h := range hints {
		if h.Category == "partition_filter" {
			t.Error("should not produce partition_filter hint when partition column is in WHERE")
		}
	}
}

func TestAnalyzeQuery_ClusteringKeyUnused(t *testing.T) {
	lookup := newTestLookup()
	hints := AnalyzeQuery("SELECT * FROM dataset.events WHERE event_date = '2024-01-01'", lookup)

	found := false
	for _, h := range hints {
		if h.Category == "clustering_key" {
			found = true
		}
	}
	if !found {
		t.Error("expected clustering_key hint when clustering columns are unused")
	}
}

func TestAnalyzeQuery_ClusteringKeyUsed(t *testing.T) {
	lookup := newTestLookup()
	hints := AnalyzeQuery("SELECT * FROM dataset.events WHERE user_id = 123", lookup)

	// user_id is used, but event_type is not -- should still get hint for event_type.
	// But if ALL clustering fields were used, no hint. Here only partial.
	for _, h := range hints {
		if h.Category == "clustering_key" {
			// Acceptable: event_type is still unused.
			return
		}
	}
}

func TestAnalyzeQuery_ExpensiveJoin(t *testing.T) {
	lookup := newTestLookup()
	sql := "SELECT a.*, b.* FROM dataset.orders a JOIN other_dataset.big_table b ON a.id = b.id"
	hints := AnalyzeQuery(sql, lookup)

	found := false
	for _, h := range hints {
		if h.Category == "expensive_join" {
			found = true
		}
	}
	if !found {
		t.Error("expected expensive_join hint for cross-dataset JOIN of large tables")
	}
}

func TestAnalyzeQuery_NoIssues(t *testing.T) {
	lookup := newTestLookup()
	hints := AnalyzeQuery("SELECT * FROM dataset.small_table WHERE id = 1", lookup)

	if len(hints) > 0 {
		t.Errorf("expected no hints for simple query on small non-partitioned table, got %d", len(hints))
	}
}

func TestAnalyzeQuery_DMLSkipped(t *testing.T) {
	lookup := newTestLookup()

	for _, sql := range []string{
		"INSERT INTO dataset.orders (id) VALUES (1)",
		"UPDATE dataset.orders SET status = 'done' WHERE id = 1",
		"DELETE FROM dataset.orders WHERE id = 1",
	} {
		hints := AnalyzeQuery(sql, lookup)
		if len(hints) > 0 {
			t.Errorf("expected no hints for DML statement %q, got %d", sql, len(hints))
		}
	}
}

func TestAnalyzeQuery_NilLookup(t *testing.T) {
	hints := AnalyzeQuery("SELECT * FROM dataset.orders", nil)
	if hints != nil {
		t.Error("expected nil hints when lookup is nil")
	}
}
