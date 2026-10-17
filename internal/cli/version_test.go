// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionLdflags(t *testing.T) {
	oldOverride, oldReadBuildInfo := versionOverride, readBuildInfo
	versionOverride = "v0.0.0-test"
	readBuildInfo = func() (*debug.BuildInfo, bool) { return nil, false }
	t.Cleanup(func() {
		versionOverride, readBuildInfo = oldOverride, oldReadBuildInfo
	})

	for _, args := range [][]string{{"version"}, {"--version"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			r := run(testEnv(t.TempDir()), "", args...)
			if r.code != 0 {
				t.Errorf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			if r.stdout != "v0.0.0-test\n" {
				t.Errorf("stdout = %q, want %q", r.stdout, "v0.0.0-test\n")
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestVersionBuildInfo(t *testing.T) {
	oldOverride, oldReadBuildInfo := versionOverride, readBuildInfo
	versionOverride = ""
	t.Cleanup(func() {
		versionOverride, readBuildInfo = oldOverride, oldReadBuildInfo
	})

	tests := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{
			name: "module version",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}},
			ok:   true,
			want: "v1.2.3",
		},
		{
			name: "development without revision",
			info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			ok:   true,
			want: "(devel)",
		},
		{
			name: "development with revision",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}},
			},
			ok:   true,
			want: "(devel) abc123",
		},
		{name: "unavailable build info", want: "(devel)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readBuildInfo = func() (*debug.BuildInfo, bool) { return tt.info, tt.ok }
			r := run(testEnv(t.TempDir()), "", "version")
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			if r.stdout != tt.want+"\n" {
				t.Errorf("stdout = %q, want %q", r.stdout, tt.want+"\n")
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestVersionCommandRejectsExtraArguments(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "version", "extra")
	if r.code != 2 {
		t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if !strings.Contains(r.stderr, "usage: adj version") {
		t.Errorf("stderr has no version usage:\n%s", r.stderr)
	}
}
