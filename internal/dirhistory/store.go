package dirhistory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/konono/aw/v4/internal/gitroot"
	"github.com/konono/aw/v4/internal/platform"
)

const (
	maxEntries = 5000
	fileVersion = 1
)

type Entry struct {
	Path     string            `json:"path"`
	RepoRoot string            `json:"repo_root,omitempty"`
	LastUsed time.Time         `json:"last_used"`
	Count    int               `json:"count"`
	Profiles map[string]int    `json:"profiles,omitempty"`
}

type fileData struct {
	Version int     `json:"version"`
	Entries []Entry `json:"entries"`
}

type Store struct {
	path    string
	entries []Entry
}

func Open() (*Store, error) {
	p := filePath()
	s := &Store{path: p}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	var fd fileData
	if err := json.Unmarshal(data, &fd); err != nil {
		return s, nil
	}

	s.entries = fd.Entries
	return s, nil
}

func (s *Store) Record(dir, profileName string) {
	dir = cleanPath(dir)

	for i := range s.entries {
		if s.entries[i].Path == dir {
			s.entries[i].LastUsed = time.Now().UTC()
			s.entries[i].Count++
			if profileName != "" {
				if s.entries[i].Profiles == nil {
					s.entries[i].Profiles = make(map[string]int)
				}
				s.entries[i].Profiles[profileName]++
			}
			return
		}
	}

	e := Entry{
		Path:     dir,
		RepoRoot: detectRepoRoot(dir),
		LastUsed: time.Now().UTC(),
		Count:    1,
	}
	if profileName != "" {
		e.Profiles = map[string]int{profileName: 1}
	}
	s.entries = append(s.entries, e)
}

func (s *Store) Candidates() []Entry {
	var valid []Entry
	for _, e := range s.entries {
		if _, err := os.Stat(e.Path); err == nil {
			valid = append(valid, e)
		}
	}

	sort.Slice(valid, func(i, j int) bool {
		ti := valid[i].LastUsed
		tj := valid[j].LastUsed
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return valid[i].Count > valid[j].Count
	})

	if len(valid) > maxEntries {
		valid = valid[:maxEntries]
	}

	return valid
}

func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}

	s.compact()

	fd := fileData{
		Version: fileVersion,
		Entries: s.entries,
	}

	data, err := json.MarshalIndent(fd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0644)
}

func (s *Store) Entries() []Entry {
	return s.entries
}

func (s *Store) compact() {
	seen := make(map[string]int)
	var deduped []Entry
	for _, e := range s.entries {
		p := cleanPath(e.Path)
		if idx, ok := seen[p]; ok {
			deduped[idx].Count += e.Count
			if e.LastUsed.After(deduped[idx].LastUsed) {
				deduped[idx].LastUsed = e.LastUsed
			}
			for k, v := range e.Profiles {
				if deduped[idx].Profiles == nil {
					deduped[idx].Profiles = make(map[string]int)
				}
				deduped[idx].Profiles[k] += v
			}
		} else {
			e.Path = p
			seen[p] = len(deduped)
			deduped = append(deduped, e)
		}
	}

	sort.Slice(deduped, func(i, j int) bool {
		return deduped[i].LastUsed.After(deduped[j].LastUsed)
	})

	if len(deduped) > maxEntries {
		deduped = deduped[:maxEntries]
	}

	s.entries = deduped
}

func filePath() string {
	return filepath.Join(platform.StateDir(), "dirs.json")
}

func cleanPath(p string) string {
	p = filepath.Clean(p)
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return p
}

var detectRepoRoot = func(dir string) string {
	root, err := gitroot.FindRootFrom(dir)
	if err != nil {
		return ""
	}
	return root
}
