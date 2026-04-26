package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var whichCmd = &cobra.Command{
	Use:   "which <name|owner/repo>",
	Short: "Print the install path of a package",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		s, err := store.Load()
		if err != nil {
			return err
		}
		pkg := s.Find(args[0])
		if pkg == nil {
			return fmt.Errorf("not installed: %s", args[0])
		}
		fmt.Fprintln(cmd.OutOrStdout(), pkg.Path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whichCmd)
}
