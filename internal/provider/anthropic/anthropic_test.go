package anthropic

import (
	"encoding/json"
	"io"
	"testing"

	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

// ---- Interface compliance ----

var (
	_ provider.Provider      = (*AnthropicProvider)(nil)
	_ provider.ModelSwitcher = (*AnthropicProvider)(nil)
	_ io.Closer              = (*AnthropicProvider)(nil)
)

// ---- Model / SetModel ----

func TestModel(t *testing.T) {
	p := &AnthropicProvider{modelName: "claude-sonnet-4-20250514"}
	if got := p.Model(); got != "claude-sonnet-4-20250514" {
		t.Errorf("Model() = %q, want %q", got, "claude-sonnet-4-20250514")
	}
}

func TestSetModel(t *testing.T) {
	p := &AnthropicProvider{modelName: "claude-sonnet-4-20250514"}
	p.SetModel("claude-opus-4-20250514")
	if got := p.Model(); got != "claude-opus-4-20250514" {
		t.Errorf("after SetModel, Model() = %q, want %q", got, "claude-opus-4-20250514")
	}
}

func TestClose(t *testing.T) {
	p := &AnthropicProvider{modelName: "claude-sonnet-4-20250514"}
	if err := p.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
}

// ---- New() constructor ----

func TestNew_MissingAPIKey(t *testing.T) {
	_, err := New("claude-sonnet-4-20250514", "CASCADE_TEST_ANTHROPIC_KEY_NONEXISTENT")
	if err == nil {
		t.Fatal("New() should return error when API key env var is not set")
	}
}

func TestNew_DefaultEnvVar(t *testing.T) {
	_, err := New("claude-sonnet-4-20250514", "")
	if err != nil {
		if got := err.Error(); !contains(got, "ANTHROPIC_API_KEY") {
			t.Errorf("error = %q, want it to mention ANTHROPIC_API_KEY", got)
		}
	}
	// If err == nil it means ANTHROPIC_API_KEY is actually set -- that's fine.
}

// ---- convertMessages ----

func TestConvertMessages_UserMessage(t *testing.T) {
	msgs := []types.Message{
		types.UserMessage("Hello, world!"),
	}
	antMsgs, sysPrompt := convertMessages(msgs)

	if sysPrompt != "" {
		t.Errorf("expected empty system prompt, got %q", sysPrompt)
	}
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)

	assertString(t, m, "role", "user")

	content := m["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
	block := content[0].(map[string]any)
	assertString(t, block, "type", "text")
	assertString(t, block, "text", "Hello, world!")
}

func TestConvertMessages_SystemMessage(t *testing.T) {
	msgs := []types.Message{
		types.SystemMessage("You are helpful."),
		types.UserMessage("Hi"),
	}
	antMsgs, sysPrompt := convertMessages(msgs)

	if sysPrompt != "You are helpful." {
		t.Errorf("system prompt = %q, want %q", sysPrompt, "You are helpful.")
	}
	// System messages are extracted, not included in antMsgs.
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message (system extracted), got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)
	assertString(t, m, "role", "user")
}

func TestConvertMessages_AssistantTextOnly(t *testing.T) {
	msgs := []types.Message{
		types.AssistantMessage("Sure, I can help.", nil),
	}
	antMsgs, _ := convertMessages(msgs)
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)
	assertString(t, m, "role", "assistant")

	content := m["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
	block := content[0].(map[string]any)
	assertString(t, block, "type", "text")
	assertString(t, block, "text", "Sure, I can help.")
}

func TestConvertMessages_AssistantWithToolCalls(t *testing.T) {
	msgs := []types.Message{
		types.AssistantMessage("Let me check.", []types.ToolCall{
			{
				ID:    "toolu_1",
				Name:  "read_file",
				Input: json.RawMessage(`{"path": "/tmp/test.go"}`),
			},
		}),
	}
	antMsgs, _ := convertMessages(msgs)
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)
	assertString(t, m, "role", "assistant")

	content := m["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("expected 2 content blocks (text + tool_use), got %d", len(content))
	}

	// First block: text
	textBlock := content[0].(map[string]any)
	assertString(t, textBlock, "type", "text")
	assertString(t, textBlock, "text", "Let me check.")

	// Second block: tool_use
	toolBlock := content[1].(map[string]any)
	assertString(t, toolBlock, "type", "tool_use")
	assertString(t, toolBlock, "id", "toolu_1")
	assertString(t, toolBlock, "name", "read_file")

	input := toolBlock["input"].(map[string]any)
	if input["path"] != "/tmp/test.go" {
		t.Errorf("tool_use input path = %v, want %q", input["path"], "/tmp/test.go")
	}
}

func TestConvertMessages_ToolResult(t *testing.T) {
	msgs := []types.Message{
		types.ToolResultMessage("toolu_1", "read_file", "file contents here", false),
	}
	antMsgs, _ := convertMessages(msgs)
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)

	// Anthropic tool results are sent as user messages.
	assertString(t, m, "role", "user")

	content := m["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
	block := content[0].(map[string]any)
	assertString(t, block, "type", "tool_result")
	assertString(t, block, "tool_use_id", "toolu_1")

	// Content should contain the tool output text.
	blockContent := block["content"].([]any)
	if len(blockContent) < 1 {
		t.Fatal("expected at least 1 content entry in tool_result")
	}
	textEntry := blockContent[0].(map[string]any)
	assertString(t, textEntry, "text", "file contents here")
}

func TestConvertMessages_ToolResultError(t *testing.T) {
	msgs := []types.Message{
		types.ToolResultMessage("toolu_2", "bash", "permission denied", true),
	}
	antMsgs, _ := convertMessages(msgs)
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)

	content := m["content"].([]any)
	block := content[0].(map[string]any)
	assertString(t, block, "type", "tool_result")

	// is_error should be true.
	isError, ok := block["is_error"]
	if !ok {
		t.Fatal("expected is_error field in tool_result")
	}
	if isError != true {
		t.Errorf("is_error = %v, want true", isError)
	}
}

func TestConvertMessages_SkipsEmptyMessages(t *testing.T) {
	msgs := []types.Message{
		types.SystemMessage(""),
		types.UserMessage(""),
		types.AssistantMessage("", nil),
		types.UserMessage("real content"),
	}
	antMsgs, sysPrompt := convertMessages(msgs)

	if sysPrompt != "" {
		t.Errorf("expected empty system prompt for empty system message, got %q", sysPrompt)
	}
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message (only non-empty), got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)
	assertString(t, m, "role", "user")
}

func TestConvertMessages_MultipleMessages(t *testing.T) {
	msgs := []types.Message{
		types.SystemMessage("Be helpful."),
		types.UserMessage("What is Go?"),
		types.AssistantMessage("Go is a programming language.", nil),
		types.UserMessage("Thanks!"),
	}
	antMsgs, sysPrompt := convertMessages(msgs)

	if sysPrompt != "Be helpful." {
		t.Errorf("system prompt = %q, want %q", sysPrompt, "Be helpful.")
	}
	// 3 messages (system is extracted).
	if len(antMsgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(antMsgs))
	}

	roles := []string{"user", "assistant", "user"}
	for i, expected := range roles {
		data := mustMarshal(t, antMsgs[i])
		m := parseJSON(t, data)
		assertString(t, m, "role", expected)
	}
}

func TestConvertMessages_ToolResultWithoutResult(t *testing.T) {
	msgs := []types.Message{
		{Role: types.RoleTool, ToolResult: nil},
	}
	antMsgs, _ := convertMessages(msgs)
	if len(antMsgs) != 0 {
		t.Fatalf("expected 0 messages for nil ToolResult, got %d", len(antMsgs))
	}
}

func TestConvertMessages_AssistantToolCallsOnly(t *testing.T) {
	// Assistant message with tool calls but no text content.
	msgs := []types.Message{
		types.AssistantMessage("", []types.ToolCall{
			{
				ID:    "toolu_1",
				Name:  "bash",
				Input: json.RawMessage(`{"command": "ls"}`),
			},
		}),
	}
	antMsgs, _ := convertMessages(msgs)
	if len(antMsgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(antMsgs))
	}

	data := mustMarshal(t, antMsgs[0])
	m := parseJSON(t, data)
	assertString(t, m, "role", "assistant")

	content := m["content"].([]any)
	// Should have only the tool_use block, no text block.
	if len(content) != 1 {
		t.Fatalf("expected 1 content block (tool_use only), got %d", len(content))
	}
	block := content[0].(map[string]any)
	assertString(t, block, "type", "tool_use")
	assertString(t, block, "name", "bash")
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
	antTools := convertTools(tools)
	if len(antTools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(antTools))
	}

	data := mustMarshal(t, antTools[0])
	m := parseJSON(t, data)

	assertString(t, m, "name", "read_file")
	assertString(t, m, "description", "Read a file from disk")

	inputSchema := m["input_schema"].(map[string]any)
	assertString(t, inputSchema, "type", "object")

	props := inputSchema["properties"].(map[string]any)
	pathProp := props["path"].(map[string]any)
	if pathProp["type"] != "string" {
		t.Errorf("path property type = %v, want %q", pathProp["type"], "string")
	}
}

func TestConvertTools_EmptySlice(t *testing.T) {
	antTools := convertTools(nil)
	if len(antTools) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(antTools))
	}
}

func TestConvertTools_MultipleTools(t *testing.T) {
	tools := []provider.Declaration{
		{Name: "tool_a", Description: "Tool A", Schema: map[string]any{"type": "object"}},
		{Name: "tool_b", Description: "Tool B", Schema: map[string]any{"type": "object"}},
	}
	antTools := convertTools(tools)
	if len(antTools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(antTools))
	}

	for i, name := range []string{"tool_a", "tool_b"} {
		data := mustMarshal(t, antTools[i])
		m := parseJSON(t, data)
		if m["name"] != name {
			t.Errorf("tool[%d] name = %v, want %q", i, m["name"], name)
		}
	}
}

func TestConvertTools_InputSchemaTypeAlwaysObject(t *testing.T) {
	// Even if schema doesn't include "type", convertTools should set it to "object".
	tools := []provider.Declaration{
		{
			Name:        "simple",
			Description: "A simple tool",
			Schema: map[string]any{
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
	}
	antTools := convertTools(tools)
	data := mustMarshal(t, antTools[0])
	m := parseJSON(t, data)

	inputSchema := m["input_schema"].(map[string]any)
	assertString(t, inputSchema, "type", "object")
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
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
