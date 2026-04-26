package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <name|owner/repo>...",
	Short: "Remove installed packages",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	s, err := store.Load()
	if err != nil {
		return err
	}

	changed := false
	for _, arg := range args {
		pkg := s.Remove(arg)
		if pkg == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "not installed: %s\n", arg)
			continue
		}
		if err := os.Remove(pkg.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", pkg.Path, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Uninstalled %s (removed %s)\n", pkg.Name, pkg.Path)
		changed = true
	}

	if changed {
		return s.Save()
	}
	return nil
}
