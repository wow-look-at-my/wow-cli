package cmd

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
	"github.com/wow-look-at-my/wow-cli/manifest"
	"github.com/wow-look-at-my/wow-cli/store"
)

// newTestKeyPair returns a fresh age X25519 keypair for tests.
func newTestKeyPair(t *testing.T) (recipient, identity string) {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	require.Nil(t, err)
	return id.Recipient().String(), id.String()
}

// startManifestServer publishes m encrypted to recipient and returns its URL.
func startManifestServer(t *testing.T, m *manifest.Manifest, recipient string) string {
	t.Helper()
	ciphertext, err := manifest.Encrypt(m, recipient)
	require.Nil(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(ciphertext)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// startBinaryServer hosts a fake "binary" and returns its URL.
func startBinaryServer(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func platformAsset(binary string) string {
	suffix := "_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		suffix += ".exe"
	}
	return binary + suffix
}

// ---- add-src / list-src / remove-src -------------------------------------

func TestAddSrc_AddsAndPersists(t *testing.T) {
	withTempState(t)
	recipient, identity := newTestKeyPair(t)

	m := manifest.New()
	url := startManifestServer(t, m, recipient)

	out, err := execute(t, "add-src", url, identity)
	require.Nil(t, err)
	assert.Contains(t, out, "Added source")

	s, _ := store.LoadSources()
	require.Equal(t, 1, len(s.All()))
	assert.Equal(t, identity, s.All()[0].Identity)
}

func TestAddSrc_RejectsBadKey(t *testing.T) {
	withTempState(t)
	recipient, _ := newTestKeyPair(t)
	_, otherIdentity := newTestKeyPair(t)

	url := startManifestServer(t, manifest.New(), recipient)

	_, err := execute(t, "add-src", url, otherIdentity)
	assert.NotNil(t, err)

	// Source should NOT have been saved when verification fails.
	s, _ := store.LoadSources()
	assert.Equal(t, 0, len(s.All()))
}

func TestAddSrc_UnreachableURL(t *testing.T) {
	withTempState(t)
	_, identity := newTestKeyPair(t)
	_, err := execute(t, "add-src", "http://127.0.0.1:1/nope", identity)
	assert.NotNil(t, err)
}

func TestListSrc_Empty(t *testing.T) {
	withTempState(t)
	out, err := execute(t, "list-src")
	require.Nil(t, err)
	assert.Contains(t, out, "No sources configured")
}

func TestListSrc_PrintsTruncatedKey(t *testing.T) {
	withTempState(t)
	s, _ := store.LoadSources()
	s.Add(&store.Source{URL: "https://example/manifest.age", Identity: "AGE-SECRET-KEY-VERYLONGSECRETSTRINGTHATSHOULDBETRUNCATED"})
	require.NoError(t, s.Save())

	out, err := execute(t, "list-src")
	require.Nil(t, err)
	assert.Contains(t, out, "https://example/manifest.age")
	assert.Contains(t, out, "...")
	assert.NotContains(t, out, "VERYLONGSECRETSTRING")
}

func TestRemoveSrc_Removes(t *testing.T) {
	withTempState(t)
	s, _ := store.LoadSources()
	s.Add(&store.Source{URL: "https://a", Identity: "AGE-SECRET-KEY-1"})
	s.Add(&store.Source{URL: "https://b", Identity: "AGE-SECRET-KEY-2"})
	require.NoError(t, s.Save())

	out, err := execute(t, "remove-src", "https://a")
	require.Nil(t, err)
	assert.Contains(t, out, "Removed source")

	s2, _ := store.LoadSources()
	assert.Equal(t, 1, len(s2.All()))
	assert.Equal(t, "https://b", s2.All()[0].URL)
}

func TestRemoveSrc_NotConfigured(t *testing.T) {
	withTempState(t)
	_, err := execute(t, "remove-src", "https://nope")
	assert.NotNil(t, err)
}

// ---- install via source --------------------------------------------------

func TestInstall_FromSource_DownloadsFromManifestURL(t *testing.T) {
	withTempState(t)

	recipient, identity := newTestKeyPair(t)
	binSrv := startBinaryServer(t, "#!/bin/sh\necho hi from manifest\n")

	m := manifest.New()
	m.Packages["owner/mytool"] = &manifest.Package{
		Slug:	"owner/mytool",
		Latest:	"v3.2.1",
		Releases: []*manifest.Release{
			{Tag: "v3.2.1", Assets: []*manifest.Asset{
				{Name: platformAsset("mytool"), URL: binSrv},
			}},
		},
	}
	manifestURL := startManifestServer(t, m, recipient)

	src, _ := store.LoadSources()
	src.Add(&store.Source{URL: manifestURL, Identity: identity})
	require.NoError(t, src.Save())

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "mytool")

	out, err := execute(t, "install", "owner/mytool", "--path", dest)
	require.Nil(t, err)
	assert.Contains(t, out, "from source")
	assert.Contains(t, out, "v3.2.1")

	// Recorded in store with the manifest's tag, no GitHub API hit needed.
	pkgStore, _ := store.Load()
	pkg := pkgStore.Find("owner/mytool")
	require.NotNil(t, pkg)
	assert.Equal(t, "v3.2.1", pkg.Version)
}

func TestInstall_NoMatchInSource_FallsBackToGitHub(t *testing.T) {
	withTempState(t)

	recipient, identity := newTestKeyPair(t)
	// Manifest has no packages, so install must fall back to the (mocked)
	// GitHub source.
	manifestURL := startManifestServer(t, manifest.New(), recipient)

	src, _ := store.LoadSources()
	src.Add(&store.Source{URL: manifestURL, Identity: identity})
	require.NoError(t, src.Save())

	withMockUpdater(t, "mytool", "v0.0.1")

	tmp := t.TempDir()
	dest := filepath.Join(tmp, "mytool")
	out, err := execute(t, "install", "owner/mytool", "--path", dest)
	require.Nil(t, err)
	// Falls back to GH path: "Fetching latest release for ..."
	assert.Contains(t, out, "Fetching latest release")
	assert.Contains(t, out, "Installed")
}

// ---- update via source ---------------------------------------------------

func TestUpdate_FromSource_PicksUpNewVersion(t *testing.T) {
	withTempState(t)

	recipient, identity := newTestKeyPair(t)
	binSrv := startBinaryServer(t, "#!/bin/sh\necho new\n")

	m := manifest.New()
	m.Packages["owner/mytool"] = &manifest.Package{
		Slug:	"owner/mytool",
		Latest:	"v2.0.0",
		Releases: []*manifest.Release{
			{Tag: "v2.0.0", Assets: []*manifest.Asset{
				{Name: platformAsset("mytool"), URL: binSrv},
			}},
		},
	}
	manifestURL := startManifestServer(t, m, recipient)

	src, _ := store.LoadSources()
	src.Add(&store.Source{URL: manifestURL, Identity: identity})
	require.NoError(t, src.Save())

	// Pre-existing installed package at v1.0.0 — wow update should bump it.
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "mytool")
	pkgs, _ := store.Load()
	pkgs.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: binPath, Version: "v1.0.0"})
	require.NoError(t, pkgs.Save())

	out, err := execute(t, "update")
	require.Nil(t, err)
	assert.Contains(t, out, "Updated mytool")
	assert.Contains(t, out, "v2.0.0")
	assert.True(t, strings.Contains(out, "from source"))
}

func TestUpdate_FromSource_AlreadyLatest(t *testing.T) {
	withTempState(t)

	recipient, identity := newTestKeyPair(t)
	binSrv := startBinaryServer(t, "stub")

	m := manifest.New()
	m.Packages["owner/mytool"] = &manifest.Package{
		Slug:	"owner/mytool",
		Latest:	"v1.0.0",
		Releases: []*manifest.Release{
			{Tag: "v1.0.0", Assets: []*manifest.Asset{
				{Name: platformAsset("mytool"), URL: binSrv},
			}},
		},
	}
	manifestURL := startManifestServer(t, m, recipient)

	src, _ := store.LoadSources()
	src.Add(&store.Source{URL: manifestURL, Identity: identity})
	require.NoError(t, src.Save())

	pkgs, _ := store.Load()
	pkgs.Add(&store.Package{Slug: "owner/mytool", Name: "mytool", Path: "/tmp/mytool", Version: "v1.0.0"})
	require.NoError(t, pkgs.Save())

	out, err := execute(t, "update")
	require.Nil(t, err)
	assert.Contains(t, out, "already up to date")
}

// ---- platformAssetName ---------------------------------------------------

func TestPlatformAssetName(t *testing.T) {
	got := platformAssetName("wow")
	wantSuffix := "_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		wantSuffix += ".exe"
	}
	assert.Equal(t, "wow"+wantSuffix, got)
}
