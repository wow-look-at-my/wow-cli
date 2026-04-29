package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
	"github.com/wow-look-at-my/wow-cli/store"
)

// wowExePathOverride overrides the executable path for wow self-update.
// Set only from tests.
var wowExePathOverride string

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
	} else {
		for _, pkg := range targets {
			if err := updateOne(cmd, s, pkg); err != nil {
				return err
			}
		}
	}

	return selfUpdateWow(cmd)
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

// selfUpdateWow upgrades the wow binary itself in-place. Dev builds (empty
// version, "(devel)", or "+dirty" suffix) are skipped: the autorelease tag
// for them is either unknowable or doesn't match a real release.
func selfUpdateWow(cmd *cobra.Command) error {
	const slug = "wow-look-at-my/wow-cli"
	current := selfupdate.CurrentVersion()
	if current == "" || current == "(devel)" || strings.Contains(current, "+dirty") {
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Checking latest release for %s...\n", slug)

	exePath := wowExePathOverride
	if exePath == "" {
		var err error
		exePath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
		exePath, err = filepath.EvalSymlinks(exePath)
		if err != nil {
			return fmt.Errorf("resolve executable symlinks: %w", err)
		}
	}

	up, err := newUpdater()
	if err != nil {
		return err
	}

	rel, err := up.UpdateCommand(context.Background(), exePath, current, selfupdate.ParseSlug(slug))
	if err != nil {
		return err
	}

	if rel.Version.Original == current || rel.Version.Original == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "wow is already up to date (%s)\n", current)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Updated wow %s -> %s\n", current, rel.Version.Original)
	}
	return nil
}
