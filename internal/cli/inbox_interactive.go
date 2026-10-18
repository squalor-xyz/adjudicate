// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const inboxActionPrompt = "Action [d=option, t=text, r=reject, w=withdraw, s=skip, q=quit, ?=help]: "
const inboxActionHelp = "d: decide by option label (blank returns to action; no options: use t); t: decide with free text; r: reject; w: withdraw; s: skip; q: quit; ?: help\n"

// inboxInput reads complete lines. EOF discards an unfinished field, so a
// truncated pipe cannot turn a partial rationale into a stored decision.
type inboxInput struct {
	reader *bufio.Reader
	out    io.Writer
	err    error
}

func (in *inboxInput) line(prompt string) (string, bool) {
	if _, in.err = io.WriteString(in.out, prompt); in.err != nil {
		return "", false
	}
	text, err := in.reader.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			in.err = err
		}
		return "", false
	}
	return strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r"), true
}

func (c *invocation) inboxRequired(in *inboxInput, prompt, field string) (string, bool) {
	for {
		text, ok := in.line(prompt)
		if !ok {
			return "", false
		}
		if strings.TrimSpace(text) != "" {
			return text, true
		}
		fmt.Fprintf(c.stderr, "adj inbox: %s must not be blank\n", field)
	}
}

// walkInbox uses the initial oldest-first list for display, but all writes go
// through the one-shot commands, which re-read and validate the current record.
func (c *invocation) walkInbox(st *store.Store, recs []model.Record, by string) int {
	if len(recs) == 0 {
		return exitOK
	}
	in := &inboxInput{reader: bufio.NewReader(c.stdin), out: c.stdout}
	var decided, rejected, withdrawn, skipped, failed int
walk:
	for i, rec := range recs {
		if i > 0 {
			if _, err := fmt.Fprintln(c.stdout); err != nil {
				return c.fail("inbox", err)
			}
		}
		if err := render.InboxRecordText(c.stdout, rec); err != nil {
			return c.fail("inbox", err)
		}
	actions:
		for {
			action, ok := in.line(inboxActionPrompt)
			if !ok {
				break walk
			}
			action = strings.TrimSpace(action)
			switch action {
			case "q":
				break walk
			case "s":
				skipped++
				continue walk
			case "?":
				if _, err := io.WriteString(c.stdout, inboxActionHelp); err != nil {
					return c.fail("inbox", err)
				}
				continue
			case "d", "t", "r", "w":
			default:
				fmt.Fprintln(c.stderr, "adj inbox: Unknown action; type ? for help")
				continue
			}
			var answer string
			if action == "d" {
				if len(rec.Options) == 0 {
					fmt.Fprintf(c.stderr, "adj inbox: record %d has no options; use t to decide with text\n", rec.ID)
					continue actions
				}
				for {
					answer, ok = in.line("Option label: ")
					if !ok {
						break walk
					}
					answer = strings.TrimSpace(answer)
					if answer == "" {
						continue actions
					}
					found := false
					for _, option := range rec.Options {
						if option.Label == answer {
							found = true
							break
						}
					}
					if found {
						break
					}
					fmt.Fprintf(c.stderr, "adj inbox: OPTION %q is not one of the options of record %d\n", answer, rec.ID)
				}
			} else if action == "t" {
				answer, ok = c.inboxRequired(in, "Decision text: ", "Decision text")
				if !ok {
					break walk
				}
			}
			why, ok := c.inboxRequired(in, "Rationale: ", "Rationale")
			if !ok {
				break walk
			}
			var err error
			switch action {
			case "d", "t":
				_, err = commands.Decide(st, rec.ID, commands.DecideInput{
					Option: answer, HasOption: action == "d", Text: answer, HasText: action == "t", Why: why, DecidedBy: by,
				}, c.env.Now())
			case "r":
				_, err = commands.Reject(st, rec.ID, commands.ReasonInput{Why: why, DecidedBy: by}, c.env.Now())
			case "w":
				_, err = commands.Withdraw(st, rec.ID, commands.ReasonInput{Why: why, DecidedBy: by}, c.env.Now())
			}
			if err != nil {
				c.fail("inbox", err)
				failed++
			} else {
				switch action {
				case "d", "t":
					decided++
				case "r":
					rejected++
				case "w":
					withdrawn++
				}
			}
			continue walk
		}
	}
	if in.err != nil {
		return c.fail("inbox", in.err)
	}
	if _, err := fmt.Fprintf(c.stdout, "\n%d decided, %d rejected, %d withdrawn, %d skipped, %d failed\n", decided, rejected, withdrawn, skipped, failed); err != nil {
		return c.fail("inbox", err)
	}
	if failed > 0 {
		return exitFail
	}
	return exitOK
}
