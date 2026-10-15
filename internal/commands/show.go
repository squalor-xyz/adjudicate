// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// Show returns the record with id as stored. A missing ID or a duplicated one
// is the store's error (store.ErrNotFound, store.ErrDuplicateID): an ordinary
// failure, not a usage error.
func Show(st *store.Store, id int) (model.Record, error) {
	return st.Get(id)
}
