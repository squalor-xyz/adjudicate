// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const contextUsage = `usage: adj context [flags]

Print the decided records as compact context for an agent: per decision its ID,
title, key, the decision, and the first line of its rationale. Open, rejected,
withdrawn and superseded records are left out, so a superseded decision gives
way to the one that replaced it. Records are in ID order and the same store
gives the same bytes. No decision is not an error: md prints nothing and json
prints {"decisions": []}. The json shape is in docs/json.md.

Every filter given must match.

  --tag T          only records with tag T; repeat to require several tags
  --key-prefix P   only records whose key starts with P (a plain prefix, as
                   typed: storage matches storage.engine and storagex)
  --format F       md (the default, a Markdown list) or json
  --root DIR       the repo root; default: found from the working directory
                   upwards (the nearest .adjudicate.json)
`

func (c *invocation) runContext(args []string) int {
	fs, root := newFlagSet("context")
	var tags stringList
	fs.Var(&tags, "tag", "")
	keyPrefix := fs.String("key-prefix", "", "")
	format := fs.String("format", "md", "")

	pos, code, done := c.parse("context", contextUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("context", contextUsage, fmt.Errorf("context takes no arguments, got %q", pos[0]))
	}
	if *format != "md" && *format != "json" {
		return c.usageError("context", contextUsage, fmt.Errorf("--format must be md or json; got %q", *format))
	}
	in := commands.ContextInput{
		Tags:         tags,
		KeyPrefix:    *keyPrefix,
		HasKeyPrefix: fsSet(fs, "key-prefix"),
	}
	if err := in.Check(); err != nil {
		return c.usageError("context", contextUsage, err)
	}
	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("context", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("context", err)
	}
	recs, err := commands.Context(st, in)
	if err != nil {
		return c.commandError("context", contextUsage, err)
	}
	if *format == "json" {
		err = render.ContextJSON(c.stdout, recs)
	} else {
		err = render.ContextText(c.stdout, recs)
	}
	if err != nil {
		return c.fail("context", err)
	}
	return exitOK
}
