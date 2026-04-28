package cmd

import (
	"runtime/debug"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
)

func TestAutoreleaseVersionFromBuildInfo_Tagged(t *testing.T) {
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "2026-04-27T09:52:22Z"},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	assert.Equal(t, "v0.0.1777283542", autoreleaseVersionFromBuildInfo(info))
}

func TestAutoreleaseVersionFromBuildInfo_DirtyTree(t *testing.T) {
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "2026-04-27T09:52:22Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	assert.Equal(t, "v0.0.1777283542+dirty", autoreleaseVersionFromBuildInfo(info))
}

func TestAutoreleaseVersionFromBuildInfo_NoVCS(t *testing.T) {
	info := &debug.BuildInfo{Settings: nil}
	assert.Equal(t, "", autoreleaseVersionFromBuildInfo(info))
}

func TestAutoreleaseVersionFromBuildInfo_BadTime(t *testing.T) {
	info := &debug.BuildInfo{
		Settings: []debug.BuildSetting{
			{Key: "vcs.time", Value: "not-a-date"},
		},
	}
	assert.Equal(t, "", autoreleaseVersionFromBuildInfo(info))
}

func TestShouldSelfUpdateWow(t *testing.T) {
	cases := map[string]bool{
		"":                       false,
		"(devel)":                false,
		"v0.0.1777283542+dirty":  false,
		"f85fe26374c6+dirty":     false,
		"v0.0.1777283542":        true,
		"v1.2.3":                 true,
	}
	for in, want := range cases {
		assert.Equal(t, want, shouldSelfUpdateWow(in), in)
	}
}
