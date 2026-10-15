// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/store"
)

// seedStore makes a fresh repo with four records, through the CLI:
//
//	1 open      key storage.engine, tags infra and perf, two options, a recommendation
//	2 decided   key storage.cache, tag infra, decided by owner
//	3 rejected  key ui.theme, tag perf
//	4 open      no key, no tags
//
// Every record is dated 2026-03-04, the clock every test uses.
func seedStore(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	env := testEnv(root)
	steps := [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--option", "b:Use B",
			"--recommend", "a", "--reasoning", "A is proven", "--key", "storage.engine",
			"--tag", "infra", "--tag", "perf", "--by", "claude-opus", "--context", "why"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--key", "storage.cache", "--tag", "infra"},
		{"ask", "Theme", "--option", "dark:Dark", "--key", "ui.theme", "--tag", "perf"},
		{"decide", "2", "redis", "--why", "fast", "--by", "owner"},
		{"reject", "3", "--why", "not needed"},
		{"ask", "Logging"},
	}
	for _, args := range steps {
		if r := run(env, "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	return root
}

// listedIDs decodes the {"records": [...]} object that list --json prints and
// returns the IDs in order.
func listedIDs(t *testing.T, stdout string) []int {
	t.Helper()
	var out struct {
		Records []struct {
			ID int `json:"id"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("list --json is not the records object: %v\n%s", err, stdout)
	}
	ids := []int{}
	for _, r := range out.Records {
		ids = append(ids, r.ID)
	}
	return ids
}

// decodeObject decodes one JSON object into a map.
func decodeObject(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("not a JSON object: %v\n%s", err, stdout)
	}
	return m
}

// keysOf returns the sorted keys of m.
func keysOf(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestShowPrintsEveryFieldAndHistory(t *testing.T) {
	root := seedStore(t)
	tests := []struct {
		name string
		id   string
		want string
	}{
		{
			name: "open record with options and a recommendation",
			id:   "1",
			want: `ID: 1
Title: Pick a store
Status: open
Key: storage.engine
Tags: infra, perf
Asked by: claude-opus
Created: 2026-03-04
Context: why
Options:
  a: Use A
  b: Use B
Recommendation: a
Reasoning: A is proven
History:
  2026-03-04 ask
`,
		},
		{
			name: "decided record",
			id:   "2",
			want: `ID: 2
Title: Cache layer
Status: decided
Key: storage.cache
Tags: infra
Created: 2026-03-04
Options:
  redis: Use Redis
Decided by: owner
Decided at: 2026-03-04
Decision: redis
Rationale: fast
History:
  2026-03-04 ask
  2026-03-04 decide
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", "show", tt.id)
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			if r.stdout != tt.want {
				t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, tt.want)
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestShowJSONPrintsTheRecordObject(t *testing.T) {
	root := seedStore(t)
	r := run(testEnv(root), "", "show", "2", "--json")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	m := decodeObject(t, r.stdout)
	// Empty fields are left out, as in the record file; body is absent
	// because this record has none.
	want := []string{"created", "decided_at", "decided_by", "decision", "history", "id", "key", "options", "rationale", "status", "tags", "title"}
	if got := keysOf(m); !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
	if m["id"] != 2.0 || m["status"] != "decided" || m["decision"] != "redis" {
		t.Errorf("id, status and decision = %v, %v, %v; want 2, decided, redis", m["id"], m["status"], m["decision"])
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestShowIncludesTheBody(t *testing.T) {
	root := seedStore(t)
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := st.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	rec.Body = "Notes in *Markdown*.\n"
	if err := st.Update(rec); err != nil {
		t.Fatal(err)
	}

	text := run(testEnv(root), "", "show", "1")
	if !strings.HasSuffix(text.stdout, "Body:\nNotes in *Markdown*.\n") {
		t.Errorf("show text does not end with the body:\n%s", text.stdout)
	}
	js := run(testEnv(root), "", "show", "1", "--json")
	if got := decodeObject(t, js.stdout)["body"]; got != "Notes in *Markdown*.\n" {
		t.Errorf("show --json body = %q, want the body", got)
	}
}

func TestShowUnknownIDExitsOne(t *testing.T) {
	root := seedStore(t)
	for _, args := range [][]string{{"show", "99"}, {"show", "99", "--json"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 1 {
			t.Errorf("%v: exit code = %d, want 1", args, r.code)
		}
		if !strings.Contains(r.stderr, "id 99") {
			t.Errorf("%v: stderr does not name id 99:\n%s", args, r.stderr)
		}
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want empty", args, r.stdout)
		}
	}
}

func TestShowDuplicateIDExitsOneNamingBothFiles(t *testing.T) {
	root := seedStore(t)
	dir := filepath.Join(root, "decisions")
	data, err := os.ReadFile(filepath.Join(dir, "0001-pick-a-store.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001-b.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"show", "1"}, {"show", "1", "--json"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 1 {
			t.Errorf("%v: exit code = %d, want 1", args, r.code)
		}
		for _, name := range []string{"0001-pick-a-store.md", "0001-b.md"} {
			if !strings.Contains(r.stderr, name) {
				t.Errorf("%v: stderr does not name %s:\n%s", args, name, r.stderr)
			}
		}
		if r.stdout != "" {
			t.Errorf("%v: stdout = %q, want empty", args, r.stdout)
		}
	}
}

func TestShowBadArgumentsAreUsageErrors(t *testing.T) {
	root := seedStore(t)
	tests := []struct {
		name string
		args []string
	}{
		{name: "no ID", args: []string{"show"}},
		{name: "zero", args: []string{"show", "0"}},
		{name: "not a number", args: []string{"show", "one"}},
		{name: "two IDs", args: []string{"show", "1", "2"}},
		{name: "unknown flag", args: []string{"show", "1", "--nope"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, "usage: adj show") {
				t.Errorf("stderr has no show usage:\n%s", r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
		})
	}
}

func TestListFiltersByStatusTagAndKey(t *testing.T) {
	root := seedStore(t)
	tests := []struct {
		name string
		args []string
		want []int
	}{
		{name: "no filter lists every record in ID order", args: nil, want: []int{1, 2, 3, 4}},
		{name: "status open", args: []string{"--status", "open"}, want: []int{1, 4}},
		{name: "status decided", args: []string{"--status", "decided"}, want: []int{2}},
		{name: "status with no record", args: []string{"--status", "superseded"}, want: []int{}},
		{name: "status is trimmed", args: []string{"--status", " open "}, want: []int{1, 4}},
		{name: "tag infra", args: []string{"--tag", "infra"}, want: []int{1, 2}},
		{name: "tag perf", args: []string{"--tag", "perf"}, want: []int{1, 3}},
		{name: "tag is trimmed", args: []string{"--tag", " infra "}, want: []int{1, 2}},
		{name: "two tags must both match", args: []string{"--tag", "infra", "--tag", "perf"}, want: []int{1}},
		{name: "tag with no record", args: []string{"--tag", "legal"}, want: []int{}},
		{name: "key", args: []string{"--key", "storage.engine"}, want: []int{1}},
		{name: "key is not a prefix", args: []string{"--key", "storage"}, want: []int{}},
		{name: "status and tag", args: []string{"--status", "open", "--tag", "perf"}, want: []int{1}},
		{name: "status and key that do not meet", args: []string{"--status", "open", "--key", "storage.cache"}, want: []int{}},
		{name: "status, tag and key", args: []string{"--status", "open", "--tag", "infra", "--key", "storage.engine"}, want: []int{1}},
		{name: "all three, no record meets them", args: []string{"--status", "rejected", "--tag", "infra", "--key", "ui.theme"}, want: []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"list"}, tt.args...)
			r := run(testEnv(root), "", append(args, "--json")...)
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			if got := listedIDs(t, r.stdout); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IDs = %v, want %v", got, tt.want)
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
		})
	}
}

func TestListTextPrintsOneLinePerRecord(t *testing.T) {
	root := seedStore(t)
	r := run(testEnv(root), "", "list", "--status", "open")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	want := "1  open  Pick a store\n4  open  Logging\n"
	if r.stdout != want {
		t.Errorf("stdout = %q, want %q", r.stdout, want)
	}

	none := run(testEnv(root), "", "list", "--status", "superseded")
	if none.code != 0 || none.stdout != "" {
		t.Errorf("no match: exit %d, stdout %q; want exit 0 and no output", none.code, none.stdout)
	}
}

func TestListJSONWrapsTheRecordsAndEmptyIsAnArray(t *testing.T) {
	root := seedStore(t)
	one := run(testEnv(root), "", "list", "--status", "decided", "--json")
	if one.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", one.code, one.stderr)
	}
	if got := keysOf(decodeObject(t, one.stdout)); !reflect.DeepEqual(got, []string{"records"}) {
		t.Errorf("top-level keys = %v, want [records]", got)
	}

	empty := run(testEnv(root), "", "list", "--status", "superseded", "--json")
	if want := "{\n  \"records\": []\n}\n"; empty.stdout != want {
		t.Errorf("empty list --json = %q, want %q", empty.stdout, want)
	}
}

func TestListRecordsHaveTheShowShape(t *testing.T) {
	root := seedStore(t)
	list := run(testEnv(root), "", "list", "--status", "decided", "--json")
	var out struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal([]byte(list.stdout), &out); err != nil {
		t.Fatalf("list --json: %v\n%s", err, list.stdout)
	}
	show := run(testEnv(root), "", "show", "2", "--json")
	if len(out.Records) != 1 || !reflect.DeepEqual(out.Records[0], decodeObject(t, show.stdout)) {
		t.Errorf("list record = %v\nwant the show --json object %s", out.Records, show.stdout)
	}
}

func TestListInvalidArgumentsAreUsageErrors(t *testing.T) {
	// The repo is not initialised: a usage error must be reported before the
	// store is looked for, so the answer is exit 2, not "run adj init".
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "unknown status", args: []string{"--status", "bogus"}, wantErr: `got "bogus"`},
		{name: "explicit empty status", args: []string{"--status", ""}, wantErr: `got ""`},
		{name: "blank tag", args: []string{"--tag", ""}, wantErr: "--tag must not be blank"},
		{name: "whitespace tag", args: []string{"--tag", "  "}, wantErr: "--tag must not be blank"},
		{name: "blank key", args: []string{"--key", ""}, wantErr: "--key must not be blank"},
		{name: "whitespace key", args: []string{"--key", "  "}, wantErr: "--key must not be blank"},
		{name: "positional argument", args: []string{"extra"}, wantErr: "no arguments"},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(t.TempDir()), "", append([]string{"list"}, tt.args...)...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantErr, r.stderr)
			}
			if !strings.Contains(r.stderr, "usage: adj list") {
				t.Errorf("stderr has no list usage:\n%s", r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
		})
	}
}

func TestShowAndListNeedAnInitialisedRepo(t *testing.T) {
	for _, args := range [][]string{{"show", "1"}, {"list"}} {
		r := run(testEnv(t.TempDir()), "", args...)
		if r.code != 1 {
			t.Errorf("%v: exit code = %d, want 1; stderr:\n%s", args, r.code, r.stderr)
		}
		if !strings.Contains(r.stderr, "adj init") {
			t.Errorf("%v: stderr does not say to run adj init:\n%s", args, r.stderr)
		}
	}
}

// TestListKeyMatchesTheKeyAsStored was added after the implementation, when the
// mutation pass found that C91 (--key matches as typed) had no test.
func TestListKeyMatchesTheKeyAsStored(t *testing.T) {
	root := initRoot(t)
	env := testEnv(root)
	// ask stores the key as typed, spaces included (C91).
	if r := run(env, "", "ask", "Spaced key", "--option", "a:A", "--key", " storage.log "); r.code != 0 {
		t.Fatalf("ask: exit %d: %s", r.code, r.stderr)
	}
	exact := run(env, "", "list", "--key", " storage.log ", "--json")
	if got := listedIDs(t, exact.stdout); exact.code != 0 || !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("list --key as stored: exit %d, IDs %v; want exit 0 and [1]", exact.code, got)
	}
	trimmed := run(env, "", "list", "--key", "storage.log", "--json")
	if got := listedIDs(t, trimmed.stdout); trimmed.code != 0 || !reflect.DeepEqual(got, []int{}) {
		t.Errorf("list --key trimmed: exit %d, IDs %v; want exit 0 and no IDs", trimmed.code, got)
	}
}

func TestJSONOutputIsStableAndEndsWithNewline(t *testing.T) {
	root := seedStore(t)
	for _, args := range [][]string{
		{"show", "1", "--json"},
		{"list", "--json"},
		{"list", "--tag", "infra", "--json"},
	} {
		first := run(testEnv(root), "", args...)
		second := run(testEnv(root), "", args...)
		if first.code != 0 {
			t.Fatalf("%v: exit code = %d, want 0; stderr:\n%s", args, first.code, first.stderr)
		}
		if first.stdout != second.stdout {
			t.Errorf("%v: two runs differ:\n%s\n---\n%s", args, first.stdout, second.stdout)
		}
		if !strings.HasSuffix(first.stdout, "}\n") {
			t.Errorf("%v: output does not end with a newline:\n%q", args, first.stdout)
		}
	}
}
