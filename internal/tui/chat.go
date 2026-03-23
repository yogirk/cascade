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
	Role     string          // "user", "assistant", "tool", "error", "system"
	Content  string
	ToolName string          // for role="tool"
	ToolArgs json.RawMessage // for role="tool"
	Display  string          // for role="tool": formatted display (diffs)
	IsError  bool            // for role="tool"
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
	// Rebuilt only on Clear/SetSize. Extended incrementally by AddMessage.
	rendered []string // rendered[i] = rendered output of messages[i]
	cache    string   // joined rendered output (immutable transcript)

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
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return ChatModel{
		viewport:   vp,
		messages:   make([]ChatMessage, 0),
		rendered:   make([]string, 0),
		followTail: true,
		width:      width,
		height:     height,
	}
}

// Update handles viewport messages (scrolling, etc.).
func (c ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	var cmd tea.Cmd
	c.viewport, cmd = c.viewport.Update(msg)
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

	// Render just this message and append to cache
	r := renderMessageAt(msg, c.width, idx) + "\n\n"
	c.rendered = append(c.rendered, r)
	c.cache += r

	c.viewport.SetContent(c.cache)
	if c.followTail {
		c.viewport.GotoBottom()
	}
}

// SetStreamingContent appends in-progress streaming text to the cached
// transcript. No historical messages are re-rendered.
func (c *ChatModel) SetStreamingContent(content string) {
	if content != "" {
		c.viewport.SetContent(c.cache + content)
	} else {
		c.viewport.SetContent(c.cache)
	}
	if c.followTail {
		c.viewport.GotoBottom()
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
	c.followTail = true
	c.viewport.SetContent("")
	c.viewport.GotoBottom()
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
		r := renderMessageAt(msg, c.width, i) + "\n\n"
		c.rendered[i] = r
		sb.WriteString(r)
	}
	c.cache = sb.String()
	c.viewport.SetContent(c.cache)
	if c.followTail {
		c.viewport.GotoBottom()
	}
}

// --- Message rendering (stateless, called once per message) ---

// renderMessageAt formats a single chat message, using its position
// to decide whether to show a turn separator.
func renderMessageAt(msg ChatMessage, width int, index int) string {
	return renderMessageOpts(msg, width, index > 0)
}

// renderMessage formats a single chat message (for streaming, index unknown).
func renderMessage(msg ChatMessage, width int) string {
	return renderMessageOpts(msg, width, true)
}

func renderMessageOpts(msg ChatMessage, width int, showSep bool) string {
	switch msg.Role {
	case "user":
		prefix := ""
		if showSep {
			prefix = turnSeparator(width) + "\n"
		}
		return prefix + UserPromptStyle.Render("> ") + msg.Content
	case "assistant":
		return AssistantBulletStyle.Render("≋") + " " + renderMarkdown(msg.Content, width-2)
	case "tool":
		return renderToolMessage(msg)
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

// renderToolMessage renders a tool call with ∞ bullet, name, args, and indented output.
func renderToolMessage(msg ChatMessage) string {
	var sb strings.Builder

	// Header: ∞ tool_name args (bullet color by tool category)
	sb.WriteString(ToolBullet(msg.ToolName))
	sb.WriteString(" ")

	name := msg.ToolName
	if name == "" {
		name = "tool"
	}
	sb.WriteString(ToolNameStyle.Render(name))

	args := abbreviateArgs(msg.ToolArgs)
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

	if msg.IsError {
		// Error output: red with "!" prefix
		lines := strings.Split(strings.TrimRight(display, "\n"), "\n")
		for _, line := range lines {
			sb.WriteString(ToolErrorStyle.Render("! " + line))
			sb.WriteString("\n")
		}
		return strings.TrimRight(sb.String(), "\n")
	}

	// Check if it's a diff
	if isDiff(display) {
		sb.WriteString(renderDiff(display))
		return strings.TrimRight(sb.String(), "\n")
	}

	// Normal output: show first + last lines for long output
	lines := strings.Split(strings.TrimRight(display, "\n"), "\n")
	headLines := 10
	tailLines := 3
	maxLines := headLines + tailLines

	if len(lines) <= maxLines {
		for _, line := range lines {
			sb.WriteString("    ")
			sb.WriteString(StatusDimStyle.Render(line))
			sb.WriteString("\n")
		}
	} else {
		for _, line := range lines[:headLines] {
			sb.WriteString("    ")
			sb.WriteString(StatusDimStyle.Render(line))
			sb.WriteString("\n")
		}
		omitted := len(lines) - headLines - tailLines
		sb.WriteString("    ")
		sb.WriteString(StatusDimStyle.Render(fmt.Sprintf("··· %d lines omitted ···", omitted)))
		sb.WriteString("\n")
		for _, line := range lines[len(lines)-tailLines:] {
			sb.WriteString("    ")
			sb.WriteString(StatusDimStyle.Render(line))
			sb.WriteString("\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
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
