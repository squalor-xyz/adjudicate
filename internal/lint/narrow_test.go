// SPDX-License-Identifier: GPL-3.0-or-later

package lint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/lint"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// narrowStore writes the given record files (name -> frontmatter JSON) into a
// temp store and returns it with its root.
func narrowStore(t *testing.T, files map[string]string) (*store.Store, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, front := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("---\n"+front+"\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return st, root
}

const decidedHead = `"status":"decided","decided_by":"owner","created":"2026-10-01","decided_at":"2026-10-02","decision":"yes","rationale":"because"`

// A link to a file that does not parse is parse's finding, and a link to an ID
// that two files share is in order if either file lists it back: neither is
// reported as narrow-asymmetric or narrow-state.
func TestNarrowChecksSkipWhatTheyCannotJudge(t *testing.T) {
	st, root := narrowStore(t, map[string]string{
		"0001-first.md":    `{"id":1,"title":"First","key":"a",` + decidedHead + `,"narrowed_by":[{"id":2,"scope":"s"},{"id":3,"scope":"s"}]}`,
		"0002-broken.md":   `{"id":2,`,
		"0003-twin-a.md":   `{"id":3,"title":"Twin a","key":"b",` + decidedHead + `}`,
		"0003-twin-b.md":   `{"id":3,"title":"Twin b","key":"c",` + decidedHead + `,"narrows":[1]}`,
		"0004-narrower.md": `{"id":4,"title":"Narrower","key":"d",` + decidedHead + `,"narrows":[1]}`,
	})
	got, err := lint.Run(st, root)
	if err != nil {
		t.Fatal(err)
	}
	var checks []string
	for _, f := range got {
		checks = append(checks, f.File+" "+f.Check)
	}
	want := []string{
		// 1 is listed back by 3 (twin b) and narrowed_by 2 cannot be judged;
		// 4 lists 1 but 1 does not list 4.
		"decisions/0002-broken.md parse",
		"decisions/0003-twin-a.md duplicate-id",
		"decisions/0003-twin-b.md duplicate-id",
		"decisions/0004-narrower.md narrow-asymmetric",
	}
	if len(checks) != len(want) {
		t.Fatalf("findings =\n%swant %v", dump(got), want)
	}
	for i := range want {
		if checks[i] != want[i] {
			t.Errorf("finding %d = %q, want %q\n%s", i, checks[i], want[i], dump(got))
		}
	}
}

// A twin whose files are all in a bad state is faulted; one good file is
// enough for the link.
func TestNarrowStateNeedsEveryFileOfAnIDToBeBad(t *testing.T) {
	st, root := narrowStore(t, map[string]string{
		"0001-first.md":  `{"id":1,"title":"First","key":"a",` + decidedHead + `,"narrowed_by":[{"id":2,"scope":"s"}]}`,
		"0002-open.md":   `{"id":2,"title":"Open one","status":"open","created":"2026-10-01","narrows":[1]}`,
		"0002-other.md":  `{"id":2,"title":"Other","status":"rejected","created":"2026-10-01","decided_at":"2026-10-02","rationale":"no","narrows":[1]}`,
		"0003-third.md":  `{"id":3,"title":"Third","key":"c",` + decidedHead + `,"narrowed_by":[{"id":4,"scope":"s"}]}`,
		"0004-fourth.md": `{"id":4,"title":"Fourth","key":"d",` + decidedHead + `,"narrows":[3]}`,
		"0004-fifth.md":  `{"id":4,"title":"Fifth","status":"open","created":"2026-10-01"}`,
	})
	got, err := lint.Run(st, root)
	if err != nil {
		t.Fatal(err)
	}
	var state []string
	for _, f := range got {
		if f.Check == lint.CheckNarrowState {
			state = append(state, f.File+": "+f.Message)
		}
	}
	want := []string{
		"decisions/0001-first.md: narrowed_by 2 names a record that is open",
		"decisions/0002-open.md: narrows is set but the status is open",
		"decisions/0002-other.md: narrows is set but the status is rejected",
	}
	if len(state) != len(want) {
		t.Fatalf("narrow-state findings = %q, want %q\n%s", state, want, dump(got))
	}
	for i := range want {
		if state[i] != want[i] {
			t.Errorf("finding %d = %q, want %q", i, state[i], want[i])
		}
	}
}
