package cmd

import (
	"runtime/debug"
	"testing"

	"github.com/wow-look-at-my/testify/assert"
)

func TestAutoreleaseVersion(t *testing.T) {
	// 2026-05-03T06:53:13Z = unix 1777791193 (the test-server v0.0.1777791193 release).
	const vcsTime = "2026-05-03T06:53:13Z"
	const wantTag = "v0.0.1777791193"

	tests := []struct {
		name string
		read func() (*debug.BuildInfo, bool)
		want string
	}{
		{
			name: "clean build matches release tag",
			read: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Settings: []debug.BuildSetting{
					{Key: "vcs.time", Value: vcsTime},
					{Key: "vcs.modified", Value: "false"},
				}}, true
			},
			want: wantTag,
		},
		{
			name: "dirty tree gets +dirty suffix",
			read: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Settings: []debug.BuildSetting{
					{Key: "vcs.time", Value: vcsTime},
					{Key: "vcs.modified", Value: "true"},
				}}, true
			},
			want: wantTag + "+dirty",
		},
		{
			name: "ignores Main.Version pseudo-version",
			read: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{
					Main: debug.Module{Version: "v0.0.0-20260503065313-088071340bf8"},
					Settings: []debug.BuildSetting{
						{Key: "vcs.time", Value: vcsTime},
						{Key: "vcs.modified", Value: "false"},
					},
				}, true
			},
			want: wantTag,
		},
		{
			name: "missing vcs.time falls through",
			read: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef"},
				}}, true
			},
			want: "",
		},
		{
			name: "unparseable vcs.time falls through",
			read: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{Settings: []debug.BuildSetting{
					{Key: "vcs.time", Value: "not-a-date"},
				}}, true
			},
			want: "",
		},
		{
			name: "build info unavailable falls through",
			read: func() (*debug.BuildInfo, bool) { return nil, false },
			want: "",
		},
		{
			name: "nil build info falls through",
			read: func() (*debug.BuildInfo, bool) { return nil, true },
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, autoreleaseVersionFrom(tt.read))
		})
	}
}
