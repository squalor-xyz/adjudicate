// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// decidedRecord is a decided record for the narrow tests.
func decidedRecord(id int, title string) model.Record {
	r := question(id, title)
	r.Status, r.Decision, r.Rationale, r.DecidedAt = model.StatusDecided, "yes", "because", "2026-10-02"
	return r
}

// narrowCollision seeds two files with ID 1 and a record 2 that narrows ID 1,
// and record 3 that is narrowed by it, so both kinds of link name the colliding
// ID.
func narrowCollision(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", decidedRecord(1, "A"))
	b := decidedRecord(1, "B")
	b.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", b)
	narrower := decidedRecord(2, "Narrower")
	narrower.Narrows = []int{1}
	seed(t, dir, "0002-narrower.md", narrower)
	narrowed := decidedRecord(3, "Narrowed")
	narrowed.NarrowedBy = []model.Narrowing{{ID: 1, Scope: "s"}}
	seed(t, dir, "0003-narrowed.md", narrowed)
	return root
}

// Narrow links to a colliding ID are ambiguous like supersede links (C147):
// Renumber names each, in file order, and changes nothing unless the caller
// accepts them.
func TestRenumberRefusesNarrowLinksToACollidingID(t *testing.T) {
	root := narrowCollision(t)
	s := mustOpen(t, root)
	before := tree(t, root)

	_, err := s.Renumber(renumberDate, false)
	if !errors.Is(err, ErrAmbiguousLinks) {
		t.Fatalf("err = %v, want ErrAmbiguousLinks", err)
	}
	for _, want := range []string{"0002-narrower.md narrows 1", "0003-narrowed.md narrowed_by 1", "0001-a.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("a refused Renumber changed the store")
	}

	got, err := s.Renumber(renumberDate, true)
	if err != nil {
		t.Fatal(err)
	}
	wantLinks := []Link{
		{File: "0002-narrower.md", ID: 2, Field: "narrows", To: 1, KeptName: "0001-a.md"},
		{File: "0003-narrowed.md", ID: 3, Field: "narrowed_by", To: 1, KeptName: "0001-a.md"},
	}
	if !reflect.DeepEqual(got.Links, wantLinks) {
		t.Errorf("links = %+v, want %+v", got.Links, wantLinks)
	}
	if r := mustRead(t, filepath.Join(s.Dir(), "0003-narrowed.md")); !reflect.DeepEqual(r.NarrowedBy, []model.Narrowing{{ID: 1, Scope: "s"}}) {
		t.Errorf("the link was rewritten: %+v", r.NarrowedBy)
	}
}

// A new ID is never one that a narrow link names without a file (C155): the
// moved record would otherwise become the target of that link.
func TestRenumberSkipsIDsNamedByNarrowLinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	b := question(1, "B")
	b.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", b)
	x := decidedRecord(2, "X")
	x.NarrowedBy = []model.Narrowing{{ID: 3, Scope: "s"}}
	seed(t, dir, "0002-x.md", x)
	y := decidedRecord(4, "Y")
	y.Narrows = []int{5}
	seed(t, dir, "0004-y.md", y)
	s := mustOpen(t, root)

	got, err := s.Renumber(renumberDate, false)
	if err != nil || len(got.Moves) != 1 || got.Moves[0].NewID != 6 {
		t.Errorf("Renumber = %+v, %v; want the move to ID 6 (3 and 5 are named by narrow links)", got.Moves, err)
	}
}

// Create does not take an ID that a narrow link names without a file.
func TestCreateSkipsIDsNamedByNarrowLinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	x := decidedRecord(1, "X")
	x.NarrowedBy = []model.Narrowing{{ID: 2, Scope: "s"}}
	seed(t, dir, "0001-x.md", x)
	s := mustOpen(t, root)

	got, err := s.Create(question(0, "New"))
	if err != nil || got.ID != 3 {
		t.Errorf("Create = %+v, %v; want ID 3 (2 is named by narrowed_by)", got, err)
	}
	if got := unlinkedID(model.Record{Narrows: []int{7}, NarrowedBy: []model.Narrowing{{ID: 9, Scope: "s"}}}); got != 10 {
		t.Errorf("unlinkedID = %d, want 10: past every narrow link", got)
	}
}

// A moved record that was narrowed after an interrupted renumber is still the
// copy of the stale file: the narrow links and entries that follow the renumber
// entry are like the supersede links and decide entries that may differ (C163).
func TestIsStaleCopyIgnoresNarrowingDoneSince(t *testing.T) {
	stale, moved := File{ID: 3, Name: "0003-foo.md"}, File{ID: 5, Name: "0005-foo.md"}
	staleRec := askedRecord(3)
	m := movedBy(t, staleRec, 5)
	m.Status, m.Decision, m.Rationale, m.DecidedAt = model.StatusDecided, "a", "because", "2026-11-03"
	m.History = append(m.History, model.HistoryEntry{Date: "2026-11-03", Event: model.EventDecide})
	if err := model.Narrow(&m, 9, "scope", "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if err := model.LinkNarrows(&m, 8, "2026-11-05"); err != nil {
		t.Fatal(err)
	}
	if !IsStaleCopy(stale, staleRec, moved, m) {
		t.Error("IsStaleCopy = false for a renumbered record that was decided and narrowed since")
	}
	// A narrow link that is already in the history before the renumber entry
	// is part of it: the stale record has it too, or it is another record.
	staleNarrowed := decidedRecord(3, "Foo")
	if err := model.Narrow(&staleNarrowed, 9, "scope", "2026-10-09"); err != nil {
		t.Fatal(err)
	}
	same := movedBy(t, staleNarrowed, 5)
	if !IsStaleCopy(stale, staleNarrowed, moved, same) {
		t.Error("IsStaleCopy = false for the interrupted move of a narrowed record")
	}
	other := same
	other.NarrowedBy = nil // the narrowed_by link is gone from the moved file
	if IsStaleCopy(stale, staleNarrowed, moved, other) {
		t.Error("IsStaleCopy = true for a moved record that lost the narrowed_by link of the stale one")
	}
}
