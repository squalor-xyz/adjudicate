// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const askUsage = `usage: adj ask TITLE [flags]

Record an open question. Prints the new record's ID.

  --option LABEL:SUMMARY    an answer on offer; repeat for each option
  --tradeoffs LABEL:TEXT    the trade-offs of the option LABEL; once per option
  --recommend LABEL         the option you recommend
  --recommend-text TEXT     a recommendation that is not one of the options
                            (instead of --recommend)
  --reasoning TEXT          why you recommend it (needs --recommend or
                            --recommend-text)
  --key KEY                 the decision key, such as storage.engine
  --tag TAG                 a tag; repeat for several
  --context TEXT            why the question is asked
  --stdin                   read the context from standard input instead
  --by NAME                 who asks; default: $ADJ_ASKED_BY, else left blank
  --root DIR                the repo root; default: found from the working
                            directory upwards (the nearest .adjudicate.json)

The flags may come before or after TITLE. LABEL:SUMMARY and LABEL:TEXT split
at the first colon, so the text may hold colons.
`

func (c *invocation) runAsk(args []string) int {
	fs, root := newFlagSet("ask")
	var options, tradeoffs, tags stringList
	fs.Var(&options, "option", "")
	fs.Var(&tradeoffs, "tradeoffs", "")
	fs.Var(&tags, "tag", "")
	recommend := fs.String("recommend", "", "")
	recommendText := fs.String("recommend-text", "", "")
	reasoning := fs.String("reasoning", "", "")
	key := fs.String("key", "", "")
	context := fs.String("context", "", "")
	fromStdin := fs.Bool("stdin", false, "")
	by := fs.String("by", "", "")

	pos, code, done := c.parse("ask", askUsage, fs, args)
	if done {
		return code
	}
	switch len(pos) {
	case 0:
		return c.usageError("ask", askUsage, errors.New("TITLE is required"))
	case 1:
	default:
		return c.usageError("ask", askUsage, fmt.Errorf("want one TITLE, got %d arguments (quote a title that has spaces)", len(pos)))
	}
	in := commands.AskInput{
		Title:     pos[0],
		Key:       *key,
		Tags:      tags,
		Context:   *context,
		AskedBy:   *by,
		Options:   options,
		Tradeoffs: tradeoffs,
		Recommend: *recommend,
		Reasoning: *reasoning,

		RecommendText: *recommendText,
	}
	if in.AskedBy == "" {
		in.AskedBy = c.env.Getenv("ADJ_ASKED_BY")
	}
	if *fromStdin && fsSet(fs, "context") {
		return c.usageError("ask", askUsage, errors.New("--context and --stdin are exclusive"))
	}
	if fsSet(fs, "recommend") && fsSet(fs, "recommend-text") {
		return c.usageError("ask", askUsage, errors.New("--recommend and --recommend-text are exclusive"))
	}
	if fsSet(fs, "recommend-text") && strings.TrimSpace(*recommendText) == "" {
		return c.usageError("ask", askUsage, errors.New("--recommend-text must not be blank"))
	}
	if fsSet(fs, "recommend") && strings.TrimSpace(*recommend) == "" {
		return c.usageError("ask", askUsage, errors.New("--recommend must not be blank"))
	}
	if err := in.Check(); err != nil {
		return c.commandError("ask", askUsage, err)
	}

	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("ask", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("ask", err)
	}
	// Standard input is read last, so a usage error or a missing init fails
	// before anything is consumed.
	if *fromStdin {
		data, err := io.ReadAll(c.stdin)
		if err != nil {
			return c.fail("ask", err)
		}
		in.Context = strings.TrimRight(string(data), "\r\n")
	}
	rec, err := commands.Ask(st, in, c.env.Now())
	if err != nil {
		return c.commandError("ask", askUsage, err)
	}
	fmt.Fprintf(c.stdout, "%d\n", rec.ID)
	return exitOK
}
