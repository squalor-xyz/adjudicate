// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const narrowUsage = `usage: adj narrow OLD --by NEW --scope TEXT [--root DIR]

Narrow a decided record with another decided record, for when NEW changes only
part of what OLD decided. OLD stays decided and in force, and adj context shows
the scope beside it; NEW lists OLD as the record it narrows. Use adj supersede
when NEW replaces all of OLD. Both records must be decided and different, and
the same pair can be linked only once (a second run exits 1), except that a run
after a failed write completes a link that only OLD has: only NEW is written,
and OLD keeps its scope. Owner only. Prints the ID of OLD.

  --by NEW       the ID of the decided record that narrows OLD (required)
  --scope TEXT   in one line, the part of OLD's decision that NEW changes
                 (required); kept as typed
  --root DIR     the repo root; default: found from the working directory
                 upwards (the nearest .adjudicate.json)
`

func (c *invocation) runNarrow(args []string) int {
	fs, root := newFlagSet("narrow")
	by := fs.String("by", "", "")
	scope := fs.String("scope", "", "")

	pos, code, done := c.parse("narrow", narrowUsage, fs, args)
	if done {
		return code
	}
	oldID, err := idArg(pos)
	if err != nil {
		return c.usageError("narrow", narrowUsage, err)
	}
	if len(pos) > 1 {
		return c.usageError("narrow", narrowUsage, fmt.Errorf("want one ID, --by and --scope, got %d arguments", len(pos)))
	}
	if !fsSet(fs, "by") {
		return c.usageError("narrow", narrowUsage, fmt.Errorf("--by is required"))
	}
	newID, err := strconv.Atoi(strings.TrimSpace(*by))
	if err != nil || newID <= 0 {
		return c.usageError("narrow", narrowUsage, fmt.Errorf("--by must be a positive integer, got %q", *by))
	}
	if !fsSet(fs, "scope") {
		return c.usageError("narrow", narrowUsage, fmt.Errorf("--scope is required"))
	}
	return c.answerRecord("narrow", narrowUsage, *root, commands.CheckNarrow(oldID, newID, *scope), func(st *store.Store) (model.Record, error) {
		return commands.Narrow(st, oldID, newID, *scope, c.env.Now())
	})
}
