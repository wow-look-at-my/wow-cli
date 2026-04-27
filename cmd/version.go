package cmd

import (
	"runtime/debug"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
)

func init() {
	populateBuildVersion()
	selfupdate.RegisterCommands(rootCmd, buildVersion, selfupdate.ParseSlug("wow-look-at-my/wow-cli"))
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

// populateBuildVersion sets buildVersion from the binary's embedded VCS
// info if it isn't already set. The autorelease tag format is
// "v0.0.<unix-seconds>" derived from vcs.time. Dirty or non-VCS builds
// leave buildVersion empty.
func populateBuildVersion() {
	if buildVersion != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	buildVersion = versionFromBuildInfo(info)
}

func versionFromBuildInfo(info *debug.BuildInfo) string {
	var vcsTime, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if modified == "true" || vcsTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, vcsTime)
	if err != nil {
		return ""
	}
	return "v0.0." + strconv.FormatInt(t.Unix(), 10)
}
