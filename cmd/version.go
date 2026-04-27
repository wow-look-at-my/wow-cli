package cmd

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the wow build version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		v := buildVersion
		if v == "" {
			v = "(dev)"
		}
		fmt.Fprintln(cmd.OutOrStdout(), v)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// populateBuildVersion sets buildVersion from the binary's embedded VCS
// info if it isn't already set. The autorelease tag format is
// "v0.0.<unix-seconds>" derived from the commit timestamp, which matches
// vcs.time. Dirty or non-VCS builds leave buildVersion empty.
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
