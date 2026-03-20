package tui

import (
	"strings"
	"sync"
)

// StreamRenderer accumulates streaming LLM tokens in a thread-safe, lossless
// buffer. Tokens are pushed by the agent goroutine and drained in batch by
// the TUI's 30fps tick. Unlike the previous channel-based approach, this
// implementation never drops tokens.
type StreamRenderer struct {
	mu      sync.Mutex
	pending []string        // Tokens waiting to be drained
	content strings.Builder // Accumulated raw content
	dirty   bool
}

// NewStreamRenderer creates a new StreamRenderer.
func NewStreamRenderer() *StreamRenderer {
	return &StreamRenderer{}
}

// Push appends a token to the pending buffer. Thread-safe, never blocks,
// never drops.
func (r *StreamRenderer) Push(token string) {
	r.mu.Lock()
	r.pending = append(r.pending, token)
	r.mu.Unlock()
}

// DrainAll moves all pending tokens into the accumulated content. Returns the
// number of tokens drained. Called from the TUI goroutine on each tick.
func (r *StreamRenderer) DrainAll() int {
	r.mu.Lock()
	tokens := r.pending
	r.pending = nil
	r.mu.Unlock()

	if len(tokens) == 0 {
		return 0
	}

	for _, t := range tokens {
		r.content.WriteString(t)
	}
	r.dirty = true
	return len(tokens)
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

// Reset clears all accumulated content and pending tokens for the next message.
func (r *StreamRenderer) Reset() {
	r.mu.Lock()
	r.pending = nil
	r.mu.Unlock()
	r.content.Reset()
	r.dirty = false
}
