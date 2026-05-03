package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/manifest"
)

var (
	buildManifestOrg            string
	buildManifestInclude        []string
	buildManifestRecipients     []string
	buildManifestRecipientsFile string
	buildManifestOutput         string
	buildManifestUnencrypt      bool
	buildManifestSkipIfEmpty    bool
)

var buildManifestCmd = &cobra.Command{
	Use:   "build",
	Short: "Build (and encrypt) a manifest of installable packages",
	Long: `Walk the configured org's repos, gather their releases, and write
an age-encrypted manifest.

Recipients (age X25519 public keys) come from --recipients-file (default:
recipients.json in the working directory) and from any --recipient flags.
Both sources are merged. The recipients file is the audit log for who can
read the manifest; revoke a user by removing their entry and republishing.

Use --plain to skip encryption (for debugging or generating templates); the
output is then plain JSON.`,
	Args: cobra.NoArgs,
	RunE: runBuildManifest,
}

func init() {
	buildManifestCmd.Flags().StringVar(&buildManifestOrg, "org", "wow-look-at-my", "GitHub org to enumerate")
	buildManifestCmd.Flags().StringArrayVar(&buildManifestInclude, "include", nil, "glob pattern for repo names to include (repeatable; if omitted, all repos)")
	buildManifestCmd.Flags().StringArrayVar(&buildManifestRecipients, "recipient", nil, "age recipient public key (repeatable, merged with --recipients-file)")
	buildManifestCmd.Flags().StringVar(&buildManifestRecipientsFile, "recipients-file", "recipients.jsonc", "path to recipients JSONC file (skip with empty string)")
	buildManifestCmd.Flags().StringVar(&buildManifestOutput, "output", "-", "output file (\"-\" for stdout)")
	buildManifestCmd.Flags().BoolVar(&buildManifestUnencrypt, "plain", false, "write plain JSON instead of encrypting")
	buildManifestCmd.Flags().BoolVar(&buildManifestSkipIfEmpty, "skip-if-empty", false, "exit 0 without writing output when no recipients are configured")
	repoCmd.AddCommand(buildManifestCmd)
}

func runBuildManifest(cmd *cobra.Command, _ []string) error {
	recipients := append([]string{}, buildManifestRecipients...)
	if buildManifestRecipientsFile != "" {
		fileRecipients, err := manifest.LoadRecipients(buildManifestRecipientsFile)
		if err != nil {
			return err
		}
		recipients = append(recipients, manifest.Keys(fileRecipients)...)
	}
	if len(recipients) == 0 && !buildManifestUnencrypt {
		if buildManifestSkipIfEmpty {
			fmt.Fprintln(cmd.ErrOrStderr(), "no recipients configured; skipping (--skip-if-empty)")
			return nil
		}
		return fmt.Errorf("no recipients (populate %s or pass --recipient, or use --plain)", buildManifestRecipientsFile)
	}

	ctx := context.Background()
	repos, err := enumerateOrgRepos(ctx, buildManifestOrg)
	if err != nil {
		return fmt.Errorf("enumerate org: %w", err)
	}
	if len(buildManifestInclude) > 0 {
		repos = filterRepos(repos, buildManifestInclude)
	}

	m := manifest.New()
	for _, repo := range repos {
		pkg, err := buildPackage(ctx, repo)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", repo.FullName, err)
			continue
		}
		if pkg == nil {
			continue
		}
		pkg.Description = repo.Description
		m.Packages[pkg.Slug] = pkg
	}

	var output []byte
	if buildManifestUnencrypt {
		output, err = json.MarshalIndent(m, "", "  ")
		if err != nil {
			return err
		}
	} else {
		output, err = manifest.Encrypt(m, recipients)
		if err != nil {
			return err
		}
	}

	if buildManifestOutput == "-" {
		_, err = cmd.OutOrStdout().Write(output)
		return err
	}
	return os.WriteFile(buildManifestOutput, output, 0o644)
}

// enumerateOrgRepos returns the search results for org:<org>, reusing the
// search command's underlying API client.
func enumerateOrgRepos(ctx context.Context, org string) ([]ghSearchItem, error) {
	combined := "org:" + org
	url := ghSearchBaseURL + "/search/repositories?per_page=100&q=" + urlQueryEscape(combined)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && os.Getenv("GH_HOST") == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var result struct {
		Items []ghSearchItem `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode org listing: %w", err)
	}
	return result.Items, nil
}

// buildPackage queries the releases API for repo and constructs a manifest
// Package entry. Returns nil if the repo has no releases.
func buildPackage(ctx context.Context, repo ghSearchItem) (*manifest.Package, error) {
	releases, err := fetchReleases(ctx, repo.FullName)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, nil
	}

	pkg := &manifest.Package{Slug: repo.FullName, Latest: releases[0].TagName}
	for _, rel := range releases {
		mr := &manifest.Release{Tag: rel.TagName}
		for _, a := range rel.Assets {
			mr.Assets = append(mr.Assets, &manifest.Asset{
				Name: a.Name,
				URL:  a.DownloadURL,
				Size: a.Size,
			})
		}
		pkg.Releases = append(pkg.Releases, mr)
	}
	return pkg, nil
}

// ghReleasesBaseURL is the GitHub releases API base; overridden in tests.
var ghReleasesBaseURL = "https://api.github.com"

type ghReleaseAsset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName    string           `json:"tag_name"`
	Draft      bool             `json:"draft"`
	Prerelease bool             `json:"prerelease"`
	Assets     []ghReleaseAsset `json:"assets"`
}

// fetchReleases returns non-draft, non-prerelease releases for the slug,
// newest first as returned by the API.
func fetchReleases(ctx context.Context, slug string) ([]ghRelease, error) {
	url := ghReleasesBaseURL + "/repos/" + slug + "/releases?per_page=100"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && os.Getenv("GH_HOST") == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}
	out := releases[:0]
	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// ghSearchItem is a slimmed copy of GitHub's repo search hit. Defined here
// (separately from search.go's anonymous struct) so repo build can pass
// items between functions without depending on search.go's internals.
type ghSearchItem struct {
	FullName    string `json:"full_name"`
	Description string `json:"description"`
}

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

func filterRepos(repos []ghSearchItem, patterns []string) []ghSearchItem {
	var out []ghSearchItem
	for _, r := range repos {
		name := r.FullName
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		for _, p := range patterns {
			if matched, _ := filepath.Match(p, name); matched {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
