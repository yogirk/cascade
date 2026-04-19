// Package anthropic implements the Provider interface using the Anthropic Claude API.
// Anthropic types are confined to this package — all other code uses pkg/types.
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	ant "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

// AnthropicProvider wraps the Anthropic client behind the Provider interface.
type AnthropicProvider struct {
	client    *ant.Client
	mu        sync.RWMutex
	modelName string
}

// New creates an AnthropicProvider using the given API key env var.
// If apiKeyEnv is empty, defaults to ANTHROPIC_API_KEY.
func New(modelName, apiKeyEnv string) (*AnthropicProvider, error) {
	if apiKeyEnv == "" {
		apiKeyEnv = "ANTHROPIC_API_KEY"
	}
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("anthropic API key not found: set %s environment variable", apiKeyEnv)
	}

	client := ant.NewClient(option.WithAPIKey(apiKey))

	return &AnthropicProvider{
		client:    &client,
		modelName: modelName,
	}, nil
}

// Model returns the configured model identifier.
func (p *AnthropicProvider) Model() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.modelName
}

// SetModel updates the model identifier.
func (p *AnthropicProvider) SetModel(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.modelName = name
}

// GenerateStream converts internal types to Anthropic format, executes a streaming
// message request, and returns tokens via the Stream channel.
func (p *AnthropicProvider) GenerateStream(ctx context.Context, messages []types.Message, tools []provider.Declaration) (*provider.Stream, error) {
	antMessages, systemPrompt := convertMessages(messages)
	antTools := convertTools(tools)

	p.mu.RLock()
	model := p.modelName
	p.mu.RUnlock()

	params := ant.MessageNewParams{
		Model:     model,
		Messages:  antMessages,
		MaxTokens: 8192,
	}
	if systemPrompt != "" {
		params.System = []ant.TextBlockParam{
			{Text: systemPrompt},
		}
	}
	if len(antTools) > 0 {
		params.Tools = antTools
	}

	tokens := make(chan string, 256)
	result := make(chan provider.StreamResult, 1)
	ctx, cancel := context.WithCancel(ctx)

	go func() {
		defer close(tokens)

		stream := p.client.Messages.NewStreaming(ctx, params)
		msg := ant.Message{}

		var textParts []string

		for stream.Next() {
			event := stream.Current()
			msg.Accumulate(event)

			// Stream text tokens from content_block_delta events
			switch e := event.AsAny().(type) {
			case ant.ContentBlockDeltaEvent:
				if e.Delta.Type == "text_delta" {
					text := e.Delta.Text
					if text != "" {
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
		}

		if err := stream.Err(); err != nil {
			result <- provider.StreamResult{Err: fmt.Errorf("streaming error: %w", err)}
			return
		}

		// Extract tool calls from accumulated message
		var toolCalls []types.ToolCall
		for _, block := range msg.Content {
			if block.Type == "tool_use" {
				inputJSON, _ := json.Marshal(block.Input)
				toolCalls = append(toolCalls, types.ToolCall{
					ID:    block.ID,
					Name:  block.Name,
					Input: inputJSON,
				})
			}
		}

		var usage *types.Usage
		if msg.Usage.InputTokens > 0 || msg.Usage.OutputTokens > 0 {
			total := msg.Usage.InputTokens + msg.Usage.OutputTokens
			usage = &types.Usage{
				PromptTokens:     int32(msg.Usage.InputTokens),
				CompletionTokens: int32(msg.Usage.OutputTokens),
				TotalTokens:      int32(total),
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

// convertMessages converts Cascade messages to Anthropic format.
// System messages are extracted and returned separately (Anthropic uses a top-level system param).
// Tool results must be sent as user messages with tool_result content blocks.
func convertMessages(msgs []types.Message) ([]ant.MessageParam, string) {
	var antMsgs []ant.MessageParam
	var systemPrompt string

	for _, msg := range msgs {
		switch msg.Role {
		case types.RoleSystem:
			if strings.TrimSpace(msg.Content) != "" {
				systemPrompt = msg.Content
			}

		case types.RoleUser:
			if strings.TrimSpace(msg.Content) != "" {
				antMsgs = append(antMsgs, ant.NewUserMessage(
					ant.NewTextBlock(msg.Content),
				))
			}

		case types.RoleAssistant:
			var blocks []ant.ContentBlockParamUnion
			if strings.TrimSpace(msg.Content) != "" {
				blocks = append(blocks, ant.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				var input map[string]interface{}
				if len(tc.Input) > 0 {
					_ = json.Unmarshal(tc.Input, &input)
				}
				blocks = append(blocks, ant.NewToolUseBlock(tc.ID, input, tc.Name))
			}
			if len(blocks) > 0 {
				antMsgs = append(antMsgs, ant.NewAssistantMessage(blocks...))
			}

		case types.RoleTool:
			if msg.ToolResult != nil {
				// Anthropic tool results are sent as user messages with tool_result blocks
				antMsgs = append(antMsgs, ant.NewUserMessage(
					ant.NewToolResultBlock(msg.ToolResult.CallID, msg.ToolResult.Content, msg.ToolResult.IsError),
				))
			}
		}
	}

	return antMsgs, systemPrompt
}

// convertTools converts Cascade tool declarations to Anthropic format.
func convertTools(tools []provider.Declaration) []ant.ToolUnionParam {
	var antTools []ant.ToolUnionParam

	for _, tool := range tools {
		schemaJSON, _ := json.Marshal(tool.Schema)
		var inputSchema ant.ToolInputSchemaParam
		_ = json.Unmarshal(schemaJSON, &inputSchema)
		// Ensure type is set
		inputSchema.Type = "object"

		tp := ant.ToolParam{
			Name:        tool.Name,
			Description: ant.String(tool.Description),
			InputSchema: inputSchema,
		}
		antTools = append(antTools, ant.ToolUnionParam{OfTool: &tp})
	}

	return antTools
}

// Ensure interface compliance.
var (
	_ provider.Provider      = (*AnthropicProvider)(nil)
	_ provider.ModelSwitcher = (*AnthropicProvider)(nil)
	_ io.Closer              = (*AnthropicProvider)(nil)
)

// Close is a no-op for Anthropic (HTTP client doesn't need closing).
func (p *AnthropicProvider) Close() error { return nil }
