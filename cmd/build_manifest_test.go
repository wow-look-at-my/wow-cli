package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
	"github.com/wow-look-at-my/wow-cli/manifest"
)

// startGitHubAPIMock serves both /search/repositories and
// /repos/<owner>/<repo>/releases endpoints with canned data.
func startGitHubAPIMock(t *testing.T, items []map[string]any, releases map[string][]map[string]any) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/search/repositories"):
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items})
		case strings.HasPrefix(r.URL.Path, "/repos/") && strings.HasSuffix(r.URL.Path, "/releases"):
			// /repos/<owner>/<repo>/releases
			parts := strings.Split(r.URL.Path, "/")
			if len(parts) >= 5 {
				slug := parts[2] + "/" + parts[3]
				if rels, ok := releases[slug]; ok {
					_ = json.NewEncoder(w).Encode(rels)
					return
				}
			}
			_ = json.NewEncoder(w).Encode([]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func withMockGitHub(t *testing.T, items []map[string]any, releases map[string][]map[string]any) {
	t.Helper()
	url := startGitHubAPIMock(t, items, releases)
	oldSearch := ghSearchBaseURL
	oldReleases := ghReleasesBaseURL
	ghSearchBaseURL = url
	ghReleasesBaseURL = url
	t.Cleanup(func() {
		ghSearchBaseURL = oldSearch
		ghReleasesBaseURL = oldReleases
	})
}

// resetBuildManifestFlags restores package-level cobra flag vars between tests.
func resetBuildManifestFlags(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		buildManifestOrg = "wow-look-at-my"
		buildManifestRecipient = ""
		buildManifestOutput = "-"
		buildManifestUnencrypt = false
	})
}

func TestBuildManifest_Plain(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t,
		[]map[string]any{
			{"full_name": "wow-look-at-my/atool", "description": "tool A"},
			{"full_name": "wow-look-at-my/btool", "description": "tool B"},
		},
		map[string][]map[string]any{
			"wow-look-at-my/atool": {{
				"tag_name": "v1.0.0",
				"assets": []map[string]any{
					{"name": "atool_linux_amd64", "size": 100, "browser_download_url": "http://a/atool_linux_amd64"},
				},
			}},
			"wow-look-at-my/btool": {}, // no releases — should be skipped
		},
	)

	tmp := t.TempDir()
	out := filepath.Join(tmp, "manifest.json")
	_, err := execute(t, "build-manifest", "--plain", "--output", out, "--org", "wow-look-at-my")
	require.Nil(t, err)

	data, err := os.ReadFile(out)
	require.Nil(t, err)

	var m manifest.Manifest
	require.NoError(t, json.Unmarshal(data, &m))
	assert.Equal(t, manifest.SchemaVersion, m.SchemaVersion)
	assert.Equal(t, 1, len(m.Packages))

	pkg, ok := m.Packages["wow-look-at-my/atool"]
	require.True(t, ok)
	assert.Equal(t, "v1.0.0", pkg.Latest)
	assert.Equal(t, "tool A", pkg.Description)
	assert.Equal(t, "atool_linux_amd64", pkg.Releases[0].Assets[0].Name)
	assert.Equal(t, "http://a/atool_linux_amd64", pkg.Releases[0].Assets[0].URL)
}

func TestBuildManifest_EncryptedRoundtrip(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t,
		[]map[string]any{{"full_name": "wow-look-at-my/atool", "description": ""}},
		map[string][]map[string]any{
			"wow-look-at-my/atool": {{
				"tag_name": "v0.1.0",
				"assets":   []map[string]any{{"name": "atool_linux_amd64", "browser_download_url": "http://a"}},
			}},
		},
	)

	recipient, identity := newTestKeyPair(t)
	tmp := t.TempDir()
	out := filepath.Join(tmp, "manifest.age")
	_, err := execute(t, "build-manifest", "--recipient", recipient, "--output", out)
	require.Nil(t, err)

	data, err := os.ReadFile(out)
	require.Nil(t, err)

	m, err := manifest.Decrypt(data, identity)
	require.Nil(t, err)
	assert.Equal(t, "v0.1.0", m.Packages["wow-look-at-my/atool"].Latest)
}

func TestBuildManifest_RecipientRequired(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t, nil, nil)
	t.Setenv("WOW_MANIFEST_RECIPIENT", "")
	_, err := execute(t, "build-manifest")
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "recipient is required")
}

func TestBuildManifest_SkipsDraftAndPrerelease(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t,
		[]map[string]any{{"full_name": "wow-look-at-my/atool", "description": ""}},
		map[string][]map[string]any{
			"wow-look-at-my/atool": {
				{"tag_name": "v2.0.0-rc1", "prerelease": true, "assets": []map[string]any{{"name": "atool_linux_amd64"}}},
				{"tag_name": "v0.0.0-draft", "draft": true, "assets": []map[string]any{{"name": "atool_linux_amd64"}}},
				{"tag_name": "v1.0.0", "assets": []map[string]any{{"name": "atool_linux_amd64", "browser_download_url": "http://a"}}},
			},
		},
	)

	tmp := t.TempDir()
	out := filepath.Join(tmp, "manifest.json")
	_, err := execute(t, "build-manifest", "--plain", "--output", out)
	require.Nil(t, err)

	data, err := os.ReadFile(out)
	require.Nil(t, err)

	var m manifest.Manifest
	require.NoError(t, json.Unmarshal(data, &m))
	pkg := m.Packages["wow-look-at-my/atool"]
	require.NotNil(t, pkg)
	assert.Equal(t, "v1.0.0", pkg.Latest)
	assert.Equal(t, 1, len(pkg.Releases))
}
