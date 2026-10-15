// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// askOpen asks "Pick a store" with the options a and b, recommending a, in a
// fresh repo, and returns the root and the record as ask left it.
func askOpen(t *testing.T) (string, model.Record) {
	t.Helper()
	root := initRoot(t)
	r := run(testEnv(root), "", "ask", "Pick a store",
		"--option", "a:Use A", "--option", "b:Use B",
		"--recommend", "a", "--reasoning", "A is proven")
	if r.code != 0 {
		t.Fatalf("ask: exit %d: %s", r.code, r.stderr)
	}
	recs := records(t, root)
	if len(recs) != 1 {
		t.Fatalf("records = %+v, want one", recs)
	}
	return root, recs[0]
}

func TestDecideOptionRecordsTheAnswer(t *testing.T) {
	root, before := askOpen(t)
	treeBefore := snapshot(t, root)
	r := run(testEnv(root), "", "decide", "1", "b", "--why", "fewer moving parts", "--by", "owner")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	if r.stdout != "1\n" {
		t.Errorf("stdout = %q, want the ID and a newline", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	want := before
	want.Status = model.StatusDecided
	want.Decision = "b"
	want.Rationale = "fewer moving parts"
	want.DecidedBy = "owner"
	want.DecidedAt = "2026-03-04"
	want.History = append(slices.Clip(before.History), model.HistoryEntry{Date: "2026-03-04", Event: model.EventDecide})
	if got := records(t, root); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("records = %+v\nwant one %+v", got, want)
	}
	treeAfter := snapshot(t, root)
	if _, ok := treeAfter["decisions/0001-pick-a-store.md"]; !ok || len(treeAfter) != len(treeBefore) {
		t.Errorf("decide did not rewrite the record in place:\nbefore %q\nafter  %q", treeBefore, treeAfter)
	}
}

func TestDecideTextRecordsAFreeTextAnswer(t *testing.T) {
	root, before := askOpen(t)
	r := run(testEnv(root), "", "decide", "1", "--text", "Use neither", "--why", "both are too slow")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	want := before
	want.Status = model.StatusDecided
	want.Decision = "Use neither"
	want.Rationale = "both are too slow"
	want.DecidedAt = "2026-03-04"
	want.History = append(slices.Clip(before.History), model.HistoryEntry{Date: "2026-03-04", Event: model.EventDecide})
	if got := records(t, root); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("records = %+v\nwant one %+v", got, want)
	}
}

func TestDecideStoresTypedTextAndTrimsTheOption(t *testing.T) {
	root, before := askOpen(t)
	r := run(testEnv(root), "", "decide", "1", " b ", "--why", "  padded reason  ")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	want := before
	want.Status = model.StatusDecided
	want.Decision = "b"
	want.Rationale = "  padded reason  "
	want.DecidedAt = "2026-03-04"
	want.History = append(slices.Clip(before.History), model.HistoryEntry{Date: "2026-03-04", Event: model.EventDecide})
	if got := records(t, root); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("records = %+v\nwant one %+v", got, want)
	}
}

func TestDecideBadInputBeatsMissingInit(t *testing.T) {
	// Bad arguments are reported before the repo is looked for, as ask does.
	root := t.TempDir()
	before := snapshot(t, root)
	r := run(testEnv(root), "", "decide", "1", "--why", "y")
	if r.code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error before the init check); stderr:\n%s", r.code, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed: %q", after)
	}
}

func TestDecideRefusesBadArgumentsAndWritesNothing(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"option is not on the record", []string{"decide", "1", "z", "--why", "x"}, `OPTION "z" is not one of the options`},
		{"option and text", []string{"decide", "1", "a", "--text", "x", "--why", "y"}, "give an OPTION or --text, not both"},
		{"neither option nor text", []string{"decide", "1", "--why", "y"}, "give an OPTION or --text"},
		{"no why", []string{"decide", "1", "a"}, "--why is required"},
		{"blank why", []string{"decide", "1", "a", "--why", "  "}, "--why is required and must not be blank"},
		{"blank text", []string{"decide", "1", "--text", " ", "--why", "y"}, "--text must not be blank"},
		{"explicit empty text", []string{"decide", "1", "--text", "", "--why", "y"}, "--text must not be blank"},
		{"blank option", []string{"decide", "1", " ", "--why", "y"}, "OPTION must not be blank"},
		{"no arguments", []string{"decide"}, "ID is required"},
		{"ID is not a number", []string{"decide", "one", "a", "--why", "y"}, `ID must be a positive integer, got "one"`},
		{"ID is zero", []string{"decide", "0", "a", "--why", "y"}, `ID must be a positive integer, got "0"`},
		{"too many arguments", []string{"decide", "1", "a", "b", "--why", "y"}, "got 3 arguments"},
		{"unknown flag", []string{"decide", "1", "a", "--nope", "--why", "y"}, "-nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := askOpen(t)
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if tt.wantErr != "" && !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantErr, r.stderr)
			}
			if !hasUsageLine(r.stderr) {
				t.Errorf("stderr has no usage line:\n%s", r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestReasonCommandsRefuseBadArgumentsAndWriteNothing(t *testing.T) {
	tests := []struct {
		name    string
		rest    []string // the arguments after the command name
		wantErr string
	}{
		{"no ID", []string{"--why", "y"}, "ID is required"},
		{"ID is not a number", []string{"one", "--why", "y"}, `ID must be a positive integer, got "one"`},
		{"ID is zero", []string{"0", "--why", "y"}, `ID must be a positive integer, got "0"`},
		{"extra argument", []string{"1", "a", "--why", "y"}, "got 2 arguments"},
		{"no why", []string{"1"}, "--why is required"},
		{"blank why", []string{"1", "--why", " "}, "--why is required and must not be blank"},
		{"unknown flag", []string{"1", "--nope", "--why", "y"}, "-nope"},
	}
	for _, cmd := range []string{"reject", "withdraw"} {
		for _, tt := range tests {
			t.Run(cmd+" "+tt.name, func(t *testing.T) {
				root, _ := askOpen(t)
				before := snapshot(t, root)
				r := run(testEnv(root), "", append([]string{cmd}, tt.rest...)...)
				if r.code != 2 {
					t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
				}
				if !strings.Contains(r.stderr, tt.wantErr) {
					t.Errorf("stderr does not contain %q:\n%s", tt.wantErr, r.stderr)
				}
				if !strings.Contains(r.stderr, "usage: adj "+cmd) {
					t.Errorf("stderr has no %s usage:\n%s", cmd, r.stderr)
				}
				if r.stdout != "" {
					t.Errorf("stdout = %q, want empty", r.stdout)
				}
				if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
					t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
				}
			})
		}
	}
}

func TestDecisionCommandsRefuseAnUnknownOrClosedRecord(t *testing.T) {
	tests := []struct {
		name    string
		first   []string // closes record 1 first; nil leaves it open
		again   []string
		wantErr string
	}{
		{name: "decide an unknown ID", again: []string{"decide", "9", "a", "--why", "x"}, wantErr: "not found"},
		{name: "reject an unknown ID", again: []string{"reject", "9", "--why", "x"}, wantErr: "not found"},
		{name: "withdraw an unknown ID", again: []string{"withdraw", "9", "--why", "x"}, wantErr: "not found"},
		{name: "decide after decide", first: []string{"decide", "1", "a", "--why", "first"},
			again: []string{"decide", "1", "b", "--why", "second"}, wantErr: "record 1 is decided; only an open record can be decided"},
		{name: "decide an unknown OPTION on a decided record", first: []string{"decide", "1", "a", "--why", "first"},
			again: []string{"decide", "1", "z", "--why", "second"}, wantErr: "record 1 is decided; only an open record can be decided"},
		{name: "reject after decide", first: []string{"decide", "1", "a", "--why", "first"},
			again: []string{"reject", "1", "--why", "second"}, wantErr: "record 1 is decided; only an open record can be rejected"},
		{name: "withdraw after decide", first: []string{"decide", "1", "a", "--why", "first"},
			again: []string{"withdraw", "1", "--why", "second"}, wantErr: "record 1 is decided; only an open record can be withdrawn"},
		{name: "decide after reject", first: []string{"reject", "1", "--why", "first"},
			again: []string{"decide", "1", "a", "--why", "second"}, wantErr: "record 1 is rejected; only an open record can be decided"},
		{name: "reject after reject", first: []string{"reject", "1", "--why", "first"},
			again: []string{"reject", "1", "--why", "second"}, wantErr: "record 1 is rejected; only an open record can be rejected"},
		{name: "withdraw after reject", first: []string{"reject", "1", "--why", "first"},
			again: []string{"withdraw", "1", "--why", "second"}, wantErr: "record 1 is rejected; only an open record can be withdrawn"},
		{name: "decide after withdraw", first: []string{"withdraw", "1", "--why", "first"},
			again: []string{"decide", "1", "a", "--why", "second"}, wantErr: "record 1 is withdrawn; only an open record can be decided"},
		{name: "reject after withdraw", first: []string{"withdraw", "1", "--why", "first"},
			again: []string{"reject", "1", "--why", "second"}, wantErr: "record 1 is withdrawn; only an open record can be rejected"},
		{name: "withdraw after withdraw", first: []string{"withdraw", "1", "--why", "first"},
			again: []string{"withdraw", "1", "--why", "second"}, wantErr: "record 1 is withdrawn; only an open record can be withdrawn"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := askOpen(t)
			if tt.first != nil {
				if r := run(testEnv(root), "", tt.first...); r.code != 0 {
					t.Fatalf("setup %q: exit %d: %s", tt.first, r.code, r.stderr)
				}
			}
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.again...)
			if r.code != 1 {
				t.Errorf("exit code = %d, want 1; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantErr, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestRejectAndWithdrawRecordTheReason(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		status model.Status
		event  model.Event
	}{
		{"reject", []string{"reject", "1", "--why", "out of scope"}, model.StatusRejected, model.EventReject},
		{"withdraw", []string{"withdraw", "1", "--why", "out of scope"}, model.StatusWithdrawn, model.EventWithdraw},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, before := askOpen(t)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			if r.stdout != "1\n" {
				t.Errorf("stdout = %q, want the ID and a newline", r.stdout)
			}
			want := before
			want.Status = tt.status
			want.Rationale = "out of scope"
			want.DecidedAt = "2026-03-04"
			want.History = append(slices.Clip(before.History), model.HistoryEntry{Date: "2026-03-04", Event: tt.event})
			if got := records(t, root); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
				t.Errorf("records = %+v\nwant one %+v", got, want)
			}
		})
	}
}

func TestDecideTextIsStoredAsTyped(t *testing.T) {
	root, before := askOpen(t)
	r := run(testEnv(root), "", "decide", "1", "--text", "  Use neither  ", "--why", "  both are too slow  ")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	want := before
	want.Status = model.StatusDecided
	want.Decision = "  Use neither  "
	want.Rationale = "  both are too slow  "
	want.DecidedAt = "2026-03-04"
	want.History = append(slices.Clip(before.History), model.HistoryEntry{Date: "2026-03-04", Event: model.EventDecide})
	if got := records(t, root); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("records = %+v\nwant one %+v", got, want)
	}
}

func TestRejectAndWithdrawStoreTheReasonAsTyped(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		status model.Status
		event  model.Event
	}{
		{"reject", []string{"reject", "1", "--why", "  out of scope  "}, model.StatusRejected, model.EventReject},
		{"withdraw", []string{"withdraw", "1", "--why", "  out of scope  "}, model.StatusWithdrawn, model.EventWithdraw},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, before := askOpen(t)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
			}
			want := before
			want.Status = tt.status
			want.Rationale = "  out of scope  "
			want.DecidedAt = "2026-03-04"
			want.History = append(slices.Clip(before.History), model.HistoryEntry{Date: "2026-03-04", Event: tt.event})
			if got := records(t, root); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
				t.Errorf("records = %+v\nwant one %+v", got, want)
			}
		})
	}
}

func TestDecisionCommandsRefuseADuplicateID(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"decide", []string{"decide", "1", "a", "--why", "w"}},
		{"reject", []string{"reject", "1", "--why", "w"}},
		{"withdraw", []string{"withdraw", "1", "--why", "w"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := askOpen(t)
			// Two files now hold ID 1: the record askOpen made, under two names.
			dir := filepath.Join(root, "decisions")
			data, err := os.ReadFile(filepath.Join(dir, "0001-pick-a-store.md"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(filepath.Join(dir, "0001-pick-a-store.md")); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"0001-a.md", "0001-b.md"} {
				if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 1 {
				t.Errorf("exit code = %d, want 1; stderr:\n%s", r.code, r.stderr)
			}
			for _, want := range []string{"more than one record", "0001-a.md", "0001-b.md"} {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("stderr does not contain %q:\n%s", want, r.stderr)
				}
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestDecisionCommandsRecordDecidedByFromFlagOrEnvironment(t *testing.T) {
	commands := []struct {
		name string
		args []string
	}{
		{"decide", []string{"decide", "1", "a", "--why", "w"}},
		{"reject", []string{"reject", "1", "--why", "w"}},
		{"withdraw", []string{"withdraw", "1", "--why", "w"}},
	}
	sources := []struct {
		name string
		env  string
		by   []string
		want string
	}{
		{"flag", "", []string{"--by", "flag-name"}, "flag-name"},
		{"environment", "env-name", nil, "env-name"},
		{"flag beats environment", "env-name", []string{"--by", "flag-name"}, "flag-name"},
		{"neither", "", nil, ""},
	}
	for _, cmd := range commands {
		for _, src := range sources {
			t.Run(cmd.name+" "+src.name, func(t *testing.T) {
				root, _ := askOpen(t)
				env := testEnv(root)
				env.Getenv = func(k string) string {
					if k == "ADJ_DECIDED_BY" {
						return src.env
					}
					return ""
				}
				r := run(env, "", append(slices.Clone(cmd.args), src.by...)...)
				if r.code != 0 {
					t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
				}
				if recs := records(t, root); len(recs) != 1 || recs[0].DecidedBy != src.want {
					t.Errorf("records = %+v, want decided_by %q", recs, src.want)
				}
			})
		}
	}
}

func TestDecisionCommandsNeedAnInitializedRepo(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"decide", []string{"decide", "1", "a", "--why", "w"}},
		{"reject", []string{"reject", "1", "--why", "w"}},
		{"withdraw", []string{"withdraw", "1", "--why", "w"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 1 || !strings.Contains(r.stderr, "adj init") {
				t.Errorf("exit code = %d, stderr = %q; want 1 and a hint to run adj init", r.code, r.stderr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("tree changed: %q", after)
			}
		})
	}
}

func TestDecisionCommandsRootFlagNamesTheRepo(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"decide", []string{"decide", "1", "a", "--why", "w"}},
		{"reject", []string{"reject", "1", "--why", "w"}},
		{"withdraw", []string{"withdraw", "1", "--why", "w"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := askOpen(t)
			other := t.TempDir() // the injected default; the flag must win
			r := run(testEnv(other), "", append(slices.Clone(tt.args), "--root", root)...)
			if r.code != 0 {
				t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
			}
			if recs := records(t, root); len(recs) != 1 || recs[0].Status == model.StatusOpen {
				t.Errorf("records = %+v, want the one record answered", recs)
			}
			if got := snapshot(t, other); len(got) != 0 {
				t.Errorf("injected root changed: %q", got)
			}
		})
	}
}

func TestDecisionCommandsHelp(t *testing.T) {
	for _, name := range []string{"decide", "reject", "withdraw"} {
		t.Run(name, func(t *testing.T) {
			root, _ := askOpen(t)
			before := snapshot(t, root)
			r := run(testEnv(root), "", name, "--help")
			if r.code != 0 {
				t.Errorf("exit code = %d, want 0", r.code)
			}
			if !strings.Contains(r.stdout, "usage: adj "+name) {
				t.Errorf("stdout = %q, want the %s usage", r.stdout, name)
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("help changed the tree")
			}
		})
	}
}

func TestUsageListsDecisionCommands(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	for _, want := range []string{"adj decide", "adj reject", "adj withdraw"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("usage does not mention %q:\n%s", want, r.stdout)
		}
	}
}
