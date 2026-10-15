// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"errors"
	"reflect"
	"testing"
)

type transitionCase struct {
	name  string
	event Event
	to    Status
	from  Status // the only status the transition accepts
	apply func(*Record) error
}

func transitionCases() []transitionCase {
	return []transitionCase{
		{"decide", EventDecide, StatusDecided, StatusOpen, func(r *Record) error {
			return Decide(r, "files", "plain text", "owner", testDate)
		}},
		{"reject", EventReject, StatusRejected, StatusOpen, func(r *Record) error {
			return Reject(r, "out of scope", "owner", testDate)
		}},
		{"withdraw", EventWithdraw, StatusWithdrawn, StatusOpen, func(r *Record) error {
			return Withdraw(r, "asked twice", "agent", testDate)
		}},
		{"supersede", EventSupersede, StatusSuperseded, StatusDecided, func(r *Record) error {
			return Supersede(r, 7, testDate)
		}},
	}
}

func TestTransitionMatrix(t *testing.T) {
	for _, tc := range transitionCases() {
		for _, from := range allStatuses {
			t.Run(tc.name+" from "+string(from), func(t *testing.T) {
				r := validRecord(from)
				before := validRecord(from)
				err := tc.apply(&r)
				if from == tc.from {
					if err != nil {
						t.Fatalf("%s from %s = %v, want success", tc.name, from, err)
					}
					if r.Status != tc.to {
						t.Errorf("status = %q, want %q", r.Status, tc.to)
					}
					if verr := r.Validate(); verr != nil {
						t.Errorf("record invalid after transition: %v", verr)
					}
					return
				}
				if !errors.Is(err, ErrInvalidTransition) {
					t.Fatalf("%s from %s = %v, want ErrInvalidTransition", tc.name, from, err)
				}
				if !reflect.DeepEqual(r, before) {
					t.Errorf("record changed by failed transition\n got %+v\nwant %+v", r, before)
				}
			})
		}
	}
}

func TestSuccessfulTransitionAppendsExactlyOneHistoryEntry(t *testing.T) {
	for _, tc := range transitionCases() {
		t.Run(tc.name, func(t *testing.T) {
			r := validRecord(tc.from)
			r.History = []HistoryEntry{{Date: "2026-10-01", Event: EventAsk}}
			if err := tc.apply(&r); err != nil {
				t.Fatal(err)
			}
			if len(r.History) != 2 {
				t.Fatalf("history length %d, want 2: %+v", len(r.History), r.History)
			}
			if r.History[0] != (HistoryEntry{Date: "2026-10-01", Event: EventAsk}) {
				t.Errorf("earlier entry changed: %+v", r.History[0])
			}
			got := r.History[1]
			if got.Date != testDate || got.Event != tc.event {
				t.Errorf("entry = %+v, want date %s event %s", got, testDate, tc.event)
			}
		})
	}
}

func TestFailedTransitionAppendsNoHistory(t *testing.T) {
	r := validRecord(StatusRejected)
	r.History = []HistoryEntry{{Date: "2026-10-02", Event: EventReject}}
	if err := Decide(&r, "files", "why", "owner", testDate); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("err = %v", err)
	}
	if len(r.History) != 1 {
		t.Fatalf("history = %+v, want 1 entry", r.History)
	}
}

func TestTransitionDoesNotShareHistoryBackingArray(t *testing.T) {
	// A failed transition must not write into spare capacity of the caller's slice.
	hist := make([]HistoryEntry, 1, 4)
	hist[0] = HistoryEntry{Date: "2026-10-01", Event: EventAsk}
	r := validRecord(StatusOpen)
	r.History = hist
	if err := Decide(&r, "", "why", "owner", testDate); err == nil {
		t.Fatal("want error for empty decision")
	}
	if got := hist[:2][1]; got != (HistoryEntry{}) {
		t.Fatalf("failed transition wrote into the caller's backing array: %+v", got)
	}
}

func TestDecideSetsFields(t *testing.T) {
	r := validRecord(StatusOpen)
	r.Options = []Option{{Label: "a"}, {Label: "b"}}
	if err := Decide(&r, "b", "cheaper", "owner", testDate); err != nil {
		t.Fatal(err)
	}
	if r.Decision != "b" || r.Rationale != "cheaper" || r.DecidedBy != "owner" || r.DecidedAt != testDate {
		t.Errorf("fields not set: %+v", r)
	}
}

func TestRejectAndWithdrawSetFields(t *testing.T) {
	for _, tc := range transitionCases()[1:3] {
		t.Run(tc.name, func(t *testing.T) {
			r := validRecord(StatusOpen)
			if err := tc.apply(&r); err != nil {
				t.Fatal(err)
			}
			if r.Rationale == "" || r.DecidedAt != testDate || r.DecidedBy == "" {
				t.Errorf("fields not set: %+v", r)
			}
			if r.Decision != "" {
				t.Errorf("decision = %q, want empty", r.Decision)
			}
		})
	}
}

func TestSupersedeSetsSupersededByAndNote(t *testing.T) {
	r := validRecord(StatusDecided)
	if err := Supersede(&r, 7, testDate); err != nil {
		t.Fatal(err)
	}
	if r.SupersededBy != 7 {
		t.Errorf("superseded_by = %d, want 7", r.SupersededBy)
	}
	if got := r.History[len(r.History)-1].Note; got != "by 7" {
		t.Errorf("note = %q, want %q", got, "by 7")
	}
	if r.Decision != "files" || r.DecidedAt != "2026-10-02" {
		t.Errorf("supersede changed the decision: %+v", r)
	}
}

func TestTransitionRejectsBadInputAndLeavesRecordUnchanged(t *testing.T) {
	tests := []struct {
		name  string
		from  Status
		apply func(*Record) error
	}{
		{"decide empty decision", StatusOpen, func(r *Record) error { return Decide(r, "", "why", "o", testDate) }},
		{"decide blank decision", StatusOpen, func(r *Record) error { return Decide(r, " ", "why", "o", testDate) }},
		{"decide blank rationale", StatusOpen, func(r *Record) error { return Decide(r, "x", " ", "o", testDate) }},
		{"reject blank rationale", StatusOpen, func(r *Record) error { return Reject(r, "\t", "o", testDate) }},
		{"withdraw blank rationale", StatusOpen, func(r *Record) error { return Withdraw(r, " ", "o", testDate) }},
		{"decide empty rationale", StatusOpen, func(r *Record) error { return Decide(r, "x", "", "o", testDate) }},
		{"decide bad date", StatusOpen, func(r *Record) error { return Decide(r, "x", "why", "o", "yesterday") }},
		{"decide empty date", StatusOpen, func(r *Record) error { return Decide(r, "x", "why", "o", "") }},
		{"reject empty rationale", StatusOpen, func(r *Record) error { return Reject(r, "", "o", testDate) }},
		{"reject bad date", StatusOpen, func(r *Record) error { return Reject(r, "why", "o", "2026-02-30") }},
		{"withdraw empty rationale", StatusOpen, func(r *Record) error { return Withdraw(r, "", "o", testDate) }},
		{"withdraw bad date", StatusOpen, func(r *Record) error { return Withdraw(r, "why", "o", "10/15/2026") }},
		{"supersede by zero", StatusDecided, func(r *Record) error { return Supersede(r, 0, testDate) }},
		{"supersede by itself", StatusDecided, func(r *Record) error { return Supersede(r, 1, testDate) }},
		{"supersede bad date", StatusDecided, func(r *Record) error { return Supersede(r, 7, "") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord(tt.from)
			before := validRecord(tt.from)
			err := tt.apply(&r)
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
			if errors.Is(err, ErrInvalidTransition) {
				t.Errorf("bad input must not match ErrInvalidTransition: %v", err)
			}
			if !reflect.DeepEqual(r, before) {
				t.Errorf("record changed\n got %+v\nwant %+v", r, before)
			}
		})
	}
}

func TestInvalidTransitionIsAlsoInvalid(t *testing.T) {
	// One family: callers can match ErrInvalid for any model error.
	r := validRecord(StatusDecided)
	err := Decide(&r, "x", "y", "o", testDate)
	if !errors.Is(err, ErrInvalidTransition) || !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestSupersededIsTerminalAndDecidedCannotBeRejected(t *testing.T) {
	r := validRecord(StatusSuperseded)
	for _, tc := range transitionCases() {
		if err := tc.apply(&r); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("%s from superseded = %v", tc.name, err)
		}
	}
}

func TestLinkSupersedesAddsTheOldIDAndOneHistoryEntry(t *testing.T) {
	r := validRecord(StatusDecided)
	r.ID = 7
	if err := LinkSupersedes(&r, 3, testDate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Supersedes, []int{3}) {
		t.Errorf("supersedes = %v, want [3]", r.Supersedes)
	}
	want := HistoryEntry{Date: testDate, Event: EventSupersede, Note: "supersedes 3"}
	if len(r.History) != 1 || r.History[0] != want {
		t.Errorf("history = %+v, want [%+v]", r.History, want)
	}
	if r.Status != StatusDecided || r.Decision != "files" || r.SupersededBy != 0 {
		t.Errorf("the record is no longer a plain decided one: %+v", r)
	}
	// A second link keeps the first and adds one entry more.
	if err := LinkSupersedes(&r, 5, testDate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Supersedes, []int{3, 5}) || len(r.History) != 2 {
		t.Errorf("supersedes = %v, history = %+v; want [3 5] and 2 entries", r.Supersedes, r.History)
	}
}

func TestLinkSupersedesListsAnIDOnce(t *testing.T) {
	r := validRecord(StatusDecided)
	r.ID = 7
	r.Supersedes = []int{3}
	if err := LinkSupersedes(&r, 3, testDate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Supersedes, []int{3}) {
		t.Errorf("supersedes = %v, want [3] once", r.Supersedes)
	}
}

func TestLinkSupersedesNeedsADecidedRecordAndRealInput(t *testing.T) {
	for _, from := range allStatuses {
		if from == StatusDecided {
			continue
		}
		r := validRecord(from)
		before := validRecord(from)
		if err := LinkSupersedes(&r, 7, testDate); !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("link from %s = %v, want ErrInvalidTransition", from, err)
		}
		if !reflect.DeepEqual(r, before) {
			t.Errorf("record changed by a refused link from %s: %+v", from, r)
		}
	}
	tests := []struct {
		name string
		old  int
		date string
	}{
		{"zero", 0, testDate},
		{"negative", -2, testDate},
		{"itself", 1, testDate},
		{"bad date", 7, "yesterday"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord(StatusDecided)
			before := validRecord(StatusDecided)
			err := LinkSupersedes(&r, tt.old, tt.date)
			if !errors.Is(err, ErrInvalid) || errors.Is(err, ErrInvalidTransition) {
				t.Errorf("err = %v, want ErrInvalid and not ErrInvalidTransition", err)
			}
			if !reflect.DeepEqual(r, before) {
				t.Errorf("record changed: %+v", r)
			}
		})
	}
}
