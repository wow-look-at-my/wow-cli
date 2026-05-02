package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func TestSources_LoadEmpty(t *testing.T) {
	withTempState(t)
	s, err := LoadSources()
	require.Nil(t, err)
	assert.Equal(t, 0, len(s.All()))
}

func TestSources_AddSaveLoad(t *testing.T) {
	withTempState(t)
	s, err := LoadSources()
	require.Nil(t, err)
	s.Add(&Source{URL: "https://example/manifest.age", Identity: "AGE-SECRET-KEY-XYZ"})
	require.NoError(t, s.Save())

	s2, err := LoadSources()
	require.Nil(t, err)
	require.Equal(t, 1, len(s2.All()))

	got := s2.Find("https://example/manifest.age")
	require.NotNil(t, got)
	assert.Equal(t, "AGE-SECRET-KEY-XYZ", got.Identity)
}

func TestSources_AddReplacesByURL(t *testing.T) {
	withTempState(t)
	s, _ := LoadSources()
	s.Add(&Source{URL: "https://a", Identity: "AGE-SECRET-KEY-1"})
	s.Add(&Source{URL: "https://a", Identity: "AGE-SECRET-KEY-2"})

	assert.Equal(t, 1, len(s.All()))
	assert.Equal(t, "AGE-SECRET-KEY-2", s.All()[0].Identity)
}

func TestSources_Remove(t *testing.T) {
	withTempState(t)
	s, _ := LoadSources()
	s.Add(&Source{URL: "https://a", Identity: "AGE-SECRET-KEY-1"})
	s.Add(&Source{URL: "https://b", Identity: "AGE-SECRET-KEY-2"})

	removed := s.Remove("https://a")
	require.NotNil(t, removed)
	assert.Equal(t, "https://a", removed.URL)
	assert.Equal(t, 1, len(s.All()))

	assert.Nil(t, s.Remove("https://nope"))
}

func TestSources_FilePermissions(t *testing.T) {
	withTempState(t)
	s, _ := LoadSources()
	s.Add(&Source{URL: "https://a", Identity: "AGE-SECRET-KEY-1"})
	require.NoError(t, s.Save())

	dir, _ := StateDir()
	info, err := os.Stat(filepath.Join(dir, "sources.json"))
	require.Nil(t, err)
	// File holds private keys; must be 0600.
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestSources_StringTruncates(t *testing.T) {
	src := &Source{URL: "https://example", Identity: "AGE-SECRET-KEY-VERYLONGSECRETSTRINGTHATSHOULDBETRUNCATED"}
	out := src.String()
	assert.Contains(t, out, "https://example")
	assert.Contains(t, out, "...")
	assert.NotContains(t, out, "VERYLONGSECRETSTRING")
}

func TestSources_StringShortKeyFullDisplay(t *testing.T) {
	src := &Source{URL: "u", Identity: "short"}
	assert.Contains(t, src.String(), "short")
}
