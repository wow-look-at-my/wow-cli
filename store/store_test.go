package store

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestStateDir_WOW_STATE_DIR(t *testing.T) {
	t.Setenv("WOW_STATE_DIR", "/tmp/wow-test-state")
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/tmp/wow-test-state" {
		t.Errorf("got %q, want /tmp/wow-test-state", dir)
	}
}

func TestStateDir_XDG(t *testing.T) {
	t.Setenv("WOW_STATE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	dir, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join("/tmp/xdg", "wow") {
		t.Errorf("got %q", dir)
	}
}

func TestStateDir_Default(t *testing.T) {
	t.Setenv("WOW_STATE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Error("StateDir returned empty string")
	}
}

func TestDefaultBinDir(t *testing.T) {
	dir, err := DefaultBinDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Error("DefaultBinDir returned empty string")
	}
}

func withTempState(t *testing.T) {
	t.Helper()
	t.Setenv("WOW_STATE_DIR", t.TempDir())
}

func TestLoad_Empty(t *testing.T) {
	withTempState(t)
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Packages) != 0 {
		t.Errorf("expected empty packages, got %d", len(s.Packages))
	}
}

func TestSaveLoad_Roundtrip(t *testing.T) {
	withTempState(t)
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	s.Add(&Package{
		Slug:    "owner/repo",
		Name:    "repo",
		Path:    "/usr/local/bin/repo",
		Version: "v1.2.3",
	})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	s2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	pkg := s2.Find("owner/repo")
	if pkg == nil {
		t.Fatal("package not found after reload")
	}
	if pkg.Name != "repo" || pkg.Version != "v1.2.3" {
		t.Errorf("got %+v", pkg)
	}
}

func TestAdd_Upserts(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v1"})
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v2"})
	if len(s.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(s.Packages))
	}
	if s.Packages["a/b"].Version != "v2" {
		t.Errorf("expected v2, got %s", s.Packages["a/b"].Version)
	}
}

func TestRemove_BySlug(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v1"})
	removed := s.Remove("a/b")
	if removed == nil {
		t.Fatal("Remove returned nil")
	}
	if removed.Slug != "a/b" {
		t.Errorf("got slug %q", removed.Slug)
	}
	if len(s.Packages) != 0 {
		t.Error("package should be gone")
	}
}

func TestRemove_ByName(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "mybin", Path: "/bin/mybin", Version: "v1"})
	removed := s.Remove("mybin")
	if removed == nil {
		t.Fatal("Remove returned nil")
	}
	if len(s.Packages) != 0 {
		t.Error("package should be gone")
	}
}

func TestRemove_NotFound(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	removed := s.Remove("nobody/here")
	if removed != nil {
		t.Error("expected nil for missing package")
	}
}

func TestFind_BySlug(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "x/y", Name: "y", Path: "/bin/y", Version: "v0"})
	pkg := s.Find("x/y")
	if pkg == nil || pkg.Slug != "x/y" {
		t.Error("Find by slug failed")
	}
}

func TestFind_ByName(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "x/y", Name: "myname", Path: "/bin/myname", Version: "v0"})
	pkg := s.Find("myname")
	if pkg == nil || pkg.Name != "myname" {
		t.Error("Find by name failed")
	}
}

func TestFind_NotFound(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	if s.Find("ghost") != nil {
		t.Error("expected nil for missing package")
	}
}

func TestAll(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/a", Name: "a", Path: "/bin/a", Version: "v1"})
	s.Add(&Package{Slug: "b/b", Name: "b", Path: "/bin/b", Version: "v2"})
	all := s.All()
	if len(all) != 2 {
		t.Errorf("expected 2 packages, got %d", len(all))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Slug < all[j].Slug })
	if all[0].Slug != "a/a" || all[1].Slug != "b/b" {
		t.Errorf("unexpected order: %v", all)
	}
}

func TestAll_Empty(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	if len(s.All()) != 0 {
		t.Error("expected empty All()")
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "nested", "dir")
	t.Setenv("WOW_STATE_DIR", nested)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v1"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(nested, "packages.json")); err != nil {
		t.Error("packages.json not created")
	}
}
