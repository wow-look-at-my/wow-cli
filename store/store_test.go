package store

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func TestStateDir_WOW_STATE_DIR(t *testing.T) {
	t.Setenv("WOW_STATE_DIR", "/tmp/wow-test-state")
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := StateDir()
	require.Nil(t, err)

	assert.Equal(t, "/tmp/wow-test-state", dir)

}

func TestStateDir_XDG(t *testing.T) {
	t.Setenv("WOW_STATE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	dir, err := StateDir()
	require.Nil(t, err)

	assert.Equal(t, filepath.Join("/tmp/xdg", "wow"), dir)

}

func TestStateDir_Default(t *testing.T) {
	t.Setenv("WOW_STATE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	dir, err := StateDir()
	require.Nil(t, err)

	assert.NotEqual(t, "", dir)

}

func TestDefaultBinDir(t *testing.T) {
	dir, err := DefaultBinDir()
	require.Nil(t, err)

	assert.NotEqual(t, "", dir)

}

func withTempState(t *testing.T) {
	t.Helper()
	t.Setenv("WOW_STATE_DIR", t.TempDir())
}

func TestLoad_Empty(t *testing.T) {
	withTempState(t)
	s, err := Load()
	require.Nil(t, err)

	assert.Equal(t, 0, len(s.Packages))

}

func TestSaveLoad_Roundtrip(t *testing.T) {
	withTempState(t)
	s, err := Load()
	require.Nil(t, err)

	s.Add(&Package{
		Slug:		"owner/repo",
		Name:		"repo",
		Path:		"/usr/local/bin/repo",
		Version:	"v1.2.3",
	})
	require.NoError(t, s.Save())

	s2, err := Load()
	require.Nil(t, err)

	pkg := s2.Find("owner/repo")
	require.NotNil(t, pkg)

	assert.False(t, pkg.Name != "repo" || pkg.Version != "v1.2.3")

}

func TestAdd_Upserts(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v1"})
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v2"})
	assert.Equal(t, 1, len(s.Packages))

	assert.Equal(t, "v2", s.Packages["a/b"].Version)

}

func TestRemove_BySlug(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v1"})
	removed := s.Remove("a/b")
	require.NotNil(t, removed)

	assert.Equal(t, "a/b", removed.Slug)

	assert.Equal(t, 0, len(s.Packages))

}

func TestRemove_ByName(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "mybin", Path: "/bin/mybin", Version: "v1"})
	removed := s.Remove("mybin")
	require.NotNil(t, removed)

	assert.Equal(t, 0, len(s.Packages))

}

func TestRemove_NotFound(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	removed := s.Remove("nobody/here")
	assert.Nil(t, removed)

}

func TestFind_BySlug(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "x/y", Name: "y", Path: "/bin/y", Version: "v0"})
	pkg := s.Find("x/y")
	assert.False(t, pkg == nil || pkg.Slug != "x/y")

}

func TestFind_ByName(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "x/y", Name: "myname", Path: "/bin/myname", Version: "v0"})
	pkg := s.Find("myname")
	assert.False(t, pkg == nil || pkg.Name != "myname")

}

func TestFind_NotFound(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	assert.Nil(t, s.Find("ghost"))

}

func TestAll(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	s.Add(&Package{Slug: "a/a", Name: "a", Path: "/bin/a", Version: "v1"})
	s.Add(&Package{Slug: "b/b", Name: "b", Path: "/bin/b", Version: "v2"})
	all := s.All()
	assert.Equal(t, 2, len(all))

	sort.Slice(all, func(i, j int) bool { return all[i].Slug < all[j].Slug })
	assert.False(t, all[0].Slug != "a/a" || all[1].Slug != "b/b")

}

func TestAll_Empty(t *testing.T) {
	withTempState(t)
	s, _ := Load()
	assert.Equal(t, 0, len(s.All()))

}

func TestSave_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	nested := filepath.Join(tmp, "nested", "dir")
	t.Setenv("WOW_STATE_DIR", nested)
	s, _ := Load()
	s.Add(&Package{Slug: "a/b", Name: "b", Path: "/bin/b", Version: "v1"})
	require.NoError(t, s.Save())

	_, err := os.Stat(filepath.Join(nested, "packages.json"))
	assert.Nil(t, err)

}
