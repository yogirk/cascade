package testutil

import (
	"context"
	"errors"
	"testing"

	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

// Compile-time interface check
var _ provider.Provider = (*MockProvider)(nil)

func TestMockProvider_GenerateStream_Tokens(t *testing.T) {
	mock := &MockProvider{
		ModelName: "test-model",
		Tokens:    []string{"Hello", ", ", "world", "!"},
		Response: &types.Response{
			Text: "Hello, world!",
		},
	}

	stream, err := mock.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var collected []string
	for token := range stream.Tokens() {
		collected = append(collected, token)
	}

	if len(collected) != 4 {
		t.Fatalf("expected 4 tokens, got %d", len(collected))
	}
	if collected[0] != "Hello" {
		t.Errorf("expected first token %q, got %q", "Hello", collected[0])
	}
	if collected[3] != "!" {
		t.Errorf("expected last token %q, got %q", "!", collected[3])
	}
}

func TestMockProvider_GenerateStream_Result(t *testing.T) {
	expectedToolCalls := []types.ToolCall{
		{ID: "call_1", Name: "read_file"},
	}
	mock := &MockProvider{
		Tokens: []string{"Let me check."},
		Response: &types.Response{
			Text:      "Let me check.",
			ToolCalls: expectedToolCalls,
		},
	}

	stream, err := mock.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Drain tokens
	for range stream.Tokens() {
	}

	resp, err := stream.Result()
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "Let me check." {
		t.Errorf("expected text %q, got %q", "Let me check.", resp.Text)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "read_file" {
		t.Errorf("expected tool call name %q, got %q", "read_file", resp.ToolCalls[0].Name)
	}
}

func TestMockProvider_GenerateStream_Error(t *testing.T) {
	mock := &MockProvider{
		Err: errors.New("model unavailable"),
	}

	_, err := mock.GenerateStream(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "model unavailable" {
		t.Errorf("expected error %q, got %q", "model unavailable", err.Error())
	}
}

func TestMockProvider_GenerateStream_Cancel(t *testing.T) {
	mock := &MockProvider{
		Tokens:   []string{"a", "b", "c", "d", "e"},
		Response: &types.Response{Text: "abcde"},
	}

	stream, err := mock.GenerateStream(context.Background(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Read one token then cancel
	<-stream.Tokens()
	stream.Cancel()

	// Tokens channel should eventually close after cancel
	remaining := 0
	for range stream.Tokens() {
		remaining++
	}
	// We may get some remaining tokens from the buffer, that's okay
	// The important thing is the channel closes
	_ = remaining
}

func TestMockProvider_Model(t *testing.T) {
	mock := &MockProvider{ModelName: "gemini-2.5-pro"}
	if mock.Model() != "gemini-2.5-pro" {
		t.Errorf("expected %q, got %q", "gemini-2.5-pro", mock.Model())
	}
}
