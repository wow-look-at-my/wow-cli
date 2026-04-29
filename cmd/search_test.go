package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

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

func TestSearch_NoArgs_ListsAll(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"full_name":"wow-look-at-my/wow-cli","description":"package manager"}]}`)
	}))
	t.Cleanup(func() {
		srv.Close()
		ghSearchBaseURL = "https://api.github.com"
	})
	ghSearchBaseURL = srv.URL

	out, err := execute(t, "search")
	require.Nil(t, err)

	assert.Contains(t, gotQuery, "org%3Awow-look-at-my")
	assert.NotContains(t, gotQuery, "%2A") // no wildcard character
	assert.Contains(t, out, "wow-look-at-my/wow-cli")
}

func TestSearch_Wildcard_ListsAll(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"full_name":"wow-look-at-my/wow-cli","description":"package manager"}]}`)
	}))
	t.Cleanup(func() {
		srv.Close()
		ghSearchBaseURL = "https://api.github.com"
	})
	ghSearchBaseURL = srv.URL

	out, err := execute(t, "search", "*")
	require.Nil(t, err)

	assert.Contains(t, gotQuery, "org%3Awow-look-at-my")
	assert.NotContains(t, gotQuery, "%2A") // * is treated as list-all, not passed through
	assert.Contains(t, out, "wow-look-at-my/wow-cli")
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
