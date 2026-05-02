package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func TestLoadRecipients_MissingFileReturnsNil(t *testing.T) {
	rs, err := LoadRecipients(filepath.Join(t.TempDir(), "nope.json"))
	require.Nil(t, err)
	assert.Nil(t, rs)
}

func TestLoadRecipients_ObjectForm(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "recipients.json", `{
		"recipients": [
			{"name": "alice", "key": "age1aaaa"},
			{"name": "bob", "key": "age1bbbb", "note": "laptop"}
		]
	}`)
	rs, err := LoadRecipients(path)
	require.Nil(t, err)
	require.Equal(t, 2, len(rs))
	assert.Equal(t, "alice", rs[0].Name)
	assert.Equal(t, "age1aaaa", rs[0].Key)
	assert.Equal(t, "laptop", rs[1].Note)
	assert.Equal(t, []string{"age1aaaa", "age1bbbb"}, Keys(rs))
}

func TestLoadRecipients_ArrayOfObjects(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "recipients.json", `[
		{"name": "alice", "key": "age1aaaa"},
		{"key": "age1bbbb"}
	]`)
	rs, err := LoadRecipients(path)
	require.Nil(t, err)
	require.Equal(t, 2, len(rs))
	assert.Equal(t, "alice", rs[0].Name)
	assert.Equal(t, "", rs[1].Name)
}

func TestLoadRecipients_ArrayOfStrings(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "recipients.json", `["age1aaaa","age1bbbb"]`)
	rs, err := LoadRecipients(path)
	require.Nil(t, err)
	require.Equal(t, 2, len(rs))
	assert.Equal(t, "age1aaaa", rs[0].Key)
	assert.Equal(t, "", rs[0].Name)
}

func TestLoadRecipients_EmptyKeyRejected(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "recipients.json", `[{"name":"alice","key":""}]`)
	_, err := LoadRecipients(path)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "empty key")
}

func TestLoadRecipients_BadJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "recipients.json", `not json`)
	_, err := LoadRecipients(path)
	assert.NotNil(t, err)
}

func TestLoadRecipients_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "recipients.json", `   `)
	rs, err := LoadRecipients(path)
	require.Nil(t, err)
	assert.Nil(t, rs)
}

func TestLoadRecipients_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "recipients.json", `[{"key":"  age1aaaa  ","name":"  alice  "}]`)
	rs, err := LoadRecipients(path)
	require.Nil(t, err)
	require.Equal(t, 1, len(rs))
	assert.Equal(t, "age1aaaa", rs[0].Key)
	assert.Equal(t, "alice", rs[0].Name)
}
