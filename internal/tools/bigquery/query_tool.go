package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	bq "github.com/yogirk/cascade/internal/bigquery"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/schema"
	"github.com/yogirk/cascade/internal/tools"
)

type queryInput struct {
	SQL    string `json:"sql"`
	DryRun bool   `json:"dry_run"`
}

// QueryTool executes BigQuery SQL queries with cost estimation and result rendering.
type QueryTool struct {
	client      *bq.Client
	cache       *schema.Cache
	costTracker *bq.CostTracker
	costConfig  *config.CostConfig
}

// NewQueryTool creates a new BigQuery query tool.
func NewQueryTool(client *bq.Client, cache *schema.Cache, costTracker *bq.CostTracker, costConfig *config.CostConfig) *QueryTool {
	return &QueryTool{
		client:      client,
		cache:       cache,
		costTracker: costTracker,
		costConfig:  costConfig,
	}
}

func (t *QueryTool) Name() string { return "bigquery_query" }

func (t *QueryTool) Description() string {
	return "Execute a BigQuery SQL query. Use this to run SELECT queries, DML statements, or DDL operations against BigQuery. The query will be cost-estimated before execution."
}

func (t *QueryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sql": map[string]any{
				"type":        "string",
				"description": "The SQL query to execute",
			},
			"dry_run": map[string]any{
				"type":        "boolean",
				"description": "If true, only estimate cost without executing",
			},
		},
		"required": []string{"sql"},
	}
}

// RiskLevel returns RiskDestructive as the worst case; the agent loop
// should call ClassifySQLRisk with the actual SQL for dynamic risk classification.
func (t *QueryTool) RiskLevel() permission.RiskLevel {
	return permission.RiskDestructive
}

func (t *QueryTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var params queryInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if strings.TrimSpace(params.SQL) == "" {
		return &tools.Result{Content: "sql parameter is required and cannot be empty", IsError: true}, nil
	}

	// Dry-run only: delegate to cost estimation.
	if params.DryRun {
		return t.dryRunOnly(ctx, params.SQL)
	}

	// Step 1: Run dry-run cost estimation.
	bytesProcessed, estimatedCost, err := t.client.EstimateCost(ctx, params.SQL)
	if err != nil {
		return &tools.Result{
			Content: fmt.Sprintf("Cost estimation failed: %v", err),
			IsError: true,
		}, nil
	}

	// Step 2: Check cost threshold (skip for DML where cost == -1).
	if estimatedCost >= 0 && t.costConfig != nil && t.costConfig.MaxQueryCost > 0 {
		if estimatedCost > t.costConfig.MaxQueryCost {
			msg := fmt.Sprintf("Query blocked: estimated cost %s exceeds maximum %s. Add partition filters or increase cost.max_query_cost_usd in config.",
				FormatCost(estimatedCost), FormatCost(t.costConfig.MaxQueryCost))
			return &tools.Result{Content: msg, Display: msg, IsError: true}, nil
		}
	}

	// Step 3: Execute query.
	maxRows := 50
	if t.costConfig != nil && t.costConfig.MaxDisplayRows > 0 {
		maxRows = t.costConfig.MaxDisplayRows
	}

	start := time.Now()
	headers, rows, totalRows, _, execErr := t.client.ExecuteQuery(ctx, params.SQL, maxRows)
	durationMs := time.Since(start).Milliseconds()

	if execErr != nil {
		return &tools.Result{
			Content: fmt.Sprintf("Query execution failed: %v", execErr),
			IsError: true,
		}, nil
	}

	// Step 4: Track cost.
	if t.costTracker != nil {
		t.costTracker.Record(bq.QueryCostEntry{
			SQL:          params.SQL,
			BytesScanned: bytesProcessed,
			Cost:         estimatedCost,
			DurationMs:   durationMs,
			IsDML:        estimatedCost < 0,
		})
	}

	// Step 5: Render results.
	display, content := RenderQueryResults(headers, rows, totalRows, maxRows, estimatedCost, durationMs, bytesProcessed)

	// Step 6: Auto-invalidate cache after DDL/destructive operations.
	risk := bq.ClassifySQLRisk(params.SQL)
	if risk == permission.RiskDDL || risk == permission.RiskDestructive {
		t.invalidateCacheForSQL(params.SQL)
	}

	return &tools.Result{Content: content, Display: display}, nil
}

// dryRunOnly performs only cost estimation without executing the query.
func (t *QueryTool) dryRunOnly(ctx context.Context, sql string) (*tools.Result, error) {
	bytesProcessed, estimatedCost, err := t.client.EstimateCost(ctx, sql)
	if err != nil {
		return &tools.Result{
			Content: fmt.Sprintf("Dry-run failed: %v", err),
			IsError: true,
		}, nil
	}

	var msg string
	if estimatedCost < 0 {
		msg = "Cost: cannot estimate for DML (syntax valid)"
	} else {
		msg = fmt.Sprintf("Estimated cost: %s (%s)", FormatCost(estimatedCost), FormatBytes(bytesProcessed))
	}

	return &tools.Result{Content: msg, Display: msg}, nil
}

// tableRefRegex matches dataset.table or `project.dataset.table` patterns in DDL.
var tableRefRegex = regexp.MustCompile(`(?i)(?:CREATE|DROP|ALTER|TRUNCATE)\s+(?:TABLE|VIEW)\s+(?:IF\s+(?:NOT\s+)?EXISTS\s+)?` + "`?" + `(?:[\w-]+\.)?(\w+)\.(\w+)` + "`?")

// invalidateCacheForSQL extracts the table reference from DDL SQL and invalidates the cache.
func (t *QueryTool) invalidateCacheForSQL(sql string) {
	if t.cache == nil {
		return
	}

	matches := tableRefRegex.FindStringSubmatch(sql)
	if len(matches) >= 3 {
		datasetID := matches[1]
		tableID := matches[2]
		_ = t.cache.InvalidateTable(datasetID, tableID)
	}
}
