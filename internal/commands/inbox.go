// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"cmp"
	"slices"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/query"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// Inbox returns the open records of st, oldest first: by created date, then
// by ID. An empty inbox is an empty result, not an error. Store failures are
// the store's errors (C61).
func Inbox(st *store.Store) ([]model.Record, error) {
	recs, err := st.List()
	if err != nil {
		return nil, err
	}
	return oldestFirst(query.Select(recs, query.Filter{Status: model.StatusOpen})), nil
}

// oldestFirst returns a copy of recs ordered by created date (YYYY-MM-DD, so
// text order is date order), then by ID. The sort is stable, so records that
// share an ID (C60) keep the order given. recs is not changed.
func oldestFirst(recs []model.Record) []model.Record {
	out := slices.Clone(recs)
	slices.SortStableFunc(out, func(a, b model.Record) int {
		return cmp.Or(cmp.Compare(a.Created, b.Created), cmp.Compare(a.ID, b.ID))
	})
	return out
}
