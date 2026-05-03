package cmd

import (
	"fmt"
	"runtime/debug"
	"time"
)

// autoreleaseVersion derives "v0.0.<unix-seconds>" from the binary's embedded
// vcs.time so the running binary reports the same tag the autorelease pipeline
// publishes. The library's own auto-detect prefers debug.BuildInfo.Main.Version,
// which Go populates with a pseudo-version like "v0.0.0-YYYYMMDDHHMMSS-shortsha"
// for binaries built from a VCS checkout — that doesn't match any GitHub release
// tag, so without this override `version` and `update` report mismatched versions.
//
// Returns "" when VCS info is unavailable, in which case the library's default
// fallback chain (EmbeddedVersion -> Main.Version -> short revision -> "(devel)")
// takes over.
func autoreleaseVersion() string {
	return autoreleaseVersionFrom(debug.ReadBuildInfo)
}

func autoreleaseVersionFrom(readBuildInfo func() (*debug.BuildInfo, bool)) string {
	info, ok := readBuildInfo()
	if !ok || info == nil {
		return ""
	}
	var vcsTime string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.time":
			vcsTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if vcsTime == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, vcsTime)
	if err != nil {
		return ""
	}
	v := fmt.Sprintf("v0.0.%d", t.Unix())
	if modified {
		v += "+dirty"
	}
	return v
}
