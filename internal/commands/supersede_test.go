// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// twoDecided makes a store with records 1 and 2, both decided.
func twoDecided(t *testing.T) (*store.Store, time.Time) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 4, 23, 30, 0, 0, time.UTC)
	for _, title := range []string{"Old store", "New store"} {
		rec, err := Ask(st, AskInput{Title: title}, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Decide(st, rec.ID, DecideInput{Text: title + " it is", HasText: true, Why: "because"}, now); err != nil {
			t.Fatal(err)
		}
	}
	return st, now
}

func get(t *testing.T, st *store.Store, id int) model.Record {
	t.Helper()
	r, err := st.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// flakyStore is a store whose Update fails when fail says so; calls counts the
// Updates of each record, so a test can fail the second write only.
type flakyStore struct {
	*store.Store
	calls map[int]int
	fail  func(id, call int) bool
}

func (f *flakyStore) Update(r model.Record) error {
	f.calls[r.ID]++
	if f.fail(r.ID, f.calls[r.ID]) {
		return errors.New("disk full")
	}
	return f.Store.Update(r)
}

func newFlaky(st *store.Store, fail func(id, call int) bool) *flakyStore {
	return &flakyStore{Store: st, calls: map[int]int{}, fail: fail}
}

// If the write of the new record fails after the old one was written, the old
// record is put back: both records are as they were and the error says so.
func TestSupersedeRestoresTheOldRecordWhenTheNewOneCannotBeWritten(t *testing.T) {
	st, now := twoDecided(t)
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)

	flaky := newFlaky(st, func(id, call int) bool { return id == 2 })
	_, err := supersede(flaky, 1, 2, now)
	if err == nil || !strings.Contains(err.Error(), "record 2") || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want one naming record 2 and the cause", err)
	}
	if strings.Contains(err.Error(), "inconsistent") {
		t.Errorf("err = %v: the old record was restored, so nothing is inconsistent", err)
	}
	if got := get(t, st, 1); !reflect.DeepEqual(got, oldBefore) {
		t.Errorf("old record not restored:\n got %+v\nwant %+v", got, oldBefore)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got, newBefore) {
		t.Errorf("new record changed:\n got %+v\nwant %+v", got, newBefore)
	}
}

// If the restore fails too, the error says which record is left changed and
// how, and the old record stays superseded: the decision in force is still the
// new one, never two decided records that contradict each other.
func TestSupersedeSaysWhatIsLeftWhenTheRestoreFailsToo(t *testing.T) {
	st, now := twoDecided(t)
	newBefore := get(t, st, 2)

	// Record 1 is written once to supersede it and once to restore it: fail the
	// second.
	flaky := newFlaky(st, func(id, call int) bool { return id == 2 || (id == 1 && call == 2) })
	_, err := supersede(flaky, 1, 2, now)
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"record 2", "disk full", "record 1", "left superseded", "inconsistent"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
	if got := get(t, st, 1); got.Status != model.StatusSuperseded || got.SupersededBy != 2 {
		t.Errorf("old record = %+v, want superseded by 2", got)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got, newBefore) {
		t.Errorf("new record changed: %+v", got)
	}
}

// A failure of the first write leaves both records alone, and the new record
// is never written.
func TestSupersedeWritesNothingMoreWhenTheOldRecordCannotBeWritten(t *testing.T) {
	st, now := twoDecided(t)
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)

	flaky := newFlaky(st, func(id, call int) bool { return id == 1 })
	if _, err := supersede(flaky, 1, 2, now); err == nil || !strings.Contains(err.Error(), "record 1") {
		t.Fatalf("err = %v, want one naming record 1", err)
	}
	if flaky.calls[2] != 0 {
		t.Errorf("record 2 was written %d times, want 0", flaky.calls[2])
	}
	if got := get(t, st, 1); !reflect.DeepEqual(got, oldBefore) {
		t.Errorf("old record changed: %+v", got)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got, newBefore) {
		t.Errorf("new record changed: %+v", got)
	}
}

// Both records are checked in memory before either is written, so a new
// record that cannot take the link stops the command with no write at all.
func TestSupersedeWritesNothingWhenEitherRecordIsRefused(t *testing.T) {
	st, now := twoDecided(t)
	if _, err := Reject(st, mustAsk(t, st, "Third", now), ReasonInput{Why: "no"}, now); err != nil {
		t.Fatal(err)
	}
	flaky := newFlaky(st, func(id, call int) bool { return false })
	for _, ids := range [][2]int{{1, 3}, {3, 2}, {1, 1}, {1, 9}, {9, 2}} {
		if _, err := supersede(flaky, ids[0], ids[1], now); err == nil {
			t.Errorf("supersede(%d, %d): no error", ids[0], ids[1])
		}
	}
	if len(flaky.calls) != 0 {
		t.Errorf("Update was called for %v, want no write", flaky.calls)
	}
}

func mustAsk(t *testing.T, st *store.Store, title string, now time.Time) int {
	t.Helper()
	rec, err := Ask(st, AskInput{Title: title}, now)
	if err != nil {
		t.Fatal(err)
	}
	return rec.ID
}

// The two refusals are ordinary failures that match model.ErrInvalidTransition,
// the same family as decide's, except old == new, which is a usage error.
func TestSupersedeErrorKinds(t *testing.T) {
	st, now := twoDecided(t)
	open := mustAsk(t, st, "Open one", now)

	_, err := Supersede(st, open, 2, now)
	if !errors.Is(err, model.ErrInvalidTransition) {
		t.Errorf("open old: err = %v, want one matching ErrInvalidTransition", err)
	}
	_, err = Supersede(st, 1, open, now)
	if !errors.Is(err, model.ErrInvalidTransition) {
		t.Errorf("open new: err = %v, want one matching ErrInvalidTransition", err)
	}
	var ue *UsageError
	if _, err = Supersede(st, 1, 1, now); !errors.As(err, &ue) {
		t.Errorf("old == new: err = %v, want a *UsageError", err)
	}
	if _, err = Supersede(st, 1, 99, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown new: err = %v, want one matching store.ErrNotFound", err)
	}
	// The date is the clock's, formatted in its own zone as decide's is.
	rec, err := Supersede(st, 1, 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if last := rec.History[len(rec.History)-1]; last.Date != "2026-03-04" || last.Note != "by 2" {
		t.Errorf("last history entry = %+v, want dated 2026-03-04 with note \"by 2\"", last)
	}
}
