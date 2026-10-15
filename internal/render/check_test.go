// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/query"
)

func checkSampleResult() (decided, open model.Record) {
	decided = model.Record{
		ID: 2, Title: "Cache layer", Status: model.StatusDecided, Key: "storage.cache",
		Created: "2026-03-04", DecidedAt: "2026-03-05", Decision: "redis", Rationale: "fast",
	}
	open = model.Record{ID: 10, Title: "Pick a store", Status: model.StatusOpen, Created: "2026-03-04"}
	return decided, open
}

func TestCheckText(t *testing.T) {
	decided, open := checkSampleResult()
	multi := decided
	multi.ID = 3
	multi.Title = "Multi"
	multi.Decision = "line one\nline two\n"
	multi.Rationale = "why\nbecause\n\n"
	bare := decided
	bare.ID = 4
	bare.Title = "Bare"
	bare.Decision = "yes"
	bare.Rationale = ""

	rejected := model.Record{ID: 5, Title: "Theme", Status: model.StatusRejected, Created: "2026-03-04", Rationale: "not needed"}
	withdrawn := model.Record{ID: 6, Title: "Queue", Status: model.StatusWithdrawn, Created: "2026-03-04"}
	superseded := model.Record{ID: 7, Title: "Old store", Status: model.StatusSuperseded, SupersededBy: 2, Created: "2026-03-04", Decision: "mysql"}

	tests := []struct {
		name string
		res  query.Result
		want string
	}{
		{
			name: "closed lists the closed records, with the replacement of a superseded one",
			res:  query.Result{Outcome: query.Closed, Closed: []model.Record{rejected, withdrawn, superseded}},
			want: "closed\n5  rejected  Theme\n  Rationale: not needed\n6  withdrawn  Queue\n7  superseded  Old store\n  Superseded by: 2\n",
		},
		{
			name: "decided, open and closed: in that order",
			res:  query.Result{Outcome: query.Decided, Decided: []model.Record{decided}, Open: []model.Record{open}, Closed: []model.Record{superseded}},
			want: "decided\n2  decided  Cache layer\n  Decision: redis\n  Rationale: fast\n10  open  Pick a store\n7  superseded  Old store\n  Superseded by: 2\n",
		},
		{
			name: "a withdrawn record shows its rationale",
			res: query.Result{Outcome: query.Closed, Closed: []model.Record{
				{ID: 6, Title: "Queue", Status: model.StatusWithdrawn, Rationale: "no longer needed"}}},
			want: "closed\n6  withdrawn  Queue\n  Rationale: no longer needed\n",
		},
		{
			name: "a superseded record shows its replacement, then its rationale",
			res: query.Result{Outcome: query.Closed, Closed: []model.Record{
				{ID: 7, Title: "Old store", Status: model.StatusSuperseded, SupersededBy: 2, Decision: "mysql", Rationale: "familiar"}}},
			want: "closed\n7  superseded  Old store\n  Superseded by: 2\n  Rationale: familiar\n",
		},
		{
			name: "a multi-line closed rationale goes under its label and loses trailing breaks",
			res: query.Result{Outcome: query.Closed, Closed: []model.Record{
				{ID: 5, Title: "Theme", Status: model.StatusRejected, Rationale: "too slow\nand too big\n\n"}}},
			want: "closed\n5  rejected  Theme\n  Rationale:\n    too slow\n    and too big\n",
		},
		{
			name: "a closed rationale of only line breaks prints no line",
			res: query.Result{Outcome: query.Closed, Closed: []model.Record{
				{ID: 5, Title: "Theme", Status: model.StatusRejected, Rationale: "\n\n"}}},
			want: "closed\n5  rejected  Theme\n",
		},
		{
			name: "an open record shows no rationale",
			res: query.Result{Outcome: query.Open, Open: []model.Record{
				{ID: 10, Title: "Pick a store", Status: model.StatusOpen, Rationale: "stray"}}},
			want: "open\n10  open  Pick a store\n",
		},
		{name: "not found is one line", res: query.Result{Outcome: query.NotFound}, want: "not found\n"},
		{
			name: "open lists the open records",
			res:  query.Result{Outcome: query.Open, Open: []model.Record{open}},
			want: "open\n10  open  Pick a store\n",
		},
		{
			name: "decided shows the decision and the rationale",
			res:  query.Result{Outcome: query.Decided, Decided: []model.Record{decided}},
			want: "decided\n2  decided  Cache layer\n  Decision: redis\n  Rationale: fast\n",
		},
		{
			name: "decided and open: decided records first, then open ones",
			res:  query.Result{Outcome: query.Decided, Decided: []model.Record{decided, bare}, Open: []model.Record{open}},
			want: "decided\n2  decided  Cache layer\n  Decision: redis\n  Rationale: fast\n4  decided  Bare\n  Decision: yes\n10  open  Pick a store\n",
		},
		{
			name: "multi-line values go under their labels and lose trailing breaks",
			res:  query.Result{Outcome: query.Decided, Decided: []model.Record{multi}},
			want: "decided\n3  decided  Multi\n  Decision:\n    line one\n    line two\n  Rationale:\n    why\n    because\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := CheckText(&b, tt.res); err != nil {
				t.Fatal(err)
			}
			if b.String() != tt.want {
				t.Errorf("CheckText =\n%q\nwant\n%q", b.String(), tt.want)
			}
		})
	}
}

func TestCheckJSON(t *testing.T) {
	decided, open := checkSampleResult()
	decided.Body = "Notes.\n"
	tests := []struct {
		name string
		res  query.Result
		want string
	}{
		{
			name: "not found has empty arrays, not null",
			res:  query.Result{Query: "x", Outcome: query.NotFound},
			want: "{\n  \"query\": \"x\",\n  \"result\": \"not found\",\n  \"decided\": [],\n  \"open\": [],\n  \"closed\": []\n}\n",
		},
		{
			name: "records are the list shape, with body; HTML is not escaped",
			res: query.Result{Query: "a<b", Outcome: query.Decided, Decided: []model.Record{decided}, Open: []model.Record{open},
				Closed: []model.Record{{ID: 7, Title: "Old store", Status: model.StatusSuperseded, SupersededBy: 2, Created: "2026-03-04", Decision: "mysql", Rationale: "old"}}},
			want: `{
  "query": "a<b",
  "result": "decided",
  "decided": [
    {
      "id": 2,
      "title": "Cache layer",
      "status": "decided",
      "key": "storage.cache",
      "created": "2026-03-04",
      "decided_at": "2026-03-05",
      "decision": "redis",
      "rationale": "fast",
      "body": "Notes.\n"
    }
  ],
  "open": [
    {
      "id": 10,
      "title": "Pick a store",
      "status": "open",
      "created": "2026-03-04"
    }
  ],
  "closed": [
    {
      "id": 7,
      "title": "Old store",
      "status": "superseded",
      "created": "2026-03-04",
      "decision": "mysql",
      "rationale": "old",
      "superseded_by": 2
    }
  ]
}
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b bytes.Buffer
			if err := CheckJSON(&b, tt.res); err != nil {
				t.Fatal(err)
			}
			if b.String() != tt.want {
				t.Errorf("CheckJSON =\n%s\nwant\n%s", b.String(), tt.want)
			}
		})
	}
}
