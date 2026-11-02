// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// seedRefInbox fills the repo at root with records that differ only in their
// refs, each asked on its own date:
//
//	1 open     2026-03-05 refs S11
//	2 open     2026-03-01 refs S110
//	3 open     2026-03-03 refs S116, S11
//	4 open     2026-03-04 refs s11
//	5 open     2026-03-02 refs " S11 " (surrounding spaces)
//	6 open     2026-03-06 no refs
//	7 decided  2026-03-07 refs S11 (answered, so never in the inbox)
//
// With --ref S11 the inbox is 3, 1: oldest first.
func seedRefInbox(t *testing.T, root string) {
	t.Helper()
	steps := []struct {
		date string
		args []string
	}{
		{"2026-03-05", []string{"ask", "Exact", "--ref", "S11", "--option", "a:A"}},
		{"2026-03-01", []string{"ask", "Longer number", "--ref", "S110"}},
		{"2026-03-03", []string{"ask", "Both", "--ref", "S116", "--ref", "S11", "--option", "a:A"}},
		{"2026-03-04", []string{"ask", "Lower case", "--ref", "s11"}},
		{"2026-03-02", []string{"ask", "Spaced", "--ref", " S11 "}},
		{"2026-03-06", []string{"ask", "No refs"}},
		{"2026-03-07", []string{"ask", "Answered", "--ref", "S11", "--option", "a:A"}},
		{"2026-03-08", []string{"decide", "7", "a", "--why", "ok"}},
	}
	for _, s := range steps {
		if r := run(envOn(t, root, s.date), "", s.args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", s.args, r.code, r.stderr)
		}
	}
}

func idsOf(t *testing.T, recs []model.Record) []int {
	t.Helper()
	ids := []int{}
	for _, rec := range recs {
		ids = append(ids, rec.ID)
	}
	return ids
}

func TestInboxRefFilter(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []int
	}{
		{"exact", []string{"--ref", "S11"}, []int{3, 1}},
		{"longer number excluded", []string{"--ref", "S110"}, []int{2}},
		{"case excluded", []string{"--ref", "s11"}, []int{4}},
		{"spaces are part of the ref", []string{"--ref", " S11 "}, []int{5}},
		{"every ref must match", []string{"--ref", "S11", "--ref", "S116"}, []int{3}},
		{"repeated ref is one requirement", []string{"--ref", "S11", "--ref", "S11"}, []int{3, 1}},
		{"refs that never meet", []string{"--ref", "S110", "--ref", "S116"}, []int{}},
		{"unknown ref", []string{"--ref", "S999"}, []int{}},
		{"no ref is the whole inbox", nil, []int{2, 5, 3, 4, 1, 6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			seedRefInbox(t, root)

			js := run(testEnv(root), "", append([]string{"inbox", "--json"}, tt.args...)...)
			if js.code != 0 || js.stderr != "" {
				t.Fatalf("json: exit %d, stderr %q", js.code, js.stderr)
			}
			var out struct {
				Records []model.Record `json:"records"`
			}
			if err := json.Unmarshal([]byte(js.stdout), &out); err != nil {
				t.Fatalf("json: %v: %s", err, js.stdout)
			}
			if out.Records == nil {
				t.Fatalf("records must be an array: %s", js.stdout)
			}
			if got := idsOf(t, out.Records); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("json ids = %v, want %v", got, tt.want)
			}

			text := run(testEnv(root), "", append([]string{"inbox"}, tt.args...)...)
			if text.code != 0 || text.stderr != "" {
				t.Fatalf("text: exit %d, stderr %q", text.code, text.stderr)
			}
			if len(tt.want) == 0 {
				if text.stdout != "" {
					t.Errorf("empty match printed %q", text.stdout)
				}
				if want := "{\n  \"records\": []\n}\n"; js.stdout != want {
					t.Errorf("empty json = %q, want %q", js.stdout, want)
				}
				return
			}
			titles := map[int]string{1: "Exact", 2: "Longer number", 3: "Both", 4: "Lower case", 5: "Spaced", 6: "No refs"}
			var gotTitles, wantTitles []string
			for _, line := range strings.Split(text.stdout, "\n") {
				if line != "" && !strings.HasPrefix(line, " ") {
					gotTitles = append(gotTitles, line)
				}
			}
			for _, id := range tt.want {
				wantTitles = append(wantTitles, fmt.Sprintf("%d  %s", id, titles[id]))
			}
			if !reflect.DeepEqual(gotTitles, wantTitles) {
				t.Errorf("text headings = %q, want %q", gotTitles, wantTitles)
			}
		})
	}
}

// With a filter the records are shown as they are without one: only the
// choice of records changes.
func TestInboxRefTextShowsFullBlock(t *testing.T) {
	root := initRoot(t)
	seedRefInbox(t, root)
	r := run(testEnv(root), "", "inbox", "--ref", "S116")
	want := "3  Both\n  Created: 2026-03-03\n  Refs: S116, S11\n  Options:\n    a: A\n"
	if full := run(testEnv(root), "", "inbox"); !strings.Contains(full.stdout, strings.TrimSuffix(want, "\n")) {
		t.Fatalf("fixture drifted: unfiltered output lacks %q:\n%s", want, full.stdout)
	}
	if r.code != 0 || r.stdout != want {
		t.Errorf("exit %d, stdout %q, want %q", r.code, r.stdout, want)
	}
}

func TestInboxRefBlankBeforeStore(t *testing.T) {
	for _, mode := range [][]string{{"inbox"}, {"inbox", "-i"}, {"inbox", "--json"}} {
		for _, value := range []string{"", " \t "} {
			t.Run(strings.Join(mode, " ")+fmt.Sprintf("/%q", value), func(t *testing.T) {
				// An empty directory has no repo: a store lookup would exit 1.
				r := run(testEnv(t.TempDir()), "", append(mode, "--ref", value)...)
				// JSON mode also prints the error object (S42).
				wantOut := ""
				if mode[len(mode)-1] == "--json" {
					wantOut = `{
  "error": {
    "code": "usage",
    "message": "--ref must not be blank",
    "command": "inbox"
  }
}
`
				}
				if r.code != 2 || r.stdout != wantOut || !strings.Contains(r.stderr, "--ref must not be blank") {
					t.Errorf("exit %d, stdout %q, stderr %q; want blank ref usage error", r.code, r.stdout, r.stderr)
				}
			})
		}
	}
	t.Run("a good ref before a blank one", func(t *testing.T) {
		r := run(testEnv(t.TempDir()), "", "inbox", "--ref", "S11", "--ref", "")
		if r.code != 2 || !strings.Contains(r.stderr, "--ref must not be blank") {
			t.Errorf("exit %d, stderr %q", r.code, r.stderr)
		}
	})
	t.Run("all reads no user config", func(t *testing.T) {
		env, _, _ := reposEnv(t)
		r := run(env, "", "inbox", "--all", "--ref", " ")
		if r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "--ref must not be blank") {
			t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
		}
	})
}

func TestInboxRefHelp(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "inbox", "--help")
	if r.code != 0 || !strings.Contains(r.stdout, "\n  --ref REF    only questions with this exact ref") {
		t.Errorf("help does not name --ref: exit %d\n%s", r.code, r.stdout)
	}
	for _, line := range strings.Split(r.stdout, "\n")[:3] {
		if !strings.Contains(line, "--ref REF") {
			t.Errorf("synopsis line %q does not name --ref", line)
		}
	}
}

// --all applies the filter in each repo; a repo with no match keeps its header.
func TestInboxAllRefFilter(t *testing.T) {
	env, path, first := reposEnv(t)
	second := initRoot(t)
	third := initRoot(t)
	for _, root := range []string{first, second} {
		seedRefInbox(t, root)
	}
	// The third repo has open questions, none with the ref.
	if r := run(envOn(t, third, "2026-03-01"), "", "ask", "Unrelated", "--ref", "S110"); r.code != 0 {
		t.Fatal(r)
	}
	data, err := json.Marshal(map[string]any{"repos": []string{second, third, first}})
	if err != nil {
		t.Fatal(err)
	}
	writeReposConfig(t, path, string(data))

	js := run(env, "", "inbox", "--all", "--json", "--ref", "S11")
	if js.code != 0 || js.stderr != "" {
		t.Fatalf("json: %+v", js)
	}
	var out struct {
		Repos []struct {
			Root    string         `json:"root"`
			Records []model.Record `json:"records"`
		} `json:"repos"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal([]byte(js.stdout), &out); err != nil {
		t.Fatalf("%v: %s", err, js.stdout)
	}
	wantRoots := []string{second, third, first}
	wantIDs := [][]int{{3, 1}, {}, {3, 1}}
	if len(out.Repos) != 3 {
		t.Fatalf("repos = %s", js.stdout)
	}
	for i, repo := range out.Repos {
		if repo.Root != wantRoots[i] {
			t.Errorf("repo %d root = %q, want %q", i, repo.Root, wantRoots[i])
		}
		if repo.Records == nil || !reflect.DeepEqual(idsOf(t, repo.Records), wantIDs[i]) {
			t.Errorf("repo %d ids = %v (nil=%v), want %v", i, idsOf(t, repo.Records), repo.Records == nil, wantIDs[i])
		}
	}

	text := run(env, "", "inbox", "--all", "--ref", "S11", "--ref", "S116")
	if text.code != 0 || text.stderr != "" {
		t.Fatalf("text: %+v", text)
	}
	want := second + "\n3  Both\n  Created: 2026-03-03\n  Refs: S116, S11\n  Options:\n    a: A\n" +
		"\n" + third + "\n" +
		"\n" + first + "\n3  Both\n  Created: 2026-03-03\n  Refs: S116, S11\n  Options:\n    a: A\n"
	if text.stdout != want {
		t.Errorf("text = %q, want %q", text.stdout, want)
	}
}

// -i walks only the matching questions and its summary counts only those.
func TestInboxInteractiveRefFilter(t *testing.T) {
	root := initRoot(t)
	seedRefInbox(t, root)
	// Matching questions are 3 then 1: skip 3, reject 1. Nothing else is shown.
	r := run(testEnv(root), "s\nr\nnot needed\n", "inbox", "-i", "--ref", "S11", "--by", "Owner")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("walk: %+v", r)
	}
	if !strings.HasSuffix(r.stdout, "0 decided, 1 rejected, 0 withdrawn, 1 skipped, 0 failed\n") {
		t.Errorf("summary: %q", r.stdout)
	}
	for _, title := range []string{"Longer number", "Lower case", "Spaced", "No refs"} {
		if strings.Contains(r.stdout, title) {
			t.Errorf("walk showed %q: %q", title, r.stdout)
		}
	}
	if strings.Index(r.stdout, "3  Both") < 0 || strings.Index(r.stdout, "3  Both") > strings.Index(r.stdout, "1  Exact") {
		t.Errorf("walk order: %q", r.stdout)
	}
	byID := map[int]model.Record{}
	for _, rec := range records(t, root) {
		byID[rec.ID] = rec
	}
	if byID[1].Status != model.StatusRejected {
		t.Errorf("record 1 status = %s, want rejected", byID[1].Status)
	}
	for _, id := range []int{2, 3, 4, 5, 6} {
		if byID[id].Status != model.StatusOpen {
			t.Errorf("record %d status = %s, want open", id, byID[id].Status)
		}
	}

	none := run(testEnv(root), "", "inbox", "-i", "--ref", "S999")
	if none.code != 0 || none.stdout != "" || none.stderr != "" {
		t.Errorf("no match: %+v", none)
	}
}
