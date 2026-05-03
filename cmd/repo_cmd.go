package cmd

import "github.com/spf13/cobra"

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage encrypted manifest repos",
}

func init() {
	rootCmd.AddCommand(repoCmd)
}
