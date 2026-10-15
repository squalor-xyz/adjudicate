// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const renumberUsage = `usage: adj renumber [--keep-links] [--root DIR]

Resolve record files that share an ID (adj lint reports them as duplicate-id),
which two branches or two asks at once can produce. Of the files with one ID,
the record created first keeps it, and the file name breaks a tie. Each other
record is written to a new file with the next free ID (one past the highest in
the store) and the same name text, and its old file is removed; its history gets
one renumber entry noting the old ID. Prints one line per record moved,
"OLD-FILE -> NEW-FILE (id N -> M)". With no shared ID it prints nothing and
changes nothing.

Links (supersedes, superseded_by) to a shared ID cannot be told apart: renumber
never rewrites them or the notes in history. If a record links to a shared ID,
renumber lists the links and stops with nothing changed. Check them (edit a
file by hand if one meant the record that moves), then run again with
--keep-links, which leaves them as they are, so they name the record that kept
the ID.

Every record file must be readable, a record that shares an ID must have a
created date, and a record that moves must be valid with its new ID; if not,
renumber stops with nothing changed. A new ID is never one that a link names
without a file. If a run was interrupted after a new file was written but
before the old one was removed, the record exists twice: adj lint reports the
old file as renumber-leftover and renumber refuses to run until you delete that
file.

Exit codes: 0 on success, 1 if renumber stopped or failed, 2 for a usage error.

  --keep-links  leave links to a shared ID as they are, and list them on stderr
  --root DIR    the repo root; default: found from the working directory
                upwards (the nearest .adjudicate.json)
`

func (c *invocation) runRenumber(args []string) int {
	fs, root := newFlagSet("renumber")
	keepLinks := fs.Bool("keep-links", false, "")

	pos, code, done := c.parse("renumber", renumberUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("renumber", renumberUsage, fmt.Errorf("renumber takes no arguments, got %q", pos[0]))
	}
	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("renumber", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("renumber", err)
	}
	res, err := commands.Renumber(st, c.env.Now(), *keepLinks)
	// Say what was done even if a later move failed.
	for _, m := range res.Moves {
		fmt.Fprintf(c.stdout, "%s -> %s (id %d -> %d)\n", m.OldName, m.NewName, m.OldID, m.NewID)
	}
	if err != nil {
		return c.commandError("renumber", renumberUsage, err)
	}
	for _, l := range res.Links {
		fmt.Fprintf(c.stderr, "adj renumber: note: %s %s %d, which stays with %s\n", l.File, l.Field, l.To, l.KeptName)
	}
	return exitOK
}
