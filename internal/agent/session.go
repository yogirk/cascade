package agent

import "github.com/slokam-ai/cascade/pkg/types"

// Session tracks conversation history for multi-turn context.
type Session struct {
	messages     []types.Message
	systemPrompt string
	onSave       func([]types.Message) // optional persistence callback
}

// NewSession creates a new session. If systemPrompt is non-empty,
// a system message is prepended.
func NewSession(systemPrompt string) *Session {
	s := &Session{systemPrompt: systemPrompt}
	if systemPrompt != "" {
		s.messages = append(s.messages, types.SystemMessage(systemPrompt))
	}
	return s
}

// Append adds a message to the session history.
func (s *Session) Append(msg types.Message) { s.messages = append(s.messages, msg) }

// AppendSystem adds a system message to the session history.
func (s *Session) AppendSystem(content string) {
	s.messages = append(s.messages, types.SystemMessage(content))
}

// Messages returns the full conversation history.
func (s *Session) Messages() []types.Message { return s.messages }

// Len returns the number of messages in the session.
func (s *Session) Len() int { return len(s.messages) }

// Replace replaces the session messages with a new set (used after compaction).
// The system prompt is preserved as the first message.
func (s *Session) Replace(messages []types.Message) {
	s.messages = make([]types.Message, 0, len(messages)+1)
	if s.systemPrompt != "" {
		s.messages = append(s.messages, types.SystemMessage(s.systemPrompt))
	}
	s.messages = append(s.messages, messages...)
}

// SystemPrompt returns the session's system prompt.
func (s *Session) SystemPrompt() string { return s.systemPrompt }

// SetOnSave registers a callback invoked when NotifySave is called.
func (s *Session) SetOnSave(fn func([]types.Message)) { s.onSave = fn }

// NotifySave triggers the persistence callback with the current messages.
func (s *Session) NotifySave() {
	if s.onSave != nil {
		s.onSave(s.messages)
	}
}

// SetSystemPrompt updates the system prompt (used for context injection).
func (s *Session) SetSystemPrompt(prompt string) {
	s.systemPrompt = prompt
	// Update the first message if it's a system message
	if len(s.messages) > 0 && s.messages[0].Role == types.RoleSystem {
		s.messages[0].Content = prompt
	}
}
