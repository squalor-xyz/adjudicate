// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/squalor-xyz/adjudicate/internal/commands"
)

const aiUsage = `usage: adj ai

Print a short AGENTS.md snippet that tells an agent how to use adj: check
before asking, ask, read the decisions, and leave deciding to the owner. Paste
it into the AGENTS.md of a repo that uses adjudicate. It reads no repo and
takes no flags.
`

func (c *invocation) runAI(args []string) int {
	fs := flag.NewFlagSet("ai", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	pos, code, done := c.parse("ai", aiUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("ai", aiUsage, fmt.Errorf("ai takes no arguments, got %q", pos[0]))
	}
	fmt.Fprint(c.stdout, commands.AgentSnippet())
	return exitOK
}
