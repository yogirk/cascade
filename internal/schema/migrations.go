// Package schema provides a SQLite-based schema cache with FTS5 full-text
// search for BigQuery table and column metadata.
package schema

import "database/sql"

const schemaSQL = `
CREATE TABLE IF NOT EXISTS datasets (
    dataset_id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    location TEXT,
    description TEXT,
    labels TEXT,
    last_refreshed INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tables (
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
    PRIMARY KEY (dataset_id, table_id)
);

CREATE TABLE IF NOT EXISTS columns (
    dataset_id TEXT NOT NULL,
    table_id TEXT NOT NULL,
    column_name TEXT NOT NULL,
    data_type TEXT NOT NULL,
    is_nullable INTEGER,
    description TEXT,
    ordinal_position INTEGER,
    is_partitioning INTEGER DEFAULT 0,
    clustering_ordinal INTEGER,
    PRIMARY KEY (dataset_id, table_id, column_name)
);

CREATE VIRTUAL TABLE IF NOT EXISTS schema_fts USING fts5(
    dataset_id,
    table_id,
    column_name,
    description
);

CREATE INDEX IF NOT EXISTS idx_columns_table ON columns(dataset_id, table_id);
`

// migrate runs the schema DDL to create or update tables.
func migrate(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}
