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
		buildManifestRecipients = nil
		buildManifestRecipientsFile = "recipients.jsonc"
		buildManifestOutput = "-"
		buildManifestUnencrypt = false
		buildManifestSkipIfEmpty = false
	})
}

func writeRecipientsFile(t *testing.T, dir string, recipients ...manifest.Recipient) string {
	t.Helper()
	path := filepath.Join(dir, "recipients.json")
	data, err := json.MarshalIndent(manifest.RecipientsFile{Recipients: recipients}, "", "  ")
	require.Nil(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
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
	_, err := execute(t, "build-manifest", "--plain", "--output", out, "--recipients-file", "")
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
	_, err := execute(t, "build-manifest", "--recipient", recipient, "--recipients-file", "", "--output", out)
	require.Nil(t, err)

	data, err := os.ReadFile(out)
	require.Nil(t, err)

	m, err := manifest.Decrypt(data, identity)
	require.Nil(t, err)
	assert.Equal(t, "v0.1.0", m.Packages["wow-look-at-my/atool"].Latest)
}

func TestBuildManifest_RecipientsFromFile(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t,
		[]map[string]any{{"full_name": "wow-look-at-my/atool", "description": ""}},
		map[string][]map[string]any{
			"wow-look-at-my/atool": {{
				"tag_name": "v1.0.0",
				"assets":   []map[string]any{{"name": "atool_linux_amd64", "browser_download_url": "http://a"}},
			}},
		},
	)

	tmp := t.TempDir()
	rA, idA := newTestKeyPair(t)
	rB, idB := newTestKeyPair(t)
	recFile := writeRecipientsFile(t, tmp,
		manifest.Recipient{Name: "alice", Key: rA},
		manifest.Recipient{Name: "bob", Key: rB, Note: "laptop"},
	)

	out := filepath.Join(tmp, "manifest.age")
	_, err := execute(t, "build-manifest", "--recipients-file", recFile, "--output", out)
	require.Nil(t, err)

	data, err := os.ReadFile(out)
	require.Nil(t, err)

	_, err = manifest.Decrypt(data, idA)
	assert.Nil(t, err)
	_, err = manifest.Decrypt(data, idB)
	assert.Nil(t, err)
}

func TestBuildManifest_FileMergedWithFlag(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t,
		[]map[string]any{{"full_name": "wow-look-at-my/atool", "description": ""}},
		map[string][]map[string]any{
			"wow-look-at-my/atool": {{
				"tag_name": "v1.0.0",
				"assets":   []map[string]any{{"name": "atool_linux_amd64", "browser_download_url": "http://a"}},
			}},
		},
	)

	tmp := t.TempDir()
	rFile, idFile := newTestKeyPair(t)
	rFlag, idFlag := newTestKeyPair(t)
	recFile := writeRecipientsFile(t, tmp, manifest.Recipient{Name: "fileuser", Key: rFile})

	out := filepath.Join(tmp, "manifest.age")
	_, err := execute(t, "build-manifest",
		"--recipients-file", recFile,
		"--recipient", rFlag,
		"--output", out,
	)
	require.Nil(t, err)

	data, err := os.ReadFile(out)
	require.Nil(t, err)

	_, err = manifest.Decrypt(data, idFile)
	assert.Nil(t, err)
	_, err = manifest.Decrypt(data, idFlag)
	assert.Nil(t, err)
}

func TestBuildManifest_NoRecipientsErrors(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t, nil, nil)
	_, err := execute(t, "build-manifest", "--recipients-file", "")
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "no recipients")
}

func TestBuildManifest_SkipIfEmpty(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t, nil, nil)

	tmp := t.TempDir()
	out := filepath.Join(tmp, "manifest.age")
	_, err := execute(t, "build-manifest",
		"--recipients-file", "",
		"--output", out,
		"--skip-if-empty",
	)
	require.Nil(t, err)

	// Output file must NOT have been created — the workflow then doesn't
	// publish a manifest at all.
	_, statErr := os.Stat(out)
	assert.True(t, os.IsNotExist(statErr))
}

func TestBuildManifest_MissingFileIsBenign_ButStillNeedsRecipient(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t, nil, nil)

	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	// Missing file alone => no recipients => error.
	_, err := execute(t, "build-manifest", "--recipients-file", missing)
	require.NotNil(t, err)

	// Missing file + --recipient flag => fine.
	resetBuildManifestFlags(t) // reset before re-running
	r, _ := newTestKeyPair(t)
	out := filepath.Join(t.TempDir(), "m.age")
	_, err = execute(t, "build-manifest",
		"--recipients-file", missing,
		"--recipient", r,
		"--output", out,
	)
	require.Nil(t, err)
}

func TestBuildManifest_FileWithBadKeyErrors(t *testing.T) {
	withTempState(t)
	resetBuildManifestFlags(t)
	withMockGitHub(t,
		[]map[string]any{{"full_name": "wow-look-at-my/atool", "description": ""}},
		map[string][]map[string]any{
			"wow-look-at-my/atool": {{
				"tag_name": "v1.0.0",
				"assets":   []map[string]any{{"name": "atool_linux_amd64", "browser_download_url": "http://a"}},
			}},
		},
	)
	tmp := t.TempDir()
	recFile := writeRecipientsFile(t, tmp, manifest.Recipient{Name: "bad", Key: "not-a-real-key"})

	_, err := execute(t, "build-manifest", "--recipients-file", recFile, "--output", filepath.Join(tmp, "m.age"))
	require.NotNil(t, err)
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
	_, err := execute(t, "build-manifest", "--plain", "--recipients-file", "", "--output", out)
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
