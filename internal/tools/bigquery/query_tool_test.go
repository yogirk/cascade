package bigquery

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	gcbq "cloud.google.com/go/bigquery"

	bq "github.com/slokam-ai/cascade/internal/bigquery"
	"github.com/slokam-ai/cascade/internal/config"
	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/pkg/types"
)

type mockQueryClient struct {
	bytes int64
	cost  float64
	err   error
}

func (m *mockQueryClient) EstimateCost(context.Context, string) (int64, float64, error) {
	return m.bytes, m.cost, m.err
}

func (m *mockQueryClient) ExecuteQuery(context.Context, string, int) ([]string, [][]string, uint64, gcbq.Schema, error) {
	return []string{"id"}, [][]string{{"1"}}, 1, nil, nil
}

func TestQueryToolName(t *testing.T) {
	qt := &QueryTool{}
	if got := qt.Name(); got != "bigquery_query" {
		t.Errorf("Name() = %q, want %q", got, "bigquery_query")
	}
}

func TestQueryToolInputSchema(t *testing.T) {
	qt := &QueryTool{}
	s := qt.InputSchema()

	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatal("InputSchema missing properties")
	}

	if _, ok := props["sql"]; !ok {
		t.Error("InputSchema missing 'sql' property")
	}

	req, ok := s["required"].([]string)
	if !ok {
		t.Fatal("InputSchema missing required")
	}

	found := false
	for _, r := range req {
		if r == "sql" {
			found = true
		}
	}
	if !found {
		t.Error("InputSchema: 'sql' not in required")
	}
}

func TestQueryToolRiskLevel(t *testing.T) {
	qt := &QueryTool{}
	// Should be RiskDestructive (worst case default).
	if got := qt.RiskLevel(); got != 3 { // permission.RiskDestructive == 3
		t.Errorf("RiskLevel() = %d, want 3 (RiskDestructive)", got)
	}
}

func TestRenderQueryResults(t *testing.T) {
	headers := []string{"id", "name", "value"}
	rows := [][]string{
		{"1", "Alice", "100.50"},
		{"2", "Bob", "200.75"},
		{"3", "Charlie", "NULL"},
	}

	display, content := RenderQueryResults(headers, rows, 3, 50, 0.43, 2100, 2_100_000_000)

	// Display should contain the headers.
	for _, h := range headers {
		if !containsStr(display, h) {
			t.Errorf("display missing header %q", h)
		}
	}

	// Display should contain the cost footer.
	if !containsStr(display, "$0.43") {
		t.Error("display missing cost in footer")
	}
	if !containsStr(display, "2.1s") {
		t.Error("display missing duration in footer")
	}
	if !containsStr(display, "2.0 GB") {
		t.Error("display missing bytes in footer")
	}

	// Content should have tab-separated plain text.
	if !containsStr(content, "id\tname\tvalue") {
		t.Error("content missing tab-separated headers")
	}
	if !containsStr(content, "Alice") {
		t.Error("content missing row data")
	}
}

func TestRenderQueryResultsTruncation(t *testing.T) {
	headers := []string{"id", "value"}
	rows := make([][]string, 100)
	for i := range rows {
		rows[i] = []string{
			formatRowCount(int64(i + 1)),
			"data",
		}
	}

	display, _ := RenderQueryResults(headers, rows, 100, 50, 0.01, 500, 1_000_000)

	if !containsStr(display, "50 more rows") {
		t.Error("display should indicate 50 more rows for 100 total with 50 max display")
	}
}

func TestRenderQueryResultsEmpty(t *testing.T) {
	display, content := RenderQueryResults([]string{}, nil, 0, 50, 0, 0, 0)
	if display != "Query returned no columns." {
		t.Errorf("unexpected display for empty result: %q", display)
	}
	if content != "Query returned no columns." {
		t.Errorf("unexpected content for empty result: %q", content)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1536, "1.5 KB"},
		{5_242_880, "5.0 MB"},
		{2_147_483_648, "2.0 GB"},
		{1_099_511_627_776, "1.0 TB"},
	}

	for _, tc := range tests {
		got := FormatBytes(tc.bytes)
		if got != tc.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0.001, "$0.00"},
		{0.43, "$0.43"},
		{52.90, "$52.90"},
		{-1, "N/A (DML)"},
	}

	for _, tc := range tests {
		got := FormatCost(tc.cost)
		if got != tc.want {
			t.Errorf("FormatCost(%f) = %q, want %q", tc.cost, got, tc.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{500, "500ms"},
		{2100, "2.1s"},
		{65000, "1m 5s"},
	}

	for _, tc := range tests {
		got := FormatDuration(tc.ms)
		if got != tc.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

func TestTableRefRegex(t *testing.T) {
	tests := []struct {
		sql     string
		dataset string
		table   string
	}{
		{"DROP TABLE `project.warehouse.tmp_staging`", "warehouse", "tmp_staging"},
		{"CREATE TABLE dataset.newtable AS SELECT 1", "dataset", "newtable"},
		{"ALTER TABLE IF EXISTS myds.mytable ADD COLUMN x INT64", "myds", "mytable"},
	}

	for _, tc := range tests {
		matches := tableRefRegex.FindStringSubmatch(tc.sql)
		if len(matches) < 3 {
			t.Errorf("regex did not match SQL: %q", tc.sql)
			continue
		}
		if matches[1] != tc.dataset {
			t.Errorf("dataset = %q, want %q for SQL: %q", matches[1], tc.dataset, tc.sql)
		}
		if matches[2] != tc.table {
			t.Errorf("table = %q, want %q for SQL: %q", matches[2], tc.table, tc.sql)
		}
	}
}

func TestQueryToolPlanPermission_DowngradesReadOnlyWithoutCostConfig(t *testing.T) {
	// Reproduces the "[DESTRUCTIVE] bigquery_query wants to execute: SELECT ..."
	// regression: when no cost config is supplied, a SELECT must still
	// downgrade to RiskReadOnly. Otherwise the base RiskDestructive bleeds
	// through and a plain read is gated as a destructive operation.
	qt := &QueryTool{} // no client, no costConfig

	plan, err := qt.PlanPermission(
		context.Background(),
		json.RawMessage(`{"sql":"SELECT * FROM analytics.orders"}`),
		permission.RiskDestructive, // base risk passed by the agent loop
	)
	if err != nil {
		t.Fatalf("PlanPermission: %v", err)
	}
	if plan == nil || plan.RiskOverride == nil {
		t.Fatalf("expected read-only downgrade, got %#v", plan)
	}
	if *plan.RiskOverride != permission.RiskReadOnly {
		t.Fatalf("RiskOverride = %v, want RiskReadOnly", *plan.RiskOverride)
	}
}

func TestQueryToolPlanPermission_DowngradesReadOnlyBelowWarnThreshold(t *testing.T) {
	qt := &QueryTool{
		client: &mockQueryClient{bytes: 100_000, cost: 0.0001},
		costConfig: &config.CostConfig{
			WarnThreshold: 1.0,
			MaxQueryCost:  10.0,
		},
	}

	plan, err := qt.PlanPermission(
		context.Background(),
		json.RawMessage(`{"sql":"SELECT * FROM analytics.orders"}`),
		permission.RiskDestructive,
	)
	if err != nil {
		t.Fatalf("PlanPermission: %v", err)
	}
	if plan == nil || plan.RiskOverride == nil || *plan.RiskOverride != permission.RiskReadOnly {
		t.Fatalf("cheap SELECT must downgrade to RiskReadOnly, got %#v", plan)
	}
}

func TestQueryToolPlanPermission_DowngradesReadOnlyOnEstimateError(t *testing.T) {
	qt := &QueryTool{
		client: &mockQueryClient{err: fmt.Errorf("estimate failed")},
		costConfig: &config.CostConfig{
			WarnThreshold: 1.0,
			MaxQueryCost:  10.0,
		},
	}

	plan, err := qt.PlanPermission(
		context.Background(),
		json.RawMessage(`{"sql":"SELECT * FROM analytics.orders"}`),
		permission.RiskDestructive,
	)
	if err != nil {
		t.Fatalf("PlanPermission: %v", err)
	}
	if plan == nil || plan.RiskOverride == nil || *plan.RiskOverride != permission.RiskReadOnly {
		t.Fatalf("estimate-error path must still downgrade to RiskReadOnly, got %#v", plan)
	}
}

func TestQueryToolPlanPermission_EscalatesWarnThreshold(t *testing.T) {
	qt := &QueryTool{
		client: &mockQueryClient{bytes: 2_000_000_000, cost: 2.50},
		costConfig: &config.CostConfig{
			WarnThreshold: 1.0,
			MaxQueryCost:  10.0,
		},
	}

	plan, err := qt.PlanPermission(context.Background(), json.RawMessage(`{"sql":"SELECT * FROM analytics.orders"}`), permission.RiskReadOnly)
	if err != nil {
		t.Fatalf("PlanPermission: %v", err)
	}
	if plan == nil || plan.RiskOverride == nil || *plan.RiskOverride != permission.RiskDML {
		t.Fatalf("expected warn-threshold query to escalate approval, got %#v", plan)
	}
}

func TestQueryToolPlanPermission_BlocksAboveMax(t *testing.T) {
	qt := &QueryTool{
		client: &mockQueryClient{bytes: 20_000_000_000, cost: 12.00},
		costConfig: &config.CostConfig{
			WarnThreshold: 1.0,
			MaxQueryCost:  10.0,
		},
	}

	plan, err := qt.PlanPermission(context.Background(), json.RawMessage(`{"sql":"SELECT * FROM analytics.orders"}`), permission.RiskReadOnly)
	if err != nil {
		t.Fatalf("PlanPermission: %v", err)
	}
	if plan == nil || plan.DenyMessage == "" {
		t.Fatalf("expected deny plan for over-budget query, got %#v", plan)
	}
	if !containsStr(plan.DenyMessage, "cost.max_query_cost") {
		t.Fatalf("expected deny message to reference config key, got %q", plan.DenyMessage)
	}
}

func TestQueryToolExecute_BlockedMessageUsesActualConfigKey(t *testing.T) {
	qt := &QueryTool{
		client: &mockQueryClient{bytes: 20_000_000_000, cost: 12.00},
		costConfig: &config.CostConfig{
			MaxQueryCost: 10.0,
		},
	}

	result, err := qt.Execute(context.Background(), json.RawMessage(`{"sql":"SELECT * FROM analytics.orders"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected blocked query to return error result")
	}
	if !containsStr(result.Content, "cost.max_query_cost") {
		t.Fatalf("expected blocked query message to reference actual config key, got %q", result.Content)
	}
}

func TestQueryToolExecute_EmitsCostUpdateEvent(t *testing.T) {
	events := make(chan types.Event, 1)
	qt := &QueryTool{
		client:      &mockQueryClient{bytes: 2_000_000_000, cost: 2.50},
		costTracker: bq.NewCostTracker(100),
		costConfig:  &config.CostConfig{MaxDisplayRows: 50},
		events:      events,
	}

	result, err := qt.Execute(context.Background(), json.RawMessage(`{"sql":"SELECT * FROM analytics.orders"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.IsError {
		t.Fatalf("unexpected error result: %s", result.Content)
	}

	select {
	case evt := <-events:
		if _, ok := evt.(*types.CostUpdateEvent); !ok {
			t.Fatalf("expected CostUpdateEvent, got %T", evt)
		}
	default:
		t.Fatal("expected cost update event to be emitted")
	}
}

// containsStr is a helper that checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findStr(s, substr))
}

func findStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
