// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"runtime/debug"
	"strings"
)

const versionUsage = `usage: adj version [--root DIR]

Print the version from Go build information. A local release build can override
it with -ldflags -X.
`

// versionOverride is set by local release builds with -ldflags -X.
var versionOverride string

// readBuildInfo is a seam for testing build metadata without building a new
// executable for every version case.
var readBuildInfo = debug.ReadBuildInfo

func (c *invocation) runVersion(args []string) int {
	fs, _ := newFlagSet("version")
	pos, code, done := c.parse("version", versionUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("version", versionUsage, fmt.Errorf("version takes no arguments, got %q", pos[0]))
	}
	fmt.Fprintln(c.stdout, currentVersion())
	return exitOK
}

func currentVersion() string {
	if versionOverride != "" {
		return versionOverride
	}

	info, ok := readBuildInfo()
	if !ok || info == nil {
		return "(devel)"
	}

	version := info.Main.Version
	if version == "" {
		version = "(devel)"
	}
	if version == "(devel)" {
		if revision := buildSetting(info.Settings, "vcs.revision"); revision != "" {
			return version + " " + revision
		}
	}
	return version
}

func buildSetting(settings []debug.BuildSetting, key string) string {
	for _, setting := range settings {
		if setting.Key == key {
			return strings.TrimSpace(setting.Value)
		}
	}
	return ""
}
