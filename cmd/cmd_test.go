package cmd

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
	"github.com/wow-look-at-my/wow-cli/store"
)

// ---- helpers -------------------------------------------------------------

func withTempState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("WOW_STATE_DIR", dir)
	return dir
}

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	_, err := rootCmd.ExecuteC()
	return buf.String(), err
}

// ---- mock selfupdate source ----------------------------------------------

type mockAsset struct {
	name string
}

func (a *mockAsset) GetID() int64                 { return 1 }
func (a *mockAsset) GetName() string               { return a.name }
func (a *mockAsset) GetSize() int                  { return 100 }
func (a *mockAsset) GetBrowserDownloadURL() string { return "http://fake/" + a.name }

type mockRelease struct {
	tag    string
	assets []selfupdate.SourceAsset
}

func (r *mockRelease) GetID() int64                      { return 1 }
func (r *mockRelease) GetTagName() string                 { return r.tag }
func (r *mockRelease) GetDraft() bool                     { return false }
func (r *mockRelease) GetPrerelease() bool                { return false }
func (r *mockRelease) GetPublishedAt() time.Time          { return time.Time{} }
func (r *mockRelease) GetReleaseNotes() string            { return "" }
func (r *mockRelease) GetName() string                    { return r.tag }
func (r *mockRelease) GetURL() string                     { return "http://fake/releases/" + r.tag }
func (r *mockRelease) GetAssets() []selfupdate.SourceAsset { return r.assets }

type mockSource struct {
	releases []selfupdate.SourceRelease
}

func (s *mockSource) ListReleases(_ context.Context, _ selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	return s.releases, nil
}

func (s *mockSource) DownloadReleaseAsset(_ context.Context, _ *selfupdate.Release, _ int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("#!/bin/sh\necho mock\n")), nil
}

// assetForPlatform returns an asset name matching the current platform.
func assetForPlatform(binary string) string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goos == "windows" {
		return binary + "_" + goos + "_" + goarch + ".exe"
	}
	return binary + "_" + goos + "_" + goarch
}

// withMockUpdater installs a mock selfupdate config for the duration of the test.
func withMockUpdater(t *testing.T, binary, tag string) {
	t.Helper()
	asset := assetForPlatform(binary)
	cfg := selfupdate.Config{
		Source: &mockSource{
			releases: []selfupdate.SourceRelease{
				&mockRelease{
					tag:    tag,
					assets: []selfupdate.SourceAsset{&mockAsset{name: asset}},
				},
			},
		},
		Install: func(r io.Reader, dest string) error {
			return os.WriteFile(dest, []byte("#!/bin/sh\necho mock\n"), 0o755)
		},
	}
	testUpdaterConfig = &cfg
	t.Cleanup(func() { testUpdaterConfig = nil })
}

// ---- platform detection tests --------------------------------------------

func TestBinaryExt(t *testing.T) {
	ext := binaryExt()
	if runtime.GOOS == "windows" {
		if ext != ".exe" {
			t.Errorf("expected .exe on windows, got %q", ext)
		}
	} else {
		if ext != "" {
			t.Errorf("expected empty on non-windows, got %q", ext)
		}
	}
}

func TestDefaultInstallPath(t *testing.T) {
	path, err := defaultInstallPath("mytool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(path, "mytool") {
		t.Errorf("expected path to contain 'mytool', got %q", path)
	}
	if !strings.Contains(path, ".local") || !strings.Contains(path, "bin") {
		t.Errorf("expected default install path under ~/.local/bin, got %q", path)
	}
}

func TestNewUpdater(t *testing.T) {
	up, err := newUpdater()
	if err != nil {
		t.Fatal(err)
	}
	if up == nil {
		t.Error("newUpdater returned nil")
	}
}

// ---- list command --------------------------------------------------------

func TestList_Empty(t *testing.T) {
	withTempState(t)
	out, err := execute(t, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No packages installed") {
		t.Errorf("expected 'No packages installed', got %q", out)
	}
}

func TestList_WithPackages(t *testing.T) {
	withTempState(t)
	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/tool", Name: "tool", Path: "/bin/tool", Version: "v1.0.0"})
	s.Save()

	out, err := execute(t, "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "owner/tool") || !strings.Contains(out, "v1.0.0") {
		t.Errorf("expected package info in output, got %q", out)
	}
}

// ---- which command -------------------------------------------------------

func TestWhich_Found(t *testing.T) {
	withTempState(t)
	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/tool", Name: "tool", Path: "/bin/tool", Version: "v1"})
	s.Save()

	out, err := execute(t, "which", "tool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/bin/tool") {
		t.Errorf("expected /bin/tool in output, got %q", out)
	}
}

func TestWhich_BySlug(t *testing.T) {
	withTempState(t)
	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/tool", Name: "tool", Path: "/bin/tool", Version: "v1"})
	s.Save()

	out, err := execute(t, "which", "owner/tool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "/bin/tool") {
		t.Errorf("expected /bin/tool, got %q", out)
	}
}

func TestWhich_NotFound(t *testing.T) {
	withTempState(t)
	_, err := execute(t, "which", "nobody")
	if err == nil {
		t.Error("expected error for missing package")
	}
}

// ---- uninstall command ---------------------------------------------------

func TestUninstall_NotFound(t *testing.T) {
	withTempState(t)
	// non-existent package: prints to stderr but doesn't return error
	out, _ := execute(t, "uninstall", "nobody")
	if !strings.Contains(out, "not installed") {
		t.Errorf("expected 'not installed' message, got %q", out)
	}
}

func TestUninstall_Found(t *testing.T) {
	withTempState(t)
	// create a real temp binary so rm succeeds
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "mytool")
	os.WriteFile(binPath, []byte("#!/bin/sh"), 0o755)

	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: binPath, Version: "v1"})
	s.Save()

	out, err := execute(t, "uninstall", "mytool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Uninstalled") {
		t.Errorf("expected 'Uninstalled' in output, got %q", out)
	}
	if _, err := os.Stat(binPath); !os.IsNotExist(err) {
		t.Error("binary file should have been deleted")
	}
}

// ---- install command -----------------------------------------------------

func TestInstall(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v0.0.1")

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "mytool")

	out, err := execute(t, "install", "owner/mytool", "--path", destPath)
	if err != nil {
		t.Fatalf("install failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Installed") {
		t.Errorf("expected 'Installed' in output, got %q", out)
	}

	// verify store entry
	s, _ := store.Load()
	pkg := s.Find("mytool")
	if pkg == nil {
		t.Fatal("package not found in store")
	}
	if pkg.Version != "v0.0.1" {
		t.Errorf("expected version v0.0.1, got %q", pkg.Version)
	}
}

func TestInstall_DefaultName(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mything", "v1.0.0")

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "mything")

	_, err := execute(t, "install", "org/mything", "--path", destPath)
	if err != nil {
		t.Fatal(err)
	}

	s, _ := store.Load()
	pkg := s.Find("org/mything")
	if pkg == nil {
		t.Fatal("package not found")
	}
	if pkg.Name != "mything" {
		t.Errorf("expected name 'mything', got %q", pkg.Name)
	}
}

// ---- update command ------------------------------------------------------

func TestUpdate_NoPackages(t *testing.T) {
	withTempState(t)
	out, err := execute(t, "update")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "No packages installed") {
		t.Errorf("expected 'No packages installed', got %q", out)
	}
}

func TestUpdate_AlreadyLatest(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v1.0.0")

	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: "/tmp/mytool", Version: "v1.0.0"})
	s.Save()

	out, err := execute(t, "update", "mytool")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("expected 'already up to date', got %q", out)
	}
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
	if err != nil {
		t.Fatalf("update failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "Updated") || !strings.Contains(out, "v2.0.0") {
		t.Errorf("expected 'Updated' to v2.0.0 in output, got %q", out)
	}

	s2, _ := store.Load()
	pkg := s2.Find("mytool")
	if pkg == nil || pkg.Version != "v2.0.0" {
		t.Errorf("expected store updated to v2.0.0, got %+v", pkg)
	}
}

func TestUpdate_NotInstalled(t *testing.T) {
	withTempState(t)
	_, err := execute(t, "update", "nonexistent")
	if err == nil {
		t.Error("expected error for non-installed package")
	}
}
