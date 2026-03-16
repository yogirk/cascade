package persist

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yogirk/cascade/pkg/types"
)

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := OpenSQLite(t.TempDir())
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSaveAndLoadSession(t *testing.T) {
	store := openTestStore(t)

	meta := SessionMeta{
		ID:        "test-001",
		Model:     "gemini-2.5-pro",
		Project:   "my-project",
		Summary:   "test session",
		CreatedAt: time.Now().Truncate(time.Second),
	}

	messages := []types.Message{
		types.SystemMessage("You are helpful."),
		types.UserMessage("What tables exist?"),
		types.AssistantMessage("Let me check.", []types.ToolCall{
			{ID: "tc1", Name: "bigquery_schema", Input: json.RawMessage(`{"action":"list_tables"}`)},
		}),
		types.ToolResultMessage("tc1", "bigquery_schema", "users, orders, products", false),
		types.AssistantMessage("You have 3 tables: users, orders, and products.", nil),
	}

	if err := store.SaveSession(meta, messages); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	// Load and verify
	loaded, msgs, err := store.LoadSession("test-001")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded.ID != meta.ID {
		t.Errorf("ID: got %q, want %q", loaded.ID, meta.ID)
	}
	if loaded.Model != meta.Model {
		t.Errorf("Model: got %q, want %q", loaded.Model, meta.Model)
	}
	if loaded.Summary != meta.Summary {
		t.Errorf("Summary: got %q, want %q", loaded.Summary, meta.Summary)
	}
	if len(msgs) != len(messages) {
		t.Fatalf("message count: got %d, want %d", len(msgs), len(messages))
	}

	// Verify each message
	for i, want := range messages {
		got := msgs[i]
		if got.Role != want.Role {
			t.Errorf("msg[%d].Role: got %q, want %q", i, got.Role, want.Role)
		}
		if got.Content != want.Content {
			t.Errorf("msg[%d].Content: got %q, want %q", i, got.Content, want.Content)
		}
		if len(got.ToolCalls) != len(want.ToolCalls) {
			t.Errorf("msg[%d].ToolCalls: got %d, want %d", i, len(got.ToolCalls), len(want.ToolCalls))
		}
		if (got.ToolResult == nil) != (want.ToolResult == nil) {
			t.Errorf("msg[%d].ToolResult: got nil=%v, want nil=%v", i, got.ToolResult == nil, want.ToolResult == nil)
		}
		if got.ToolResult != nil && want.ToolResult != nil {
			if got.ToolResult.CallID != want.ToolResult.CallID {
				t.Errorf("msg[%d].ToolResult.CallID: got %q, want %q", i, got.ToolResult.CallID, want.ToolResult.CallID)
			}
			if got.ToolResult.Content != want.ToolResult.Content {
				t.Errorf("msg[%d].ToolResult.Content: got %q, want %q", i, got.ToolResult.Content, want.ToolResult.Content)
			}
			if got.ToolResult.IsError != want.ToolResult.IsError {
				t.Errorf("msg[%d].ToolResult.IsError: got %v, want %v", i, got.ToolResult.IsError, want.ToolResult.IsError)
			}
		}
	}
}

func TestSaveSession_Idempotent(t *testing.T) {
	store := openTestStore(t)

	meta := SessionMeta{ID: "idem-001", Model: "test", Summary: "v1"}
	msgs1 := []types.Message{types.UserMessage("hello")}
	if err := store.SaveSession(meta, msgs1); err != nil {
		t.Fatalf("SaveSession v1: %v", err)
	}

	// Save again with different messages — should replace
	meta.Summary = "v2"
	msgs2 := []types.Message{types.UserMessage("hello"), types.AssistantMessage("hi", nil)}
	if err := store.SaveSession(meta, msgs2); err != nil {
		t.Fatalf("SaveSession v2: %v", err)
	}

	loaded, msgs, err := store.LoadSession("idem-001")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if loaded.Summary != "v2" {
		t.Errorf("Summary: got %q, want %q", loaded.Summary, "v2")
	}
	if len(msgs) != 2 {
		t.Errorf("message count: got %d, want 2", len(msgs))
	}
}

func TestListSessions_Order(t *testing.T) {
	store := openTestStore(t)

	// Save s1, wait, then save s2 — s2 gets a later updated_at
	meta1 := SessionMeta{ID: "s1", Model: "test"}
	if err := store.SaveSession(meta1, []types.Message{types.UserMessage("msg")}); err != nil {
		t.Fatalf("SaveSession s1: %v", err)
	}

	time.Sleep(1100 * time.Millisecond) // ensure different Unix second

	meta2 := SessionMeta{ID: "s2", Model: "test"}
	if err := store.SaveSession(meta2, []types.Message{types.UserMessage("msg")}); err != nil {
		t.Fatalf("SaveSession s2: %v", err)
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("count: got %d, want 2", len(sessions))
	}
	// s2 was saved later — should be first (ORDER BY updated_at DESC)
	if sessions[0].ID != "s2" {
		t.Errorf("first session: got %q, want %q", sessions[0].ID, "s2")
	}
	if sessions[1].ID != "s1" {
		t.Errorf("second session: got %q, want %q", sessions[1].ID, "s1")
	}
}

func TestDeleteSession(t *testing.T) {
	store := openTestStore(t)

	meta := SessionMeta{ID: "del-001", Model: "test"}
	if err := store.SaveSession(meta, []types.Message{types.UserMessage("hello")}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	if err := store.DeleteSession("del-001"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, _, err := store.LoadSession("del-001")
	if err == nil {
		t.Error("expected error loading deleted session")
	}
}

func TestLatestSessionID(t *testing.T) {
	store := openTestStore(t)

	// Empty store
	id, err := store.LatestSessionID()
	if err != nil {
		t.Fatalf("LatestSessionID: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty ID, got %q", id)
	}

	// Add a session
	if err := store.SaveSession(SessionMeta{ID: "latest-001", Model: "test"}, []types.Message{types.UserMessage("hi")}); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	id, err = store.LatestSessionID()
	if err != nil {
		t.Fatalf("LatestSessionID: %v", err)
	}
	if id != "latest-001" {
		t.Errorf("got %q, want %q", id, "latest-001")
	}
}

func TestLoadSession_NotFound(t *testing.T) {
	store := openTestStore(t)

	_, _, err := store.LoadSession("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestToolResultErrorRoundTrip(t *testing.T) {
	store := openTestStore(t)

	meta := SessionMeta{ID: "err-001", Model: "test"}
	msgs := []types.Message{
		types.ToolResultMessage("tc1", "bash", "command not found", true),
	}

	if err := store.SaveSession(meta, msgs); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	_, loaded, err := store.LoadSession("err-001")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("got %d messages, want 1", len(loaded))
	}
	if !loaded[0].ToolResult.IsError {
		t.Error("expected IsError=true after round-trip")
	}
	if loaded[0].ToolResult.Content != "command not found" {
		t.Errorf("content: got %q, want %q", loaded[0].ToolResult.Content, "command not found")
	}
}

func TestGenerateSessionID(t *testing.T) {
	id := GenerateSessionID()
	if len(id) < 20 {
		t.Errorf("ID too short: %q", id)
	}
	// Should have format YYYYMMDD-HHMMSS-XXXX
	if id[8] != '-' || id[15] != '-' {
		t.Errorf("unexpected format: %q", id)
	}
}
