package bigquery

import (
	"strings"
	"testing"
)

func TestRenderSparkline_Normal(t *testing.T) {
	data := []float64{1, 3, 5, 7, 10, 8, 4}
	result := RenderSparkline(data)
	if result == "" {
		t.Error("sparkline should not be empty for valid data")
	}
	// Should contain 7 block characters (one per data point)
	// Strip ANSI to count runes
	plain := stripANSI(result)
	if len([]rune(plain)) != 7 {
		t.Errorf("expected 7 characters, got %d: %q", len([]rune(plain)), plain)
	}
}

func TestRenderSparkline_Empty(t *testing.T) {
	result := RenderSparkline(nil)
	if result != "" {
		t.Error("sparkline should be empty for nil data")
	}
	result = RenderSparkline([]float64{})
	if result != "" {
		t.Error("sparkline should be empty for empty data")
	}
}

func TestRenderSparkline_SinglePoint(t *testing.T) {
	result := RenderSparkline([]float64{42})
	if result == "" {
		t.Error("sparkline should render single point")
	}
}

func TestRenderSparkline_AllSame(t *testing.T) {
	result := RenderSparkline([]float64{5, 5, 5, 5})
	if result == "" {
		t.Error("sparkline should render equal values")
	}
	plain := stripANSI(result)
	if len([]rune(plain)) != 4 {
		t.Errorf("expected 4 characters, got %d", len([]rune(plain)))
	}
}

func TestRenderSparkline_Ascending(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	result := RenderSparkline(data)
	plain := stripANSI(result)
	runes := []rune(plain)
	// First char should be lowest block, last should be highest
	if runes[0] != '▁' {
		t.Errorf("first char should be ▁, got %c", runes[0])
	}
	if runes[len(runes)-1] != '█' {
		t.Errorf("last char should be █, got %c", runes[len(runes)-1])
	}
}

func TestRenderBarChart_Normal(t *testing.T) {
	items := []BarChartItem{
		{Label: "user@co.com", Value: 10.5, FormattedValue: "$10.50"},
		{Label: "admin@co.com", Value: 5.2, FormattedValue: "$5.20"},
		{Label: "etl@co.com", Value: 2.1, FormattedValue: "$2.10"},
	}
	result := RenderBarChart(items, 20)
	if result == "" {
		t.Error("bar chart should not be empty")
	}
	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestRenderBarChart_Empty(t *testing.T) {
	result := RenderBarChart(nil, 20)
	if result == "" {
		t.Error("should return 'No data' for empty input")
	}
}

func TestRenderBarChart_SingleItem(t *testing.T) {
	items := []BarChartItem{
		{Label: "query", Value: 100, FormattedValue: "$100.00"},
	}
	result := RenderBarChart(items, 20)
	if !strings.Contains(stripANSI(result), "query") {
		t.Error("should contain the label")
	}
}

func TestSparklineWithAnnotation(t *testing.T) {
	data := []float64{1.5, 2.3, 5.1, 3.2, 4.8, 2.1, 6.0}
	result := SparklineWithAnnotation(data, "7d", FormatDollars)
	plain := stripANSI(result)
	if !strings.Contains(plain, "total") {
		t.Error("annotation should contain 'total'")
	}
	if !strings.Contains(plain, "avg") {
		t.Error("annotation should contain 'avg'")
	}
	if !strings.Contains(plain, "peak") {
		t.Error("annotation should contain 'peak'")
	}
}

func TestFormatDollars(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "$0.00"},
		{1.5, "$1.50"},
		{0.001, "<$0.01"},
		{1234.56, "$1234.56"},
	}
	for _, tt := range tests {
		got := FormatDollars(tt.in)
		if got != tt.want {
			t.Errorf("FormatDollars(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// stripANSI removes ANSI escape sequences for testing plain text content.
func stripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip escape sequence until 'm'
			for i < len(s) && s[i] != 'm' {
				i++
			}
			if i < len(s) {
				i++ // skip 'm'
			}
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
}
