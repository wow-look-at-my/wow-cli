package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
	"github.com/wow-look-at-my/wow-cli/store"
)

// ---- update command ------------------------------------------------------

func TestUpdate_NoPackages(t *testing.T) {
	withTempState(t)
	out, err := execute(t, "update")
	require.Nil(t, err)

	assert.Contains(t, out, "No packages installed")

}

func TestUpdate_AlreadyLatest(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v1.0.0")

	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: "/tmp/mytool", Version: "v1.0.0"})
	s.Save()

	out, err := execute(t, "update")
	require.Nil(t, err)

	assert.Contains(t, out, "already up to date")

}

func TestUpdate_NewVersion(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v2.0.0")

	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "mytool")

	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: binPath, Version: "v1.0.0"})
	s.Save()

	out, err := execute(t, "update")
	require.Nil(t, err)

	assert.False(t, !strings.Contains(out, "Updated") || !strings.Contains(out, "v2.0.0"))

	s2, _ := store.Load()
	pkg := s2.Find("mytool")
	assert.False(t, pkg == nil || pkg.Version != "v2.0.0")

}

func TestUpdate_RejectsArgs(t *testing.T) {
	withTempState(t)
	_, err := execute(t, "update", "nonexistent")
	assert.NotNil(t, err)

}

func TestUpdate_SelfUpdateAlreadyLatest(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "wow-cli", "v2.0.0")

	old := selfupdate.EmbeddedVersion
	selfupdate.EmbeddedVersion = "v2.0.0"
	t.Cleanup(func() { selfupdate.EmbeddedVersion = old })

	out, err := execute(t, "update")
	require.Nil(t, err)

	assert.Contains(t, out, "wow is already up to date")
	assert.Contains(t, out, "v2.0.0")
}

func TestUpdate_SelfUpdateNewVersion(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "wow-cli", "v2.0.0")

	// Write a temp file to stand in for the wow executable; UpdateCommand
	// stats cmdPath before installing.
	tmp := t.TempDir()
	exePath := filepath.Join(tmp, "wow")
	os.WriteFile(exePath, []byte("#!/bin/sh\necho old\n"), 0o755)

	oldExe := wowExePathOverride
	wowExePathOverride = exePath
	t.Cleanup(func() { wowExePathOverride = oldExe })

	old := selfupdate.EmbeddedVersion
	selfupdate.EmbeddedVersion = "v1.0.0"
	t.Cleanup(func() { selfupdate.EmbeddedVersion = old })

	out, err := execute(t, "update")
	require.Nil(t, err)

	assert.Contains(t, out, "Updated wow")
	assert.Contains(t, out, "v1.0.0")
	assert.Contains(t, out, "v2.0.0")
}

func TestUpdate_SelfUpdateUsesRealExePath(t *testing.T) {
	withTempState(t)

	// Mock that detects a newer version but errors on install. With no
	// wowExePathOverride, selfUpdateWow resolves os.Executable() (the test
	// binary), UpdateCommand stats it (it exists), and the Install error then
	// propagates back through UpdateTo.
	asset := assetForPlatform("wow-cli")
	cfg := selfupdate.Config{
		Source: &mockSource{
			releases: []selfupdate.SourceRelease{
				&mockRelease{tag: "v2.0.0", assets: []selfupdate.SourceAsset{&mockAsset{name: asset}}},
			},
		},
		Install: func(_ io.Reader, _ string) error {
			return fmt.Errorf("install aborted")
		},
	}
	testUpdaterConfig = &cfg
	t.Cleanup(func() { testUpdaterConfig = nil })

	old := selfupdate.EmbeddedVersion
	selfupdate.EmbeddedVersion = "v1.0.0"
	t.Cleanup(func() { selfupdate.EmbeddedVersion = old })

	out, err := execute(t, "update")
	require.NotNil(t, err)
	assert.Contains(t, out, "Checking latest release")
}

func TestUpdate_SelfUpdate_NoReleasesIsBenign(t *testing.T) {
	withTempState(t)

	// Source with zero releases. UpdateCommand returns the current version as
	// the "latest" without calling Install, so selfUpdateWow prints
	// "already up to date" instead of erroring.
	cfg := selfupdate.Config{Source: &mockSource{releases: nil}}
	testUpdaterConfig = &cfg
	t.Cleanup(func() { testUpdaterConfig = nil })

	old := selfupdate.EmbeddedVersion
	selfupdate.EmbeddedVersion = "v1.0.0"
	t.Cleanup(func() { selfupdate.EmbeddedVersion = old })

	out, err := execute(t, "update")
	require.Nil(t, err)
	assert.Contains(t, out, "already up to date")
}

func TestUpdate_SelfUpdate_SkipsDirty(t *testing.T) {
	withTempState(t)

	// Dirty (or "(devel)") versions must short-circuit before any release
	// lookup: if selfUpdateWow proceeded, it would print "Checking latest
	// release for wow-look-at-my/wow-cli..." and we'd see it in stdout.
	withMockUpdater(t, "wow-cli", "v9.9.9")

	old := selfupdate.EmbeddedVersion
	selfupdate.EmbeddedVersion = "v1.0.0+dirty"
	t.Cleanup(func() { selfupdate.EmbeddedVersion = old })

	out, err := execute(t, "update")
	require.Nil(t, err)
	assert.NotContains(t, out, "Checking latest release")
}

func TestInstall_WithVersion(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v0.0.1")
	t.Cleanup(func() { installVersion = "" })

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "mytool")

	out, err := execute(t, "install", "owner/mytool", "--path", destPath, "--version", "v0.0.1")
	require.Nil(t, err)

	assert.Contains(t, out, "Installed")

}

func TestExecute_Succeeds(t *testing.T) {
	withTempState(t)
	rootCmd.SetArgs([]string{"list"})
	rootCmd.SetOut(new(bytes.Buffer))
	Execute()
}
