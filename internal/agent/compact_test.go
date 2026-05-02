package agent

import (
	"context"
	"testing"

	"github.com/slokam-ai/cascade/internal/provider"
	"github.com/slokam-ai/cascade/pkg/types"
)

// --- ShouldCompact tests ---

func TestShouldCompact_AtThreshold(t *testing.T) {
	// 800k of 1M = 80% -> should compact
	if !ShouldCompact(800_000, "gemini-2.5-pro", 80.0) {
		t.Error("expected true for 800k/1M (80%)")
	}
}

func TestShouldCompact_BelowThreshold(t *testing.T) {
	// 790k of 1M = 79% -> should NOT compact
	if ShouldCompact(790_000, "gemini-2.5-pro", 80.0) {
		t.Error("expected false for 790k/1M (79%)")
	}
}

func TestShouldCompact_ZeroTokens(t *testing.T) {
	if ShouldCompact(0, "gemini-2.5-pro", 80.0) {
		t.Error("expected false for 0 tokens")
	}
}

func TestShouldCompact_AboveThreshold(t *testing.T) {
	// 900k of 1M = 90% -> should compact
	if !ShouldCompact(900_000, "gemini-2.5-pro", 80.0) {
		t.Error("expected true for 900k/1M (90%)")
	}
}

// --- contextWindowForModel tests ---

func TestContextWindowForModel_Gemini25(t *testing.T) {
	if got := contextWindowForModel("gemini-2.5-pro"); got != 1_000_000 {
		t.Errorf("expected 1M for gemini-2.5-pro, got %d", got)
	}
}

func TestContextWindowForModel_Gemini15Pro(t *testing.T) {
	if got := contextWindowForModel("gemini-1.5-pro"); got != 2_000_000 {
		t.Errorf("expected 2M for gemini-1.5-pro, got %d", got)
	}
}

func TestContextWindowForModel_Unknown(t *testing.T) {
	if got := contextWindowForModel("unknown-model"); got != 1_000_000 {
		t.Errorf("expected 1M default for unknown model, got %d", got)
	}
}

// --- CompactSession tests ---

// compactMockProvider is a simple mock that returns a canned summary for compaction.
type compactMockProvider struct {
	summary  string
	messages []types.Message // captures the messages sent
}

func (p *compactMockProvider) Model() string { return "mock-model" }

func (p *compactMockProvider) GenerateStream(_ context.Context, messages []types.Message, _ []provider.Declaration) (*provider.Stream, error) {
	p.messages = messages
	tokens := make(chan string, 1)
	result := make(chan provider.StreamResult, 1)
	go func() {
		defer close(tokens)
		tokens <- p.summary
		result <- provider.StreamResult{
			Response: &types.Response{Text: p.summary},
		}
	}()
	return provider.NewStream(tokens, result, func() {}), nil
}

func TestCompactSession_TooFewMessages(t *testing.T) {
	prov := &compactMockProvider{summary: "should not be called"}
	messages := []types.Message{
		types.SystemMessage("system prompt"),
		types.UserMessage("hello"),
		types.AssistantMessage("hi there", nil),
	}

	// recentKeep=6, only 3 messages total -> nothing to compact
	result, summary, err := CompactSession(context.Background(), prov, messages, 6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "" {
		t.Errorf("expected empty summary, got %q", summary)
	}
	if len(result) != len(messages) {
		t.Errorf("expected %d messages unchanged, got %d", len(messages), len(result))
	}
}

func TestCompactSession_SplitsCorrectly(t *testing.T) {
	prov := &compactMockProvider{summary: "Schema: users table with id, name columns"}

	// Build a session with system prompt + 8 messages
	messages := []types.Message{
		types.SystemMessage("You are a helpful assistant."),
		types.UserMessage("What tables do we have?"),
		types.AssistantMessage("We have a users table.", nil),
		types.UserMessage("Show me the schema."),
		types.AssistantMessage("users: id (INT), name (STRING)", nil),
		types.UserMessage("Query: SELECT * FROM users"),
		types.AssistantMessage("Here are the results...", nil),
		types.UserMessage("What was the cost?"),
		types.AssistantMessage("$0.02 for that query.", nil),
	}

	// Keep last 4 messages, compact the rest
	result, summary, err := CompactSession(context.Background(), prov, messages, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary != "Schema: users table with id, name columns" {
		t.Errorf("unexpected summary: %q", summary)
	}

	// Result should be: summary system message + last 4 messages = 5 messages
	if len(result) != 5 {
		t.Fatalf("expected 5 messages (1 summary + 4 recent), got %d", len(result))
	}

	// First message should be the compacted summary
	if result[0].Role != types.RoleSystem {
		t.Errorf("expected first message to be system (summary), got %s", result[0].Role)
	}
	if result[0].Content != "## Conversation Summary (compacted)\n\nSchema: users table with id, name columns" {
		t.Errorf("unexpected summary message content: %q", result[0].Content)
	}

	// Last 4 messages should match original last 4
	for i := 0; i < 4; i++ {
		orig := messages[len(messages)-4+i]
		got := result[1+i]
		if orig.Role != got.Role || orig.Content != got.Content {
			t.Errorf("message %d mismatch: expected role=%s content=%q, got role=%s content=%q",
				i, orig.Role, orig.Content, got.Role, got.Content)
		}
	}

	// Verify the older messages were sent to the LLM for summarization
	// Should include system prompt + older messages (indices 1-4) + compaction prompt
	if len(prov.messages) != 6 { // system + 4 older + compaction prompt
		t.Errorf("expected 6 messages sent to LLM for compaction, got %d", len(prov.messages))
	}
}

func TestCompactSession_NoSystemPrompt(t *testing.T) {
	prov := &compactMockProvider{summary: "Summary without system prompt"}

	messages := []types.Message{
		types.UserMessage("hello"),
		types.AssistantMessage("hi", nil),
		types.UserMessage("query"),
		types.AssistantMessage("results", nil),
		types.UserMessage("more"),
		types.AssistantMessage("done", nil),
	}

	// Keep last 2 messages
	result, summary, err := CompactSession(context.Background(), prov, messages, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Result: summary + last 2 = 3 messages
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != types.RoleSystem {
		t.Errorf("expected summary system message, got %s", result[0].Role)
	}
}

// --- Session.Replace tests ---

func TestSession_Replace(t *testing.T) {
	s := NewSession("original prompt")
	s.Append(types.UserMessage("hello"))
	s.Append(types.AssistantMessage("hi", nil))

	if s.Len() != 3 { // system + user + assistant
		t.Fatalf("expected 3 messages before replace, got %d", s.Len())
	}

	// Replace with new messages
	newMsgs := []types.Message{
		types.SystemMessage("## Summary"),
		types.UserMessage("recent question"),
	}
	s.Replace(newMsgs)

	msgs := s.Messages()
	// Should have: original system prompt + summary + recent question = 3
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages after replace, got %d", len(msgs))
	}
	if msgs[0].Content != "original prompt" {
		t.Errorf("expected original system prompt preserved, got %q", msgs[0].Content)
	}
	if msgs[1].Content != "## Summary" {
		t.Errorf("expected summary message, got %q", msgs[1].Content)
	}
}

func TestSession_Replace_NoSystemPrompt(t *testing.T) {
	s := NewSession("")
	s.Append(types.UserMessage("hello"))

	newMsgs := []types.Message{
		types.SystemMessage("## Summary"),
		types.UserMessage("recent"),
	}
	s.Replace(newMsgs)

	msgs := s.Messages()
	// No system prompt -> just the 2 new messages
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after replace (no system prompt), got %d", len(msgs))
	}
}

// --- Session.SystemPrompt and SetSystemPrompt tests ---

func TestSession_SystemPrompt(t *testing.T) {
	s := NewSession("test prompt")
	if got := s.SystemPrompt(); got != "test prompt" {
		t.Errorf("expected 'test prompt', got %q", got)
	}
}

func TestSession_SetSystemPrompt(t *testing.T) {
	s := NewSession("original")
	s.SetSystemPrompt("updated")

	if got := s.SystemPrompt(); got != "updated" {
		t.Errorf("expected 'updated', got %q", got)
	}
	// First message should also be updated
	if s.Messages()[0].Content != "updated" {
		t.Errorf("expected first message content to be 'updated', got %q", s.Messages()[0].Content)
	}
}
