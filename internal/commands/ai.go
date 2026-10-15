// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import _ "embed"

// agentSnippet is the AGENTS.md snippet that adj ai prints (S12). It is a
// Markdown file so it reads as it will be pasted; the cli tests check that
// every command, flag and variable it names exists.
//
//go:embed ai.md
var agentSnippet string

// AgentSnippet returns the AGENTS.md snippet that tells an agent how to use
// adj: check before asking, ask, read decisions, and leave deciding to the
// owner (C27). It ends with one newline.
func AgentSnippet() string { return agentSnippet }
