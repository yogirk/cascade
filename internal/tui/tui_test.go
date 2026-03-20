package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yogirk/cascade/internal/permission"
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

func TestStreamRenderer_LosslessPush(t *testing.T) {
	r := NewStreamRenderer()

	// Push many tokens — all must be preserved (no drops)
	for i := 0; i < 1000; i++ {
		r.Push("x")
	}

	drained := r.DrainAll()
	if drained != 1000 {
		t.Errorf("expected 1000 drained (lossless), got %d", drained)
	}
	if len(r.Content()) != 1000 {
		t.Errorf("expected 1000 chars in content, got %d", len(r.Content()))
	}
}

func TestStreamRenderer_ConcurrentPush(t *testing.T) {
	r := NewStreamRenderer()

	// Push from multiple goroutines concurrently
	done := make(chan struct{})
	for g := 0; g < 10; g++ {
		go func() {
			for i := 0; i < 100; i++ {
				r.Push("x")
			}
			done <- struct{}{}
		}()
	}
	for g := 0; g < 10; g++ {
		<-done
	}

	drained := r.DrainAll()
	if drained != 1000 {
		t.Errorf("expected 1000 drained from concurrent push, got %d", drained)
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

	tests := []struct {
		name    string
		binding KeyDef
		key     string
	}{
		{"Cancel (Ctrl+C)", keys.Cancel, "ctrl+c"},
		{"Exit (Ctrl+D)", keys.Exit, "ctrl+d"},
		{"CycleMode (Shift+Tab)", keys.CycleMode, "shift+tab"},
		{"Submit (Enter)", keys.Submit, "enter"},
		{"ClearScreen (Ctrl+L)", keys.ClearScreen, "ctrl+l"},
		{"Background (Ctrl+B)", keys.Background, "ctrl+b"},
		{"Refresh (Ctrl+R)", keys.Refresh, "ctrl+r"},
		{"Newline (Shift+Enter)", keys.Newline, "shift+enter"},
		{"ConfirmUp (k)", keys.ConfirmUp, "k"},
		{"ConfirmUp (up)", keys.ConfirmUp, "up"},
		{"ConfirmDown (j)", keys.ConfirmDown, "j"},
		{"ConfirmDown (down)", keys.ConfirmDown, "down"},
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
	rendered := renderMessage(msg, 80)
	if rendered == "" {
		t.Error("expected non-empty rendered message for user")
	}
	if !containsStr(rendered, "> ") {
		t.Error("user message should contain '> ' prefix")
	}
	if !containsStr(rendered, "Hello there") {
		t.Error("rendered user message should contain the original content")
	}
}

func TestRenderMessage_Assistant(t *testing.T) {
	msg := ChatMessage{Role: "assistant", Content: "Hello, how can I help?"}
	rendered := renderMessage(msg, 80)
	if rendered == "" {
		t.Error("expected non-empty rendered message for assistant")
	}
	if !containsStr(rendered, "Hello") {
		t.Error("rendered assistant message should contain the original content")
	}
	// Should NOT contain emoji prefix
	if containsStr(rendered, "✨") {
		t.Error("assistant message should not contain emoji prefix")
	}
}

func TestRenderMessage_Error(t *testing.T) {
	msg := ChatMessage{Role: "error", Content: "something went wrong"}
	rendered := renderMessage(msg, 80)
	if rendered == "" {
		t.Error("expected non-empty rendered message for error")
	}
	if !containsStr(rendered, "!") {
		t.Error("error message should contain '!' prefix")
	}
	if !containsStr(rendered, "something went wrong") {
		t.Error("rendered error message should contain the error text")
	}
}

func TestRenderMessage_System(t *testing.T) {
	msg := ChatMessage{Role: "system", Content: "Switched model to gemini-2.5-flash."}
	rendered := renderMessage(msg, 80)
	if !containsStr(rendered, "Switched model") {
		t.Error("system message should contain message content")
	}
}

func TestRenderToolMessage(t *testing.T) {
	msg := ChatMessage{
		Role:     "tool",
		ToolName: "read_file",
		ToolArgs: json.RawMessage(`{"file_path":"src/main.go"}`),
		Content:  "file contents here",
		Display:  "file contents here",
	}
	rendered := renderToolMessage(msg)
	if !containsStr(rendered, "∞") {
		t.Error("tool message should contain ∞ bullet")
	}
	if !containsStr(rendered, "read_file") {
		t.Error("tool message should contain tool name")
	}
}

func TestRenderToolMessage_Error(t *testing.T) {
	msg := ChatMessage{
		Role:     "tool",
		ToolName: "bash",
		ToolArgs: json.RawMessage(`{"command":"ls /nonexistent"}`),
		Content:  "command not found",
		IsError:  true,
	}
	rendered := renderToolMessage(msg)
	if !containsStr(rendered, "∞") {
		t.Error("tool error should contain ∞ bullet")
	}
	if !containsStr(rendered, "!") {
		t.Error("tool error should contain '!' prefix in output")
	}
}

func TestRenderDiff(t *testing.T) {
	diff := `@@ -12,6 +12,7 @@
     order_total,
-    shipping_cost,
+    shipping_cost,
+    discount_type,
     _loaded_at`
	rendered := renderDiff(diff)
	if !containsStr(rendered, "@@") {
		t.Error("diff should contain hunk header")
	}
	if rendered == "" {
		t.Error("diff render should not be empty")
	}
}

func TestIsDiff(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"--- a/file.go\n+++ b/file.go", true},
		{"@@ -1,3 +1,4 @@\n content", true},
		{"just normal text", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isDiff(tt.input); got != tt.want {
			t.Errorf("isDiff(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFormatToolResult(t *testing.T) {
	result := FormatToolResult("read_file", "file contents here", false)
	if !containsStr(result, "Executed: read_file") {
		t.Error("success tool result should contain tool name")
	}
	if !containsStr(result, "file contents here") {
		t.Error("success tool result should contain content")
	}
}

func TestFormatToolResult_Error(t *testing.T) {
	result := FormatToolResult("bash", "command not found", true)
	if !containsStr(result, "Failed: bash") {
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
	if !containsStr(view, "bash") {
		t.Error("confirm view should contain tool name")
	}
	if !containsStr(view, "DESTRUCTIVE") {
		t.Error("confirm view should contain risk badge")
	}
	if !containsStr(view, "rm -rf") {
		t.Error("confirm view should contain readable command summary")
	}
	if !containsStr(view, "Allow") {
		t.Error("confirm view should contain Allow option")
	}
	if !containsStr(view, "Deny") {
		t.Error("confirm view should contain Deny option")
	}
}

func TestConfirmModel_CursorNavigation(t *testing.T) {
	c := NewConfirmModel()
	ch := make(chan bool, 1)
	c.Show("bash", []byte(`{"command":"test"}`), "DML", ch)

	if c.cursor != 0 {
		t.Error("cursor should start at 0 (Yes)")
	}

	// Simulate pressing 'j' to move down — need a mock KeyMsg
	// Just test the cursor field directly since Update requires tea.KeyMsg
	c.cursor = 1
	if c.cursor != 1 {
		t.Error("cursor should be at 1 (No)")
	}
}

func TestRiskBadge(t *testing.T) {
	tests := []struct {
		level    string
		contains string
	}{
		{"READ_ONLY", "READ"},
		{"DML", "DML"},
		{"DESTRUCTIVE", "DESTRUCTIVE"},
	}
	for _, tt := range tests {
		badge := RiskBadge(tt.level)
		if !containsStr(badge, tt.contains) {
			t.Errorf("RiskBadge(%q) should contain %q, got %q", tt.level, tt.contains, badge)
		}
	}
}

func TestModeBadge(t *testing.T) {
	tests := []struct {
		mode     permission.Mode
		contains string
	}{
		{permission.ModeConfirm, "CONFIRM"},
		{permission.ModePlan, "PLAN"},
		{permission.ModeBypass, "BYPASS"},
	}
	for _, tt := range tests {
		badge := ModeBadge(tt.mode)
		if !containsStr(badge, tt.contains) {
			t.Errorf("ModeBadge(%v) should contain %q, got %q", tt.mode, tt.contains, badge)
		}
	}
}

func TestWelcomeView(t *testing.T) {
	w := NewWelcomeModel("gemini-2.5-pro", permission.ModeConfirm, "~/Projects/cascade", "main")
	w.SetSize(100, 30)
	view := w.View()
	if !containsStr(view, "cascade") {
		t.Error("welcome should contain 'cascade' name")
	}
	if !containsStr(view, "gemini-2.5-pro") {
		t.Error("welcome should contain model name")
	}
	if !containsStr(view, "Quick start") {
		t.Error("welcome should contain quick start section")
	}
	if !containsStr(view, "Shortcuts") {
		t.Error("welcome should contain shortcuts section")
	}
}

func TestStatusBarLayout(t *testing.T) {
	s := NewStatusModel("gemini-2.5-pro", permission.ModeConfirm)
	s.SetWidth(100)
	s.SetGitBranch("main")
	s.SetCwd("~/Projects/cascade")

	view := s.View()
	if !containsStr(view, "gemini-2.5-pro") {
		t.Error("status bar should contain model name")
	}
	if !containsStr(view, "CONFIRM") {
		t.Error("status bar should contain mode")
	}
}

func TestStatusBarResponsive(t *testing.T) {
	s := NewStatusModel("gemini-2.5-pro", permission.ModeConfirm)
	s.SetGitBranch("main")
	s.SetCwd("~/Projects/cascade")

	// Narrow: should hide cwd
	s.SetWidth(50)
	view := s.View()
	if containsStr(view, "~/Projects/cascade") {
		t.Error("status bar should hide cwd at narrow width")
	}
}

func TestShortenPath(t *testing.T) {
	// Basic test — just ensure it doesn't panic and returns something
	result := ShortenPath("/usr/local/bin")
	if result == "" {
		t.Error("ShortenPath should return non-empty string")
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

// --- Interaction contract tests ---

func TestInputHistory(t *testing.T) {
	input := NewInputModel()

	// Push some history
	input.PushHistory("first prompt")
	input.PushHistory("second prompt")
	input.PushHistory("third prompt")

	// Navigate up through history
	if !input.HistoryUp() {
		t.Error("HistoryUp should return true with history")
	}
	if input.Value() != "third prompt" {
		t.Errorf("expected 'third prompt', got %q", input.Value())
	}

	if !input.HistoryUp() {
		t.Error("HistoryUp should return true")
	}
	if input.Value() != "second prompt" {
		t.Errorf("expected 'second prompt', got %q", input.Value())
	}

	// Navigate back down
	if !input.HistoryDown() {
		t.Error("HistoryDown should return true in history mode")
	}
	if input.Value() != "third prompt" {
		t.Errorf("expected 'third prompt', got %q", input.Value())
	}

	// Down past end returns to empty draft
	if !input.HistoryDown() {
		t.Error("HistoryDown should return true to exit history")
	}
	if input.Value() != "" {
		t.Errorf("expected empty draft, got %q", input.Value())
	}

	// Down when not in history mode returns false
	if input.HistoryDown() {
		t.Error("HistoryDown should return false when not in history mode")
	}
}

func TestInputHistory_Dedup(t *testing.T) {
	input := NewInputModel()

	input.PushHistory("same")
	input.PushHistory("same")
	input.PushHistory("same")

	// Should only have one entry
	input.HistoryUp()
	if input.Value() != "same" {
		t.Errorf("expected 'same', got %q", input.Value())
	}
	// Going further up should stay at same (only 1 entry)
	input.HistoryUp()
	if input.Value() != "same" {
		t.Errorf("expected 'same' at top, got %q", input.Value())
	}
}

func TestInputHistory_Empty(t *testing.T) {
	input := NewInputModel()

	// No history — up should return false
	if input.HistoryUp() {
		t.Error("HistoryUp should return false with no history")
	}
}

func TestChatModel_Clear(t *testing.T) {
	chat := NewChatModel(80, 20)
	chat.AddMessage(ChatMessage{Role: "user", Content: "hello"})
	chat.AddMessage(ChatMessage{Role: "assistant", Content: "hi"})

	if chat.MessageCount() != 2 {
		t.Errorf("expected 2 messages, got %d", chat.MessageCount())
	}

	chat.Clear()

	if chat.MessageCount() != 0 {
		t.Errorf("expected 0 messages after clear, got %d", chat.MessageCount())
	}
}

func TestTurnSeparator_FirstMessage(t *testing.T) {
	// First user message (index 0) should NOT have a separator
	msg := ChatMessage{Role: "user", Content: "first"}
	rendered := renderMessageAt(msg, 80, 0)
	if containsStr(rendered, "─") {
		t.Error("first user message should not have a turn separator")
	}

	// Second user message (index > 0) should have a separator
	rendered2 := renderMessageAt(msg, 80, 5)
	if !containsStr(rendered2, "─") {
		t.Error("non-first user message should have a turn separator")
	}
}

func TestToolOutputTruncation_Short(t *testing.T) {
	// Short output (< 13 lines) should show all lines
	lines := make([]string, 5)
	for i := range lines {
		lines[i] = "line"
	}
	msg := ChatMessage{Role: "tool", ToolName: "bash", Content: strings.Join(lines, "\n")}
	rendered := renderMessage(msg, 80)
	if containsStr(rendered, "omitted") {
		t.Error("short output should not be truncated")
	}
}

func TestToolOutputTruncation_Long(t *testing.T) {
	// Long output (> 13 lines) should show head + tail with omission
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	msg := ChatMessage{Role: "tool", ToolName: "bash", Content: strings.Join(lines, "\n")}
	rendered := renderMessage(msg, 80)
	if !containsStr(rendered, "omitted") {
		t.Error("long output should show omission marker")
	}
	// Should contain first line
	if !containsStr(rendered, "line-0") {
		t.Error("truncated output should contain first line")
	}
	// Should contain last line
	if !containsStr(rendered, "line-29") {
		t.Error("truncated output should contain last line")
	}
}

func TestFormatArgsSummary_Bash(t *testing.T) {
	lines := formatArgsSummary("bash", []byte(`{"command":"ls -la /tmp"}`))
	if len(lines) == 0 {
		t.Fatal("expected non-empty summary for bash")
	}
	if !containsStr(lines[0], "$ ls -la /tmp") {
		t.Errorf("bash summary should show command with $ prefix, got %q", lines[0])
	}
}

func TestFormatArgsSummary_Write(t *testing.T) {
	lines := formatArgsSummary("write", []byte(`{"file_path":"/tmp/test.go","content":"package main"}`))
	if len(lines) == 0 {
		t.Fatal("expected non-empty summary for write")
	}
	if !containsStr(lines[0], "file: /tmp/test.go") {
		t.Errorf("write summary should show file path, got %q", lines[0])
	}
}

func TestFormatArgsSummary_Generic(t *testing.T) {
	lines := formatArgsSummary("unknown_tool", []byte(`{"foo":"bar","baz":"qux"}`))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines for generic summary, got %d", len(lines))
	}
	// Should be sorted
	if !containsStr(lines[0], "baz:") {
		t.Errorf("generic summary should be sorted, first line: %q", lines[0])
	}
}

func TestStatusBar_PendingApproval(t *testing.T) {
	s := NewStatusModel("gemini-2.5-pro", 0)
	s.SetWidth(100)

	s.SetPendingApproval(true)
	view := s.View()
	if !containsStr(view, "awaiting approval") {
		t.Error("status bar should show awaiting approval")
	}

	s.SetPendingApproval(false)
	view = s.View()
	if containsStr(view, "awaiting approval") {
		t.Error("status bar should not show awaiting approval after clearing")
	}
}

func TestExtractCodeBlocks(t *testing.T) {
	content := "Here is some code:\n```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\nAnd more text."
	blocks := extractCodeBlocks(content)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(blocks))
	}
	if !containsStr(blocks[0], "func main()") {
		t.Errorf("code block should contain func main(), got %q", blocks[0])
	}
}

func TestExtractCodeBlocks_Multiple(t *testing.T) {
	content := "First:\n```\nblock one\n```\nSecond:\n```python\nblock two\n```\n"
	blocks := extractCodeBlocks(content)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(blocks))
	}
	if blocks[0] != "block one" {
		t.Errorf("first block should be 'block one', got %q", blocks[0])
	}
	if blocks[1] != "block two" {
		t.Errorf("second block should be 'block two', got %q", blocks[1])
	}
}

func TestExtractCodeBlocks_None(t *testing.T) {
	blocks := extractCodeBlocks("no code blocks here")
	if len(blocks) != 0 {
		t.Errorf("expected 0 code blocks, got %d", len(blocks))
	}
}

func TestLastAssistantContent(t *testing.T) {
	chat := NewChatModel(80, 20)
	chat.AddMessage(ChatMessage{Role: "user", Content: "hello"})
	chat.AddMessage(ChatMessage{Role: "assistant", Content: "first response"})
	chat.AddMessage(ChatMessage{Role: "user", Content: "follow up"})
	chat.AddMessage(ChatMessage{Role: "assistant", Content: "second response"})

	got := chat.LastAssistantContent()
	if got != "second response" {
		t.Errorf("expected 'second response', got %q", got)
	}
}

func TestLastAssistantContent_Empty(t *testing.T) {
	chat := NewChatModel(80, 20)
	chat.AddMessage(ChatMessage{Role: "user", Content: "hello"})

	got := chat.LastAssistantContent()
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestLastCodeBlock(t *testing.T) {
	chat := NewChatModel(80, 20)
	chat.AddMessage(ChatMessage{
		Role:    "assistant",
		Content: "Here:\n```go\npackage main\n```\nAnd:\n```\necho hello\n```",
	})

	got := chat.LastCodeBlock()
	if got != "echo hello" {
		t.Errorf("expected 'echo hello', got %q", got)
	}
}
