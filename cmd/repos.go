package cmd

import (
	"context"
	"runtime"

	"github.com/wow-look-at-my/wow-cli/manifest"
	"github.com/wow-look-at-my/wow-cli/store"
)

// repoHit ties an asset back to the repo that supplied it.
type repoHit struct {
	Repo    *store.Repo
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

// findInRepos walks configured repos, fetches and decrypts each manifest,
// and returns the first matching asset. tag may be empty to select the
// manifest's "latest" release. Returns (nil, nil) if no repo matches; an
// error only on transport/decrypt failure.
//
// Note: this fetches each repo on every call. Callers that hit it in a loop
// (e.g. update) should cache the results — see repoCache.
func findInRepos(ctx context.Context, slug, binary, tag string) (*repoHit, error) {
	c := newRepoCache()
	return c.find(ctx, slug, binary, tag)
}

// repoCache memoizes manifest fetches across multiple lookups. Build via
// newRepoCache and call find for each package.
type repoCache struct {
	repos  []*store.Repo
	loaded map[string]*manifest.Manifest // url -> manifest (nil if fetch failed)
	errs   map[string]error
}

func newRepoCache() *repoCache {
	return &repoCache{
		loaded: make(map[string]*manifest.Manifest),
		errs:   make(map[string]error),
	}
}

func (c *repoCache) ensureLoaded(ctx context.Context) error {
	if c.repos != nil {
		return nil
	}
	s, err := store.LoadRepoList()
	if err != nil {
		return err
	}
	c.repos = s.All()
	return nil
}

func (c *repoCache) fetch(ctx context.Context, r *store.Repo) (*manifest.Manifest, error) {
	if m, ok := c.loaded[r.URL]; ok {
		return m, c.errs[r.URL]
	}
	m, err := manifest.Fetch(ctx, r.URL, r.Identity)
	c.loaded[r.URL] = m
	c.errs[r.URL] = err
	return m, err
}

// find returns the first repo/asset matching slug + platform binary at the
// requested tag (or latest if tag is empty). Repos that error or that lack
// a matching platform asset are skipped silently — the caller can fall back
// to GitHub if this returns nil.
func (c *repoCache) find(ctx context.Context, slug, binary, tag string) (*repoHit, error) {
	if err := c.ensureLoaded(ctx); err != nil {
		return nil, err
	}
	if len(c.repos) == 0 {
		return nil, nil
	}
	assetName := platformAssetName(binary)
	for _, r := range c.repos {
		m, err := c.fetch(ctx, r)
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
		return &repoHit{Repo: r, Package: pkg, Tag: wantTag, Asset: a}, nil
	}
	return nil, nil
}
