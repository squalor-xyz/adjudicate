// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/model"
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

With --max-bytes N the output is at most N bytes: the newest decisions (by
decided_at, then ID) that fit are kept and printed in ID order, and md ends with
"N decisions omitted" (json gains "omitted", the count; docs/json.md). If not
even the omitted line fits, nothing is printed and the exit code is 1.

  --tag T          only records with tag T; repeat to require several tags
  --key-prefix P   only records whose key starts with P (a plain prefix, as
                   typed: storage matches storage.engine and storagex)
  --format F       md (the default, a Markdown list) or json
  --max-bytes N    keep the newest decisions that fit in N bytes (N >= 1)
  --root DIR       the repo root; default: found from the working directory
                   upwards (the nearest .adjudicate.json)
`

func (c *invocation) runContext(args []string) int {
	fs, root := newFlagSet("context")
	var tags stringList
	fs.Var(&tags, "tag", "")
	keyPrefix := fs.String("key-prefix", "", "")
	format := fs.String("format", "md", "")
	maxBytes := fs.String("max-bytes", "", "")

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
	if fsSet(fs, "max-bytes") {
		n, err := strconv.Atoi(strings.TrimSpace(*maxBytes))
		if err != nil {
			return c.usageError("context", contextUsage, fmt.Errorf("--max-bytes must be a whole number of at least 1; got %q", *maxBytes))
		}
		in.MaxBytes, in.HasMaxBytes = n, true
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
	switch {
	case in.HasMaxBytes:
		write := render.ContextTextOmitted
		if *format == "json" {
			write = render.ContextJSONOmitted
		}
		measure := func(kept []model.Record, omitted int) (int, error) {
			var n countWriter
			err := write(&n, kept, omitted)
			return int(n), err
		}
		var omitted int
		recs, omitted, err = commands.FitContext(recs, in.MaxBytes, measure)
		if err != nil {
			return c.commandError("context", contextUsage, err)
		}
		err = write(c.stdout, recs, omitted)
	case *format == "json":
		err = render.ContextJSON(c.stdout, recs)
	default:
		err = render.ContextText(c.stdout, recs)
	}
	if err != nil {
		return c.fail("context", err)
	}
	return exitOK
}

// countWriter counts the bytes written to it.
type countWriter int

func (n *countWriter) Write(p []byte) (int, error) {
	*n += countWriter(len(p))
	return len(p), nil
}
