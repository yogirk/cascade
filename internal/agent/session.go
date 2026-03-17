package agent

import "github.com/cascade-cli/cascade/pkg/types"

// Session tracks conversation history for multi-turn context.
type Session struct {
	messages     []types.Message
	systemPrompt string
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
