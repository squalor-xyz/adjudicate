// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const importADRUsage = `usage: adj import-adr DIR [--tag TAG]... [--by NAME] [--dry-run] [--root DIR]

Create one record for each ADR (architecture decision record) in the folder
DIR, so a team that keeps ADRs can move to adj with its history. DIR holds
files in the format adr-tools writes (Michael Nygard's template: "# N. Title",
a Date line, then Status, Context, Decision and Consequences sections) or MADR
(optional YAML front matter or a Status line, then Context and Problem
Statement, Considered Options and Decision Outcome). Only files whose names
start with a number, such as 0007-use-postgres.md or adr-0007-use-postgres.md,
are read, and only in DIR itself. The ADR files are never changed.

The status word gives the record's status: accepted is decided (the Decision
section, or the MADR chosen option, is the decision), proposed is open (its
decision is the recommendation), rejected is rejected, deprecated is
withdrawn, and superseded is superseded when the ADR that replaced it is in
the same run and is accepted or itself superseded. If it is not, or the links
make a loop, the record is decided and a warning says why. The rationale is the
MADR justification, else the Consequences section, else the Context; for a
rejected or withdrawn ADR the justification, then the Decision text, then the
Consequences, then the Context. The ADR's date is the record's created and
decided_at date; an ADR with no date, a date that is not YYYY-MM-DD or not a
real date, or a date after the run date gets the run date and a warning. Each
record has one import history entry ("from FILE"), and the refs name the file
(its path from the repo root) and the ADR number (ADR-0007). The records take
the next free IDs, in ADR number order.

Every file is checked first. If any file has no title, no status adj knows, no
decision (when accepted) or no rationale, or cannot be read, nothing is
written and each problem is listed. If a write fails partway, the records
created so far stay in the store and are printed, and the error lists the
supersede links not yet written as adj supersede commands. DIR must be inside
the repo.

A file is already imported when a record has an import history entry for it;
it is skipped, so running the import again adds only new files. A record asked
by hand with the file as a ref does not count. A skipped file is read again
only for its "Superseded by" line: if its record is still decided and the file
now says it is superseded by an ADR whose record is decided, a warning gives
the command to run, adj supersede OLD --by NEW; the import never writes that
link for a record it did not create. A file that is a symbolic link is not
read; a warning says so.

Prints one line per file, "FILE -> ID (STATUS)", or "FILE: already imported as
ID" for a file skipped. Warnings go to stderr. A folder with no ADR files
prints nothing and succeeds.

  --tag TAG      a tag for every record; repeat for several
  --by NAME      decided_by of the answered records; default:
                 $ADJ_DECIDED_BY, else the ADR's deciders field (MADR), else
                 left blank
  --dry-run      write nothing; print what would be created
  --root DIR     the repo root; default: found from the working directory
                 upwards (the nearest .adjudicate.json)
`

func (c *invocation) runImportADR(args []string) int {
	const name = "import-adr"
	fs, root := newFlagSet(name)
	var tags stringList
	fs.Var(&tags, "tag", "")
	by := fs.String("by", "", "")
	dryRun := fs.Bool("dry-run", false, "")

	pos, code, done := c.parse(name, importADRUsage, fs, args)
	if done {
		return code
	}
	switch len(pos) {
	case 0:
		return c.usageError(name, importADRUsage, errors.New("DIR is required"))
	case 1:
	default:
		return c.usageError(name, importADRUsage, fmt.Errorf("want one DIR, got %d arguments", len(pos)))
	}
	if strings.TrimSpace(pos[0]) == "" {
		return c.usageError(name, importADRUsage, errors.New("DIR must not be blank"))
	}
	in := commands.ImportADRInput{Tags: tags, DecidedBy: *by, DryRun: *dryRun}
	if in.DecidedBy == "" {
		in.DecidedBy = c.env.Getenv("ADJ_DECIDED_BY")
	}
	if err := in.Check(); err != nil {
		return c.commandError(name, importADRUsage, err)
	}

	repo, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError(name, err)
	}
	st, err := store.Open(repo)
	if err != nil {
		return c.fail(name, err)
	}
	// Both paths are made absolute, so the records can name each file by its
	// path from the root.
	if in.Root, err = c.absolute(repo); err != nil {
		return c.fail(name, err)
	}
	if in.Dir, err = c.absolute(pos[0]); err != nil {
		return c.fail(name, err)
	}

	res, err := commands.ImportADR(st, in, c.env.Now())
	// Say what was done even if a later file failed, and the warnings either way.
	for _, a := range res.ADRs {
		switch a.Action {
		case commands.ImportSkipped:
			fmt.Fprintf(c.stdout, "%s: already imported as %d\n", a.File, a.ID)
		case commands.ImportWouldCreate:
			fmt.Fprintf(c.stdout, "%s -> would create (%s)\n", a.File, a.Status)
		default:
			fmt.Fprintf(c.stdout, "%s -> %d (%s)\n", a.File, a.ID, a.Status)
		}
	}
	for _, w := range res.Warnings {
		fmt.Fprintf(c.stderr, "adj %s: warning: %s: %s\n", name, w.File, w.Message)
	}
	if err != nil {
		return c.commandError(name, importADRUsage, err)
	}
	return exitOK
}

// absolute makes path absolute against the working directory.
func (c *invocation) absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	cwd, err := c.env.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, path), nil
}
