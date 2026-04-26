package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
	"github.com/wow-look-at-my/wow-cli/store"
)

var installCmd = &cobra.Command{
	Use:   "install <owner/repo>",
	Short: "Download and install a binary from a GitHub release",
	Args:  cobra.ExactArgs(1),
	RunE:  runInstall,
}

var (
	installName    string
	installPath    string
	installVersion string
)

func init() {
	installCmd.Flags().StringVar(&installName, "name", "", "binary name (default: repo name)")
	installCmd.Flags().StringVar(&installPath, "path", "", "install path (default: ~/.local/bin/<name>)")
	installCmd.Flags().StringVar(&installVersion, "version", "", "release tag to install (default: latest)")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	slug := args[0]

	name := installName
	if name == "" {
		// Default: repo portion of "owner/repo"
		repo := selfupdate.ParseSlug(slug)
		_, repoName, err := repo.GetSlug()
		if err != nil {
			return fmt.Errorf("invalid slug %q: %w", slug, err)
		}
		name = repoName
	}

	dest := installPath
	if dest == "" {
		var err error
		dest, err = defaultInstallPath(name)
		if err != nil {
			return err
		}
	}

	up, err := newUpdater()
	if err != nil {
		return err
	}

	var rel *selfupdate.Release
	if installVersion != "" {
		rel, _, err = up.DetectVersion(context.Background(), selfupdate.ParseSlug(slug), installVersion)
		if err != nil {
			return err
		}
		if rel == nil {
			return fmt.Errorf("version %s not found for %s", installVersion, slug)
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Fetching latest release for %s...\n", slug)
		rel, err = detectLatest(slug)
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s %s...\n", name, rel.Version.Version)
	if err := installRelease(rel, dest); err != nil {
		return err
	}

	s, err := store.Load()
	if err != nil {
		return err
	}
	s.Add(&store.Package{
		Slug:    slug,
		Name:    name,
		Path:    dest,
		Version: rel.Version.Original,
	})
	if err := s.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Installed %s %s -> %s\n", name, rel.Version.Original, dest)
	return nil
}
