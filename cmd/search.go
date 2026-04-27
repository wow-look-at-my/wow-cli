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
	Use:   "search <query>",
	Short: "Search wow-look-at-my org for installable packages",
	Args:  cobra.ExactArgs(1),
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
	q := url.QueryEscape(args[0] + " org:wow-look-at-my")
	apiURL := ghSearchBaseURL + "/search/repositories?q=" + q

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && os.Getenv("GH_HOST") == "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var result ghSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
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
