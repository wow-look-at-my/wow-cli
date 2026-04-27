package cmd

import (
	"runtime/debug"
	"strconv"
	"time"

	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
)

func init() {
	populateBuildVersion()
	rootCmd.Version = buildVersion
	rootCmd.AddCommand(selfupdate.NewVersionCommand(buildVersion, selfupdate.ParseSlug("wow-look-at-my/wow-cli")))
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
