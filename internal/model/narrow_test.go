// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Narrow records on OLD, a decided record, that NEW narrows it to a scope: one
// narrowed_by link and one narrow history entry, "by NEW: SCOPE". OLD stays
// decided and nothing else about it changes (S51).
func TestNarrowAddsTheLinkAndOneHistoryEntry(t *testing.T) {
	r := validRecord(StatusDecided)
	r.RecordedBy, r.Via = "claude", "chat"
	if err := Narrow(&r, 7, "only for structured questions", testDate); err != nil {
		t.Fatal(err)
	}
	if want := []Narrowing{{ID: 7, Scope: "only for structured questions"}}; !reflect.DeepEqual(r.NarrowedBy, want) {
		t.Errorf("narrowed_by = %+v, want %+v", r.NarrowedBy, want)
	}
	want := HistoryEntry{Date: testDate, Event: EventNarrow, Note: "by 7: only for structured questions"}
	if len(r.History) != 1 || r.History[0] != want {
		t.Errorf("history = %+v, want [%+v]", r.History, want)
	}
	if r.Status != StatusDecided || r.Decision != "files" || r.SupersededBy != 0 || len(r.Narrows) != 0 {
		t.Errorf("the record is no longer a plain decided one: %+v", r)
	}
	if r.RecordedBy != "claude" || r.Via != "chat" {
		t.Errorf("recorded_by = %q, via = %q, want them unchanged (S52)", r.RecordedBy, r.Via)
	}
	// A second narrowing record keeps the first and adds one entry more.
	if err := Narrow(&r, 9, "not for the sidebar", testDate); err != nil {
		t.Fatal(err)
	}
	if want := []Narrowing{{7, "only for structured questions"}, {9, "not for the sidebar"}}; !reflect.DeepEqual(r.NarrowedBy, want) || len(r.History) != 2 {
		t.Errorf("narrowed_by = %+v, history = %+v; want both links and 2 entries", r.NarrowedBy, r.History)
	}
}

// LinkNarrows is the other half: NEW lists OLD in narrows, once, with one
// narrow history entry "narrows OLD".
func TestLinkNarrowsAddsTheOldIDAndOneHistoryEntry(t *testing.T) {
	r := validRecord(StatusDecided)
	r.ID = 7
	if err := LinkNarrows(&r, 3, testDate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Narrows, []int{3}) {
		t.Errorf("narrows = %v, want [3]", r.Narrows)
	}
	want := HistoryEntry{Date: testDate, Event: EventNarrow, Note: "narrows 3"}
	if len(r.History) != 1 || r.History[0] != want {
		t.Errorf("history = %+v, want [%+v]", r.History, want)
	}
	if r.Status != StatusDecided || len(r.NarrowedBy) != 0 || len(r.Supersedes) != 0 {
		t.Errorf("the record is no longer a plain decided one: %+v", r)
	}
	if err := LinkNarrows(&r, 5, testDate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Narrows, []int{3, 5}) || len(r.History) != 2 {
		t.Errorf("narrows = %v, history = %+v; want [3 5] and 2 entries", r.Narrows, r.History)
	}
	// An ID already listed stays listed once.
	if err := LinkNarrows(&r, 3, testDate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Narrows, []int{3, 5}) {
		t.Errorf("narrows = %v, want [3 5] with 3 once", r.Narrows)
	}
}

// Only a decided record narrows or is narrowed (a superseded one is replaced,
// the others have no decision); a refusal leaves the record as it was.
func TestNarrowAndLinkNarrowsNeedADecidedRecord(t *testing.T) {
	for _, from := range allStatuses {
		if from == StatusDecided {
			continue
		}
		for name, do := range map[string]func(*Record) error{
			"narrow":       func(r *Record) error { return Narrow(r, 7, "scope", testDate) },
			"link narrows": func(r *Record) error { return LinkNarrows(r, 7, testDate) },
		} {
			r := validRecord(from)
			before := validRecord(from)
			if err := do(&r); !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("%s from %s = %v, want ErrInvalidTransition", name, from, err)
			}
			if !reflect.DeepEqual(r, before) {
				t.Errorf("%s changed a %s record: %+v", name, from, r)
			}
		}
	}
}

func TestNarrowRefusesBadInputAndLeavesTheRecordAlone(t *testing.T) {
	tests := []struct {
		name  string
		apply func(*Record) error
	}{
		{"narrow: id zero", func(r *Record) error { return Narrow(r, 0, "s", testDate) }},
		{"narrow: id negative", func(r *Record) error { return Narrow(r, -2, "s", testDate) }},
		{"narrow: itself", func(r *Record) error { return Narrow(r, 1, "s", testDate) }},
		{"narrow: empty scope", func(r *Record) error { return Narrow(r, 7, "", testDate) }},
		{"narrow: blank scope", func(r *Record) error { return Narrow(r, 7, " \t ", testDate) }},
		{"narrow: scope on two lines", func(r *Record) error { return Narrow(r, 7, "a\nb", testDate) }},
		{"narrow: scope with a carriage return", func(r *Record) error { return Narrow(r, 7, "a\rb", testDate) }},
		{"narrow: bad date", func(r *Record) error { return Narrow(r, 7, "s", "yesterday") }},
		{"link: id zero", func(r *Record) error { return LinkNarrows(r, 0, testDate) }},
		{"link: negative", func(r *Record) error { return LinkNarrows(r, -2, testDate) }},
		{"link: itself", func(r *Record) error { return LinkNarrows(r, 1, testDate) }},
		{"link: bad date", func(r *Record) error { return LinkNarrows(r, 7, "yesterday") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord(StatusDecided)
			before := validRecord(StatusDecided)
			err := tt.apply(&r)
			if !errors.Is(err, ErrInvalid) || errors.Is(err, ErrInvalidTransition) {
				t.Errorf("err = %v, want ErrInvalid and not ErrInvalidTransition", err)
			}
			if !reflect.DeepEqual(r, before) {
				t.Errorf("record changed: %+v", r)
			}
		})
	}
}

// The same pair is never linked twice (C32): the second Narrow is refused with
// ErrAlreadyNarrowed, which is also ErrInvalid, and nothing is added.
func TestNarrowTheSamePairTwiceIsRefused(t *testing.T) {
	r := validRecord(StatusDecided)
	if err := Narrow(&r, 7, "first scope", testDate); err != nil {
		t.Fatal(err)
	}
	before := cloneRecord(r)
	err := Narrow(&r, 7, "another scope", testDate)
	if !errors.Is(err, ErrAlreadyNarrowed) || !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrAlreadyNarrowed (an ErrInvalid)", err)
	}
	if !strings.Contains(err.Error(), "record 1 is already narrowed by record 7") {
		t.Errorf("err = %q, want it to name both records", err)
	}
	if !reflect.DeepEqual(r, before) {
		t.Errorf("record changed by the refused second narrow: %+v", r)
	}
}

// Validate refuses narrow fields that cannot be right whatever the status:
// a link to the record itself or to no record, a blank or multi-line scope, or
// the same ID twice. Status is lint's business (narrow-state), as for
// superseded_by.
func TestValidateChecksNarrowFields(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Record)
		want string // "" for valid
	}{
		{"valid", func(r *Record) { r.NarrowedBy = []Narrowing{{2, "s"}}; r.Narrows = []int{3} }, ""},
		{"narrowed_by itself", func(r *Record) { r.NarrowedBy = []Narrowing{{1, "s"}} }, "narrowed_by: 1 is not another record's id"},
		{"narrowed_by zero", func(r *Record) { r.NarrowedBy = []Narrowing{{0, "s"}} }, "narrowed_by: 0 is not another record's id"},
		{"narrowed_by negative", func(r *Record) { r.NarrowedBy = []Narrowing{{-1, "s"}} }, "narrowed_by: -1 is not another record's id"},
		{"narrowed_by blank scope", func(r *Record) { r.NarrowedBy = []Narrowing{{2, "  "}} }, "narrowed_by: the scope for 2 must not be blank"},
		{"narrowed_by empty scope", func(r *Record) { r.NarrowedBy = []Narrowing{{2, ""}} }, "narrowed_by: the scope for 2 must not be blank"},
		{"narrowed_by scope on two lines", func(r *Record) { r.NarrowedBy = []Narrowing{{2, "a\nb"}} }, "narrowed_by: the scope for 2 must be one line"},
		{"narrowed_by twice", func(r *Record) { r.NarrowedBy = []Narrowing{{2, "a"}, {2, "b"}} }, "narrowed_by: 2 is listed twice"},
		{"narrows itself", func(r *Record) { r.Narrows = []int{1} }, "narrows: 1 is not another record's id"},
		{"narrows zero", func(r *Record) { r.Narrows = []int{0} }, "narrows: 0 is not another record's id"},
		{"narrows twice", func(r *Record) { r.Narrows = []int{3, 3} }, "narrows: 3 is listed twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord(StatusDecided)
			tt.edit(&r)
			err := r.Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate = %v, want an ErrInvalid containing %q", err, tt.want)
			}
		})
	}
}

// narrow is an event this version knows; a record with one is valid and its
// entry is not reported as unknown.
func TestNarrowIsAKnownEvent(t *testing.T) {
	if EventNarrow != "narrow" || !EventNarrow.Known() {
		t.Errorf("EventNarrow = %q, Known() = %v; want %q and true", EventNarrow, EventNarrow.Known(), "narrow")
	}
	r := validRecord(StatusDecided)
	r.History = []HistoryEntry{{Date: testDate, Event: EventNarrow, Note: "narrows 3"}}
	if err := r.Validate(); err != nil {
		t.Errorf("Validate = %v", err)
	}
	r.History[0].Was = &Was{Title: sp("x")}
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "only an amend entry has was") {
		t.Errorf("a narrow entry with was: Validate = %v, want the only-amend rule", err)
	}
}

// The two fields are written after superseded_by, in a fixed shape, and left
// out when empty; they read back as written.
func TestNarrowFieldsRoundTripInTheFile(t *testing.T) {
	r := validRecord(StatusDecided)
	plain, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "narrow") {
		t.Errorf("a record with no narrow link prints narrow keys:\n%s", plain)
	}

	r.SupersededBy = 0
	r.Supersedes = []int{9}
	r.NarrowedBy = []Narrowing{{ID: 4, Scope: `only for "structured" <questions> & more`}, {ID: 6, Scope: "second"}}
	r.Narrows = []int{2, 3}
	r.Refs = []string{"S51"}
	data, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	order := []string{`"supersedes"`, `"narrowed_by"`, `"narrows"`, `"refs"`}
	last := -1
	for _, key := range order {
		i := strings.Index(text, key)
		if i < 0 || i < last {
			t.Fatalf("key %s missing or out of order in:\n%s", key, text)
		}
		last = i
	}
	front, _, _ := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(front), &obj); err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	for key, want := range map[string]string{
		"narrowed_by": `[{"id":4,"scope":"only for \"structured\" <questions> & more"},{"id":6,"scope":"second"}]`,
		"narrows":     `[2,3]`,
	} {
		compact.Reset()
		if err := json.Compact(&compact, obj[key]); err != nil {
			t.Fatal(err)
		}
		if compact.String() != want {
			t.Errorf("%s = %s, want %s", key, compact.String(), want)
		}
	}
	back, err := Unmarshal("x.md", data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.NarrowedBy, r.NarrowedBy) || !reflect.DeepEqual(back.Narrows, r.Narrows) {
		t.Errorf("read back narrowed_by = %+v, narrows = %v; want %+v and %v", back.NarrowedBy, back.Narrows, r.NarrowedBy, r.Narrows)
	}
	if keys := back.UnknownKeys(); len(keys) != 0 {
		t.Errorf("narrowed_by and narrows are known keys, but UnknownKeys = %v", keys)
	}
}

// The new keys are known keys: they do not count as unknown ones (S46), and the
// strictness inside a known key stays: an object in narrowed_by with a key this
// version does not know is refused, as it is inside an option.
func TestNarrowedByStaysStrictInside(t *testing.T) {
	const head = "---\n" + `{"id":1,"title":"T","status":"decided","created":"2026-10-01","decided_at":"2026-10-02","decision":"d","rationale":"r",`
	good, err := Unmarshal("x.md", []byte(head+`"narrowed_by":[{"id":2,"scope":"s"}],"narrows":[3]}`+"\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(good.UnknownKeys()) != 0 {
		t.Errorf("unknown keys = %v, want none", good.UnknownKeys())
	}
	_, err = Unmarshal("x.md", []byte(head+`"narrowed_by":[{"id":2,"scope":"s","colour":"red"}]}`+"\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "colour") {
		t.Errorf("an unknown key inside narrowed_by: err = %v, want it refused naming the key", err)
	}
	_, err = Unmarshal("x.md", []byte(head+`"narrows":"3"}`+"\n---\n"))
	if err == nil {
		t.Error("narrows with the wrong type was accepted")
	}
}

// Every other transition leaves the narrow links as they are: a decided,
// narrowed record can still be superseded, amended or renumbered.
func TestOtherChangesLeaveTheNarrowLinksAlone(t *testing.T) {
	base := func() Record {
		r := amendable(StatusDecided)
		r.NarrowedBy = []Narrowing{{ID: 4, Scope: "s"}}
		r.Narrows = []int{5}
		return r
	}
	tests := []struct {
		name  string
		apply func(*Record) error
	}{
		{"supersede", func(r *Record) error { return Supersede(r, 7, testDate) }},
		{"link supersedes", func(r *Record) error { return LinkSupersedes(r, 7, testDate) }},
		{"amend", func(r *Record) error {
			return Amend(r, Amendment{Title: sp("Other"), Rationale: sp("new why")}, testDate)
		}},
		{"renumber", func(r *Record) error { return Renumber(r, 9, testDate) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := base()
			if err := tt.apply(&r); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(r.NarrowedBy, []Narrowing{{ID: 4, Scope: "s"}}) || !reflect.DeepEqual(r.Narrows, []int{5}) {
				t.Errorf("narrowed_by = %+v, narrows = %v, want them unchanged", r.NarrowedBy, r.Narrows)
			}
		})
	}
}

// Narrow and LinkNarrows do not share a backing array with the record they
// were given: a failed later step cannot see half a change.
func TestNarrowDoesNotAliasTheOriginalLinks(t *testing.T) {
	r := validRecord(StatusDecided)
	r.NarrowedBy = make([]Narrowing, 1, 8)
	r.NarrowedBy[0] = Narrowing{ID: 4, Scope: "s"}
	r.Narrows = make([]int, 1, 8)
	r.Narrows[0] = 5
	orig := r
	if err := Narrow(&r, 7, "t", testDate); err != nil {
		t.Fatal(err)
	}
	if err := LinkNarrows(&r, 8, testDate); err != nil {
		t.Fatal(err)
	}
	if len(orig.NarrowedBy) != 1 || orig.NarrowedBy[:2][1] != (Narrowing{}) || orig.Narrows[:2][1] != 0 {
		t.Errorf("the original's backing arrays were written: %+v %v", orig.NarrowedBy[:2], orig.Narrows[:2])
	}
}

// Narrow and LinkNarrows keep the unknown top-level keys and the unknown keys of
// an older entry's was, as every other transition does (S46).
func TestNarrowKeepsUnknownKeys(t *testing.T) {
	const want1, want2 = `"alpha": true`, `"zeta": {`
	for name, do := range map[string]func(*Record) error{
		"narrow":       func(r *Record) error { return Narrow(r, 9, "scope", "2026-11-02") },
		"link narrows": func(r *Record) error { return LinkNarrows(r, 9, "2026-11-02") },
	} {
		t.Run(name, func(t *testing.T) {
			r, err := Unmarshal("x.md", []byte(`---
{"id":4,"title":"Logging","status":"decided","created":"2026-10-01","decided_at":"2026-10-02","decision":"d","rationale":"r",`+
				`"history":[`+askEntry+`],`+futureTop+`}
---
`))
			if err != nil {
				t.Fatal(err)
			}
			if err := do(&r); err != nil {
				t.Fatal(err)
			}
			data, err := Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{want1, want2} {
				if !strings.Contains(string(data), want) {
					t.Errorf("file lost %s:\n%s", want, data)
				}
			}
			if keys := r.UnknownKeys(); len(keys) == 0 {
				t.Error("the record lost its unknown keys")
			}
		})
	}
}
