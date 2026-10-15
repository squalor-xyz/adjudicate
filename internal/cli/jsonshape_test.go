// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// docs/json.md is the contract for every read command's JSON (S12). Its
// examples are read by one convention: the first ```json block under a
// "## " heading (docsJSONExample), and its key tables are the lines that
// start with "| `key` |" (docsTableKeys).

// docsText returns docs/json.md.
func docsText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "json.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// sectionUnder returns the text of the section that starts at heading and
// ends at the next "## " heading.
func sectionUnder(doc, heading string) (string, error) {
	_, rest, ok := strings.Cut(doc, "\n"+heading+"\n")
	if !ok {
		return "", fmt.Errorf("docs/json.md has no %q section", heading)
	}
	if next := strings.Index(rest, "\n## "); next >= 0 {
		rest = rest[:next]
	}
	return rest, nil
}

// docsJSONExample returns the first ```json block under the heading in
// docs/json.md.
func docsJSONExample(t *testing.T, heading string) string {
	t.Helper()
	block, err := jsonBlockUnder(docsText(t), heading)
	if err != nil {
		t.Fatal(err)
	}
	return block
}

// jsonBlockUnder returns the first ```json block of the section that starts at
// heading and ends at the next "## " heading; a section with no block is an
// error, not the next section's block.
func jsonBlockUnder(doc, heading string) (string, error) {
	rest, err := sectionUnder(doc, heading)
	if err != nil {
		return "", err
	}
	_, rest, ok := strings.Cut(rest, "```json\n")
	if !ok {
		return "", fmt.Errorf("section %q has no json block", heading)
	}
	block, _, ok := strings.Cut(rest, "```")
	if !ok {
		return "", fmt.Errorf("section %q: json block is not closed", heading)
	}
	return block, nil
}

// tableKeysUnder returns the keys of the first column of the section's table,
// sorted: the text between the backticks of every line that starts "| `".
func tableKeysUnder(doc, heading string) ([]string, error) {
	rest, err := sectionUnder(doc, heading)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, line := range strings.Split(rest, "\n") {
		after, ok := strings.CutPrefix(line, "| `")
		if !ok {
			continue
		}
		key, _, ok := strings.Cut(after, "`")
		if !ok {
			return nil, fmt.Errorf("section %q: table line %q has no closing backtick", heading, line)
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("section %q has no key table", heading)
	}
	sort.Strings(keys)
	return keys, nil
}

// docsTableKeys is tableKeysUnder on docs/json.md.
func docsTableKeys(t *testing.T, heading string) []string {
	t.Helper()
	keys, err := tableKeysUnder(docsText(t), heading)
	if err != nil {
		t.Fatal(err)
	}
	return keys
}

func TestJSONBlockUnderStopsAtTheNextHeading(t *testing.T) {
	doc := "# T\n\n## A\n\ntext only\n\n## B\n\n```json\n{\"b\": 1}\n```\n"
	if _, err := jsonBlockUnder(doc, "## A"); err == nil || !strings.Contains(err.Error(), "no json block") {
		t.Errorf("section A: err = %v, want a no json block error", err)
	}
	if got, err := jsonBlockUnder(doc, "## B"); err != nil || got != "{\"b\": 1}\n" {
		t.Errorf("section B: block %q, err %v; want the block", got, err)
	}
	if _, err := jsonBlockUnder(doc, "## C"); err == nil || !strings.Contains(err.Error(), "no \"## C\" section") {
		t.Errorf("section C: err = %v, want a no section error", err)
	}
}

func TestTableKeysUnderReadsOneSectionsTable(t *testing.T) {
	doc := "# T\n\n## A\n\n| Key | Type |\n|---|---|\n| `b` | x |\n| `a` | y |\n\n## B\n\n| `c` | z |\n\n## D\n\nno table\n"
	if got, err := tableKeysUnder(doc, "## A"); err != nil || !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("section A: keys %v, err %v; want [a b]", got, err)
	}
	if got, err := tableKeysUnder(doc, "## B"); err != nil || !reflect.DeepEqual(got, []string{"c"}) {
		t.Errorf("section B: keys %v, err %v; want [c]", got, err)
	}
	if _, err := tableKeysUnder(doc, "## D"); err == nil || !strings.Contains(err.Error(), "no key table") {
		t.Errorf("section D: err = %v, want a no key table error", err)
	}
}

// docsFixture is a repo whose show and list output is the example in
// docs/json.md, byte for byte (record keys are omitted when empty, C87, so
// only a fixture that matches the example pins the keys):
//
//	1 open     Pick a store, key storage.engine
//	2 decided  Cache layer, key storage.cache, tag infra, redis, by owner
func docsFixture(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Pick a store", "--key", "storage.engine"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--key", "storage.cache", "--tag", "infra"},
		{"decide", "2", "redis", "--why", "fast", "--by", "owner"},
	}
	for _, args := range steps {
		if r := run(envOn(t, root, "2026-10-15"), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	return root
}

// The documented example is the real output for the records it describes: the
// docs are the contract (docs/json.md), so a drift fails here. Inbox, check
// and context have the same test beside their commands (C104, C112).
func TestShowAndListJSONMatchTheDocumentedExamples(t *testing.T) {
	root := docsFixture(t)
	tests := []struct {
		heading string
		args    []string
	}{
		{"## `adj show ID --json`", []string{"show", "2", "--json"}},
		{"## `adj list --json`", []string{"list", "--json"}},
	}
	for _, tt := range tests {
		t.Run(tt.args[0], func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			if want := docsJSONExample(t, tt.heading); r.stdout != want {
				t.Errorf("%v =\n%s\nwant the docs/json.md example\n%s", tt.args, r.stdout, want)
			}
		})
	}
}

// For each read command, the top-level keys of its --json output are the keys
// of the example in docs/json.md. The fixture is the one that the show and
// list examples match, so the check does not depend on which keys a record
// leaves out.
func TestJSONTopLevelKeysMatchTheDocs(t *testing.T) {
	root := docsFixture(t)
	tests := []struct {
		name    string
		heading string
		args    []string
		want    []string // the keys the docs example must have
	}{
		{"show", "## `adj show ID --json`", []string{"show", "2", "--json"},
			[]string{"created", "decided_at", "decided_by", "decision", "history", "id", "key", "options", "rationale", "status", "tags", "title"}},
		{"list", "## `adj list --json`", []string{"list", "--json"}, []string{"records"}},
		{"inbox", "## `adj inbox --json`", []string{"inbox", "--json"}, []string{"records"}},
		{"check", "## `adj check QUERY --json`", []string{"check", "--json", "storage"},
			[]string{"closed", "decided", "open", "query", "result"}},
		{"context", "## `adj context --format json`", []string{"context", "--format", "json"}, []string{"decisions"}},
		{"lint", "## `adj lint --json`", []string{"lint", "--json"}, []string{"errors", "findings", "warnings"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			got := keysOf(decodeObject(t, r.stdout))
			docs := keysOf(decodeObject(t, docsJSONExample(t, tt.heading)))
			if !reflect.DeepEqual(got, docs) {
				t.Errorf("top-level keys of %v = %v, docs/json.md example has %v", tt.args, got, docs)
			}
			if !reflect.DeepEqual(docs, tt.want) {
				t.Errorf("docs/json.md example keys = %v, want %v", docs, tt.want)
			}
		})
	}

	// check also lists its keys in a table: it must say the same.
	if got, want := docsTableKeys(t, "## `adj check QUERY --json`"), []string{"closed", "decided", "open", "query", "result"}; !reflect.DeepEqual(got, want) {
		t.Errorf("check key table = %v, want %v", got, want)
	}
}

// recordObjects decodes the record objects in one command's JSON output:
// the object itself (show), or the arrays under the given top-level keys.
func recordObjects(t *testing.T, stdout string, arrays ...string) []map[string]any {
	t.Helper()
	top := decodeObject(t, stdout)
	if len(arrays) == 0 {
		return []map[string]any{top}
	}
	var out []map[string]any
	for _, name := range arrays {
		items, ok := top[name].([]any)
		if !ok {
			t.Fatalf("%q is not an array in:\n%s", name, stdout)
		}
		for _, it := range items {
			obj, ok := it.(map[string]any)
			if !ok {
				t.Fatalf("an item of %q is not an object in:\n%s", name, stdout)
			}
			out = append(out, obj)
		}
	}
	return out
}

// Every key of every record object, from every command that prints one, is in
// the "Keys of a record" table, and the four always-present keys are there.
func TestRecordObjectsUseOnlyDocumentedKeys(t *testing.T) {
	root := seedStore(t)
	table := docsTableKeys(t, "## Keys of a record")
	known := map[string]bool{}
	for _, k := range table {
		known[k] = true
	}
	cases := []struct {
		name   string
		args   []string
		arrays []string
	}{
		{"show 1", []string{"show", "1", "--json"}, nil},
		{"show 2", []string{"show", "2", "--json"}, nil},
		{"show 3", []string{"show", "3", "--json"}, nil},
		{"show 4", []string{"show", "4", "--json"}, nil},
		{"list", []string{"list", "--json"}, []string{"records"}},
		{"inbox", []string{"inbox", "--json"}, []string{"records"}},
		{"check", []string{"check", "--json", "storage"}, []string{"decided", "open", "closed"}},
		{"check rejected", []string{"check", "--json", "ui.theme"}, []string{"decided", "open", "closed"}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			recs := recordObjects(t, r.stdout, tt.arrays...)
			if len(recs) == 0 {
				t.Fatalf("no record objects in:\n%s", r.stdout)
			}
			for _, rec := range recs {
				for k := range rec {
					if !known[k] {
						t.Errorf("record %v has key %q, which docs/json.md does not list (%v)", rec["id"], k, table)
					}
				}
				for _, k := range []string{"id", "title", "status", "created"} {
					if _, ok := rec[k]; !ok {
						t.Errorf("record %v has no %q", rec["id"], k)
					}
				}
			}
		})
	}
}

// A context entry has only the keys in the table of its section.
func TestContextEntriesUseOnlyDocumentedKeys(t *testing.T) {
	root := seedStore(t)
	table := docsTableKeys(t, "## `adj context --format json`")
	known := map[string]bool{}
	for _, k := range table {
		known[k] = true
	}
	r := run(testEnv(root), "", "context", "--format", "json")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	entries := recordObjects(t, r.stdout, "decisions")
	if len(entries) == 0 {
		t.Fatalf("no decisions in:\n%s", r.stdout)
	}
	for _, e := range entries {
		for k := range e {
			if !known[k] {
				t.Errorf("entry %v has key %q, which the context table does not list (%v)", e["id"], k, table)
			}
		}
		for _, k := range []string{"id", "title", "decision"} {
			if _, ok := e[k]; !ok {
				t.Errorf("entry %v has no %q", e["id"], k)
			}
		}
	}
}

// The "Keys of a record" table lists exactly the model's frontmatter keys and
// body, so a field added to the record without a docs row (or a row with no
// field) fails here.
func TestRecordKeyTableMatchesTheModel(t *testing.T) {
	var want []string
	typ := reflect.TypeOf(model.Record{})
	for i := 0; i < typ.NumField(); i++ {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			want = append(want, name)
		}
	}
	want = append(want, "body") // the file's Markdown after the frontmatter (C88)
	sort.Strings(want)
	if got := docsTableKeys(t, "## Keys of a record"); !reflect.DeepEqual(got, want) {
		t.Errorf("docs/json.md record key table = %v, want the model's keys %v", got, want)
	}
}

// code, C117 and AGENTS.md all say the first non-blank line.
func TestContextTableSaysFirstNonBlankLine(t *testing.T) {
	section, err := sectionUnder(docsText(t), "## `adj context --format json`")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section, "the first non-blank line of the record's rationale") {
		t.Errorf("the context rationale row does not say the first non-blank line:\n%s", section)
	}
}
