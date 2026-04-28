package cmd

import (
	"runtime/debug"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
)

func TestVersionFromBuildInfo_Tagged(t *testing.T) {
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "2026-04-27T09:52:22Z"},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	assert.Equal(t, "v0.0.1777283542", versionFromBuildInfo(info))
}

func TestVersionFromBuildInfo_DirtyTree(t *testing.T) {
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "2026-04-27T09:52:22Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	assert.Equal(t, "", versionFromBuildInfo(info))
}

func TestVersionFromBuildInfo_NoVCS(t *testing.T) {
	info := &debug.BuildInfo{Settings: nil}
	assert.Equal(t, "", versionFromBuildInfo(info))
}

func TestVersionFromBuildInfo_BadTime(t *testing.T) {
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "not-a-date"},
		},
	}
	assert.Equal(t, "", versionFromBuildInfo(info))
}
