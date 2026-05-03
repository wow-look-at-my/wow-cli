package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
	"github.com/wow-look-at-my/wow-cli/store"
)

// ---- upgrade command --------------------------------------------------------

func TestUpgrade_NoPackages(t *testing.T) {
	withTempState(t)
	out, err := execute(t, "upgrade")
	require.Nil(t, err)

	assert.Contains(t, out, "No packages installed")
}

func TestUpgrade_AlreadyLatest(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v1.0.0")

	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: "/tmp/mytool", Version: "v1.0.0"})
	s.Save()

	out, err := execute(t, "upgrade")
	require.Nil(t, err)

	assert.Contains(t, out, "already up to date")
}

func TestUpgrade_NewVersion(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v2.0.0")

	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "mytool")

	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: binPath, Version: "v1.0.0"})
	s.Save()

	out, err := execute(t, "upgrade")
	require.Nil(t, err)

	assert.True(t, strings.Contains(out, "Upgraded") && strings.Contains(out, "v2.0.0"))

	s2, _ := store.Load()
	pkg := s2.Find("mytool")
	require.NotNil(t, pkg)
	assert.Equal(t, "v2.0.0", pkg.Version)
}

func TestUpgrade_RejectsArgs(t *testing.T) {
	withTempState(t)
	_, err := execute(t, "upgrade", "nonexistent")
	assert.NotNil(t, err)
}
