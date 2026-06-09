package dirhistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecord_NewEntry(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "dirs.json")}
	s.Record("/home/user/project", "claude")

	if len(s.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(s.entries))
	}
	e := s.entries[0]
	if e.Path != "/home/user/project" {
		t.Errorf("got path %q", e.Path)
	}
	if e.Count != 1 {
		t.Errorf("got count %d, want 1", e.Count)
	}
	if e.Profiles["claude"] != 1 {
		t.Errorf("got profiles[claude]=%d, want 1", e.Profiles["claude"])
	}
}

func TestRecord_UpdateExisting(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "dirs.json")}
	s.Record("/home/user/project", "claude")
	s.Record("/home/user/project", "codex")
	s.Record("/home/user/project", "claude")

	if len(s.entries) != 1 {
		t.Fatalf("expected 1 entry (deduped), got %d", len(s.entries))
	}
	e := s.entries[0]
	if e.Count != 3 {
		t.Errorf("got count %d, want 3", e.Count)
	}
	if e.Profiles["claude"] != 2 {
		t.Errorf("got profiles[claude]=%d, want 2", e.Profiles["claude"])
	}
	if e.Profiles["codex"] != 1 {
		t.Errorf("got profiles[codex]=%d, want 1", e.Profiles["codex"])
	}
}

func TestRecord_MultipleEntries(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "dirs.json")}
	s.Record("/a", "claude")
	s.Record("/b", "codex")

	if len(s.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s.entries))
	}
}

func TestCandidates_FiltersMissing(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "exists")
	if err := os.MkdirAll(existing, 0755); err != nil {
		t.Fatal(err)
	}

	s := &Store{
		path: filepath.Join(dir, "dirs.json"),
		entries: []Entry{
			{Path: existing, LastUsed: time.Now(), Count: 1},
			{Path: filepath.Join(dir, "gone"), LastUsed: time.Now(), Count: 1},
		},
	}

	candidates := s.Candidates()
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate (missing filtered), got %d", len(candidates))
	}
	if candidates[0].Path != existing {
		t.Errorf("got path %q, want %q", candidates[0].Path, existing)
	}
}

func TestCandidates_SortByRecent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	os.MkdirAll(a, 0755)
	os.MkdirAll(b, 0755)

	old := time.Now().Add(-1 * time.Hour)
	recent := time.Now()

	s := &Store{
		path: filepath.Join(dir, "dirs.json"),
		entries: []Entry{
			{Path: a, LastUsed: old, Count: 10},
			{Path: b, LastUsed: recent, Count: 1},
		},
	}

	candidates := s.Candidates()
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Path != b {
		t.Errorf("expected most recent (b) first, got %q", candidates[0].Path)
	}
}

func TestSaveAndOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "dirs.json")

	s := &Store{path: path}
	s.Record("/home/user/project", "claude")
	if err := s.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var fd fileData
	if err := json.Unmarshal(data, &fd); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if fd.Version != 1 {
		t.Errorf("got version %d, want 1", fd.Version)
	}
	if len(fd.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(fd.Entries))
	}
}

func TestOpen_NonExistent(t *testing.T) {
	orig := os.Getenv("XDG_STATE_HOME")
	defer os.Setenv("XDG_STATE_HOME", orig)
	os.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "nonexistent"))

	s, err := Open()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.Entries()) != 0 {
		t.Errorf("expected 0 entries, got %d", len(s.Entries()))
	}
}

func TestCompact_Deduplicates(t *testing.T) {
	s := &Store{
		path: filepath.Join(t.TempDir(), "dirs.json"),
		entries: []Entry{
			{Path: "/a", LastUsed: time.Now().Add(-1 * time.Hour), Count: 5},
			{Path: "/a", LastUsed: time.Now(), Count: 3},
		},
	}

	s.compact()

	if len(s.entries) != 1 {
		t.Fatalf("expected 1 entry after compact, got %d", len(s.entries))
	}
	if s.entries[0].Count != 8 {
		t.Errorf("got count %d, want 8", s.entries[0].Count)
	}
}

func TestRecord_EmptyProfile(t *testing.T) {
	s := &Store{path: filepath.Join(t.TempDir(), "dirs.json")}
	s.Record("/home/user/project", "")

	if len(s.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(s.entries))
	}
	if s.entries[0].Profiles != nil {
		t.Error("expected nil profiles map for empty profile name")
	}
}
