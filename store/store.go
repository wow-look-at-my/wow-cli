package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Package is one installed program tracked by wow.
type Package struct {
	Slug    string `json:"slug"`
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
}

// Store holds the full registry of installed packages.
type Store struct {
	Packages map[string]*Package `json:"packages"`
	path     string
}

// StateDir returns the directory where wow keeps its state.
// WOW_STATE_DIR overrides the default (useful for tests).
func StateDir() (string, error) {
	if d := os.Getenv("WOW_STATE_DIR"); d != "" {
		return d, nil
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "wow"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "wow"), nil
}

// DefaultBinDir returns the default directory for installed binaries.
func DefaultBinDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func statePath() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "packages.json"), nil
}

// Load reads the store from disk, returning an empty store if none exists yet.
func Load() (*Store, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	s := &Store{Packages: make(map[string]*Package), path: path}
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
	s.path = path
	return s, nil
}

// Save writes the store to disk.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}

// Add inserts or replaces the package entry for pkg.Slug.
func (s *Store) Add(pkg *Package) {
	s.Packages[pkg.Slug] = pkg
}

// Remove deletes the entry matching either slug or binary name.
// Returns the removed package, or nil if not found.
func (s *Store) Remove(slugOrName string) *Package {
	pkg := s.Find(slugOrName)
	if pkg == nil {
		return nil
	}
	delete(s.Packages, pkg.Slug)
	return pkg
}

// Find returns the package matching slug or binary name, or nil.
func (s *Store) Find(slugOrName string) *Package {
	if pkg, ok := s.Packages[slugOrName]; ok {
		return pkg
	}
	for _, pkg := range s.Packages {
		if pkg.Name == slugOrName {
			return pkg
		}
	}
	return nil
}

// All returns all packages in insertion-stable order (sorted by slug).
func (s *Store) All() []*Package {
	pkgs := make([]*Package, 0, len(s.Packages))
	for _, pkg := range s.Packages {
		pkgs = append(pkgs, pkg)
	}
	return pkgs
}
