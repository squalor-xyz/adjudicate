// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const inboxUsage = `usage: adj inbox [--json] [--root DIR]
       adj inbox --all [--json]
       adj inbox -i [--by NAME] [--root DIR]

List the open questions, oldest first (by created date, then ID), each with its
options and recommendation. With --json, print {"records": [...]} (the shape is
in docs/json.md). An empty inbox is not an error.

  --all        read the user repo list; group by repo, including empty repos
               warnings go to stderr and the JSON errors array; partial failure exits 1
               cannot be combined with --root
  -i           owner-only line-mode walk; cannot combine with --all or --json
  --by NAME    who decides in -i; default: $ADJ_DECIDED_BY, else blank
  --json       print JSON instead of text
  --root DIR   the repo root; default: found from the working directory
               upwards (the nearest .adjudicate.json)
`

func (c *invocation) runInbox(args []string) int {
	fs, root := newFlagSet("inbox")
	asJSON := fs.Bool("json", false, "")
	all := fs.Bool("all", false, "")
	interactive := fs.Bool("i", false, "")
	by := fs.String("by", "", "")

	pos, code, done := c.parse("inbox", inboxUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("inbox", inboxUsage, fmt.Errorf("inbox takes no arguments, got %q", pos[0]))
	}
	if *interactive && (*asJSON || *all) {
		return c.usageError("inbox", inboxUsage, fmt.Errorf("-i cannot be combined with --json or --all"))
	}
	if !*interactive && fsSet(fs, "by") {
		return c.usageError("inbox", inboxUsage, fmt.Errorf("--by requires -i"))
	}
	if *all {
		if fsSet(fs, "root") {
			return c.usageError("inbox", inboxUsage, fmt.Errorf("--all cannot be combined with --root"))
		}
		result, err := commands.InboxAll(c.env.Getenv)
		if err != nil {
			return c.fail("inbox", err)
		}
		for _, warning := range result.Errors {
			fmt.Fprintf(c.stderr, "adj inbox: warning: %s: %s\n", warning.Root, warning.Error)
		}
		if *asJSON {
			err = render.InboxAllJSON(c.stdout, result)
		} else {
			err = render.InboxAllText(c.stdout, result)
		}
		if err != nil {
			return c.fail("inbox", err)
		}
		if len(result.Errors) > 0 {
			return exitFail
		}
		return exitOK
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
	if *interactive {
		if *by == "" {
			*by = c.env.Getenv("ADJ_DECIDED_BY")
		}
		return c.walkInbox(st, recs, *by)
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
