package bigquery

import (
	"testing"
)

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
