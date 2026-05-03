package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
	"github.com/wow-look-at-my/wow-cli/manifest"
	"github.com/wow-look-at-my/wow-cli/store"
)

// ---- search command (manifest-backed) ----------------------------------------

func withManifestSearch(t *testing.T, m *manifest.Manifest) {
	t.Helper()
	dir := withTempState(t)

	recipient, identity := newTestKeyPair(t)
	url := startManifestServer(t, m, recipient)

	s, err := store.LoadRepoList()
	require.Nil(t, err)
	s.Add(&store.Repo{URL: url, Identity: identity})
	require.Nil(t, s.Save())

	_ = dir
}

func testManifest() *manifest.Manifest {
	m := manifest.New()
	m.Packages["wow-look-at-my/go-toolchain"] = &manifest.Package{
		Slug:        "wow-look-at-my/go-toolchain",
		Description: "Go toolchain manager",
		Latest:      "v1.0.0",
	}
	m.Packages["wow-look-at-my/ccze-go"] = &manifest.Package{
		Slug:        "wow-look-at-my/ccze-go",
		Description: "ccze colorizer in Go",
		Latest:      "v0.0.1",
	}
	m.Packages["wow-look-at-my/nerf"] = &manifest.Package{
		Slug:   "wow-look-at-my/nerf",
		Latest: "v0.1.0",
	}
	return m
}

func TestSearch_Results(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search", "go-toolchain")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/go-toolchain")
	assert.Contains(t, out, "Go toolchain manager")
	assert.NotContains(t, out, "ccze-go")
}

func TestSearch_NoResults(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search", "nonexistent")
	require.Nil(t, err)

	assert.Contains(t, out, "No results found")
}

func TestSearch_NoDescription(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search", "nerf")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/nerf")
	assert.NotContains(t, out, "—")
}

func TestSearch_NoArgs_ListsAll(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/go-toolchain")
	assert.Contains(t, out, "wow-look-at-my/ccze-go")
	assert.Contains(t, out, "wow-look-at-my/nerf")
}

func TestSearch_Wildcard_ListsAll(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search", "*")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/go-toolchain")
	assert.Contains(t, out, "wow-look-at-my/ccze-go")
	assert.Contains(t, out, "wow-look-at-my/nerf")
}

func TestSearch_MatchesDescription(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search", "colorizer")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/ccze-go")
	assert.NotContains(t, out, "go-toolchain")
}

func TestSearch_CaseInsensitive(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search", "GO-TOOLCHAIN")
	require.Nil(t, err)

	assert.Contains(t, out, "wow-look-at-my/go-toolchain")
}

func TestSearch_Sorted(t *testing.T) {
	withManifestSearch(t, testManifest())

	out, err := execute(t, "search")
	require.Nil(t, err)

	lines := out
	ccze := indexOf(lines, "ccze-go")
	toolchain := indexOf(lines, "go-toolchain")
	nerf := indexOf(lines, "nerf")
	assert.True(t, ccze < toolchain, "ccze-go should come before go-toolchain")
	assert.True(t, toolchain < nerf, "go-toolchain should come before nerf")
}

func indexOf(s, sub string) int {
	for i := 0; i < len(s); i++ {
		if len(s[i:]) >= len(sub) && s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestSearch_NoRepos(t *testing.T) {
	withTempState(t)

	out, err := execute(t, "search")
	require.Nil(t, err)

	assert.Contains(t, out, "No repos configured")
}

func TestSearch_RepoFlag(t *testing.T) {
	dir := withTempState(t)

	recipient1, identity1 := newTestKeyPair(t)
	recipient2, identity2 := newTestKeyPair(t)

	m1 := manifest.New()
	m1.Packages["org/tool-a"] = &manifest.Package{Slug: "org/tool-a", Description: "Tool A", Latest: "v1"}
	url1 := startManifestServer(t, m1, recipient1)

	m2 := manifest.New()
	m2.Packages["org/tool-b"] = &manifest.Package{Slug: "org/tool-b", Description: "Tool B", Latest: "v1"}
	url2 := startManifestServer(t, m2, recipient2)

	s, err := store.LoadRepoList()
	require.Nil(t, err)
	s.Add(&store.Repo{URL: url1, Identity: identity1})
	s.Add(&store.Repo{URL: url2, Identity: identity2})
	require.Nil(t, s.Save())

	_ = dir

	// --repo filters to just one source
	old := searchRepo
	searchRepo = url1
	t.Cleanup(func() { searchRepo = old })

	out, err := execute(t, "search", "--repo", url1)
	require.Nil(t, err)

	assert.Contains(t, out, "org/tool-a")
	assert.NotContains(t, out, "org/tool-b")
}

func TestSearch_RepoFlag_NoMatch(t *testing.T) {
	withManifestSearch(t, testManifest())

	old := searchRepo
	searchRepo = "http://no-such-repo.test/manifest.age"
	t.Cleanup(func() { searchRepo = old })

	out, err := execute(t, "search", "--repo", "http://no-such-repo.test/manifest.age")
	require.Nil(t, err)

	assert.Contains(t, out, "No results found")
}

// ---- ghSearch (still used by install suggestions) ---------------------------

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

func TestGHSearch_IgnoresGHHost(t *testing.T) {
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

	_, err := ghSearch(t.Context(), "anything")
	require.Nil(t, err)

	assert.Empty(t, gotAuth, "should not send token when GH_HOST is set")
}
