package cmd

import selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"

func init() {
	selfupdate.RegisterCommands(rootCmd, selfupdate.ParseSlug("wow-look-at-my/wow-cli"))
}
