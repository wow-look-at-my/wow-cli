package cmd

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	selfupdate "github.com/wow-look-at-my/go-selfupdate-mini"
)

var versionBare bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		out := cmd.OutOrStdout()
		if versionBare {
			fmt.Fprintln(out, buildVersion)
			return nil
		}

		up, err := newUpdater()
		if err != nil {
			return err
		}
		latest, found, err := up.DetectLatest(context.Background(), selfupdate.ParseSlug("wow-look-at-my/wow-cli"))
		if err != nil || !found {
			fmt.Fprintf(out, "version: %s\n", buildVersion)
			return nil
		}

		age := humanizeAge(time.Since(latest.PublishedAt))
		if latest.Version.Original == buildVersion {
			fmt.Fprintf(out, "version: %s (latest, released %s)\n", buildVersion, age)
		} else {
			fmt.Fprintf(out, "version: %s\n", buildVersion)
			fmt.Fprintf(out, "latest:  %s (released %s)\n", latest.Version.Original, age)
		}
		return nil
	},
}

func init() {
	populateBuildVersion()
	rootCmd.Version = buildVersion
	versionCmd.Flags().BoolVar(&versionBare, "bare", false, "print only the version string")
	rootCmd.AddCommand(versionCmd)
}

func humanizeAge(d time.Duration) string {
	days := int(d.Hours() / 24)
	switch {
	case days < 1:
		return "today"
	case days == 1:
		return "1 day ago"
	case days < 14:
		return fmt.Sprintf("%d days ago", days)
	case days < 30:
		return fmt.Sprintf("%d weeks ago", days/7)
	default:
		months := days / 30
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
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
