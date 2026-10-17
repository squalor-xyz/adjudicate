// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const amendUsage = `usage: adj amend ID [flags]

Correct the text of a record of any status. Prints its ID. Only the text
changes: never the status, decision, who decided or asked, dates, links, key,
tags, an option's label or the recommendation. Give at least one flag. The
record gets one amend history entry, whatever the number of fields changed,
that keeps the old text; adj show prints it.

  --title TEXT              the new title (one line)
  --context TEXT            the new context
  --stdin                   read the context from standard input instead
  --summary LABEL:TEXT      the new summary of the option LABEL; once per
                            option; an empty TEXT clears it
  --tradeoffs LABEL:TEXT    the new trade-offs of the option LABEL; once per
                            option; an empty TEXT clears them
  --rationale TEXT          the new rationale (an open record has none)
  --root DIR                the repo root; default: found from the working
                            directory upwards (the nearest .adjudicate.json)

LABEL:TEXT splits at the first colon, so the text may hold colons. A text that
the record already has is not a change; if nothing changes, amend fails.
`

func (c *invocation) runAmend(args []string) int {
	fs, root := newFlagSet("amend")
	var summaries, tradeoffs stringList
	fs.Var(&summaries, "summary", "")
	fs.Var(&tradeoffs, "tradeoffs", "")
	title := fs.String("title", "", "")
	context := fs.String("context", "", "")
	fromStdin := fs.Bool("stdin", false, "")
	rationale := fs.String("rationale", "", "")

	pos, code, done := c.parse("amend", amendUsage, fs, args)
	if done {
		return code
	}
	id, err := idArg(pos)
	if err != nil {
		return c.usageError("amend", amendUsage, err)
	}
	if len(pos) > 1 {
		return c.usageError("amend", amendUsage, fmt.Errorf("want one ID, got %d arguments", len(pos)))
	}
	if *fromStdin && fsSet(fs, "context") {
		return c.usageError("amend", amendUsage, errors.New("--context and --stdin are exclusive"))
	}
	in := commands.AmendInput{
		Title: *title, HasTitle: fsSet(fs, "title"),
		Context: *context, HasContext: fsSet(fs, "context"),
		Rationale: *rationale, HasRationale: fsSet(fs, "rationale"),
		Summaries: summaries, Tradeoffs: tradeoffs,
	}
	// Standard input is read last, so bad arguments and a missing init fail
	// before anything is consumed; until then it counts as a context.
	check := in
	if *fromStdin {
		check.Context, check.HasContext = "-", true
	}
	return c.answerRecord("amend", amendUsage, *root, check.Check(), func(st *store.Store) (model.Record, error) {
		if *fromStdin {
			data, err := io.ReadAll(c.stdin)
			if err != nil {
				return model.Record{}, err
			}
			in.Context, in.HasContext = strings.TrimRight(string(data), "\r\n"), true
		}
		return commands.Amend(st, id, in, c.env.Now())
	})
}
