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

const narrowScope = "only for structured questions"

// Narrow links both records, leaves both decided and returns OLD as stored.
func TestNarrowLinksBothRecords(t *testing.T) {
	st, now := twoDecided(t)
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)

	got, err := Narrow(st, 1, 2, narrowScope, now)
	if err != nil {
		t.Fatal(err)
	}
	wantOld := oldBefore
	wantOld.NarrowedBy = []model.Narrowing{{ID: 2, Scope: narrowScope}}
	wantOld.History = append(append([]model.HistoryEntry(nil), oldBefore.History...),
		model.HistoryEntry{Date: "2026-03-04", Event: model.EventNarrow, Note: "by 2: " + narrowScope})
	wantNew := newBefore
	wantNew.Narrows = []int{1}
	wantNew.History = append(append([]model.HistoryEntry(nil), newBefore.History...),
		model.HistoryEntry{Date: "2026-03-04", Event: model.EventNarrow, Note: "narrows 1"})

	if !reflect.DeepEqual(got, wantOld) {
		t.Errorf("returned record:\n got %+v\nwant %+v", got, wantOld)
	}
	if rec := get(t, st, 1); !reflect.DeepEqual(rec, wantOld) {
		t.Errorf("old record:\n got %+v\nwant %+v", rec, wantOld)
	}
	if rec := get(t, st, 2); !reflect.DeepEqual(rec, wantNew) {
		t.Errorf("new record:\n got %+v\nwant %+v", rec, wantNew)
	}
}

// If the write of NEW fails after OLD was written, OLD is put back: both
// records are as they were and the error says so (C136, as for supersede).
func TestNarrowRestoresTheOldRecordWhenTheNewOneCannotBeWritten(t *testing.T) {
	st, now := twoDecided(t)
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)

	flaky := newFlaky(st, func(id, call int) bool { return id == 2 })
	_, err := narrow(flaky, 1, 2, narrowScope, now)
	if err == nil || !strings.Contains(err.Error(), "record 2") || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want one naming record 2 and the cause", err)
	}
	if strings.Contains(err.Error(), "inconsistent") {
		t.Errorf("err = %v: the old record was restored, so nothing is inconsistent", err)
	}
	if flaky.calls[1] != 2 {
		t.Errorf("record 1 was written %d times, want 2 (the narrowing and the restore)", flaky.calls[1])
	}
	if got := get(t, st, 1); !reflect.DeepEqual(got, oldBefore) {
		t.Errorf("old record not restored:\n got %+v\nwant %+v", got, oldBefore)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got, newBefore) {
		t.Errorf("new record changed:\n got %+v\nwant %+v", got, newBefore)
	}
}

// If the restore fails too, the error says which record is left changed and
// how: OLD lists NEW, NEW has no link back, both are still decided.
func TestNarrowSaysWhatIsLeftWhenTheRestoreFailsToo(t *testing.T) {
	st, now := twoDecided(t)
	newBefore := get(t, st, 2)

	flaky := newFlaky(st, func(id, call int) bool { return id == 2 || (id == 1 && call == 2) })
	_, err := narrow(flaky, 1, 2, narrowScope, now)
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"record 2", "disk full", "record 1", "left narrowed by 2", "no link back", "inconsistent"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
	if got := get(t, st, 1); got.Status != model.StatusDecided || len(got.NarrowedBy) != 1 {
		t.Errorf("old record = %+v, want decided and narrowed by 2", got)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got, newBefore) {
		t.Errorf("new record changed: %+v", got)
	}
}

// A failure of the first write leaves both records alone and NEW is never
// written.
func TestNarrowWritesNothingMoreWhenTheOldRecordCannotBeWritten(t *testing.T) {
	st, now := twoDecided(t)
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)

	flaky := newFlaky(st, func(id, call int) bool { return id == 1 })
	if _, err := narrow(flaky, 1, 2, narrowScope, now); err == nil || !strings.Contains(err.Error(), "record 1") {
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

// Both records are checked in memory before either is written.
func TestNarrowWritesNothingWhenEitherRecordIsRefused(t *testing.T) {
	st, now := twoDecided(t)
	if _, err := Reject(st, mustAsk(t, st, "Third", now), ReasonInput{Why: "no"}, now); err != nil {
		t.Fatal(err)
	}
	flaky := newFlaky(st, func(id, call int) bool { return false })
	for _, ids := range [][2]int{{1, 3}, {3, 2}, {1, 1}, {1, 9}, {9, 2}} {
		if _, err := narrow(flaky, ids[0], ids[1], narrowScope, now); err == nil {
			t.Errorf("narrow(%d, %d): no error", ids[0], ids[1])
		}
	}
	if len(flaky.calls) != 0 {
		t.Errorf("Update was called for %v, want no write", flaky.calls)
	}
}

func TestNarrowErrorKinds(t *testing.T) {
	st, now := twoDecided(t)
	open := mustAsk(t, st, "Open one", now)

	_, err := Narrow(st, open, 2, narrowScope, now)
	if !errors.Is(err, model.ErrInvalidTransition) || !strings.Contains(err.Error(), "record 3 is open; only a decided record can be narrowed") {
		t.Errorf("open old: err = %v, want ErrInvalidTransition naming the status", err)
	}
	_, err = Narrow(st, 1, open, narrowScope, now)
	if !errors.Is(err, model.ErrInvalidTransition) || !strings.Contains(err.Error(), "record 3 is open; only a decided record can narrow another") {
		t.Errorf("open new: err = %v, want ErrInvalidTransition naming the status", err)
	}
	var ue *UsageError
	if _, err = Narrow(st, 1, 1, narrowScope, now); !errors.As(err, &ue) {
		t.Errorf("old == new: err = %v, want a *UsageError", err)
	}
	for _, scope := range []string{"", "  ", "a\nb"} {
		if _, err = Narrow(st, 1, 2, scope, now); !errors.As(err, &ue) {
			t.Errorf("scope %q: err = %v, want a *UsageError", scope, err)
		}
	}
	if _, err = Narrow(st, 1, 99, narrowScope, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown new: err = %v, want one matching store.ErrNotFound", err)
	}
	if _, err = Narrow(st, 99, 2, narrowScope, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown old: err = %v, want one matching store.ErrNotFound", err)
	}
}

// The same pair a second time is refused, writes nothing and adds no link (C32).
func TestNarrowTheSamePairTwiceIsRefusedAndWritesNothing(t *testing.T) {
	st, now := twoDecided(t)
	if _, err := Narrow(st, 1, 2, narrowScope, now); err != nil {
		t.Fatal(err)
	}
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)

	flaky := newFlaky(st, func(id, call int) bool { return false })
	_, err := narrow(flaky, 1, 2, "another scope", now)
	if !errors.Is(err, model.ErrAlreadyNarrowed) || !strings.Contains(err.Error(), "record 1 is already narrowed by record 2") {
		t.Fatalf("err = %v, want ErrAlreadyNarrowed naming both records", err)
	}
	if len(flaky.calls) != 0 {
		t.Errorf("Update was called for %v, want no write", flaky.calls)
	}
	if got := get(t, st, 1); !reflect.DeepEqual(got, oldBefore) {
		t.Errorf("old record changed: %+v", got)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got, newBefore) {
		t.Errorf("new record changed: %+v", got)
	}
}

// A record may be narrowed by several records, and one record may narrow
// several.
func TestNarrowManyToMany(t *testing.T) {
	st, now := twoDecided(t)
	third := mustAsk(t, st, "Third", now)
	if _, err := Decide(st, third, DecideInput{Text: "Third it is", HasText: true, Why: "because"}, now); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		old, by int
		scope   string
	}{{1, 2, "scope a"}, {1, 3, "scope b"}, {2, 3, "scope c"}} {
		if _, err := Narrow(st, c.old, c.by, c.scope, now); err != nil {
			t.Fatal(err)
		}
	}
	if got := get(t, st, 1).NarrowedBy; !reflect.DeepEqual(got, []model.Narrowing{{ID: 2, Scope: "scope a"}, {ID: 3, Scope: "scope b"}}) {
		t.Errorf("record 1 narrowed_by = %+v", got)
	}
	if got := get(t, st, 3).Narrows; !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("record 3 narrows = %v, want [1 2]", got)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got.Narrows, []int{1}) || len(got.NarrowedBy) != 1 {
		t.Errorf("record 2 = %+v, want it narrowing 1 and narrowed by 3", got)
	}
}

// invalidNewStore serves record 2 with a title the model refuses, as a file
// edited by hand outside the store's own reads could be: the link cannot be
// added to it.
type invalidNewStore struct{ *flakyStore }

func (s invalidNewStore) Get(id int) (model.Record, error) {
	r, err := s.flakyStore.Get(id)
	if id == 2 {
		r.Title = "two\nlines"
	}
	return r, err
}

// NEW is changed in memory before OLD is written, so a NEW that cannot take the
// link stops the command with no write at all.
func TestNarrowValidatesTheNewRecordBeforeWritingTheOldOne(t *testing.T) {
	st, now := twoDecided(t)
	oldBefore := get(t, st, 1)
	flaky := newFlaky(st, func(id, call int) bool { return false })
	_, err := narrow(invalidNewStore{flaky}, 1, 2, narrowScope, now)
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("err = %v, want one matching ErrInvalid", err)
	}
	if len(flaky.calls) != 0 {
		t.Errorf("Update was called for %v, want no write", flaky.calls)
	}
	if got := get(t, st, 1); !reflect.DeepEqual(got, oldBefore) {
		t.Errorf("old record changed: %+v", got)
	}
}

// halfLinked leaves the store as a narrow whose NEW write and whose restore both
// failed: record 1 lists record 2, record 2 has no link back.
func halfLinked(t *testing.T) (*store.Store, time.Time) {
	t.Helper()
	st, now := twoDecided(t)
	flaky := newFlaky(st, func(id, call int) bool { return id == 2 || (id == 1 && call == 2) })
	if _, err := narrow(flaky, 1, 2, narrowScope, now); err == nil || !strings.Contains(err.Error(), "inconsistent") {
		t.Fatalf("setup: err = %v, want the inconsistent-store error", err)
	}
	if got := get(t, st, 1); len(got.NarrowedBy) != 1 || len(get(t, st, 2).Narrows) != 0 {
		t.Fatalf("setup: not half linked: %+v", got)
	}
	return st, now
}

// Narrowing again completes the half link that a failed restore left: OLD is not
// written (it keeps its scope and its one entry), NEW gets the link and its
// entry, and the command succeeds with OLD's ID (S51, C32).
func TestNarrowCompletesAHalfLinkWithoutTouchingTheOldRecord(t *testing.T) {
	st, now := halfLinked(t)
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)
	later := now.AddDate(0, 0, 1)

	flaky := newFlaky(st, func(id, call int) bool { return false })
	got, err := narrow(flaky, 1, 2, "a scope that is not used", later)
	if err != nil {
		t.Fatalf("err = %v, want the half link completed", err)
	}
	if flaky.calls[1] != 0 || flaky.calls[2] != 1 {
		t.Errorf("writes = %v, want record 2 once and record 1 never", flaky.calls)
	}
	if !reflect.DeepEqual(got, oldBefore) {
		t.Errorf("returned record:\n got %+v\nwant OLD as stored %+v", got, oldBefore)
	}
	if rec := get(t, st, 1); !reflect.DeepEqual(rec, oldBefore) || rec.NarrowedBy[0].Scope != narrowScope || len(rec.History) != len(oldBefore.History) {
		t.Errorf("old record changed:\n got %+v\nwant %+v", rec, oldBefore)
	}
	wantNew := newBefore
	wantNew.Narrows = []int{1}
	wantNew.History = append(append([]model.HistoryEntry(nil), newBefore.History...),
		model.HistoryEntry{Date: "2026-03-05", Event: model.EventNarrow, Note: "narrows 1"})
	if rec := get(t, st, 2); !reflect.DeepEqual(rec, wantNew) {
		t.Errorf("new record:\n got %+v\nwant %+v", rec, wantNew)
	}

	// Now both sides link, so the same pair again is refused and writes nothing.
	again := newFlaky(st, func(id, call int) bool { return false })
	if _, err := narrow(again, 1, 2, narrowScope, later); !errors.Is(err, model.ErrAlreadyNarrowed) {
		t.Errorf("third narrow: err = %v, want ErrAlreadyNarrowed", err)
	}
	if len(again.calls) != 0 {
		t.Errorf("third narrow wrote %v", again.calls)
	}
}

// Completing the link writes only NEW, so a failed write leaves the half link
// as it was and OLD needs no restore.
func TestNarrowCompletingAHalfLinkFailsCleanlyWhenNewCannotBeWritten(t *testing.T) {
	st, now := halfLinked(t)
	oldBefore, newBefore := get(t, st, 1), get(t, st, 2)

	flaky := newFlaky(st, func(id, call int) bool { return id == 2 })
	_, err := narrow(flaky, 1, 2, narrowScope, now)
	if err == nil || !strings.Contains(err.Error(), "record 2") || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("err = %v, want one naming record 2 and the cause", err)
	}
	if flaky.calls[1] != 0 {
		t.Errorf("record 1 was written %d times, want 0", flaky.calls[1])
	}
	if got := get(t, st, 1); !reflect.DeepEqual(got, oldBefore) {
		t.Errorf("old record changed: %+v", got)
	}
	if got := get(t, st, 2); !reflect.DeepEqual(got, newBefore) {
		t.Errorf("new record changed: %+v", got)
	}
}
