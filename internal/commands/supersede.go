// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"fmt"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// recordStore is the part of the store that supersede uses; tests wrap the
// real store to make a write fail.
type recordStore interface {
	Get(id int) (model.Record, error)
	Update(r model.Record) error
}

// CheckSupersede reports a *UsageError if the two IDs cannot name a supersede
// whatever the records are: a record cannot replace itself (C134). The CLI runs it
// before the store is looked for.
func CheckSupersede(oldID, newID int) error {
	if oldID == newID {
		return usagef("OLD and --by must be different records")
	}
	return nil
}

// notDecidedError is a supersede refused because a record is not decided. Its
// message names the record's status and what a decided record can do. Unwrap
// keeps the model's error, so errors.Is(err, model.ErrInvalidTransition)
// still holds.
type notDecidedError struct {
	id     int
	status model.Status
	what   string // "be superseded" or "supersede another"
}

func (e *notDecidedError) Error() string {
	return fmt.Sprintf("record %d is %s; only a decided record can %s", e.id, e.status, e.what)
}

func (e *notDecidedError) Unwrap() error { return model.ErrInvalidTransition }

// Supersede replaces the decided record oldID with the decided record newID,
// dated now. The old record becomes superseded, with superseded_by newID and
// one supersede history entry ("by <newID>"); the new record stays decided,
// lists oldID in supersedes and gets one supersede history entry ("supersedes
// <oldID>"). It returns the old record as stored. Equal IDs are a *UsageError;
// a missing record, or one that is not decided, is an ordinary error, and the
// old record is checked first. Both changes are made and validated in memory
// before anything is written, so a refused supersede writes nothing (C136).
func Supersede(st *store.Store, oldID, newID int, now time.Time) (model.Record, error) {
	return supersede(st, oldID, newID, now)
}

// supersede is Supersede on any recordStore.
//
// The two files cannot be written together. The old record is written first
// (C136): if the second write then fails, the old record is put back, and if
// that fails too the store is left with the old record superseded and the new
// one still decided but without its link. The decision in force is then still
// the new record (check and context agree), never two decided records that
// contradict each other. The error says which state was left.
func supersede(st recordStore, oldID, newID int, now time.Time) (model.Record, error) {
	if err := CheckSupersede(oldID, newID); err != nil {
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
		return model.Record{}, &notDecidedError{oldID, oldRec.Status, "be superseded"}
	}
	if newRec.Status != model.StatusDecided {
		return model.Record{}, &notDecidedError{newID, newRec.Status, "supersede another"}
	}

	date := now.Format(time.DateOnly)
	original := oldRec
	if err := model.Supersede(&oldRec, newID, date); err != nil {
		return model.Record{}, err
	}
	if err := model.LinkSupersedes(&newRec, oldID, date); err != nil {
		return model.Record{}, err
	}

	if err := st.Update(oldRec); err != nil {
		return model.Record{}, fmt.Errorf("record %d: %w", oldID, err)
	}
	if err := st.Update(newRec); err != nil {
		if rerr := st.Update(original); rerr != nil {
			return model.Record{}, fmt.Errorf("record %d: %w; putting record %d back also failed (%v): "+
				"record %d is left superseded by %d and record %d has no link back, so the store is inconsistent",
				newID, err, oldID, rerr, oldID, newID, newID)
		}
		return model.Record{}, fmt.Errorf("record %d: %w (record %d was put back, nothing changed)", newID, err, oldID)
	}
	return oldRec, nil
}
