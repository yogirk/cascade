package bigquery

import (
	"testing"
)

func TestCostToolName(t *testing.T) {
	ct := &CostTool{}
	if got := ct.Name(); got != "bigquery_cost" {
		t.Errorf("Name() = %q, want %q", got, "bigquery_cost")
	}
}

func TestCostToolRiskLevel(t *testing.T) {
	ct := &CostTool{}
	// Should be RiskReadOnly (0).
	if got := ct.RiskLevel(); got != 0 {
		t.Errorf("RiskLevel() = %d, want 0 (RiskReadOnly)", got)
	}
}

func TestCostToolInputSchema(t *testing.T) {
	ct := &CostTool{}
	s := ct.InputSchema()

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
