package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cascade-cli/cascade/internal/permission"
	"github.com/cascade-cli/cascade/internal/provider"
	"github.com/cascade-cli/cascade/internal/tools"
	"github.com/cascade-cli/cascade/pkg/types"
)

// --- Test helpers ---

// multiMockProvider returns different responses on successive GenerateStream calls.
type multiMockProvider struct {
	mu        sync.Mutex
	responses []mockResponse
	callIdx   int
}

type mockResponse struct {
	tokens   []string
	response *types.Response
	err      error
}

func (m *multiMockProvider) Model() string { return "test-model" }

func (m *multiMockProvider) GenerateStream(_ context.Context, _ []types.Message, _ []provider.Declaration) (*provider.Stream, error) {
	m.mu.Lock()
	idx := m.callIdx
	if idx >= len(m.responses) {
		idx = len(m.responses) - 1 // reuse last response if overflows
	}
	m.callIdx++
	resp := m.responses[idx]
	m.mu.Unlock()

	if resp.err != nil {
		return nil, resp.err
	}

	tokens := make(chan string, 256)
	result := make(chan provider.StreamResult, 1)
	go func() {
		defer close(tokens)
		for _, t := range resp.tokens {
			tokens <- t
		}
		result <- provider.StreamResult{Response: resp.response}
	}()
	return provider.NewStream(tokens, result, func() {}), nil
}

// mockTool implements tools.Tool with predetermined results.
type mockTool struct {
	name      string
	riskLevel permission.RiskLevel
	result    *tools.Result
	err       error
}

func (t *mockTool) Name() string                     { return t.name }
func (t *mockTool) Description() string              { return "Mock tool for testing" }
func (t *mockTool) InputSchema() map[string]any      { return map[string]any{"type": "object"} }
func (t *mockTool) RiskLevel() permission.RiskLevel   { return t.riskLevel }
func (t *mockTool) Execute(_ context.Context, _ json.RawMessage) (*tools.Result, error) {
	return t.result, t.err
}

// collectEvents creates an event handler that collects all events.
// Call collect() AFTER RunTurn returns. It drains remaining async events
// (from token-forwarding goroutines) using a short timeout.
func collectEvents(buf int) (EventHandler, func() []types.Event) {
	ch := make(chan types.Event, buf)
	handler := EventChan(ch)

	collect := func() []types.Event {
		// Give async goroutines a moment to flush remaining events
		var events []types.Event
		deadline := time.After(200 * time.Millisecond)
		for {
			select {
			case evt := <-ch:
				events = append(events, evt)
			case <-deadline:
				return events
			}
		}
	}
	return handler, collect
}

// --- Session Tests ---

func TestSession_NewEmpty(t *testing.T) {
	s := NewSession("")
	if s.Len() != 0 {
		t.Errorf("expected empty session, got %d messages", s.Len())
	}
}

func TestSession_NewWithSystemPrompt(t *testing.T) {
	s := NewSession("You are helpful.")
	if s.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", s.Len())
	}
	msg := s.Messages()[0]
	if msg.Role != types.RoleSystem {
		t.Errorf("expected system role, got %s", msg.Role)
	}
	if msg.Content != "You are helpful." {
		t.Errorf("expected system prompt content, got %q", msg.Content)
	}
}

func TestSession_Append(t *testing.T) {
	s := NewSession("")
	s.Append(types.UserMessage("hello"))
	s.Append(types.AssistantMessage("hi", nil))
	msgs := s.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != types.RoleUser {
		t.Errorf("expected user role, got %s", msgs[0].Role)
	}
	if msgs[1].Role != types.RoleAssistant {
		t.Errorf("expected assistant role, got %s", msgs[1].Role)
	}
}

func TestSession_AppendSystem(t *testing.T) {
	s := NewSession("")
	s.AppendSystem("nudge message")
	if s.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", s.Len())
	}
	if s.Messages()[0].Role != types.RoleSystem {
		t.Errorf("expected system role, got %s", s.Messages()[0].Role)
	}
}

// --- Governor Tests ---

func TestGovernor_CheckLimit_Under(t *testing.T) {
	g := NewGovernor(15)
	if g.CheckLimit(1) {
		t.Error("expected false for count=1 with max=15")
	}
}

func TestGovernor_CheckLimit_AtLimit(t *testing.T) {
	g := NewGovernor(15)
	if !g.CheckLimit(15) {
		t.Error("expected true for count=15 with max=15")
	}
}

func TestGovernor_IsDuplicate_FirstCall(t *testing.T) {
	g := NewGovernor(15)
	args := json.RawMessage(`{"path":"a.txt"}`)
	if g.IsDuplicate("read_file", args) {
		t.Error("expected false for first call")
	}
}

func TestGovernor_IsDuplicate_SecondCall(t *testing.T) {
	g := NewGovernor(15)
	args := json.RawMessage(`{"path":"a.txt"}`)
	g.IsDuplicate("read_file", args) // first call
	if !g.IsDuplicate("read_file", args) {
		t.Error("expected true for second call with same name+args")
	}
}

func TestGovernor_IsDuplicate_DifferentArgs(t *testing.T) {
	g := NewGovernor(15)
	g.IsDuplicate("read_file", json.RawMessage(`{"path":"a.txt"}`))
	if g.IsDuplicate("read_file", json.RawMessage(`{"path":"b.txt"}`)) {
		t.Error("expected false for same name but different args")
	}
}

func TestGovernor_ShouldNudge(t *testing.T) {
	g := NewGovernor(15)

	if g.ShouldNudge(4) {
		t.Error("expected false for count=4")
	}
	if !g.ShouldNudge(5) {
		t.Error("expected true for count=5")
	}
	if !g.ShouldNudge(10) {
		t.Error("expected true for count=10")
	}
	if g.ShouldNudge(0) {
		t.Error("expected false for count=0")
	}
}

func TestGovernor_Reset(t *testing.T) {
	g := NewGovernor(15)
	args := json.RawMessage(`{"path":"a.txt"}`)
	g.IsDuplicate("read_file", args) // first call
	g.Reset()
	if g.IsDuplicate("read_file", args) {
		t.Error("expected false after reset")
	}
}

// --- Agent Loop Tests ---

func TestRunTurn_TextOnly(t *testing.T) {
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens:   []string{"Hello", " world"},
				response: &types.Response{Text: "Hello world"},
			},
		},
	}
	registry := tools.NewRegistry()
	perms := permission.NewEngine(permission.ModeConfirm)
	handler, collect := collectEvents(256)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 15,
		Events:       handler,
	})

	err := ag.RunTurn(context.Background(), "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := collect()

	// Should have TokenEvents then DoneEvent
	var tokenCount int
	var hasDone bool
	for _, evt := range events {
		switch evt.(type) {
		case *types.TokenEvent:
			tokenCount++
		case *types.DoneEvent:
			hasDone = true
		}
	}
	if tokenCount != 2 {
		t.Errorf("expected 2 token events, got %d", tokenCount)
	}
	if !hasDone {
		t.Error("expected DoneEvent")
	}
}

func TestRunTurn_WithToolCall(t *testing.T) {
	toolInput := json.RawMessage(`{"path":"test.txt"}`)
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens: []string{"Let me read that."},
				response: &types.Response{
					Text: "Let me read that.",
					ToolCalls: []types.ToolCall{
						{ID: "call1", Name: "read_file", Input: toolInput},
					},
				},
			},
			{
				tokens:   []string{"The file contains test data."},
				response: &types.Response{Text: "The file contains test data."},
			},
		},
	}

	registry := tools.NewRegistry()
	registry.Register(&mockTool{
		name:      "read_file",
		riskLevel: permission.RiskReadOnly,
		result:    &tools.Result{Content: "file contents here"},
	})
	perms := permission.NewEngine(permission.ModeConfirm)
	handler, collect := collectEvents(256)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 15,
		Events:       handler,
	})

	err := ag.RunTurn(context.Background(), "read the file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := collect()

	var hasToolStart, hasToolEnd, hasDone bool
	for _, evt := range events {
		switch evt.(type) {
		case *types.ToolStartEvent:
			hasToolStart = true
		case *types.ToolEndEvent:
			hasToolEnd = true
		case *types.DoneEvent:
			hasDone = true
		}
	}
	if !hasToolStart {
		t.Error("expected ToolStartEvent")
	}
	if !hasToolEnd {
		t.Error("expected ToolEndEvent")
	}
	if !hasDone {
		t.Error("expected DoneEvent")
	}
}

func TestRunTurn_UnknownTool(t *testing.T) {
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens: []string{"Calling tool."},
				response: &types.Response{
					Text: "Calling tool.",
					ToolCalls: []types.ToolCall{
						{ID: "call1", Name: "nonexistent_tool", Input: json.RawMessage(`{}`)},
					},
				},
			},
			{
				tokens:   []string{"Sorry, that tool does not exist."},
				response: &types.Response{Text: "Sorry, that tool does not exist."},
			},
		},
	}

	registry := tools.NewRegistry()
	perms := permission.NewEngine(permission.ModeConfirm)
	handler, collect := collectEvents(256)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 15,
		Events:       handler,
	})

	err := ag.RunTurn(context.Background(), "use unknown tool")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := collect()

	// The error result should have been fed back to LLM (second call produces text-only)
	var hasDone bool
	for _, evt := range events {
		if _, ok := evt.(*types.DoneEvent); ok {
			hasDone = true
		}
	}
	if !hasDone {
		t.Error("expected DoneEvent after unknown tool handled")
	}

	// Verify tool result message was appended to session with error
	msgs := ag.Session().Messages()
	foundError := false
	for _, m := range msgs {
		if m.ToolResult != nil && m.ToolResult.IsError {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Error("expected error tool result in session for unknown tool")
	}
}

func TestRunTurn_PermissionConfirm(t *testing.T) {
	toolInput := json.RawMessage(`{"command":"git commit -m test"}`)
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens: []string{"Running command."},
				response: &types.Response{
					Text: "Running command.",
					ToolCalls: []types.ToolCall{
						{ID: "call1", Name: "write_file", Input: toolInput},
					},
				},
			},
			{
				tokens:   []string{"Done."},
				response: &types.Response{Text: "Done."},
			},
		},
	}

	registry := tools.NewRegistry()
	registry.Register(&mockTool{
		name:      "write_file",
		riskLevel: permission.RiskDML,
		result:    &tools.Result{Content: "write successful"},
	})
	perms := permission.NewEngine(permission.ModeConfirm)

	// Use a large buffer and manually handle the permission event
	eventCh := make(chan types.Event, 256)
	handler := EventChan(eventCh)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 15,
		Events:       handler,
	})

	// Run in goroutine since it blocks on permission response
	var runErr error
	done := make(chan struct{})
	go func() {
		runErr = ag.RunTurn(context.Background(), "write file")
		close(done)
	}()

	// Drain events and auto-approve permission requests
	var events []types.Event
	func() {
		for {
			select {
			case evt := <-eventCh:
				events = append(events, evt)
				if pr, ok := evt.(*types.PermissionRequestEvent); ok {
					pr.Response <- true // Approve
				}
			case <-done:
				// Drain remaining events
				for {
					select {
					case evt := <-eventCh:
						events = append(events, evt)
					default:
						return
					}
				}
			}
		}
	}()

	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	var hasPermReq bool
	for _, evt := range events {
		if _, ok := evt.(*types.PermissionRequestEvent); ok {
			hasPermReq = true
		}
	}
	if !hasPermReq {
		t.Error("expected PermissionRequestEvent for DML tool in CONFIRM mode")
	}
}

func TestRunTurn_PermissionDeny_PlanMode(t *testing.T) {
	toolInput := json.RawMessage(`{"file_path":"test.txt","content":"data"}`)
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens: []string{"Writing file."},
				response: &types.Response{
					Text: "Writing file.",
					ToolCalls: []types.ToolCall{
						{ID: "call1", Name: "write_file", Input: toolInput},
					},
				},
			},
			{
				tokens:   []string{"I cannot write in plan mode."},
				response: &types.Response{Text: "I cannot write in plan mode."},
			},
		},
	}

	registry := tools.NewRegistry()
	registry.Register(&mockTool{
		name:      "write_file",
		riskLevel: permission.RiskDML,
		result:    &tools.Result{Content: "write successful"},
	})
	perms := permission.NewEngine(permission.ModePlan) // PLAN mode denies writes
	handler, collect := collectEvents(256)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 15,
		Events:       handler,
	})

	err := ag.RunTurn(context.Background(), "write file in plan mode")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := collect()

	// Should have ToolEndEvent with denial, then DoneEvent
	var hasToolEndDenied bool
	for _, evt := range events {
		if te, ok := evt.(*types.ToolEndEvent); ok && te.IsError {
			hasToolEndDenied = true
		}
	}
	if !hasToolEndDenied {
		t.Error("expected ToolEndEvent with error for denied tool")
	}

	// Verify denial message in session
	msgs := ag.Session().Messages()
	foundDenial := false
	for _, m := range msgs {
		if m.ToolResult != nil && m.ToolResult.IsError {
			foundDenial = true
			break
		}
	}
	if !foundDenial {
		t.Error("expected denial tool result in session")
	}
}

func TestRunTurn_GovernorLimit(t *testing.T) {
	// Provider always returns a tool call - the governor should stop it
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens: []string{"Reading."},
				response: &types.Response{
					Text: "Reading.",
					ToolCalls: []types.ToolCall{
						{ID: "call1", Name: "read_file", Input: json.RawMessage(`{"path":"a.txt"}`)},
					},
				},
			},
			// Second call would return another tool call, but governor limit (1) stops it
			{
				tokens: []string{"Reading more."},
				response: &types.Response{
					Text: "Reading more.",
					ToolCalls: []types.ToolCall{
						{ID: "call2", Name: "read_file", Input: json.RawMessage(`{"path":"b.txt"}`)},
					},
				},
			},
		},
	}

	registry := tools.NewRegistry()
	registry.Register(&mockTool{
		name:      "read_file",
		riskLevel: permission.RiskReadOnly,
		result:    &tools.Result{Content: "contents"},
	})
	perms := permission.NewEngine(permission.ModeConfirm)
	handler, collect := collectEvents(256)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 1, // Very low limit
		Events:       handler,
	})

	err := ag.RunTurn(context.Background(), "read everything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := collect()

	var hasError, hasDone bool
	for _, evt := range events {
		switch e := evt.(type) {
		case *types.ErrorEvent:
			if e.Err != nil {
				hasError = true
			}
		case *types.DoneEvent:
			hasDone = true
		}
	}
	if !hasError {
		t.Error("expected ErrorEvent when governor limit reached")
	}
	if !hasDone {
		t.Error("expected DoneEvent after governor limit")
	}
}

func TestRunTurn_DuplicateDetection(t *testing.T) {
	sameInput := json.RawMessage(`{"path":"same.txt"}`)
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens: []string{"Reading."},
				response: &types.Response{
					Text: "Reading.",
					ToolCalls: []types.ToolCall{
						{ID: "call1", Name: "read_file", Input: sameInput},
					},
				},
			},
			{
				// Second call returns same tool+args - should be caught as duplicate
				tokens: []string{"Reading again."},
				response: &types.Response{
					Text: "Reading again.",
					ToolCalls: []types.ToolCall{
						{ID: "call2", Name: "read_file", Input: sameInput},
					},
				},
			},
			{
				// Third call: LLM responds with text only after duplicate feedback
				tokens:   []string{"OK, the file was already read."},
				response: &types.Response{Text: "OK, the file was already read."},
			},
		},
	}

	registry := tools.NewRegistry()
	registry.Register(&mockTool{
		name:      "read_file",
		riskLevel: permission.RiskReadOnly,
		result:    &tools.Result{Content: "file data"},
	})
	perms := permission.NewEngine(permission.ModeConfirm)
	handler, collect := collectEvents(256)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 15,
		Events:       handler,
	})

	err := ag.RunTurn(context.Background(), "read this file twice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = collect()

	// Verify the duplicate message was fed back
	msgs := ag.Session().Messages()
	foundDuplicateMsg := false
	for _, m := range msgs {
		if m.ToolResult != nil && m.ToolResult.IsError &&
			m.ToolResult.Content == "Duplicate tool call detected. Please try a different approach or ask the user for clarification." {
			foundDuplicateMsg = true
			break
		}
	}
	if !foundDuplicateMsg {
		t.Error("expected duplicate detection message in session")
	}
}

func TestRunTurn_ToolExecutionError(t *testing.T) {
	mp := &multiMockProvider{
		responses: []mockResponse{
			{
				tokens: []string{"Reading."},
				response: &types.Response{
					Text: "Reading.",
					ToolCalls: []types.ToolCall{
						{ID: "call1", Name: "read_file", Input: json.RawMessage(`{"path":"missing.txt"}`)},
					},
				},
			},
			{
				tokens:   []string{"The file was not found."},
				response: &types.Response{Text: "The file was not found."},
			},
		},
	}

	registry := tools.NewRegistry()
	registry.Register(&mockTool{
		name:      "read_file",
		riskLevel: permission.RiskReadOnly,
		result:    nil,
		err:       fmt.Errorf("file not found"),
	})
	perms := permission.NewEngine(permission.ModeConfirm)
	handler, collect := collectEvents(256)

	ag := New(AgentConfig{
		Provider:     mp,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: 15,
		Events:       handler,
	})

	err := ag.RunTurn(context.Background(), "read missing file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	events := collect()

	// Should have ToolEndEvent with error
	var hasToolEndError bool
	for _, evt := range events {
		if te, ok := evt.(*types.ToolEndEvent); ok && te.Err != nil {
			hasToolEndError = true
		}
	}
	if !hasToolEndError {
		t.Error("expected ToolEndEvent with error for tool execution failure")
	}

	// Error should be fed back to LLM via session
	msgs := ag.Session().Messages()
	foundErrorResult := false
	for _, m := range msgs {
		if m.ToolResult != nil && m.ToolResult.IsError {
			foundErrorResult = true
			break
		}
	}
	if !foundErrorResult {
		t.Error("expected error result in session for tool failure")
	}
}
