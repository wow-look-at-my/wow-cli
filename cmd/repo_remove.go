package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wow-look-at-my/wow-cli/store"
)

var removeSrcCmd = &cobra.Command{
	Use:   "remove <url>",
	Short: "Remove a configured manifest repo",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemoveSrc,
}

func init() {
	repoCmd.AddCommand(removeSrcCmd)
}

func runRemoveSrc(cmd *cobra.Command, args []string) error {
	url := args[0]
	s, err := store.LoadRepoList()
	if err != nil {
		return err
	}
	removed := s.Remove(url)
	if removed == nil {
		return fmt.Errorf("not configured: %s", url)
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Removed repo %s\n", url)
	return nil
}
