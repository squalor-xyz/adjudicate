// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// fullRecord sets every field a record can hold, so the golden outputs show
// each one.
func fullRecord() model.Record {
	return model.Record{
		ID:      7,
		Title:   "Pick a storage engine",
		Status:  model.StatusSuperseded,
		Key:     "storage.engine",
		Tags:    []string{"infra", "perf"},
		AskedBy: "claude-opus",
		Created: "2026-03-04",
		Context: "why we need to choose\nsecond line",
		Options: []model.Option{
			{Label: "a", Summary: "Use A", Tradeoffs: "Younger project"},
			{Label: "b", Summary: "Use B"},
			{Label: "c"},
		},
		Recommendation: &model.Recommendation{Label: "a", Reasoning: "A is proven"},
		DecidedBy:      "owner",
		DecidedAt:      "2026-03-09",
		Decision:       "a",
		Rationale:      "fewer moving parts",
		Consequences:   "one more migration",
		Supersedes:     []int{2, 3},
		SupersededBy:   9,
		Refs:           []string{"docs/a.md", "https://example.com/x"},
		History: []model.HistoryEntry{
			{Date: "2026-03-04", Event: model.EventAsk},
			{Date: "2026-03-09", Event: model.EventDecide},
			{Date: "2026-03-12", Event: model.EventSupersede, Note: "by 9"},
		},
		Body: "Notes in *Markdown*.\n",
	}
}

func TestRecordTextShowsEveryField(t *testing.T) {
	var buf bytes.Buffer
	if err := RecordText(&buf, fullRecord()); err != nil {
		t.Fatal(err)
	}
	want := `ID: 7
Title: Pick a storage engine
Status: superseded
Key: storage.engine
Tags: infra, perf
Asked by: claude-opus
Created: 2026-03-04
Context:
  why we need to choose
  second line
Options:
  a: Use A
    tradeoffs: Younger project
  b: Use B
  c
Recommendation: a
Reasoning: A is proven
Decided by: owner
Decided at: 2026-03-09
Decision: a
Rationale: fewer moving parts
Consequences: one more migration
Supersedes: 2, 3
Superseded by: 9
Refs: docs/a.md, https://example.com/x
History:
  2026-03-04 ask
  2026-03-09 decide
  2026-03-12 supersede: by 9
Body:
Notes in *Markdown*.
`
	if got := buf.String(); got != want {
		t.Errorf("RecordText =\n%s\nwant\n%s", got, want)
	}
}

func TestRecordTextOmitsUnsetFields(t *testing.T) {
	var buf bytes.Buffer
	r := model.Record{ID: 1, Title: "Pick a store", Status: model.StatusOpen, Created: "2026-03-04"}
	if err := RecordText(&buf, r); err != nil {
		t.Fatal(err)
	}
	want := "ID: 1\nTitle: Pick a store\nStatus: open\nCreated: 2026-03-04\n"
	if got := buf.String(); got != want {
		t.Errorf("RecordText = %q, want %q", got, want)
	}
}

func TestRecordsTextPrintsOneAlignedLinePerRecord(t *testing.T) {
	tests := []struct {
		name string
		recs []model.Record
		want string
	}{
		{name: "no records prints nothing", want: ""},
		{
			name: "two records line up",
			recs: []model.Record{
				{ID: 1, Title: "Pick a store", Status: model.StatusOpen},
				{ID: 12, Title: "Use B", Status: model.StatusDecided},
			},
			want: "1   open     Pick a store\n12  decided  Use B\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := RecordsText(&buf, tt.recs); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("RecordsText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecordJSONShape(t *testing.T) {
	var buf bytes.Buffer
	if err := RecordJSON(&buf, fullRecord()); err != nil {
		t.Fatal(err)
	}
	want := `{
  "id": 7,
  "title": "Pick a storage engine",
  "status": "superseded",
  "key": "storage.engine",
  "tags": [
    "infra",
    "perf"
  ],
  "asked_by": "claude-opus",
  "decided_by": "owner",
  "created": "2026-03-04",
  "decided_at": "2026-03-09",
  "context": "why we need to choose\nsecond line",
  "options": [
    {
      "label": "a",
      "summary": "Use A",
      "tradeoffs": "Younger project"
    },
    {
      "label": "b",
      "summary": "Use B"
    },
    {
      "label": "c"
    }
  ],
  "recommendation": {
    "label": "a",
    "reasoning": "A is proven"
  },
  "decision": "a",
  "rationale": "fewer moving parts",
  "consequences": "one more migration",
  "supersedes": [
    2,
    3
  ],
  "superseded_by": 9,
  "refs": [
    "docs/a.md",
    "https://example.com/x"
  ],
  "history": [
    {
      "date": "2026-03-04",
      "event": "ask"
    },
    {
      "date": "2026-03-09",
      "event": "decide"
    },
    {
      "date": "2026-03-12",
      "event": "supersede",
      "note": "by 9"
    }
  ],
  "body": "Notes in *Markdown*.\n"
}
`
	if got := buf.String(); got != want {
		t.Errorf("RecordJSON =\n%s\nwant\n%s", got, want)
	}
}

func TestRecordJSONOmitsEmptyFieldsAndBody(t *testing.T) {
	var buf bytes.Buffer
	r := model.Record{ID: 1, Title: "Pick a store", Status: model.StatusOpen, Created: "2026-03-04"}
	if err := RecordJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	want := `{
  "id": 1,
  "title": "Pick a store",
  "status": "open",
  "created": "2026-03-04"
}
`
	if got := buf.String(); got != want {
		t.Errorf("RecordJSON = %q, want %q", got, want)
	}
}

func TestRecordsJSONWrapsTheRecords(t *testing.T) {
	tests := []struct {
		name string
		recs []model.Record
		want string
	}{
		{
			name: "no records is an empty array",
			want: "{\n  \"records\": []\n}\n",
		},
		{
			name: "one record",
			recs: []model.Record{{ID: 1, Title: "Pick a store", Status: model.StatusOpen, Created: "2026-03-04"}},
			want: `{
  "records": [
    {
      "id": 1,
      "title": "Pick a store",
      "status": "open",
      "created": "2026-03-04"
    }
  ]
}
`,
		},
		{
			// Added after the implementation, with the mutation pass: list
			// records have the same shape as show, body included (C87, C88).
			name: "a record with a body keeps the body key",
			recs: []model.Record{{ID: 1, Title: "Pick a store", Status: model.StatusOpen, Created: "2026-03-04", Body: "Notes.\n"}},
			want: `{
  "records": [
    {
      "id": 1,
      "title": "Pick a store",
      "status": "open",
      "created": "2026-03-04",
      "body": "Notes.\n"
    }
  ]
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := RecordsJSON(&buf, tt.recs); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("RecordsJSON =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestJSONDoesNotEscapeHTMLCharacters(t *testing.T) {
	var buf bytes.Buffer
	r := model.Record{ID: 1, Title: "a < b & c", Status: model.StatusOpen, Created: "2026-03-04"}
	if err := RecordJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	if want := `"title": "a < b & c"`; !bytes.Contains(buf.Bytes(), []byte(want)) {
		t.Errorf("RecordJSON does not contain %s:\n%s", want, buf.String())
	}
}

func TestRecordTextIndentsContinuationLinesDeeperThanTheirEntry(t *testing.T) {
	tests := []struct {
		name string
		r    model.Record
		want string
	}{
		{
			name: "option summary",
			r: model.Record{ID: 1, Title: "T", Status: model.StatusOpen, Created: "2026-03-04",
				Options: []model.Option{{Label: "a", Summary: "first\nsecond"}, {Label: "b"}}},
			want: "ID: 1\nTitle: T\nStatus: open\nCreated: 2026-03-04\nOptions:\n  a: first\n    second\n  b\n",
		},
		{
			name: "option tradeoffs",
			r: model.Record{ID: 1, Title: "T", Status: model.StatusOpen, Created: "2026-03-04",
				Options: []model.Option{{Label: "a", Summary: "Use A", Tradeoffs: "first\nsecond"}}},
			want: "ID: 1\nTitle: T\nStatus: open\nCreated: 2026-03-04\nOptions:\n  a: Use A\n    tradeoffs: first\n      second\n",
		},
		{
			name: "history note",
			r: model.Record{ID: 1, Title: "T", Status: model.StatusOpen, Created: "2026-03-04",
				History: []model.HistoryEntry{{Date: "2026-03-04", Event: model.EventAsk, Note: "first\nsecond"}, {Date: "2026-03-05", Event: model.EventDecide}}},
			want: "ID: 1\nTitle: T\nStatus: open\nCreated: 2026-03-04\nHistory:\n  2026-03-04 ask: first\n    second\n  2026-03-05 decide\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := RecordText(&buf, tt.r); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("RecordText = %q, want %q", got, tt.want)
			}
		})
	}
}

// Trailing line breaks are dropped for display only (RecordText takes the
// record by value, so the stored value cannot change).
func TestRecordTextDropsAllTrailingLineBreaksOfAField(t *testing.T) {
	tests := []struct {
		name    string
		context string
		want    string
	}{
		{name: "one line", context: "why\n\n", want: "Context: why\n"},
		{name: "several lines", context: "why\nmore\n\n\n", want: "Context:\n  why\n  more\n"},
		{name: "only line breaks", context: "\n\n", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := model.Record{ID: 1, Title: "T", Status: model.StatusOpen, Created: "2026-03-04", Context: tt.context}
			if err := RecordText(&buf, r); err != nil {
				t.Fatal(err)
			}
			want := "ID: 1\nTitle: T\nStatus: open\nCreated: 2026-03-04\n" + tt.want
			if got := buf.String(); got != want {
				t.Errorf("RecordText = %q, want %q", got, want)
			}
		})
	}
}
