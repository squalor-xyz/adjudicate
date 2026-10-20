// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// unknownHistoryFile is an open record whose history holds an event this
// version does not know, and a decide-like entry with a key it does not know;
// by is that key's value (S44).
func unknownHistoryFile(id int, by string) string {
	return `---
{"id":` + strconv.Itoa(id) + `,"title":"Foo","status":"open","created":"2026-10-02","history":[{"date":"2026-10-02","event":"ask"},{"date":"2026-10-03","event":"future-thing","note":"x"},{"date":"2026-10-04","event":"review","by":"` + by + `"}]}
---
`
}

// List and Get read a store whose records hold unknown events and keys.
func TestListAndGetAcceptUnknownHistoryEventsAndKeys(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-ok.md", question(1, "Ok"))
	writeFile(t, filepath.Join(dir, "0002-foo.md"), unknownHistoryFile(2, "someone"))
	s := mustOpen(t, root)

	recs, err := s.List()
	if err != nil || len(recs) != 2 {
		t.Fatalf("List() = %d records, %v; want 2 and no error", len(recs), err)
	}
	got, err := s.Get(2)
	if err != nil {
		t.Fatalf("Get(2) = %v, want nil", err)
	}
	if len(got.History) != 3 || got.History[1].Event != "future-thing" || !reflect.DeepEqual(got.History[2].UnknownKeys(), []string{"by"}) {
		t.Errorf("Get(2).History = %+v, want the unknown event and key kept", got.History)
	}
}

// Update writes the record back with the unknown entries as they were.
func TestUpdateKeepsUnknownHistoryEntries(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	writeFile(t, filepath.Join(dir, "0002-foo.md"), unknownHistoryFile(2, "someone"))
	s := mustOpen(t, root)
	rec, err := s.Get(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Withdraw(&rec, "no longer needed", "owner", "2026-10-05"); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(rec); err != nil {
		t.Fatal(err)
	}
	again, err := s.Get(2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.History[:3], rec.History[:3]) || again.History[2].UnknownKeys()[0] != "by" || len(again.History) != 4 {
		t.Errorf("history after Update = %+v, want the three old entries unchanged and a withdraw", again.History)
	}
}

// Two histories that differ only in an unknown key do not make one record the
// moved copy of the other (S44).
func TestStaleCopyComparesUnknownHistoryKeys(t *testing.T) {
	parse := func(id int, by string) model.Record {
		t.Helper()
		r, err := model.Parse("x.md", []byte(unknownHistoryFile(id, by)))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	stale, moved := File{ID: 3, Name: "0003-foo.md"}, File{ID: 5, Name: "0005-foo.md"}
	staleRec := parse(3, "one")
	movedRec := reread(t, movedBy(t, staleRec, 5))
	if !IsStaleCopy(stale, staleRec, moved, movedRec) {
		t.Fatal("the control: the moved copy, with its unknown key, is not a stale copy")
	}
	if IsStaleCopy(stale, parse(3, "two"), moved, movedRec) {
		t.Error("IsStaleCopy = true for histories that differ only in an unknown key")
	}
}
