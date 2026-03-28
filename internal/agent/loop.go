package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/internal/tools"
	"github.com/yogirk/cascade/pkg/types"
)

// Agent drives the observe-reason-act conversation cycle.
// It connects the LLM provider, tool registry, and permission engine.
type Agent struct {
	provider         provider.Provider
	registry         *tools.Registry
	permissions      *permission.Engine
	approvals        chan<- types.ApprovalRequest
	contextInjector  func(string) string
	governor         *Governor
	session          *Session
	events           EventHandler
	maxRetries       int // per tool call error retry, default 2
	toolTimeout      time.Duration
	lastPromptTokens int32
}

// AgentConfig holds the configuration for creating a new Agent.
type AgentConfig struct {
	Provider     provider.Provider
	Registry     *tools.Registry
	Permissions  *permission.Engine
	MaxToolCalls int
	ToolTimeout  int // seconds; 0 means default (120s)
	SystemPrompt string
	Events       EventHandler
	Approvals    chan<- types.ApprovalRequest
	ContextInjector func(string) string
}

// New creates a new Agent from the given configuration.
func New(cfg AgentConfig) *Agent {
	maxCalls := cfg.MaxToolCalls
	if maxCalls <= 0 {
		maxCalls = 200
	}
	timeout := time.Duration(cfg.ToolTimeout) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &Agent{
		provider:    cfg.Provider,
		registry:    cfg.Registry,
		permissions: cfg.Permissions,
		approvals:   cfg.Approvals,
		contextInjector: cfg.ContextInjector,
		governor:    NewGovernor(maxCalls),
		session:     NewSession(cfg.SystemPrompt),
		events:      cfg.Events,
		maxRetries:  2,
		toolTimeout: timeout,
	}
}

// RunTurn executes a single conversation turn: sends user input to the LLM,
// processes tool calls, and loops until the LLM produces a text-only response
// or governor limits are hit.
func (a *Agent) RunTurn(ctx context.Context, userInput string) error {
	a.emit(&types.TurnStartEvent{Input: userInput})
	a.session.Append(types.UserMessage(userInput))
	a.governor.Reset()
	toolCallCount := 0

	// Compute context injection once per turn (not per loop iteration).
	// Schema context is derived from userInput which is constant within a turn.
	var injectedContext string
	if a.contextInjector != nil {
		injectedContext = a.contextInjector(userInput)
	}

	for {
		// Auto-compact if context window is filling up (>= 80%)
		if a.lastPromptTokens > 0 && ShouldCompact(a.lastPromptTokens, a.provider.Model(), 80.0) {
			newMessages, _, err := CompactSession(ctx, a.provider, a.session.Messages(), 6)
			if err == nil {
				beforeTokens := a.lastPromptTokens
				a.session.Replace(newMessages)
				a.session.NotifySave()
				a.emit(&types.CompactEvent{BeforeTokens: beforeTokens, AfterTokens: 0})
				// AfterTokens will be updated on next LLM response
			}
		}

		// Check governor limit
		if a.governor.CheckLimit(toolCallCount) {
			a.emit(&types.ErrorEvent{Err: fmt.Errorf("reached tool call limit (%d)", a.governor.maxToolCalls)})
			a.emit(&types.DoneEvent{})
			return nil
		}

		// Get tool declarations for LLM
		declarations := a.registry.Declarations()

		// Stream LLM response
		a.emit(&types.StreamStartEvent{})
		messages := a.session.Messages()
		if injectedContext != "" {
			if len(messages) > 0 && messages[0].Role == types.RoleSystem {
				enriched := make([]types.Message, 0, len(messages)+1)
				enriched = append(enriched, messages[0], types.SystemMessage(injectedContext))
				enriched = append(enriched, messages[1:]...)
				messages = enriched
			} else {
				messages = append([]types.Message{types.SystemMessage(injectedContext)}, messages...)
			}
		}

		stream, err := a.provider.GenerateStream(ctx, messages, declarations)
		if err != nil {
			a.emit(&types.ErrorEvent{Err: err})
			return err
		}

		// Forward streaming tokens
		tokensDone := make(chan struct{})
		go func() {
			defer close(tokensDone)
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

		// Wait for all tokens to be forwarded before signaling completion
		<-tokensDone

		// Track context usage for auto-compaction
		if response.Usage != nil {
			a.lastPromptTokens = response.Usage.PromptTokens
		}

		// Avoid recording an empty assistant turn; Vertex rejects model content with no parts.
		if response.Text != "" || len(response.ToolCalls) > 0 {
			a.session.Append(types.AssistantMessage(response.Text, response.ToolCalls))
		}

		// Emit StreamComplete event before processing tool calls
		a.emit(&types.StreamCompleteEvent{Content: response.Text, Usage: response.Usage})

		// No tool calls = turn complete
		if len(response.ToolCalls) == 0 {
			a.emit(&types.DoneEvent{})
			a.session.NotifySave()
			return nil
		}

		// Execute each tool call
		for _, call := range response.ToolCalls {
			toolCallCount++

			// Governor: duplicate detection
			if a.governor.IsDuplicate(call.Name, call.Input) {
				a.session.Append(types.ToolResultMessage(call.ID, call.Name,
					"Duplicate tool call detected. Please try a different approach or ask the user for clarification.", true))
				continue
			}

			// Governor: progress nudge
			if a.governor.ShouldNudge(toolCallCount) {
				a.session.AppendSystem("You have made several tool calls without producing user-facing output. Please provide a progress update or ask for clarification.")
			}

			// Execute with permission check
			result := a.executeWithPermission(ctx, call)
			a.session.Append(types.ToolResultMessage(call.ID, call.Name, result.Content, result.IsError))
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

	a.emit(&types.ToolStartEvent{Name: call.Name, Input: call.Input, RiskLevel: tool.RiskLevel().String()})

	// Permission check — tools implement PermissionPlanner for input-aware risk classification
	var riskTool permission.ToolRiskProvider = tool
	baseRisk := riskTool.RiskLevel()
	if planner, ok := tool.(tools.PermissionPlanner); ok {
		plan, err := planner.PlanPermission(ctx, call.Input, baseRisk)
		if err != nil {
			result := &tools.Result{
				Content: fmt.Sprintf("Permission planning failed for %s: %v", call.Name, err),
				IsError: true,
			}
			a.emit(&types.ToolEndEvent{Name: call.Name, Content: result.Content, IsError: true})
			return result
		}
		if plan != nil {
			if plan.DenyMessage != "" {
				result := &tools.Result{
					Content: plan.DenyMessage,
					Display: plan.DenyMessage,
					IsError: true,
				}
				a.emit(&types.ToolEndEvent{Name: call.Name, Content: result.Content, Display: result.Display, IsError: true})
				return result
			}
			if plan.RiskOverride != nil {
				riskTool = &dynamicRiskTool{Tool: tool, risk: *plan.RiskOverride}
			}
		}
	}

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
		if a.approvals == nil {
			return &tools.Result{
				Content: "Permission check requires approval handling, but no approver is configured.",
				IsError: true,
			}
		}

		// Send a dedicated approval request instead of using the generic event stream.
		responseCh := make(chan types.ApprovalDecision, 1)
		request := types.ApprovalRequest{
			ToolName:  call.Name,
			Input:     call.Input,
			RiskLevel: riskTool.RiskLevel().String(),
			Response:  responseCh,
		}
		select {
		case a.approvals <- request:
		case <-ctx.Done():
			return &tools.Result{
				Content: fmt.Sprintf("Permission request cancelled for %s.", call.Name),
				IsError: true,
			}
		}

		var decision types.ApprovalDecision
		select {
		case decision = <-responseCh:
		case <-ctx.Done():
			return &tools.Result{
				Content: fmt.Sprintf("Permission request cancelled for %s.", call.Name),
				IsError: true,
			}
		}
		switch decision.Action {
		case types.ApprovalAllowOnce:
			// proceed without caching
		case types.ApprovalAllowToolSession:
			a.permissions.CacheToolDecision(call.Name, permission.Allow)
		default:
			result := &tools.Result{
				Content: fmt.Sprintf("User denied permission for %s.", call.Name),
				IsError: true,
			}
			a.emit(&types.ToolEndEvent{Name: call.Name, Content: result.Content, IsError: true})
			return result
		}
	}

	// Execute the tool with timeout
	execCtx, cancel := context.WithTimeout(ctx, a.toolTimeout)
	defer cancel()
	result, err := tool.Execute(execCtx, call.Input)
	if err != nil {
		errResult := &tools.Result{
			Content: fmt.Sprintf("Tool error: %v", err),
			IsError: true,
		}
		a.emit(&types.ToolEndEvent{Name: call.Name, Content: errResult.Content, Display: errResult.Content, IsError: true, Err: err})
		return errResult
	}

	a.emit(&types.ToolEndEvent{Name: call.Name, Content: result.Content, Display: result.Display, IsError: result.IsError})
	return result
}

// emit sends an event to the event handler if one is configured.
func (a *Agent) emit(event types.Event) {
	if a.events != nil {
		a.events.HandleEvent(event)
	}
}

// Compact triggers manual session compaction.
func (a *Agent) Compact(ctx context.Context) error {
	newMessages, _, err := CompactSession(ctx, a.provider, a.session.Messages(), 6)
	if err != nil {
		return err
	}
	beforeTokens := a.lastPromptTokens
	a.session.Replace(newMessages)
	a.session.NotifySave()
	a.emit(&types.CompactEvent{BeforeTokens: beforeTokens, AfterTokens: 0})
	return nil
}

// Session returns the agent's session for external access.
func (a *Agent) Session() *Session { return a.session }

// dynamicRiskTool wraps a tool with a dynamically-determined risk level.
type dynamicRiskTool struct {
	tools.Tool
	risk permission.RiskLevel
}

func (d *dynamicRiskTool) RiskLevel() permission.RiskLevel { return d.risk }
