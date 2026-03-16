// Package testutil provides mock implementations for testing.
package testutil

import (
	"context"

	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

// MockProvider implements provider.Provider for testing.
type MockProvider struct {
	ModelName string
	Tokens    []string        // Tokens to emit during streaming
	Response  *types.Response // Final response after streaming
	Err       error           // Error to return from GenerateStream
}

var _ provider.Provider = (*MockProvider)(nil)      // Compile-time check
var _ provider.ModelSwitcher = (*MockProvider)(nil) // Compile-time check

// Model returns the configured model name.
func (m *MockProvider) Model() string {
	return m.ModelName
}

// SetModel updates the configured model name.
func (m *MockProvider) SetModel(name string) {
	m.ModelName = name
}

// GenerateStream returns a stream that emits preconfigured tokens and response.
func (m *MockProvider) GenerateStream(ctx context.Context, messages []types.Message, tools []provider.Declaration) (*provider.Stream, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	tokens := make(chan string, 256)
	result := make(chan provider.StreamResult, 1)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(tokens)
		for _, t := range m.Tokens {
			select {
			case tokens <- t:
			case <-ctx.Done():
				result <- provider.StreamResult{Err: ctx.Err()}
				return
			}
		}
		result <- provider.StreamResult{Response: m.Response}
	}()

	return provider.NewStream(tokens, result, cancel), nil
}
