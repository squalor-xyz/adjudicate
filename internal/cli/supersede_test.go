// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// seedSupersede makes a repo with five records, all asked and answered on the
// fixed clock:
//
//	1 decided    "Old store"
//	2 decided    "New store"
//	3 decided    "Newer store"
//	4 open       "Undecided store"
//	5 rejected   "Rejected store"
func seedSupersede(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Old store", "--option", "a:Use A", "--key", "storage.engine", "--tag", "infra"},
		{"ask", "New store", "--option", "b:Use B", "--key", "storage.engine", "--tag", "infra"},
		{"ask", "Newer store", "--option", "c:Use C", "--key", "storage.engine"},
		{"ask", "Undecided store"},
		{"ask", "Rejected store"},
		{"decide", "1", "a", "--why", "proven", "--by", "owner"},
		{"decide", "2", "b", "--why", "faster", "--by", "owner"},
		{"decide", "3", "c", "--why", "newest"},
		{"reject", "5", "--why", "not needed"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	return root
}

func recordByID(t *testing.T, root string, id int) model.Record {
	t.Helper()
	for _, r := range records(t, root) {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no record %d in %+v", id, records(t, root))
	return model.Record{}
}

func TestSupersedeLinksBothRecords(t *testing.T) {
	root := seedSupersede(t)
	oldBefore, newBefore := recordByID(t, root, 1), recordByID(t, root, 2)
	treeBefore := snapshot(t, root)

	r := run(envOn(t, root, "2026-05-06"), "", "supersede", "1", "--by", "2")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	if r.stdout != "1\n" {
		t.Errorf("stdout = %q, want the superseded ID and a newline", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}

	wantOld := oldBefore
	wantOld.Status = model.StatusSuperseded
	wantOld.SupersededBy = 2
	wantOld.History = append(slices.Clip(oldBefore.History), model.HistoryEntry{Date: "2026-05-06", Event: model.EventSupersede, Note: "by 2"})
	if got := recordByID(t, root, 1); !reflect.DeepEqual(got, wantOld) {
		t.Errorf("old record:\n got %+v\nwant %+v", got, wantOld)
	}
	wantNew := newBefore
	wantNew.Supersedes = []int{1}
	wantNew.History = append(slices.Clip(newBefore.History), model.HistoryEntry{Date: "2026-05-06", Event: model.EventSupersede, Note: "supersedes 1"})
	if got := recordByID(t, root, 2); !reflect.DeepEqual(got, wantNew) {
		t.Errorf("new record:\n got %+v\nwant %+v", got, wantNew)
	}
	if got := recordByID(t, root, 3); got.Status != model.StatusDecided || len(got.Supersedes) != 0 {
		t.Errorf("an unrelated record changed: %+v", got)
	}

	// The files stay where they were: no record is renamed, none is added.
	treeAfter := snapshot(t, root)
	if len(treeAfter) != len(treeBefore) {
		t.Errorf("files before %q\nfiles after  %q", treeBefore, treeAfter)
	}
	for name := range treeBefore {
		if _, ok := treeAfter[name]; !ok {
			t.Errorf("%s is gone after supersede", name)
		}
	}
}

func TestSupersedeKeepsTheOldRecordReadableAndOutOfContext(t *testing.T) {
	root := seedSupersede(t)
	if got := contextIDs(t, root); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("context before = %v, want [1 2 3]", got)
	}
	if r := run(testEnv(root), "", "supersede", "1", "--by", "2"); r.code != 0 {
		t.Fatalf("supersede: exit %d: %s", r.code, r.stderr)
	}
	if got := contextIDs(t, root); !slices.Equal(got, []int{2, 3}) {
		t.Errorf("context after = %v, want [2 3]: the superseded record leaves it", got)
	}

	show := run(testEnv(root), "", "show", "1")
	if show.code != 0 {
		t.Fatalf("show 1: exit %d: %s", show.code, show.stderr)
	}
	for _, want := range []string{"Old store", "superseded", "Superseded by", "2", "ask", "decide", "supersede", "by 2", "Use A"} {
		if !strings.Contains(show.stdout, want) {
			t.Errorf("show 1 does not print %q:\n%s", want, show.stdout)
		}
	}
	showNew := run(testEnv(root), "", "show", "2")
	if !strings.Contains(showNew.stdout, "Supersedes") || !strings.Contains(showNew.stdout, "supersedes 1") {
		t.Errorf("show 2 does not print the link and its history entry:\n%s", showNew.stdout)
	}

	// check leads from the superseded record to its replacement (C113).
	chk := run(testEnv(root), "", "check", "Old store")
	if first, _, _ := strings.Cut(chk.stdout, "\n"); chk.code != 0 || first != "decided" || !strings.Contains(chk.stdout, "Superseded by") {
		t.Errorf("check \"Old store\": exit %d, output:\n%s\nwant decided, with the replacement", chk.code, chk.stdout)
	}
}

func TestSupersedeOneRecordCanReplaceSeveral(t *testing.T) {
	root := seedSupersede(t)
	for _, old := range []string{"1", "2"} {
		if r := run(testEnv(root), "", "supersede", old, "--by", "3"); r.code != 0 {
			t.Fatalf("supersede %s: exit %d: %s", old, r.code, r.stderr)
		}
	}
	got := recordByID(t, root, 3)
	if !slices.Equal(got.Supersedes, []int{1, 2}) {
		t.Errorf("supersedes = %v, want [1 2]", got.Supersedes)
	}
	if len(got.History) != 4 { // ask, decide, supersede 1, supersede 2
		t.Errorf("history = %+v, want 4 entries", got.History)
	}
	if got := contextIDs(t, root); !slices.Equal(got, []int{3}) {
		t.Errorf("context = %v, want [3]", got)
	}
}

func TestSupersedeFlagPositionAndRoot(t *testing.T) {
	root := seedSupersede(t)
	// --by before OLD, and --root instead of the injected root.
	env := testEnv("")
	if r := run(env, "", "supersede", "--root", root, "--by", "2", "1"); r.code != 0 || r.stdout != "1\n" {
		t.Fatalf("exit %d, stdout %q, stderr %q; want exit 0 and \"1\\n\"", r.code, r.stdout, r.stderr)
	}
	if got := recordByID(t, root, 1); got.Status != model.StatusSuperseded || got.SupersededBy != 2 {
		t.Errorf("record 1 = %+v, want superseded by 2", got)
	}
}

func TestSupersedeRefusesWhatItCannotDoAndWritesNothing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    int
		wantErr string
	}{
		{"old is open", []string{"supersede", "4", "--by", "2"}, 1, "record 4 is open; only a decided record can be superseded"},
		{"old is rejected", []string{"supersede", "5", "--by", "2"}, 1, "record 5 is rejected; only a decided record can be superseded"},
		{"new is open", []string{"supersede", "1", "--by", "4"}, 1, "record 4 is open; only a decided record can supersede another"},
		{"new is rejected", []string{"supersede", "1", "--by", "5"}, 1, "record 5 is rejected; only a decided record can supersede another"},
		{"both not decided: old is reported first", []string{"supersede", "4", "--by", "5"}, 1, "record 4 is open; only a decided record can be superseded"},
		{"old equals new", []string{"supersede", "1", "--by", "1"}, 2, "OLD and --by must be different records"},
		{"unknown old", []string{"supersede", "99", "--by", "2"}, 1, "record not found"},
		{"unknown new", []string{"supersede", "1", "--by", "99"}, 1, "record not found"},
		{"no --by", []string{"supersede", "1"}, 2, "--by is required"},
		{"blank --by", []string{"supersede", "1", "--by", " "}, 2, "--by must be a positive integer"},
		{"--by is not a number", []string{"supersede", "1", "--by", "two"}, 2, "--by must be a positive integer"},
		{"--by is zero", []string{"supersede", "1", "--by", "0"}, 2, "--by must be a positive integer"},
		{"--by is negative", []string{"supersede", "1", "--by", "-2"}, 2, "--by must be a positive integer"},
		{"no OLD", []string{"supersede", "--by", "2"}, 2, "ID is required"},
		{"OLD is not a number", []string{"supersede", "x", "--by", "2"}, 2, "ID must be a positive integer"},
		{"extra argument", []string{"supersede", "1", "2", "--by", "2"}, 2, "want one ID"},
		{"unknown flag", []string{"supersede", "1", "--by", "2", "--why", "x"}, 2, "why"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := seedSupersede(t)
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
			if tt.want == 2 && !strings.Contains(r.stderr, "usage: adj supersede") {
				t.Errorf("a usage error should print the usage:\n%s", r.stderr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("the store changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestSupersedeTwiceFailsTheSecondTime(t *testing.T) {
	root := seedSupersede(t)
	if r := run(testEnv(root), "", "supersede", "1", "--by", "2"); r.code != 0 {
		t.Fatalf("first supersede: exit %d: %s", r.code, r.stderr)
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "", "supersede", "1", "--by", "3")
	if r.code != 1 || !strings.Contains(r.stderr, "record 1 is superseded; only a decided record can be superseded") {
		t.Errorf("exit %d, stderr:\n%s; want exit 1 naming the status", r.code, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("the store changed on the refused second supersede")
	}
}

func TestSupersedeBadArgumentsBeatMissingInit(t *testing.T) {
	root := t.TempDir() // no .adjudicate.json
	before := snapshot(t, root)
	if r := run(testEnv(root), "", "supersede", "1", "--by", "1"); r.code != 2 {
		t.Errorf("equal IDs without init: exit %d, want 2 (usage error before the init check); stderr:\n%s", r.code, r.stderr)
	}
	r := run(testEnv(root), "", "supersede", "1", "--by", "2")
	if r.code != 1 || !strings.Contains(r.stderr, "adj init") {
		t.Errorf("no repo: exit %d, stderr:\n%s; want exit 1 telling the user to run adj init", r.code, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed: %q", after)
	}
}

func TestSupersedeHelpAndMainUsage(t *testing.T) {
	env := testEnv(t.TempDir())
	for _, flag := range []string{"--help", "-h"} {
		r := run(env, "", "supersede", flag)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj supersede OLD --by NEW") || r.stderr != "" {
			t.Errorf("supersede %s: exit %d, stdout %q, stderr %q; want exit 0 and the usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
	if main := run(env, "", "--help").stdout; !strings.Contains(main, "adj supersede") {
		t.Errorf("main usage does not list adj supersede:\n%s", main)
	}
}
