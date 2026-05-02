package duckdb

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Session owns the per-Cascade-invocation DuckDB database file plus the
// in-process locking that keeps parallel agent tool calls from clobbering
// each other.
//
// One DB per `cascade` invocation is the v1 default. The file lives at
// ~/.cascade/duckdb/{session-id}.db and is removed on Close() unless
// Keep is true. DuckDB itself takes an OS-level file lock on the .db,
// so a second cascade process targeting the same file (or a standalone
// `duckdb` CLI) is rejected by the kernel — we surface that as a clear
// error rather than silently failing.
type Session struct {
	// Path is the absolute filesystem path of the session DB.
	Path string
	// ID is the random session identifier (hex). Cascade picks this, not
	// the agent.
	ID string
	// Keep, when true, retains the DB file on Close (overrides delete).
	Keep bool

	// mu serializes writes within this Cascade process. Reads (Query
	// against a local DB) take RLock; writes (Exec, COPY, CREATE) take
	// the write lock. Cross-process safety is delegated to DuckDB's
	// own file lock — see package doc.
	mu sync.RWMutex
}

// NewSession creates a fresh session DB under ~/.cascade/duckdb/. The DB
// file itself is not opened or pre-created by Cascade — DuckDB creates
// it lazily on first connect. We just allocate a path.
func NewSession(keep bool) (*Session, error) {
	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	dir, err := sessionDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("duckdb session dir: %w", err)
	}
	return &Session{
		Path: filepath.Join(dir, id+".db"),
		ID:   id,
		Keep: keep,
	}, nil
}

// Close removes the session DB unless Keep is set. Best-effort — errors
// are returned but already-deleted files are not treated as failures.
// DuckDB writes both `.db` and `.db.wal`; both are cleaned up.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	if s.Keep {
		return nil
	}
	var firstErr error
	for _, suffix := range []string{"", ".wal"} {
		path := s.Path + suffix
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// RLock acquires a read lock for in-process query parallelism.
func (s *Session) RLock()   { s.mu.RLock() }
func (s *Session) RUnlock() { s.mu.RUnlock() }

// Lock acquires a write lock. Use for Exec, COPY, CREATE-style work.
func (s *Session) Lock()   { s.mu.Lock() }
func (s *Session) Unlock() { s.mu.Unlock() }

func newSessionID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("session id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func sessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, ".cascade", "duckdb"), nil
}
