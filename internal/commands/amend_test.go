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

// amendStore returns a store with record 1 open (options a and b, tradeoffs
// for a) and record 2 decided, both asked on 2026-03-04.
func amendStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	asked := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	in := AskInput{Title: "Pick a store", Context: "why\nmore", Options: []string{"a:Use A", "b:Use B"},
		Tradeoffs: []string{"a:slow"}, Recommend: "a"}
	for range 2 {
		if _, err := Ask(st, in, asked); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Decide(st, 2, DecideInput{Option: "a", HasOption: true, Why: "proven"}, asked); err != nil {
		t.Fatal(err)
	}
	return st
}

var amendDay = time.Date(2026, 10, 17, 23, 30, 0, 0, time.UTC)

func TestAmendInputCheck(t *testing.T) {
	tests := []struct {
		name string
		in   AmendInput
		want string // "" means valid
	}{
		{"title", AmendInput{Title: "T2", HasTitle: true}, ""},
		{"nothing given", AmendInput{}, "give at least one"},
		{"blank title", AmendInput{Title: " ", HasTitle: true}, "--title must not be blank"},
		{"empty title", AmendInput{HasTitle: true}, "--title must not be blank"},
		{"multi-line title", AmendInput{Title: "a\nb", HasTitle: true}, "--title must be one line"},
		{"blank context", AmendInput{Context: "\n ", HasContext: true}, "--context must not be blank"},
		{"blank rationale", AmendInput{Rationale: "\t", HasRationale: true}, "--rationale must not be blank"},
		{"summary without a colon", AmendInput{Summaries: []string{"a"}}, "LABEL:TEXT"},
		{"summary with an empty label", AmendInput{Summaries: []string{": x"}}, "label is empty"},
		{"summary twice for one option", AmendInput{Summaries: []string{"a:x", "a:y"}}, `"a"`},
		{"tradeoffs without a colon", AmendInput{Tradeoffs: []string{"a"}}, "LABEL:TEXT"},
		{"tradeoffs twice for one option", AmendInput{Tradeoffs: []string{"a:x", " a :y"}}, `"a"`},
		{"summary and tradeoffs for one option", AmendInput{Summaries: []string{"a:x"}, Tradeoffs: []string{"a:y"}}, ""},
		{"tradeoffs cleared", AmendInput{Tradeoffs: []string{"a:"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Check()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Check = %v, want nil", err)
				}
				return
			}
			var ue *UsageError
			if !errors.As(err, &ue) || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Check = %v, want a *UsageError containing %q", err, tt.want)
			}
		})
	}
}

func TestAmendWritesTheRecordAndReturnsIt(t *testing.T) {
	st := amendStore(t)
	for _, id := range []int{1, 2} {
		before, _ := st.Get(id)
		got, err := Amend(st, id, AmendInput{
			Title: " Pick a datastore ", HasTitle: true,
			Summaries: []string{" b : Use B, pinned "},
			Tradeoffs: []string{"a:"},
		}, amendDay)
		if err != nil {
			t.Fatal(err)
		}
		stored, err := st.Get(id)
		if err != nil || !reflect.DeepEqual(stored, got) {
			t.Fatalf("record %d: stored %+v (%v), returned %+v", id, stored, err, got)
		}
		if got.Title != " Pick a datastore " { // as typed, like ask
			t.Errorf("title = %q, want it as typed", got.Title)
		}
		if got.Options[1].Summary != "Use B, pinned" || got.Options[0].Tradeoffs != "" {
			t.Errorf("options = %+v, want the summary trimmed and the tradeoffs cleared", got.Options)
		}
		last := got.History[len(got.History)-1]
		want := model.HistoryEntry{Date: "2026-10-17", Event: model.EventAmend, Note: "title, tradeoffs a, summary b", Was: &model.Was{
			Title: &before.Title,
			Options: []model.OptionWas{
				{Label: "a", Tradeoffs: &before.Options[0].Tradeoffs},
				{Label: "b", Summary: &before.Options[1].Summary},
			},
		}}
		if len(got.History) != len(before.History)+1 || !reflect.DeepEqual(last, want) {
			t.Errorf("record %d history = %+v, want one more entry %+v", id, got.History, want)
		}
	}
}

func TestAmendRefusesAndWritesNothing(t *testing.T) {
	tests := []struct {
		name  string
		id    int
		in    AmendInput
		usage bool   // a *UsageError, exit 2
		want  string // text in the error
		is    error
	}{
		{"unknown option label", 1, AmendInput{Summaries: []string{"z:x"}}, true, `"z"`, nil},
		{"unknown label in tradeoffs", 2, AmendInput{Tradeoffs: []string{"z:x"}}, true, `"z"`, nil},
		{"bad input", 1, AmendInput{Title: " ", HasTitle: true}, true, "--title", nil},
		{"rationale of an open record", 1, AmendInput{Rationale: "why", HasRationale: true}, false, "open", model.ErrInvalidTransition},
		{"nothing changes", 2, AmendInput{Title: "Pick a store", HasTitle: true}, false, "record 2: nothing to change", model.ErrNoChange},
		{"unknown record", 9, AmendInput{Title: "T2", HasTitle: true}, false, "not found", store.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := amendStore(t)
			before, _ := st.List()
			_, err := Amend(st, tt.id, tt.in, amendDay)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Amend err = %v, want one containing %q", err, tt.want)
			}
			var ue *UsageError
			if errors.As(err, &ue) != tt.usage {
				t.Errorf("usage error = %v, want %v (%v)", !tt.usage, tt.usage, err)
			}
			if tt.is != nil && !errors.Is(err, tt.is) {
				t.Errorf("err = %v, want one matching %v", err, tt.is)
			}
			if after, _ := st.List(); !reflect.DeepEqual(after, before) {
				t.Errorf("a refused amend changed the store\nbefore %+v\nafter  %+v", before, after)
			}
		})
	}
}
