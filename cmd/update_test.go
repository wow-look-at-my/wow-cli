package cmd

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

// ---- install --version flag -------------------------------------------------

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
