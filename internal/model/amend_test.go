// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func sp(s string) *string { return &s }

// amendable is a valid record in status s with every field amend can change:
// a multi-line context, two options with summaries and one with tradeoffs.
func amendable(s Status) Record {
	r := validRecord(s)
	r.Context = "first line\nsecond line\n"
	r.Options = []Option{
		{Label: "a", Summary: "Use A", Tradeoffs: "slower"},
		{Label: "b", Summary: "Use B"},
	}
	r.Recommendation = &Recommendation{Label: "a", Reasoning: "proven"}
	r.History = []HistoryEntry{{Date: "2026-10-01", Event: EventAsk}}
	return r
}

// cloneRecord copies the slices that Amend might write into.
func cloneRecord(r Record) Record {
	r.Options = slices.Clone(r.Options)
	r.History = slices.Clone(r.History)
	return r
}

func TestAmendChangesOnlyTheNamedField(t *testing.T) {
	cases := []struct {
		name  string
		amend Amendment
		skip  Status // a status the case does not apply to
		want  func(r *Record)
		note  string
		was   Was
	}{
		{"title", Amendment{Title: sp("Pick a datastore")}, "",
			func(r *Record) { r.Title = "Pick a datastore" }, "title", Was{Title: sp("Pick a store")}},
		{"context", Amendment{Context: sp("new\ncontext")}, "",
			func(r *Record) { r.Context = "new\ncontext" }, "context", Was{Context: sp("first line\nsecond line\n")}},
		{"option summary", Amendment{Summaries: map[string]string{"b": "Use B, pinned"}}, "",
			func(r *Record) { r.Options[1].Summary = "Use B, pinned" }, "summary b",
			Was{Options: []OptionWas{{Label: "b", Summary: sp("Use B")}}}},
		{"option tradeoffs", Amendment{Tradeoffs: map[string]string{"a": "slower, but proven"}}, "",
			func(r *Record) { r.Options[0].Tradeoffs = "slower, but proven" }, "tradeoffs a",
			Was{Options: []OptionWas{{Label: "a", Tradeoffs: sp("slower")}}}},
		{"tradeoffs set where there were none", Amendment{Tradeoffs: map[string]string{"b": "new"}}, "",
			func(r *Record) { r.Options[1].Tradeoffs = "new" }, "tradeoffs b",
			Was{Options: []OptionWas{{Label: "b", Tradeoffs: sp("")}}}},
		{"tradeoffs cleared", Amendment{Tradeoffs: map[string]string{"a": ""}}, "",
			func(r *Record) { r.Options[0].Tradeoffs = "" }, "tradeoffs a",
			Was{Options: []OptionWas{{Label: "a", Tradeoffs: sp("slower")}}}},
		{"rationale", Amendment{Rationale: sp("a better reason")}, StatusOpen,
			func(r *Record) { r.Rationale = "a better reason" }, "rationale", Was{Rationale: sp("")}}, // old text set below
	}
	for _, s := range allStatuses {
		for _, tc := range cases {
			if tc.skip == s {
				continue
			}
			t.Run(string(s)+"/"+tc.name, func(t *testing.T) {
				r := amendable(s)
				before := cloneRecord(r)
				want := cloneRecord(r)
				tc.want(&want)
				wasWant := tc.was
				if tc.name == "rationale" {
					wasWant = Was{Rationale: sp(before.Rationale)}
				}
				want.History = append(want.History, HistoryEntry{Date: testDate, Event: EventAmend, Note: tc.note, Was: &wasWant})
				if err := Amend(&r, tc.amend, testDate); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(r, want) {
					t.Errorf("amended record\n got %+v\nwant %+v", r, want)
				}
				if r.Status != before.Status || r.Decision != before.Decision || r.DecidedBy != before.DecidedBy ||
					r.DecidedAt != before.DecidedAt || r.SupersededBy != before.SupersededBy {
					t.Errorf("amend changed status, decision or links: %+v", r)
				}
			})
		}
	}
}

// One entry, whatever the number of fields changed; a field given its current
// value is not a change and is not in the entry.
func TestAmendWithSeveralFieldsAppendsOneEntryWithEveryOldText(t *testing.T) {
	r := amendable(StatusDecided)
	err := Amend(&r, Amendment{
		Title:     sp("New title"),
		Context:   sp("same?"),
		Rationale: sp(r.Rationale), // unchanged
		Summaries: map[string]string{"a": "Use A!", "b": "Use B"},
		Tradeoffs: map[string]string{"a": "slower"},
	}, testDate)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.History) != 2 {
		t.Fatalf("history = %+v, want the ask and exactly one amend", r.History)
	}
	got := r.History[1]
	want := HistoryEntry{Date: testDate, Event: EventAmend, Note: "title, context, summary a", Was: &Was{
		Title:   sp("Pick a store"),
		Context: sp("first line\nsecond line\n"),
		Options: []OptionWas{{Label: "a", Summary: sp("Use A")}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entry\n got %+v\nwant %+v", got, want)
	}
	if r.Title != "New title" || r.Context != "same?" || r.Options[0].Summary != "Use A!" {
		t.Errorf("fields not set: %+v", r)
	}
}

func TestAmendRefuses(t *testing.T) {
	cases := []struct {
		name    string
		status  Status
		amend   Amendment
		wantErr error
		text    string
	}{
		{"nothing given", StatusDecided, Amendment{}, ErrNoChange, "nothing to change"},
		{"every field already has the text", StatusDecided, Amendment{
			Title: sp("Pick a store"), Summaries: map[string]string{"a": "Use A"}, Tradeoffs: map[string]string{"b": ""},
		}, ErrNoChange, "nothing to change"},
		{"rationale of an open record", StatusOpen, Amendment{Rationale: sp("why")}, ErrInvalidTransition, "open"},
		{"rationale of an open record, with a real change beside it", StatusOpen,
			Amendment{Title: sp("T2"), Rationale: sp("why")}, ErrInvalidTransition, "open"},
		{"unknown option in summary", StatusOpen, Amendment{Summaries: map[string]string{"z": "x"}}, ErrInvalid, `"z"`},
		{"unknown option in tradeoffs", StatusDecided, Amendment{Tradeoffs: map[string]string{"z": "x"}}, ErrInvalid, `"z"`},
		{"blank title", StatusDecided, Amendment{Title: sp("  ")}, ErrInvalid, "title"},
		{"multi-line title", StatusDecided, Amendment{Title: sp("a\nb")}, ErrInvalid, "title"},
		{"blank rationale", StatusDecided, Amendment{Rationale: sp(" \n")}, ErrInvalid, "rationale"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := amendable(tc.status)
			before := cloneRecord(r)
			err := Amend(&r, tc.amend, testDate)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want one matching %v", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.text) {
				t.Errorf("err = %q, want it to contain %q", err, tc.text)
			}
			if !reflect.DeepEqual(r, before) {
				t.Errorf("a refused amend changed the record\n got %+v\nwant %+v", r, before)
			}
		})
	}
	t.Run("bad date", func(t *testing.T) {
		r := amendable(StatusOpen)
		if err := Amend(&r, Amendment{Title: sp("T2")}, "10/17/2026"); !errors.Is(err, ErrInvalid) {
			t.Errorf("err = %v, want ErrInvalid", err)
		}
	})
}

// A no-change amend is not a transition problem and not a plain invalid record:
// callers tell it apart with errors.Is(ErrNoChange).
func TestAmendNoChangeIsItsOwnError(t *testing.T) {
	r := amendable(StatusOpen)
	err := Amend(&r, Amendment{Title: sp(r.Title)}, testDate)
	if !errors.Is(err, ErrNoChange) || errors.Is(err, ErrInvalidTransition) {
		t.Errorf("err = %v, want ErrNoChange and not ErrInvalidTransition", err)
	}
}

// The caller's options array must not be written into, on success or failure.
func TestAmendDoesNotWriteIntoTheCallersSlices(t *testing.T) {
	r := amendable(StatusOpen)
	opts := r.Options
	hist := make([]HistoryEntry, 1, 4)
	copy(hist, r.History)
	r.History = hist
	orig := slices.Clone(opts)
	if err := Amend(&r, Amendment{Summaries: map[string]string{"a": "changed"}}, testDate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(opts, orig) {
		t.Errorf("Amend wrote into the original options: %+v", opts)
	}
	if got := hist[:2][1]; !reflect.DeepEqual(got, HistoryEntry{}) {
		t.Errorf("Amend wrote into the original history's spare capacity: %+v", got)
	}
}

// The old text has to survive the file format, multi-line text and an empty
// old value included.
func TestAmendedRecordRoundTripsThroughAFile(t *testing.T) {
	r := amendable(StatusDecided)
	r.Rationale = "line one\nline two\n\nafter a blank line"
	if err := Amend(&r, Amendment{
		Context:   sp("short"),
		Rationale: sp("short too"),
		Tradeoffs: map[string]string{"b": "now with a trade-off"},
	}, testDate); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "0001-x.md")
	if err := WriteFile(path, r); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, r) {
		t.Fatalf("round trip changed the record\n got %+v\nwant %+v", got, r)
	}
	was := got.History[1].Was
	if *was.Context != "first line\nsecond line\n" || *was.Rationale != "line one\nline two\n\nafter a blank line" {
		t.Errorf("old multi-line text changed: %q, %q", *was.Context, *was.Rationale)
	}
	if was.Options[0].Tradeoffs == nil || *was.Options[0].Tradeoffs != "" {
		t.Errorf("an old empty tradeoffs text was lost: %+v", was.Options)
	}
}

func TestValidateChecksAmendEntries(t *testing.T) {
	entry := func(e HistoryEntry) Record {
		r := amendable(StatusOpen)
		r.History = append(r.History, e)
		return r
	}
	good := HistoryEntry{Date: testDate, Event: EventAmend, Was: &Was{Title: sp("old")}}
	if err := entry(good).Validate(); err != nil {
		t.Fatalf("a good amend entry is invalid: %v", err)
	}
	bad := []struct {
		name string
		e    HistoryEntry
	}{
		{"amend without old text", HistoryEntry{Date: testDate, Event: EventAmend}},
		{"amend with empty old text", HistoryEntry{Date: testDate, Event: EventAmend, Was: &Was{}}},
		{"old text on another event", HistoryEntry{Date: testDate, Event: EventDecide, Was: &Was{Title: sp("old")}}},
		{"option without label", HistoryEntry{Date: testDate, Event: EventAmend, Was: &Was{Options: []OptionWas{{Summary: sp("x")}}}}},
		{"option without old text", HistoryEntry{Date: testDate, Event: EventAmend, Was: &Was{Options: []OptionWas{{Label: "a"}}}}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			err := entry(tc.e).Validate()
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "history") {
				t.Errorf("Validate = %v, want a history error", err)
			}
		})
	}
}

func TestEventAmendIsValid(t *testing.T) {
	if !EventAmend.Valid() || EventAmend != "amend" {
		t.Errorf("EventAmend = %q, valid %v; want a valid %q", EventAmend, EventAmend.Valid(), "amend")
	}
}
