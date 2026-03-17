package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cascade-cli/cascade/internal/permission"
	"github.com/cascade-cli/cascade/internal/provider"
	"github.com/cascade-cli/cascade/internal/tools"
	"github.com/cascade-cli/cascade/internal/tools/core"
	"github.com/cascade-cli/cascade/pkg/types"
)

// Agent drives the observe-reason-act conversation cycle.
// It connects the LLM provider, tool registry, and permission engine.
type Agent struct {
	provider    provider.Provider
	registry    *tools.Registry
	permissions *permission.Engine
	governor    *Governor
	session     *Session
	events      EventHandler
	maxRetries  int // per tool call error retry, default 2
}

// AgentConfig holds the configuration for creating a new Agent.
type AgentConfig struct {
	Provider     provider.Provider
	Registry     *tools.Registry
	Permissions  *permission.Engine
	MaxToolCalls int
	SystemPrompt string
	Events       EventHandler
}

// New creates a new Agent from the given configuration.
func New(cfg AgentConfig) *Agent {
	maxCalls := cfg.MaxToolCalls
	if maxCalls <= 0 {
		maxCalls = 15
	}
	return &Agent{
		provider:    cfg.Provider,
		registry:    cfg.Registry,
		permissions: cfg.Permissions,
		governor:    NewGovernor(maxCalls),
		session:     NewSession(cfg.SystemPrompt),
		events:      cfg.Events,
		maxRetries:  2,
	}
}

// RunTurn executes a single conversation turn: sends user input to the LLM,
// processes tool calls, and loops until the LLM produces a text-only response
// or governor limits are hit.
func (a *Agent) RunTurn(ctx context.Context, userInput string) error {
	a.session.Append(types.UserMessage(userInput))
	a.governor.Reset()
	toolCallCount := 0

	for {
		// Check governor limit
		if a.governor.CheckLimit(toolCallCount) {
			a.emit(&types.ErrorEvent{Err: fmt.Errorf("reached tool call limit (%d)", a.governor.maxToolCalls)})
			a.emit(&types.DoneEvent{})
			return nil
		}

		// Get tool declarations for LLM
		declarations := a.registry.Declarations()

		// Stream LLM response
		stream, err := a.provider.GenerateStream(ctx, a.session.Messages(), declarations)
		if err != nil {
			a.emit(&types.ErrorEvent{Err: err})
			return err
		}

		// Forward streaming tokens
		go func() {
			for token := range stream.Tokens() {
				a.emit(&types.TokenEvent{Token: token})
			}
		}()

		// Wait for complete response
		response, err := stream.Result()
		if err != nil {
			a.emit(&types.ErrorEvent{Err: err})
			return err
		}

		// Append assistant message to session
		a.session.Append(types.AssistantMessage(response.Text, response.ToolCalls))

		// No tool calls = turn complete
		if len(response.ToolCalls) == 0 {
			a.emit(&types.DoneEvent{})
			return nil
		}

		// Execute each tool call
		for _, call := range response.ToolCalls {
			toolCallCount++

			// Governor: duplicate detection
			if a.governor.IsDuplicate(call.Name, call.Input) {
				a.session.Append(types.ToolResultMessage(call.ID,
					"Duplicate tool call detected. Please try a different approach or ask the user for clarification.", true))
				continue
			}

			// Governor: progress nudge
			if a.governor.ShouldNudge(toolCallCount) {
				a.session.AppendSystem("You have made several tool calls without producing user-facing output. Please provide a progress update or ask for clarification.")
			}

			// Execute with permission check
			result := a.executeWithPermission(ctx, call)
			a.session.Append(types.ToolResultMessage(call.ID, result.Content, result.IsError))
		}
	}
}

// executeWithPermission looks up the tool, checks permissions, and executes.
func (a *Agent) executeWithPermission(ctx context.Context, call types.ToolCall) *tools.Result {
	tool := a.registry.Get(call.Name)
	if tool == nil {
		return &tools.Result{
			Content: fmt.Sprintf("Unknown tool: %s. Available tools: use the tools provided.", call.Name),
			IsError: true,
		}
	}

	a.emit(&types.ToolStartEvent{Name: call.Name, Input: call.Input})

	// Dynamic risk classification for bash
	var riskTool permission.ToolRiskProvider = tool
	if call.Name == "bash" {
		var input struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(call.Input, &input); err == nil {
			riskTool = &dynamicRiskTool{Tool: tool, risk: core.ClassifyBashRisk(input.Command)}
		}
	}

	// Permission check
	decision := a.permissions.Check(riskTool, call.Input)
	switch decision {
	case permission.Deny:
		result := &tools.Result{
			Content: fmt.Sprintf("Permission denied: %s is not allowed in %s mode.",
				call.Name, a.permissions.Mode()),
			IsError: true,
		}
		a.emit(&types.ToolEndEvent{Name: call.Name, Content: result.Content, IsError: true})
		return result

	case permission.Confirm:
		// Emit permission request and wait for response
		responseCh := make(chan bool, 1)
		a.emit(&types.PermissionRequestEvent{
			ToolName:  call.Name,
			Input:     call.Input,
			RiskLevel: riskTool.RiskLevel().String(),
			Response:  responseCh,
		})
		approved := <-responseCh
		if !approved {
			result := &tools.Result{
				Content: fmt.Sprintf("User denied permission for %s.", call.Name),
				IsError: true,
			}
			a.emit(&types.ToolEndEvent{Name: call.Name, Content: result.Content, IsError: true})
			return result
		}
		// Cache the approval for this tool+args
		a.permissions.CacheDecision(call.Name, call.Input, permission.Allow)
	}

	// Execute the tool
	result, err := tool.Execute(ctx, call.Input)
	if err != nil {
		errResult := &tools.Result{
			Content: fmt.Sprintf("Tool error: %v", err),
			IsError: true,
		}
		a.emit(&types.ToolEndEvent{Name: call.Name, Content: errResult.Content, IsError: true, Err: err})
		return errResult
	}

	a.emit(&types.ToolEndEvent{Name: call.Name, Content: result.Content, IsError: result.IsError})
	return result
}

// emit sends an event to the event handler if one is configured.
func (a *Agent) emit(event types.Event) {
	if a.events != nil {
		a.events.HandleEvent(event)
	}
}

// Session returns the agent's session for external access.
func (a *Agent) Session() *Session { return a.session }

// dynamicRiskTool wraps a tool with a dynamically-determined risk level.
type dynamicRiskTool struct {
	tools.Tool
	risk permission.RiskLevel
}

func (d *dynamicRiskTool) RiskLevel() permission.RiskLevel { return d.risk }
