package cmd

import (
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/wow-look-at-my/testify/assert"
	"github.com/wow-look-at-my/testify/require"
	"github.com/wow-look-at-my/wow-cli/manifest"
)

func TestKeygen_OutputsParseableKeyPair(t *testing.T) {
	out, err := execute(t, "repo", "keygen")
	require.Nil(t, err)

	var recipient, identity string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "age1"):
			recipient = line
		case strings.HasPrefix(line, "AGE-SECRET-KEY-"):
			identity = line
		}
	}
	require.NotEmpty(t, recipient)
	require.NotEmpty(t, identity)

	// Sanity-check that the printed keys actually pair: encrypt a manifest with
	// the recipient and decrypt with the identity.
	r, err := age.ParseX25519Recipient(recipient)
	require.Nil(t, err)
	id, err := age.ParseX25519Identity(identity)
	require.Nil(t, err)
	assert.Equal(t, r.String(), id.Recipient().String())

	cipher, err := manifest.Encrypt(manifest.New(), []string{recipient})
	require.Nil(t, err)
	_, err = manifest.Decrypt(cipher, identity)
	assert.Nil(t, err)
}
