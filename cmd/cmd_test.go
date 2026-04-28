package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
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

func (a *mockAsset) GetID() int64			{ return 1 }
func (a *mockAsset) GetName() string			{ return a.name }
func (a *mockAsset) GetSize() int			{ return 100 }
func (a *mockAsset) GetBrowserDownloadURL() string	{ return "http://fake/" + a.name }

type mockRelease struct {
	tag	string
	assets	[]selfupdate.SourceAsset
}

func (r *mockRelease) GetID() int64				{ return 1 }
func (r *mockRelease) GetTagName() string			{ return r.tag }
func (r *mockRelease) GetDraft() bool				{ return false }
func (r *mockRelease) GetPrerelease() bool			{ return false }
func (r *mockRelease) GetPublishedAt() time.Time		{ return time.Time{} }
func (r *mockRelease) GetReleaseNotes() string			{ return "" }
func (r *mockRelease) GetName() string				{ return r.tag }
func (r *mockRelease) GetURL() string				{ return "http://fake/releases/" + r.tag }
func (r *mockRelease) GetAssets() []selfupdate.SourceAsset	{ return r.assets }

type mockSource struct {
	releases []selfupdate.SourceRelease
}

func (s *mockSource) ListReleases(_ context.Context, _ selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	return s.releases, nil
}

func (s *mockSource) DownloadReleaseAsset(_ context.Context, _ *selfupdate.Release, _ int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("#!/bin/sh\necho mock\n")), nil
}

// mockSourcePerSlug returns different releases depending on which repo is queried.
type mockSourcePerSlug struct {
	// keyed by "owner/repo"
	perSlug map[string][]selfupdate.SourceRelease
}

func (s *mockSourcePerSlug) ListReleases(_ context.Context, repo selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	owner, name, _ := repo.GetSlug()
	return s.perSlug[owner+"/"+name], nil
}

func (s *mockSourcePerSlug) DownloadReleaseAsset(_ context.Context, _ *selfupdate.Release, _ int64) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("#!/bin/sh\necho mock\n")), nil
}

// withMockUpdaterPerSlug sets up a slug-aware mock updater for the test duration.
func withMockUpdaterPerSlug(t *testing.T, perSlug map[string][]selfupdate.SourceRelease) {
	t.Helper()
	cfg := selfupdate.Config{
		Source: &mockSourcePerSlug{perSlug: perSlug},
		Install: func(r io.Reader, dest string) error {
			return os.WriteFile(dest, []byte("#!/bin/sh\necho mock\n"), 0o755)
		},
	}
	testUpdaterConfig = &cfg
	t.Cleanup(func() { testUpdaterConfig = nil })
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
					tag:	tag,
					assets:	[]selfupdate.SourceAsset{&mockAsset{name: asset}},
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

// ---- installAtomic -------------------------------------------------------

func TestInstallAtomic_FreshInstall(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "mytool")

	require.Nil(t, installAtomic(strings.NewReader("hello"), target))

	data, err := os.ReadFile(target)
	require.Nil(t, err)
	assert.Equal(t, "hello", string(data))

	// No stale .new/.old siblings should remain.
	entries, _ := os.ReadDir(tmp)
	assert.Equal(t, 1, len(entries))
}

func TestInstallAtomic_OverwritesExisting(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "mytool")
	require.Nil(t, os.WriteFile(target, []byte("old"), 0o755))

	require.Nil(t, installAtomic(strings.NewReader("new"), target))

	data, err := os.ReadFile(target)
	require.Nil(t, err)
	assert.Equal(t, "new", string(data))

	entries, _ := os.ReadDir(tmp)
	assert.Equal(t, 1, len(entries))
}

func TestInstallAtomic_ClearsStaleOld(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "mytool")
	stale := filepath.Join(tmp, ".mytool.old")
	require.Nil(t, os.WriteFile(stale, []byte("stale"), 0o755))

	require.Nil(t, installAtomic(strings.NewReader("fresh"), target))

	data, err := os.ReadFile(target)
	require.Nil(t, err)
	assert.Equal(t, "fresh", string(data))

	_, err = os.Stat(stale)
	assert.True(t, os.IsNotExist(err))
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, fmt.Errorf("read failed") }

func TestInstallAtomic_ReadError(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "mytool")

	err := installAtomic(errReader{}, target)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "read failed")
}

func TestInstallAtomic_WriteError(t *testing.T) {
	// Target directory does not exist — WriteFile of the .new sibling fails.
	target := filepath.Join(t.TempDir(), "no-such-dir", "mytool")

	err := installAtomic(strings.NewReader("hello"), target)
	require.NotNil(t, err)
}

// ---- platform detection tests --------------------------------------------

func TestBinaryExt(t *testing.T) {
	ext := binaryExt()
	if runtime.GOOS == "windows" {
		assert.Equal(t, ".exe", ext)

	} else {
		assert.Equal(t, "", ext)

	}
}

func TestDefaultInstallPath(t *testing.T) {
	path, err := defaultInstallPath("mytool")
	require.Nil(t, err)

	assert.Contains(t, path, "mytool")

	assert.False(t, !strings.Contains(path, ".local") || !strings.Contains(path, "bin"))

}

func TestNewUpdater(t *testing.T) {
	up, err := newUpdater()
	require.Nil(t, err)

	assert.NotNil(t, up)

}

// ---- list command --------------------------------------------------------

func TestList_Empty(t *testing.T) {
	withTempState(t)
	out, err := execute(t, "list")
	require.Nil(t, err)

	assert.Contains(t, out, "No packages installed")

}

func TestList_WithPackages(t *testing.T) {
	withTempState(t)
	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/tool", Name: "tool", Path: "/bin/tool", Version: "v1.0.0"})
	s.Save()

	out, err := execute(t, "list")
	require.Nil(t, err)

	assert.False(t, !strings.Contains(out, "owner/tool") || !strings.Contains(out, "v1.0.0"))

}

// ---- which command -------------------------------------------------------

func TestWhich_Found(t *testing.T) {
	withTempState(t)
	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/tool", Name: "tool", Path: "/bin/tool", Version: "v1"})
	s.Save()

	out, err := execute(t, "which", "tool")
	require.Nil(t, err)

	assert.Contains(t, out, "/bin/tool")

}

func TestWhich_BySlug(t *testing.T) {
	withTempState(t)
	s, _ := store.Load()
	s.Add(&store.Package{Slug: "owner/tool", Name: "tool", Path: "/bin/tool", Version: "v1"})
	s.Save()

	out, err := execute(t, "which", "owner/tool")
	require.Nil(t, err)

	assert.Contains(t, out, "/bin/tool")

}

func TestWhich_NotFound(t *testing.T) {
	withTempState(t)
	_, err := execute(t, "which", "nobody")
	assert.NotNil(t, err)

}

// ---- uninstall command ---------------------------------------------------

func TestUninstall_NotFound(t *testing.T) {
	withTempState(t)
	// non-existent package: prints to stderr but doesn't return error
	out, _ := execute(t, "uninstall", "nobody")
	assert.Contains(t, out, "not installed")

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
	require.Nil(t, err)

	assert.Contains(t, out, "Uninstalled")

	_, statErr := os.Stat(binPath)
	assert.True(t, os.IsNotExist(statErr))

}

// ---- install command -----------------------------------------------------

func TestInstall(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v0.0.1")

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "mytool")

	out, err := execute(t, "install", "owner/mytool", "--path", destPath)
	require.Nil(t, err)

	assert.Contains(t, out, "Installed")

	// verify store entry
	s, _ := store.Load()
	pkg := s.Find("mytool")
	require.NotNil(t, pkg)

	assert.Equal(t, "v0.0.1", pkg.Version)

}

// TestInstall_FreshInstall_RealInstaller exercises the production install
// path (installAtomic) end-to-end, regressing the bug where fresh installs
// failed because the upstream defaultInstall rename'd a non-existent target.
func TestInstall_FreshInstall_RealInstaller(t *testing.T) {
	withTempState(t)

	asset := assetForPlatform("mytool")
	cfg := selfupdate.Config{
		Source: &mockSource{
			releases: []selfupdate.SourceRelease{
				&mockRelease{
					tag:    "v0.0.1",
					assets: []selfupdate.SourceAsset{&mockAsset{name: asset}},
				},
			},
		},
		Install: installAtomic,
	}
	testUpdaterConfig = &cfg
	t.Cleanup(func() { testUpdaterConfig = nil })

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "mytool")

	out, err := execute(t, "install", "owner/mytool", "--path", destPath)
	require.Nil(t, err)
	assert.Contains(t, out, "Installed")

	info, err := os.Stat(destPath)
	require.Nil(t, err)
	assert.True(t, info.Size() > 0)
}

func TestInstall_DefaultName(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mything", "v1.0.0")

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "mything")

	_, err := execute(t, "install", "org/mything", "--path", destPath)
	require.Nil(t, err)

	s, _ := store.Load()
	pkg := s.Find("org/mything")
	require.NotNil(t, pkg)

	assert.Equal(t, "mything", pkg.Name)

}

func TestInstall_BareNameDefaultsOrg(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "mytool", "v1.0.0")

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "mytool")

	_, err := execute(t, "install", "mytool", "--path", destPath)
	require.Nil(t, err)

	s, _ := store.Load()
	pkg := s.Find("mytool")
	require.NotNil(t, pkg)

	assert.Equal(t, "wow-look-at-my/mytool", pkg.Slug)
}

func TestInstall_DidYouMean_UserConfirms(t *testing.T) {
	withTempState(t)

	goAsset := assetForPlatform("ccze-go")
	withMockUpdaterPerSlug(t, map[string][]selfupdate.SourceRelease{
		// wow-look-at-my/ccze has no releases; ccze-go has one
		"wow-look-at-my/ccze-go": {
			&mockRelease{tag: "v0.0.1", assets: []selfupdate.SourceAsset{&mockAsset{name: goAsset}}},
		},
	})
	withMockSearchServer(t, `{"items":[{"full_name":"wow-look-at-my/ccze-go","description":"ccze colorizer"}]}`)

	tmp := t.TempDir()
	destPath := filepath.Join(tmp, "ccze")

	rootCmd.SetIn(strings.NewReader("y\n"))
	out, err := execute(t, "install", "ccze", "--path", destPath)
	require.Nil(t, err)
	assert.Contains(t, out, "Installed")

	s, _ := store.Load()
	pkg := s.Find("ccze")
	require.NotNil(t, pkg)
	assert.Equal(t, "ccze", pkg.Name)
	assert.Equal(t, "wow-look-at-my/ccze-go", pkg.Slug)
}

func TestInstall_DidYouMean_UserDeclines(t *testing.T) {
	withTempState(t)

	goAsset := assetForPlatform("ccze-go")
	withMockUpdaterPerSlug(t, map[string][]selfupdate.SourceRelease{
		"wow-look-at-my/ccze-go": {
			&mockRelease{tag: "v0.0.1", assets: []selfupdate.SourceAsset{&mockAsset{name: goAsset}}},
		},
	})
	withMockSearchServer(t, `{"items":[{"full_name":"wow-look-at-my/ccze-go","description":"ccze colorizer"}]}`)

	rootCmd.SetIn(strings.NewReader("n\n"))
	_, err := execute(t, "install", "ccze")
	assert.NotNil(t, err)
}

func TestInstall_DidYouMean_NoMatch(t *testing.T) {
	withTempState(t)

	withMockUpdaterPerSlug(t, map[string][]selfupdate.SourceRelease{})
	withMockSearchServer(t, `{"items":[]}`)

	_, err := execute(t, "install", "nonexistent")
	assert.NotNil(t, err)
}

func TestInstall_DidYouMean_ExplicitSlugNoSearch(t *testing.T) {
	withTempState(t)

	withMockUpdaterPerSlug(t, map[string][]selfupdate.SourceRelease{})
	// Search server never called for explicit slugs; any accidental call returns nothing.
	withMockSearchServer(t, `{"items":[]}`)

	_, err := execute(t, "install", "owner/ccze")
	assert.NotNil(t, err)
}

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

	old := buildVersion
	buildVersion = "v2.0.0"
	t.Cleanup(func() { buildVersion = old })

	out, err := execute(t, "update")
	require.Nil(t, err)

	assert.Contains(t, out, "wow is already up to date")
	assert.Contains(t, out, "v2.0.0")
}

func TestUpdate_SelfUpdateNewVersion(t *testing.T) {
	withTempState(t)
	withMockUpdater(t, "wow-cli", "v2.0.0")

	// Write a temp file to stand in for the wow executable.
	tmp := t.TempDir()
	exePath := filepath.Join(tmp, "wow")
	os.WriteFile(exePath, []byte("#!/bin/sh\necho old\n"), 0o755)

	// Point selfUpdateWow at the temp file instead of the real binary.
	oldExe := wowExePathOverride
	wowExePathOverride = exePath
	t.Cleanup(func() { wowExePathOverride = oldExe })

	old := buildVersion
	buildVersion = "v1.0.0"
	t.Cleanup(func() { buildVersion = old })

	out, err := execute(t, "update")
	require.Nil(t, err)

	assert.Contains(t, out, "Updating wow")
	assert.Contains(t, out, "v1.0.0")
	assert.Contains(t, out, "v2.0.0")
	assert.Contains(t, out, "Updated wow")
}

func TestUpdate_SelfUpdateUsesRealExePath(t *testing.T) {
	withTempState(t)

	// Mock that detects a newer version but errors on install (safe: no file write).
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

	old := buildVersion
	buildVersion = "v1.0.0"
	t.Cleanup(func() { buildVersion = old })
	// wowExePathOverride intentionally not set — exercises os.Executable() path.

	out, err := execute(t, "update")
	require.NotNil(t, err)
	assert.Contains(t, out, "Updating wow")
}

func TestUpdate_SelfUpdateDetectError(t *testing.T) {
	withTempState(t)

	// Source with no releases produces a "no release found" error from detectLatest.
	cfg := selfupdate.Config{
		Source: &mockSource{releases: nil},
	}
	testUpdaterConfig = &cfg
	t.Cleanup(func() { testUpdaterConfig = nil })

	old := buildVersion
	buildVersion = "v1.0.0"
	t.Cleanup(func() { buildVersion = old })

	_, err := execute(t, "update")
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "no release found")
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

// ---- search command -------------------------------------------------------

func withMockSearchServer(t *testing.T, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(func() {
		srv.Close()
		ghSearchBaseURL = "https://api.github.com"
	})
	ghSearchBaseURL = srv.URL
}

func TestSearch_Results(t *testing.T) {
	withMockSearchServer(t, `{"items":[{"full_name":"wow-look-at-my/go-toolchain","description":"Go toolchain manager"}]}`)

	out, err := execute(t, "search", "go-toolchain")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/go-toolchain")
	assert.Contains(t, out, "Go toolchain manager")
}

func TestSearch_NoResults(t *testing.T) {
	withMockSearchServer(t, `{"items":[]}`)

	out, err := execute(t, "search", "nonexistent")
	require.Nil(t, err)

	assert.Contains(t, out, "No results found")
}

func TestSearch_NoDescription(t *testing.T) {
	withMockSearchServer(t, `{"items":[{"full_name":"wow-look-at-my/bare","description":""}]}`)

	out, err := execute(t, "search", "bare")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/bare")
	assert.NotContains(t, out, "—")
}

func TestSearch_IgnoresGHHost(t *testing.T) {
	t.Setenv("GH_HOST", "github.mycompany.com")
	t.Setenv("GITHUB_TOKEN", "enterprise-token")

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[]}`)
	}))
	t.Cleanup(func() {
		srv.Close()
		ghSearchBaseURL = "https://api.github.com"
	})
	ghSearchBaseURL = srv.URL

	_, err := execute(t, "search", "anything")
	require.Nil(t, err)

	assert.Empty(t, gotAuth, "should not send token when GH_HOST is set")
}
