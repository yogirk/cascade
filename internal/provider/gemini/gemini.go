// Package gemini implements the Provider interface using Google's GenAI SDK.
// genai types are confined to this package — all other code uses pkg/types.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/slokam-ai/cascade/internal/provider"
	"github.com/slokam-ai/cascade/pkg/types"
	"google.golang.org/genai"
)

// GeminiProvider wraps the GenAI SDK client behind the Provider interface.
type GeminiProvider struct {
	client    *genai.Client
	mu        sync.RWMutex
	modelName string
}

// New creates a GeminiProvider. If clientConfig is nil, an empty config is
// used and the SDK auto-detects the backend from environment variables:
//
//   - GOOGLE_API_KEY → Gemini API (AI Studio, no project needed)
//   - GOOGLE_GENAI_USE_VERTEXAI=true + GOOGLE_CLOUD_PROJECT → Vertex AI with ADC
//
// See: https://pkg.go.dev/google.golang.org/genai#BackendUnspecified
func New(ctx context.Context, modelName string, clientConfig *genai.ClientConfig) (*GeminiProvider, error) {
	if clientConfig == nil {
		clientConfig = &genai.ClientConfig{}
	}

	client, err := genai.NewClient(ctx, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &GeminiProvider{
		client:    client,
		modelName: modelName,
	}, nil
}

// Model returns the configured model identifier.
func (g *GeminiProvider) Model() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.modelName
}

// SetModel updates the model identifier.
func (g *GeminiProvider) SetModel(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.modelName = name
}

// GenerateStream converts internal types to genai SDK types, executes a single model and returns
// a streaming response. Tokens arrive on Stream.Tokens() channel.
func (g *GeminiProvider) GenerateStream(ctx context.Context, messages []types.Message, tools []provider.Declaration) (*provider.Stream, error) {
	contents := convertToGenAI(messages)
	config := buildConfig(messages, tools)

	// Capture model name under lock to avoid racing with SetModel.
	g.mu.RLock()
	model := g.modelName
	g.mu.RUnlock()

	tokens := make(chan string, 256)
	result := make(chan provider.StreamResult, 1)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(tokens)

		var textParts []string
		var toolCalls []types.ToolCall
		var usage *types.Usage

		for resp, err := range g.client.Models.GenerateContentStream(ctx, model, contents, config) {
			if err != nil {
				result <- provider.StreamResult{Err: fmt.Errorf("streaming error: %w", err)}
				return
			}

			// Capture usage metadata (typically on the last chunk)
			if resp.UsageMetadata != nil {
				usage = &types.Usage{
					PromptTokens:     resp.UsageMetadata.PromptTokenCount,
					CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
					TotalTokens:      resp.UsageMetadata.TotalTokenCount,
				}
			}

			if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
				continue
			}

			for _, part := range resp.Candidates[0].Content.Parts {
				if part.Text != "" && !part.Thought {
					// Send token via non-blocking send to prevent deadlock
					select {
					case tokens <- part.Text:
					case <-ctx.Done():
						result <- provider.StreamResult{Err: ctx.Err()}
						return
					}
					textParts = append(textParts, part.Text)
				}

				if part.FunctionCall != nil {
					tc, err := convertFunctionCall(part.FunctionCall)
					if err != nil {
						result <- provider.StreamResult{Err: err}
						return
					}
					// Preserve thought signature for thinking models (Gemini 3+)
					tc.ThoughtSignature = part.ThoughtSignature
					toolCalls = append(toolCalls, tc)
				}
			}
		}

		result <- provider.StreamResult{
			Response: &types.Response{
				Text:      strings.Join(textParts, ""),
				ToolCalls: toolCalls,
				Usage:     usage,
			},
		}
	}()

	return provider.NewStream(tokens, result, cancel), nil
}

// buildConfig creates a GenerateContentConfig from messages and tool declarations.
func buildConfig(messages []types.Message, tools []provider.Declaration) *genai.GenerateContentConfig {
	config := &genai.GenerateContentConfig{}

	// Extract system instruction from messages
	for _, msg := range messages {
		if msg.Role == types.RoleSystem && strings.TrimSpace(msg.Content) != "" {
			config.SystemInstruction = &genai.Content{
				Parts: []*genai.Part{
					{Text: msg.Content},
				},
			}
			break // Only one system instruction supported
		}
	}

	// Convert tool declarations
	if len(tools) > 0 {
		var funcDecls []*genai.FunctionDeclaration
		for _, tool := range tools {
			fd := &genai.FunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
			}
			if tool.Schema != nil {
				fd.ParametersJsonSchema = tool.Schema
			}
			funcDecls = append(funcDecls, fd)
		}
		config.Tools = []*genai.Tool{
			{FunctionDeclarations: funcDecls},
		}
	}

	return config
}

// convertToGenAI converts Cascade messages to GenAI Content format.
// System messages are filtered out (handled via SystemInstruction in config).
func convertToGenAI(msgs []types.Message) []*genai.Content {
	var contents []*genai.Content

	for _, msg := range msgs {
		switch msg.Role {
		case types.RoleSystem:
			// System messages handled separately via GenerateContentConfig.SystemInstruction
			continue

		case types.RoleUser:
			if strings.TrimSpace(msg.Content) == "" {
				continue
			}
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: msg.Content}},
			})

		case types.RoleAssistant:
			c := &genai.Content{Role: "model"}
			if strings.TrimSpace(msg.Content) != "" {
				c.Parts = append(c.Parts, &genai.Part{Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				args := make(map[string]any)
				if len(tc.Input) > 0 {
					_ = json.Unmarshal(tc.Input, &args)
				}
				c.Parts = append(c.Parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   tc.ID,
						Name: tc.Name,
						Args: args,
					},
					// Echo back thought signature for thinking models (Gemini 3+)
					ThoughtSignature: tc.ThoughtSignature,
				})
			}
			if len(c.Parts) > 0 {
				contents = append(contents, c)
			}

		case types.RoleTool:
			if msg.ToolResult != nil {
				respMap := make(map[string]any)
				if msg.ToolResult.IsError {
					respMap["error"] = msg.ToolResult.Content
				} else {
					respMap["output"] = msg.ToolResult.Content
				}
				contents = append(contents, &genai.Content{
					Role: "user",
					Parts: []*genai.Part{
						{
							FunctionResponse: &genai.FunctionResponse{
								Name:     msg.ToolResult.Name,
								Response: respMap,
							},
						},
					},
				})
			}
		}
	}

	return contents
}

// convertFromGenAI converts a GenAI response to a Cascade Response.
func convertFromGenAI(resp *genai.GenerateContentResponse) *types.Response {
	result := &types.Response{}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return result
	}

	var textParts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
		if part.FunctionCall != nil {
			tc, err := convertFunctionCall(part.FunctionCall)
			if err == nil {
				result.ToolCalls = append(result.ToolCalls, tc)
			}
		}
	}
	result.Text = strings.Join(textParts, "")

	return result
}

// convertFunctionCall converts a genai FunctionCall to a types.ToolCall.
func convertFunctionCall(fc *genai.FunctionCall) (types.ToolCall, error) {
	inputJSON, err := json.Marshal(fc.Args)
	if err != nil {
		return types.ToolCall{}, fmt.Errorf("failed to marshal function call args: %w", err)
	}
	return types.ToolCall{
		ID:    fc.ID,
		Name:  fc.Name,
		Input: inputJSON,
	}, nil
}
