package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packages",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := store.Load()
		if err != nil {
			return err
		}
		pkgs := s.All()
		if len(pkgs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No packages installed.")
			return nil
		}
		sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Slug < pkgs[j].Slug })
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-20s %-20s %s\n", "PACKAGE", "NAME", "VERSION", "PATH")
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-20s %-20s %s\n", "-------", "----", "-------", "----")
		for _, pkg := range pkgs {
			fmt.Fprintf(cmd.OutOrStdout(), "%-40s %-20s %-20s %s\n",
				pkg.Slug, pkg.Name, pkg.Version, pkg.Path)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
