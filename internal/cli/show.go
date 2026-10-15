// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const showUsage = `usage: adj show ID [flags]

Print one record: its fields and history. With --json, print the record as a
JSON object (the shape is in docs/json.md). Exits 1 if no record has the ID.

  --json       print JSON instead of text
  --root DIR   the repo root; default: found from the working directory
               upwards (the nearest .adjudicate.json)
`

func (c *invocation) runShow(args []string) int {
	fs, root := newFlagSet("show")
	asJSON := fs.Bool("json", false, "")

	pos, code, done := c.parse("show", showUsage, fs, args)
	if done {
		return code
	}
	id, err := idArg(pos)
	if err != nil {
		return c.usageError("show", showUsage, err)
	}
	if len(pos) > 1 {
		return c.usageError("show", showUsage, fmt.Errorf("want one ID, got %d arguments", len(pos)))
	}
	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("show", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("show", err)
	}
	rec, err := commands.Show(st, id)
	if err != nil {
		return c.fail("show", err)
	}
	if *asJSON {
		err = render.RecordJSON(c.stdout, rec)
	} else {
		err = render.RecordText(c.stdout, rec)
	}
	if err != nil {
		return c.fail("show", err)
	}
	return exitOK
}
