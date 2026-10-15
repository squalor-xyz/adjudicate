// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const listUsage = `usage: adj list [flags]

List records in ID order. Every filter given must match. With --json, print
{"records": [...]} (the shape is in docs/json.md). No match is not an error.

  --status S   only records with status S: open, decided, rejected, withdrawn
               or superseded
  --tag T      only records with tag T; repeat to require several tags
  --key K      only records whose key is K exactly
  --json       print JSON instead of one line per record
  --root DIR   the repo root; default: found from the working directory
               upwards (the nearest .adjudicate.json)
`

func (c *invocation) runList(args []string) int {
	fs, root := newFlagSet("list")
	status := fs.String("status", "", "")
	var tags stringList
	fs.Var(&tags, "tag", "")
	key := fs.String("key", "", "")
	asJSON := fs.Bool("json", false, "")

	pos, code, done := c.parse("list", listUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("list", listUsage, fmt.Errorf("list takes no arguments, got %q", pos[0]))
	}
	in := commands.ListInput{
		Status:    *status,
		HasStatus: fsSet(fs, "status"),
		Tags:      tags,
		Key:       *key,
		HasKey:    fsSet(fs, "key"),
	}
	if err := in.Check(); err != nil {
		return c.usageError("list", listUsage, err)
	}
	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("list", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("list", err)
	}
	recs, err := commands.List(st, in)
	if err != nil {
		return c.commandError("list", listUsage, err)
	}
	if *asJSON {
		err = render.RecordsJSON(c.stdout, recs)
	} else {
		err = render.RecordsText(c.stdout, recs)
	}
	if err != nil {
		return c.fail("list", err)
	}
	return exitOK
}
