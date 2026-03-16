// Package schema provides a SQLite-based schema cache with FTS5 full-text
// search for BigQuery table and column metadata.
package schema

import "database/sql"

const schemaV2SQL = `
CREATE TABLE IF NOT EXISTS datasets (
    project_id TEXT NOT NULL,
    dataset_id TEXT NOT NULL,
    location TEXT,
    description TEXT,
    labels TEXT,
    last_refreshed INTEGER NOT NULL,
    PRIMARY KEY (project_id, dataset_id)
);

CREATE TABLE IF NOT EXISTS tables (
    project_id TEXT NOT NULL,
    dataset_id TEXT NOT NULL,
    table_id TEXT NOT NULL,
    table_type TEXT,
    description TEXT,
    row_count INTEGER,
    size_bytes INTEGER,
    partition_field TEXT,
    partition_type TEXT,
    clustering_fields TEXT,
    labels TEXT,
    last_modified INTEGER,
    last_refreshed INTEGER NOT NULL,
    PRIMARY KEY (project_id, dataset_id, table_id)
);

CREATE TABLE IF NOT EXISTS columns (
    project_id TEXT NOT NULL,
    dataset_id TEXT NOT NULL,
    table_id TEXT NOT NULL,
    column_name TEXT NOT NULL,
    data_type TEXT NOT NULL,
    is_nullable INTEGER,
    description TEXT,
    ordinal_position INTEGER,
    is_partitioning INTEGER DEFAULT 0,
    clustering_ordinal INTEGER,
    PRIMARY KEY (project_id, dataset_id, table_id, column_name)
);

CREATE VIRTUAL TABLE IF NOT EXISTS schema_fts USING fts5(
    project_id UNINDEXED,
    dataset_id,
    table_id,
    column_name,
    description
);

CREATE INDEX IF NOT EXISTS idx_columns_table ON columns(project_id, dataset_id, table_id);
`

// migrate creates or upgrades the schema to v2 (multi-project).
// If old v1 tables exist (no project_id in PKs), they are dropped and
// the cache will be repopulated on next /sync.
func migrate(db *sql.DB) error {
	// Check if we need to migrate from v1 → v2.
	// v1 datasets table has dataset_id as sole PK; v2 has (project_id, dataset_id).
	// Detect v1 by checking if datasets PK has only 1 column.
	var needsMigration bool
	row := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('datasets') WHERE pk > 0
	`)
	var pkCount int
	if err := row.Scan(&pkCount); err != nil {
		// Table doesn't exist yet — fresh install, just create v2.
		_, err := db.Exec(schemaV2SQL)
		return err
	}

	if pkCount == 1 {
		// v1 schema detected — drop old tables and recreate.
		needsMigration = true
	}

	if needsMigration {
		for _, table := range []string{"schema_fts", "columns", "tables", "datasets"} {
			db.Exec("DROP TABLE IF EXISTS " + table)
		}
	}

	_, err := db.Exec(schemaV2SQL)
	return err
}
