package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
)

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string // "user", "assistant", "tool", "error"
	Content string
}

// ChatModel manages the scrollable chat message history.
type ChatModel struct {
	viewport viewport.Model
	messages []ChatMessage
	width    int
	height   int
}

// NewChatModel creates a new chat model with the given dimensions.
func NewChatModel(width, height int) ChatModel {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	return ChatModel{
		viewport: vp,
		messages: make([]ChatMessage, 0),
		width:    width,
		height:   height,
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

// AddMessage appends a message to the conversation history and re-renders.
func (c *ChatModel) AddMessage(msg ChatMessage) {
	c.messages = append(c.messages, msg)
	c.renderMessages()
}

// SetStreamingContent updates the viewport to show conversation history plus
// in-progress streaming content at the bottom.
func (c *ChatModel) SetStreamingContent(content string) {
	var sb strings.Builder
	for _, msg := range c.messages {
		sb.WriteString(renderMessage(msg))
		sb.WriteString("\n\n")
	}
	if content != "" {
		sb.WriteString(AssistantStyle.Render("assistant") + " ")
		sb.WriteString(content)
	}
	c.viewport.SetContent(sb.String())
	c.viewport.GotoBottom()
}

// SetSize updates the chat viewport dimensions.
func (c *ChatModel) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.SetWidth(width)
	c.viewport.SetHeight(height)
	c.renderMessages()
}

// renderMessages re-renders all messages into the viewport.
func (c *ChatModel) renderMessages() {
	var sb strings.Builder
	for _, msg := range c.messages {
		sb.WriteString(renderMessage(msg))
		sb.WriteString("\n\n")
	}
	c.viewport.SetContent(sb.String())
	c.viewport.GotoBottom()
}

// renderMessage formats a single chat message with role prefix and styling.
func renderMessage(msg ChatMessage) string {
	switch msg.Role {
	case "user":
		return UserStyle.Render("you") + " " + msg.Content
	case "assistant":
		rendered := renderMarkdown(msg.Content)
		return AssistantStyle.Render("assistant") + " " + rendered
	case "tool":
		return ToolStyle.Render("tool") + " " + msg.Content
	case "error":
		return ErrorStyle.Render("error") + " " + msg.Content
	default:
		return msg.Content
	}
}

// renderMarkdown renders markdown content using Glamour for completed messages.
func renderMarkdown(content string) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(80),
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

// MessageCount returns the number of messages in the chat.
func (c *ChatModel) MessageCount() int {
	return len(c.messages)
}

// FormatToolResult formats a tool result for display in the chat.
func FormatToolResult(toolName, content string, isError bool) string {
	if isError {
		return fmt.Sprintf("%s failed: %s", toolName, content)
	}
	// Truncate long tool output for display
	lines := strings.Split(content, "\n")
	if len(lines) > 50 {
		content = strings.Join(lines[:50], "\n") + fmt.Sprintf("\n... (%d more lines)", len(lines)-50)
	}
	return fmt.Sprintf("%s:\n%s", toolName, content)
}
