package cmd

import (
	"github.com/spf13/cobra"
	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
)

func init() {
	selfupdate.RegisterCommands(rootCmd, selfupdate.ParseSlug("wow-look-at-my/wow-cli"))
	// RegisterCommands also registers single-binary install/update; we already
	// have package-aware versions of those, so drop the library's duplicates.
	var dupes []*cobra.Command
	for _, c := range rootCmd.Commands() {
		switch c.Short {
		case "Install the binary from a GitHub release", "Update the binary to the latest version":
			dupes = append(dupes, c)
		}
	}
	rootCmd.RemoveCommand(dupes...)
}
