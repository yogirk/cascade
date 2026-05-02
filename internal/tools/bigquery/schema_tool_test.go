package bigquery

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/slokam-ai/cascade/internal/schema"
)

func TestSchemaToolName(t *testing.T) {
	st := &SchemaTool{}
	if got := st.Name(); got != "bigquery_schema" {
		t.Errorf("Name() = %q, want %q", got, "bigquery_schema")
	}
}

func TestSchemaToolRiskLevel(t *testing.T) {
	st := &SchemaTool{}
	// Should be RiskReadOnly (0).
	if got := st.RiskLevel(); got != 0 {
		t.Errorf("RiskLevel() = %d, want 0 (RiskReadOnly)", got)
	}
}

func TestSchemaToolInputSchema(t *testing.T) {
	st := &SchemaTool{}
	s := st.InputSchema()

	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatal("InputSchema missing properties")
	}

	actionProp, ok := props["action"].(map[string]any)
	if !ok {
		t.Fatal("InputSchema missing 'action' property")
	}

	enumVals, ok := actionProp["enum"].([]string)
	if !ok {
		t.Fatal("InputSchema action missing enum")
	}

	expected := map[string]bool{
		"list_datasets":  false,
		"list_tables":    false,
		"describe_table": false,
		"search_columns": false,
	}
	for _, v := range enumVals {
		if _, ok := expected[v]; ok {
			expected[v] = true
		}
	}
	for k, found := range expected {
		if !found {
			t.Errorf("InputSchema action enum missing %q", k)
		}
	}

	req, ok := s["required"].([]string)
	if !ok {
		t.Fatal("InputSchema missing required")
	}
	if len(req) != 1 || req[0] != "action" {
		t.Errorf("InputSchema required = %v, want [action]", req)
	}
}

func TestSchemaToolNotPopulated(t *testing.T) {
	// Create a cache that is not populated (no Open call, so db is nil).
	cache := schema.NewCache(t.TempDir())
	st := NewSchemaTool(cache, "test-project")

	input, _ := json.Marshal(map[string]string{"action": "list_datasets"})
	result, err := st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.IsError {
		t.Error("Expected non-error result for unpopulated cache")
	}
	if !containsStr(result.Content, "No schema cache found") {
		t.Errorf("unexpected content: %q", result.Content)
	}
}

func TestSchemaToolListDatasets(t *testing.T) {
	cache := setupTestCache(t)
	st := NewSchemaTool(cache, "test-project")

	input, _ := json.Marshal(map[string]string{"action": "list_datasets"})
	result, err := st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
	if !containsStr(result.Content, "test_dataset") {
		t.Errorf("content should contain test_dataset: %q", result.Content)
	}
}

func TestSchemaToolDescribeTable(t *testing.T) {
	cache := setupTestCache(t)
	st := NewSchemaTool(cache, "test-project")

	input, _ := json.Marshal(map[string]string{
		"action":  "describe_table",
		"dataset": "test_dataset",
		"table":   "users",
	})
	result, err := st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
	if !containsStr(result.Content, "user_id") {
		t.Errorf("content should contain user_id column: %q", result.Content)
	}
	if !containsStr(result.Content, "STRING") {
		t.Errorf("content should contain STRING type: %q", result.Content)
	}
}

func TestSchemaToolSearchColumns(t *testing.T) {
	cache := setupTestCache(t)
	st := NewSchemaTool(cache, "test-project")

	input, _ := json.Marshal(map[string]string{
		"action": "search_columns",
		"query":  "user",
	})
	result, err := st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
	// FTS5 search should find columns with "user" in the name.
	if !containsStr(result.Content, "user_id") {
		t.Errorf("content should contain user_id from search: %q", result.Content)
	}
}

func TestSchemaToolListTables(t *testing.T) {
	cache := setupTestCache(t)
	st := NewSchemaTool(cache, "test-project")

	input, _ := json.Marshal(map[string]string{
		"action":  "list_tables",
		"dataset": "test_dataset",
	})
	result, err := st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if result.IsError {
		t.Errorf("unexpected error: %s", result.Content)
	}
	if !containsStr(result.Content, "users") {
		t.Errorf("content should contain users table: %q", result.Content)
	}
}

func TestSchemaToolMissingParams(t *testing.T) {
	cache := setupTestCache(t)
	st := NewSchemaTool(cache, "test-project")

	// describe_table without dataset.
	input, _ := json.Marshal(map[string]string{"action": "describe_table"})
	result, err := st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing dataset")
	}

	// list_tables without dataset.
	input, _ = json.Marshal(map[string]string{"action": "list_tables"})
	result, err = st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing dataset in list_tables")
	}

	// search_columns without query.
	input, _ = json.Marshal(map[string]string{"action": "search_columns"})
	result, err = st.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing query in search_columns")
	}
}

// setupTestCache creates a temporary SQLite schema cache populated with test data.
func setupTestCache(t *testing.T) *schema.Cache {
	t.Helper()

	dir := t.TempDir()
	cache := schema.NewCache(dir)
	if err := cache.Open(); err != nil {
		t.Fatalf("failed to open cache: %v", err)
	}
	t.Cleanup(func() { cache.Close() })

	// Verify the DB file was created.
	dbPath := filepath.Join(dir, "cascade.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}

	db := cache.DB()

	// Insert test dataset (last_refreshed is NOT NULL).
	_, err := db.Exec(`INSERT INTO datasets (project_id, dataset_id, location, last_refreshed) VALUES (?, ?, ?, ?)`,
		"test-project", "test_dataset", "US", 1700000000)
	if err != nil {
		t.Fatalf("insert dataset: %v", err)
	}

	// Insert test table (last_refreshed is NOT NULL).
	_, err = db.Exec(`INSERT INTO tables (project_id, dataset_id, table_id, table_type, row_count, size_bytes, partition_field, clustering_fields, last_refreshed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-project", "test_dataset", "users", "TABLE", 1000, 1048576, "created_at", `["user_id"]`, 1700000000)
	if err != nil {
		t.Fatalf("insert table: %v", err)
	}

	// Insert test columns.
	cols := []struct {
		name    string
		dtype   string
		ord     int
		isPart  bool
		cluster int
	}{
		{"user_id", "STRING", 1, false, 1},
		{"email", "STRING", 2, false, 0},
		{"created_at", "TIMESTAMP", 3, true, 0},
		{"active", "BOOL", 4, false, 0},
	}
	for _, c := range cols {
		isPart := 0
		if c.isPart {
			isPart = 1
		}
		_, err := db.Exec(`INSERT INTO columns (project_id, dataset_id, table_id, column_name, data_type, is_nullable, ordinal_position, is_partitioning, clustering_ordinal)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"test-project", "test_dataset", "users", c.name, c.dtype, 1, c.ord, isPart, c.cluster)
		if err != nil {
			t.Fatalf("insert column %s: %v", c.name, err)
		}

		// Insert into FTS index (schema_fts has: dataset_id, table_id, column_name, description).
		_, err = db.Exec(`INSERT INTO schema_fts (project_id, dataset_id, table_id, column_name, description)
			VALUES (?, ?, ?, ?, ?)`,
			"test-project", "test_dataset", "users", c.name, "")
		if err != nil {
			t.Fatalf("insert fts %s: %v", c.name, err)
		}
	}

	return cache
}
