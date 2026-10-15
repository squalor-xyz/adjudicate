// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"github.com/squalor-xyz/adjudicate/internal/lint"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// Lint checks every record file of st and returns the findings in report
// order (lint.Run); file names in them are relative to root, the repo root.
// It works on a store that List refuses (C61), so only a store directory that
// cannot be scanned is an error.
func Lint(st *store.Store, root string) ([]lint.Finding, error) {
	return lint.Run(st, root)
}
