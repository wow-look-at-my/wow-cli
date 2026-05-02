package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
	"github.com/wow-look-at-my/wow-cli/manifest"
	"github.com/wow-look-at-my/wow-cli/store"
)

var installCmd = &cobra.Command{
	Use:   "install <name|owner/repo>",
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
	slug := normalizeSlug(args[0])

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

	// First look in configured repos. A hit avoids the GitHub API entirely.
	hit, err := findInRepos(context.Background(), slug, name, installVersion)
	if err != nil {
		return err
	}
	if hit != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Installing %s %s from repo %s...\n", name, hit.Tag, hit.Repo.URL)
		if err := installFromAsset(cmd, hit.Asset, dest); err != nil {
			return err
		}
		return recordInstall(cmd, slug, name, dest, hit.Tag)
	}

	return installFromGitHub(cmd, args[0], slug, name, dest)
}

// installFromGitHub is the original GitHub-API code path for when no repo
// matches.
func installFromGitHub(cmd *cobra.Command, input, slug, name, dest string) error {
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
			slug, rel, err = suggestAndConfirm(cmd, input, slug, func(s string) (*selfupdate.Release, error) {
				r, _, e := up.DetectVersion(context.Background(), selfupdate.ParseSlug(s), installVersion)
				return r, e
			})
			if err != nil {
				return err
			}
			if rel == nil {
				return fmt.Errorf("version %s not found for %s", installVersion, slug)
			}
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Fetching latest release for %s...\n", slug)
		rel, err = detectLatest(slug)
		if err != nil {
			slug, rel, err = suggestAndConfirm(cmd, input, slug, detectLatest)
			if err != nil {
				return err
			}
			if rel == nil {
				return fmt.Errorf("no release found for %s", slug)
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Downloading %s %s...\n", name, rel.Version.Version)
	if err := installRelease(rel, dest); err != nil {
		return err
	}
	return recordInstall(cmd, slug, name, dest, rel.Version.Original)
}

// installFromAsset downloads an asset URL named in a manifest and writes it
// to dest atomically. Creates the parent directory if needed.
func installFromAsset(cmd *cobra.Command, asset *manifest.Asset, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}
	body, _, err := manifest.DownloadAsset(context.Background(), asset.URL)
	if err != nil {
		return err
	}
	defer body.Close()
	return installAtomic(body, dest)
}

// recordInstall writes the package entry to the store and prints a summary.
func recordInstall(cmd *cobra.Command, slug, name, dest, version string) error {
	s, err := store.Load()
	if err != nil {
		return err
	}
	s.Add(&store.Package{
		Slug:    slug,
		Name:    name,
		Path:    dest,
		Version: version,
	})
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Installed %s %s -> %s\n", name, version, dest)
	return nil
}

// suggestAndConfirm searches for a near match when the primary slug has no release,
// then asks the user whether to proceed with it. Only applies to bare names (no "/").
// Returns the slug, release, and error to use going forward.
func suggestAndConfirm(cmd *cobra.Command, input, failedSlug string, detect func(string) (*selfupdate.Release, error)) (slug string, rel *selfupdate.Release, err error) {
	if strings.Contains(input, "/") {
		return failedSlug, nil, nil
	}

	match, searchErr := findBestOrgMatch(context.Background(), input)
	if searchErr != nil || match == "" || match == failedSlug {
		return failedSlug, nil, nil
	}

	// See if the suggested repo has a release before bothering the user.
	candidate, candErr := detect(match)
	if candErr != nil || candidate == nil {
		return failedSlug, nil, nil
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "No release found for %s.\nDid you mean %s? [y/N] ", failedSlug, match)
	var answer string
	fmt.Fscan(cmd.InOrStdin(), &answer)
	if !strings.EqualFold(strings.TrimSpace(answer), "y") {
		return failedSlug, nil, nil
	}

	return match, candidate, nil
}
