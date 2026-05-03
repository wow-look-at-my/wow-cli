package cmd

import selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"

func init() {
	selfupdate.RegisterCommands(rootCmd, selfupdate.ParseSlug("wow-look-at-my/wow-cli"))
	// RegisterCommands also registers a single-binary install command; we
	// already have a package-aware version of install, so drop that one.
	// The library's update and version commands stay.
	for _, c := range rootCmd.Commands() {
		if c.Short == "Install the binary from a GitHub release" {
			rootCmd.RemoveCommand(c)
			break
		}
	}
}
