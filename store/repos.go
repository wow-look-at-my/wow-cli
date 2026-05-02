package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Repo is a configured encrypted-manifest endpoint. URL points to the
// age-encrypted manifest; Identity is the X25519 private key used to decrypt
// it (e.g. "AGE-SECRET-KEY-...").
type Repo struct {
	URL      string    `json:"url"`
	Identity string    `json:"identity"`
	AddedAt  time.Time `json:"added_at"`
}

// RepoList is the on-disk registry of configured repos.
type RepoList struct {
	Repos []*Repo `json:"repos"`
	path  string
}

func repoListPath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repos.json"), nil
}

// LoadRepoList reads the repo list from disk, returning an empty list if
// none exists yet.
func LoadRepoList() (*RepoList, error) {
	path, err := repoListPath()
	if err != nil {
		return nil, err
	}
	s := &RepoList{Repos: []*Repo{}, path: path}
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
	if s.Repos == nil {
		s.Repos = []*Repo{}
	}
	s.path = path
	return s, nil
}

// Save writes the repo list to disk. The file is written 0600 because it
// holds private decryption keys.
func (s *RepoList) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// Add appends a repo. If a repo with the same URL already exists, its
// identity is replaced (no duplicates).
func (s *RepoList) Add(r *Repo) {
	for i, existing := range s.Repos {
		if existing.URL == r.URL {
			s.Repos[i] = r
			return
		}
	}
	s.Repos = append(s.Repos, r)
}

// Remove deletes the repo with the given URL. Returns the removed entry, or
// nil if not found.
func (s *RepoList) Remove(url string) *Repo {
	for i, r := range s.Repos {
		if r.URL == url {
			s.Repos = append(s.Repos[:i], s.Repos[i+1:]...)
			return r
		}
	}
	return nil
}

// Find returns the repo matching url, or nil.
func (s *RepoList) Find(url string) *Repo {
	for _, r := range s.Repos {
		if r.URL == url {
			return r
		}
	}
	return nil
}

// All returns all configured repos in stored order.
func (s *RepoList) All() []*Repo {
	return s.Repos
}

// String renders a repo for display, truncating the identity to avoid
// printing it in full.
func (r *Repo) String() string {
	return fmt.Sprintf("%s (key: %s)", r.URL, truncateKey(r.Identity))
}

func truncateKey(key string) string {
	const showHead = 16
	const showTail = 6
	if len(key) <= showHead+showTail+1 {
		return key
	}
	return key[:showHead] + "..." + key[len(key)-showTail:]
}
