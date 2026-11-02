// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/render"
)

const versionUsage = `usage: adj version [--json] [--root DIR]

Print the version from Go build information. A release build can override
it with -ldflags -X. With --json, print {"version", "contract"}: the version
text and the JSON contract this adj speaks (adj contract).

  --json       print JSON instead of the version alone
`

// versionOverride is set by release builds with -ldflags -X (release.yml).
var versionOverride string

// readBuildInfo is a seam for testing build metadata without building a new
// executable for every version case.
var readBuildInfo = debug.ReadBuildInfo

func (c *invocation) runVersion(args []string) int {
	fs, _ := newFlagSet("version")
	asJSON := fs.Bool("json", false, "")
	pos, code, done := c.parse("version", versionUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("version", versionUsage, fmt.Errorf("version takes no arguments, got %q", pos[0]))
	}
	if *asJSON {
		if err := render.VersionJSON(c.stdout, currentVersion()); err != nil {
			return c.fail("version", err)
		}
		return exitOK
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
