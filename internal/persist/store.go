// Package persist provides session persistence for Cascade conversations.
package persist

import (
	"time"

	"github.com/slokam-ai/cascade/pkg/types"
)

// SessionMeta holds session metadata without message content.
type SessionMeta struct {
	ID        string
	Model     string
	Project   string
	Summary   string // first user message or compacted summary
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Store defines the session persistence interface.
type Store interface {
	// SaveSession upserts session metadata and replaces all messages.
	SaveSession(meta SessionMeta, messages []types.Message) error

	// LoadSession loads a session by ID.
	LoadSession(id string) (*SessionMeta, []types.Message, error)

	// ListSessions returns all sessions ordered by updated_at desc.
	ListSessions() ([]SessionMeta, error)

	// DeleteSession removes a session and its messages.
	DeleteSession(id string) error

	// LatestSessionID returns the most recently updated session ID, or "" if none.
	LatestSessionID() (string, error)

	// Close releases resources.
	Close() error
}
