package schema

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// DatasetInfo holds summary metadata for a dataset.
type DatasetInfo struct {
	ProjectID   string
	DatasetID   string
	Location    string
	Description string
	TableCount  int
	TotalBytes  int64
}

// TableInfo holds summary metadata for a table.
type TableInfo struct {
	ProjectID        string
	DatasetID        string
	TableID          string
	TableType        string
	Description      string
	RowCount         int64
	SizeBytes        int64
	PartitionField   string
	ClusteringFields []string
}

// TableDetail holds full table metadata including columns.
type TableDetail struct {
	TableInfo
	Columns []ColumnInfo
}

// ColumnInfo holds metadata for a single column.
type ColumnInfo struct {
	Name              string
	DataType          string
	IsNullable        bool
	Description       string
	OrdinalPosition   int
	IsPartitioning    bool
	ClusteringOrdinal int
}

// Cache manages a unified SQLite schema cache with FTS5 across all projects.
type Cache struct {
	mu       sync.RWMutex
	db       *sql.DB
	cacheDir string
}

// NewCache creates a new cache manager rooted at the given directory.
func NewCache(cacheDir string) *Cache {
	return &Cache{cacheDir: cacheDir}
}

// Open opens or creates the unified SQLite database (cascade.db).
// It enables WAL mode and busy timeout, then runs migrations.
func (c *Cache) Open() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	dbPath := filepath.Join(c.cacheDir, "cascade.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for concurrent read access during background refresh.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return fmt.Errorf("set WAL mode: %w", err)
	}

	// Set busy timeout to avoid "database is locked" during concurrent access.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return fmt.Errorf("set busy timeout: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return fmt.Errorf("migrate: %w", err)
	}

	c.db = db
	return nil
}

// Close closes the underlying SQLite database.
func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db != nil {
		err := c.db.Close()
		c.db = nil
		return err
	}
	return nil
}

// DB returns the underlying *sql.DB for use by populate and search operations.
func (c *Cache) DB() *sql.DB {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.db
}

// IsPopulated returns true if the tables table has any rows.
func (c *Cache) IsPopulated() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return false
	}

	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM tables").Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// GetDatasets returns summary info for all cached datasets.
func (c *Cache) GetDatasets() ([]DatasetInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("cache not open")
	}

	rows, err := c.db.Query(`
		SELECT d.project_id, d.dataset_id, COALESCE(d.location, ''), COALESCE(d.description, ''),
		       COUNT(t.table_id), COALESCE(SUM(t.size_bytes), 0)
		FROM datasets d
		LEFT JOIN tables t ON d.project_id = t.project_id AND d.dataset_id = t.dataset_id
		GROUP BY d.project_id, d.dataset_id
		ORDER BY d.project_id, d.dataset_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DatasetInfo
	for rows.Next() {
		var d DatasetInfo
		if err := rows.Scan(&d.ProjectID, &d.DatasetID, &d.Location, &d.Description, &d.TableCount, &d.TotalBytes); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// GetTables returns summary info for all tables in a dataset within a project.
func (c *Cache) GetTables(projectID, datasetID string) ([]TableInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("cache not open")
	}

	rows, err := c.db.Query(`
		SELECT project_id, dataset_id, table_id, COALESCE(table_type, ''), COALESCE(description, ''),
		       COALESCE(row_count, 0), COALESCE(size_bytes, 0),
		       COALESCE(partition_field, ''), COALESCE(clustering_fields, '[]')
		FROM tables
		WHERE project_id = ? AND dataset_id = ?
		ORDER BY table_id
	`, projectID, datasetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TableInfo
	for rows.Next() {
		var t TableInfo
		var clusterJSON string
		if err := rows.Scan(&t.ProjectID, &t.DatasetID, &t.TableID, &t.TableType, &t.Description,
			&t.RowCount, &t.SizeBytes, &t.PartitionField, &clusterJSON); err != nil {
			return nil, err
		}
		if clusterJSON != "" && clusterJSON != "[]" {
			_ = json.Unmarshal([]byte(clusterJSON), &t.ClusteringFields)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// GetTableDetail returns full table metadata including columns.
func (c *Cache) GetTableDetail(projectID, datasetID, tableID string) (*TableDetail, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("cache not open")
	}

	// Fetch table row.
	var td TableDetail
	var clusterJSON string
	err := c.db.QueryRow(`
		SELECT project_id, dataset_id, table_id, COALESCE(table_type, ''), COALESCE(description, ''),
		       COALESCE(row_count, 0), COALESCE(size_bytes, 0),
		       COALESCE(partition_field, ''), COALESCE(clustering_fields, '[]')
		FROM tables
		WHERE project_id = ? AND dataset_id = ? AND table_id = ?
	`, projectID, datasetID, tableID).Scan(
		&td.ProjectID, &td.DatasetID, &td.TableID, &td.TableType, &td.Description,
		&td.RowCount, &td.SizeBytes, &td.PartitionField, &clusterJSON,
	)
	if err != nil {
		return nil, fmt.Errorf("get table: %w", err)
	}
	if clusterJSON != "" && clusterJSON != "[]" {
		_ = json.Unmarshal([]byte(clusterJSON), &td.ClusteringFields)
	}

	// Fetch columns.
	rows, err := c.db.Query(`
		SELECT column_name, data_type, COALESCE(is_nullable, 0), COALESCE(description, ''),
		       COALESCE(ordinal_position, 0), COALESCE(is_partitioning, 0), COALESCE(clustering_ordinal, 0)
		FROM columns
		WHERE project_id = ? AND dataset_id = ? AND table_id = ?
		ORDER BY ordinal_position
	`, projectID, datasetID, tableID)
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var col ColumnInfo
		var nullable, partitioning int
		if err := rows.Scan(&col.Name, &col.DataType, &nullable, &col.Description,
			&col.OrdinalPosition, &partitioning, &col.ClusteringOrdinal); err != nil {
			return nil, err
		}
		col.IsNullable = nullable != 0
		col.IsPartitioning = partitioning != 0
		td.Columns = append(td.Columns, col)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &td, nil
}

// InvalidateTable removes a table and its columns from the cache and FTS index.
func (c *Cache) InvalidateTable(projectID, datasetID, tableID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.db == nil {
		return fmt.Errorf("cache not open")
	}

	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		"DELETE FROM schema_fts WHERE project_id = ? AND dataset_id = ? AND table_id = ?",
		projectID, datasetID, tableID,
	); err != nil {
		return fmt.Errorf("delete fts: %w", err)
	}

	if _, err := tx.Exec("DELETE FROM columns WHERE project_id = ? AND dataset_id = ? AND table_id = ?", projectID, datasetID, tableID); err != nil {
		return err
	}

	if _, err := tx.Exec("DELETE FROM tables WHERE project_id = ? AND dataset_id = ? AND table_id = ?", projectID, datasetID, tableID); err != nil {
		return err
	}

	return tx.Commit()
}
