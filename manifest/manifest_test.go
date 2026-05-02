package manifest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"filippo.io/age"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func newKeyPair(t *testing.T) (recipient, identity string) {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	require.Nil(t, err)
	return id.Recipient().String(), id.String()
}

func TestNew_Defaults(t *testing.T) {
	m := New()
	assert.Equal(t, SchemaVersion, m.SchemaVersion)
	assert.NotNil(t, m.Packages)
	assert.False(t, m.GeneratedAt.IsZero())
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	recipient, identity := newKeyPair(t)

	m := New()
	m.Packages["owner/tool"] = &Package{
		Slug:   "owner/tool",
		Latest: "v1.0.0",
		Releases: []*Release{
			{
				Tag: "v1.0.0",
				Assets: []*Asset{
					{Name: "tool_linux_amd64", URL: "http://example/tool_linux_amd64", Size: 100},
				},
			},
		},
	}

	ciphertext, err := Encrypt(m, []string{recipient})
	require.Nil(t, err)
	assert.True(t, len(ciphertext) > 0)

	got, err := Decrypt(ciphertext, identity)
	require.Nil(t, err)
	assert.Equal(t, SchemaVersion, got.SchemaVersion)

	pkg, ok := got.Packages["owner/tool"]
	require.True(t, ok)
	assert.Equal(t, "v1.0.0", pkg.Latest)
	assert.Equal(t, 1, len(pkg.Releases))
	assert.Equal(t, "tool_linux_amd64", pkg.Releases[0].Assets[0].Name)
}

func TestEncrypt_MultipleRecipients_AnyIdentityCanDecrypt(t *testing.T) {
	recipientA, identityA := newKeyPair(t)
	recipientB, identityB := newKeyPair(t)
	_, otherIdentity := newKeyPair(t)

	ciphertext, err := Encrypt(New(), []string{recipientA, recipientB})
	require.Nil(t, err)

	// Either rightful holder can decrypt.
	_, err = Decrypt(ciphertext, identityA)
	assert.Nil(t, err)
	_, err = Decrypt(ciphertext, identityB)
	assert.Nil(t, err)

	// Outsider still cannot.
	_, err = Decrypt(ciphertext, otherIdentity)
	assert.NotNil(t, err)
}

func TestEncrypt_NoRecipients(t *testing.T) {
	_, err := Encrypt(New(), nil)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "at least one recipient")
}

func TestDecrypt_WrongKey(t *testing.T) {
	recipient, _ := newKeyPair(t)
	_, otherIdentity := newKeyPair(t)

	ciphertext, err := Encrypt(New(), []string{recipient})
	require.Nil(t, err)

	_, err = Decrypt(ciphertext, otherIdentity)
	assert.NotNil(t, err)
}

func TestEncrypt_BadRecipient(t *testing.T) {
	_, err := Encrypt(New(), []string{"not-a-real-recipient"})
	assert.NotNil(t, err)
}

func TestDecrypt_BadIdentity(t *testing.T) {
	_, err := Decrypt([]byte("garbage"), "not-a-real-identity")
	assert.NotNil(t, err)
}

func TestDecrypt_GarbageCiphertext(t *testing.T) {
	_, identity := newKeyPair(t)
	_, err := Decrypt([]byte("not-an-age-file"), identity)
	assert.NotNil(t, err)
}

func TestFindAsset(t *testing.T) {
	m := New()
	m.Packages["owner/tool"] = &Package{
		Slug:   "owner/tool",
		Latest: "v2.0.0",
		Releases: []*Release{
			{Tag: "v1.0.0", Assets: []*Asset{{Name: "tool_linux_amd64", URL: "old"}}},
			{Tag: "v2.0.0", Assets: []*Asset{{Name: "tool_linux_amd64", URL: "new"}}},
		},
	}

	a := m.FindAsset("owner/tool", "tool_linux_amd64")
	require.NotNil(t, a)
	assert.Equal(t, "new", a.URL)

	a = m.FindAssetForVersion("owner/tool", "v1.0.0", "tool_linux_amd64")
	require.NotNil(t, a)
	assert.Equal(t, "old", a.URL)

	assert.Nil(t, m.FindAsset("owner/missing", "tool_linux_amd64"))
	assert.Nil(t, m.FindAsset("owner/tool", "tool_windows_amd64"))
	assert.Nil(t, m.FindAssetForVersion("owner/tool", "v9.0.0", "tool_linux_amd64"))
}

func TestFetch_Roundtrip(t *testing.T) {
	recipient, identity := newKeyPair(t)
	m := New()
	m.Packages["a/b"] = &Package{Slug: "a/b", Latest: "v1"}

	ciphertext, err := Encrypt(m, []string{recipient})
	require.Nil(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(ciphertext)
	}))
	t.Cleanup(srv.Close)

	got, err := Fetch(context.Background(), srv.URL, identity)
	require.Nil(t, err)
	assert.Equal(t, "v1", got.Packages["a/b"].Latest)
}

func TestFetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, identity := newKeyPair(t)
	_, err := Fetch(context.Background(), srv.URL, identity)
	assert.NotNil(t, err)
}

func TestDownloadAsset_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	t.Cleanup(srv.Close)

	body, _, err := DownloadAsset(context.Background(), srv.URL)
	require.Nil(t, err)
	defer body.Close()

	buf := make([]byte, 5)
	n, _ := body.Read(buf)
	assert.Equal(t, "hello", string(buf[:n]))
}

func TestDownloadAsset_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, _, err := DownloadAsset(context.Background(), srv.URL)
	assert.NotNil(t, err)
}
