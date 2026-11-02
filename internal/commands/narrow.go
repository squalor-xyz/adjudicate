// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// CheckNarrow reports a *UsageError if the arguments of a narrow cannot be
// right whatever the records are: a record cannot narrow itself, and the scope
// must be one line that is not blank (S51). The CLI runs it before the store is
// looked for.
func CheckNarrow(oldID, newID int, scope string) error {
	switch {
	case oldID == newID:
		return usagef("OLD and --by must be different records")
	case strings.TrimSpace(scope) == "":
		return usagef("--scope must not be blank")
	case strings.ContainsAny(scope, "\r\n"):
		return usagef("--scope must be one line")
	}
	return nil
}

// Narrow narrows the decided record oldID with the decided record newID, dated
// now. The old record stays decided and in force: it gets a narrowed_by link
// to newID with the scope (as typed) and one narrow history entry ("by <newID>:
// <scope>"). The new record stays decided, lists oldID in narrows and gets one
// narrow history entry ("narrows <oldID>"). It returns the old record as
// stored. Equal IDs and a bad scope are a *UsageError; a missing record, one
// that is not decided, or a pair that is already linked on both sides
// (model.ErrAlreadyNarrowed) is an ordinary error, and the old record is
// checked first. Both changes are made and validated in memory before anything
// is written, so a refused narrow writes nothing. Unlike supersede nothing
// leaves adj context (S51).
func Narrow(st *store.Store, oldID, newID int, scope string, now time.Time) (model.Record, error) {
	return narrow(st, oldID, newID, scope, now)
}

// narrow is Narrow on any recordStore.
//
// The two files cannot be written together. The old record is written first, as
// for supersede (C136): if the second write then fails, the old record is put
// back, and if that fails too the old record lists the narrowing and the new
// one has no link back. Both stay decided, so the decisions in force are the
// same; lint reports the one-sided link as narrow-asymmetric. The error says
// which state was left. Narrowing the same pair again completes that link (only
// NEW is written); a pair that links on both sides is refused.
func narrow(st recordStore, oldID, newID int, scope string, now time.Time) (model.Record, error) {
	if err := CheckNarrow(oldID, newID, scope); err != nil {
		return model.Record{}, err
	}
	oldRec, err := st.Get(oldID)
	if err != nil {
		return model.Record{}, err
	}
	newRec, err := st.Get(newID)
	if err != nil {
		return model.Record{}, err
	}
	if oldRec.Status != model.StatusDecided {
		return model.Record{}, &notDecidedError{oldID, oldRec.Status, "be narrowed"}
	}
	if newRec.Status != model.StatusDecided {
		return model.Record{}, &notDecidedError{newID, newRec.Status, "narrow another"}
	}

	date := now.Format(time.DateOnly)
	// OLD already lists NEW and NEW does not list OLD: the half link that a
	// failed restore leaves (see above). Only NEW is written, to add the link
	// back and its entry; OLD keeps its scope and its one entry, and the scope
	// given now is not used. When both sides link, model.Narrow refuses.
	oldLists := slices.ContainsFunc(oldRec.NarrowedBy, func(n model.Narrowing) bool { return n.ID == newID })
	if oldLists && !slices.Contains(newRec.Narrows, oldID) {
		if err := model.LinkNarrows(&newRec, oldID, date); err != nil {
			return model.Record{}, err
		}
		if err := st.Update(newRec); err != nil {
			return model.Record{}, fmt.Errorf("record %d: %w (nothing changed)", newID, err)
		}
		return oldRec, nil
	}
	original := oldRec
	if err := model.Narrow(&oldRec, newID, scope, date); err != nil {
		return model.Record{}, err
	}
	if err := model.LinkNarrows(&newRec, oldID, date); err != nil {
		return model.Record{}, err
	}

	if err := st.Update(oldRec); err != nil {
		return model.Record{}, fmt.Errorf("record %d: %w", oldID, err)
	}
	if err := st.Update(newRec); err != nil {
		if rerr := st.Update(original); rerr != nil {
			return model.Record{}, fmt.Errorf("record %d: %w; putting record %d back also failed (%v): "+
				"record %d is left narrowed by %d and record %d has no link back, so the store is inconsistent",
				newID, err, oldID, rerr, oldID, newID, newID)
		}
		return model.Record{}, fmt.Errorf("record %d: %w (record %d was put back, nothing changed)", newID, err, oldID)
	}
	return oldRec, nil
}
