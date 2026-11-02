// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// A change to a record that is not open fails with an error that names the
// record's status, still matches model.ErrInvalidTransition, and is not a
// usage error (so the CLI exits 1).
func TestClosedRecordErrorMatchesInvalidTransition(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 4, 23, 30, 0, 0, time.UTC)
	rec, err := Ask(st, AskInput{Title: "Pick a store"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Reject(st, rec.ID, ReasonInput{Why: "no"}, now); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		change  func() error
		wantErr string
	}{
		{"decide", func() error {
			_, err := Decide(st, rec.ID, DecideInput{Text: "x", HasText: true, Why: "w"}, now)
			return err
		}, "record 1 is rejected; only an open record can be decided"},
		{"withdraw", func() error {
			_, err := Withdraw(st, rec.ID, ReasonInput{Why: "w"}, now)
			return err
		}, "record 1 is rejected; only an open record can be withdrawn"},
	}
	for _, tt := range tests {
		err := tt.change()
		if err == nil {
			t.Errorf("%s: no error on a rejected record", tt.name)
			continue
		}
		if !errors.Is(err, model.ErrInvalidTransition) {
			t.Errorf("%s: err = %v, want one matching model.ErrInvalidTransition", tt.name, err)
		}
		if !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s: err = %q, want it to contain %q", tt.name, err, tt.wantErr)
		}
		var ue *UsageError
		if errors.As(err, &ue) {
			t.Errorf("%s: err = %v is a usage error, want an ordinary failure", tt.name, err)
		}
	}
}

func TestDecideDecidedAt(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	rec, err := Ask(st, AskInput{Title: "Pick a store"}, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decide(st, rec.ID, DecideInput{
		Text: "files", HasText: true, Why: "decided before logging",
		DecidedAt: "2026-10-01", HasDecidedAt: true,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.DecidedAt != "2026-10-01" {
		t.Errorf("decided_at = %q, want 2026-10-01", got.DecidedAt)
	}
	want := model.HistoryEntry{Date: "2026-10-16", Event: model.EventDecide, Note: "backdated to 2026-10-01"}
	if len(got.History) != 2 || got.History[1] != want {
		t.Errorf("history = %+v, want final entry %+v", got.History, want)
	}
}

func TestDecideDecidedAtFutureIsUsageErrorAndLeavesStoreOpen(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	rec, err := Ask(st, AskInput{Title: "Pick a store"}, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Decide(st, rec.ID, DecideInput{
		Text: "files", HasText: true, Why: "future",
		DecidedAt: "2026-10-17", HasDecidedAt: true,
	}, now)
	var usageErr *UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("error = %v, want UsageError", err)
	}
	after, err := st.Get(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != model.StatusOpen || len(after.History) != 1 {
		t.Errorf("record changed after refused future date: %+v", after)
	}
}

// openRecord asks one open question in a fresh store.
func openRecord(t *testing.T, now time.Time) (*store.Store, int) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rec, err := Ask(st, AskInput{Title: "Pick a store", Options: []string{"a:Use A"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	return st, rec.ID
}

// Decide, Reject and Withdraw store recorded_by and via as typed, and what is
// stored is what a later read returns (S52).
func TestAnswersStoreRecordedByAndVia(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	const by, via = " claude ", "interactive question, session abc "
	tests := []struct {
		name string
		do   func(*store.Store, int) (model.Record, error)
	}{
		{"decide", func(st *store.Store, id int) (model.Record, error) {
			return Decide(st, id, DecideInput{Option: "a", HasOption: true, Why: "w", DecidedBy: "owner",
				RecordedBy: by, HasRecordedBy: true, Via: via, HasVia: true}, now)
		}},
		{"reject", func(st *store.Store, id int) (model.Record, error) {
			return Reject(st, id, ReasonInput{Why: "w", DecidedBy: "owner",
				RecordedBy: by, HasRecordedBy: true, Via: via, HasVia: true}, now)
		}},
		{"withdraw", func(st *store.Store, id int) (model.Record, error) {
			return Withdraw(st, id, ReasonInput{Why: "w", DecidedBy: "owner",
				RecordedBy: by, HasRecordedBy: true, Via: via, HasVia: true}, now)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, id := openRecord(t, now)
			got, err := tt.do(st, id)
			if err != nil {
				t.Fatal(err)
			}
			stored, err := st.Get(id)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range []model.Record{got, stored} {
				if r.RecordedBy != by || r.Via != via || r.DecidedBy != "owner" {
					t.Errorf("recorded_by = %q, via = %q, decided_by = %q; want %q, %q, owner", r.RecordedBy, r.Via, r.DecidedBy, by, via)
				}
			}
		})
	}
}

// A flag that is given but blank (or whitespace only) is a usage error that
// Check finds before any store access, and writes nothing.
func TestBlankRecordedByOrViaIsAUsageErrorAndWritesNothing(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	type attempt struct {
		name  string
		check error
		do    func(*store.Store, int) error
	}
	for _, blank := range []string{"", " ", "\t\n"} {
		for _, flag := range []string{"recorded-by", "via"} {
			d := DecideInput{Option: "a", HasOption: true, Why: "w"}
			r := ReasonInput{Why: "w"}
			if flag == "via" {
				d.Via, d.HasVia, r.Via, r.HasVia = blank, true, blank, true
			} else {
				d.RecordedBy, d.HasRecordedBy, r.RecordedBy, r.HasRecordedBy = blank, true, blank, true
			}
			attempts := []attempt{
				{"decide", d.Check(), func(st *store.Store, id int) error { _, err := Decide(st, id, d, now); return err }},
				{"reject", r.Check(), func(st *store.Store, id int) error { _, err := Reject(st, id, r, now); return err }},
				{"withdraw", r.Check(), func(st *store.Store, id int) error { _, err := Withdraw(st, id, r, now); return err }},
			}
			wantErr := "--" + flag + " must not be blank"
			for _, tt := range attempts {
				t.Run(fmt.Sprintf("%s --%s %q", tt.name, flag, blank), func(t *testing.T) {
					st, id := openRecord(t, now)
					var ue *UsageError
					if !errors.As(tt.check, &ue) || !strings.Contains(tt.check.Error(), wantErr) {
						t.Errorf("Check() = %v, want a usage error containing %q", tt.check, wantErr)
					}
					if err := tt.do(st, id); !errors.As(err, &ue) || !strings.Contains(err.Error(), wantErr) {
						t.Errorf("error = %v, want a usage error containing %q", err, wantErr)
					}
					after, err := st.Get(id)
					if err != nil {
						t.Fatal(err)
					}
					if after.Status != model.StatusOpen || len(after.History) != 1 {
						t.Errorf("record changed after a refused answer: %+v", after)
					}
				})
			}
		}
	}
}
