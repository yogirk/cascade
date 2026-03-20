package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	bq "github.com/yogirk/cascade/internal/bigquery"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/tools"
)

type costInput struct {
	SQL string `json:"sql"`
}

// CostTool estimates the cost of a BigQuery SQL query without executing it.
type CostTool struct {
	client *bq.Client
}

// NewCostTool creates a new BigQuery cost estimation tool.
func NewCostTool(client *bq.Client) *CostTool {
	return &CostTool{client: client}
}

func (t *CostTool) Name() string { return "bigquery_cost" }

func (t *CostTool) Description() string {
	return "Estimate the cost of a BigQuery SQL query without executing it. Returns bytes to be scanned and estimated dollar cost based on on-demand pricing."
}

func (t *CostTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sql": map[string]any{
				"type":        "string",
				"description": "The SQL query to estimate cost for",
			},
		},
		"required": []string{"sql"},
	}
}

// RiskLevel returns RiskReadOnly since dry-run never executes.
func (t *CostTool) RiskLevel() permission.RiskLevel {
	return permission.RiskReadOnly
}

func (t *CostTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var params costInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if strings.TrimSpace(params.SQL) == "" {
		return &tools.Result{Content: "sql parameter is required and cannot be empty", IsError: true}, nil
	}

	bytesProcessed, estimatedCost, err := t.client.EstimateCost(ctx, params.SQL)
	if err != nil {
		return &tools.Result{
			Content: fmt.Sprintf("Cost estimation failed: %v", err),
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
