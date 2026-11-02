// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

func narrowedRecords() []model.Record {
	old := model.Record{
		ID: 1, Title: "Free text answers", Status: model.StatusDecided, Key: "ui.answers", Created: "2026-03-04",
		DecidedAt: "2026-03-05", Decision: "free text\nalways", Rationale: "simple\nand clear",
		NarrowedBy: []model.Narrowing{{ID: 2, Scope: "only for structured questions"}, {ID: 4, Scope: "not for the sidebar"}},
	}
	narrower := model.Record{
		ID: 2, Title: "Structured answers", Status: model.StatusDecided, Created: "2026-03-04",
		DecidedAt: "2026-03-05", Decision: "pick one", Rationale: "faster", Narrows: []int{1, 3},
	}
	plain := model.Record{
		ID: 3, Title: "Plain", Status: model.StatusDecided, Created: "2026-03-04",
		DecidedAt: "2026-03-05", Decision: "yes", Rationale: "because",
	}
	return []model.Record{old, narrower, plain}
}

// The scope is on the line of the narrowed record, and the narrowing record
// names what it narrows; a multi-line decision keeps its continuation lines
// before them.
func TestContextTextShowsNarrowing(t *testing.T) {
	var buf bytes.Buffer
	if err := ContextText(&buf, narrowedRecords()); err != nil {
		t.Fatal(err)
	}
	const want = `- 1 Free text answers [ui.answers]: free text
    always
  Narrowed by: 2 (only for structured questions)
  Narrowed by: 4 (not for the sidebar)
  Rationale: simple
- 2 Structured answers: pick one
  Narrows: 1, 3
  Rationale: faster
- 3 Plain: yes
  Rationale: because
`
	if buf.String() != want {
		t.Errorf("ContextText =\n%s\nwant\n%s", buf.String(), want)
	}
}

// With a budget the omitted line follows the entries as before, and the size
// counts the narrowing lines (the CLI measures this output).
func TestContextTextOmittedCountsNarrowing(t *testing.T) {
	var plain, narrowed bytes.Buffer
	recs := narrowedRecords()
	bare := append([]model.Record(nil), recs...)
	bare[0].NarrowedBy, bare[1].Narrows = nil, nil
	if err := ContextTextOmitted(&plain, bare[:2], 1); err != nil {
		t.Fatal(err)
	}
	if err := ContextTextOmitted(&narrowed, recs[:2], 1); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(narrowed.String(), "\n1 decision omitted\n") {
		t.Errorf("output does not end with the omitted line:\n%s", narrowed.String())
	}
	extra := len("  Narrowed by: 2 (only for structured questions)\n") + len("  Narrowed by: 4 (not for the sidebar)\n") + len("  Narrows: 1, 3\n")
	if narrowed.Len()-plain.Len() != extra {
		t.Errorf("narrowing adds %d bytes, want %d", narrowed.Len()-plain.Len(), extra)
	}
}

// The JSON entries carry the links as optional keys, after the existing keys;
// an entry without a link has neither key.
func TestContextJSONCarriesNarrowing(t *testing.T) {
	var buf bytes.Buffer
	if err := ContextJSON(&buf, narrowedRecords()); err != nil {
		t.Fatal(err)
	}
	const want = `{
  "decisions": [
    {
      "id": 1,
      "title": "Free text answers",
      "key": "ui.answers",
      "decision": "free text\nalways",
      "rationale": "simple",
      "narrowed_by": [
        {
          "id": 2,
          "scope": "only for structured questions"
        },
        {
          "id": 4,
          "scope": "not for the sidebar"
        }
      ]
    },
    {
      "id": 2,
      "title": "Structured answers",
      "decision": "pick one",
      "rationale": "faster",
      "narrows": [
        1,
        3
      ]
    },
    {
      "id": 3,
      "title": "Plain",
      "decision": "yes",
      "rationale": "because"
    }
  ]
}
`
	if buf.String() != want {
		t.Errorf("ContextJSON =\n%s\nwant\n%s", buf.String(), want)
	}
}

// show prints one Narrowed by line per narrowing record and one Narrows line,
// after Superseded by and before Refs.
func TestRecordTextShowsNarrowing(t *testing.T) {
	recs := narrowedRecords()
	recs[0].Supersedes = []int{9}
	recs[0].Refs = []string{"S51"}
	var buf bytes.Buffer
	if err := RecordText(&buf, recs[0]); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	const block = "Supersedes: 9\nNarrowed by: 2 (only for structured questions)\nNarrowed by: 4 (not for the sidebar)\nRefs: S51\n"
	if !strings.Contains(text, block) {
		t.Errorf("RecordText does not contain\n%s\nin\n%s", block, text)
	}
	buf.Reset()
	if err := RecordText(&buf, recs[1]); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Rationale: faster\nNarrows: 1, 3\n") {
		t.Errorf("RecordText does not print the Narrows line:\n%s", buf.String())
	}
	buf.Reset()
	if err := RecordText(&buf, recs[2]); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Narrow") {
		t.Errorf("a record with no narrowing prints a narrow line:\n%s", buf.String())
	}
}
