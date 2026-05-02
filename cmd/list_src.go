package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var listSrcCmd = &cobra.Command{
	Use:   "list-src",
	Short: "List configured manifest sources",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := store.LoadSources()
		if err != nil {
			return err
		}
		all := s.All()
		if len(all) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No sources configured.")
			return nil
		}
		for _, src := range all {
			fmt.Fprintln(cmd.OutOrStdout(), src.String())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(listSrcCmd)
}
