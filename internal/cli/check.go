// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const checkUsage = `usage: adj check [flags] QUERY

Check whether QUERY has been decided before asking the owner. QUERY is one
substring, matched ignoring case against a record's title, key, tags, refs and
decision text; it is not split into words, so a single word or a key
(storage.engine) finds the most. Quote a query that has spaces. The first line
of the output is the result:

  decided     a decided record matches (open and closed matches are listed after)
  open        no decided record matches, but an open question does
  closed      only rejected, withdrawn or superseded records match
  not found   nothing matches

A superseded match is listed as closed, and the decided record that replaced it
(following superseded_by) is listed as decided. With --json, print {"query",
"result", "decided", "open", "closed"} (the shape is in docs/json.md).

Exit codes: exit 0 for every completed check, whatever the result, so read the
result from the output; exit 1 if the check failed (no repo, an unreadable
store); exit 2 for a usage error.

  --json       print JSON instead of text
  --root DIR   the repo root; default: found from the working directory
               upwards (the nearest .adjudicate.json)
`

func (c *invocation) runCheck(args []string) int {
	fs, root := newFlagSet("check")
	asJSON := fs.Bool("json", false, "")

	pos, code, done := c.parse("check", checkUsage, fs, args)
	if done {
		return code
	}
	if len(pos) != 1 {
		return c.usageError("check", checkUsage, fmt.Errorf("check takes one QUERY, got %d arguments; quote a query that has spaces", len(pos)))
	}
	in := commands.CheckInput{Query: pos[0]}
	if err := in.Check(); err != nil {
		return c.usageError("check", checkUsage, err)
	}
	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("check", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("check", err)
	}
	res, err := commands.Check(st, in)
	if err != nil {
		return c.commandError("check", checkUsage, err)
	}
	if *asJSON {
		err = render.CheckJSON(c.stdout, res)
	} else {
		err = render.CheckText(c.stdout, res)
	}
	if err != nil {
		return c.fail("check", err)
	}
	return exitOK
}
