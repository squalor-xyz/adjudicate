// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const inboxUsage = `usage: adj inbox [flags]

List the open questions, oldest first (by created date, then ID), each with its
options and recommendation. With --json, print {"records": [...]} (the shape is
in docs/json.md). An empty inbox is not an error.

  --json       print JSON instead of text
  --root DIR   the repo root; default: found from the working directory
               upwards (the nearest .adjudicate.json)
`

func (c *invocation) runInbox(args []string) int {
	fs, root := newFlagSet("inbox")
	asJSON := fs.Bool("json", false, "")

	pos, code, done := c.parse("inbox", inboxUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("inbox", inboxUsage, fmt.Errorf("inbox takes no arguments, got %q", pos[0]))
	}
	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("inbox", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("inbox", err)
	}
	recs, err := commands.Inbox(st)
	if err != nil {
		return c.commandError("inbox", inboxUsage, err)
	}
	if *asJSON {
		err = render.RecordsJSON(c.stdout, recs)
	} else {
		err = render.InboxText(c.stdout, recs)
	}
	if err != nil {
		return c.fail("inbox", err)
	}
	return exitOK
}
