package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search wow-look-at-my org for installable packages",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSearch,
}

func init() {
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
		query = args[0]
	}
	result, err := ghSearch(context.Background(), query)
	if err != nil {
		return err
	}

	if len(result.Items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No results found.")
		return nil
	}

	for _, item := range result.Items {
		if item.Description != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %s\n", item.FullName, item.Description)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), item.FullName)
		}
	}
	return nil
}

// ghSearch queries the GitHub repository search API for the given query in the wow-look-at-my org.
// An empty query lists all repos in the org.
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
func findBestOrgMatch(ctx context.Context, name string) (string, error) {
	result, err := ghSearch(ctx, name)
	if err != nil || len(result.Items) == 0 {
		return "", err
	}
	return result.Items[0].FullName, nil
}
