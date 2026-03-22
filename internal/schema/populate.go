package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RowIterator abstracts BigQuery row iteration for testability.
type RowIterator interface {
	Next(dst interface{}) error
}

// BQQuerier abstracts the BigQuery client for schema population.
type BQQuerier interface {
	RunQuery(ctx context.Context, sql string) (RowIterator, error)
	ProjectID() string
}

// ProgressFunc is called during population with completed and total counts.
type ProgressFunc func(completed, total int)

// Populator populates the schema cache from BigQuery INFORMATION_SCHEMA.
type Populator struct {
	cache *Cache
	bq    BQQuerier
}

// NewPopulator creates a new populator for the given cache and BQ client.
func NewPopulator(cache *Cache, bq BQQuerier) *Populator {
	return &Populator{cache: cache, bq: bq}
}

// PopulateAll populates the cache for all given datasets.
func (p *Populator) PopulateAll(ctx context.Context, datasets []string, progress ProgressFunc) error {
	totalTables := 0
	completedTables := 0

	for _, ds := range datasets {
		if err := p.PopulateDataset(ctx, ds, func(completed, total int) {
			completedTables += completed
			totalTables = completedTables + total - completed
			if progress != nil {
				progress(completedTables, totalTables)
			}
		}); err != nil {
			return fmt.Errorf("populate dataset %s: %w", ds, err)
		}
	}
	return nil
}

// PopulateDataset populates the cache for a single dataset from INFORMATION_SCHEMA.
func (p *Populator) PopulateDataset(ctx context.Context, datasetID string, progress ProgressFunc) error {
	db := p.cache.DB()
	if db == nil {
		return fmt.Errorf("cache not open")
	}

	projectID := p.bq.ProjectID()
	now := time.Now().Unix()

	// Step 1: Upsert dataset record.
	if _, err := db.Exec(`
		INSERT INTO datasets (dataset_id, project_id, location, description, labels, last_refreshed)
		VALUES (?, ?, '', '', '', ?)
		ON CONFLICT(dataset_id) DO UPDATE SET last_refreshed = ?
	`, datasetID, projectID, now, now); err != nil {
		return fmt.Errorf("upsert dataset: %w", err)
	}

	// Step 2: Fetch table metadata from INFORMATION_SCHEMA.TABLES.
	tablesSQL := fmt.Sprintf(`
		SELECT t.table_name, t.table_type,
		       COALESCE(opt.option_value, '') AS description
		FROM `+"`%s.%s.INFORMATION_SCHEMA.TABLES`"+` t
		LEFT JOIN `+"`%s.%s.INFORMATION_SCHEMA.TABLE_OPTIONS`"+` opt
		    ON t.table_name = opt.table_name AND opt.option_name = 'description'
	`, projectID, datasetID, projectID, datasetID)

	type tableRow struct {
		Name        string
		Type        string
		Description string
	}

	// We'll collect table names for progress tracking.
	var tableRows []tableRow
	if err := p.queryRows(ctx, tablesSQL, func(row []interface{}) {
		tableRows = append(tableRows, tableRow{
			Name:        valString(row, 0),
			Type:        valString(row, 1),
			Description: valString(row, 2),
		})
	}); err != nil {
		return fmt.Errorf("fetch tables: %w", err)
	}

	totalTables := len(tableRows)
	if progress != nil {
		progress(0, totalTables)
	}

	// Step 3: Fetch column metadata from INFORMATION_SCHEMA.COLUMNS.
	columnsSQL := fmt.Sprintf(`
		SELECT c.table_name, c.column_name, c.ordinal_position, c.is_nullable, c.data_type,
		       COALESCE(cfp.description, '') AS column_description,
		       COALESCE(c.is_partitioning_column, 'NO') AS is_partitioning_column,
		       COALESCE(CAST(c.clustering_ordinal_position AS STRING), '') AS clustering_ordinal_position
		FROM `+"`%s.%s.INFORMATION_SCHEMA.COLUMNS`"+` c
		LEFT JOIN `+"`%s.%s.INFORMATION_SCHEMA.COLUMN_FIELD_PATHS`"+` cfp
		    ON c.table_name = cfp.table_name
		    AND c.column_name = cfp.column_name
		    AND cfp.column_name = cfp.field_path
		ORDER BY c.table_name, c.ordinal_position
	`, projectID, datasetID, projectID, datasetID)

	type columnRow struct {
		TableName         string
		ColumnName        string
		OrdinalPosition   int64
		IsNullable        string
		DataType          string
		Description       string
		IsPartitioning    string
		ClusteringOrdinal string
	}

	var columnRows []columnRow
	if err := p.queryRows(ctx, columnsSQL, func(row []interface{}) {
		columnRows = append(columnRows, columnRow{
			TableName:         valString(row, 0),
			ColumnName:        valString(row, 1),
			OrdinalPosition:   valInt64(row, 2),
			IsNullable:        valString(row, 3),
			DataType:          valString(row, 4),
			Description:       valString(row, 5),
			IsPartitioning:    valString(row, 6),
			ClusteringOrdinal: valString(row, 7),
		})
	}); err != nil {
		return fmt.Errorf("fetch columns: %w", err)
	}

	// Step 4: Optionally fetch storage stats.
	type storageRow struct {
		TableName  string
		TotalRows  int64
		TotalBytes int64
	}
	// Try TABLE_STORAGE first, fall back to __TABLES__ (universally available).
	var storageErr error
	storageSQL := fmt.Sprintf(`
		SELECT table_name, total_rows, total_logical_bytes
		FROM `+"`%s.%s.INFORMATION_SCHEMA.TABLE_STORAGE`"+`
	`, projectID, datasetID)

	storageMap := make(map[string]storageRow)
	storageErr = p.queryRows(ctx, storageSQL, func(row []interface{}) {
		r := storageRow{
			TableName:  valString(row, 0),
			TotalRows:  valInt64(row, 1),
			TotalBytes: valInt64(row, 2),
		}
		storageMap[r.TableName] = r
	})
	if storageErr != nil || len(storageMap) == 0 {
		// Fallback: __TABLES__ is available in all projects/datasets.
		fallbackSQL := fmt.Sprintf(`
			SELECT table_id, row_count, size_bytes
			FROM `+"`%s.%s.__TABLES__`"+`
		`, projectID, datasetID)
		_ = p.queryRows(ctx, fallbackSQL, func(row []interface{}) {
			r := storageRow{
				TableName:  valString(row, 0),
				TotalRows:  valInt64(row, 1),
				TotalBytes: valInt64(row, 2),
			}
			storageMap[r.TableName] = r
		})
	}

	// Step 5: Insert everything into the cache within a transaction.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Group columns by table for clustering field detection.
	columnsByTable := make(map[string][]columnRow)
	for _, col := range columnRows {
		columnsByTable[col.TableName] = append(columnsByTable[col.TableName], col)
	}

	for i, tbl := range tableRows {
		storage, hasStorage := storageMap[tbl.Name]
		var rowCount, sizeBytes int64
		if hasStorage {
			rowCount = storage.TotalRows
			sizeBytes = storage.TotalBytes
		}

		// Determine partition and clustering from columns.
		var partitionField string
		var clusteringFields []string
		if cols, ok := columnsByTable[tbl.Name]; ok {
			for _, col := range cols {
				if col.IsPartitioning == "YES" {
					partitionField = col.ColumnName
				}
				if col.ClusteringOrdinal != "" && col.ClusteringOrdinal != "0" {
					clusteringFields = append(clusteringFields, col.ColumnName)
				}
			}
		}

		clusterJSON, _ := json.Marshal(clusteringFields)
		if clusteringFields == nil {
			clusterJSON = []byte("[]")
		}

		if _, err := tx.Exec(`
			INSERT INTO tables (dataset_id, table_id, table_type, description, row_count, size_bytes,
			                    partition_field, partition_type, clustering_fields, labels, last_modified, last_refreshed)
			VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, '', 0, ?)
			ON CONFLICT(dataset_id, table_id) DO UPDATE SET
			    table_type=excluded.table_type, description=excluded.description,
			    row_count=excluded.row_count, size_bytes=excluded.size_bytes,
			    partition_field=excluded.partition_field, clustering_fields=excluded.clustering_fields,
			    last_refreshed=excluded.last_refreshed
		`, datasetID, tbl.Name, tbl.Type, tbl.Description, rowCount, sizeBytes,
			partitionField, string(clusterJSON), now); err != nil {
			return fmt.Errorf("upsert table %s: %w", tbl.Name, err)
		}

		// Insert columns for this table.
		if cols, ok := columnsByTable[tbl.Name]; ok {
			for _, col := range cols {
				nullable := 0
				if col.IsNullable == "YES" {
					nullable = 1
				}
				partitioning := 0
				if col.IsPartitioning == "YES" {
					partitioning = 1
				}
				var clusterOrd int
				if col.ClusteringOrdinal != "" {
					fmt.Sscanf(col.ClusteringOrdinal, "%d", &clusterOrd)
				}

				if _, err := tx.Exec(`
					INSERT INTO columns (dataset_id, table_id, column_name, data_type, is_nullable,
					                     description, ordinal_position, is_partitioning, clustering_ordinal)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(dataset_id, table_id, column_name) DO UPDATE SET
					    data_type=excluded.data_type, is_nullable=excluded.is_nullable,
					    description=excluded.description, ordinal_position=excluded.ordinal_position,
					    is_partitioning=excluded.is_partitioning, clustering_ordinal=excluded.clustering_ordinal
				`, datasetID, col.TableName, col.ColumnName, col.DataType, nullable,
					col.Description, col.OrdinalPosition, partitioning, clusterOrd); err != nil {
					return fmt.Errorf("upsert column %s.%s: %w", col.TableName, col.ColumnName, err)
				}

				// Insert into FTS index.
				if _, err := tx.Exec(`
					INSERT INTO schema_fts (dataset_id, table_id, column_name, description)
					VALUES (?, ?, ?, ?)
				`, datasetID, col.TableName, col.ColumnName, col.Description); err != nil {
					return fmt.Errorf("fts insert %s.%s: %w", col.TableName, col.ColumnName, err)
				}
			}
		}

		if progress != nil {
			progress(i+1, totalTables)
		}
	}

	return tx.Commit()
}

// queryRows runs a BigQuery query and calls collectRow for each result row.
// BigQuery's RowIterator.Next fills a []interface{} ([]bigquery.Value) with
// the column values in query order. collectRow receives these values directly.
func (p *Populator) queryRows(ctx context.Context, query string, collectRow func([]interface{})) error {
	it, err := p.bq.RunQuery(ctx, query)
	if err != nil {
		return err
	}

	for {
		var row []interface{}
		if err := it.Next(&row); err != nil {
			if err.Error() == "no more items in iterator" || err.Error() == "iterator done" {
				break
			}
			return err
		}

		if collectRow != nil {
			collectRow(row)
		}
	}

	return nil
}

// valString safely extracts a string from a BigQuery row at the given index.
// BigQuery returns values as interface{} (bigquery.Value). Strings come as string,
// NULLs as nil.
func valString(row []interface{}, i int) string {
	if i >= len(row) || row[i] == nil {
		return ""
	}
	switch v := row[i].(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// valInt64 safely extracts an int64 from a BigQuery row at the given index.
// BigQuery returns integers as int64, NULLs as nil.
func valInt64(row []interface{}, i int) int64 {
	if i >= len(row) || row[i] == nil {
		return 0
	}
	switch v := row[i].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return 0
	}
}
