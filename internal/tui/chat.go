package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role          string // "user", "assistant", "tool", "error", "system"
	Content       string
	ToolName      string          // for role="tool"
	ToolArgs      json.RawMessage // for role="tool"
	ToolRiskLevel string          // for role="tool": risk level for bullet glyph selection
	Display       string          // for role="tool": formatted display (diffs)
	IsError       bool            // for role="tool"
}

// lineRange is the line span (in viewport content lines) occupied by a
// rendered message. start is inclusive, end is exclusive. Used to map a
// click's Y coordinate back to a message index.
type lineRange struct{ start, end int }

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
	rendered   []string    // rendered[i] = rendered output of messages[i]
	cache      string      // joined rendered output (immutable transcript)
	lineRanges []lineRange // lineRanges[i] = viewport line span of messages[i]

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
		lineRanges:  make([]lineRange, 0),
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

	start := 0
	if idx > 0 {
		start = c.lineRanges[idx-1].end
	}
	c.lineRanges = append(c.lineRanges, lineRange{start: start, end: start + strings.Count(r, "\n")})
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
	c.lineRanges = c.lineRanges[:0]
	c.cache = ""
	c.expandedSet = make(map[int]bool)
	c.followTail = true
	c.viewport.SetContent("")
	c.viewport.GotoBottom()
}

// isExpandableTool reports whether the message at idx is a truncated tool
// output that can be expanded/collapsed (non-error, non-diff, > defaultVisible
// lines of body content).
func (c *ChatModel) isExpandableTool(idx int) bool {
	if idx < 0 || idx >= len(c.messages) {
		return false
	}
	msg := c.messages[idx]
	if msg.Role != "tool" || msg.IsError {
		return false
	}
	display := msg.Content
	if msg.Display != "" {
		display = msg.Display
	}
	if isDiff(display) {
		return false
	}
	lines := strings.Split(strings.TrimRight(display, "\n"), "\n")
	return len(lines) > 3 // defaultVisible
}

// MessageAtLine returns the index of the message that owns the given absolute
// content line (i.e. line index within the joined cache, NOT a terminal Y).
// Returns -1 if absLine falls outside any message.
func (c *ChatModel) MessageAtLine(absLine int) int {
	if absLine < 0 {
		return -1
	}
	for i, r := range c.lineRanges {
		if absLine >= r.start && absLine < r.end {
			return i
		}
	}
	return -1
}

// MessageAtViewportY maps a Y coordinate within the chat viewport (0 = top
// visible row) to a message index, accounting for the current scroll offset.
// Returns -1 if no message is at that line or y is out of range.
func (c *ChatModel) MessageAtViewportY(y int) int {
	if y < 0 || y >= c.height {
		return -1
	}
	return c.MessageAtLine(c.viewport.YOffset() + y)
}

// FocusedExpandableTool returns the index of the bottom-most expandable tool
// message currently visible in the viewport. Returns -1 if none visible.
func (c *ChatModel) FocusedExpandableTool() int {
	vpTop := c.viewport.YOffset()
	vpBottom := vpTop + c.height
	best := -1
	for i := range c.messages {
		if !c.isExpandableTool(i) {
			continue
		}
		r := c.lineRanges[i]
		if r.end <= vpTop || r.start >= vpBottom {
			continue
		}
		best = i // keep updating; later messages overwrite earlier ones
	}
	return best
}

// ToggleExpandAt flips the expanded state of the tool message at idx. Returns
// true if the state changed (i.e. idx was an expandable tool).
func (c *ChatModel) ToggleExpandAt(idx int) bool {
	if !c.isExpandableTool(idx) {
		return false
	}
	if c.expandedSet[idx] {
		delete(c.expandedSet, idx)
	} else {
		c.expandedSet[idx] = true
	}
	c.rebuildCache()
	return true
}

// ToggleExpand toggles the expand state of the bottom-most expandable tool
// block currently in view. Falls back to the most recent expandable tool in
// the transcript when nothing matching is visible. Used by the Ctrl+E binding.
func (c *ChatModel) ToggleExpand() bool {
	if idx := c.FocusedExpandableTool(); idx >= 0 {
		return c.ToggleExpandAt(idx)
	}
	for i := len(c.messages) - 1; i >= 0; i-- {
		if c.isExpandableTool(i) {
			return c.ToggleExpandAt(i)
		}
	}
	return false
}

// ChatHeight returns the current viewport height in rows.
func (c *ChatModel) ChatHeight() int { return c.height }

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

// ForceRebuild re-renders the full transcript from scratch. Call this after
// any change that affects how messages render but doesn't change their
// content — e.g. theme switch via /theme, where every message must pick up
// the new palette's styles.
func (c *ChatModel) ForceRebuild() {
	c.rebuildCache()
}

// RefreshWelcomeSnapshot replaces the Content of every welcome-role message
// with newContent, then rebuilds the cache. Welcome messages store a
// pre-rendered ANSI string (welcome banners aren't re-styled at render time,
// they're snapshotted on first submit for scrollback). After a theme switch
// those snapshots are baked with the previous palette, so `/theme` must
// call this to replace them with a fresh render.
func (c *ChatModel) RefreshWelcomeSnapshot(newContent string) {
	changed := false
	for i := range c.messages {
		if c.messages[i].Role == "welcome" {
			c.messages[i].Content = newContent
			changed = true
		}
	}
	if changed {
		c.rebuildCache()
	}
}

// rebuildCache re-renders all messages from scratch. Called only when
// the viewport width changes (messages may wrap differently).
func (c *ChatModel) rebuildCache() {
	c.rendered = make([]string, len(c.messages))
	c.lineRanges = make([]lineRange, len(c.messages))
	var sb strings.Builder
	cursor := 0
	for i, msg := range c.messages {
		expanded := c.expandedSet[i]
		spacing := "\n\n"
		if msg.Role == "tool" && i > 0 && c.messages[i-1].Role == "tool" {
			spacing = "\n"
		}
		r := renderMessageExpanded(msg, c.width, i, expanded) + spacing
		c.rendered[i] = r
		nl := strings.Count(r, "\n")
		c.lineRanges[i] = lineRange{start: cursor, end: cursor + nl}
		cursor += nl
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

	argKey, argVal := compactArgs(msg.ToolArgs)
	if argVal != "" {
		sb.WriteString(" ")
		if lang := languageForArgKey(argKey); lang != "" {
			sb.WriteString(highlightCode(argVal, lang))
		} else {
			sb.WriteString(StatusDimStyle.Render(argVal))
		}
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

	// Errors: show in full (user needs the stack trace). The "!" glyph rides
	// the branch line; continuation lines just align under the body.
	if msg.IsError {
		lines := strings.Split(strings.TrimRight(display, "\n"), "\n")
		for i, line := range lines {
			sb.WriteString(bodyPrefix(i == 0))
			if i == 0 {
				sb.WriteString(ToolErrorStyle.Render("! " + line))
			} else {
				sb.WriteString(ToolErrorStyle.Render(line))
			}
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
		for i, line := range lines {
			sb.WriteString(bodyPrefix(i == 0))
			sb.WriteString(StatusDimStyle.Render(line))
			sb.WriteString("\n")
		}
		if expanded && len(lines) > defaultVisible {
			sb.WriteString(branchIndent)
			sb.WriteString(StatusDimStyle.Faint(true).Render("[Ctrl+E or click to collapse]"))
			sb.WriteString("\n")
		}
	} else {
		for i, line := range lines[:defaultVisible] {
			sb.WriteString(bodyPrefix(i == 0))
			sb.WriteString(StatusDimStyle.Render(line))
			sb.WriteString("\n")
		}
		sb.WriteString(branchIndent)
		sb.WriteString(StatusDimStyle.Faint(true).Render(fmt.Sprintf("... [%d more lines] Ctrl+E or click", len(lines)-defaultVisible)))
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// branchIndent is the 5-cell continuation indent for tool body lines after the
// first. It aligns lines under the content position established by branchHead.
const branchIndent = "     "

// branchHead returns the styled "  ⎿  " prefix that connects a tool's header
// to its body, mirroring Claude Code's information hierarchy. The glyph uses
// the dim separator color so it reads as background structure, not content.
func branchHead() string {
	return "  " + SeparatorStyle.Render("⎿") + "  "
}

// bodyPrefix returns branchHead for the first body line and branchIndent for
// every line after. Centralised so every tool-body sub-path (errors, diffs,
// normal output) renders the hierarchy identically.
func bodyPrefix(first bool) string {
	if first {
		return branchHead()
	}
	return branchIndent
}

// compactArgs extracts a key/value pair from tool args for a compact header
// display. Returns the matched key plus its value (e.g. "sql", "SELECT ..."),
// with the value truncated to 40 characters. Returning the key lets the
// caller choose a renderer — e.g. SQL syntax highlighting for sql/query.
func compactArgs(input json.RawMessage) (key, value string) {
	if len(input) == 0 {
		return "", ""
	}

	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", ""
	}

	truncate := func(s string) string {
		if len(s) > 40 {
			return s[:37] + "..."
		}
		return s
	}

	// Priority keys: show the most meaningful value
	for _, k := range []string{"file_path", "command", "sql", "pattern", "action", "query", "bucket"} {
		if v, ok := args[k]; ok {
			return k, truncate(fmt.Sprintf("%v", v))
		}
	}

	// Fallback: first value (key unknown — caller treats as plain text)
	for _, v := range args {
		return "", truncate(fmt.Sprintf("%v", v))
	}

	return "", ""
}

// languageByArgKey maps a tool-args key to a chroma lexer name. When a tool's
// args contain a code-bearing field (sql, python source, etc.), the renderer
// looks the key up here to decide which lexer to use for syntax highlighting.
//
// To add a new language: register the lexer name (must match
// chroma's lexer registry — see github.com/alecthomas/chroma/v2/lexers/) for
// each arg key the tool exposes. The token→style mapping in
// highlightCode is language-agnostic, so no other code changes are needed.
var languageByArgKey = map[string]string{
	"sql":   "sql",
	"query": "sql",
}

// languageForArgKey returns the chroma lexer name for the given arg key, or
// "" if the key carries plain text and should not be syntax-highlighted.
func languageForArgKey(key string) string {
	return languageByArgKey[key]
}

// lexerCache memoises chroma lexers by language name so we don't re-resolve
// (and re-coalesce) on every render. Populated lazily by highlightCode.
// Single-goroutine TUI render path — no synchronisation needed.
var lexerCache = map[string]chroma.Lexer{}

// lexerFor returns a coalesced chroma lexer for lang, or nil if chroma has
// no matching lexer. nil leads to a graceful fall-through to plain dim text
// in highlightCode.
func lexerFor(lang string) chroma.Lexer {
	if l, ok := lexerCache[lang]; ok {
		return l
	}
	l := lexers.Get(lang)
	if l != nil {
		l = chroma.Coalesce(l)
	}
	lexerCache[lang] = l // cache nil too — no point in retrying a missing lexer
	return l
}

// highlightCode emits ANSI-styled source using our active palette. Keywords
// take the accent color, literals take the regular text color, everything
// else stays in dim text — the goal is to lift code structure without making
// the tool header visually dominant. Language-agnostic: works for any chroma
// lexer (sql, python, yaml, ...). The caller is responsible for truncation;
// chroma tokenises whatever it gets and never panics on partial input.
func highlightCode(src, lang string) string {
	lexer := lexerFor(lang)
	if lexer == nil || src == "" {
		return StatusDimStyle.Render(src)
	}
	iter, err := lexer.Tokenise(nil, src)
	if err != nil {
		return StatusDimStyle.Render(src)
	}
	keywordStyle := lipgloss.NewStyle().Foreground(accentColor)
	literalStyle := lipgloss.NewStyle().Foreground(textColor)
	dim := lipgloss.NewStyle().Foreground(dimTextColor)
	var sb strings.Builder
	for _, tok := range iter.Tokens() {
		switch {
		case tok.Type.InCategory(chroma.Keyword):
			sb.WriteString(keywordStyle.Render(tok.Value))
		case tok.Type.InCategory(chroma.LiteralString),
			tok.Type.InCategory(chroma.LiteralNumber):
			sb.WriteString(literalStyle.Render(tok.Value))
		default:
			sb.WriteString(dim.Render(tok.Value))
		}
	}
	return sb.String()
}

// isDiff checks if content looks like a unified diff.
func isDiff(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "---") || strings.HasPrefix(trimmed, "@@")
}

// renderDiff parses unified diff content and applies diff coloring. The first
// line carries the branch glyph; subsequent lines align under it.
func renderDiff(diff string) string {
	var sb strings.Builder
	lines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	for i, line := range lines {
		sb.WriteString(bodyPrefix(i == 0))
		switch {
		case strings.HasPrefix(line, "@@"):
			sb.WriteString(DiffHunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			sb.WriteString(DiffAddStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			sb.WriteString(DiffRemoveStyle.Render(line))
		default:
			sb.WriteString(StatusDimStyle.Render(line))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderMarkdown renders markdown content for completed messages. Pipe
// tables are extracted and rendered through our shrink-wrapping lipgloss
// table renderer; everything else is sent through Glamour as before. See
// markdown_tables.go for why we don't just let Glamour do tables.
func renderMarkdown(content string, width int) string {
	// Fast path: no pipe character anywhere → no table possible, single
	// Glamour pass. Avoids paying segmentation cost on the common case.
	if !strings.Contains(content, "|") {
		return renderProseMarkdown(content, width)
	}

	segs := splitMarkdownTables(content)
	if len(segs) == 0 {
		return renderProseMarkdown(content, width)
	}

	// Single prose segment (no tables detected after scanning) — fast path.
	if len(segs) == 1 && !segs[0].isTable {
		return renderProseMarkdown(content, width)
	}

	var sb strings.Builder
	for i, seg := range segs {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		if seg.isTable {
			sb.WriteString(renderMarkdownTable(seg.text))
			continue
		}
		text := strings.TrimSpace(seg.text)
		if text == "" {
			continue
		}
		sb.WriteString(renderProseMarkdown(text, width))
	}
	return strings.TrimSpace(sb.String())
}

// renderProseMarkdown runs Glamour on a chunk of markdown that contains no
// table — used both for content with no tables at all and for prose
// segments between extracted tables.
func renderProseMarkdown(content string, width int) string {
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
