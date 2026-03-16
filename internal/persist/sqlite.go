package persist

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yogirk/cascade/pkg/types"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using a SQLite database.
type SQLiteStore struct {
	mu sync.RWMutex
	db *sql.DB
}

// OpenSQLite opens or creates sessions.db in the given directory.
func OpenSQLite(dir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	dbPath := filepath.Join(dir, "sessions.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	model TEXT NOT NULL DEFAULT '',
	project TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL,
	updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
	session_id TEXT NOT NULL,
	seq INTEGER NOT NULL,
	role TEXT NOT NULL,
	content TEXT NOT NULL DEFAULT '',
	tool_calls_json TEXT,
	tool_result_json TEXT,
	created_at INTEGER NOT NULL,
	PRIMARY KEY (session_id, seq),
	FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
`
	_, err := db.Exec(ddl)
	return err
}

// GenerateSessionID returns an ID like "20260328-143022-a7f2".
func GenerateSessionID() string {
	ts := time.Now().Format("20060102-150405")
	b := make([]byte, 2)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", ts, hex.EncodeToString(b))
}

func (s *SQLiteStore) SaveSession(meta SessionMeta, messages []types.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().Unix()
	createdAt := meta.CreatedAt.Unix()
	if createdAt <= 0 {
		createdAt = now
	}

	_, err = tx.Exec(`
		INSERT INTO sessions (id, model, project, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			model=excluded.model, project=excluded.project,
			summary=excluded.summary, updated_at=excluded.updated_at
	`, meta.ID, meta.Model, meta.Project, meta.Summary, createdAt, now)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}

	// Replace all messages
	if _, err := tx.Exec("DELETE FROM messages WHERE session_id=?", meta.ID); err != nil {
		return fmt.Errorf("delete old messages: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO messages (session_id, seq, role, content, tool_calls_json, tool_result_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for i, msg := range messages {
		var toolCallsJSON, toolResultJSON *string

		if len(msg.ToolCalls) > 0 {
			b, err := json.Marshal(msg.ToolCalls)
			if err != nil {
				return fmt.Errorf("marshal tool calls [%d]: %w", i, err)
			}
			s := string(b)
			toolCallsJSON = &s
		}

		if msg.ToolResult != nil {
			b, err := json.Marshal(msg.ToolResult)
			if err != nil {
				return fmt.Errorf("marshal tool result [%d]: %w", i, err)
			}
			s := string(b)
			toolResultJSON = &s
		}

		if _, err := stmt.Exec(meta.ID, i, string(msg.Role), msg.Content, toolCallsJSON, toolResultJSON, now); err != nil {
			return fmt.Errorf("insert message [%d]: %w", i, err)
		}
	}

	return tx.Commit()
}

func (s *SQLiteStore) LoadSession(id string) (*SessionMeta, []types.Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var meta SessionMeta
	var createdAt, updatedAt int64
	err := s.db.QueryRow(`SELECT id, model, project, summary, created_at, updated_at FROM sessions WHERE id=?`, id).
		Scan(&meta.ID, &meta.Model, &meta.Project, &meta.Summary, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil, fmt.Errorf("session %q not found", id)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query session: %w", err)
	}
	meta.CreatedAt = time.Unix(createdAt, 0)
	meta.UpdatedAt = time.Unix(updatedAt, 0)

	rows, err := s.db.Query(`SELECT role, content, tool_calls_json, tool_result_json FROM messages WHERE session_id=? ORDER BY seq`, id)
	if err != nil {
		return nil, nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var messages []types.Message
	for rows.Next() {
		var msg types.Message
		var role string
		var toolCallsJSON, toolResultJSON sql.NullString

		if err := rows.Scan(&role, &msg.Content, &toolCallsJSON, &toolResultJSON); err != nil {
			return nil, nil, fmt.Errorf("scan message: %w", err)
		}
		msg.Role = types.Role(role)

		if toolCallsJSON.Valid {
			if err := json.Unmarshal([]byte(toolCallsJSON.String), &msg.ToolCalls); err != nil {
				return nil, nil, fmt.Errorf("unmarshal tool calls: %w", err)
			}
		}
		if toolResultJSON.Valid {
			var tr types.ToolResult
			if err := json.Unmarshal([]byte(toolResultJSON.String), &tr); err != nil {
				return nil, nil, fmt.Errorf("unmarshal tool result: %w", err)
			}
			msg.ToolResult = &tr
		}

		messages = append(messages, msg)
	}

	return &meta, messages, rows.Err()
}

func (s *SQLiteStore) ListSessions() ([]SessionMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, model, project, summary, created_at, updated_at FROM sessions ORDER BY updated_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionMeta
	for rows.Next() {
		var m SessionMeta
		var createdAt, updatedAt int64
		if err := rows.Scan(&m.ID, &m.Model, &m.Project, &m.Summary, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		m.CreatedAt = time.Unix(createdAt, 0)
		m.UpdatedAt = time.Unix(updatedAt, 0)
		sessions = append(sessions, m)
	}
	return sessions, rows.Err()
}

func (s *SQLiteStore) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM sessions WHERE id=?", id)
	return err
}

func (s *SQLiteStore) LatestSessionID() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var id string
	err := s.db.QueryRow("SELECT id FROM sessions ORDER BY updated_at DESC LIMIT 1").Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
