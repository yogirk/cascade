package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/internal/schema"
	"github.com/slokam-ai/cascade/internal/tools"
)

type schemaInput struct {
	Action  string `json:"action"`
	Dataset string `json:"dataset"`
	Table   string `json:"table"`
	Query   string `json:"query"`
}

// SchemaTool explores BigQuery schema: datasets, tables, columns, and search.
type SchemaTool struct {
	cache     *schema.Cache
	projectID string
}

// NewSchemaTool creates a new BigQuery schema tool.
func NewSchemaTool(cache *schema.Cache, projectID string) *SchemaTool {
	return &SchemaTool{
		cache:     cache,
		projectID: projectID,
	}
}

func (t *SchemaTool) Name() string { return "bigquery_schema" }

func (t *SchemaTool) Description() string {
	return "Explore BigQuery schema: list datasets, list tables in a dataset, describe a table's columns/partitioning/clustering, or search for columns by name across all cached tables."
}

func (t *SchemaTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list_datasets", "list_tables", "describe_table", "search_columns"},
				"description": "The schema action to perform",
			},
			"dataset": map[string]any{
				"type":        "string",
				"description": "Dataset ID (required for list_tables and describe_table)",
			},
			"table": map[string]any{
				"type":        "string",
				"description": "Table ID (required for describe_table)",
			},
			"query": map[string]any{
				"type":        "string",
				"description": "Search query for search_columns action",
			},
		},
		"required": []string{"action"},
	}
}

// RiskLevel returns RiskReadOnly since schema exploration is always read-only.
func (t *SchemaTool) RiskLevel() permission.RiskLevel {
	return permission.RiskReadOnly
}

func (t *SchemaTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var params schemaInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	// Check if cache is populated.
	if t.cache == nil || !t.cache.IsPopulated() {
		msg := "No schema cache found. Ask a question about your data or run /sync to build the cache."
		return &tools.Result{Content: msg, Display: msg}, nil
	}

	switch params.Action {
	case "list_datasets":
		return t.listDatasets()
	case "list_tables":
		return t.listTables(params.Dataset)
	case "describe_table":
		return t.describeTable(params.Dataset, params.Table)
	case "search_columns":
		return t.searchColumns(params.Query)
	default:
		msg := fmt.Sprintf("unknown action %q: must be one of list_datasets, list_tables, describe_table, search_columns", params.Action)
		return &tools.Result{Content: msg, IsError: true}, nil
	}
}

func (t *SchemaTool) listDatasets() (*tools.Result, error) {
	datasets, err := t.cache.GetDatasets()
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("Failed to list datasets: %v", err), IsError: true}, nil
	}

	display, content := RenderDatasetList(datasets, t.projectID)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *SchemaTool) listTables(dataset string) (*tools.Result, error) {
	if strings.TrimSpace(dataset) == "" {
		return &tools.Result{Content: "dataset parameter is required for list_tables action", IsError: true}, nil
	}

	tables, err := t.cache.GetTables(t.projectID, dataset)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("Failed to list tables: %v", err), IsError: true}, nil
	}

	display, content := RenderTableList(tables, dataset)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *SchemaTool) describeTable(dataset, table string) (*tools.Result, error) {
	if strings.TrimSpace(dataset) == "" {
		return &tools.Result{Content: "dataset parameter is required for describe_table action", IsError: true}, nil
	}
	if strings.TrimSpace(table) == "" {
		return &tools.Result{Content: "table parameter is required for describe_table action", IsError: true}, nil
	}

	detail, err := t.cache.GetTableDetail(t.projectID, dataset, table)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("Failed to describe table: %v", err), IsError: true}, nil
	}

	display, content := RenderTableDetail(detail)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *SchemaTool) searchColumns(query string) (*tools.Result, error) {
	if strings.TrimSpace(query) == "" {
		return &tools.Result{Content: "query parameter is required for search_columns action", IsError: true}, nil
	}

	results, err := t.cache.SearchColumns(query, 20)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("Failed to search columns: %v", err), IsError: true}, nil
	}

	display, content := RenderColumnSearch(results, query)
	return &tools.Result{Content: content, Display: display}, nil
}
