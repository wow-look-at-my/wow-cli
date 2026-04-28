package cmd

import (
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

	v := selfupdate.CurrentVersion()
	if shouldSelfUpdateWow(v) {
		if err := selfUpdateWow(cmd, v); err != nil {
			return err
		}
	}
	return nil
}

// shouldSelfUpdateWow returns true when the running binary's version is a
// real autorelease tag we can compare against the latest release. Empty
// strings, the "(devel)" fallback, and "+dirty" suffixes all indicate a build
// that should not be silently overwritten.
func shouldSelfUpdateWow(v string) bool {
	if v == "" || v == "(devel)" {
		return false
	}
	if strings.Contains(v, "+dirty") {
		return false
	}
	return true
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

func selfUpdateWow(cmd *cobra.Command, current string) error {
	const slug = "wow-look-at-my/wow-cli"
	fmt.Fprintf(cmd.OutOrStdout(), "Checking latest release for %s...\n", slug)

	rel, err := detectLatest(slug)
	if err != nil {
		return err
	}

	if rel.Version.Original == current {
		fmt.Fprintf(cmd.OutOrStdout(), "wow is already up to date (%s)\n", current)
		return nil
	}

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

	fmt.Fprintf(cmd.OutOrStdout(), "Updating wow %s -> %s...\n", current, rel.Version.Original)
	if err := installRelease(rel, exePath); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated wow -> %s\n", rel.Version.Original)
	return nil
}
