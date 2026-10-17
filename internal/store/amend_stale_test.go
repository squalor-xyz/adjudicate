// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

func strp(s string) *string { return &s }

// amendDay is when the amends in these tests happen: after the renumber.
const amendDay = "2026-11-05"

// amended applies one model.Amend to r (a copy) and returns the result, as the
// file system holds it: written and read back, so no pointer is shared with r.
func amended(t *testing.T, r model.Record, a model.Amendment) model.Record {
	t.Helper()
	r.Options = append([]model.Option(nil), r.Options...)
	if err := model.Amend(&r, a, amendDay); err != nil {
		t.Fatal(err)
	}
	return reread(t, r)
}

// reread returns r as a later read of its file gives it.
func reread(t *testing.T, r model.Record) model.Record {
	t.Helper()
	data, err := model.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.Parse("x.md", data)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func decidedSince(r model.Record) model.Record { // what adj decide does after the crash
	r.Status, r.Decision, r.Rationale, r.DecidedAt, r.DecidedBy = model.StatusDecided, "a", "because", "2026-11-03", "owner"
	r.History = append(append([]model.HistoryEntry(nil), r.History...), model.HistoryEntry{Date: "2026-11-03", Event: model.EventDecide})
	return r
}

// A moved record that is amended after the crash is still the moved copy, and
// a record that only shares the ID and the name text with it is still not
// (both C163 cases). adj amend changes the title, context, option summaries and
// tradeoffs that adj ask sets, so IsStaleCopy cannot ask for them to be equal;
// it takes each amend entry after the renumber entry back to the text it kept
// and compares that.
func TestStaleCopyAfterAmend(t *testing.T) {
	stale, moved := File{ID: 3, Name: "0003-foo.md"}, File{ID: 5, Name: "0005-foo.md"}
	staleRec := askedRecord(3)
	staleRec.Options = []model.Option{{Label: "a", Summary: "A", Tradeoffs: "t"}, {Label: "b", Summary: "B"}}
	movedRec := reread(t, movedBy(t, staleRec, 5))

	// Every field an amend can change, one at a time and all at once.
	title := model.Amendment{Title: strp("Foo, corrected")}
	context := model.Amendment{Context: strp("a corrected\ncontext")}
	summary := model.Amendment{Summaries: map[string]string{"b": "B, corrected"}}
	tradeoffs := model.Amendment{Tradeoffs: map[string]string{"a": ""}}
	all := model.Amendment{Title: title.Title, Context: context.Context, Summaries: summary.Summaries,
		Tradeoffs: map[string]string{"a": "new", "b": "added"}}

	t.Run("the moved copy, amended after the crash", func(t *testing.T) {
		for name, a := range map[string]model.Amendment{"title": title, "context": context, "summary": summary,
			"tradeoffs": tradeoffs, "all fields": all} {
			if !IsStaleCopy(stale, staleRec, moved, amended(t, movedRec, a)) {
				t.Errorf("%s: IsStaleCopy = false for the moved copy, amended since", name)
			}
		}
	})
	t.Run("amended several times, the same field twice", func(t *testing.T) {
		r := amended(t, amended(t, amended(t, movedRec, title), context), model.Amendment{Title: strp("Foo, again")})
		if !IsStaleCopy(stale, staleRec, moved, r) {
			t.Error("IsStaleCopy = false after three amends")
		}
	})
	t.Run("amended and decided since, in either order", func(t *testing.T) {
		if !IsStaleCopy(stale, staleRec, moved, amended(t, reread(t, decidedSince(movedRec)), context)) {
			t.Error("IsStaleCopy = false for decided, then amended")
		}
		if !IsStaleCopy(stale, staleRec, moved, reread(t, decidedSince(amended(t, movedRec, context)))) {
			t.Error("IsStaleCopy = false for amended, then decided")
		}
		if !IsStaleCopy(stale, staleRec, moved, amended(t, reread(t, decidedSince(movedRec)), model.Amendment{Rationale: strp("a better reason")})) {
			t.Error("IsStaleCopy = false for decided, then its rationale amended")
		}
	})
	t.Run("the stale record was amended before the crash", func(t *testing.T) {
		before := amended(t, staleRec, title) // its history ends in an amend entry with old text
		again := reread(t, movedBy(t, before, 5))
		if !IsStaleCopy(stale, before, moved, again) {
			t.Error("IsStaleCopy = false for a stale record with an amend entry of its own")
		}
		if !IsStaleCopy(stale, before, moved, amended(t, again, context)) {
			t.Error("IsStaleCopy = false for a stale record with an amend entry of its own, amended again")
		}
	})

	// A keeper: another live record with the same ID and name text. Its
	// history may be just as short, so the content has to tell it apart.
	t.Run("a live record that shares the ID and the name text is not flagged", func(t *testing.T) {
		other := staleRec
		other.Context = "another record"
		otherMoved := reread(t, movedBy(t, other, 5))
		if IsStaleCopy(stale, staleRec, moved, otherMoved) {
			t.Fatal("the control: another record, not amended, is flagged")
		}
		for name, a := range map[string]model.Amendment{
			"amended to look like the keeper": {Context: strp(staleRec.Context)},
			"amended elsewhere":               title,
			"amended in another field":        summary,
		} {
			if IsStaleCopy(stale, staleRec, moved, amended(t, otherMoved, a)) {
				t.Errorf("%s: IsStaleCopy = true for another record, amended since", name)
			}
		}
	})
	t.Run("amended text that does not come back to the stale record", func(t *testing.T) {
		r := amended(t, movedRec, context)
		forged := *r.History[2].Was.Context
		r.History[2].Was.Context = strp(forged + " (not what it said)")
		if IsStaleCopy(stale, staleRec, moved, r) {
			t.Error("IsStaleCopy = true when the old text kept in the amend entry is not the stale record's")
		}
	})
	t.Run("a field amend cannot change still has to match", func(t *testing.T) {
		for name, edit := range map[string]func(*model.Record){
			"key":      func(r *model.Record) { r.Key = "other" },
			"tags":     func(r *model.Record) { r.Tags = []string{"u"} },
			"asked_by": func(r *model.Record) { r.AskedBy = "someone" },
			"created":  func(r *model.Record) { r.Created = "2026-10-05" },
			"an option label": func(r *model.Record) {
				r.Options = []model.Option{{Label: "z", Summary: "A", Tradeoffs: "t"}, r.Options[1]}
			},
			"an extra option":    func(r *model.Record) { r.Options = append(r.Options, model.Option{Label: "c"}) },
			"the recommendation": func(r *model.Record) { r.Recommendation = &model.Recommendation{Label: "b"} },
		} {
			r := amended(t, movedRec, context)
			r.Options = append([]model.Option(nil), r.Options...)
			edit(&r)
			if IsStaleCopy(stale, staleRec, moved, r) {
				t.Errorf("%s: IsStaleCopy = true although it differs and amend cannot change it", name)
			}
		}
	})
	t.Run("an amend entry that names an option the record does not have", func(t *testing.T) {
		// The context is rolled back to the stale text, so only the unknown
		// option can fail the match.
		r := amended(t, movedRec, context)
		r.History[2].Was.Options = []model.OptionWas{{Label: "gone", Summary: strp("x")}}
		if IsStaleCopy(stale, staleRec, moved, r) {
			t.Error("IsStaleCopy = true although the entry's old text belongs to no option")
		}
	})
	t.Run("the history before the renumber entry must still be the stale record's", func(t *testing.T) {
		r := amended(t, movedRec, context)
		r.History[0].Date = "2026-10-03"
		if IsStaleCopy(stale, staleRec, moved, r) {
			t.Error("IsStaleCopy = true although the history before the renumber entry differs")
		}
	})
	t.Run("an amend entry before the renumber entry is not rolled back", func(t *testing.T) {
		// The stale record was amended before the crash: its amend entry is part
		// of the common history, and its old text is not the stale text.
		before := amended(t, staleRec, title)
		r := reread(t, movedBy(t, before, 5))
		if !IsStaleCopy(stale, before, moved, r) {
			t.Fatal("the control: not recognised without an amend since")
		}
		r.Title = staleRec.Title // not what the stale record says
		if IsStaleCopy(stale, before, moved, r) {
			t.Error("IsStaleCopy = true although the title differs from the stale record's")
		}
	})
}

// At the level of Renumber: the stale file whose moved copy was amended is
// refused as a stale copy, and a keeper is not taken for one.
func TestRenumberAndAnAmendedStaleCopy(t *testing.T) {
	staleRec := askedRecord(3)
	staleRec.Context = "ctx"

	t.Run("refused", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "decisions")
		seed(t, dir, "0003-foo.md", staleRec)
		seed(t, dir, "0003-bar.md", func() model.Record { r := askedRecord(3); r.Title = "Bar"; r.Created = "2026-10-03"; return r }())
		seed(t, dir, "0005-foo.md", amended(t, reread(t, movedBy(t, staleRec, 5)), model.Amendment{
			Title: strp("Foo, corrected"), Context: strp("corrected"), Summaries: map[string]string{"a": "AA"}}))
		before := tree(t, root)
		got, err := mustOpen(t, root).Renumber(renumberDate, false)
		if !errors.Is(err, ErrStaleCopy) {
			t.Fatalf("Renumber = %+v, %v; want ErrStaleCopy", got.Moves, err)
		}
		if len(got.Moves) != 0 || !reflect.DeepEqual(tree(t, root), before) {
			t.Errorf("a refused Renumber moved %+v or changed the store", got.Moves)
		}
	})
	t.Run("a keeper is not flagged", func(t *testing.T) {
		root := t.TempDir()
		dir := filepath.Join(root, "decisions")
		keeper := staleRec
		seed(t, dir, "0003-foo.md", keeper)
		seed(t, dir, "0003-bar.md", func() model.Record { r := askedRecord(3); r.Title = "Bar"; r.Created = "2026-10-03"; return r }())
		other := askedRecord(3)
		other.Context = "the moved one"
		// The moved record is amended so that it now says what the keeper says.
		seed(t, dir, "0005-foo.md", amended(t, reread(t, movedBy(t, other, 5)), model.Amendment{Context: strp("ctx")}))
		got, err := mustOpen(t, root).Renumber(renumberDate, false)
		if err != nil {
			t.Fatalf("Renumber = %+v, %v; want the move of 0003-bar.md", got.Moves, err)
		}
		if len(got.Moves) != 1 || got.Moves[0].OldName != "0003-bar.md" {
			t.Errorf("moves = %+v, want only 0003-bar.md", got.Moves)
		}
		if _, err := os.Stat(filepath.Join(dir, "0003-foo.md")); err != nil {
			t.Errorf("the keeper is gone: %v", err)
		}
	})
}
