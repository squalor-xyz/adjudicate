// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const decideUsage = `usage: adj decide ID OPTION --why TEXT
       adj decide ID --text TEXT --why TEXT

Answer an open question: OPTION must be one of the record's option labels, or
--text gives a free-text answer. The record becomes decided. Prints its ID.

  --text TEXT    the answer as free text, instead of OPTION
  --why TEXT     the reason for the answer (required)
  --by NAME      who decided; default: $ADJ_DECIDED_BY, else left blank
  --root DIR     the repo root; default: found from the working directory
                 upwards (the nearest .adjudicate.json)
`

const rejectUsage = `usage: adj reject ID --why TEXT [flags]

Close an open question without an answer: it becomes rejected, so it will not
be answered. Prints its ID.

  --why TEXT     the reason (required)
  --by NAME      who decided; default: $ADJ_DECIDED_BY, else left blank
  --root DIR     the repo root; default: found from the working directory
                 upwards (the nearest .adjudicate.json)
`

const withdrawUsage = `usage: adj withdraw ID --why TEXT [flags]

Close an open question because the asker no longer needs an answer: it becomes
withdrawn. Prints its ID.

  --why TEXT     the reason (required)
  --by NAME      who withdrew it; default: $ADJ_DECIDED_BY, else left blank
  --root DIR     the repo root; default: found from the working directory
                 upwards (the nearest .adjudicate.json)
`

// reasonCommand is what reject and withdraw run in the commands package.
type reasonCommand func(*store.Store, int, commands.ReasonInput, time.Time) (model.Record, error)

// idArg returns the record ID that the first positional argument names: a
// positive integer.
func idArg(pos []string) (int, error) {
	if len(pos) == 0 {
		return 0, errors.New("ID is required")
	}
	id, err := strconv.Atoi(pos[0])
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("ID must be a positive integer, got %q", pos[0])
	}
	return id, nil
}

func (c *invocation) runDecide(args []string) int {
	fs, root := newFlagSet("decide")
	text := fs.String("text", "", "")
	why := fs.String("why", "", "")
	by := fs.String("by", "", "")

	pos, code, done := c.parse("decide", decideUsage, fs, args)
	if done {
		return code
	}
	id, err := idArg(pos)
	if err != nil {
		return c.usageError("decide", decideUsage, err)
	}
	if len(pos) > 2 {
		return c.usageError("decide", decideUsage, fmt.Errorf("want an ID and one OPTION, got %d arguments", len(pos)))
	}
	in := commands.DecideInput{
		HasText:   fsSet(fs, "text"),
		Text:      *text,
		Why:       *why,
		DecidedBy: *by,
	}
	if len(pos) == 2 {
		in.HasOption = true
		in.Option = pos[1]
	}
	if in.DecidedBy == "" {
		in.DecidedBy = c.env.Getenv("ADJ_DECIDED_BY")
	}
	return c.answerRecord("decide", decideUsage, *root, in.Check(), func(st *store.Store) (model.Record, error) {
		return commands.Decide(st, id, in, c.env.Now())
	})
}

// runReasonCommand runs reject or withdraw: an ID and a --why.
func (c *invocation) runReasonCommand(name, usageText string, args []string, do reasonCommand) int {
	fs, root := newFlagSet(name)
	why := fs.String("why", "", "")
	by := fs.String("by", "", "")

	pos, code, done := c.parse(name, usageText, fs, args)
	if done {
		return code
	}
	id, err := idArg(pos)
	if err != nil {
		return c.usageError(name, usageText, err)
	}
	if len(pos) > 1 {
		return c.usageError(name, usageText, fmt.Errorf("want an ID and --why, got %d arguments", len(pos)))
	}
	in := commands.ReasonInput{Why: *why, DecidedBy: *by}
	if in.DecidedBy == "" {
		in.DecidedBy = c.env.Getenv("ADJ_DECIDED_BY")
	}
	return c.answerRecord(name, usageText, *root, in.Check(), func(st *store.Store) (model.Record, error) {
		return do(st, id, in, c.env.Now())
	})
}

// answerRecord is the part of a decision command after its arguments are
// parsed. A check error (bad input, found before the repo is looked for) is a
// usage error. Otherwise it finds the repo, opens the store, runs do and prints
// the ID of the record it changed (C74).
func (c *invocation) answerRecord(name, usageText, flagRoot string, check error, do func(*store.Store) (model.Record, error)) int {
	if check != nil {
		return c.usageError(name, usageText, check)
	}
	dir, err := c.rootFor(flagRoot, false)
	if err != nil {
		return c.rootError(name, err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail(name, err)
	}
	rec, err := do(st)
	if err != nil {
		return c.commandError(name, usageText, err)
	}
	fmt.Fprintf(c.stdout, "%d\n", rec.ID)
	return exitOK
}
