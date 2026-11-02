// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const narrowScope = "only for structured questions"

// seedNarrow makes a repo with five records, each with its own key so lint is
// clean, asked and answered on the fixed clock:
//
//	1 decided    "Answers are free text"   ui.answers
//	2 decided    "Structured answers"      ui.structured
//	3 decided    "Sidebar shows running"   ui.sidebar
//	4 open       "Undecided question"
//	5 rejected   "Rejected question"
func seedNarrow(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Answers are free text", "--option", "a:Free text", "--key", "ui.answers"},
		{"ask", "Structured answers", "--option", "b:Pick one", "--key", "ui.structured"},
		{"ask", "Sidebar shows running", "--option", "c:Show it", "--key", "ui.sidebar"},
		{"ask", "Undecided question"},
		{"ask", "Rejected question"},
		{"decide", "1", "a", "--why", "proven", "--by", "owner"},
		{"decide", "2", "b", "--why", "faster", "--by", "owner"},
		{"decide", "3", "c", "--why", "newest", "--by", "owner"},
		{"reject", "5", "--why", "not needed"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	return root
}

func TestNarrowLinksBothRecordsAndChangesNothingElse(t *testing.T) {
	root := seedNarrow(t)
	oldBefore, newBefore := recordByID(t, root, 1), recordByID(t, root, 2)
	treeBefore := snapshot(t, root)

	r := run(envOn(t, root, "2026-05-06"), "", "narrow", "1", "--by", "2", "--scope", narrowScope)
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	if r.stdout != "1\n" || r.stderr != "" {
		t.Errorf("stdout = %q, stderr = %q; want the narrowed ID and nothing on stderr", r.stdout, r.stderr)
	}

	wantOld := oldBefore
	wantOld.NarrowedBy = []model.Narrowing{{ID: 2, Scope: narrowScope}}
	wantOld.History = append(slices.Clip(oldBefore.History), model.HistoryEntry{Date: "2026-05-06", Event: model.EventNarrow, Note: "by 2: " + narrowScope})
	if got := recordByID(t, root, 1); !reflect.DeepEqual(got, wantOld) {
		t.Errorf("old record:\n got %+v\nwant %+v", got, wantOld)
	}
	wantNew := newBefore
	wantNew.Narrows = []int{1}
	wantNew.History = append(slices.Clip(newBefore.History), model.HistoryEntry{Date: "2026-05-06", Event: model.EventNarrow, Note: "narrows 1"})
	if got := recordByID(t, root, 2); !reflect.DeepEqual(got, wantNew) {
		t.Errorf("new record:\n got %+v\nwant %+v", got, wantNew)
	}
	if got := recordByID(t, root, 3); len(got.NarrowedBy) != 0 || len(got.Narrows) != 0 || got.Status != model.StatusDecided {
		t.Errorf("an unrelated record changed: %+v", got)
	}
	treeAfter := snapshot(t, root)
	if len(treeAfter) != len(treeBefore) {
		t.Errorf("files before %q\nfiles after  %q", treeBefore, treeAfter)
	}
	for name, was := range treeBefore {
		if after, ok := treeAfter[name]; !ok {
			t.Errorf("%s is gone after narrow", name)
		} else if after != was && !strings.Contains(name, "0001-") && !strings.Contains(name, "0002-") {
			t.Errorf("%s changed", name)
		}
	}
}

// OLD stays in force: it is in adj context with the scope and the narrowing
// record, which names OLD.
func TestNarrowKeepsTheOldRecordInContextWithItsScope(t *testing.T) {
	root := seedNarrow(t)
	if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
		t.Fatalf("narrow: exit %d: %s", r.code, r.stderr)
	}
	if got := contextIDs(t, root); !slices.Equal(got, []int{1, 2, 3}) {
		t.Errorf("context = %v, want [1 2 3]: a narrowed record stays in force", got)
	}
	r := run(testEnv(root), "", "context")
	const want = `- 1 Answers are free text [ui.answers]: a
  Narrowed by: 2 (only for structured questions)
  Rationale: proven
- 2 Structured answers [ui.structured]: b
  Narrows: 1
  Rationale: faster
- 3 Sidebar shows running [ui.sidebar]: c
  Rationale: newest
`
	if r.code != 0 || r.stdout != want {
		t.Errorf("context: exit %d, stdout:\n%s\nwant\n%s", r.code, r.stdout, want)
	}

	j := run(testEnv(root), "", "context", "--format", "json")
	if j.code != 0 {
		t.Fatalf("context json: exit %d: %s", j.code, j.stderr)
	}
	entries := recordObjects(t, j.stdout, "decisions")
	wantBy := []any{map[string]any{"id": 2.0, "scope": narrowScope}}
	if !reflect.DeepEqual(entries[0]["narrowed_by"], wantBy) || entries[0]["narrows"] != nil {
		t.Errorf("entry 1 = %v, want narrowed_by %v and no narrows", entries[0], wantBy)
	}
	if !reflect.DeepEqual(entries[1]["narrows"], []any{1.0}) || entries[1]["narrowed_by"] != nil {
		t.Errorf("entry 2 = %v, want narrows [1] and no narrowed_by", entries[1])
	}
	if _, ok := entries[2]["narrowed_by"]; ok {
		t.Errorf("entry 3 has a narrowed_by key: %v", entries[2])
	}
	if _, ok := entries[2]["narrows"]; ok {
		t.Errorf("entry 3 has a narrows key: %v", entries[2])
	}
}

func TestNarrowShowsTheLinkBothWays(t *testing.T) {
	root := seedNarrow(t)
	if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
		t.Fatalf("narrow: exit %d: %s", r.code, r.stderr)
	}

	oldShow := run(testEnv(root), "", "show", "1")
	for _, want := range []string{"\nNarrowed by: 2 (only for structured questions)\n", "narrow: by 2: only for structured questions\n"} {
		if !strings.Contains(oldShow.stdout, want) {
			t.Errorf("show 1 does not print %q:\n%s", want, oldShow.stdout)
		}
	}
	if strings.Contains(oldShow.stdout, "Narrows:") {
		t.Errorf("show 1 prints Narrows:\n%s", oldShow.stdout)
	}
	newShow := run(testEnv(root), "", "show", "2")
	for _, want := range []string{"\nNarrows: 1\n", "narrow: narrows 1\n"} {
		if !strings.Contains(newShow.stdout, want) {
			t.Errorf("show 2 does not print %q:\n%s", want, newShow.stdout)
		}
	}
	if strings.Contains(newShow.stdout, "Narrowed by:") {
		t.Errorf("show 2 prints Narrowed by:\n%s", newShow.stdout)
	}
	// The records that are not narrowed print neither.
	if other := run(testEnv(root), "", "show", "3").stdout; strings.Contains(other, "Narrow") {
		t.Errorf("show 3 mentions narrowing:\n%s", other)
	}

	// The JSON of every read command carries the keys.
	for _, args := range [][]string{{"show", "1", "--json"}, {"list", "--json"}, {"check", "--json", "free text"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
		var objs []map[string]any
		switch args[0] {
		case "show":
			objs = recordObjects(t, r.stdout)
		case "list":
			objs = recordObjects(t, r.stdout, "records")
		default:
			objs = recordObjects(t, r.stdout, "decided")
		}
		got := objs[0]["narrowed_by"]
		if want := []any{map[string]any{"id": 2.0, "scope": narrowScope}}; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: narrowed_by = %v, want %v", args, got, want)
		}
	}
	list := recordObjects(t, run(testEnv(root), "", "list", "--json").stdout, "records")
	if !reflect.DeepEqual(list[1]["narrows"], []any{1.0}) {
		t.Errorf("list: record 2 narrows = %v, want [1]", list[1]["narrows"])
	}
	for _, key := range []string{"narrowed_by", "narrows"} {
		if _, ok := list[2][key]; ok {
			t.Errorf("list: record 3 has a %s key", key)
		}
	}
}

func TestNarrowOneRecordCanBeNarrowedByManyAndNarrowManyRecords(t *testing.T) {
	root := seedNarrow(t)
	for _, args := range [][]string{
		{"narrow", "1", "--by", "2", "--scope", "scope two"},
		{"narrow", "1", "--by", "3", "--scope", "scope three"},
		{"narrow", "2", "--by", "3", "--scope", "scope three, on two"},
	} {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	if got := recordByID(t, root, 1).NarrowedBy; !reflect.DeepEqual(got, []model.Narrowing{{ID: 2, Scope: "scope two"}, {ID: 3, Scope: "scope three"}}) {
		t.Errorf("record 1 narrowed_by = %+v", got)
	}
	if got := recordByID(t, root, 3).Narrows; !reflect.DeepEqual(got, []int{1, 2}) {
		t.Errorf("record 3 narrows = %v, want [1 2]", got)
	}
	show := run(testEnv(root), "", "show", "1").stdout
	if !strings.Contains(show, "Narrowed by: 2 (scope two)\nNarrowed by: 3 (scope three)\n") {
		t.Errorf("show 1 does not print one Narrowed by line per link, in order:\n%s", show)
	}
	ctx := run(testEnv(root), "", "context").stdout
	if !strings.Contains(ctx, "  Narrowed by: 2 (scope two)\n  Narrowed by: 3 (scope three)\n") {
		t.Errorf("context does not print one line per narrowing:\n%s", ctx)
	}
	if lint := run(testEnv(root), "", "lint"); lint.code != 0 || lint.stdout != "" {
		t.Errorf("lint: exit %d, stdout %q; want a clean store", lint.code, lint.stdout)
	}
}

func TestNarrowFlagPositionAndRoot(t *testing.T) {
	root := seedNarrow(t)
	r := run(testEnv(""), "", "narrow", "--root", root, "--scope", narrowScope, "--by", "2", "1")
	if r.code != 0 || r.stdout != "1\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want exit 0 and \"1\\n\"", r.code, r.stdout, r.stderr)
	}
	// The scope is kept as typed, surrounding space included.
	root2 := seedNarrow(t)
	if r := run(testEnv(root2), "", "narrow", "1", "--by", "2", "--scope", "  padded, with <angle> & \"quotes\"  "); r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if got := recordByID(t, root2, 1).NarrowedBy[0].Scope; got != "  padded, with <angle> & \"quotes\"  " {
		t.Errorf("scope = %q, want it as typed", got)
	}
}

func TestNarrowRefusesWhatItCannotDoAndWritesNothing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    int
		wantErr string
	}{
		{"old is open", []string{"narrow", "4", "--by", "2", "--scope", "s"}, 1, "record 4 is open; only a decided record can be narrowed"},
		{"old is rejected", []string{"narrow", "5", "--by", "2", "--scope", "s"}, 1, "record 5 is rejected; only a decided record can be narrowed"},
		{"new is open", []string{"narrow", "1", "--by", "4", "--scope", "s"}, 1, "record 4 is open; only a decided record can narrow another"},
		{"new is rejected", []string{"narrow", "1", "--by", "5", "--scope", "s"}, 1, "record 5 is rejected; only a decided record can narrow another"},
		{"both not decided: old is reported first", []string{"narrow", "4", "--by", "5", "--scope", "s"}, 1, "record 4 is open; only a decided record can be narrowed"},
		{"unknown old", []string{"narrow", "99", "--by", "2", "--scope", "s"}, 1, "record not found"},
		{"unknown new", []string{"narrow", "1", "--by", "99", "--scope", "s"}, 1, "record not found"},
		{"old equals new", []string{"narrow", "1", "--by", "1", "--scope", "s"}, 2, "OLD and --by must be different records"},
		{"no --by", []string{"narrow", "1", "--scope", "s"}, 2, "--by is required"},
		{"blank --by", []string{"narrow", "1", "--by", " ", "--scope", "s"}, 2, "--by must be a positive integer"},
		{"--by is not a number", []string{"narrow", "1", "--by", "two", "--scope", "s"}, 2, "--by must be a positive integer"},
		{"--by is zero", []string{"narrow", "1", "--by", "0", "--scope", "s"}, 2, "--by must be a positive integer"},
		{"--by is negative", []string{"narrow", "1", "--by", "-2", "--scope", "s"}, 2, "--by must be a positive integer"},
		{"no --scope", []string{"narrow", "1", "--by", "2"}, 2, "--scope is required"},
		{"empty --scope", []string{"narrow", "1", "--by", "2", "--scope", ""}, 2, "--scope must not be blank"},
		{"blank --scope", []string{"narrow", "1", "--by", "2", "--scope", " \t"}, 2, "--scope must not be blank"},
		{"--scope on two lines", []string{"narrow", "1", "--by", "2", "--scope", "a\nb"}, 2, "--scope must be one line"},
		{"no OLD", []string{"narrow", "--by", "2", "--scope", "s"}, 2, "ID is required"},
		{"OLD is not a number", []string{"narrow", "x", "--by", "2", "--scope", "s"}, 2, "ID must be a positive integer"},
		{"OLD is zero", []string{"narrow", "0", "--by", "2", "--scope", "s"}, 2, "ID must be a positive integer"},
		{"extra argument", []string{"narrow", "1", "2", "--by", "2", "--scope", "s"}, 2, "want one ID"},
		{"unknown flag", []string{"narrow", "1", "--by", "2", "--scope", "s", "--why", "x"}, 2, "why"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := seedNarrow(t)
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != tt.want {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", r.code, tt.want, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("stderr should contain %q:\n%s", tt.wantErr, r.stderr)
			}
			if tt.want == 2 && !strings.Contains(r.stderr, "usage: adj narrow") {
				t.Errorf("a usage error should print the usage:\n%s", r.stderr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("the store changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

// The same pair twice is a failure, not a second link (C32).
func TestNarrowTheSamePairTwiceFailsTheSecondTime(t *testing.T) {
	root := seedNarrow(t)
	if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
		t.Fatalf("first narrow: exit %d: %s", r.code, r.stderr)
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", "a different scope")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, "record 1 is already narrowed by record 2") {
		t.Errorf("exit %d, stdout %q, stderr:\n%s; want exit 1 saying it is already narrowed", r.code, r.stdout, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Error("the store changed on the refused second narrow")
	}
	if got := recordByID(t, root, 1).NarrowedBy; len(got) != 1 || got[0].Scope != narrowScope {
		t.Errorf("narrowed_by = %+v, want the first link only", got)
	}
}

func TestNarrowBadArgumentsBeatMissingInit(t *testing.T) {
	root := t.TempDir() // no .adjudicate.json
	before := snapshot(t, root)
	if r := run(testEnv(root), "", "narrow", "1", "--by", "1", "--scope", "s"); r.code != 2 {
		t.Errorf("equal IDs without init: exit %d, want 2 (usage error before the init check); stderr:\n%s", r.code, r.stderr)
	}
	if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", " "); r.code != 2 {
		t.Errorf("blank scope without init: exit %d, want 2; stderr:\n%s", r.code, r.stderr)
	}
	r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", "s")
	if r.code != 1 || !strings.Contains(r.stderr, "adj init") {
		t.Errorf("no repo: exit %d, stderr:\n%s; want exit 1 telling the user to run adj init", r.code, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed: %q", after)
	}
}

func TestNarrowHelpAndMainUsage(t *testing.T) {
	env := testEnv(t.TempDir())
	for _, flag := range []string{"--help", "-h"} {
		r := run(env, "", "narrow", flag)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj narrow OLD --by NEW --scope TEXT") || r.stderr != "" {
			t.Errorf("narrow %s: exit %d, stdout %q, stderr %q; want exit 0 and the usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
	if main := run(env, "", "--help").stdout; !strings.Contains(main, "adj narrow") {
		t.Errorf("main usage does not list adj narrow:\n%s", main)
	}
}

// narrow has no --json: a failure prints no error object (S42).
func TestNarrowFailureHasNoJSONErrorObject(t *testing.T) {
	root := seedNarrow(t)
	r := run(testEnv(root), "", "narrow", "4", "--by", "2", "--scope", "s")
	if r.code != 1 || r.stdout != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and nothing on stdout", r.code, r.stdout)
	}
}

// A full supersede of a narrowed record still works, whichever side of the
// narrowing it is on, and lint tells when the narrowing record was replaced.
func TestSupersedeStillWorksOnNarrowedRecords(t *testing.T) {
	t.Run("the narrowed record is superseded", func(t *testing.T) {
		root := seedNarrow(t)
		if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
			t.Fatalf("narrow: exit %d: %s", r.code, r.stderr)
		}
		if r := run(testEnv(root), "", "supersede", "1", "--by", "3"); r.code != 0 || r.stdout != "1\n" {
			t.Fatalf("supersede: exit %d, stdout %q: %s", r.code, r.stdout, r.stderr)
		}
		old := recordByID(t, root, 1)
		if old.Status != model.StatusSuperseded || old.SupersededBy != 3 || len(old.NarrowedBy) != 1 {
			t.Errorf("record 1 = %+v, want superseded by 3 and still narrowed by 2", old)
		}
		if got := contextIDs(t, root); !slices.Equal(got, []int{2, 3}) {
			t.Errorf("context = %v, want [2 3]", got)
		}
		if l := run(testEnv(root), "", "lint"); l.code != 0 || l.stdout != "" {
			t.Errorf("lint: exit %d, stdout %q; want a clean store", l.code, l.stdout)
		}
	})
	t.Run("the narrowing record is superseded", func(t *testing.T) {
		root := seedNarrow(t)
		if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
			t.Fatalf("narrow: exit %d: %s", r.code, r.stderr)
		}
		if r := run(testEnv(root), "", "supersede", "2", "--by", "3"); r.code != 0 {
			t.Fatalf("supersede: exit %d: %s", r.code, r.stderr)
		}
		l := run(testEnv(root), "", "lint")
		const want = "decisions/0001-answers-are-free-text.md: record 1: warning: narrow-superseded: " +
			"narrowed_by 2, which is superseded by 3: check that the scope still holds\n0 errors, 1 warning\n"
		if l.code != 0 || l.stdout != want {
			t.Errorf("lint: exit %d, stdout:\n%s\nwant\n%s", l.code, l.stdout, want)
		}
	})
	t.Run("a superseding record can be narrowed and narrow", func(t *testing.T) {
		root := seedNarrow(t)
		for _, args := range [][]string{
			{"supersede", "1", "--by", "2"},
			{"narrow", "3", "--by", "2", "--scope", "s"},
		} {
			if r := run(testEnv(root), "", args...); r.code != 0 {
				t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
			}
		}
		if got := recordByID(t, root, 2); !reflect.DeepEqual(got.Supersedes, []int{1}) || !reflect.DeepEqual(got.Narrows, []int{3}) {
			t.Errorf("record 2 = %+v, want supersedes [1] and narrows [3]", got)
		}
	})
	t.Run("a superseded record cannot be narrowed", func(t *testing.T) {
		root := seedNarrow(t)
		if r := run(testEnv(root), "", "supersede", "1", "--by", "2"); r.code != 0 {
			t.Fatalf("supersede: exit %d: %s", r.code, r.stderr)
		}
		r := run(testEnv(root), "", "narrow", "1", "--by", "3", "--scope", "s")
		if r.code != 1 || !strings.Contains(r.stderr, "record 1 is superseded; only a decided record can be narrowed") {
			t.Errorf("exit %d, stderr:\n%s", r.code, r.stderr)
		}
	})
}

// Every command that rewrites a record keeps its narrow links; amend and
// renumber are the ones that can run on a narrowed record.
func TestCommandsLeaveTheNarrowLinksAlone(t *testing.T) {
	t.Run("amend", func(t *testing.T) {
		root := seedNarrow(t)
		if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
			t.Fatalf("narrow: %+v", r)
		}
		for _, id := range []string{"1", "2"} {
			if r := run(testEnv(root), "", "amend", id, "--rationale", "better why"); r.code != 0 {
				t.Fatalf("amend %s: %+v", id, r)
			}
		}
		if got := recordByID(t, root, 1).NarrowedBy; !reflect.DeepEqual(got, []model.Narrowing{{ID: 2, Scope: narrowScope}}) {
			t.Errorf("record 1 narrowed_by = %+v after amend", got)
		}
		if got := recordByID(t, root, 2).Narrows; !reflect.DeepEqual(got, []int{1}) {
			t.Errorf("record 2 narrows = %v after amend", got)
		}
	})
	t.Run("renumber keeps the links of the record it moves", func(t *testing.T) {
		root := seedNarrow(t)
		if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
			t.Fatalf("narrow: %+v", r)
		}
		dir := filepath.Join(root, "decisions")
		data, err := os.ReadFile(filepath.Join(dir, "0002-structured-answers.md"))
		if err != nil {
			t.Fatal(err)
		}
		// A second file with ID 2 that was created later, as a merged branch
		// makes one: it is the one that moves. Record 1 names ID 2, so the
		// link has to be accepted.
		twin := strings.Replace(string(data), `"created": "2026-`, `"created": "2027-`, 1)
		if err := os.WriteFile(filepath.Join(dir, "0002-structured-answers-twin.md"), []byte(twin), 0o644); err != nil {
			t.Fatal(err)
		}
		r := run(testEnv(root), "", "renumber", "--keep-links")
		if r.code != 0 || !strings.HasPrefix(r.stdout, "0002-structured-answers-twin.md -> ") {
			t.Fatalf("renumber: %+v, want the twin moved", r)
		}
		moved := recordByID(t, root, 6)
		if !reflect.DeepEqual(moved.Narrows, []int{1}) || moved.History[len(moved.History)-1].Event != model.EventRenumber {
			t.Errorf("moved record = %+v, want narrows [1] kept and a renumber entry last", moved)
		}
		if got := recordByID(t, root, 1).NarrowedBy; len(got) != 1 {
			t.Errorf("record 1 narrowed_by = %+v, want it unchanged", got)
		}
	})
}

// A narrow link to a colliding ID cannot be told apart from a link to the other
// record, so renumber lists it and refuses, as for supersede links (C147).
func TestRenumberListsNarrowLinksToACollidingID(t *testing.T) {
	root := seedNarrow(t)
	if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope); r.code != 0 {
		t.Fatalf("narrow: %+v", r)
	}
	dir := filepath.Join(root, "decisions")
	data, err := os.ReadFile(filepath.Join(dir, "0001-answers-are-free-text.md"))
	if err != nil {
		t.Fatal(err)
	}
	// A second file with ID 1, as a merged branch makes: record 2 narrows "1".
	twin := strings.Replace(string(data), "2026-", "2027-", 1)
	if err := os.WriteFile(filepath.Join(dir, "0001-answers-twin.md"), []byte(twin), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)

	r := run(testEnv(root), "", "renumber")
	if r.code != 1 || r.stdout != "" {
		t.Fatalf("exit %d, stdout %q; want exit 1 and nothing moved", r.code, r.stdout)
	}
	for _, want := range []string{"0002-structured-answers.md narrows 1", "0001-answers-are-free-text.md"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, r.stderr)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Error("a refused renumber changed the store")
	}

	k := run(testEnv(root), "", "renumber", "--keep-links")
	if k.code != 0 || !strings.Contains(k.stdout, "0001-answers-twin.md -> ") {
		t.Fatalf("renumber --keep-links: exit %d, stdout %q; want the twin moved", k.code, k.stdout)
	}
	if !strings.Contains(k.stderr, "0002-structured-answers.md narrows 1") {
		t.Errorf("stderr should list the narrow link that stays:\n%s", k.stderr)
	}
}

// The JSON of a narrowed record does not need a version bump: the new keys are
// additive (S45). They are in the docs table of a record, and a record without
// narrowing prints neither.
func TestNarrowKeysAreDocumentedAndOptional(t *testing.T) {
	for _, heading := range []string{"## Keys of a record", "## `adj context --format json`"} {
		keys := docsTableKeys(t, heading)
		for _, want := range []string{"narrowed_by", "narrows"} {
			if !slices.Contains(keys, want) {
				t.Errorf("%s table lacks %q: %v", heading, want, keys)
			}
		}
	}
	root := seedNarrow(t)
	var obj map[string]any
	if err := json.Unmarshal([]byte(run(testEnv(root), "", "show", "1", "--json").stdout), &obj); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"narrowed_by", "narrows"} {
		if _, ok := obj[key]; ok {
			t.Errorf("a record that is not narrowed prints %q", key)
		}
	}
}

// The half link that a narrow leaves when its NEW write and its restore both
// fail (OLD lists NEW, NEW has no link back) is completed by narrowing again:
// exit 0, OLD's ID, record 1's file untouched, record 2 linked; lint flags the half link before and
// is clean after.
func TestNarrowAgainCompletesAHalfLink(t *testing.T) {
	root := seedNarrow(t)
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := st.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := model.Narrow(&rec, 2, narrowScope, "2026-05-06"); err != nil {
		t.Fatal(err)
	}
	if err := st.Update(rec); err != nil {
		t.Fatal(err)
	}
	l := run(testEnv(root), "", "lint")
	if l.code != 1 || !strings.Contains(l.stdout, "narrow-asymmetric: narrowed_by 2, but record 2 does not narrow 1") {
		t.Fatalf("lint of the half link: exit %d, stdout:\n%s", l.code, l.stdout)
	}
	oldFile := filepath.Join(root, "decisions", "0001-answers-are-free-text.md")
	oldBytes, err := os.ReadFile(oldFile)
	if err != nil {
		t.Fatal(err)
	}

	r := run(envOn(t, root, "2026-05-07"), "", "narrow", "1", "--by", "2", "--scope", "ignored scope")
	if r.code != 0 || r.stdout != "1\n" || r.stderr != "" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want exit 0 and \"1\\n\"", r.code, r.stdout, r.stderr)
	}
	if after, _ := os.ReadFile(oldFile); string(after) != string(oldBytes) {
		t.Error("record 1 was rewritten")
	}
	got := recordByID(t, root, 2)
	last := got.History[len(got.History)-1]
	if !reflect.DeepEqual(got.Narrows, []int{1}) || last != (model.HistoryEntry{Date: "2026-05-07", Event: model.EventNarrow, Note: "narrows 1"}) {
		t.Errorf("record 2 = %+v, want narrows [1] and one narrows 1 entry last", got)
	}
	if l := run(testEnv(root), "", "lint"); l.code != 0 || l.stdout != "" {
		t.Errorf("lint after: exit %d, stdout %q; want clean", l.code, l.stdout)
	}
	// Linked on both sides, the same pair is refused and nothing changes.
	before := snapshot(t, root)
	again := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", narrowScope)
	if again.code != 1 || !strings.Contains(again.stderr, "record 1 is already narrowed by record 2") || strings.Contains(again.stderr, "complete") {
		t.Errorf("exit %d, stderr:\n%s; want exit 1 and no promise of a repair", again.code, again.stderr)
	}
	if !reflect.DeepEqual(snapshot(t, root), before) {
		t.Error("the refused narrow changed the store")
	}
}
