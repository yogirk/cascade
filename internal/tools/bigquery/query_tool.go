package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	gcbq "cloud.google.com/go/bigquery"
	bq "github.com/yogirk/cascade/internal/bigquery"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/schema"
	"github.com/yogirk/cascade/internal/tools"
	"github.com/yogirk/cascade/pkg/types"
)

type queryInput struct {
	SQL    string `json:"sql"`
	DryRun bool   `json:"dry_run"`
}

type queryExecutor interface {
	EstimateCost(ctx context.Context, sql string) (bytesProcessed int64, estimatedCost float64, err error)
	ExecuteQuery(ctx context.Context, sql string, maxRows int) (headers []string, rows [][]string, totalRows uint64, schema gcbq.Schema, err error)
}

// QueryTool executes BigQuery SQL queries with cost estimation and result rendering.
type QueryTool struct {
	client      queryExecutor
	cache       *schema.Cache
	projectID   string
	costTracker *bq.CostTracker
	costConfig  *config.CostConfig
	events      chan types.Event
}

// NewQueryTool creates a new BigQuery query tool.
func NewQueryTool(client *bq.Client, cache *schema.Cache, projectID string, costTracker *bq.CostTracker, costConfig *config.CostConfig, events chan types.Event) *QueryTool {
	return &QueryTool{
		client:      client,
		cache:       cache,
		projectID:   projectID,
		costTracker: costTracker,
		costConfig:  costConfig,
		events:      events,
	}
}

func (t *QueryTool) Name() string { return "bigquery_query" }

func (t *QueryTool) Description() string {
	return "Execute a BigQuery SQL query or estimate its cost. Set dry_run=true to only see the estimated cost without executing. When executing, cost is automatically estimated first and shown in the results footer. Queries exceeding the cost threshold require confirmation."
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

// PlanPermission classifies SQL risk and applies cost-aware gating.
// First classifies the SQL statement to determine the real risk (handling CTEs etc.),
// then for read-only queries checks cost thresholds.
//
// Every return path for parsed input MUST set a RiskOverride. The tool's
// base RiskLevel() is RiskDestructive — a conservative stub for the worst
// case. Without an explicit downgrade here, a plain SELECT would surface in
// the confirm dialog as [DESTRUCTIVE] (and trigger a confirm at all in modes
// where read-only auto-allows).
func (t *QueryTool) PlanPermission(ctx context.Context, input json.RawMessage, baseRisk permission.RiskLevel) (*tools.PermissionPlan, error) {
	var params queryInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, nil // unparseable input — fall back to base risk
	}

	// Classify the actual SQL risk (handles CTE-wrapped DML, etc.).
	sqlRisk := baseRisk
	if params.SQL != "" {
		sqlRisk = bq.ClassifySQLRisk(params.SQL)
	}

	// SQL escalates above read-only (DML/DDL/destructive/admin) — surface
	// the classified risk directly.
	if sqlRisk > permission.RiskReadOnly {
		return &tools.PermissionPlan{RiskOverride: &sqlRisk}, nil
	}

	// Read-only path. Cost gating may further escalate to DML or deny, but
	// the default outcome must be a RiskReadOnly downgrade.
	readOnly := permission.RiskReadOnly

	if t.client == nil || t.costConfig == nil {
		return &tools.PermissionPlan{RiskOverride: &readOnly}, nil
	}
	if params.DryRun || strings.TrimSpace(params.SQL) == "" {
		return &tools.PermissionPlan{RiskOverride: &readOnly}, nil
	}

	_, estimatedCost, err := t.client.EstimateCost(ctx, params.SQL)
	if err != nil || estimatedCost < 0 {
		return &tools.PermissionPlan{RiskOverride: &readOnly}, nil
	}

	if t.costConfig.MaxQueryCost > 0 && estimatedCost > t.costConfig.MaxQueryCost {
		return &tools.PermissionPlan{
			DenyMessage: fmt.Sprintf(
				"Query blocked: estimated cost %s exceeds maximum %s. Add partition filters or increase cost.max_query_cost in config.",
				FormatCost(estimatedCost), FormatCost(t.costConfig.MaxQueryCost),
			),
		}, nil
	}

	if t.costConfig.WarnThreshold > 0 && estimatedCost >= t.costConfig.WarnThreshold {
		risk := permission.RiskDML
		return &tools.PermissionPlan{RiskOverride: &risk}, nil
	}

	return &tools.PermissionPlan{RiskOverride: &readOnly}, nil
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
			msg := fmt.Sprintf("Query blocked: estimated cost %s exceeds maximum %s. Add partition filters or increase cost.max_query_cost in config.",
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

		// Emit cost update event for TUI status bar.
		if t.events != nil {
			t.events <- &types.CostUpdateEvent{
				QueryCost:    estimatedCost,
				SessionTotal: t.costTracker.SessionTotal(),
				BytesScanned: bytesProcessed,
			}
		}
	}

	// Step 5: Render results.
	display, content := RenderQueryResults(headers, rows, totalRows, maxRows, estimatedCost, durationMs, bytesProcessed)

	// Step 5b: Analyze query for optimization suggestions.
	if t.cache != nil {
		adapter := &cacheSchemaAdapter{cache: t.cache, projectID: t.projectID}
		if optHints := bq.AnalyzeQuery(params.SQL, adapter); len(optHints) > 0 {
			hintDisplay, hintContent := RenderOptimizationHints(optHints)
			display += hintDisplay
			content += hintContent
		}
	}

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
		_ = t.cache.InvalidateTable(t.projectID, datasetID, tableID)
	}
}

// cacheSchemaAdapter adapts schema.Cache to bq.SchemaLookup.
// This bridges the schema and bigquery packages without creating a circular import.
type cacheSchemaAdapter struct {
	cache     *schema.Cache
	projectID string
}

func (a *cacheSchemaAdapter) GetTableMeta(datasetID, tableID string) (*bq.TableMeta, error) {
	detail, err := a.cache.GetTableDetail(a.projectID, datasetID, tableID)
	if err != nil {
		return nil, err
	}
	return &bq.TableMeta{
		DatasetID:        detail.DatasetID,
		TableID:          detail.TableID,
		SizeBytes:        detail.SizeBytes,
		PartitionField:   detail.PartitionField,
		ClusteringFields: detail.ClusteringFields,
	}, nil
}

func (a *cacheSchemaAdapter) ListTableMeta(datasetID string) ([]bq.TableMeta, error) {
	tables, err := a.cache.GetTables(a.projectID, datasetID)
	if err != nil {
		return nil, err
	}
	metas := make([]bq.TableMeta, len(tables))
	for i, t := range tables {
		metas[i] = bq.TableMeta{
			DatasetID:        t.DatasetID,
			TableID:          t.TableID,
			SizeBytes:        t.SizeBytes,
			PartitionField:   t.PartitionField,
			ClusteringFields: t.ClusteringFields,
		}
	}
	return metas, nil
}
