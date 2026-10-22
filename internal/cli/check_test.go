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

// seedCheck makes a repo with seven records, all asked on 2026-03-04:
//
//	1 open        "Pick a store"   key storage.engine, tag infra
//	2 decided     "Cache layer"    key storage.cache, tag perf, decision redis
//	3 rejected    "Theme"          key ui.theme, tag design
//	4 withdrawn   "Queue"          key infra.queue
//	5 superseded  "Old database"   key storage.db, decision postgres, replaced by 6
//	6 decided     "Database"       key storage.db, decision sqlite
//	7 open        "Logging format" tag ops
func seedCheck(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--key", "storage.engine", "--tag", "infra"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--key", "storage.cache", "--tag", "perf"},
		{"ask", "Theme", "--option", "dark:Dark", "--key", "ui.theme", "--tag", "design"},
		{"ask", "Queue", "--key", "infra.queue"},
		{"ask", "Old database", "--option", "postgres:Use Postgres", "--key", "storage.db"},
		{"ask", "Database", "--option", "sqlite:Use SQLite", "--key", "storage.db"},
		{"ask", "Logging format", "--tag", "ops"},
		{"decide", "2", "redis", "--why", "fast"},
		{"reject", "3", "--why", "not needed"},
		{"withdraw", "4", "--why", "no longer needed"},
		{"decide", "5", "postgres", "--why", "familiar"},
		{"decide", "6", "sqlite", "--why", "embedded"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	// Supersede through the model so the check tests pin the one-sided link
	// shape (record 6 has no supersedes entry).
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

// checkOut is the check --json object.
type checkOut struct {
	Query   string `json:"query"`
	Result  string `json:"result"`
	Decided []struct {
		ID int `json:"id"`
	} `json:"decided"`
	Open []struct {
		ID int `json:"id"`
	} `json:"open"`
	Closed []struct {
		ID int `json:"id"`
	} `json:"closed"`
}

func decodeCheck(t *testing.T, stdout string) (result string, decided, open, closed []int) {
	t.Helper()
	var out checkOut
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("check --json is not the check object: %v\n%s", err, stdout)
	}
	decided, open, closed = []int{}, []int{}, []int{}
	for _, r := range out.Decided {
		decided = append(decided, r.ID)
	}
	for _, r := range out.Open {
		open = append(open, r.ID)
	}
	for _, r := range out.Closed {
		closed = append(closed, r.ID)
	}
	return out.Result, decided, open, closed
}

func TestCheckFindsRecordsByTitleKeyTagAndDecisionText(t *testing.T) {
	root := seedCheck(t)
	tests := []struct {
		name        string
		query       string
		wantResult  string
		wantDecided []int
		wantOpen    []int
		wantClosed  []int
	}{
		{name: "title of a decided record", query: "Cache layer", wantResult: "decided", wantDecided: []int{2}},
		{name: "key of a decided record", query: "storage.cache", wantResult: "decided", wantDecided: []int{2}},
		{name: "tag of a decided record", query: "perf", wantResult: "decided", wantDecided: []int{2}},
		{name: "decision text of a decided record", query: "redis", wantResult: "decided", wantDecided: []int{2}},
		{name: "title of an open record", query: "Pick a store", wantResult: "open", wantOpen: []int{1}},
		{name: "key of an open record", query: "storage.engine", wantResult: "open", wantOpen: []int{1}},
		{name: "tag of an open record", query: "ops", wantResult: "open", wantOpen: []int{7}},
		{name: "case and surrounding space do not matter", query: "  CACHE layer ", wantResult: "decided", wantDecided: []int{2}},
		{name: "decided and open both match: decided wins, both listed", query: "storage", wantResult: "decided", wantDecided: []int{2, 6}, wantOpen: []int{1}, wantClosed: []int{5}},
		{name: "nothing matches", query: "kubernetes", wantResult: "not found"},
		{name: "a rejected record is closed", query: "theme", wantResult: "closed", wantClosed: []int{3}},
		{name: "a withdrawn record is closed", query: "queue", wantResult: "closed", wantClosed: []int{4}},
		{name: "a superseded record is closed and its replacement decided", query: "postgres", wantResult: "decided", wantDecided: []int{6}, wantClosed: []int{5}},
		{name: "the record that replaced it is found", query: "sqlite", wantResult: "decided", wantDecided: []int{6}},
		{name: "decided and closed both match", query: "database", wantResult: "decided", wantDecided: []int{6}, wantClosed: []int{5}},
		{name: "open and closed both match: open wins", query: "infra", wantResult: "open", wantOpen: []int{1}, wantClosed: []int{4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", "check", "--json", tt.query)
			if r.code != 0 || r.stderr != "" {
				t.Fatalf("exit %d, stderr %q; want exit 0 and no stderr", r.code, r.stderr)
			}
			result, decided, open, closed := decodeCheck(t, r.stdout)
			if result != tt.wantResult {
				t.Errorf("result = %q, want %q", result, tt.wantResult)
			}
			if tt.wantDecided == nil {
				tt.wantDecided = []int{}
			}
			if tt.wantOpen == nil {
				tt.wantOpen = []int{}
			}
			if tt.wantClosed == nil {
				tt.wantClosed = []int{}
			}
			if !reflect.DeepEqual(decided, tt.wantDecided) || !reflect.DeepEqual(open, tt.wantOpen) || !reflect.DeepEqual(closed, tt.wantClosed) {
				t.Errorf("decided = %v, open = %v, closed = %v; want %v, %v and %v", decided, open, closed, tt.wantDecided, tt.wantOpen, tt.wantClosed)
			}
		})
	}
}

// Exit 0 for every completed search (decided, open or not found): a non-zero
// exit means the check itself failed, so a broken store is never read as "not
// found" and then asked about.
func TestCheckExitCodesSeparateAnAnswerFromAFailure(t *testing.T) {
	root := seedCheck(t)
	for _, query := range []string{"redis", "Logging", "theme", "kubernetes"} {
		if r := run(testEnv(root), "", "check", query); r.code != 0 || r.stderr != "" {
			t.Errorf("check %q: exit %d, stderr %q; want exit 0", query, r.code, r.stderr)
		}
	}
	// A record file that cannot be read fails the check: exit 1, no answer.
	bad := filepath.Join(root, "decisions", "0008-broken.md")
	if err := os.WriteFile(bad, []byte("not a record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"check", "kubernetes"}, {"check", "--json", "kubernetes"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 1 || !strings.Contains(r.stderr, "0008-broken.md") || !failureStdout(r.stdout, args) {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1 naming 0008-broken.md and no answer", args, r.code, r.stdout, r.stderr)
		}
	}
}

func TestCheckTextStartsWithTheResult(t *testing.T) {
	root := seedCheck(t)
	tests := []struct {
		query string
		want  string
	}{
		{"kubernetes", "not found\n"},
		{"ops", "open\n7  open  Logging format\n"},
		{"theme", "closed\n3  rejected  Theme\n  Rationale: not needed\n"},
		{"queue", "closed\n4  withdrawn  Queue\n  Rationale: no longer needed\n"},
		{"postgres", "decided\n6  decided  Database\n  Decision: sqlite\n  Rationale: embedded\n5  superseded  Old database\n  Superseded by: 6\n  Rationale: familiar\n"},
		{"redis", "decided\n2  decided  Cache layer\n  Decision: redis\n  Rationale: fast\n"},
		{"storage", "decided\n" +
			"2  decided  Cache layer\n  Decision: redis\n  Rationale: fast\n" +
			"6  decided  Database\n  Decision: sqlite\n  Rationale: embedded\n" +
			"1  open  Pick a store\n" +
			"5  superseded  Old database\n  Superseded by: 6\n  Rationale: familiar\n"},
	}
	for _, tt := range tests {
		r := run(testEnv(root), "", "check", tt.query)
		if r.code != 0 || r.stdout != tt.want || r.stderr != "" {
			t.Errorf("check %q: exit %d, stdout %q, stderr %q; want exit 0 and %q", tt.query, r.code, r.stdout, r.stderr, tt.want)
		}
	}
}

func TestCheckJSONCarriesTheQueryAndFullRecords(t *testing.T) {
	root := seedCheck(t)
	r := run(testEnv(root), "", "check", "--json", "  storage ")
	var out struct {
		Query   string           `json:"query"`
		Result  string           `json:"result"`
		Decided []map[string]any `json:"decided"`
		Open    []map[string]any `json:"open"`
		Closed  []map[string]any `json:"closed"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, r.stdout)
	}
	if out.Query != "storage" {
		t.Errorf("query = %q, want the trimmed query %q", out.Query, "storage")
	}
	if len(out.Decided) != 2 || out.Decided[0]["decision"] != "redis" || out.Decided[0]["rationale"] != "fast" {
		t.Errorf("decided = %v, want records 2 and 6 with decision and rationale", out.Decided)
	}
	if len(out.Closed) != 1 || out.Closed[0]["status"] != "superseded" || out.Closed[0]["superseded_by"] != float64(6) {
		t.Errorf("closed = %v, want record 5, superseded by 6", out.Closed)
	}
	if len(out.Open) != 1 || out.Open[0]["status"] != "open" || out.Open[0]["title"] != "Pick a store" {
		t.Errorf("open = %v, want record 1", out.Open)
	}
}

func TestCheckOutputIsStableAndReadOnly(t *testing.T) {
	root := seedCheck(t)
	before := snapshot(t, root)
	for _, args := range [][]string{{"check", "--json", "storage"}, {"check", "storage"}, {"check", "--json", "kubernetes"}} {
		first := run(testEnv(root), "", args...)
		second := run(testEnv(root), "", args...)
		if first.code != 0 {
			t.Fatalf("%v: exit code = %d, want 0; stderr:\n%s", args, first.code, first.stderr)
		}
		if first.stdout != second.stdout {
			t.Errorf("%v: two runs differ:\n%s\n---\n%s", args, first.stdout, second.stdout)
		}
		if !strings.HasSuffix(first.stdout, "\n") {
			t.Errorf("%v: output does not end with a newline: %q", args, first.stdout)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("check changed the repo")
	}
}

// The documented example is the real output for the records it describes: the
// docs are the contract (docs/json.md), so a drift fails here.
func TestCheckJSONMatchesTheDocumentedExample(t *testing.T) {
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--key", "storage.engine", "--by", "claude-opus"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--key", "storage.cache", "--tag", "infra"},
		{"ask", "Storage engine v0", "--key", "storage.v0"},
		{"decide", "2", "redis", "--why", "fast", "--by", "owner"},
		{"reject", "3", "--why", "too slow", "--by", "owner"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	r := run(testEnv(root), "", "check", "--json", "storage")
	if want := docsJSONExample(t, "## `adj check QUERY --json`"); r.stdout != want {
		t.Errorf("check --json =\n%s\nwant the docs/json.md example\n%s", r.stdout, want)
	}
}

func TestCheckQueryAfterFlagsAndAfterDoubleDash(t *testing.T) {
	root := seedCheck(t)
	for _, args := range [][]string{
		{"check", "redis", "--json"},
		{"check", "--json", "redis"},
		{"check", "--root", root, "--json", "--", "redis"},
	} {
		r := run(testEnv(root), "", args...)
		if result, decided, _, _ := decodeCheck(t, r.stdout); r.code != 0 || result != "decided" || !reflect.DeepEqual(decided, []int{2}) {
			t.Errorf("%v: exit %d, result %q, decided %v; want exit 0, decided [2]", args, r.code, result, decided)
		}
	}
	// A query that starts with a dash is given after --.
	r := run(testEnv(root), "", "check", "--", "-nothing")
	if r.code != 0 || r.stdout != "not found\n" {
		t.Errorf("check -- -nothing: exit %d, stdout %q; want exit 0 and not found", r.code, r.stdout)
	}
}

func TestCheckHelpPrintsUsageToStdout(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		r := run(testEnv(t.TempDir()), "", "check", flag)
		if r.code != 0 || !strings.Contains(r.stdout, "usage: adj check [flags] QUERY\n") || r.stderr != "" {
			t.Errorf("check %s: exit %d, stdout %q, stderr %q; want exit 0 and usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
}

func TestCheckUsageNamesTheExitCodes(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "check", "--help")
	for _, want := range []string{"exit 0", "exit 1", "exit 2", "not found", "\n  closed ", "one\nsubstring", "superseded_by"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("check usage does not mention %q:\n%s", want, r.stdout)
		}
	}
}

// check matches a record's refs as well as its title, key, tags and decision
// text (C171), so its usage must name them among the fields it searches.
func TestCheckUsageNamesRefsAmongTheSearchedFields(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "check", "--help")
	flat := strings.Join(strings.Fields(r.stdout), " ")
	if want := "title, key, tags, refs and decision text"; !strings.Contains(flat, want) {
		t.Errorf("check usage does not say it searches %q:\n%s", want, r.stdout)
	}
}

func TestCheckListedInTheMainUsage(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(r.stdout, "adj check") {
		t.Errorf("main usage does not list adj check:\n%s", r.stdout)
	}
}

func TestCheckBadArgumentsAreUsageErrors(t *testing.T) {
	root := seedCheck(t)
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no query", args: []string{"check"}, wantErr: "QUERY"},
		{name: "two arguments", args: []string{"check", "storage", "engine"}, wantErr: "quote"},
		{name: "blank query", args: []string{"check", "  "}, wantErr: "blank"},
		{name: "empty query", args: []string{"check", ""}, wantErr: "blank"},
		{name: "unknown flag", args: []string{"check", "--nope", "x"}, wantErr: "nope"},
		{name: "list filter is not a check flag", args: []string{"check", "--status", "open", "x"}, wantErr: "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj check [flags] QUERY\n") {
				t.Errorf("stderr should contain %q and the check usage:\n%s", tt.wantErr, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
		})
	}
}

// Bad arguments are reported before the store is looked for (C73).
func TestCheckBlankQueryIsAUsageErrorEvenWithoutARepo(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "check", " ")
	if r.code != 2 || !strings.Contains(r.stderr, "blank") || strings.Contains(r.stderr, "run `adj init`") {
		t.Errorf("exit %d, stderr %q; want exit 2 for a blank query, not the missing-repo error", r.code, r.stderr)
	}
}

func TestCheckNeedsAnInitialisedRepo(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "check", "x")
	if r.code != 1 || !strings.Contains(r.stderr, "adj init") || r.stdout != "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 telling the user to run adj init", r.code, r.stdout, r.stderr)
	}
}

func TestCheckOnAnEmptyStoreIsNotFound(t *testing.T) {
	r := run(testEnv(initRoot(t)), "", "check", "anything")
	if r.code != 0 || r.stdout != "not found\n" || r.stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 0 and not found", r.code, r.stdout, r.stderr)
	}
}

func TestCheckFailsOnAnInvalidConfig(t *testing.T) {
	root := initRoot(t)
	if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), []byte(`{"store": "../out"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r := run(testEnv(root), "", "check", "x")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, "adj check:") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 with the config error and no output", r.code, r.stdout, r.stderr)
	}
}
