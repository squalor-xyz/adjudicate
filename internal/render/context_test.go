// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// contextRecords are the four decided records of the golden files. They carry
// fields that the context output must leave out (tags, context, options,
// consequences, history, body), so the golden files show that nothing else of
// a record is printed.
func contextRecords() []model.Record {
	dec := func(id int, title, key, decision, rationale string) model.Record {
		return model.Record{
			ID: id, Title: title, Status: model.StatusDecided, Key: key,
			Tags: []string{"infra"}, AskedBy: "claude-opus", DecidedBy: "owner",
			Created: "2026-03-04", DecidedAt: "2026-03-05", Context: "why we ask",
			Options:      []model.Option{{Label: "a", Summary: "Use A", Tradeoffs: "slow"}},
			Decision:     decision,
			Rationale:    rationale,
			Consequences: "later",
			Refs:         []string{"ref"},
			History:      []model.HistoryEntry{{Date: "2026-03-04", Event: model.EventAsk}},
			Body:         "Notes.\n",
		}
	}
	return []model.Record{
		dec(2, "Cache layer", "storage.cache", "redis", "fast"),
		dec(6, "Database", "storage.db", "sqlite", "embedded\nno server to run"),
		dec(7, "Logging format", "", "Use JSON lines.\nOne object per line.", "easy to grep"),
		dec(8, "UI theme", "ui.theme", "dark", "looks good"),
	}
}

func golden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestContextTextMatchesTheGoldenFile(t *testing.T) {
	var buf bytes.Buffer
	if err := ContextText(&buf, contextRecords()); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), golden(t, "context.md"); got != want {
		t.Errorf("ContextText =\n%s\nwant testdata/context.md\n%s", got, want)
	}
}

func TestContextJSONMatchesTheGoldenFile(t *testing.T) {
	var buf bytes.Buffer
	if err := ContextJSON(&buf, contextRecords()); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), golden(t, "context.json"); got != want {
		t.Errorf("ContextJSON =\n%s\nwant testdata/context.json\n%s", got, want)
	}
}

func TestContextTextShape(t *testing.T) {
	dec := func(decision, rationale string) model.Record {
		return model.Record{ID: 3, Title: "T", Status: model.StatusDecided, Created: "2026-03-04", Decision: decision, Rationale: rationale}
	}
	tests := []struct {
		name string
		recs []model.Record
		want string
	}{
		{name: "no records prints nothing", want: ""},
		{name: "a record with no key has no brackets", recs: []model.Record{dec("yes", "because")}, want: "- 3 T: yes\n  Rationale: because\n"},
		{name: "only the first line of the rationale", recs: []model.Record{dec("yes", "first\nsecond\nthird")}, want: "- 3 T: yes\n  Rationale: first\n"},
		{name: "leading blank lines and space of the rationale are skipped", recs: []model.Record{dec("yes", "\n \n  first  \nsecond")}, want: "- 3 T: yes\n  Rationale: first\n"},
		{name: "a rationale of only line breaks is left out", recs: []model.Record{dec("yes", "\n\n")}, want: "- 3 T: yes\n"},
		{name: "trailing line breaks of the decision are dropped", recs: []model.Record{dec("yes\n\n", "because")}, want: "- 3 T: yes\n  Rationale: because\n"},
		{name: "records are printed in the order given", recs: []model.Record{
			{ID: 9, Title: "Nine", Decision: "n", Rationale: "r"},
			{ID: 4, Title: "Four", Decision: "f", Rationale: "r"},
		}, want: "- 9 Nine: n\n  Rationale: r\n- 4 Four: f\n  Rationale: r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := ContextText(&buf, tt.recs); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("ContextText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContextJSONShape(t *testing.T) {
	var buf bytes.Buffer
	if err := ContextJSON(&buf, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := buf.String(), "{\n  \"decisions\": []\n}\n"; got != want {
		t.Errorf("ContextJSON(nil) = %q, want %q", got, want)
	}

	buf.Reset()
	rec := model.Record{ID: 3, Title: "A <b> & c", Status: model.StatusDecided, Created: "2026-03-04", Decision: "x\n\n", Rationale: "first\nsecond"}
	if err := ContextJSON(&buf, []model.Record{rec}); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, `"title": "A <b> & c"`) {
		t.Errorf("HTML characters are escaped:\n%s", got)
	}
	if !strings.Contains(got, `"rationale": "first"`) || strings.Contains(got, "second") {
		t.Errorf("rationale is not the first line only:\n%s", got)
	}
	if !strings.Contains(got, `"decision": "x",`) {
		t.Errorf("trailing line breaks of the decision are kept:\n%s", got)
	}
	if strings.Contains(got, `"key"`) {
		t.Errorf("a record with no key has a key entry:\n%s", got)
	}
}

// With a budget (C32, S35) the text output may end with an omitted line, set
// off by a blank line from the entries above it.
func TestContextTextOmittedLine(t *testing.T) {
	rec := func(id int) model.Record {
		return model.Record{ID: id, Title: "T", Status: model.StatusDecided, Created: "2026-03-04", Decision: "yes", Rationale: "because"}
	}
	entry := func(id int) string { return fmt.Sprintf("- %d T: yes\n  Rationale: because\n", id) }
	tests := []struct {
		name    string
		recs    []model.Record
		omitted int
		want    string
	}{
		{name: "nothing omitted adds no line", recs: []model.Record{rec(1), rec(2)}, omitted: 0, want: entry(1) + entry(2)},
		{name: "nothing omitted and no records prints nothing", want: ""},
		{name: "several omitted", recs: []model.Record{rec(1), rec(2)}, omitted: 3, want: entry(1) + entry(2) + "\n3 decisions omitted\n"},
		{name: "one omitted is singular", recs: []model.Record{rec(1)}, omitted: 1, want: entry(1) + "\n1 decision omitted\n"},
		{name: "all omitted prints the line alone", omitted: 4, want: "4 decisions omitted\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := ContextTextOmitted(&buf, tt.recs, tt.omitted); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("ContextTextOmitted = %q, want %q", got, tt.want)
			}
		})
	}
}

// The budgeted JSON always has the omitted key, 0 included; ContextJSON, used
// without a budget, never has it.
func TestContextJSONOmittedKey(t *testing.T) {
	rec := model.Record{ID: 3, Title: "T", Status: model.StatusDecided, Created: "2026-03-04", Decision: "yes", Rationale: "because"}
	tests := []struct {
		name    string
		recs    []model.Record
		omitted int
		want    string
	}{
		{name: "none omitted", recs: []model.Record{rec}, omitted: 0, want: "{\n  \"decisions\": [\n    {\n      \"id\": 3,\n      \"title\": \"T\",\n      \"decision\": \"yes\",\n      \"rationale\": \"because\"\n    }\n  ],\n  \"omitted\": 0\n}\n"},
		{name: "some omitted", recs: []model.Record{rec}, omitted: 2, want: "{\n  \"decisions\": [\n    {\n      \"id\": 3,\n      \"title\": \"T\",\n      \"decision\": \"yes\",\n      \"rationale\": \"because\"\n    }\n  ],\n  \"omitted\": 2\n}\n"},
		{name: "all omitted", omitted: 4, want: "{\n  \"decisions\": [],\n  \"omitted\": 4\n}\n"},
		{name: "no records, none omitted", omitted: 0, want: "{\n  \"decisions\": [],\n  \"omitted\": 0\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := ContextJSONOmitted(&buf, tt.recs, tt.omitted); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("ContextJSONOmitted = %q, want %q", got, tt.want)
			}
		})
	}
	var buf bytes.Buffer
	if err := ContextJSON(&buf, []model.Record{rec}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "omitted") {
		t.Errorf("ContextJSON has an omitted key:\n%s", buf.String())
	}
}
