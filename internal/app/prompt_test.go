package app

import (
	"strings"
	"testing"

	"github.com/yogirk/cascade/internal/schema"
)

func TestBuildRequestContext_UsesRelevantSchema(t *testing.T) {
	dir := t.TempDir()
	cache := schema.NewCache(dir)
	if err := cache.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer cache.Close()

	db := cache.DB()
	_, err := db.Exec(`
		INSERT INTO datasets (project_id, dataset_id, location, last_refreshed)
		VALUES ('test-project', 'analytics', 'US', 1000)
	`)
	if err != nil {
		t.Fatalf("insert dataset: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO tables (project_id, dataset_id, table_id, table_type, description, row_count, size_bytes, last_refreshed)
		VALUES ('test-project', 'analytics', 'customers', 'TABLE', 'customer facts', 100, 1024, 1000)
	`)
	if err != nil {
		t.Fatalf("insert table: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO columns (project_id, dataset_id, table_id, column_name, data_type, is_nullable, description, ordinal_position)
		VALUES ('test-project', 'analytics', 'customers', 'customer_id', 'STRING', 0, 'customer identifier', 1)
	`)
	if err != nil {
		t.Fatalf("insert column: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO schema_fts (project_id, dataset_id, table_id, column_name, description)
		VALUES ('test-project', 'analytics', 'customers', 'customer_id', 'customer identifier')
	`)
	if err != nil {
		t.Fatalf("insert fts: %v", err)
	}

	ctx := BuildRequestContext(&BigQueryComponents{Cache: cache}, "customer")
	if !strings.Contains(ctx, "Relevant Schema Context") {
		t.Fatalf("expected request context header, got %q", ctx)
	}
	if !strings.Contains(ctx, "analytics.customers") {
		t.Fatalf("expected relevant table in request context, got %q", ctx)
	}
}
