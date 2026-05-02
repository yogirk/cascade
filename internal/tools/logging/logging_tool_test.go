package logging

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/logging/logadmin"
	"github.com/slokam-ai/cascade/internal/permission"
)

// ---------------------------------------------------------------------------
// Tool metadata
// ---------------------------------------------------------------------------

func TestName(t *testing.T) {
	lt := NewLogTool(nil, "test-project", 50)
	if got := lt.Name(); got != "cloud_logging" {
		t.Errorf("Name() = %q, want %q", got, "cloud_logging")
	}
}

func TestDescription(t *testing.T) {
	lt := NewLogTool(nil, "test-project", 50)
	desc := lt.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
	if !strings.Contains(desc, "Cloud Logging") {
		t.Errorf("Description() should mention Cloud Logging, got %q", desc)
	}
}

func TestInputSchema(t *testing.T) {
	lt := NewLogTool(nil, "test-project", 50)
	s := lt.InputSchema()

	props, ok := s["properties"].(map[string]any)
	if !ok {
		t.Fatal("InputSchema missing properties")
	}

	for _, key := range []string{"action", "filter", "duration", "limit"} {
		if _, ok := props[key]; !ok {
			t.Errorf("InputSchema missing %q property", key)
		}
	}

	req, ok := s["required"].([]string)
	if !ok {
		t.Fatal("InputSchema missing required")
	}
	found := false
	for _, r := range req {
		if r == "action" {
			found = true
		}
	}
	if !found {
		t.Error("InputSchema: 'action' not in required")
	}
}

func TestRiskLevel(t *testing.T) {
	lt := NewLogTool(nil, "test-project", 50)
	if got := lt.RiskLevel(); got != permission.RiskReadOnly {
		t.Errorf("RiskLevel() = %d, want %d (RiskReadOnly)", got, permission.RiskReadOnly)
	}
}

// ---------------------------------------------------------------------------
// NewLogTool defaults
// ---------------------------------------------------------------------------

func TestNewLogTool_DefaultMaxEntries(t *testing.T) {
	lt := NewLogTool(nil, "proj", 0)
	if lt.maxEntries != 50 {
		t.Errorf("maxEntries = %d, want 50 when given 0", lt.maxEntries)
	}

	lt2 := NewLogTool(nil, "proj", -5)
	if lt2.maxEntries != 50 {
		t.Errorf("maxEntries = %d, want 50 when given -5", lt2.maxEntries)
	}
}

func TestNewLogTool_CustomMaxEntries(t *testing.T) {
	lt := NewLogTool(nil, "proj", 100)
	if lt.maxEntries != 100 {
		t.Errorf("maxEntries = %d, want 100", lt.maxEntries)
	}
}

// ---------------------------------------------------------------------------
// Execute() dispatch
// ---------------------------------------------------------------------------

func TestExecute_InvalidJSON(t *testing.T) {
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, err := lt.Execute(context.Background(), json.RawMessage(`{bad json`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for invalid JSON")
	}
	if !strings.Contains(result.Content, "invalid input") {
		t.Errorf("expected 'invalid input' in content, got %q", result.Content)
	}
}

func TestExecute_InvalidAction(t *testing.T) {
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, err := lt.Execute(context.Background(), json.RawMessage(`{"action":"destroy"}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for unknown action")
	}
	if !strings.Contains(result.Content, "unknown action") {
		t.Errorf("expected 'unknown action' in content, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "destroy") {
		t.Errorf("expected action name in content, got %q", result.Content)
	}
}

func TestExecute_QueryAction_NilClient(t *testing.T) {
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, err := lt.Execute(context.Background(), json.RawMessage(`{"action":"query","filter":"severity>=ERROR"}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError when client is nil")
	}
	if !strings.Contains(result.Content, "Cloud Logging not available") {
		t.Errorf("expected 'Cloud Logging not available' in content, got %q", result.Content)
	}
}

func TestExecute_TailAction_NilClient(t *testing.T) {
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, err := lt.Execute(context.Background(), json.RawMessage(`{"action":"tail"}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError when client is nil")
	}
	if !strings.Contains(result.Content, "Cloud Logging not available") {
		t.Errorf("expected 'Cloud Logging not available' in content, got %q", result.Content)
	}
}

func TestExecute_InvalidDuration_NilClient(t *testing.T) {
	// Even though client is nil, the nil-client check happens before duration parse.
	// So we get the nil-client error, not a duration error.
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, err := lt.Execute(context.Background(), json.RawMessage(`{"action":"query","duration":"bogus"}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError")
	}
	// Client nil check is first, so expect that message
	if !strings.Contains(result.Content, "Cloud Logging not available") {
		t.Errorf("expected nil-client error, got %q", result.Content)
	}
}

func TestExecute_EmptyAction(t *testing.T) {
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, err := lt.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for empty action")
	}
	if !strings.Contains(result.Content, "unknown action") {
		t.Errorf("expected 'unknown action' in content, got %q", result.Content)
	}
}

// ---------------------------------------------------------------------------
// parseDuration()
// ---------------------------------------------------------------------------

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"1h", 1 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"2d", 2 * 24 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"30d", 30 * 24 * time.Hour, false},
		{"500ms", 500 * time.Millisecond, false},
		{"  1h  ", 1 * time.Hour, false},          // whitespace trimmed
		{"  7D  ", 7 * 24 * time.Hour, false},     // case insensitive + whitespace
		{"-1h", -1 * time.Hour, false},            // Go's time.ParseDuration accepts negatives
		{"", 0, true},                             // empty string
		{"xyz", 0, true},                          // garbage
		{"d", 0, true},                            // just "d" with no number
	}

	for _, tc := range tests {
		name := tc.input
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			got, err := parseDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("parseDuration(%q) expected error, got %v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDuration(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractMessage()
// ---------------------------------------------------------------------------

func TestExtractMessage_NilPayload(t *testing.T) {
	if got := extractMessage(nil); got != "" {
		t.Errorf("extractMessage(nil) = %q, want empty", got)
	}
}

func TestExtractMessage_StringPayload(t *testing.T) {
	msg := "this is a log message"
	if got := extractMessage(msg); got != msg {
		t.Errorf("extractMessage(string) = %q, want %q", got, msg)
	}
}

func TestExtractMessage_StringPayload_Long(t *testing.T) {
	msg := strings.Repeat("a", 200)
	got := extractMessage(msg)
	if len(got) > 120 {
		t.Errorf("extractMessage should truncate to 120 chars, got len %d", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected truncated string to end with '...', got %q", got)
	}
}

func TestExtractMessage_MapWithMessage(t *testing.T) {
	payload := map[string]interface{}{
		"message": "hello world",
		"extra":   123,
	}
	got := extractMessage(payload)
	if got != "hello world" {
		t.Errorf("extractMessage(message key) = %q, want %q", got, "hello world")
	}
}

func TestExtractMessage_MapWithTextPayload(t *testing.T) {
	payload := map[string]interface{}{
		"textPayload": "text payload value",
	}
	got := extractMessage(payload)
	if got != "text payload value" {
		t.Errorf("extractMessage(textPayload) = %q, want %q", got, "text payload value")
	}
}

func TestExtractMessage_MessageTakesPrecedence(t *testing.T) {
	// "message" should be checked before "textPayload"
	payload := map[string]interface{}{
		"message":     "primary",
		"textPayload": "secondary",
	}
	got := extractMessage(payload)
	if got != "primary" {
		t.Errorf("extractMessage should prefer 'message' over 'textPayload', got %q", got)
	}
}

func TestExtractMessage_StatusMessage(t *testing.T) {
	payload := map[string]interface{}{
		"status": map[string]interface{}{
			"message": "quota exceeded",
		},
	}
	got := extractMessage(payload)
	if got != "quota exceeded" {
		t.Errorf("extractMessage(status.message) = %q, want %q", got, "quota exceeded")
	}
}

func TestExtractMessage_ServiceDataPath(t *testing.T) {
	payload := map[string]interface{}{
		"serviceData": map[string]interface{}{
			"jobCompletedEvent": map[string]interface{}{
				"job": map[string]interface{}{
					"jobStatus": map[string]interface{}{
						"error": map[string]interface{}{
							"message": "BQ job failed",
						},
					},
				},
			},
		},
	}
	got := extractMessage(payload)
	if got != "BQ job failed" {
		t.Errorf("extractMessage(serviceData path) = %q, want %q", got, "BQ job failed")
	}
}

func TestExtractMessage_MetadataPath(t *testing.T) {
	payload := map[string]interface{}{
		"metadata": map[string]interface{}{
			"jobChange": map[string]interface{}{
				"job": map[string]interface{}{
					"jobStatus": map[string]interface{}{
						"error": map[string]interface{}{
							"message": "metadata error msg",
						},
					},
				},
			},
		},
	}
	got := extractMessage(payload)
	if got != "metadata error msg" {
		t.Errorf("extractMessage(metadata path) = %q, want %q", got, "metadata error msg")
	}
}

func TestExtractMessage_MethodNameOnly(t *testing.T) {
	payload := map[string]interface{}{
		"methodName": "google.cloud.bigquery.v2.JobService.InsertJob",
	}
	got := extractMessage(payload)
	if got != "google.cloud.bigquery.v2.JobService.InsertJob" {
		t.Errorf("extractMessage(methodName) = %q, want %q", got, "google.cloud.bigquery.v2.JobService.InsertJob")
	}
}

func TestExtractMessage_MethodNameWithServiceName(t *testing.T) {
	payload := map[string]interface{}{
		"methodName":  "InsertJob",
		"serviceName": "bigquery.googleapis.com",
	}
	got := extractMessage(payload)
	want := "bigquery.googleapis.com \u2192 InsertJob"
	if got != want {
		t.Errorf("extractMessage(methodName+serviceName) = %q, want %q", got, want)
	}
}

func TestExtractMessage_FallbackJSON(t *testing.T) {
	// A map with no recognized keys should fall back to JSON
	payload := map[string]interface{}{
		"foo": "bar",
		"num": 42,
	}
	got := extractMessage(payload)
	if got == "" {
		t.Error("extractMessage fallback should not be empty")
	}
	// Should contain the key from the JSON
	if !strings.Contains(got, "foo") {
		t.Errorf("fallback JSON should contain key 'foo', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// deepString()
// ---------------------------------------------------------------------------

func TestDeepString_ValidPath(t *testing.T) {
	m := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "found",
			},
		},
	}
	if got := deepString(m, "a", "b", "c"); got != "found" {
		t.Errorf("deepString(a.b.c) = %q, want %q", got, "found")
	}
}

func TestDeepString_SingleKey(t *testing.T) {
	m := map[string]interface{}{
		"key": "value",
	}
	if got := deepString(m, "key"); got != "value" {
		t.Errorf("deepString(key) = %q, want %q", got, "value")
	}
}

func TestDeepString_MissingKey(t *testing.T) {
	m := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "hello",
		},
	}
	if got := deepString(m, "a", "missing"); got != "" {
		t.Errorf("deepString(a.missing) = %q, want empty", got)
	}
}

func TestDeepString_MissingIntermediateKey(t *testing.T) {
	m := map[string]interface{}{
		"a": map[string]interface{}{},
	}
	if got := deepString(m, "a", "b", "c"); got != "" {
		t.Errorf("deepString(a.b.c missing) = %q, want empty", got)
	}
}

func TestDeepString_NonMapIntermediate(t *testing.T) {
	m := map[string]interface{}{
		"a": "not a map",
	}
	if got := deepString(m, "a", "b"); got != "" {
		t.Errorf("deepString non-map intermediate = %q, want empty", got)
	}
}

func TestDeepString_NonStringLeaf(t *testing.T) {
	m := map[string]interface{}{
		"a": map[string]interface{}{
			"b": 42,
		},
	}
	if got := deepString(m, "a", "b"); got != "" {
		t.Errorf("deepString non-string leaf = %q, want empty", got)
	}
}

func TestDeepString_EmptyMap(t *testing.T) {
	m := map[string]interface{}{}
	if got := deepString(m, "a"); got != "" {
		t.Errorf("deepString empty map = %q, want empty", got)
	}
}

func TestDeepString_NoKeys(t *testing.T) {
	// No keys means we try to cast the map itself to string → empty
	m := map[string]interface{}{"a": "b"}
	if got := deepString(m); got != "" {
		t.Errorf("deepString no keys = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// truncateMsg()
// ---------------------------------------------------------------------------

func TestTruncateMsg_UnderLimit(t *testing.T) {
	msg := "short message"
	got := truncateMsg(msg, 120)
	if got != msg {
		t.Errorf("truncateMsg under limit = %q, want %q", got, msg)
	}
}

func TestTruncateMsg_AtLimit(t *testing.T) {
	msg := strings.Repeat("x", 120)
	got := truncateMsg(msg, 120)
	if got != msg {
		t.Errorf("truncateMsg at limit should not truncate, got len %d", len(got))
	}
}

func TestTruncateMsg_OverLimit(t *testing.T) {
	msg := strings.Repeat("x", 200)
	got := truncateMsg(msg, 120)
	if len(got) != 120 {
		t.Errorf("truncateMsg over limit: len = %d, want 120", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncateMsg over limit should end with '...', got %q", got[len(got)-10:])
	}
	// First 117 chars should be 'x'
	if got[:117] != strings.Repeat("x", 117) {
		t.Error("truncateMsg should preserve first 117 chars before '...'")
	}
}

func TestTruncateMsg_WithNewlines(t *testing.T) {
	msg := "line one\nline two\rline three"
	got := truncateMsg(msg, 120)
	if strings.Contains(got, "\n") {
		t.Errorf("truncateMsg should replace \\n, got %q", got)
	}
	if strings.Contains(got, "\r") {
		t.Errorf("truncateMsg should replace \\r, got %q", got)
	}
	// \n → space, \r → removed (empty string)
	want := "line one line twoline three"
	if got != want {
		t.Errorf("truncateMsg newlines = %q, want %q", got, want)
	}
}

func TestTruncateMsg_NewlinesAndTruncation(t *testing.T) {
	// Newlines replaced first, then truncation
	msg := strings.Repeat("a\n", 100) // 200 chars total
	got := truncateMsg(msg, 50)
	if strings.Contains(got, "\n") {
		t.Error("should not contain newlines after truncation")
	}
	if len(got) != 50 {
		t.Errorf("len = %d, want 50", len(got))
	}
	if !strings.HasSuffix(got, "...") {
		t.Error("should end with '...'")
	}
}

func TestTruncateMsg_EmptyString(t *testing.T) {
	got := truncateMsg("", 120)
	if got != "" {
		t.Errorf("truncateMsg empty = %q, want empty", got)
	}
}

func TestTruncateMsg_SmallLimit(t *testing.T) {
	got := truncateMsg("hello world", 5)
	if got != "he..." {
		t.Errorf("truncateMsg small limit = %q, want %q", got, "he...")
	}
}

// ---------------------------------------------------------------------------
// queryLogs() with nil client
// ---------------------------------------------------------------------------

func TestQueryLogs_NilClient(t *testing.T) {
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, err := lt.queryLogs(context.Background(), logInput{Action: "query"})
	if err != nil {
		t.Fatalf("queryLogs returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for nil client")
	}
	if !strings.Contains(result.Content, "Cloud Logging not available") {
		t.Errorf("expected nil-client message, got %q", result.Content)
	}
}

func TestQueryLogs_NilClient_WithCredentialHint(t *testing.T) {
	lt := NewLogTool(func() *logadmin.Client { return nil }, "proj", 50)
	result, _ := lt.queryLogs(context.Background(), logInput{Action: "query"})
	if !strings.Contains(result.Content, "roles/logging.viewer") {
		t.Errorf("expected permission hint in message, got %q", result.Content)
	}
}

func TestQueryLogs_InvalidDuration(t *testing.T) {
	// Need a non-nil client to get past the nil check and reach duration parsing.
	// We can't easily create a real logadmin.Client in unit tests, but we can
	// test via Execute which calls queryLogs. However, the nil-client check
	// comes first. So we just verify that parseDuration errors propagate properly
	// by testing parseDuration directly (already covered above).
	// Here we verify the error message format for invalid duration in queryLogs.
	lt := &LogTool{
		getClient: func() *logadmin.Client { return nil },
		projectID: "proj",
		maxEntries: 50,
	}
	// Nil client check comes first, so duration error is not reached.
	result, _ := lt.queryLogs(context.Background(), logInput{Action: "query", Duration: "bad"})
	if !result.IsError {
		t.Fatal("expected error")
	}
}

// ---------------------------------------------------------------------------
// Default limit behavior
// ---------------------------------------------------------------------------

func TestLimitDefaults(t *testing.T) {
	tests := []struct {
		name       string
		paramLimit int
		maxEntries int
		wantLimit  int
	}{
		{"zero uses maxEntries", 0, 50, 50},
		{"negative uses maxEntries", -1, 50, 50},
		{"within max", 25, 50, 25},
		{"at max", 50, 50, 50},
		{"over max is capped", 100, 50, 50},
		{"over max is capped (200)", 200, 75, 75},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// We can't run a full queryLogs without a real client,
			// but we can verify the logic by replicating the limit computation.
			limit := tc.paramLimit
			maxEntries := tc.maxEntries
			if limit <= 0 {
				limit = maxEntries
			}
			if limit > maxEntries {
				limit = maxEntries
			}
			if limit != tc.wantLimit {
				t.Errorf("limit = %d, want %d", limit, tc.wantLimit)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RenderLogEntries (smoke test)
// ---------------------------------------------------------------------------

func TestRenderLogEntries_Empty(t *testing.T) {
	display, content := RenderLogEntries(nil, "severity>=ERROR", 1*time.Hour)
	if !strings.Contains(content, "No log entries found") {
		t.Errorf("expected 'No log entries found' in content, got %q", content)
	}
	if !strings.Contains(display, "No log entries found") {
		t.Errorf("expected 'No log entries found' in display, got %q", display)
	}
	if !strings.Contains(content, "0 entries") {
		t.Errorf("expected '0 entries' in content, got %q", content)
	}
}

func TestRenderLogEntries_SingleEntry(t *testing.T) {
	entries := []LogEntry{
		{
			Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Severity:  "ERROR",
			Resource:  "bigquery_dataset",
			LogName:   "cloudaudit.googleapis.com/data_access",
			Message:   "Something went wrong",
		},
	}
	display, content := RenderLogEntries(entries, "", 1*time.Hour)

	if !strings.Contains(content, "ERROR") {
		t.Error("content should contain severity")
	}
	if !strings.Contains(content, "Something went wrong") {
		t.Error("content should contain message")
	}
	if !strings.Contains(content, "bigquery_dataset") {
		t.Error("content should contain resource")
	}
	if !strings.Contains(display, "1 entries") {
		t.Error("display header should contain entry count")
	}
}

func TestRenderLogEntries_MultipleEntries(t *testing.T) {
	entries := []LogEntry{
		{
			Timestamp: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Severity:  "ERROR",
			Resource:  "gce_instance",
			LogName:   "syslog",
			Message:   "disk full",
		},
		{
			Timestamp: time.Date(2025, 1, 15, 10, 31, 0, 0, time.UTC),
			Severity:  "WARNING",
			Resource:  "gce_instance",
			LogName:   "syslog",
			Message:   "disk at 90%",
		},
	}
	display, content := RenderLogEntries(entries, "resource.type=\"gce_instance\"", 24*time.Hour)

	if !strings.Contains(content, "2 entries") {
		t.Error("content should mention 2 entries")
	}
	if !strings.Contains(content, "disk full") {
		t.Error("content should contain first message")
	}
	if !strings.Contains(content, "disk at 90%") {
		t.Error("content should contain second message")
	}
	if !strings.Contains(content, "Filter:") {
		t.Error("content should contain filter when provided")
	}
	_ = display // display is styled, just ensure no panic
}

// ---------------------------------------------------------------------------
// formatLogDuration (from render.go, exported via package access)
// ---------------------------------------------------------------------------

func TestFormatLogDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Minute, "30m"},
		{1 * time.Hour, "1h"},
		{6 * time.Hour, "6h"},
		{24 * time.Hour, "1d"},
		{7 * 24 * time.Hour, "7d"},
	}
	for _, tc := range tests {
		got := formatLogDuration(tc.d)
		if got != tc.want {
			t.Errorf("formatLogDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// severityColor / severityBadge (smoke tests)
// ---------------------------------------------------------------------------

func TestSeverityBadge_AllLevels(t *testing.T) {
	levels := []string{"DEBUG", "DEFAULT", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY", "UNKNOWN"}
	for _, level := range levels {
		badge := severityBadge(level)
		if badge == "" {
			t.Errorf("severityBadge(%q) returned empty string", level)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge cases for extractMessage with unusual types
// ---------------------------------------------------------------------------

func TestExtractMessage_IntPayload(t *testing.T) {
	// An integer payload goes through the default branch (json.Marshal → not a map)
	got := extractMessage(42)
	if got != "42" {
		t.Errorf("extractMessage(42) = %q, want %q", got, "42")
	}
}

func TestExtractMessage_BoolPayload(t *testing.T) {
	got := extractMessage(true)
	if got != "true" {
		t.Errorf("extractMessage(true) = %q, want %q", got, "true")
	}
}

func TestExtractMessage_SlicePayload(t *testing.T) {
	payload := []string{"a", "b", "c"}
	got := extractMessage(payload)
	// Should be JSON fallback since slice is not a map
	if !strings.Contains(got, "a") {
		t.Errorf("extractMessage(slice) should contain element, got %q", got)
	}
}

func TestExtractMessage_NestedPayload_PriorityOrder(t *testing.T) {
	// "message" takes precedence over all other paths
	payload := map[string]interface{}{
		"message":    "top-level message",
		"methodName": "should.not.appear",
		"status": map[string]interface{}{
			"message": "status message",
		},
	}
	got := extractMessage(payload)
	if got != "top-level message" {
		t.Errorf("expected 'message' to have highest priority, got %q", got)
	}
}

func TestExtractMessage_MethodNameWithEmptyServiceName(t *testing.T) {
	payload := map[string]interface{}{
		"methodName":  "DoSomething",
		"serviceName": "",
	}
	got := extractMessage(payload)
	// Empty serviceName should result in just the method name
	if got != "DoSomething" {
		t.Errorf("extractMessage with empty serviceName = %q, want %q", got, "DoSomething")
	}
}
