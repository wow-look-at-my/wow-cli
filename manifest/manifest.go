// Package manifest defines the encrypted package catalog used by wow sources.
//
// A manifest is a JSON document listing packages, releases, and per-platform
// asset URLs. It is encrypted with age (X25519) before being published so that
// only holders of the matching age identity can read it. The publisher (CI)
// holds the recipient public key; clients hold the identity private key, which
// they pass to `wow add-src` and which gets stored in repos.json.
package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"filippo.io/age"
)

// SchemaVersion is the current manifest schema version. Bump this when fields
// are removed or repurposed; additive changes are backwards-compatible and
// don't require a bump.
const SchemaVersion = 1

// Manifest is the decrypted catalog. Each package lists its known releases
// and per-platform assets so clients can install without calling the GitHub
// API.
type Manifest struct {
	SchemaVersion int                 `json:"schema_version"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Packages      map[string]*Package `json:"packages"`
}

// Package is one repo's entry in the manifest. Slug is the canonical
// "owner/repo" key.
type Package struct {
	Slug        string     `json:"slug"`
	Description string     `json:"description,omitempty"`
	Latest      string     `json:"latest"`
	Releases    []*Release `json:"releases"`
}

// Release is a single tagged release.
type Release struct {
	Tag    string   `json:"tag"`
	Assets []*Asset `json:"assets"`
}

// Asset is one downloadable file in a release. Name is the asset filename
// (e.g. "wow-cli_linux_amd64"); URL is the direct download link.
type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Size   int64  `json:"size,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// New creates an empty manifest stamped with the current schema version and
// generation time.
func New() *Manifest {
	return &Manifest{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		Packages:      make(map[string]*Package),
	}
}

// FindAsset returns the asset matching the given name in the latest release of
// the named package, or nil if not found.
func (m *Manifest) FindAsset(slug, assetName string) *Asset {
	pkg, ok := m.Packages[slug]
	if !ok {
		return nil
	}
	return pkg.findAsset(pkg.Latest, assetName)
}

// FindAssetForVersion returns the asset matching the given name in the
// specified release tag, or nil.
func (m *Manifest) FindAssetForVersion(slug, tag, assetName string) *Asset {
	pkg, ok := m.Packages[slug]
	if !ok {
		return nil
	}
	return pkg.findAsset(tag, assetName)
}

func (p *Package) findAsset(tag, assetName string) *Asset {
	for _, rel := range p.Releases {
		if rel.Tag != tag {
			continue
		}
		for _, a := range rel.Assets {
			if a.Name == assetName {
				return a
			}
		}
	}
	return nil
}

// Encrypt serializes m to JSON and encrypts it for one or more age
// recipients. Any of the matching identities can decrypt the result, which
// is what enables per-user keys: generate one keypair per user, encrypt to
// all their recipients here, and revoke a user later by dropping their
// recipient from the list.
//
// Recipients are typically X25519 public keys (e.g. "age1...").
func Encrypt(m *Manifest, recipients []string) ([]byte, error) {
	if len(recipients) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}
	parsed := make([]age.Recipient, 0, len(recipients))
	for i, r := range recipients {
		pr, err := age.ParseX25519Recipient(r)
		if err != nil {
			return nil, fmt.Errorf("parse recipient #%d: %w", i+1, err)
		}
		parsed = append(parsed, pr)
	}
	plaintext, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, parsed...)
	if err != nil {
		return nil, fmt.Errorf("init encrypt: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("write plaintext: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("finalize encryption: %w", err)
	}
	return buf.Bytes(), nil
}

// Decrypt reads an age-encrypted manifest using the given identity (X25519
// private key, e.g. "AGE-SECRET-KEY-...") and parses it.
func Decrypt(ciphertext []byte, identity string) (*Manifest, error) {
	id, err := age.ParseX25519Identity(identity)
	if err != nil {
		return nil, fmt.Errorf("parse identity: %w", err)
	}
	r, err := age.Decrypt(bytes.NewReader(ciphertext), id)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	plaintext, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read plaintext: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(plaintext, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}
