// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"fmt"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/store"
)

// Renumber resolves every ID that more than one record file has (store.
// Renumber, C26), dating the history entries now. With keepLinks, links to a
// colliding ID are left as they are and listed in the result; without it such
// a link is an error and nothing changes (C147). With no collision it does
// nothing.
func Renumber(st *store.Store, now time.Time, keepLinks bool) (store.RenumberResult, error) {
	res, err := st.Renumber(now.Format(time.DateOnly), keepLinks)
	if errors.Is(err, store.ErrAmbiguousLinks) {
		err = fmt.Errorf("%w; check them, then run again with --keep-links to leave them as they are", err)
	}
	return res, err
}
