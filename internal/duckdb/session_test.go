package duckdb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSession_AllocatesUniquePathInUserDir(t *testing.T) {
	a, err := NewSession(false)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	b, err := NewSession(false)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if a.ID == b.ID {
		t.Errorf("expected unique session ids, got %q twice", a.ID)
	}
	if !strings.Contains(a.Path, filepath.Join(".cascade", "duckdb")) {
		t.Errorf("session path %q not under ~/.cascade/duckdb/", a.Path)
	}
	if !strings.HasSuffix(a.Path, ".db") {
		t.Errorf("session path missing .db suffix: %q", a.Path)
	}
}

func TestSession_Close_RemovesFiles(t *testing.T) {
	dir := t.TempDir()
	s := &Session{
		Path: filepath.Join(dir, "abc.db"),
		ID:   "abc",
		Keep: false,
	}
	// Pretend duckdb wrote both files.
	for _, suf := range []string{"", ".wal"} {
		if err := os.WriteFile(s.Path+suf, []byte("fake"), 0o600); err != nil {
			t.Fatalf("write fake: %v", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	for _, suf := range []string{"", ".wal"} {
		if _, err := os.Stat(s.Path + suf); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, stat err = %v", s.Path+suf, err)
		}
	}
}

func TestSession_Close_KeepRetainsFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keep.db")
	if err := os.WriteFile(path, []byte("fake"), 0o600); err != nil {
		t.Fatalf("write fake: %v", err)
	}
	s := &Session{Path: path, ID: "keep", Keep: true}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to be retained when Keep=true, stat err = %v", err)
	}
}

func TestSession_Close_NilSafe(t *testing.T) {
	var s *Session
	if err := s.Close(); err != nil {
		t.Errorf("nil session Close returned %v, want nil", err)
	}
}
