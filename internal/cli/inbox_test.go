// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// envOn is testEnv with the clock on the given YYYY-MM-DD date.
func envOn(t *testing.T, root, date string) Env {
	t.Helper()
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		t.Fatal(err)
	}
	env := testEnv(root)
	env.Now = func() time.Time { return day }
	return env
}

// seedInbox makes a repo with seven records, each asked on its own date:
//
//	1 open       2026-03-05  two options (one with tradeoffs) and a recommendation
//	2 open       2026-03-03  one option
//	3 open       2026-03-03  no options
//	4 decided    2026-03-01
//	5 rejected   2026-03-02
//	6 withdrawn  2026-03-02
//	7 superseded 2026-03-02
//
// The inbox is 2, 3, 1: oldest first, then by ID.
func seedInbox(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := []struct {
		date string
		args []string
	}{
		{"2026-03-05", []string{"ask", "Pick a store", "--option", "a:Use A", "--option", "b:Use B",
			"--tradeoffs", "a:Younger project", "--recommend", "a", "--reasoning", "A is proven",
			"--key", "storage.engine", "--tag", "infra", "--by", "claude-opus", "--context", "why"}},
		{"2026-03-03", []string{"ask", "Cache layer", "--option", "redis:Use Redis"}},
		{"2026-03-03", []string{"ask", "Logging"}},
		{"2026-03-01", []string{"ask", "Theme", "--option", "dark:Dark"}},
		{"2026-03-02", []string{"ask", "Old question"}},
		{"2026-03-02", []string{"ask", "Dropped question"}},
		{"2026-03-02", []string{"ask", "Replaced choice", "--option", "x:X"}},
		{"2026-03-06", []string{"decide", "4", "dark", "--why", "looks good"}},
		{"2026-03-06", []string{"reject", "5", "--why", "not needed"}},
		{"2026-03-06", []string{"withdraw", "6", "--why", "no longer needed"}},
		{"2026-03-06", []string{"decide", "7", "x", "--why", "fine"}},
	}
	for _, s := range steps {
		if r := run(envOn(t, root, s.date), "", s.args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", s.args, r.code, r.stderr)
		}
	}
	// There is no supersede command yet: move record 7 through the model.
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := st.Get(7)
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Supersede(&rec, 1, "2026-03-07"); err != nil {
		t.Fatal(err)
	}
	if err := st.Update(rec); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestInboxListsOnlyOpenRecordsOldestFirstThenByID(t *testing.T) {
	root := seedInbox(t)
	r := run(testEnv(root), "", "inbox", "--json")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	if got, want := listedIDs(t, r.stdout), []int{2, 3, 1}; !reflect.DeepEqual(got, want) {
		t.Errorf("inbox --json IDs = %v, want %v", got, want)
	}

	text := run(testEnv(root), "", "inbox")
	var headers []string
	for _, line := range strings.Split(text.stdout, "\n") {
		if line != "" && !strings.HasPrefix(line, " ") {
			headers = append(headers, line)
		}
	}
	if want := []string{"2  Cache layer", "3  Logging", "1  Pick a store"}; !reflect.DeepEqual(headers, want) {
		t.Errorf("inbox entries = %q, want %q", headers, want)
	}
	if r.stderr != "" || text.stderr != "" {
		t.Errorf("stderr = %q and %q, want empty", r.stderr, text.stderr)
	}
}

func TestInboxTextShowsIDTitleContextOptionsTradeoffsAndRecommendation(t *testing.T) {
	root := seedInbox(t)
	r := run(testEnv(root), "", "inbox")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	want := `2  Cache layer
  Created: 2026-03-03
  Options:
    redis: Use Redis

3  Logging
  Created: 2026-03-03

1  Pick a store
  Created: 2026-03-05
  Context: why
  Options:
    a: Use A
      tradeoffs: Younger project
    b: Use B
  Recommendation: a
  Reasoning: A is proven
`
	if r.stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, want)
	}
}

func TestInboxJSONRecordsCarryOptionsTradeoffsAndRecommendation(t *testing.T) {
	root := seedInbox(t)
	r := run(testEnv(root), "", "inbox", "--json")
	var out struct {
		Records []struct {
			ID             int                 `json:"id"`
			Options        []map[string]string `json:"options"`
			Recommendation map[string]string   `json:"recommendation"`
		} `json:"records"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
		t.Fatalf("inbox --json: %v\n%s", err, r.stdout)
	}
	if len(out.Records) != 3 || out.Records[2].ID != 1 {
		t.Fatalf("records = %+v, want 3 with ID 1 last", out.Records)
	}
	one := out.Records[2]
	wantOpts := []map[string]string{{"label": "a", "summary": "Use A", "tradeoffs": "Younger project"}, {"label": "b", "summary": "Use B"}}
	if !reflect.DeepEqual(one.Options, wantOpts) {
		t.Errorf("options = %v, want %v", one.Options, wantOpts)
	}
	if want := map[string]string{"label": "a", "reasoning": "A is proven"}; !reflect.DeepEqual(one.Recommendation, want) {
		t.Errorf("recommendation = %v, want %v", one.Recommendation, want)
	}
}

func TestInboxEmptyExitsZeroWithEmptyResult(t *testing.T) {
	// A store with no records, and a store whose records are all answered.
	answered := initRoot(t)
	for _, args := range [][]string{{"ask", "Q", "--option", "a:A"}, {"decide", "1", "a", "--why", "ok"}} {
		if r := run(testEnv(answered), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	for name, root := range map[string]string{"no records": initRoot(t), "all answered": answered} {
		t.Run(name, func(t *testing.T) {
			text := run(testEnv(root), "", "inbox")
			if text.code != 0 || text.stdout != "" || text.stderr != "" {
				t.Errorf("text: exit %d, stdout %q, stderr %q; want exit 0 and no output", text.code, text.stdout, text.stderr)
			}
			js := run(testEnv(root), "", "inbox", "--json")
			if want := "{\n  \"records\": []\n}\n"; js.code != 0 || js.stdout != want || js.stderr != "" {
				t.Errorf("json: exit %d, stdout %q, stderr %q; want exit 0 and %q", js.code, js.stdout, js.stderr, want)
			}
		})
	}
}

func TestInboxOutputIsStableAndReadOnly(t *testing.T) {
	root := seedInbox(t)
	before := snapshot(t, root)
	for _, args := range [][]string{{"inbox", "--json"}, {"inbox"}} {
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
		t.Error("inbox changed the repo")
	}
}

// The documented example is the real output for a record it describes: the
// docs are the contract (docs/json.md), so a drift fails here.
func TestInboxJSONMatchesTheDocumentedExample(t *testing.T) {
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--option", "b:Use B",
			"--tradeoffs", "a:Younger project", "--recommend", "a", "--reasoning", "A is proven",
			"--key", "storage.engine", "--by", "claude-opus"},
		{"ask", "Logging"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	r := run(testEnv(root), "", "inbox", "--json")
	if want := docsJSONExample(t, "## `adj inbox --json`"); r.stdout != want {
		t.Errorf("inbox --json =\n%s\nwant the docs/json.md example\n%s", r.stdout, want)
	}
}

func TestInboxHelpPrintsUsageToStdout(t *testing.T) {
	for _, flag := range []string{"--help", "-h"} {
		r := run(testEnv(t.TempDir()), "", "inbox", flag)
		if r.code != 0 || !strings.Contains(r.stdout, "usage: adj inbox [flags]\n") || r.stderr != "" {
			t.Errorf("inbox %s: exit %d, stdout %q, stderr %q; want exit 0 and usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
}

func TestInboxListedInTheMainUsage(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(r.stdout, "adj inbox") {
		t.Errorf("main usage does not list adj inbox:\n%s", r.stdout)
	}
}

func TestInboxBadArgumentsAreUsageErrors(t *testing.T) {
	root := seedInbox(t)
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "positional argument", args: []string{"inbox", "extra"}, wantErr: "no arguments"},
		{name: "unknown flag", args: []string{"inbox", "--nope"}, wantErr: "nope"},
		{name: "list filter is not an inbox flag", args: []string{"inbox", "--status", "open"}, wantErr: "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj inbox [flags]\n") {
				t.Errorf("stderr should contain %q and the inbox usage:\n%s", tt.wantErr, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
		})
	}
}

func TestInboxNeedsAnInitialisedRepo(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "inbox")
	if r.code != 1 || !strings.Contains(r.stderr, "adj init") || r.stdout != "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 telling the user to run adj init", r.code, r.stdout, r.stderr)
	}
}

func TestInboxFailsOnAnUnreadableRecordFile(t *testing.T) {
	root := seedInbox(t)
	bad := filepath.Join(root, "decisions", "0008-broken.md")
	if err := os.WriteFile(bad, []byte("not a record\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"inbox"}, {"inbox", "--json"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 1 || !strings.Contains(r.stderr, "0008-broken.md") || r.stdout != "" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1 naming 0008-broken.md and no output", args, r.code, r.stdout, r.stderr)
		}
	}
}
