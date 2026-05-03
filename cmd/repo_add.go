package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/manifest"
	"github.com/wow-look-at-my/wow-cli/store"
)

var addSrcCmd = &cobra.Command{
	Use:   "add <url> <key>",
	Short: "Add an encrypted manifest repo",
	Long: `Add a remote encrypted-manifest repo so wow can install packages
without calling the GitHub API.

  url   the URL of the age-encrypted manifest (typically a GitHub Pages link)
  key   the age identity (private key) used to decrypt it
        (e.g. "AGE-SECRET-KEY-...")

The key is stored in ~/.local/share/wow/repos.json with 0600 permissions
since it grants read access to the manifest contents.`,
	Args: cobra.ExactArgs(2),
	RunE: runAddSrc,
}

func init() {
	repoCmd.AddCommand(addSrcCmd)
}

func runAddSrc(cmd *cobra.Command, args []string) error {
	url := strings.TrimSpace(args[0])
	identity := strings.TrimSpace(args[1])

	// Verify the repo is reachable and the key decrypts it before saving;
	// otherwise the user finds out later when install fails mysteriously.
	m, err := manifest.Fetch(context.Background(), url, identity)
	if err != nil {
		return fmt.Errorf("verify repo: %w", err)
	}

	s, err := store.LoadRepoList()
	if err != nil {
		return err
	}
	s.Add(&store.Repo{URL: url, Identity: identity, AddedAt: time.Now().UTC()})
	if err := s.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Added repo %s (%d packages)\n", url, len(m.Packages))
	return nil
}
