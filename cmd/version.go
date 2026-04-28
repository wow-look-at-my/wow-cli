package cmd

import (
	"runtime/debug"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
)

func init() {
	selfupdate.EmbeddedVersion = autoreleaseVersion()
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

// autoreleaseVersion returns the binary's version in the format produced by
// the go-toolchain autorelease workflow: "v0.0.<unix-seconds>" derived from
// vcs.time, with "+dirty" appended for modified working trees. Returns "" when
// no VCS info is available, which lets selfupdate.CurrentVersion fall back to
// its built-in detection (short revision / "(devel)").
//
// Without this, selfupdate.CurrentVersion would return a 12-char SHA that
// never compares equal to autorelease tags (v0.0.<unix-seconds>), so wow's
// self-update equality check would always fire.
func autoreleaseVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return autoreleaseVersionFromBuildInfo(info)
}

func autoreleaseVersionFromBuildInfo(info *debug.BuildInfo) string {
	var vcsTime, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if vcsTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, vcsTime)
	if err != nil {
		return ""
	}
	v := "v0.0." + strconv.FormatInt(t.Unix(), 10)
	if modified == "true" {
		v += "+dirty"
	}
	return v
}
