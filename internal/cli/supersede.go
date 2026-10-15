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

const supersedeUsage = `usage: adj supersede OLD --by NEW [--root DIR]

Replace a decided record with another decided record: OLD becomes superseded
and points to NEW, and NEW lists OLD as superseded by it. OLD stays readable
with its history but leaves the context. Both records must be decided and
different. Prints the ID of OLD.

  --by NEW       the ID of the decided record that replaces OLD (required)
  --root DIR     the repo root; default: found from the working directory
                 upwards (the nearest .adjudicate.json)
`

func (c *invocation) runSupersede(args []string) int {
	fs, root := newFlagSet("supersede")
	by := fs.String("by", "", "")

	pos, code, done := c.parse("supersede", supersedeUsage, fs, args)
	if done {
		return code
	}
	oldID, err := idArg(pos)
	if err != nil {
		return c.usageError("supersede", supersedeUsage, err)
	}
	if len(pos) > 1 {
		return c.usageError("supersede", supersedeUsage, fmt.Errorf("want one ID and --by, got %d arguments", len(pos)))
	}
	if !fsSet(fs, "by") {
		return c.usageError("supersede", supersedeUsage, fmt.Errorf("--by is required"))
	}
	newID, err := strconv.Atoi(strings.TrimSpace(*by))
	if err != nil || newID <= 0 {
		return c.usageError("supersede", supersedeUsage, fmt.Errorf("--by must be a positive integer, got %q", *by))
	}
	return c.answerRecord("supersede", supersedeUsage, *root, commands.CheckSupersede(oldID, newID), func(st *store.Store) (model.Record, error) {
		return commands.Supersede(st, oldID, newID, c.env.Now())
	})
}
