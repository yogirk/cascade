package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
)

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role          string          // "user", "assistant", "tool", "error", "system"
	Content       string
	ToolName      string          // for role="tool"
	ToolArgs      json.RawMessage // for role="tool"
	ToolRiskLevel string          // for role="tool": risk level for bullet glyph selection
	Display       string          // for role="tool": formatted display (diffs)
	IsError       bool            // for role="tool"
}

// ChatModel manages the scrollable chat message history.
//
// Architecture: completed messages are rendered once and cached. During
// streaming, only the live suffix is appended to the cache — Glamour is
// never re-run on historical messages. An explicit followTail flag
// controls whether the viewport auto-scrolls.
type ChatModel struct {
	viewport viewport.Model
	messages []ChatMessage

	// Render cache: pre-rendered output for all completed messages.
	// Rebuilt only on Clear/SetSize/ToggleExpand. Extended incrementally by AddMessage.
	rendered []string // rendered[i] = rendered output of messages[i]
	cache    string   // joined rendered output (immutable transcript)

	// expandedSet tracks which truncated tool messages are expanded.
	// Key is the message index. Toggled by Space key in viewport mode.
	expandedSet map[int]bool

	// followTail controls auto-scroll behavior. True by default.
	// Set to false when the user manually scrolls up. Reset to true
	// when the user scrolls back to the bottom or a new turn starts.
	followTail bool

	width  int
	height int
}

// NewChatModel creates a new chat model with the given dimensions.
func NewChatModel(width, height int) ChatModel {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))

	// Disable viewport's built-in key bindings — we handle scrolling ourselves
	// to avoid conflicts with typing (j, k, d, u, f, b, space are pager keys).
	km := vp.KeyMap
	km.Up.SetEnabled(false)
	km.Down.SetEnabled(false)
	km.PageUp.SetEnabled(false)
	km.PageDown.SetEnabled(false)
	km.HalfPageUp.SetEnabled(false)
	km.HalfPageDown.SetEnabled(false)
	vp.KeyMap = km
	vp.MouseWheelEnabled = true // Viewport handles scroll; followTail tracked in Update
	vp.MouseWheelDelta = 3

	return ChatModel{
		viewport:    vp,
		messages:    make([]ChatMessage, 0),
		rendered:    make([]string, 0),
		expandedSet: make(map[int]bool),
		followTail:  true,
		width:       width,
		height:      height,
	}
}

// Update handles viewport messages (scrolling, etc.).
// Tracks followTail state when the viewport handles mouse wheel internally.
func (c ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)

	// After viewport processes a wheel event, sync followTail
	if _, ok := msg.(tea.MouseWheelMsg); ok {
		if c.viewport.AtBottom() {
			c.followTail = true
		} else {
			c.followTail = false
		}
	}

	return c, cmd
}

// View renders the chat viewport.
func (c ChatModel) View() string {
	return c.viewport.View()
}

// AddMessage appends a message and renders it incrementally (no re-render of history).
func (c *ChatModel) AddMessage(msg ChatMessage) {
	idx := len(c.messages)
	c.messages = append(c.messages, msg)

	// Consecutive tool messages get tight spacing (no blank line between them).
	// Everything else gets a blank line separator.
	spacing := "\n\n"
	if msg.Role == "tool" && idx > 0 && c.messages[idx-1].Role == "tool" {
		spacing = "\n"
	}
	r := renderMessageAt(msg, c.width, idx) + spacing
	c.rendered = append(c.rendered, r)
	c.cache += r

	c.setContentPreserveScroll(c.cache)
}

// SetStreamingContent appends in-progress streaming text to the cached
// transcript. No historical messages are re-rendered.
func (c *ChatModel) SetStreamingContent(content string) {
	if content != "" {
		c.setContentPreserveScroll(c.cache + content)
	} else {
		c.setContentPreserveScroll(c.cache)
	}
}

// setContentPreserveScroll updates viewport content while preserving
// the user's scroll position when followTail is false.
func (c *ChatModel) setContentPreserveScroll(content string) {
	yOffset := c.viewport.YOffset()
	c.viewport.SetContent(content)
	if c.followTail {
		c.viewport.GotoBottom()
	} else {
		c.viewport.SetYOffset(yOffset)
	}
}

// SetSize updates the chat viewport dimensions. No-ops if size unchanged.
func (c *ChatModel) SetSize(width, height int) {
	if c.width == width && c.height == height {
		return
	}
	c.width = width
	c.height = height
	c.viewport.SetWidth(width)
	c.viewport.SetHeight(height)
	c.rebuildCache() // Width changed — re-render everything
}

// ScrollUp scrolls the viewport up by n lines and disables auto-follow.
func (c *ChatModel) ScrollUp(n int) {
	c.viewport.ScrollUp(n)
	c.followTail = false
}

// ScrollDown scrolls the viewport down by n lines.
// Re-enables auto-follow if the user reaches the bottom.
func (c *ChatModel) ScrollDown(n int) {
	c.viewport.ScrollDown(n)
	if c.viewport.AtBottom() {
		c.followTail = true
	}
}

// HalfPageUp scrolls the viewport up by half a page and disables auto-follow.
func (c *ChatModel) HalfPageUp() {
	c.viewport.HalfPageUp()
	c.followTail = false
}

// HalfPageDown scrolls the viewport down by half a page.
// Re-enables auto-follow if the user reaches the bottom.
func (c *ChatModel) HalfPageDown() {
	c.viewport.HalfPageDown()
	if c.viewport.AtBottom() {
		c.followTail = true
	}
}

// ResumeFollow re-enables tail following (called on new turn start).
func (c *ChatModel) ResumeFollow() {
	c.followTail = true
}

// Clear removes all messages, cache, and resets the viewport.
func (c *ChatModel) Clear() {
	c.messages = c.messages[:0]
	c.rendered = c.rendered[:0]
	c.cache = ""
	c.expandedSet = make(map[int]bool)
	c.followTail = true
	c.viewport.SetContent("")
	c.viewport.GotoBottom()
}

// ToggleExpand cycles through truncated tool messages in reverse order.
// Each press expands the next collapsed message. When all are expanded,
// the next press collapses them all.
func (c *ChatModel) ToggleExpand() bool {
	// Collect indices of all expandable tool messages (truncated, non-error, non-diff)
	var expandable []int
	for i := len(c.messages) - 1; i >= 0; i-- {
		msg := c.messages[i]
		if msg.Role != "tool" || msg.IsError {
			continue
		}
		display := msg.Content
		if msg.Display != "" {
			display = msg.Display
		}
		if isDiff(display) {
			continue
		}
		lines := strings.Split(strings.TrimRight(display, "\n"), "\n")
		if len(lines) <= 3 { // defaultVisible
			continue
		}
		expandable = append(expandable, i)
	}

	if len(expandable) == 0 {
		return false
	}

	// Find the first collapsed message (most recent first)
	for _, idx := range expandable {
		if !c.expandedSet[idx] {
			c.expandedSet[idx] = true
			c.rebuildCache()
			return true
		}
	}

	// All expanded — collapse them all
	for _, idx := range expandable {
		delete(c.expandedSet, idx)
	}
	c.rebuildCache()
	return true
	return false
}

// LastAssistantContent returns the raw content of the most recent assistant message.
func (c *ChatModel) LastAssistantContent() string {
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.messages[i].Role == "assistant" {
			return c.messages[i].Content
		}
	}
	return ""
}

// LastCodeBlock extracts the last fenced code block from the most recent
// assistant message. Returns empty string if no code block is found.
func (c *ChatModel) LastCodeBlock() string {
	content := c.LastAssistantContent()
	if content == "" {
		return ""
	}

	// Find the last ``` ... ``` block
	blocks := extractCodeBlocks(content)
	if len(blocks) == 0 {
		return ""
	}
	return blocks[len(blocks)-1]
}

// extractCodeBlocks pulls all fenced code blocks from markdown content.
func extractCodeBlocks(content string) []string {
	var blocks []string
	lines := strings.Split(content, "\n")
	inBlock := false
	var current []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inBlock {
				// End of block
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
				inBlock = false
			} else {
				// Start of block
				inBlock = true
				current = nil
			}
			continue
		}
		if inBlock {
			current = append(current, line)
		}
	}
	return blocks
}

// MessageCount returns the number of messages in the chat.
func (c *ChatModel) MessageCount() int {
	return len(c.messages)
}

// rebuildCache re-renders all messages from scratch. Called only when
// the viewport width changes (messages may wrap differently).
func (c *ChatModel) rebuildCache() {
	c.rendered = make([]string, len(c.messages))
	var sb strings.Builder
	for i, msg := range c.messages {
		expanded := c.expandedSet[i]
		spacing := "\n\n"
		if msg.Role == "tool" && i > 0 && c.messages[i-1].Role == "tool" {
			spacing = "\n"
		}
		r := renderMessageExpanded(msg, c.width, i, expanded) + spacing
		c.rendered[i] = r
		sb.WriteString(r)
	}
	c.cache = sb.String()
	c.setContentPreserveScroll(c.cache)
}

// --- Message rendering (stateless, called once per message) ---

// renderMessageAt formats a single chat message, using its position
// to decide whether to show a turn separator.
func renderMessageAt(msg ChatMessage, width int, index int) string {
	return renderMessageFull(msg, width, index > 0, false)
}

// renderMessageExpanded formats a message with optional expansion for tool output.
func renderMessageExpanded(msg ChatMessage, width int, index int, expanded bool) string {
	return renderMessageFull(msg, width, index > 0, expanded)
}

// renderMessage formats a single chat message (for streaming, index unknown).
func renderMessage(msg ChatMessage, width int) string {
	return renderMessageFull(msg, width, true, false)
}

func renderMessageFull(msg ChatMessage, width int, showSep bool, expanded bool) string {
	switch msg.Role {
	case "user":
		// Match the input box dimensions: full width, same padding
		barWidth := width - 4 // account for border + padding
		if barWidth < 20 {
			barWidth = 20
		}
		bar := UserMessageBarStyle.Width(barWidth).Render(msg.Content)
		if showSep {
			return turnSeparator(width) + "\n\n" + bar
		}
		return bar
	case "assistant":
		return AssistantBulletStyle.Render("≋") + " " + renderMarkdown(msg.Content, width-2)
	case "tool":
		return renderToolMessage(msg, expanded)
	case "error":
		return ErrorPrefixStyle.Render("!") + " " + msg.Content
	case "welcome":
		return msg.Content // Already styled by WelcomeModel
	case "system":
		if msg.Display != "" {
			return msg.Display // Pre-styled (e.g., /insights, /cost)
		}
		return SystemMsgStyle.Render(msg.Content)
	default:
		return msg.Content
	}
}


// turnSeparator renders a dim horizontal rule for visual turn separation.
func turnSeparator(width int) string {
	w := width - 2
	if w < 10 {
		w = 10
	}
	if w > 80 {
		w = 80
	}
	return SeparatorStyle.Render(strings.Repeat("─", w))
}

// renderToolMessage renders a tool call with shape bullet, dim name, compact args,
// and indented output collapsed to 3 lines by default.
func renderToolMessage(msg ChatMessage, expanded bool) string {
	var sb strings.Builder

	// Header: ○ tool_name args (shape bullet + dim name + dimmer args)
	sb.WriteString(ToolBulletByRisk(msg.ToolName, msg.ToolRiskLevel))
	sb.WriteString(" ")

	name := msg.ToolName
	if name == "" {
		name = "tool"
	}
	sb.WriteString(ToolNameStyle.Render(name))

	args := compactArgs(msg.ToolArgs)
	if args != "" {
		sb.WriteString(" ")
		sb.WriteString(StatusDimStyle.Render(args))
	}

	// Body: display or content, indented
	display := msg.Display
	if display == "" {
		display = msg.Content
	}
	if display == "" {
		return sb.String()
	}

	sb.WriteString("\n")

	// Errors: show in full (user needs the stack trace)
	if msg.IsError {
		lines := strings.Split(strings.TrimRight(display, "\n"), "\n")
		for _, line := range lines {
			sb.WriteString(ToolErrorStyle.Render("! " + line))
			sb.WriteString("\n")
		}
		return strings.TrimRight(sb.String(), "\n")
	}

	// Diffs: show in full (truncation hides context)
	if isDiff(display) {
		sb.WriteString(renderDiff(display))
		return strings.TrimRight(sb.String(), "\n")
	}

	// Normal output: 3 lines default, expandable with Ctrl+E
	lines := strings.Split(strings.TrimRight(display, "\n"), "\n")
	const defaultVisible = 3

	if len(lines) <= defaultVisible || expanded {
		for _, line := range lines {
			sb.WriteString("    ")
			sb.WriteString(StatusDimStyle.Render(line))
			sb.WriteString("\n")
		}
		if expanded && len(lines) > defaultVisible {
			sb.WriteString("    ")
			sb.WriteString(StatusDimStyle.Faint(true).Render("[Ctrl+E to collapse]"))
			sb.WriteString("\n")
		}
	} else {
		for _, line := range lines[:defaultVisible] {
			sb.WriteString("    ")
			sb.WriteString(StatusDimStyle.Render(line))
			sb.WriteString("\n")
		}
		sb.WriteString("    ")
		sb.WriteString(StatusDimStyle.Faint(true).Render(fmt.Sprintf("... [%d more lines] Ctrl+E", len(lines)-defaultVisible)))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// compactArgs extracts key values from tool args for a compact header display.
// Returns just the values (e.g., "src/main.go" not "file_path=src/main.go"),
// truncated to 40 characters.
func compactArgs(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}

	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return ""
	}

	// Priority keys: show the most meaningful value
	for _, key := range []string{"file_path", "command", "sql", "pattern", "action", "query", "bucket"} {
		if v, ok := args[key]; ok {
			s := fmt.Sprintf("%v", v)
			if len(s) > 40 {
				s = s[:37] + "..."
			}
			return s
		}
	}

	// Fallback: first value
	for _, v := range args {
		s := fmt.Sprintf("%v", v)
		if len(s) > 40 {
			s = s[:37] + "..."
		}
		return s
	}

	return ""
}

// isDiff checks if content looks like a unified diff.
func isDiff(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "@@")
}

// renderDiff parses unified diff content and applies diff coloring.
func renderDiff(diff string) string {
	var sb strings.Builder
	lines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	for _, line := range lines {
		styled := "    " // 4-space indent
		switch {
		case strings.HasPrefix(line, "@@"):
			styled += DiffHunkStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			styled += DiffAddStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			styled += DiffRemoveStyle.Render(line)
		default:
			styled += StatusDimStyle.Render(line)
		}
		sb.WriteString(styled)
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderMarkdown renders markdown content using Glamour for completed messages.
func renderMarkdown(content string, width int) string {
	w := width - 4
	if w < 40 {
		w = 40
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(cascadeMarkdownStyle()),
		glamour.WithWordWrap(w),
	)
	if err != nil {
		return content
	}
	rendered, err := r.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

// FormatToolResult formats a tool result for display in the chat.
func FormatToolResult(toolName, content string, isError bool) string {
	if isError {
		return fmt.Sprintf("Failed: %s\n%s", toolName, content)
	}
	lines := strings.Split(content, "\n")
	if len(lines) > 20 {
		content = strings.Join(lines[:20], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-20)
	}
	return fmt.Sprintf("Executed: %s\n%s", toolName, content)
}
