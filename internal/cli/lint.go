// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/lint"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const lintUsage = `usage: adj lint [flags]

Check the store for integrity problems. Each finding names the record, its file
and the check that found it; the checks are:

  parse            the file cannot be read or decoded
  fields           a record breaks the model's rules (a required field is
                   missing, a date is bad, the status is unknown)
  id-mismatch      the record's id is not the ID in its file name
  dangling-link    supersedes or superseded_by names an ID with no record file
  supersede-state  superseded_by is set on a record that is not superseded
  supersede-cycle  the record is in a loop of supersede links
  key-conflict     another decided record has the same key
  decided-by       a decided record has no decided_by (a warning)

Lint reads the record files itself, so it works on a store that other commands
refuse to load. Text output is one line per finding, "FILE: record ID:
SEVERITY: CHECK: MESSAGE", then a count; a clean store prints nothing. With
--json, print {"errors", "warnings", "findings"} (the shape is in
docs/json.md).

Exit codes: 0 if there is no error (warnings alone exit 0); 1 if there is at
least one error, or lint could not run (no repo, an unreadable store
directory); 2 for a usage error.

  --json       print JSON instead of text
  --root DIR   the repo root; default: found from the working directory
               upwards (the nearest .adjudicate.json)
`

func (c *invocation) runLint(args []string) int {
	fs, root := newFlagSet("lint")
	asJSON := fs.Bool("json", false, "")

	pos, code, done := c.parse("lint", lintUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("lint", lintUsage, fmt.Errorf("lint takes no arguments, got %q", pos[0]))
	}
	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("lint", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("lint", err)
	}
	findings, err := commands.Lint(st, dir)
	if err != nil {
		return c.commandError("lint", lintUsage, err)
	}
	if *asJSON {
		err = render.LintJSON(c.stdout, findings)
	} else {
		err = render.LintText(c.stdout, findings)
	}
	if err != nil {
		return c.fail("lint", err)
	}
	if errs, _ := lint.Counts(findings); errs > 0 {
		return exitFail
	}
	return exitOK
}
