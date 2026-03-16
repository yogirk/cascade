package gcs

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/yogirk/cascade/internal/permission"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// nilClientProvider always returns nil, simulating missing credentials.
func nilClientProvider() *storage.Client { return nil }

// makeInput marshals a gcsInput to json.RawMessage for Execute calls.
func makeInput(t *testing.T, in gcsInput) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	return b
}

// ---------------------------------------------------------------------------
// 1. Tool metadata
// ---------------------------------------------------------------------------

func TestName(t *testing.T) {
	tool := NewGCSTool(nilClientProvider, "test-project", 100)
	if got := tool.Name(); got != "gcs" {
		t.Errorf("Name() = %q, want %q", got, "gcs")
	}
}

func TestDescription(t *testing.T) {
	tool := NewGCSTool(nilClientProvider, "test-project", 100)
	desc := tool.Description()
	for _, want := range []string{"list_buckets", "list_objects", "read_object", "object_info"} {
		if !strings.Contains(desc, want) {
			t.Errorf("Description() missing %q", want)
		}
	}
}

func TestInputSchema(t *testing.T) {
	tool := NewGCSTool(nilClientProvider, "test-project", 100)
	schema := tool.InputSchema()

	if schema["type"] != "object" {
		t.Errorf("InputSchema type = %v, want %q", schema["type"], "object")
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("InputSchema properties is not map[string]any")
	}

	for _, key := range []string{"action", "bucket", "prefix", "object", "max_lines"} {
		if _, ok := props[key]; !ok {
			t.Errorf("InputSchema missing property %q", key)
		}
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("InputSchema required is not []string")
	}
	if len(required) != 1 || required[0] != "action" {
		t.Errorf("InputSchema required = %v, want [\"action\"]", required)
	}
}

func TestRiskLevel(t *testing.T) {
	tool := NewGCSTool(nilClientProvider, "test-project", 100)
	if got := tool.RiskLevel(); got != permission.RiskReadOnly {
		t.Errorf("RiskLevel() = %v, want RiskReadOnly", got)
	}
}

// ---------------------------------------------------------------------------
// 2. Execute() dispatch — valid actions with nil client
//    (they should all hit the nil-client guard)
// ---------------------------------------------------------------------------

func TestExecute_NilClient_AllActions(t *testing.T) {
	tool := NewGCSTool(nilClientProvider, "test-project", 100)
	ctx := context.Background()

	actions := []string{"list_buckets", "list_objects", "read_object", "object_info"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			input := makeInput(t, gcsInput{Action: action, Bucket: "b", Object: "o"})
			res, err := tool.Execute(ctx, input)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true for nil client")
			}
			if !strings.Contains(res.Content, "Cloud Storage not available") {
				t.Errorf("unexpected content: %s", res.Content)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Execute() — unknown action
// ---------------------------------------------------------------------------

func TestExecute_UnknownAction(t *testing.T) {
	// Need a non-nil client provider so we get past the nil check.
	// We create a real client-less provider that returns a zero-value client.
	// But actually, the nil check happens first only if client==nil, and unknown
	// action is handled after the nil check. So we must pass nil and check that
	// nil-client fires first, OR we pass non-nil and check unknown action.
	//
	// With nil client, the nil-client guard fires before the switch. To test the
	// unknown-action branch, we need a non-nil client. We can use an empty
	// storage.Client (unsafe but fine for unit tests since we never call GCS).
	fakeClient := &storage.Client{}
	tool := NewGCSTool(func() *storage.Client { return fakeClient }, "test-project", 100)

	input := makeInput(t, gcsInput{Action: "delete_everything"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for unknown action")
	}
	if !strings.Contains(res.Content, "unknown action") {
		t.Errorf("expected 'unknown action' in content, got: %s", res.Content)
	}
	if !strings.Contains(res.Content, "delete_everything") {
		t.Errorf("expected action name in error, got: %s", res.Content)
	}
}

// ---------------------------------------------------------------------------
// 4. Execute() — invalid JSON input
// ---------------------------------------------------------------------------

func TestExecute_InvalidJSON(t *testing.T) {
	tool := NewGCSTool(nilClientProvider, "test-project", 100)

	res, err := tool.Execute(context.Background(), json.RawMessage(`{not valid json`))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for invalid JSON")
	}
	if !strings.Contains(res.Content, "invalid input") {
		t.Errorf("expected 'invalid input' in content, got: %s", res.Content)
	}
}

// ---------------------------------------------------------------------------
// 5. isTextContent()
// ---------------------------------------------------------------------------

func TestIsTextContent(t *testing.T) {
	tests := []struct {
		contentType string
		want        bool
	}{
		// text/* prefix
		{"text/plain", true},
		{"text/csv", true},
		{"text/html", true},
		{"TEXT/PLAIN", true}, // case-insensitive

		// application types that are text-readable
		{"application/json", true},
		{"application/xml", true},
		{"application/csv", true},
		{"application/x-ndjson", true},
		{"application/octet-stream", true},

		// yaml variants
		{"application/x-yaml", true},
		{"text/yaml", true},
		{"application/vnd.yaml", true},

		// sql variants
		{"application/sql", true},
		{"text/x-sql", true},

		// binary types — should be false
		{"image/png", false},
		{"image/jpeg", false},
		{"application/pdf", false},
		{"application/zip", false},
		{"video/mp4", false},
		{"application/gzip", false},

		// empty string
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := isTextContent(tt.contentType)
			if got != tt.want {
				t.Errorf("isTextContent(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 6. formatSize()
// ---------------------------------------------------------------------------

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{10240, "10.0 KB"},
		{1048576, "1.0 MB"},           // 1 MB exactly
		{1572864, "1.5 MB"},           // 1.5 MB
		{104857600, "100.0 MB"},       // 100 MB
		{1073741824, "1.0 GB"},        // 1 GB exactly
		{1610612736, "1.5 GB"},        // 1.5 GB
		{10737418240, "10.0 GB"},      // 10 GB
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 7. listObjects() — missing bucket param
// ---------------------------------------------------------------------------

func TestListObjects_MissingBucket(t *testing.T) {
	fakeClient := &storage.Client{}
	tool := NewGCSTool(func() *storage.Client { return fakeClient }, "test-project", 100)

	input := makeInput(t, gcsInput{Action: "list_objects"})
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for missing bucket")
	}
	if !strings.Contains(res.Content, "bucket parameter is required") {
		t.Errorf("expected 'bucket parameter is required' in content, got: %s", res.Content)
	}
}

// ---------------------------------------------------------------------------
// 8. readObject() — missing params
// ---------------------------------------------------------------------------

func TestReadObject_MissingParams(t *testing.T) {
	fakeClient := &storage.Client{}
	tool := NewGCSTool(func() *storage.Client { return fakeClient }, "test-project", 100)

	tests := []struct {
		name   string
		bucket string
		object string
	}{
		{"both empty", "", ""},
		{"bucket empty", "", "some/object.txt"},
		{"object empty", "my-bucket", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeInput(t, gcsInput{Action: "read_object", Bucket: tt.bucket, Object: tt.object})
			res, err := tool.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true for missing params")
			}
			if !strings.Contains(res.Content, "bucket and object parameters are required") {
				t.Errorf("unexpected content: %s", res.Content)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 9. objectInfo() — missing params
// ---------------------------------------------------------------------------

func TestObjectInfo_MissingParams(t *testing.T) {
	fakeClient := &storage.Client{}
	tool := NewGCSTool(func() *storage.Client { return fakeClient }, "test-project", 100)

	tests := []struct {
		name   string
		bucket string
		object string
	}{
		{"both empty", "", ""},
		{"bucket empty", "", "some/object.txt"},
		{"object empty", "my-bucket", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := makeInput(t, gcsInput{Action: "object_info", Bucket: tt.bucket, Object: tt.object})
			res, err := tool.Execute(context.Background(), input)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if !res.IsError {
				t.Error("expected IsError=true for missing params")
			}
			if !strings.Contains(res.Content, "bucket and object parameters are required") {
				t.Errorf("unexpected content: %s", res.Content)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 10. Close() — no-op
// ---------------------------------------------------------------------------

func TestClose(t *testing.T) {
	tool := NewGCSTool(nilClientProvider, "test-project", 100)
	if err := tool.Close(); err != nil {
		t.Errorf("Close() = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// 11. NewGCSTool defaults maxReadLines when <= 0
// ---------------------------------------------------------------------------

func TestNewGCSTool_DefaultMaxReadLines(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"zero", 0, 100},
		{"negative", -5, 100},
		{"positive", 50, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := NewGCSTool(nilClientProvider, "proj", tt.input)
			if tool.maxReadLines != tt.expected {
				t.Errorf("maxReadLines = %d, want %d", tool.maxReadLines, tt.expected)
			}
		})
	}
}
