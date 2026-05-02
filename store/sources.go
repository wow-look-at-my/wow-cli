package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Source is a configured encrypted-manifest endpoint. URL points to the
// age-encrypted manifest; Identity is the X25519 private key used to decrypt
// it (e.g. "AGE-SECRET-KEY-...").
type Source struct {
	URL      string    `json:"url"`
	Identity string    `json:"identity"`
	AddedAt  time.Time `json:"added_at"`
}

// SourceStore is the on-disk registry of configured sources.
type SourceStore struct {
	Sources []*Source `json:"sources"`
	path    string
}

func sourcesPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sources.json"), nil
}

// LoadSources reads the sources file from disk, returning an empty store if
// none exists yet.
func LoadSources() (*SourceStore, error) {
	path, err := sourcesPath()
	if err != nil {
		return nil, err
	}
	s := &SourceStore{Sources: []*Source{}, path: path}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Sources == nil {
		s.Sources = []*Source{}
	}
	s.path = path
	return s, nil
}

// Save writes the sources store to disk. The file is written 0600 because it
// holds private decryption keys.
func (s *SourceStore) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// Add appends a source. If a source with the same URL already exists, its
// identity is replaced (no duplicates).
func (s *SourceStore) Add(src *Source) {
	for i, existing := range s.Sources {
		if existing.URL == src.URL {
			s.Sources[i] = src
			return
		}
	}
	s.Sources = append(s.Sources, src)
}

// Remove deletes the source with the given URL. Returns the removed entry, or
// nil if not found.
func (s *SourceStore) Remove(url string) *Source {
	for i, src := range s.Sources {
		if src.URL == url {
			s.Sources = append(s.Sources[:i], s.Sources[i+1:]...)
			return src
		}
	}
	return nil
}

// Find returns the source matching url, or nil.
func (s *SourceStore) Find(url string) *Source {
	for _, src := range s.Sources {
		if src.URL == url {
			return src
		}
	}
	return nil
}

// All returns all configured sources in stored order.
func (s *SourceStore) All() []*Source {
	return s.Sources
}

// String renders a source for display, truncating the identity to avoid
// printing it in full.
func (src *Source) String() string {
	return fmt.Sprintf("%s (key: %s)", src.URL, truncateKey(src.Identity))
}

func truncateKey(key string) string {
	const showHead = 16
	const showTail = 6
	if len(key) <= showHead+showTail+1 {
		return key
	}
	return key[:showHead] + "..." + key[len(key)-showTail:]
}
