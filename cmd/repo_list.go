package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var listSrcCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured manifest repos",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		s, err := store.LoadRepoList()
		if err != nil {
			return err
		}
		all := s.All()
		if len(all) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No repos configured.")
			return nil
		}
		for _, r := range all {
			fmt.Fprintln(cmd.OutOrStdout(), r.String())
		}
		return nil
	},
}

func init() {
	repoCmd.AddCommand(listSrcCmd)
}
