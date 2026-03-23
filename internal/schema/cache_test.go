package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestCache(t *testing.T) (*Cache, string) {
	t.Helper()
	dir := t.TempDir()
	cache := NewCache(dir)
	if err := cache.Open(); err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	return cache, dir
}

func TestCacheOpenClose(t *testing.T) {
	dir := t.TempDir()
	cache := NewCache(dir)

	if err := cache.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	dbPath := filepath.Join(dir, "cascade.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("DB file not created at %s", dbPath)
	}

	if err := cache.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCacheMigration(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	db := cache.DB()

	// Verify tables exist by querying them.
	for _, table := range []string{"datasets", "tables", "columns"} {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s not accessible: %v", table, err)
		}
	}

	// Verify FTS5 virtual table exists.
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_fts").Scan(&count)
	if err != nil {
		t.Errorf("schema_fts not accessible: %v", err)
	}
}

func TestCacheWALMode(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	db := cache.DB()
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestCacheIsPopulated(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	// Empty cache should not be populated.
	if cache.IsPopulated() {
		t.Error("empty cache reports IsPopulated=true")
	}

	// Insert a table row.
	db := cache.DB()
	_, err := db.Exec(`
		INSERT INTO tables (project_id, dataset_id, table_id, table_type, last_refreshed)
		VALUES ('test-project', 'ds1', 'tbl1', 'TABLE', 1000)
	`)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if !cache.IsPopulated() {
		t.Error("populated cache reports IsPopulated=false")
	}
}

func insertTestData(t *testing.T, cache *Cache) {
	t.Helper()
	db := cache.DB()

	// Insert dataset.
	_, err := db.Exec(`
		INSERT INTO datasets (project_id, dataset_id, location, description, labels, last_refreshed)
		VALUES ('test-project', 'analytics', 'US', 'Analytics dataset', '', 1000)
	`)
	if err != nil {
		t.Fatalf("insert dataset: %v", err)
	}

	// Insert tables.
	_, err = db.Exec(`
		INSERT INTO tables (project_id, dataset_id, table_id, table_type, description, row_count, size_bytes,
		                    partition_field, clustering_fields, last_refreshed)
		VALUES ('test-project', 'analytics', 'orders', 'TABLE', 'Customer orders', 1247832, 2147483648,
		        'order_date', '["customer_id","region"]', 1000)
	`)
	if err != nil {
		t.Fatalf("insert table: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO tables (project_id, dataset_id, table_id, table_type, description, row_count, size_bytes,
		                    partition_field, clustering_fields, last_refreshed)
		VALUES ('test-project', 'analytics', 'customers', 'TABLE', 'Customer master data', 50000, 10485760,
		        '', '[]', 1000)
	`)
	if err != nil {
		t.Fatalf("insert table: %v", err)
	}

	// Insert columns for orders.
	columns := []struct {
		table, name, dtype string
		nullable, pos      int
		partition, cluster int
		desc               string
	}{
		{"orders", "customer_id", "STRING", 0, 1, 0, 1, "Customer identifier"},
		{"orders", "order_date", "DATE", 0, 2, 1, 0, "Date of order"},
		{"orders", "region", "STRING", 1, 3, 0, 2, "Geographic region"},
		{"orders", "order_total", "FLOAT64", 1, 4, 0, 0, "Order total amount"},
		{"customers", "customer_id", "STRING", 0, 1, 0, 0, "Unique customer ID"},
		{"customers", "name", "STRING", 0, 2, 0, 0, "Customer full name"},
		{"customers", "email", "STRING", 1, 3, 0, 0, "Contact email address"},
	}

	for _, col := range columns {
		_, err := db.Exec(`
			INSERT INTO columns (project_id, dataset_id, table_id, column_name, data_type, is_nullable,
			                     description, ordinal_position, is_partitioning, clustering_ordinal)
			VALUES ('test-project', 'analytics', ?, ?, ?, ?, ?, ?, ?, ?)
		`, col.table, col.name, col.dtype, col.nullable, col.desc, col.pos, col.partition, col.cluster)
		if err != nil {
			t.Fatalf("insert column %s.%s: %v", col.table, col.name, err)
		}
	}

	// Insert FTS entries for all columns.
	for _, col := range columns {
		_, err := db.Exec(`
			INSERT INTO schema_fts (project_id, dataset_id, table_id, column_name, description)
			VALUES ('test-project', 'analytics', ?, ?, ?)
		`, col.table, col.name, col.desc)
		if err != nil {
			t.Fatalf("insert fts %s.%s: %v", col.table, col.name, err)
		}
	}
}

func TestCacheInsertAndGetTables(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	tables, err := cache.GetTables("analytics")
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("got %d tables, want 2", len(tables))
	}

	// Tables are ordered by table_id, so customers first.
	if tables[0].TableID != "customers" {
		t.Errorf("first table = %q, want %q", tables[0].TableID, "customers")
	}
	if tables[1].TableID != "orders" {
		t.Errorf("second table = %q, want %q", tables[1].TableID, "orders")
	}
	if tables[1].RowCount != 1247832 {
		t.Errorf("orders.RowCount = %d, want 1247832", tables[1].RowCount)
	}
}

func TestCacheGetTableDetail(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	detail, err := cache.GetTableDetail("analytics", "orders")
	if err != nil {
		t.Fatalf("GetTableDetail: %v", err)
	}

	if detail.TableID != "orders" {
		t.Errorf("TableID = %q, want %q", detail.TableID, "orders")
	}
	if detail.RowCount != 1247832 {
		t.Errorf("RowCount = %d, want 1247832", detail.RowCount)
	}
	if detail.PartitionField != "order_date" {
		t.Errorf("PartitionField = %q, want %q", detail.PartitionField, "order_date")
	}
	if len(detail.ClusteringFields) != 2 {
		t.Fatalf("ClusteringFields = %v, want 2 fields", detail.ClusteringFields)
	}
	if detail.ClusteringFields[0] != "customer_id" {
		t.Errorf("ClusteringFields[0] = %q, want %q", detail.ClusteringFields[0], "customer_id")
	}
	if len(detail.Columns) != 4 {
		t.Fatalf("columns = %d, want 4", len(detail.Columns))
	}

	// Check first column.
	col := detail.Columns[0]
	if col.Name != "customer_id" {
		t.Errorf("column[0].Name = %q, want %q", col.Name, "customer_id")
	}
	if col.DataType != "STRING" {
		t.Errorf("column[0].DataType = %q, want %q", col.DataType, "STRING")
	}
	if col.IsNullable {
		t.Error("column[0].IsNullable = true, want false")
	}
}

func TestCacheGetDatasets(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	datasets, err := cache.GetDatasets()
	if err != nil {
		t.Fatalf("GetDatasets: %v", err)
	}

	if len(datasets) != 1 {
		t.Fatalf("got %d datasets, want 1", len(datasets))
	}

	ds := datasets[0]
	if ds.DatasetID != "analytics" {
		t.Errorf("DatasetID = %q, want %q", ds.DatasetID, "analytics")
	}
	if ds.ProjectID != "test-project" {
		t.Errorf("ProjectID = %q, want %q", ds.ProjectID, "test-project")
	}
	if ds.TableCount != 2 {
		t.Errorf("TableCount = %d, want 2", ds.TableCount)
	}
}

func TestCacheInvalidateTable(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	// Verify orders exists.
	tables, err := cache.GetTables("analytics")
	if err != nil {
		t.Fatalf("GetTables before invalidate: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("before: %d tables, want 2", len(tables))
	}

	// Invalidate orders.
	if err := cache.InvalidateTable("analytics", "orders"); err != nil {
		t.Fatalf("InvalidateTable: %v", err)
	}

	// Check orders is gone.
	tables, err = cache.GetTables("analytics")
	if err != nil {
		t.Fatalf("GetTables after invalidate: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("after: %d tables, want 1", len(tables))
	}
	if tables[0].TableID != "customers" {
		t.Errorf("remaining table = %q, want %q", tables[0].TableID, "customers")
	}

	// Check columns are gone.
	db := cache.DB()
	var colCount int
	err = db.QueryRow("SELECT COUNT(*) FROM columns WHERE dataset_id='analytics' AND table_id='orders'").Scan(&colCount)
	if err != nil {
		t.Fatalf("count columns: %v", err)
	}
	if colCount != 0 {
		t.Errorf("columns after invalidate = %d, want 0", colCount)
	}
}

func TestFTS5Search(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	// Search for "customer" should find both tables.
	refs, err := cache.Search("customer", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("Search returned no results")
	}

	// Both orders and customers should appear (both have customer_id).
	tableIDs := make(map[string]bool)
	for _, ref := range refs {
		tableIDs[ref.TableID] = true
	}
	if !tableIDs["orders"] {
		t.Error("Search for 'customer' did not find orders table")
	}
	if !tableIDs["customers"] {
		t.Error("Search for 'customer' did not find customers table")
	}

	// Search for "email" should only find customers.
	refs, err = cache.Search("email", 10)
	if err != nil {
		t.Fatalf("Search email: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("Search email: got %d results, want 1", len(refs))
	}
	if refs[0].TableID != "customers" {
		t.Errorf("Search email: table = %q, want %q", refs[0].TableID, "customers")
	}
}

func TestFTS5SearchNoResults(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	// Search for something that doesn't exist.
	refs, err := cache.Search("nonexistent_xyz", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("Search for nonexistent: got %d results, want 0", len(refs))
	}

	// Empty query returns nil.
	refs, err = cache.Search("", 10)
	if err != nil {
		t.Fatalf("Search empty: %v", err)
	}
	if refs != nil {
		t.Errorf("Search empty: got %v, want nil", refs)
	}
}

func TestSearchColumns(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	results, err := cache.SearchColumns("order", 10)
	if err != nil {
		t.Fatalf("SearchColumns: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchColumns returned no results")
	}

	// Should find order-related columns.
	foundOrderDate := false
	for _, r := range results {
		if r.ColumnName == "order_date" {
			foundOrderDate = true
			if r.DataType != "DATE" {
				t.Errorf("order_date.DataType = %q, want %q", r.DataType, "DATE")
			}
		}
	}
	if !foundOrderDate {
		t.Error("SearchColumns did not find order_date column")
	}
}

func TestBuildSchemaContext(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	ctx, err := BuildSchemaContext(cache, "customer", 10)
	if err != nil {
		t.Fatalf("BuildSchemaContext: %v", err)
	}

	if ctx == "" {
		t.Fatal("BuildSchemaContext returned empty string")
	}

	// Should contain expected formatting elements.
	checks := []string{
		"## Available Tables",
		"### `test-project.analytics.",
		"Type: TABLE",
		"customer_id STRING",
	}
	for _, check := range checks {
		if !containsString(ctx, check) {
			t.Errorf("context missing %q", check)
		}
	}
}

func TestBuildDatasetSummary(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	summary, err := BuildDatasetSummary(cache)
	if err != nil {
		t.Fatalf("BuildDatasetSummary: %v", err)
	}

	if !containsString(summary, "Project: test-project") {
		t.Error("summary missing project")
	}
	if !containsString(summary, "analytics") {
		t.Error("summary missing dataset name")
	}
	if !containsString(summary, "2 tables") {
		t.Error("summary missing table count")
	}
}

func TestBuildSchemaContextNoResults(t *testing.T) {
	cache, _ := setupTestCache(t)
	defer cache.Close()

	insertTestData(t, cache)

	ctx, err := BuildSchemaContext(cache, "nonexistent_xyz", 10)
	if err != nil {
		t.Fatalf("BuildSchemaContext: %v", err)
	}
	if ctx != "" {
		t.Errorf("expected empty context for no results, got %q", ctx)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s, substr))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
