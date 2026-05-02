package cmd

import (
	"context"
	"runtime"

	"github.com/wow-look-at-my/wow-cli/manifest"
	"github.com/wow-look-at-my/wow-cli/store"
)

// sourceHit ties an asset back to the source that supplied it.
type sourceHit struct {
	Source  *store.Source
	Package *manifest.Package
	Tag     string
	Asset   *manifest.Asset
}

// platformAssetName builds the wow asset filename for the current platform.
// Mirrors the go-toolchain autorelease naming convention.
func platformAssetName(binary string) string {
	suffix := "_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		suffix += ".exe"
	}
	return binary + suffix
}

// findInSources walks configured sources, fetches and decrypts each manifest,
// and returns the first matching asset. tag may be empty to select the
// manifest's "latest" release. Returns (nil, nil) if no source matches; an
// error only on transport/decrypt failure.
//
// Note: this fetches each source on every call. Callers that hit it in a loop
// (e.g. update) should cache the results — see cachedSources.
func findInSources(ctx context.Context, slug, binary, tag string) (*sourceHit, error) {
	c := newSourceCache()
	return c.find(ctx, slug, binary, tag)
}

// sourceCache memoizes manifest fetches across multiple lookups. Build via
// newSourceCache and call find for each package.
type sourceCache struct {
	sources []*store.Source
	loaded  map[string]*manifest.Manifest // url -> manifest (nil if fetch failed)
	errs    map[string]error
}

func newSourceCache() *sourceCache {
	return &sourceCache{
		loaded: make(map[string]*manifest.Manifest),
		errs:   make(map[string]error),
	}
}

func (c *sourceCache) ensureLoaded(ctx context.Context) error {
	if c.sources != nil {
		return nil
	}
	s, err := store.LoadSources()
	if err != nil {
		return err
	}
	c.sources = s.All()
	return nil
}

func (c *sourceCache) fetch(ctx context.Context, src *store.Source) (*manifest.Manifest, error) {
	if m, ok := c.loaded[src.URL]; ok {
		return m, c.errs[src.URL]
	}
	m, err := manifest.Fetch(ctx, src.URL, src.Identity)
	c.loaded[src.URL] = m
	c.errs[src.URL] = err
	return m, err
}

// find returns the first source/asset matching slug + platform binary at the
// requested tag (or latest if tag is empty). Sources that error or that lack
// a matching platform asset are skipped silently — the caller can fall back
// to GitHub if this returns nil.
func (c *sourceCache) find(ctx context.Context, slug, binary, tag string) (*sourceHit, error) {
	if err := c.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	if len(c.sources) == 0 {
		return nil, nil
	}
	assetName := platformAssetName(binary)
	for _, src := range c.sources {
		m, err := c.fetch(ctx, src)
		if err != nil {
			continue
		}
		pkg, ok := m.Packages[slug]
		if !ok {
			continue
		}
		wantTag := tag
		if wantTag == "" {
			wantTag = pkg.Latest
		}
		var a *manifest.Asset
		if tag == "" {
			a = m.FindAsset(slug, assetName)
		} else {
			a = m.FindAssetForVersion(slug, wantTag, assetName)
		}
		if a == nil {
			continue
		}
		return &sourceHit{Source: src, Package: pkg, Tag: wantTag, Asset: a}, nil
	}
	return nil, nil
}
