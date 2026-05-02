package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func TestRepoList_LoadEmpty(t *testing.T) {
	withTempState(t)
	s, err := LoadRepoList()
	require.Nil(t, err)
	assert.Equal(t, 0, len(s.All()))
}

func TestRepoList_AddSaveLoad(t *testing.T) {
	withTempState(t)
	s, err := LoadRepoList()
	require.Nil(t, err)
	s.Add(&Repo{URL: "https://example/manifest.age", Identity: "AGE-SECRET-KEY-XYZ"})
	require.NoError(t, s.Save())

	s2, err := LoadRepoList()
	require.Nil(t, err)
	require.Equal(t, 1, len(s2.All()))

	got := s2.Find("https://example/manifest.age")
	require.NotNil(t, got)
	assert.Equal(t, "AGE-SECRET-KEY-XYZ", got.Identity)
}

func TestRepoList_AddReplacesByURL(t *testing.T) {
	withTempState(t)
	s, _ := LoadRepoList()
	s.Add(&Repo{URL: "https://a", Identity: "AGE-SECRET-KEY-1"})
	s.Add(&Repo{URL: "https://a", Identity: "AGE-SECRET-KEY-2"})

	assert.Equal(t, 1, len(s.All()))
	assert.Equal(t, "AGE-SECRET-KEY-2", s.All()[0].Identity)
}

func TestRepoList_Remove(t *testing.T) {
	withTempState(t)
	s, _ := LoadRepoList()
	s.Add(&Repo{URL: "https://a", Identity: "AGE-SECRET-KEY-1"})
	s.Add(&Repo{URL: "https://b", Identity: "AGE-SECRET-KEY-2"})

	removed := s.Remove("https://a")
	require.NotNil(t, removed)
	assert.Equal(t, "https://a", removed.URL)
	assert.Equal(t, 1, len(s.All()))

	assert.Nil(t, s.Remove("https://nope"))
}

func TestRepoList_FilePermissions(t *testing.T) {
	withTempState(t)
	s, _ := LoadRepoList()
	s.Add(&Repo{URL: "https://a", Identity: "AGE-SECRET-KEY-1"})
	require.NoError(t, s.Save())

	dir, _ := StateDir()
	info, err := os.Stat(filepath.Join(dir, "repos.json"))
	require.Nil(t, err)
	// File holds private keys; must be 0600.
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestRepo_StringTruncates(t *testing.T) {
	r := &Repo{URL: "https://example", Identity: "AGE-SECRET-KEY-VERYLONGSECRETSTRINGTHATSHOULDBETRUNCATED"}
	out := r.String()
	assert.Contains(t, out, "https://example")
	assert.Contains(t, out, "...")
	assert.NotContains(t, out, "VERYLONGSECRETSTRING")
}

func TestRepo_StringShortKeyFullDisplay(t *testing.T) {
	r := &Repo{URL: "u", Identity: "short"}
	assert.Contains(t, r.String(), "short")
}
