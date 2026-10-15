// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
)

const initUsage = `usage: adj init [--root DIR]

Create the config file (.adjudicate.json) and the store directory in DIR, or
in the working directory. Safe to run again: an existing config and the
records in the store are left as they are.

  --root DIR   the repo root; default: the working directory
`

func (c *invocation) runInit(args []string) int {
	fs, root := newFlagSet("init")
	pos, code, done := c.parse("init", initUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("init", initUsage, fmt.Errorf("unexpected argument %q", pos[0]))
	}
	dir, err := c.rootFor(*root, true)
	if err != nil {
		return c.rootError("init", err)
	}
	wrote, err := commands.Init(dir)
	if err != nil {
		return c.fail("init", err)
	}
	if wrote {
		fmt.Fprintf(c.stdout, "initialized %s\n", dir)
	} else {
		fmt.Fprintf(c.stdout, "already initialized %s\n", dir)
	}
	return exitOK
}
