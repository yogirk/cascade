package gemini

import (
	"encoding/json"
	"testing"

	"github.com/cascade-cli/cascade/pkg/types"
	"google.golang.org/genai"
)

func TestConvertToGenAI_UserMessage(t *testing.T) {
	msgs := []types.Message{
		types.UserMessage("Hello, world!"),
	}

	contents := convertToGenAI(msgs)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	c := contents[0]
	if c.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", c.Role)
	}
	if len(c.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(c.Parts))
	}
	if c.Parts[0].Text != "Hello, world!" {
		t.Errorf("expected text %q, got %q", "Hello, world!", c.Parts[0].Text)
	}
}

func TestConvertToGenAI_AssistantWithToolCalls(t *testing.T) {
	msgs := []types.Message{
		types.AssistantMessage("Let me check.", []types.ToolCall{
			{
				ID:    "call_1",
				Name:  "read_file",
				Input: json.RawMessage(`{"path": "/tmp/test.go"}`),
			},
		}),
	}

	contents := convertToGenAI(msgs)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	c := contents[0]
	if c.Role != "model" {
		t.Errorf("expected role %q, got %q", "model", c.Role)
	}
	// Should have text part + function call part
	if len(c.Parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(c.Parts))
	}
	if c.Parts[0].Text != "Let me check." {
		t.Errorf("expected text %q, got %q", "Let me check.", c.Parts[0].Text)
	}
	fc := c.Parts[1].FunctionCall
	if fc == nil {
		t.Fatal("expected function call part")
	}
	if fc.Name != "read_file" {
		t.Errorf("expected function call name %q, got %q", "read_file", fc.Name)
	}
	if fc.ID != "call_1" {
		t.Errorf("expected function call ID %q, got %q", "call_1", fc.ID)
	}
	path, ok := fc.Args["path"]
	if !ok {
		t.Fatal("expected 'path' in function call args")
	}
	if path != "/tmp/test.go" {
		t.Errorf("expected path %q, got %v", "/tmp/test.go", path)
	}
}

func TestConvertToGenAI_ToolResult(t *testing.T) {
	msgs := []types.Message{
		types.ToolResultMessage("call_1", "file contents here", false),
	}

	contents := convertToGenAI(msgs)

	if len(contents) != 1 {
		t.Fatalf("expected 1 content, got %d", len(contents))
	}
	c := contents[0]
	if c.Role != "user" {
		t.Errorf("expected role %q, got %q", "user", c.Role)
	}
	if len(c.Parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(c.Parts))
	}
	fr := c.Parts[0].FunctionResponse
	if fr == nil {
		t.Fatal("expected function response part")
	}
	if fr.Name != "call_1" {
		t.Errorf("expected function response name %q, got %q", "call_1", fr.Name)
	}
	output, ok := fr.Response["output"]
	if !ok {
		t.Fatal("expected 'output' in function response")
	}
	if output != "file contents here" {
		t.Errorf("expected output %q, got %v", "file contents here", output)
	}
}

func TestConvertToGenAI_ToolResultError(t *testing.T) {
	msgs := []types.Message{
		types.ToolResultMessage("call_2", "permission denied", true),
	}

	contents := convertToGenAI(msgs)
	c := contents[0]
	fr := c.Parts[0].FunctionResponse
	if fr == nil {
		t.Fatal("expected function response part")
	}
	errVal, ok := fr.Response["error"]
	if !ok {
		t.Fatal("expected 'error' in function response for error result")
	}
	if errVal != "permission denied" {
		t.Errorf("expected error %q, got %v", "permission denied", errVal)
	}
}

func TestConvertToGenAI_SystemMessage(t *testing.T) {
	msgs := []types.Message{
		types.SystemMessage("You are a helpful assistant."),
		types.UserMessage("Hello"),
	}

	contents := convertToGenAI(msgs)

	// System messages should be skipped (handled separately via SystemInstruction)
	if len(contents) != 1 {
		t.Fatalf("expected 1 content (system filtered), got %d", len(contents))
	}
	if contents[0].Role != "user" {
		t.Errorf("expected role %q, got %q", "user", contents[0].Role)
	}
}

func TestConvertFromGenAI_TextResponse(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Hello! How can I help?"},
					},
				},
			},
		},
	}

	result := convertFromGenAI(resp)

	if result.Text != "Hello! How can I help?" {
		t.Errorf("expected text %q, got %q", "Hello! How can I help?", result.Text)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(result.ToolCalls))
	}
}

func TestConvertFromGenAI_ToolCallResponse(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Let me read that file."},
						{
							FunctionCall: &genai.FunctionCall{
								ID:   "call_abc",
								Name: "read_file",
								Args: map[string]any{
									"path": "/tmp/test.go",
								},
							},
						},
					},
				},
			},
		},
	}

	result := convertFromGenAI(resp)

	if result.Text != "Let me read that file." {
		t.Errorf("expected text %q, got %q", "Let me read that file.", result.Text)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.ID != "call_abc" {
		t.Errorf("expected tool call ID %q, got %q", "call_abc", tc.ID)
	}
	if tc.Name != "read_file" {
		t.Errorf("expected tool call name %q, got %q", "read_file", tc.Name)
	}

	var args map[string]any
	if err := json.Unmarshal(tc.Input, &args); err != nil {
		t.Fatalf("failed to unmarshal tool call input: %v", err)
	}
	if args["path"] != "/tmp/test.go" {
		t.Errorf("expected path %q, got %v", "/tmp/test.go", args["path"])
	}
}

func TestConvertFromGenAI_EmptyResponse(t *testing.T) {
	resp := &genai.GenerateContentResponse{}
	result := convertFromGenAI(resp)
	if result.Text != "" {
		t.Errorf("expected empty text, got %q", result.Text)
	}
	if len(result.ToolCalls) != 0 {
		t.Errorf("expected no tool calls, got %d", len(result.ToolCalls))
	}
}

func TestConvertFromGenAI_MultipleToolCalls(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []*genai.Part{
						{
							FunctionCall: &genai.FunctionCall{
								ID:   "call_1",
								Name: "read_file",
								Args: map[string]any{"path": "/a.go"},
							},
						},
						{
							FunctionCall: &genai.FunctionCall{
								ID:   "call_2",
								Name: "read_file",
								Args: map[string]any{"path": "/b.go"},
							},
						},
					},
				},
			},
		},
	}

	result := convertFromGenAI(resp)
	if len(result.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ID != "call_1" {
		t.Errorf("expected first call ID %q, got %q", "call_1", result.ToolCalls[0].ID)
	}
	if result.ToolCalls[1].ID != "call_2" {
		t.Errorf("expected second call ID %q, got %q", "call_2", result.ToolCalls[1].ID)
	}
}

func TestGeminiProvider_Model(t *testing.T) {
	p := &GeminiProvider{modelName: "gemini-2.5-pro"}
	if p.Model() != "gemini-2.5-pro" {
		t.Errorf("expected model %q, got %q", "gemini-2.5-pro", p.Model())
	}
}
