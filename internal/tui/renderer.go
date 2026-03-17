package tui

import "strings"

// StreamRenderer implements a ring buffer + render tick pattern for streaming
// LLM tokens to the TUI without deadlocking the Bubble Tea event loop.
// Tokens are pushed to a buffered channel by the agent goroutine and drained
// in batch by the TUI's 30fps tick.
type StreamRenderer struct {
	buffer  chan string      // Buffered channel, capacity 256
	content strings.Builder  // Accumulated raw content
	dirty   bool             // Content changed since last drain
}

// NewStreamRenderer creates a new StreamRenderer with a 256-token buffer.
func NewStreamRenderer() *StreamRenderer {
	return &StreamRenderer{
		buffer: make(chan string, 256),
	}
}

// Push adds a token to the buffer. Non-blocking: drops token if buffer is full.
// This prevents the streaming goroutine from ever blocking on send, avoiding
// the Bubble Tea deadlock pitfall.
func (r *StreamRenderer) Push(token string) {
	select {
	case r.buffer <- token:
	default:
		// Buffer full -- drop token to prevent deadlock.
		// Extremely rare at cap 256 with 30fps drain.
	}
}

// DrainAll reads all available tokens from the buffer (non-blocking) and
// appends them to the accumulated content. Returns the number of tokens drained.
func (r *StreamRenderer) DrainAll() int {
	count := 0
	for {
		select {
		case token := <-r.buffer:
			r.content.WriteString(token)
			count++
			r.dirty = true
		default:
			return count
		}
	}
}

// Content returns the accumulated raw text from all drained tokens.
func (r *StreamRenderer) Content() string {
	return r.content.String()
}

// Dirty returns whether content has changed since the last ResetDirty call.
func (r *StreamRenderer) Dirty() bool {
	return r.dirty
}

// ResetDirty clears the dirty flag after a render cycle.
func (r *StreamRenderer) ResetDirty() {
	r.dirty = false
}

// Reset clears all accumulated content and the dirty flag for the next message.
func (r *StreamRenderer) Reset() {
	// Drain any remaining tokens
	for {
		select {
		case <-r.buffer:
		default:
			r.content.Reset()
			r.dirty = false
			return
		}
	}
}
