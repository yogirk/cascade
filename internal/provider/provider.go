// Package provider defines the LLM provider abstraction for Cascade.
// ADK Go is used as the LLM client, NOT as the agent loop orchestrator.
// genai types do NOT leak outside the gemini package.
package provider

import (
	"context"

	"github.com/slokam-ai/cascade/pkg/types"
)

// Provider abstracts LLM backends. Gemini is the default implementation.
type Provider interface {
	// GenerateStream sends conversation history to the LLM and returns
	// a streaming response. Tokens arrive on Stream.Tokens() channel.
	// Final response (including tool calls) available via Stream.Result().
	GenerateStream(ctx context.Context, messages []types.Message, tools []Declaration) (*Stream, error)

	// Model returns the model identifier (e.g., "gemini-2.5-pro").
	Model() string
}

// ModelSwitcher is an optional interface for providers that support
// changing the model at runtime (e.g., via /model slash command).
type ModelSwitcher interface {
	SetModel(name string)
}

// Declaration describes a tool for the LLM.
type Declaration struct {
	Name        string
	Description string
	Schema      map[string]any // JSON Schema for parameters
}

// StreamResult holds the final result of a streaming response.
type StreamResult struct {
	Response *types.Response
	Err      error
}

// Stream wraps a streaming LLM response.
type Stream struct {
	tokens chan string
	result chan StreamResult
	cancel func()
}

// NewStream creates a Stream from channels and a cancel function.
func NewStream(tokens chan string, result chan StreamResult, cancel func()) *Stream {
	return &Stream{tokens: tokens, result: result, cancel: cancel}
}

// Tokens returns a channel that emits text tokens as they arrive.
func (s *Stream) Tokens() <-chan string { return s.tokens }

// Result blocks until streaming is complete, then returns the full response
// including any tool calls.
func (s *Stream) Result() (*types.Response, error) {
	r := <-s.result
	return r.Response, r.Err
}

// Cancel aborts the stream.
func (s *Stream) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}
