// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/query"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// CheckInput is the raw input of adj check: the query as typed.
type CheckInput struct {
	Query string
}

// Check reports a *UsageError if the query is blank (C109). The CLI runs
// it before the store is looked for.
func (in CheckInput) Check() error {
	if strings.TrimSpace(in.Query) == "" {
		return usagef("QUERY must not be blank")
	}
	return nil
}

// Check searches the records of st for the query and reports whether it has
// been decided, is still open, or is not found (query.Search). Bad input is a
// *UsageError and nothing is read. Store failures are the store's errors
// (C61), so a store that cannot be read is never reported as "not found".
func Check(st *store.Store, in CheckInput) (query.Result, error) {
	if err := in.Check(); err != nil {
		return query.Result{}, err
	}
	recs, err := st.List()
	if err != nil {
		return query.Result{}, err
	}
	return query.Search(recs, in.Query), nil
}
