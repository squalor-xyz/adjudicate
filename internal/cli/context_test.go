// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// seedContext makes a repo with eight records, one or more of each status:
//
//	1 open        "Pick a store"    key storage.engine, tag infra
//	2 decided     "Cache layer"     key storage.cache, tag perf
//	3 rejected    "Theme"           key ui.theme, tag design
//	4 withdrawn   "Queue"           key infra.queue
//	5 superseded  "Old database"     key storage.db, decision postgres, replaced by 6
//	6 decided     "Database"        key storage.db, tag infra, two-line rationale
//	7 decided     "Logging format"  no key, tag ops, two-line decision
//	8 decided     "UI theme"        key ui.theme, tags design and perf
//
// The decided records are 2, 6, 7 and 8; they are the records of
// internal/render/testdata/context.md and context.json.
func seedContext(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--key", "storage.engine", "--tag", "infra"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--key", "storage.cache", "--tag", "perf"},
		{"ask", "Theme", "--option", "dark:Dark", "--key", "ui.theme", "--tag", "design"},
		{"ask", "Queue", "--key", "infra.queue"},
		{"ask", "Old database", "--option", "postgres:Use Postgres", "--key", "storage.db"},
		{"ask", "Database", "--option", "sqlite:Use SQLite", "--key", "storage.db", "--tag", "infra"},
		{"ask", "Logging format", "--tag", "ops"},
		{"ask", "UI theme", "--option", "dark:Dark", "--key", "ui.theme", "--tag", "design", "--tag", "perf"},
		{"decide", "2", "redis", "--why", "fast"},
		{"reject", "3", "--why", "not needed"},
		{"withdraw", "4", "--why", "no longer needed"},
		{"decide", "5", "postgres", "--why", "familiar"},
		{"decide", "6", "sqlite", "--why", "embedded\nno server to run"},
		{"decide", "7", "--text", "Use JSON lines.\nOne object per line.", "--why", "easy to grep"},
		{"decide", "8", "dark", "--why", "looks good"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	// There is no supersede command yet: move record 5 through the model.
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := st.Get(5)
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Supersede(&rec, 6, "2026-03-04"); err != nil {
		t.Fatal(err)
	}
	if err := st.Update(rec); err != nil {
		t.Fatal(err)
	}
	return root
}

// contextIDs runs adj context --format json and returns the IDs listed.
func contextIDs(t *testing.T, root string, args ...string) []int {
	t.Helper()
	r := run(testEnv(root), "", append([]string{"context", "--format", "json"}, args...)...)
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("context %v: exit %d, stderr %q; want exit 0 and no stderr", args, r.code, r.stderr)
	}
	var out struct {
		Decisions []struct {
			ID int `json:"id"`
		} `json:"decisions"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
		t.Fatalf("context --format json is not the decisions object: %v\n%s", err, r.stdout)
	}
	ids := []int{}
	for _, d := range out.Decisions {
		ids = append(ids, d.ID)
	}
	return ids
}

func TestContextListsOnlyDecidedRecords(t *testing.T) {
	root := seedContext(t)
	// Open, rejected, withdrawn and superseded records (1, 3, 4, 5) are left
	// out, in ID order.
	if got, want := contextIDs(t, root), []int{2, 6, 7, 8}; !reflect.DeepEqual(got, want) {
		t.Errorf("context lists %v, want %v", got, want)
	}
	r := run(testEnv(root), "", "context")
	for _, line := range strings.Split(r.stdout, "\n") {
		for _, prefix := range []string{"- 1 ", "- 3 ", "- 4 ", "- 5 "} {
			if strings.HasPrefix(line, prefix) {
				t.Errorf("md output has the list line %q, which is not a decided record:\n%s", line, r.stdout)
			}
		}
	}
}

func TestContextFilters(t *testing.T) {
	root := seedContext(t)
	tests := []struct {
		name string
		args []string
		want []int
	}{
		{name: "tag", args: []string{"--tag", "perf"}, want: []int{2, 8}},
		{name: "a tag also on an open record lists the decided one only", args: []string{"--tag", "infra"}, want: []int{6}},
		{name: "tag nobody has", args: []string{"--tag", "legal"}, want: []int{}},
		{name: "repeated tags must all be on the record", args: []string{"--tag", "perf", "--tag", "design"}, want: []int{8}},
		{name: "tag is trimmed", args: []string{"--tag", " perf "}, want: []int{2, 8}},
		{name: "key prefix", args: []string{"--key-prefix", "storage"}, want: []int{2, 6}},
		{name: "key prefix is a plain prefix, not a segment", args: []string{"--key-prefix", "stor"}, want: []int{2, 6}},
		{name: "key prefix is not a substring match", args: []string{"--key-prefix", "cache"}, want: []int{}},
		{name: "key prefix with a dot", args: []string{"--key-prefix", "storage.c"}, want: []int{2}},
		{name: "key prefix is not trimmed", args: []string{"--key-prefix", " storage"}, want: []int{}},
		{name: "a key prefix of a withdrawn record lists nothing", args: []string{"--key-prefix", "infra"}, want: []int{}},
		{name: "key prefix skips records with no key", args: []string{"--key-prefix", "u"}, want: []int{8}},
		{name: "tag and key prefix combine with AND", args: []string{"--tag", "perf", "--key-prefix", "storage"}, want: []int{2}},
		{name: "tag and key prefix, no record meets both", args: []string{"--tag", "ops", "--key-prefix", "storage"}, want: []int{}},
		{name: "tag and key prefix, other values", args: []string{"--key-prefix", "ui", "--tag", "design"}, want: []int{8}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contextIDs(t, root, tt.args...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("context %v lists %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// The golden files are shared with the render tests, which build the same four
// records by hand.
func TestContextOutputMatchesTheGoldenFiles(t *testing.T) {
	root := seedContext(t)
	tests := []struct {
		name   string
		args   []string
		golden string
	}{
		{name: "md", args: []string{"context", "--format", "md"}, golden: "context.md"},
		{name: "md is the default", args: []string{"context"}, golden: "context.md"},
		{name: "json", args: []string{"context", "--format", "json"}, golden: "context.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("..", "render", "testdata", tt.golden))
			if err != nil {
				t.Fatal(err)
			}
			r := run(testEnv(root), "", tt.args...)
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("exit %d, stderr %q; want exit 0 and no stderr", r.code, r.stderr)
			}
			if r.stdout != string(want) {
				t.Errorf("stdout =\n%s\nwant testdata/%s\n%s", r.stdout, tt.golden, want)
			}
		})
	}
}

func TestContextFormatAfterFilters(t *testing.T) {
	root := seedContext(t)
	for _, args := range [][]string{
		{"context", "--tag", "perf", "--format", "json"},
		{"context", "--format", "json", "--tag", "perf"},
		{"context", "--format=json", "--tag=perf"},
		{"context", "--root", root, "--format", "json", "--tag", "perf"},
	} {
		r := run(testEnv(root), "", args...)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "{\n  \"decisions\": [") || strings.Contains(r.stdout, "Database") {
			t.Errorf("%v: exit %d, stdout %q; want JSON of the perf decisions", args, r.code, r.stdout)
		}
	}
}

func TestContextEmptyResult(t *testing.T) {
	root := seedContext(t)
	r := run(testEnv(root), "", "context", "--tag", "legal")
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("md: exit %d, stdout %q, stderr %q; want exit 0 and no output", r.code, r.stdout, r.stderr)
	}
	r = run(testEnv(root), "", "context", "--tag", "legal", "--format", "json")
	if want := "{\n  \"decisions\": []\n}\n"; r.code != 0 || r.stdout != want || r.stderr != "" {
		t.Errorf("json: exit %d, stdout %q, stderr %q; want exit 0 and %q", r.code, r.stdout, r.stderr, want)
	}
	// A repo with no records at all.
	empty := initRoot(t)
	if r := run(testEnv(empty), "", "context"); r.code != 0 || r.stdout != "" {
		t.Errorf("empty store: exit %d, stdout %q; want exit 0 and no output", r.code, r.stdout)
	}
}

// Two runs on one store give the same bytes, and context never writes.
func TestContextOutputIsStableAndReadOnly(t *testing.T) {
	root := seedContext(t)
	before := snapshot(t, root)
	for _, args := range [][]string{
		{"context"},
		{"context", "--format", "json"},
		{"context", "--tag", "perf", "--key-prefix", "ui"},
	} {
		first := run(testEnv(root), "", args...)
		second := run(testEnv(root), "", args...)
		if first.code != 0 {
			t.Fatalf("%v: exit code = %d, want 0; stderr:\n%s", args, first.code, first.stderr)
		}
		if first.stdout != second.stdout || first.stderr != second.stderr {
			t.Errorf("%v: two runs differ:\n%s\n---\n%s", args, first.stdout, second.stdout)
		}
		if !strings.HasSuffix(first.stdout, "\n") {
			t.Errorf("%v: output does not end with a newline: %q", args, first.stdout)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("context changed the repo")
	}
}

func TestContextBadArgumentsAreUsageErrors(t *testing.T) {
	root := seedContext(t)
	before := snapshot(t, root)
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "unknown format", args: []string{"context", "--format", "yaml"}, wantErr: "--format"},
		{name: "format is case sensitive", args: []string{"context", "--format", "JSON"}, wantErr: "--format"},
		{name: "empty format", args: []string{"context", "--format", ""}, wantErr: "--format"},
		{name: "blank format", args: []string{"context", "--format", " "}, wantErr: "--format"},
		{name: "blank tag", args: []string{"context", "--tag", "  "}, wantErr: "--tag"},
		{name: "empty key prefix", args: []string{"context", "--key-prefix", ""}, wantErr: "--key-prefix"},
		{name: "blank key prefix", args: []string{"context", "--key-prefix", "  "}, wantErr: "--key-prefix"},
		{name: "an argument", args: []string{"context", "storage"}, wantErr: "storage"},
		{name: "unknown flag", args: []string{"context", "--nope"}, wantErr: "nope"},
		{name: "list flag is not a context flag", args: []string{"context", "--status", "open"}, wantErr: "status"},
		{name: "json flag is not a context flag", args: []string{"context", "--json"}, wantErr: "json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 || r.stdout != "" {
				t.Errorf("exit %d, stdout %q; want exit 2 and no stdout", r.code, r.stdout)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj context") {
				t.Errorf("stderr = %q; want it to mention %q and the context usage", r.stderr, tt.wantErr)
			}
		})
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("a usage error changed the repo")
	}
	// Bad arguments are reported even where there is no repo to read.
	if r := run(testEnv(t.TempDir()), "", "context", "--format", "yaml"); r.code != 2 {
		t.Errorf("no repo and a bad format: exit %d, want 2 before the store is looked for", r.code)
	}
}

func TestContextFailuresExitOne(t *testing.T) {
	// No config: exit 1 and a pointer to adj init.
	r := run(testEnv(t.TempDir()), "", "context")
	if r.code != 1 || !strings.Contains(r.stderr, "adj init") || r.stdout != "" {
		t.Errorf("no repo: exit %d, stdout %q, stderr %q; want exit 1 naming adj init", r.code, r.stdout, r.stderr)
	}
	// A record file that cannot be read fails the command; no partial output.
	root := seedContext(t)
	bad := filepath.Join(root, "decisions", "0009-broken.md")
	if err := os.WriteFile(bad, []byte("not a record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"context"}, {"context", "--format", "json"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 1 || !strings.Contains(r.stderr, "0009-broken.md") || !failureStdout(r.stdout, args) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1 naming 0009-broken.md and no output", args, r.code, r.stdout, r.stderr)
		}
	}
}

func TestContextHelpAndMainUsage(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		r := run(testEnv(t.TempDir()), "", "context", flag)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj context [flags]\n") || r.stderr != "" {
			t.Errorf("context %s: exit %d, stdout %q, stderr %q; want exit 0 and usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
	r := run(testEnv(t.TempDir()), "", "context", "--help")
	for _, want := range []string{"--tag", "--key-prefix", "--format", "md", "json", "decided", "docs/json.md"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("context usage does not mention %q:\n%s", want, r.stdout)
		}
	}
	if m := run(testEnv(t.TempDir()), "", "--help"); !strings.Contains(m.stdout, "adj context") {
		t.Errorf("main usage does not list adj context:\n%s", m.stdout)
	}
}

// The documented example is the real output for the records it describes: the
// docs are the contract (docs/json.md), so a drift fails here.
func TestContextJSONMatchesTheDocumentedExample(t *testing.T) {
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--key", "storage.engine"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--key", "storage.cache", "--tag", "infra"},
		{"ask", "Logging format", "--tag", "ops"},
		{"decide", "2", "redis", "--why", "fast\nand cheap to run", "--by", "owner"},
		{"decide", "3", "--text", "Use JSON lines.", "--why", "easy to grep"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	r := run(testEnv(root), "", "context", "--format", "json")
	if want := docsJSONExample(t, "## `adj context --format json`"); r.stdout != want {
		t.Errorf("context --format json =\n%s\nwant the docs/json.md example\n%s", r.stdout, want)
	}
}
