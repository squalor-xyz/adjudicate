// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const testDate = "2026-10-15"

// validRecord returns a minimal record that passes Validate for status s.
func validRecord(s Status) Record {
	r := Record{ID: 1, Title: "Pick a store", Status: s, Created: "2026-10-01"}
	switch s {
	case StatusDecided:
		r.Decision, r.Rationale, r.DecidedAt = "files", "plain text", "2026-10-02"
	case StatusRejected, StatusWithdrawn:
		r.Rationale, r.DecidedAt = "not needed", "2026-10-02"
	case StatusSuperseded:
		r.Decision, r.Rationale, r.DecidedAt = "files", "plain text", "2026-10-02"
		r.SupersededBy = 2
	}
	return r
}

var allStatuses = []Status{StatusOpen, StatusDecided, StatusRejected, StatusWithdrawn, StatusSuperseded}

func TestRecordFieldsFollowCoreConceptsTable(t *testing.T) {
	// PROJECT.md Core concepts rows in order, without kind (C30), then history.
	want := []string{
		"id", "title", "status", "key", "tags", "asked_by", "decided_by",
		"created", "decided_at", "context", "options", "recommendation",
		"decision", "rationale", "consequences", "supersedes",
		"superseded_by", "refs", "history",
	}
	typ := reflect.TypeOf(Record{})
	var got []string
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "-" {
			continue // Body is the Markdown after the frontmatter, not a frontmatter field (file.go)
		}
		got = append(got, name)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("json field names/order\n got %v\nwant %v", got, want)
	}
}

func TestOptionFields(t *testing.T) {
	typ := reflect.TypeOf(Option{})
	var got []string
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		got = append(got, name)
	}
	want := []string{"label", "summary", "tradeoffs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("option json names %v, want %v", got, want)
	}
}

func TestMarshalIsDeterministicAndOmitsEmptyOptionals(t *testing.T) {
	r := validRecord(StatusOpen)
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":1,"title":"Pick a store","status":"open","created":"2026-10-01"}`
	if string(got) != want {
		t.Fatalf("minimal open record\n got %s\nwant %s", got, want)
	}

	full := Record{
		ID: 3, Title: "T", Status: StatusDecided, Key: "k", Tags: []string{"a"},
		AskedBy: "agent", DecidedBy: "owner", Created: "2026-10-01",
		DecidedAt: "2026-10-02", Context: "c",
		Options:        []Option{{Label: "x", Summary: "s", Tradeoffs: "t"}},
		Recommendation: &Recommendation{Label: "x", Reasoning: "r"},
		Decision:       "x", Rationale: "why", Consequences: "cons",
		Supersedes: []int{1}, SupersededBy: 0, Refs: []string{"S01"},
		History: []HistoryEntry{{Date: "2026-10-02", Event: EventDecide, Note: "n"}},
	}
	got, err = json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want = `{"id":3,"title":"T","status":"decided","key":"k","tags":["a"],` +
		`"asked_by":"agent","decided_by":"owner","created":"2026-10-01",` +
		`"decided_at":"2026-10-02","context":"c",` +
		`"options":[{"label":"x","summary":"s","tradeoffs":"t"}],` +
		`"recommendation":{"label":"x","reasoning":"r"},` +
		`"decision":"x","rationale":"why","consequences":"cons",` +
		`"supersedes":[1],"refs":["S01"],` +
		`"history":[{"date":"2026-10-02","event":"decide","note":"n"}]}`
	if string(got) != want {
		t.Fatalf("full record\n got %s\nwant %s", got, want)
	}
}

func TestValidateAcceptsMinimalRecordForEachStatus(t *testing.T) {
	for _, s := range allStatuses {
		t.Run(string(s), func(t *testing.T) {
			r := validRecord(s)
			if err := r.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidateAcceptsOptionsAndRecommendation(t *testing.T) {
	r := validRecord(StatusOpen)
	r.Options = []Option{{Label: "a"}, {Label: "b", Summary: "s", Tradeoffs: "t"}}
	r.Recommendation = &Recommendation{Label: "b", Reasoning: "cheaper"}
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// A recommendation may be free text with no option label (C167): it does not
// need any option, and a label that is not an option is still refused.
func TestValidateAcceptsATextOnlyRecommendation(t *testing.T) {
	for _, opts := range [][]Option{nil, {{Label: "a"}, {Label: "b"}}} {
		r := validRecord(StatusOpen)
		r.Options = opts
		r.Recommendation = &Recommendation{Text: "Use neither; keep the files", Reasoning: "simpler"}
		if err := r.Validate(); err != nil {
			t.Errorf("Validate() with %d options = %v, want nil", len(opts), err)
		}
	}

	r := validRecord(StatusOpen)
	r.Options = []Option{{Label: "a"}}
	r.Recommendation = &Recommendation{Label: "z", Reasoning: "because"}
	err := r.Validate()
	if err == nil || !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "recommendation") {
		t.Errorf("Validate() with an unknown label = %v, want an invalid recommendation error", err)
	}
}

func TestMarshalOmitsTheRecommendationFieldThatIsNotSet(t *testing.T) {
	r := Record{ID: 1, Title: "T", Status: StatusOpen, Created: "2026-10-01"}
	r.Recommendation = &Recommendation{Text: "free", Reasoning: "r"}
	got, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"id":1,"title":"T","status":"open","created":"2026-10-01","recommendation":{"text":"free","reasoning":"r"}}`
	if string(got) != want {
		t.Errorf("text recommendation\n got %s\nwant %s", got, want)
	}
	r.Recommendation = &Recommendation{Label: "a"}
	r.Options = []Option{{Label: "a"}}
	got, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"recommendation":{"label":"a"}}`; !strings.Contains(string(got), want) {
		t.Errorf("label recommendation = %s, want it to end with %s (no text key)", got, want)
	}
}

func TestValidateRejects(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		mutate func(*Record)
		field  string // the error must carry the prefix "invalid record: <field>:"
	}{
		{"missing id", StatusOpen, func(r *Record) { r.ID = 0 }, "id"},
		{"negative id", StatusOpen, func(r *Record) { r.ID = -3 }, "id"},
		{"missing title", StatusOpen, func(r *Record) { r.Title = "" }, "title"},
		{"blank title", StatusOpen, func(r *Record) { r.Title = "  " }, "title"},
		{"multi-line title", StatusOpen, func(r *Record) { r.Title = "a\nb" }, "title"},
		{"title with carriage return", StatusOpen, func(r *Record) { r.Title = "a\rb" }, "title"},
		{"missing status", StatusOpen, func(r *Record) { r.Status = "" }, "status"},
		{"unknown status", StatusOpen, func(r *Record) { r.Status = "accepted" }, "status"},
		{"missing created", StatusOpen, func(r *Record) { r.Created = "" }, "created"},
		{"created not a date", StatusOpen, func(r *Record) { r.Created = "15/10/2026" }, "created"},
		{"created not zero padded", StatusOpen, func(r *Record) { r.Created = "2026-1-5" }, "created"},
		{"created with time", StatusOpen, func(r *Record) { r.Created = "2026-10-01T10:00:00Z" }, "created"},
		{"open with bad decided_at", StatusOpen, func(r *Record) { r.DecidedAt = "soon" }, "decided_at"},
		{"decided missing decision", StatusDecided, func(r *Record) { r.Decision = "" }, "decision"},
		{"decided blank decision", StatusDecided, func(r *Record) { r.Decision = " \t" }, "decision"},
		{"decided blank rationale", StatusDecided, func(r *Record) { r.Rationale = " \t" }, "rationale"},
		{"rejected blank rationale", StatusRejected, func(r *Record) { r.Rationale = "  " }, "rationale"},
		{"decided missing rationale", StatusDecided, func(r *Record) { r.Rationale = "" }, "rationale"},
		{"decided missing decided_at", StatusDecided, func(r *Record) { r.DecidedAt = "" }, "decided_at"},
		{"decided bad decided_at", StatusDecided, func(r *Record) { r.DecidedAt = "2026-13-01" }, "decided_at"},
		{"rejected missing rationale", StatusRejected, func(r *Record) { r.Rationale = "" }, "rationale"},
		{"rejected missing decided_at", StatusRejected, func(r *Record) { r.DecidedAt = "" }, "decided_at"},
		{"withdrawn missing rationale", StatusWithdrawn, func(r *Record) { r.Rationale = "" }, "rationale"},
		{"withdrawn missing decided_at", StatusWithdrawn, func(r *Record) { r.DecidedAt = "" }, "decided_at"},
		{"superseded missing decision", StatusSuperseded, func(r *Record) { r.Decision = "" }, "decision"},
		{"superseded missing rationale", StatusSuperseded, func(r *Record) { r.Rationale = "" }, "rationale"},
		{"superseded missing decided_at", StatusSuperseded, func(r *Record) { r.DecidedAt = "" }, "decided_at"},
		{"superseded missing superseded_by", StatusSuperseded, func(r *Record) { r.SupersededBy = 0 }, "superseded_by"},
		{"superseded by itself", StatusSuperseded, func(r *Record) { r.SupersededBy = r.ID }, "superseded_by"},
		{"negative superseded_by on decided", StatusDecided, func(r *Record) { r.SupersededBy = -1 }, "superseded_by"},
		{"history entry bad date", StatusOpen, func(r *Record) {
			r.History = []HistoryEntry{{Date: "10/15/2026", Event: EventAsk}}
		}, "history"},
		{"history entry empty date", StatusOpen, func(r *Record) {
			r.History = []HistoryEntry{{Event: EventAsk}}
		}, "history"},
		{"history entry empty event", StatusOpen, func(r *Record) {
			r.History = []HistoryEntry{{Date: "2026-10-01"}}
		}, "history"},
		{"second history entry bad", StatusOpen, func(r *Record) {
			r.History = []HistoryEntry{{Date: "2026-10-01", Event: EventAsk}, {Date: "x", Event: EventDecide}}
		}, "history"},
		{"supersedes itself", StatusDecided, func(r *Record) { r.Supersedes = []int{r.ID} }, "supersedes"},
		{"supersedes non-positive id", StatusDecided, func(r *Record) { r.Supersedes = []int{0} }, "supersedes"},
		{"option without label", StatusOpen, func(r *Record) { r.Options = []Option{{Label: ""}} }, "options"},
		{"duplicate option labels", StatusOpen, func(r *Record) {
			r.Options = []Option{{Label: "a"}, {Label: "b"}, {Label: "a", Summary: "again"}}
		}, "options"},
		{"recommendation not an option", StatusOpen, func(r *Record) {
			r.Options = []Option{{Label: "a"}, {Label: "b"}}
			r.Recommendation = &Recommendation{Label: "c"}
		}, "recommendation"},
		{"recommendation without options", StatusOpen, func(r *Record) {
			r.Recommendation = &Recommendation{Label: "a"}
		}, "recommendation"},
		{"recommendation with a label and text", StatusOpen, func(r *Record) {
			r.Options = []Option{{Label: "a"}}
			r.Recommendation = &Recommendation{Label: "a", Text: "or this"}
		}, "recommendation"},
		{"recommendation with blank text", StatusOpen, func(r *Record) {
			r.Recommendation = &Recommendation{Text: "  \n"}
		}, "recommendation"},
		{"recommendation with only reasoning and no options", StatusOpen, func(r *Record) {
			r.Recommendation = &Recommendation{Reasoning: "because"}
		}, "recommendation"},
		{"recommendation without label", StatusOpen, func(r *Record) {
			r.Options = []Option{{Label: "a"}}
			r.Recommendation = &Recommendation{Reasoning: "because"}
		}, "recommendation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord(tt.status)
			tt.mutate(&r)
			err := r.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not match ErrInvalid", err)
			}
			if want := ErrInvalid.Error() + ": " + tt.field + ":"; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not contain %q", err, want)
			}
		})
	}
}

func TestValidateAcceptsEveryKnownHistoryEvent(t *testing.T) {
	r := validRecord(StatusOpen)
	for _, e := range []Event{EventAsk, EventDecide, EventReject, EventWithdraw, EventSupersede, EventRenumber} {
		r.History = append(r.History, HistoryEntry{Date: "2026-10-01", Event: e, Note: "n"})
	}
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

// adj import-adr writes one import entry, with the note "from FILE", in place
// of the ask and decide entries a record made by hand has (S22).
func TestValidateAcceptsAnImportEntry(t *testing.T) {
	r := validRecord(StatusOpen)
	r.History = []HistoryEntry{{Date: "2026-10-01", Event: EventImport, Note: "from docs/adr/0001-x.md"}}
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if EventImport != "import" || !EventImport.Known() {
		t.Errorf("EventImport = %q, Known() = %v; want %q and true", EventImport, EventImport.Known(), "import")
	}
	// Only an amend entry has was.
	r.History[0].Was = &Was{Title: ptr("old")}
	err := r.Validate()
	if err == nil || !strings.Contains(err.Error(), "only an amend entry has was") {
		t.Errorf("Validate() = %v, want the error that only an amend entry has was", err)
	}
}

func TestValidateDoesNotRequireDecidedBy(t *testing.T) {
	// decided_by is attribution, not authentication; lint warns, Validate does not (C27).
	r := validRecord(StatusDecided)
	r.DecidedBy = ""
	if err := r.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	r := Record{Status: "bogus"}
	err := r.Validate()
	if err == nil {
		t.Fatal("Validate() = nil")
	}
	for _, f := range []string{"id", "title", "status", "created"} {
		if want := ErrInvalid.Error() + ": " + f + ":"; !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestStatusValid(t *testing.T) {
	for _, s := range allStatuses {
		if !s.Valid() {
			t.Errorf("%q.Valid() = false", s)
		}
	}
	for _, s := range []Status{"", "Open", "accepted", "question"} {
		if s.Valid() {
			t.Errorf("%q.Valid() = true", s)
		}
	}
}
