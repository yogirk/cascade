package schema

import "fmt"

// TableRef represents a table reference returned from FTS5 search.
type TableRef struct {
	ProjectID string
	DatasetID string
	TableID   string
	Rank      float64
}

// ColumnSearchResult holds a column match from FTS5 search with full metadata.
type ColumnSearchResult struct {
	ProjectID   string
	DatasetID   string
	TableID     string
	ColumnName  string
	DataType    string
	Description string
}

// Search performs an FTS5 search and returns ranked table references.
// Results are grouped by table and ordered by BM25 rank (lower = better match).
// Default limit is 20 if limit <= 0.
func (c *Cache) Search(query string, limit int) ([]TableRef, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("cache not open")
	}

	if query == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = 20
	}

	rows, err := c.db.Query(`
		SELECT project_id, dataset_id, table_id, MIN(rank) AS best_rank
		FROM schema_fts
		WHERE schema_fts MATCH ?
		GROUP BY project_id, dataset_id, table_id
		ORDER BY best_rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []TableRef
	for rows.Next() {
		var ref TableRef
		if err := rows.Scan(&ref.ProjectID, &ref.DatasetID, &ref.TableID, &ref.Rank); err != nil {
			return nil, err
		}
		results = append(results, ref)
	}
	return results, rows.Err()
}

// SearchColumns performs an FTS5 search and returns matching columns with full metadata.
// Results are joined back to the columns table for complete information.
func (c *Cache) SearchColumns(query string, limit int) ([]ColumnSearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.db == nil {
		return nil, fmt.Errorf("cache not open")
	}

	if query == "" {
		return nil, nil
	}

	if limit <= 0 {
		limit = 20
	}

	rows, err := c.db.Query(`
		SELECT c.project_id, c.dataset_id, c.table_id, c.column_name,
		       COALESCE(c.data_type, ''), COALESCE(c.description, '')
		FROM schema_fts f
		JOIN columns c
		    ON f.project_id = c.project_id AND f.dataset_id = c.dataset_id
		    AND f.table_id = c.table_id AND f.column_name = c.column_name
		WHERE schema_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts column search: %w", err)
	}
	defer rows.Close()

	var results []ColumnSearchResult
	for rows.Next() {
		var r ColumnSearchResult
		if err := rows.Scan(&r.ProjectID, &r.DatasetID, &r.TableID, &r.ColumnName, &r.DataType, &r.Description); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
