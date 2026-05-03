package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade all installed packages to their latest release",
	Args:  cobra.NoArgs,
	RunE:  runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	s, err := store.Load()
	if err != nil {
		return err
	}

	targets := s.All()
	if len(targets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No packages installed.")
		return nil
	}

	cache := newRepoCache()
	for _, pkg := range targets {
		if err := upgradeOne(cmd, s, pkg, cache); err != nil {
			return err
		}
	}
	return nil
}

func upgradeOne(cmd *cobra.Command, s *store.Store, pkg *store.Package, cache *repoCache) error {
	hit, err := cache.find(context.Background(), pkg.Slug, pkg.Name, "")
	if err != nil {
		return err
	}
	if hit != nil {
		if hit.Tag == pkg.Version {
			fmt.Fprintf(cmd.OutOrStdout(), "%s is already up to date (%s)\n", pkg.Name, pkg.Version)
			return nil
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Upgrading %s %s -> %s from repo %s...\n", pkg.Name, pkg.Version, hit.Tag, hit.Repo.URL)
		if err := installFromAsset(cmd, hit.Asset, pkg.Path); err != nil {
			return err
		}
		pkg.Version = hit.Tag
		s.Add(pkg)
		if err := s.Save(); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Upgraded %s -> %s\n", pkg.Name, pkg.Version)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Checking latest release for %s...\n", pkg.Slug)
	rel, err := detectLatest(pkg.Slug)
	if err != nil {
		return err
	}

	if rel.Version.Original == pkg.Version {
		fmt.Fprintf(cmd.OutOrStdout(), "%s is already up to date (%s)\n", pkg.Name, pkg.Version)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Upgrading %s %s -> %s...\n", pkg.Name, pkg.Version, rel.Version.Original)
	if err := installRelease(rel, pkg.Path); err != nil {
		return err
	}

	pkg.Version = rel.Version.Original
	s.Add(pkg)
	if err := s.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Upgraded %s -> %s\n", pkg.Name, pkg.Version)
	return nil
}
