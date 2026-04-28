package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

// testUpdaterConfig, when non-nil, is used instead of the default Config.
// Set only from tests.
var testUpdaterConfig *selfupdate.Config

// buildVersion is set at build time via -ldflags.
var buildVersion string

var rootCmd = &cobra.Command{
	Use:   "wow",
	Short: "Package manager for go-toolchain autorelease pattern programs",
	Long: `wow installs and manages programs that publish GitHub releases
with assets named <binary>_<os>_<arch>[.exe], following the
go-toolchain autorelease convention.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// binaryExt returns ".exe" on Windows, empty string elsewhere.
func binaryExt() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// defaultInstallPath returns ~/.local/bin/<name>[.exe].
func defaultInstallPath(name string) (string, error) {
	binDir, err := store.DefaultBinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(binDir, name+binaryExt()), nil
}

// newUpdater creates an Updater, using testUpdaterConfig when set.
func newUpdater() (*selfupdate.Updater, error) {
	if testUpdaterConfig != nil {
		return selfupdate.NewUpdater(*testUpdaterConfig)
	}
	return selfupdate.NewUpdater(selfupdate.Config{Install: installAtomic})
}

// installAtomic atomically writes src to targetPath. It mirrors upstream
// defaultInstall's Windows-friendly .new/.old rename dance but tolerates a
// missing target (fresh install): defaultInstall's unconditional rename of
// targetPath to .old fails when the target doesn't yet exist.
func installAtomic(src io.Reader, targetPath string) error {
	newBytes, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	dir := filepath.Dir(targetPath)
	filename := filepath.Base(targetPath)

	newPath := filepath.Join(dir, fmt.Sprintf(".%s.new", filename))
	if err := os.WriteFile(newPath, newBytes, 0o755); err != nil {
		return err
	}

	oldPath := filepath.Join(dir, fmt.Sprintf(".%s.old", filename))
	_ = os.Remove(oldPath) // stale .old from a prior failed update (Windows)

	_, statErr := os.Lstat(targetPath)
	targetExists := statErr == nil
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		_ = os.Remove(newPath)
		return statErr
	}

	if targetExists {
		if err := os.Rename(targetPath, oldPath); err != nil {
			_ = os.Remove(newPath)
			return err
		}
	}

	if err := os.Rename(newPath, targetPath); err != nil {
		if targetExists {
			if rerr := os.Rename(oldPath, targetPath); rerr != nil {
				return fmt.Errorf("install failed (%w) and rollback also failed (%v)", err, rerr)
			}
		}
		return err
	}

	if targetExists {
		_ = os.Remove(oldPath)
	}
	return nil
}

// normalizeSlug expands a bare repo name to "wow-look-at-my/<name>".
func normalizeSlug(slug string) string {
	if !strings.Contains(slug, "/") {
		return "wow-look-at-my/" + slug
	}
	return slug
}

// detectLatest returns the latest release for the given slug.
func detectLatest(slug string) (*selfupdate.Release, error) {
	up, err := newUpdater()
	if err != nil {
		return nil, err
	}
	rel, found, err := up.DetectLatest(context.Background(), selfupdate.ParseSlug(slug))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("no release found for %s", slug)
	}
	return rel, nil
}

// installRelease downloads rel and places it at dest.
func installRelease(rel *selfupdate.Release, dest string) error {
	up, err := newUpdater()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}
	return up.UpdateTo(context.Background(), rel, dest)
}
