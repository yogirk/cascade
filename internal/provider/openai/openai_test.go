package openai

import (
	"encoding/json"
	"io"
	"testing"

	oai "github.com/openai/openai-go/v3"

	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

// ---- Interface compliance ----

var (
	_ provider.Provider      = (*OpenAIProvider)(nil)
	_ provider.ModelSwitcher = (*OpenAIProvider)(nil)
	_ io.Closer              = (*OpenAIProvider)(nil)
)

// ---- Model / SetModel ----

func TestModel(t *testing.T) {
	p := &OpenAIProvider{modelName: "gpt-4o"}
	if got := p.Model(); got != "gpt-4o" {
		t.Errorf("Model() = %q, want %q", got, "gpt-4o")
	}
}

func TestSetModel(t *testing.T) {
	p := &OpenAIProvider{modelName: "gpt-4o"}
	p.SetModel("gpt-4o-mini")
	if got := p.Model(); got != "gpt-4o-mini" {
		t.Errorf("after SetModel, Model() = %q, want %q", got, "gpt-4o-mini")
	}
}

func TestClose(t *testing.T) {
	p := &OpenAIProvider{modelName: "gpt-4o"}
	if err := p.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// ---- New() constructor ----

func TestNew_MissingAPIKey(t *testing.T) {
	// Use a non-existent env var name to guarantee it is unset.
	_, err := New("gpt-4o", "CASCADE_TEST_OPENAI_KEY_NONEXISTENT")
	if err == nil {
		t.Fatal("New() should return error when API key env var is not set")
	}
}

func TestNew_DefaultEnvVar(t *testing.T) {
	// When apiKeyEnv is empty, it should fall back to OPENAI_API_KEY.
	// Since we can't rely on that being set in CI, just ensure no panic
	// and the error message references OPENAI_API_KEY.
	_, err := New("gpt-4o", "")
	if err != nil {
		// Error should reference the default env var name.
		if got := err.Error(); !contains(got, "OPENAI_API_KEY") {
			t.Errorf("error = %q, want it to mention OPENAI_API_KEY", got)
		}
	}
	// If err == nil it means OPENAI_API_KEY is actually set -- that's fine.
}

// ---- convertMessages ----

func TestConvertMessages_UserMessage(t *testing.T) {
	msgs := []types.Message{
		types.UserMessage("Hello, world!"),
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(oaiMsgs))
	}

	data := mustMarshal(t, oaiMsgs[0])
	m := parseJSON(t, data)

	assertString(t, m, "role", "user")
	assertString(t, m, "content", "Hello, world!")
}

func TestConvertMessages_SystemMessage(t *testing.T) {
	msgs := []types.Message{
		types.SystemMessage("You are helpful."),
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(oaiMsgs))
	}

	data := mustMarshal(t, oaiMsgs[0])
	m := parseJSON(t, data)

	assertString(t, m, "role", "system")
	assertString(t, m, "content", "You are helpful.")
}

func TestConvertMessages_AssistantTextOnly(t *testing.T) {
	msgs := []types.Message{
		types.AssistantMessage("Sure, I can help.", nil),
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(oaiMsgs))
	}

	data := mustMarshal(t, oaiMsgs[0])
	m := parseJSON(t, data)

	assertString(t, m, "role", "assistant")
	assertString(t, m, "content", "Sure, I can help.")
}

func TestConvertMessages_AssistantWithToolCalls(t *testing.T) {
	msgs := []types.Message{
		types.AssistantMessage("Let me check.", []types.ToolCall{
			{
				ID:    "call_1",
				Name:  "read_file",
				Input: json.RawMessage(`{"path": "/tmp/test.go"}`),
			},
		}),
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(oaiMsgs))
	}

	data := mustMarshal(t, oaiMsgs[0])
	m := parseJSON(t, data)

	assertString(t, m, "role", "assistant")

	toolCalls, ok := m["tool_calls"].([]any)
	if !ok || len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool_call, got %v", m["tool_calls"])
	}
	tc := toolCalls[0].(map[string]any)
	if tc["id"] != "call_1" {
		t.Errorf("tool_call id = %v, want %q", tc["id"], "call_1")
	}
	fn := tc["function"].(map[string]any)
	if fn["name"] != "read_file" {
		t.Errorf("tool_call function name = %v, want %q", fn["name"], "read_file")
	}
	if fn["arguments"] != `{"path": "/tmp/test.go"}` {
		t.Errorf("tool_call function arguments = %v, want %q", fn["arguments"], `{"path": "/tmp/test.go"}`)
	}
}

func TestConvertMessages_ToolResult(t *testing.T) {
	msgs := []types.Message{
		types.ToolResultMessage("call_1", "read_file", "file contents here", false),
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(oaiMsgs))
	}

	data := mustMarshal(t, oaiMsgs[0])
	m := parseJSON(t, data)

	assertString(t, m, "role", "tool")

	assertString(t, m, "content", "file contents here")
	assertString(t, m, "tool_call_id", "call_1")
}

func TestConvertMessages_SkipsEmptyMessages(t *testing.T) {
	msgs := []types.Message{
		types.SystemMessage(""),
		types.UserMessage(""),
		types.AssistantMessage("", nil),
		types.UserMessage("real content"),
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 1 {
		t.Fatalf("expected 1 message (only non-empty), got %d", len(oaiMsgs))
	}

	data := mustMarshal(t, oaiMsgs[0])
	m := parseJSON(t, data)
	assertString(t, m, "role", "user")
	assertString(t, m, "content", "real content")
}

func TestConvertMessages_MultipleMessages(t *testing.T) {
	msgs := []types.Message{
		types.SystemMessage("You are helpful."),
		types.UserMessage("What is Go?"),
		types.AssistantMessage("Go is a programming language.", nil),
		types.UserMessage("Thanks!"),
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(oaiMsgs))
	}

	roles := []string{"system", "user", "assistant", "user"}
	for i, expected := range roles {
		data := mustMarshal(t, oaiMsgs[i])
		m := parseJSON(t, data)
		assertString(t, m, "role", expected)
	}
}

func TestConvertMessages_ToolResultWithoutResult(t *testing.T) {
	// A tool message with nil ToolResult should be skipped.
	msgs := []types.Message{
		{Role: types.RoleTool, ToolResult: nil},
	}
	oaiMsgs := convertMessages(msgs)
	if len(oaiMsgs) != 0 {
		t.Fatalf("expected 0 messages for nil ToolResult, got %d", len(oaiMsgs))
	}
}

// ---- convertTools ----

func TestConvertTools_SingleTool(t *testing.T) {
	tools := []provider.Declaration{
		{
			Name:        "read_file",
			Description: "Read a file from disk",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "File path",
					},
				},
				"required": []any{"path"},
			},
		},
	}
	oaiTools := convertTools(tools)
	if len(oaiTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(oaiTools))
	}

	data := mustMarshal(t, oaiTools[0])
	m := parseJSON(t, data)

	assertString(t, m, "type", "function")

	fn := m["function"].(map[string]any)
	if fn["name"] != "read_file" {
		t.Errorf("tool name = %v, want %q", fn["name"], "read_file")
	}
	if fn["description"] != "Read a file from disk" {
		t.Errorf("tool description = %v, want %q", fn["description"], "Read a file from disk")
	}

	params := fn["parameters"].(map[string]any)
	if params["type"] != "object" {
		t.Errorf("parameters type = %v, want %q", params["type"], "object")
	}
	props := params["properties"].(map[string]any)
	pathProp := props["path"].(map[string]any)
	if pathProp["type"] != "string" {
		t.Errorf("path property type = %v, want %q", pathProp["type"], "string")
	}
}

func TestConvertTools_EmptySlice(t *testing.T) {
	oaiTools := convertTools(nil)
	if len(oaiTools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(oaiTools))
	}
}

func TestConvertTools_MultipleTools(t *testing.T) {
	tools := []provider.Declaration{
		{Name: "tool_a", Description: "Tool A", Schema: map[string]any{"type": "object"}},
		{Name: "tool_b", Description: "Tool B", Schema: map[string]any{"type": "object"}},
	}
	oaiTools := convertTools(tools)
	if len(oaiTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(oaiTools))
	}

	for i, name := range []string{"tool_a", "tool_b"} {
		data := mustMarshal(t, oaiTools[i])
		m := parseJSON(t, data)
		fn := m["function"].(map[string]any)
		if fn["name"] != name {
			t.Errorf("tool[%d] name = %v, want %q", i, fn["name"], name)
		}
	}
}

func TestBuildResponse_EmptyAccumulator(t *testing.T) {
	resp := buildResponse([]string{"partial", " response"}, oai.ChatCompletionAccumulator{})

	if resp.Text != "partial response" {
		t.Fatalf("Text = %q, want %q", resp.Text, "partial response")
	}
	if len(resp.ToolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(resp.ToolCalls))
	}
	if resp.Usage != nil {
		t.Fatalf("expected nil usage, got %#v", resp.Usage)
	}
}

// ---- Helpers ----

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	return data
}

func parseJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v\nraw: %s", err, string(data))
	}
	return m
}

func assertString(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	got, ok := m[key]
	if !ok {
		t.Errorf("key %q not found in JSON", key)
		return
	}
	if got != want {
		t.Errorf("%s = %v, want %q", key, got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
