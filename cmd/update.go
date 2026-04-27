package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all installed packages to their latest release",
	Args:  cobra.NoArgs,
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, _ []string) error {
	s, err := store.Load()
	if err != nil {
		return err
	}

	targets := s.All()
	if len(targets) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No packages installed.")
		return nil
	}

	for _, pkg := range targets {
		if err := updateOne(cmd, s, pkg); err != nil {
			return err
		}
	}
	return nil
}

func updateOne(cmd *cobra.Command, s *store.Store, pkg *store.Package) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Checking latest release for %s...\n", pkg.Slug)
	rel, err := detectLatest(pkg.Slug)
	if err != nil {
		return err
	}

	if rel.Version.Original == pkg.Version {
		fmt.Fprintf(cmd.OutOrStdout(), "%s is already up to date (%s)\n", pkg.Name, pkg.Version)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updating %s %s -> %s...\n", pkg.Name, pkg.Version, rel.Version.Original)
	if err := installRelease(rel, pkg.Path); err != nil {
		return err
	}

	pkg.Version = rel.Version.Original
	s.Add(pkg)
	if err := s.Save(); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated %s -> %s\n", pkg.Name, pkg.Version)
	return nil
}
