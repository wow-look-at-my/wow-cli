package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var searchRepo string

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search configured repos for installable packages",
	Long: `List packages available from configured manifest sources.

Without arguments, lists all packages from all configured repos.
With a query, filters by slug or description (case-insensitive substring).
Use --repo to restrict results to a single configured repo URL.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchRepo, "repo", "", "restrict to a specific repo URL")
	rootCmd.AddCommand(searchCmd)
}

type ghSearchResponse struct {
	Items []struct {
		FullName    string `json:"full_name"`
		Description string `json:"description"`
	} `json:"items"`
}

// ghSearchBaseURL is the GitHub API base; overridden in tests.
var ghSearchBaseURL = "https://api.github.com"

func runSearch(cmd *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 && args[0] != "*" {
		query = strings.ToLower(args[0])
	}

	ctx := context.Background()
	cache := newRepoCache()
	if err := cache.ensureLoaded(ctx); err != nil {
		return err
	}
	if len(cache.repos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No repos configured. Use 'wow repo add' to add a manifest source.")
		return nil
	}

	type entry struct {
		slug string
		desc string
	}
	seen := make(map[string]bool)
	var entries []entry

	for _, r := range cache.repos {
		if searchRepo != "" && r.URL != searchRepo {
			continue
		}
		m, err := cache.fetch(ctx, r)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", r.URL, err)
			continue
		}
		for slug, pkg := range m.Packages {
			if seen[slug] {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(slug), query) && !strings.Contains(strings.ToLower(pkg.Description), query) {
				continue
			}
			seen[slug] = true
			entries = append(entries, entry{slug: slug, desc: pkg.Description})
		}
	}

	if len(entries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No results found.")
		return nil
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].slug < entries[j].slug })

	for _, e := range entries {
		if e.desc != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n", e.slug, e.desc)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), e.slug)
		}
	}
	return nil
}

// ghSearch queries the GitHub repository search API for the given query in the wow-look-at-my org.
// Used by install's "did you mean" suggestion flow.
func ghSearch(ctx context.Context, query string) (*ghSearchResponse, error) {
	combined := "org:wow-look-at-my"
	if query != "" {
		combined = query + " " + combined
	}
	q := url.QueryEscape(combined)
	apiURL := ghSearchBaseURL + "/search/repositories?q=" + q

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
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

	var result ghSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}
	return &result, nil
}

// findBestOrgMatch searches the org for the closest repo matching name and returns
// its full slug (e.g. "wow-look-at-my/ccze-go"), or "" if nothing useful is found.
// First checks configured manifest sources, then falls back to GitHub search.
func findBestOrgMatch(ctx context.Context, name string) (string, error) {
	cache := newRepoCache()
	if err := cache.ensureLoaded(ctx); err == nil && len(cache.repos) > 0 {
		lower := strings.ToLower(name)
		for _, r := range cache.repos {
			m, err := cache.fetch(ctx, r)
			if err != nil {
				continue
			}
			for slug := range m.Packages {
				parts := strings.SplitN(slug, "/", 2)
				if len(parts) == 2 && strings.Contains(strings.ToLower(parts[1]), lower) {
					return slug, nil
				}
			}
		}
	}

	result, err := ghSearch(ctx, name)
	if err != nil || len(result.Items) == 0 {
		return "", err
	}
	return result.Items[0].FullName, nil
}
