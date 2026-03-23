// Package openai implements the Provider interface using the OpenAI API.
// OpenAI types are confined to this package — all other code uses pkg/types.
package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	oai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

// OpenAIProvider wraps the OpenAI client behind the Provider interface.
type OpenAIProvider struct {
	client    *oai.Client
	mu        sync.RWMutex
	modelName string
}

// New creates an OpenAIProvider using the given API key env var.
// If apiKeyEnv is empty, defaults to OPENAI_API_KEY.
func New(modelName, apiKeyEnv string) (*OpenAIProvider, error) {
	if apiKeyEnv == "" {
		apiKeyEnv = "OPENAI_API_KEY"
	}
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not found: set %s environment variable", apiKeyEnv)
	}

	client := oai.NewClient(option.WithAPIKey(apiKey))

	return &OpenAIProvider{
		client:    &client,
		modelName: modelName,
	}, nil
}

// Model returns the configured model identifier.
func (p *OpenAIProvider) Model() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modelName
}

// SetModel updates the model identifier.
func (p *OpenAIProvider) SetModel(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.modelName = name
}

// GenerateStream converts internal types to OpenAI format, executes a streaming
// chat completion, and returns tokens via the Stream channel.
func (p *OpenAIProvider) GenerateStream(ctx context.Context, messages []types.Message, tools []provider.Declaration) (*provider.Stream, error) {
	oaiMessages := convertMessages(messages)
	oaiTools := convertTools(tools)

	p.mu.RLock()
	model := p.modelName
	p.mu.RUnlock()

	params := oai.ChatCompletionNewParams{
		Model:    model,
		Messages: oaiMessages,
	}
	if len(oaiTools) > 0 {
		params.Tools = oaiTools
	}

	tokens := make(chan string, 256)
	result := make(chan provider.StreamResult, 1)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(tokens)

		stream := p.client.Chat.Completions.NewStreaming(ctx, params)
		acc := oai.ChatCompletionAccumulator{}

		var textParts []string

		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)

			// Stream text tokens
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					text := choice.Delta.Content
					select {
					case tokens <- text:
					case <-ctx.Done():
						result <- provider.StreamResult{Err: ctx.Err()}
						return
					}
					textParts = append(textParts, text)
				}
			}
		}

		if err := stream.Err(); err != nil {
			result <- provider.StreamResult{Err: fmt.Errorf("streaming error: %w", err)}
			return
		}

		// Extract final response from accumulator
		completion := acc.Choices[0]
		var toolCalls []types.ToolCall
		for _, tc := range completion.Message.ToolCalls {
			inputJSON := json.RawMessage(tc.Function.Arguments)
			toolCalls = append(toolCalls, types.ToolCall{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputJSON,
			})
		}

		var usage *types.Usage
		if acc.Usage.TotalTokens > 0 {
			usage = &types.Usage{
				PromptTokens:     int32(acc.Usage.PromptTokens),
				CompletionTokens: int32(acc.Usage.CompletionTokens),
				TotalTokens:      int32(acc.Usage.TotalTokens),
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

// convertMessages converts Cascade messages to OpenAI chat messages.
func convertMessages(msgs []types.Message) []oai.ChatCompletionMessageParamUnion {
	var oaiMsgs []oai.ChatCompletionMessageParamUnion

	for _, msg := range msgs {
		switch msg.Role {
		case types.RoleSystem:
			if strings.TrimSpace(msg.Content) != "" {
				oaiMsgs = append(oaiMsgs, oai.SystemMessage(msg.Content))
			}

		case types.RoleUser:
			if strings.TrimSpace(msg.Content) != "" {
				oaiMsgs = append(oaiMsgs, oai.UserMessage(msg.Content))
			}

		case types.RoleAssistant:
			if len(msg.ToolCalls) > 0 {
				// Assistant message with tool calls
				var toolCalls []oai.ChatCompletionMessageToolCallUnionParam
				for _, tc := range msg.ToolCalls {
					toolCalls = append(toolCalls, oai.ChatCompletionMessageToolCallUnionParam{
						OfFunction: &oai.ChatCompletionMessageFunctionToolCallParam{
							ID: tc.ID,
							Function: oai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      tc.Name,
								Arguments: string(tc.Input),
							},
						},
					})
				}
				assistant := oai.ChatCompletionAssistantMessageParam{
					Content: oai.ChatCompletionAssistantMessageParamContentUnion{
						OfString: oai.String(msg.Content),
					},
					ToolCalls: toolCalls,
				}
				oaiMsgs = append(oaiMsgs, oai.ChatCompletionMessageParamUnion{OfAssistant: &assistant})
			} else if strings.TrimSpace(msg.Content) != "" {
				oaiMsgs = append(oaiMsgs, oai.AssistantMessage(msg.Content))
			}

		case types.RoleTool:
			if msg.ToolResult != nil {
				oaiMsgs = append(oaiMsgs, oai.ToolMessage(msg.ToolResult.CallID, msg.ToolResult.Content))
			}
		}
	}

	return oaiMsgs
}

// convertTools converts Cascade tool declarations to OpenAI format.
func convertTools(tools []provider.Declaration) []oai.ChatCompletionToolUnionParam {
	var oaiTools []oai.ChatCompletionToolUnionParam

	for _, tool := range tools {
		schemaJSON, _ := json.Marshal(tool.Schema)
		var schemaParam shared.FunctionParameters
		_ = json.Unmarshal(schemaJSON, &schemaParam)

		oaiTools = append(oaiTools, oai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: oai.String(tool.Description),
			Parameters:  schemaParam,
		}))
	}

	return oaiTools
}

// Ensure interface compliance.
var (
	_ provider.Provider      = (*OpenAIProvider)(nil)
	_ provider.ModelSwitcher = (*OpenAIProvider)(nil)
	_ io.Closer              = (*OpenAIProvider)(nil)
)

// Close is a no-op for OpenAI (HTTP client doesn't need closing).
func (p *OpenAIProvider) Close() error { return nil }
