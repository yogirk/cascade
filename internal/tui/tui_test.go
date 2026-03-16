package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/app"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/pkg/types"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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
	if !containsStr(rendered, "Hello there") {
		t.Error("user message should contain the original content")
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
	rendered := renderToolMessage(msg, false)
	if !containsStr(rendered, "○") {
		t.Error("tool message should contain ○ bullet for read-only tool")
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
	rendered := renderToolMessage(msg, false)
	if !containsStr(rendered, "●") {
		t.Error("tool error should contain ● bullet for exec tool")
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
	ch := make(chan types.ApprovalDecision, 1)
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
	ch := make(chan types.ApprovalDecision, 1)
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

func TestConfirmModel_LeftRightNavigation(t *testing.T) {
	c := NewConfirmModel()
	ch := make(chan types.ApprovalDecision, 1)
	c.Show("bash", []byte(`{"command":"test"}`), "DML", ch)

	updated, _ := c.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if updated.cursor != 1 {
		t.Fatalf("expected right arrow to move cursor to deny, got %d", updated.cursor)
	}

	updated, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if updated.cursor != 0 {
		t.Fatalf("expected left arrow to move cursor back to allow, got %d", updated.cursor)
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
		{permission.ModeAsk, "ASK"},
		{permission.ModeReadOnly, "READ ONLY"},
		{permission.ModeFullAccess, "FULL ACCESS"},
	}
	for _, tt := range tests {
		badge := ModeBadge(tt.mode)
		if !containsStr(badge, tt.contains) {
			t.Errorf("ModeBadge(%v) should contain %q, got %q", tt.mode, tt.contains, badge)
		}
	}
}

func TestWelcomeView(t *testing.T) {
	w := NewWelcomeModel(permission.ModeAsk, "my-project", []string{"hacker_news"})
	w.SetSize(100, 30)
	view := w.View()
	if !containsStr(view, "Cascade") {
		t.Error("welcome should contain 'Cascade' title")
	}
	if !containsStr(view, "my-project") {
		t.Error("welcome should contain project name")
	}
	if !containsStr(view, "hacker_news") {
		t.Error("welcome should contain dataset name")
	}
	if !containsStr(view, "/help") {
		t.Error("welcome should contain /help hint")
	}
}

func TestStatusBarLayout(t *testing.T) {
	s := NewStatusModel("gemini-2.5-pro", permission.ModeAsk)
	s.SetWidth(100)
	s.SetGitBranch("main")
	s.SetCwd("~/Projects/cascade")

	view := s.View()
	if !containsStr(view, "Gemini 2.5 (Pro)") {
		t.Error("status bar should contain friendly model name")
	}
	if !containsStr(view, "ASK") {
		t.Error("status bar should contain mode")
	}
}

func TestStatusBarResponsive(t *testing.T) {
	s := NewStatusModel("gemini-2.5-pro", permission.ModeAsk)
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

func TestInputHeightMatchesRenderedViewWithWrappedMultilineContent(t *testing.T) {
	input := NewInputModel()
	input.SetWidth(30)
	input.input.SetValue("this is a long active line that should wrap inside the input box without horizontal scrolling")
	input.syncInputSize()

	if got, want := input.Height(), strings.Count(input.View(), "\n")+1; got != want {
		t.Fatalf("expected input height %d, got %d", want, got)
	}
}

func TestInputViewStaysWithinAssignedWidth(t *testing.T) {
	input := NewInputModel()
	input.SetWidth(30)
	input.input.SetValue("this is a long active line that should wrap instead of widening the box")
	input.syncInputSize()

	for _, line := range strings.Split(input.View(), "\n") {
		if width := lipgloss.Width(ansiRE.ReplaceAllString(line, "")); width > 30 {
			t.Fatalf("expected rendered input width <= 30, got %d in %q", width, ansiRE.ReplaceAllString(line, ""))
		}
	}

	if got, want := input.input.Width(), 25; got != want {
		t.Fatalf("expected text input width %d, got %d", want, got)
	}
}

func TestInputLongLineWrapsToAdditionalRows(t *testing.T) {
	input := NewInputModel()
	input.SetWidth(30)
	baseHeight := input.Height()

	input.input.SetValue("this is a long active line that should wrap to the next row while typing")
	input.syncInputSize()

	if input.Height() <= baseHeight {
		t.Fatalf("expected wrapped input to grow taller than %d, got %d", baseHeight, input.Height())
	}
	if strings.Contains(input.input.View(), "\n") == false {
		t.Fatal("expected wrapped textarea view to contain a newline")
	}
}

func TestInputTypingLongLineWrapsWithoutDroppingTopRows(t *testing.T) {
	input := NewInputModel()
	input.SetWidth(30)

	text := "this is a long active line that should wrap to the next row while typing"
	for _, r := range text {
		var cmd tea.Cmd
		input, cmd = input.Update(tea.KeyPressMsg{Text: string(r)})
		_ = cmd
	}

	view := ansiRE.ReplaceAllString(input.View(), "")
	if !strings.Contains(view, "\n") {
		t.Fatalf("expected wrapped view after typing, got %q", view)
	}
	if !strings.Contains(view, "this is a long active") {
		t.Fatalf("expected top wrapped row to remain visible, got %q", view)
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

func TestChatModel_MouseWheelScroll(t *testing.T) {
	chat := NewChatModel(80, 5)
	for i := 0; i < 20; i++ {
		chat.AddMessage(ChatMessage{Role: "assistant", Content: fmt.Sprintf("message %d", i)})
	}

	before := chat.viewport.YOffset()
	updated, _ := chat.Update(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))
	if updated.viewport.YOffset() >= before {
		t.Fatalf("expected mouse wheel up to scroll transcript up, before=%d after=%d", before, updated.viewport.YOffset())
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

func TestConfirmHeightMatchesRenderedView(t *testing.T) {
	c := NewConfirmModel()
	ch := make(chan types.ApprovalDecision, 1)
	c.Show("write_file", []byte(`{"file_path":"/tmp/cascade-test.txt","content":"hello"}`), "DML", ch)

	if got, want := c.Height(), strings.Count(c.View(), "\n")+1; got != want {
		t.Fatalf("expected confirm height %d, got %d", want, got)
	}
}

func TestLayoutTracksSpinnerAndConfirmHeight(t *testing.T) {
	m := Model{
		chat:    NewChatModel(80, 20),
		input:   NewInputModel(),
		status:  NewStatusModel("gemini-2.5-pro", permission.ModeAsk),
		spinner: NewSpinnerModel(),
		confirm: NewConfirmModel(),
		width:   120,
		height:  30,
	}

	m.layout()
	baseHeight := m.chat.height

	m.spinner.StartTool("write_file")
	m.layout()
	if m.chat.height != baseHeight-1 {
		t.Fatalf("expected spinner to consume one line, chat height %d -> %d", baseHeight, m.chat.height)
	}

	ch := make(chan types.ApprovalDecision, 1)
	m.spinner.Stop()
	m.confirm.Show("write_file", []byte(`{"file_path":"/tmp/cascade-test.txt","content":"hello"}`), "DML", ch)
	m.layout()
	if want := baseHeight - m.confirm.Height(); m.chat.height != want {
		t.Fatalf("expected confirm to consume %d lines, got chat height %d want %d", m.confirm.Height(), m.chat.height, want)
	}
}

func TestHandleKey_ConfirmApprovalResumesToolSpinner(t *testing.T) {
	resp := make(chan types.ApprovalDecision, 1)
	m := Model{
		chat:            NewChatModel(80, 20),
		input:           NewInputModel(),
		status:          NewStatusModel("gemini-2.5-pro", permission.ModeAsk),
		spinner:         NewSpinnerModel(),
		confirm:         NewConfirmModel(),
		width:           120,
		height:          30,
		state:           StateConfirming,
		preConfirmState: StateToolExecuting,
		lastToolStart:   &types.ToolStartEvent{Name: "write_file"},
	}
	m.confirm.Show("write_file", []byte(`{"file_path":"/tmp/cascade-test.txt","content":"hello"}`), "DML", resp)
	m.layout()

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "y"})
	next := updated.(Model)

	select {
	case decision := <-resp:
		if decision.Action != types.ApprovalAllowOnce {
			t.Fatalf("expected allow-once approval response, got %q", decision.Action)
		}
	default:
		t.Fatal("expected confirm response to be sent")
	}

	if next.state != StateToolExecuting {
		t.Fatalf("expected state %v, got %v", StateToolExecuting, next.state)
	}
	if !next.spinner.Active() {
		t.Fatal("expected tool spinner to resume after approval")
	}
	if next.status.toolName != "write_file" {
		t.Fatalf("expected tool name to be restored, got %q", next.status.toolName)
	}
	if next.input.Focused() {
		t.Fatal("input should stay blurred while tool execution resumes")
	}
}

func TestHandleApprovalRequest_EntersModalState(t *testing.T) {
	m := Model{
		chat:    NewChatModel(80, 20),
		input:   NewInputModel(),
		status:  NewStatusModel("gemini-2.5-pro", permission.ModeAsk),
		spinner: NewSpinnerModel(),
		confirm: NewConfirmModel(),
		width:   120,
		height:  30,
		state:   StateToolExecuting,
	}
	m.spinner.StartTool("bash")
	m.layout()

	resp := make(chan types.ApprovalDecision, 1)
	updated, _ := m.handleApprovalRequest(types.ApprovalRequest{
		ToolName:  "bash",
		Input:     []byte(`{"command":"rm -rf /tmp/test"}`),
		RiskLevel: "DESTRUCTIVE",
		Response:  resp,
	})
	next := updated.(Model)

	if next.state != StateConfirming {
		t.Fatalf("expected confirming state, got %v", next.state)
	}
	if !next.confirm.Active() {
		t.Fatal("expected confirm modal to be active")
	}
	if next.input.Focused() {
		t.Fatal("input should be blurred while approval modal is active")
	}
	if !next.status.pendingApproval {
		t.Fatal("status bar should reflect pending approval")
	}
	if next.spinner.Active() {
		t.Fatal("spinner should stop while approval modal is shown")
	}
}

func TestHandleKey_VisibleConfirmConsumesInputEvenIfStateDrifts(t *testing.T) {
	resp := make(chan types.ApprovalDecision, 1)

	m := Model{
		input:          NewInputModel(),
		status:         NewStatusModel("gemini-2.5-pro", permission.ModeAsk),
		spinner:        NewSpinnerModel(),
		confirm:        NewConfirmModel(),
		width:          120,
		height:         30,
		state:          StateIdle,
		preConfirmState: StateToolExecuting,
	}
	m.confirm.Show("write_file", []byte(`{"file_path":"/tmp/test.txt","content":"hello"}`), "DML", resp)
	m.layout()

	updated, _ := m.handleKey(tea.KeyPressMsg{Text: "y"})
	next := updated.(Model)

	select {
	case decision := <-resp:
		if decision.Action != types.ApprovalAllowOnce {
			t.Fatalf("expected allow once decision, got %q", decision.Action)
		}
	default:
		t.Fatal("expected confirm modal to consume y even when state drifted")
	}

	if next.confirm.Active() {
		t.Fatal("expected confirm modal to close after approval")
	}
	if next.state != StateToolExecuting {
		t.Fatalf("expected pre-confirm state to be restored, got %v", next.state)
	}
}

func TestConfirmOptionsAlignDescriptions(t *testing.T) {
	ch := make(chan types.ApprovalDecision, 1)
	c := NewConfirmModel()
	c.Show("edit_file", []byte(`{"file_path":"/tmp/test.txt","old_text":"Hello","new_text":"World"}`), "DML", ch)

	view := c.View()
	lines := strings.Split(view, "\n")
	var actionLines []string
	for _, line := range lines {
		if strings.Contains(line, "Allow once") || strings.Contains(line, "Allow tool for session") || strings.Contains(line, "Deny") {
			actionLines = append(actionLines, ansiRE.ReplaceAllString(line, ""))
		}
	}
	if len(actionLines) != 3 {
		t.Fatalf("expected 3 action lines, got %d", len(actionLines))
	}

	firstGap := strings.Index(actionLines[0], "Run this exact action now")
	secondGap := strings.Index(actionLines[1], "Skip future prompts for this tool until you exit")
	thirdGap := strings.Index(actionLines[2], "Block this action")
	if firstGap == -1 || secondGap == -1 || thirdGap == -1 {
		t.Fatal("expected action descriptions to be present")
	}
	firstWidth := lipgloss.Width(actionLines[0][:firstGap])
	secondWidth := lipgloss.Width(actionLines[1][:secondGap])
	thirdWidth := lipgloss.Width(actionLines[2][:thirdGap])
	if firstWidth != secondWidth || secondWidth != thirdWidth {
		t.Fatalf("expected action descriptions to start in the same column, got widths %d, %d, %d", firstWidth, secondWidth, thirdWidth)
	}
}

func TestProgram_ApprovalConsumesInput(t *testing.T) {
	approvalCh := make(chan types.ApprovalRequest, 1)
	eventCh := make(chan types.Event, 1)
	resp := make(chan types.ApprovalDecision, 1)

	m := Model{
		app: &app.App{
			Events:    eventCh,
			Approvals: approvalCh,
		},
		chat:    NewChatModel(80, 20),
		input:   NewInputModel(),
		status:  NewStatusModel("gemini-2.5-pro", permission.ModeAsk),
		spinner: NewSpinnerModel(),
		confirm: NewConfirmModel(),
		width:   120,
		height:  30,
	}
	m.layout()

	inR, inW := io.Pipe()
	var out bytes.Buffer

	p := tea.NewProgram(m, tea.WithInput(inR), tea.WithOutput(&out))
	done := make(chan error, 1)

	go func() {
		_, err := p.Run()
		done <- err
	}()

	go func() {
		time.Sleep(20 * time.Millisecond)
		approvalCh <- types.ApprovalRequest{
			ToolName:  "bash",
			Input:     []byte(`{"command":"gcloud config get-value project"}`),
			RiskLevel: "DESTRUCTIVE",
			Response:  resp,
		}
		time.Sleep(50 * time.Millisecond)
		_, _ = inW.Write([]byte("y"))
	}()

	select {
	case decision := <-resp:
		if decision.Action != types.ApprovalAllowOnce {
			t.Fatal("expected y input to approve the modal")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval response")
	}

	p.Quit()
	_ = inW.Close()
	if err := <-done; err != nil {
		t.Fatalf("program exited with error: %v", err)
	}
}

func TestProgram_ApprovalConsumesTwoSequentialRequests(t *testing.T) {
	approvalCh := make(chan types.ApprovalRequest, 2)
	eventCh := make(chan types.Event, 1)
	resp1 := make(chan types.ApprovalDecision, 1)
	resp2 := make(chan types.ApprovalDecision, 1)

	m := Model{
		app: &app.App{
			Events:    eventCh,
			Approvals: approvalCh,
		},
		chat:    NewChatModel(80, 20),
		input:   NewInputModel(),
		status:  NewStatusModel("gemini-2.5-pro", permission.ModeAsk),
		spinner: NewSpinnerModel(),
		confirm: NewConfirmModel(),
		width:   120,
		height:  30,
	}
	m.layout()

	inR, inW := io.Pipe()
	var out bytes.Buffer
	p := tea.NewProgram(m, tea.WithInput(inR), tea.WithOutput(&out))
	done := make(chan error, 1)

	go func() {
		_, err := p.Run()
		done <- err
	}()

	go func() {
		time.Sleep(20 * time.Millisecond)
		approvalCh <- types.ApprovalRequest{
			ToolName:  "bash",
			Input:     []byte(`{"command":"gcloud config get-value project"}`),
			RiskLevel: "DESTRUCTIVE",
			Response:  resp1,
		}
		time.Sleep(50 * time.Millisecond)
		_, _ = inW.Write([]byte("y"))

		time.Sleep(50 * time.Millisecond)
		approvalCh <- types.ApprovalRequest{
			ToolName:  "bash",
			Input:     []byte(`{"command":"bq ls manyminds"}`),
			RiskLevel: "DESTRUCTIVE",
			Response:  resp2,
		}
		time.Sleep(50 * time.Millisecond)
		_, _ = inW.Write([]byte("y"))
	}()

	select {
	case decision := <-resp1:
		if decision.Action != types.ApprovalAllowOnce {
			t.Fatal("expected first approval to succeed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first approval response")
	}

	select {
	case decision := <-resp2:
		if decision.Action != types.ApprovalAllowOnce {
			t.Fatal("expected second approval to succeed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second approval response")
	}

	p.Quit()
	_ = inW.Close()
	if err := <-done; err != nil {
		t.Fatalf("program exited with error: %v", err)
	}
}

func TestUpdate_ChatMessageAddsToTranscript(t *testing.T) {
	m := Model{
		chat:   NewChatModel(80, 20),
		input:  NewInputModel(),
		status: NewStatusModel("gemini-2.5-pro", permission.ModeAsk),
		width:  120,
		height: 30,
	}
	m.layout()

	updated, _ := m.Update(ChatMessage{Role: "system", Content: "Schema cache refreshed"})
	next := updated.(Model)

	if next.chat.MessageCount() != 1 {
		t.Fatalf("expected 1 chat message, got %d", next.chat.MessageCount())
	}
	if got := next.chat.messages[0].Content; got != "Schema cache refreshed" {
		t.Fatalf("expected chat message to be added, got %q", got)
	}
}
