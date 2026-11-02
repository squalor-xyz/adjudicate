// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/store"
)

const (
	provBy  = "claude"
	provVia = "interactive question, session abc"
)

// seedProvenance asks two questions with option a and decides both with
// recorded_by and via.
func seedProvenance(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	for _, args := range [][]string{
		{"ask", "Old store", "--option", "a:Use A"},
		{"ask", "New store", "--option", "a:Use A"},
		{"decide", "1", "a", "--why", "w", "--by", "owner", "--recorded-by", provBy, "--via", provVia},
		{"decide", "2", "a", "--why", "w", "--by", "owner", "--recorded-by", provBy, "--via", provVia},
	} {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	return root
}

// amend, supersede, narrow and renumber change other things and leave recorded_by and
// via as they were (S52).
func TestOtherCommandsLeaveRecordedByAndViaAlone(t *testing.T) {
	check := func(t *testing.T, root string, ids ...int) {
		t.Helper()
		for _, id := range ids {
			rec := recordByID(t, root, id)
			if rec.RecordedBy != provBy || rec.Via != provVia {
				t.Errorf("record %d: recorded_by = %q, via = %q, want %q and %q", id, rec.RecordedBy, rec.Via, provBy, provVia)
			}
		}
	}
	t.Run("amend", func(t *testing.T) {
		root := seedProvenance(t)
		if r := run(testEnv(root), "", "amend", "1", "--title", "Older store", "--rationale", "better why"); r.code != 0 {
			t.Fatalf("amend: %+v", r)
		}
		if got := recordByID(t, root, 1).Title; got != "Older store" {
			t.Fatalf("title = %q, the amend did not run", got)
		}
		check(t, root, 1, 2)
	})
	t.Run("supersede", func(t *testing.T) {
		root := seedProvenance(t)
		if r := run(testEnv(root), "", "supersede", "1", "--by", "2"); r.code != 0 {
			t.Fatalf("supersede: %+v", r)
		}
		if got := recordByID(t, root, 1).SupersededBy; got != 2 {
			t.Fatalf("superseded_by = %d, the supersede did not run", got)
		}
		check(t, root, 1, 2)
	})
	t.Run("narrow", func(t *testing.T) {
		root := seedProvenance(t)
		if r := run(testEnv(root), "", "narrow", "1", "--by", "2", "--scope", "only for tests"); r.code != 0 {
			t.Fatalf("narrow: %+v", r)
		}
		if got := recordByID(t, root, 1).NarrowedBy; len(got) != 1 {
			t.Fatalf("narrowed_by = %+v, the narrow did not run", got)
		}
		check(t, root, 1, 2)
	})
	t.Run("renumber", func(t *testing.T) {
		root := seedProvenance(t)
		dir := filepath.Join(root, "decisions")
		data, err := os.ReadFile(filepath.Join(dir, "0001-old-store.md"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "0001-old-store-copy.md"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		if r := run(testEnv(root), "", "renumber"); r.code != 0 || r.stdout == "" {
			t.Fatalf("renumber: %+v, want a move", r)
		}
		check(t, root, 1, 2, 3)
	})
}

// The -i walk writes the answer as decide does and neither adds nor changes
// recorded_by and via: a record that has them keeps them, one that has none
// gets none (S52).
func TestInteractiveWalkLeavesRecordedByAndViaAlone(t *testing.T) {
	root := interactiveRoot(t)
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	first.RecordedBy, first.Via = provBy, provVia
	if err := st.Update(first); err != nil {
		t.Fatal(err)
	}
	r := run(testEnv(root), "d\na\nbecause\nd\na\nbecause\n", "inbox", "-i", "--by", "Owner")
	if r.code != 0 || !strings.Contains(r.stdout, "2 decided") {
		t.Fatalf("walk: %+v", r)
	}
	if got := recordByID(t, root, 1); got.RecordedBy != provBy || got.Via != provVia || got.DecidedBy != "Owner" {
		t.Errorf("record 1: recorded_by = %q, via = %q, decided_by = %q; want %q, %q, Owner", got.RecordedBy, got.Via, got.DecidedBy, provBy, provVia)
	}
	if got := recordByID(t, root, 2); got.RecordedBy != "" || got.Via != "" {
		t.Errorf("record 2: recorded_by = %q, via = %q, want both empty", got.RecordedBy, got.Via)
	}
}

// adj ai is the agents' snippet, and agents never run decide (C137): it does
// not name the owner's flags (S52).
func TestAISnippetDoesNotNameRecordedByOrVia(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "ai")
	if r.code != 0 {
		t.Fatalf("ai: %+v", r)
	}
	for _, flag := range []string{"--recorded-by", "--via"} {
		if strings.Contains(r.stdout, flag) {
			t.Errorf("adj ai names %s", flag)
		}
	}
}
