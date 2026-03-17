package tui

import (
	"testing"
)

func TestStreamRenderer_Push_DrainAll(t *testing.T) {
	r := NewStreamRenderer()

	// Push 10 tokens
	tokens := []string{"Hello", " ", "world", "!", " ", "How", " ", "are", " ", "you?"}
	for _, tok := range tokens {
		r.Push(tok)
	}

	// Drain all
	drained := r.DrainAll()
	if drained != 10 {
		t.Errorf("expected 10 drained, got %d", drained)
	}

	content := r.Content()
	expected := "Hello world! How are you?"
	if content != expected {
		t.Errorf("expected %q, got %q", expected, content)
	}

	if !r.Dirty() {
		t.Error("expected dirty to be true after drain")
	}
}

func TestStreamRenderer_NonBlockingPush(t *testing.T) {
	r := NewStreamRenderer()

	// Fill buffer to capacity (256) + extra to verify no deadlock
	for i := 0; i < 300; i++ {
		r.Push("x") // Must not block
	}

	// Drain what we can
	drained := r.DrainAll()
	if drained != 256 {
		t.Errorf("expected 256 drained (buffer capacity), got %d", drained)
	}
}

func TestStreamRenderer_Reset(t *testing.T) {
	r := NewStreamRenderer()
	r.Push("hello")
	r.DrainAll()

	r.Reset()

	if r.Content() != "" {
		t.Errorf("expected empty content after reset, got %q", r.Content())
	}
	if r.Dirty() {
		t.Error("expected dirty to be false after reset")
	}
}

func TestStreamRenderer_DrainAll_EmptyBuffer(t *testing.T) {
	r := NewStreamRenderer()
	drained := r.DrainAll()
	if drained != 0 {
		t.Errorf("expected 0 drained from empty buffer, got %d", drained)
	}
}

func TestKeyBindings(t *testing.T) {
	keys := DefaultKeyMap()

	// Verify required key bindings exist with correct keys
	tests := []struct {
		name    string
		binding KeyDef
		key     string // expected key string
	}{
		{"Cancel (Ctrl+C)", keys.Cancel, "ctrl+c"},
		{"Exit (Ctrl+D)", keys.Exit, "ctrl+d"},
		{"CycleMode (Shift+Tab)", keys.CycleMode, "shift+tab"},
		{"Submit (Enter)", keys.Submit, "enter"},
		{"ClearScreen (Ctrl+L)", keys.ClearScreen, "ctrl+l"},
		{"Background (Ctrl+B)", keys.Background, "ctrl+b"},
		{"Refresh (Ctrl+R)", keys.Refresh, "ctrl+r"},
		{"Newline (Shift+Enter)", keys.Newline, "shift+enter"},
	}

	for _, tt := range tests {
		if len(tt.binding.Keys) == 0 {
			t.Errorf("key binding %s has no keys", tt.name)
			continue
		}
		if !tt.binding.Matches(tt.key) {
			t.Errorf("key binding %s should match %q", tt.name, tt.key)
		}
		if tt.binding.Help == "" {
			t.Errorf("key binding %s has empty help text", tt.name)
		}
	}
}

func TestRenderMessage_User(t *testing.T) {
	msg := ChatMessage{Role: "user", Content: "Hello there"}
	rendered := renderMessage(msg)
	if rendered == "" {
		t.Error("expected non-empty rendered message for user")
	}
	// Should contain the content text
	if !containsStr(rendered, "Hello there") {
		t.Error("rendered user message should contain the original content")
	}
}

func TestRenderMessage_Assistant(t *testing.T) {
	msg := ChatMessage{Role: "assistant", Content: "Hello, how can I help?"}
	rendered := renderMessage(msg)
	if rendered == "" {
		t.Error("expected non-empty rendered message for assistant")
	}
	// Should contain the content text (possibly with markdown formatting)
	if !containsStr(rendered, "Hello") {
		t.Error("rendered assistant message should contain the original content")
	}
}

func TestRenderMessage_Error(t *testing.T) {
	msg := ChatMessage{Role: "error", Content: "something went wrong"}
	rendered := renderMessage(msg)
	if rendered == "" {
		t.Error("expected non-empty rendered message for error")
	}
	if !containsStr(rendered, "something went wrong") {
		t.Error("rendered error message should contain the error text")
	}
}

func TestFormatToolResult(t *testing.T) {
	result := FormatToolResult("read_file", "file contents here", false)
	if !containsStr(result, "read_file") {
		t.Error("tool result should contain tool name")
	}
	if !containsStr(result, "file contents here") {
		t.Error("tool result should contain content")
	}
}

func TestFormatToolResult_Error(t *testing.T) {
	result := FormatToolResult("bash", "command not found", true)
	if !containsStr(result, "failed") {
		t.Error("error tool result should indicate failure")
	}
}

func TestAbbreviateArgs(t *testing.T) {
	input := []byte(`{"command":"ls -la","path":"/tmp"}`)
	result := abbreviateArgs(input)
	if result == "" {
		t.Error("expected non-empty abbreviated args")
	}
}

func TestAbbreviateArgs_Long(t *testing.T) {
	long := `{"command":"` + string(make([]byte, 200)) + `"}`
	result := abbreviateArgs([]byte(long))
	if len(result) > 100 {
		t.Errorf("abbreviated args should be truncated, got length %d", len(result))
	}
}

func TestNewConfirmModel(t *testing.T) {
	c := NewConfirmModel()
	if c.Active() {
		t.Error("new confirm model should not be active")
	}
	if c.View() != "" {
		t.Error("inactive confirm model should render empty string")
	}
}

func TestConfirmModel_Show(t *testing.T) {
	c := NewConfirmModel()
	ch := make(chan bool, 1)
	c.Show("bash", []byte(`{"command":"rm -rf /tmp/test"}`), "DESTRUCTIVE", ch)

	if !c.Active() {
		t.Error("confirm model should be active after Show")
	}

	view := c.View()
	if view == "" {
		t.Error("active confirm model should render non-empty string")
	}
}

// containsStr checks if s contains substr (simple helper to avoid importing strings).
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
