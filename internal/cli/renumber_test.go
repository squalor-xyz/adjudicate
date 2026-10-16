// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// storeFixture returns a temp copy of the store fixture internal/store/
// testdata/<name>: renumber writes and renames files, so no test runs on a
// fixture itself.
func storeFixture(t *testing.T, name string) string {
	t.Helper()
	return copyTree(t, filepath.Join("..", "store", "testdata", name))
}

// fileNames lists the entries of the store directory of root, without the lock
// file (C114).
func fileNames(t *testing.T, root string) []string {
	t.Helper()
	des, err := os.ReadDir(filepath.Join(root, "decisions"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, de := range des {
		if de.Name() != ".adj.lock" {
			out = append(out, de.Name())
		}
	}
	return out
}

const collisionLint = `decisions/0002-alpha.md: record 2: error: duplicate-id: more than one file has id 2: 0002-alpha.md, 0002-beta.md
decisions/0002-beta.md: record 2: error: duplicate-id: more than one file has id 2: 0002-alpha.md, 0002-beta.md
decisions/0003-aaa-late.md: record 3: error: duplicate-id: more than one file has id 3: 0003-aaa-late.md, 0003-mmm-mid.md, 0003-zzz-early.md
decisions/0003-mmm-mid.md: record 3: error: duplicate-id: more than one file has id 3: 0003-aaa-late.md, 0003-mmm-mid.md, 0003-zzz-early.md
decisions/0003-zzz-early.md: record 3: error: duplicate-id: more than one file has id 3: 0003-aaa-late.md, 0003-mmm-mid.md, 0003-zzz-early.md
5 errors, 0 warnings
`

// The whole flow (S15): lint finds the collision and names the files,
// renumber gives the later records new IDs and says what it did, and lint is
// clean after it.
func TestRenumberFixesACollisionAndLintIsCleanAfter(t *testing.T) {
	root := storeFixture(t, "collision")
	env := envOn(t, root, "2026-11-02")

	r := run(env, "", "lint")
	if r.code != 1 || r.stdout != collisionLint || r.stderr != "" {
		t.Fatalf("lint before: exit %d, stdout\n%s\nstderr %q; want exit 1 and\n%s", r.code, r.stdout, r.stderr, collisionLint)
	}

	r = run(env, "", "renumber")
	wantOut := "0002-beta.md -> 0004-beta.md (id 2 -> 4)\n" +
		"0003-mmm-mid.md -> 0005-mmm-mid.md (id 3 -> 5)\n" +
		"0003-aaa-late.md -> 0006-aaa-late.md (id 3 -> 6)\n"
	if r.code != 0 || r.stdout != wantOut || r.stderr != "" {
		t.Fatalf("renumber: exit %d, stdout\n%s\nstderr %q; want exit 0 and\n%s", r.code, r.stdout, r.stderr, wantOut)
	}
	wantFiles := []string{"0001-intro.md", "0002-alpha.md", "0003-zzz-early.md", "0004-beta.md", "0005-mmm-mid.md", "0006-aaa-late.md"}
	if got := fileNames(t, root); !reflect.DeepEqual(got, wantFiles) {
		t.Errorf("files = %v, want %v", got, wantFiles)
	}

	if r = run(env, "", "lint"); r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("lint after: exit %d, stdout %q, stderr %q; want a clean store", r.code, r.stdout, r.stderr)
	}

	// The moved record reads under its new ID and says where it came from.
	r = run(env, "", "show", "4", "--json")
	if r.code != 0 {
		t.Fatalf("show 4: exit %d: %s", r.code, r.stderr)
	}
	rec := decodeObject(t, r.stdout)
	if rec["id"] != float64(4) || rec["title"] != "Beta" {
		t.Errorf("record 4 = %v", rec)
	}
	hist, _ := rec["history"].([]any)
	want := map[string]any{"date": "2026-11-02", "event": "renumber", "note": "from 2"}
	if len(hist) != 2 || !reflect.DeepEqual(hist[1], want) {
		t.Errorf("history = %v, want the second entry %v", hist, want)
	}
	// ID 2 is the first record's again, and the kept record is unchanged.
	if r = run(env, "", "show", "2", "--json"); r.code != 0 || decodeObject(t, r.stdout)["title"] != "Alpha" {
		t.Errorf("show 2: exit %d, stdout %s, stderr %s", r.code, r.stdout, r.stderr)
	}
}

// No collision: nothing is printed, nothing changes, exit 0.
func TestRenumberWithoutACollisionChangesNothing(t *testing.T) {
	root := lintRoot(t, "clean")
	before := snapshot(t, root)
	r := run(envOn(t, root, "2026-11-02"), "", "renumber")
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 0 and no output", r.code, r.stdout, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Errorf("renumber changed a store with no collision:\nbefore %v\nafter  %v", before, after)
	}
}

// Links to a colliding ID are ambiguous (C147): renumber refuses, names them
// and changes nothing, until --keep-links says they may stay where they are.
func TestRenumberRefusesLinksUntilKeepLinks(t *testing.T) {
	root := storeFixture(t, "collision-links")
	env := envOn(t, root, "2026-11-02")
	before := snapshot(t, root)

	r := run(env, "", "renumber")
	if r.code != 1 || r.stdout != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and no stdout", r.code, r.stdout)
	}
	for _, want := range []string{"0002-replacement.md supersedes 1", "0003-gone.md superseded_by 1", "0001-store-a.md", "--keep-links"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, r.stderr)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("a refused renumber changed the store")
	}

	r = run(env, "", "renumber", "--keep-links")
	if r.code != 0 || r.stdout != "0001-store-b.md -> 0004-store-b.md (id 1 -> 4)\n" {
		t.Fatalf("--keep-links: exit %d, stdout %q, stderr %s", r.code, r.stdout, r.stderr)
	}
	// The links that stayed are listed on stderr for the owner to check.
	for _, want := range []string{"0002-replacement.md supersedes 1", "0003-gone.md superseded_by 1", "0001-store-a.md"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("--keep-links: stderr does not contain %q:\n%s", want, r.stderr)
		}
	}
	if r = run(env, "", "lint"); r.code != 0 || r.stdout != "" {
		t.Errorf("lint after: exit %d, stdout %q; want a clean store", r.code, r.stdout)
	}
	// The links are as they were: record 1 is still the kept one.
	if r = run(env, "", "show", "2", "--json"); decodeObject(t, r.stdout)["supersedes"].([]any)[0] != float64(1) {
		t.Errorf("record 2 lost its link: %s", r.stdout)
	}
}

// A store that cannot be read is a failed run (exit 1) that names the file.
func TestRenumberFailsOnAnUnreadableFile(t *testing.T) {
	root := storeFixture(t, "collision")
	if err := os.WriteFile(filepath.Join(root, "decisions", "0009-broken.md"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	r := run(envOn(t, root, "2026-11-02"), "", "renumber")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, "0009-broken.md") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 naming 0009-broken.md", r.code, r.stdout, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("a failed renumber changed the store")
	}
}

func TestRenumberHelpAndBadArguments(t *testing.T) {
	env := testEnv(storeFixture(t, "collision"))
	for _, flag := range []string{"--help", "-h"} {
		r := run(env, "", "renumber", flag)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj renumber") || r.stderr != "" {
			t.Errorf("renumber %s: exit %d, stdout %q, stderr %q; want exit 0 and usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
	// The usage says which record keeps an ID, how links are handled and what
	// is printed, so the rule is where the owner looks.
	help := run(env, "", "renumber", "--help").stdout
	for _, want := range []string{"created", "file name", "--keep-links", "--root", "next free ID", "history"} {
		if !strings.Contains(help, want) {
			t.Errorf("renumber usage does not mention %q:\n%s", want, help)
		}
	}
	for _, flag := range []string{"  --keep-links ", "  --root DIR "} {
		if !strings.Contains(help, "\n"+flag) {
			t.Errorf("renumber usage does not describe the flag on a line of its own starting %q:\n%s", flag, help)
		}
	}
	// The leftover rule can be wrong about which file is the copy, so the
	// owner is told to check both files before deleting one.
	_, leftover, _ := strings.Cut(help, "renumber-leftover")
	leftover = strings.Join(strings.Fields(leftover), " ") // the usage wraps
	if check, del := strings.Index(leftover, "check both"), strings.Index(leftover, "delete"); check < 0 || del < 0 || check > del {
		t.Errorf("renumber usage must say \"check both\" before \"delete\" where it names renumber-leftover:\n%s", leftover)
	}
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"positional argument", []string{"renumber", "3"}, "no arguments"},
		{"unknown flag", []string{"renumber", "--nope"}, "nope"},
		{"json is not a flag", []string{"renumber", "--json"}, "json"},
	}
	before := snapshot(t, env.Root)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(env, "", tt.args...)
			if r.code != 2 || r.stdout != "" {
				t.Errorf("exit %d, stdout %q; want exit 2 and no stdout", r.code, r.stdout)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj renumber") {
				t.Errorf("stderr should contain %q and the renumber usage:\n%s", tt.wantErr, r.stderr)
			}
		})
	}
	if after := snapshot(t, env.Root); !reflect.DeepEqual(before, after) {
		t.Error("a usage error changed the store")
	}
}

func TestRenumberWithoutARepo(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "renumber")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, "adj init") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1, no stdout and a hint to run adj init", r.code, r.stdout, r.stderr)
	}
}

func TestRenumberListedInTheMainUsage(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(r.stdout, "adj renumber ") {
		t.Errorf("main usage does not list adj renumber:\n%s", r.stdout)
	}
}

// docs/json.md names every history event, the new one included.
func TestDocsNameEveryHistoryEvent(t *testing.T) {
	doc := docsText(t)
	_, rest, ok := strings.Cut(doc, "A history entry's `event` is one of")
	if !ok {
		t.Fatal("docs/json.md does not say which events a history entry has")
	}
	// The sentence that lists them, before the paragraph goes on to say what
	// each command writes.
	para, _, _ := strings.Cut(rest, "`adj supersede`")
	for _, ev := range []string{"ask", "decide", "reject", "withdraw", "supersede", "renumber"} {
		if !strings.Contains(para, "`"+ev+"`") {
			t.Errorf("docs/json.md does not name the event `%s` in:\n%s", ev, para)
		}
	}
}

// If a move fails after others were done, the moves done are printed before
// the error, so the owner knows what changed; the exit code is 1.
func TestRenumberPrintsTheMovesDoneBeforeAFailure(t *testing.T) {
	root := storeFixture(t, "collision")
	for id := 5; id < 15; id++ {
		if err := os.Mkdir(filepath.Join(root, "decisions", fmt.Sprintf("%04d-mmm-mid.md", id)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := run(envOn(t, root, "2026-11-02"), "", "renumber")
	if r.code != 1 || r.stdout != "0002-beta.md -> 0004-beta.md (id 2 -> 4)\n" || !strings.Contains(r.stderr, "0003-mmm-mid.md") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1, the first move on stdout and the failure naming 0003-mmm-mid.md", r.code, r.stdout, r.stderr)
	}
}

const interruptedLint = `decisions/0002-alpha.md: record 2: error: duplicate-id: more than one file has id 2: 0002-alpha.md, 0002-beta.md
decisions/0002-beta.md: record 2: error: duplicate-id: more than one file has id 2: 0002-alpha.md, 0002-beta.md
decisions/0002-beta.md: record 2: error: renumber-leftover: 0004-beta.md has this record renumbered from id 2: this file is a stale copy left by an interrupted renumber; check both, delete it
3 errors, 0 warnings
`

// A renumber that died after writing the new file leaves the old one: lint
// reports it, renumber refuses to go on (and says which file to delete), and
// after the delete the store is clean.
func TestRenumberAfterAnInterruptedRenumber(t *testing.T) {
	root := storeFixture(t, "collision-interrupted")
	env := envOn(t, root, "2026-11-02")

	if r := run(env, "", "lint"); r.code != 1 || r.stdout != interruptedLint {
		t.Fatalf("lint: exit %d, stdout\n%s\nwant exit 1 and\n%s", r.code, r.stdout, interruptedLint)
	}
	before := snapshot(t, root)
	r := run(env, "", "renumber")
	if r.code != 1 || r.stdout != "" {
		t.Errorf("renumber: exit %d, stdout %q; want exit 1 and no stdout", r.code, r.stdout)
	}
	for _, want := range []string{"0002-beta.md", "0004-beta.md", "delete"} {
		if !strings.Contains(r.stderr, want) {
			t.Errorf("stderr does not contain %q:\n%s", want, r.stderr)
		}
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("a refused renumber changed the store")
	}

	if err := os.Remove(filepath.Join(root, "decisions", "0002-beta.md")); err != nil {
		t.Fatal(err)
	}
	if r := run(env, "", "lint"); r.code != 0 || r.stdout != "" {
		t.Errorf("lint after the delete: exit %d, stdout %q; want a clean store", r.code, r.stdout)
	}
	if r := run(env, "", "renumber"); r.code != 0 || r.stdout != "" {
		t.Errorf("renumber after the delete: exit %d, stdout %q; want nothing to do", r.code, r.stdout)
	}
}
