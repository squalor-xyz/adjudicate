// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// fakeGit stands in for the git binary: the output of `rev-parse
// --show-toplevel` and of `worktree list --porcelain`, or an error for both.
// It keeps the arguments of every call.
type fakeGit struct {
	top       string
	worktrees string
	err       error
	calls     [][]string
}

func (f *fakeGit) run(ctx context.Context, root string, args ...string) (string, error) {
	f.calls = append(f.calls, slices.Clone(args))
	if f.err != nil {
		return "", f.err
	}
	switch {
	case len(args) > 0 && args[0] == "rev-parse":
		return f.top + "\n", nil
	case len(args) > 1 && args[0] == "worktree" && args[1] == "list":
		return f.worktrees, nil
	}
	return "", fmt.Errorf("fakeGit: unexpected arguments %v", args)
}

// listed reports whether the worktree list was asked for.
func (f *fakeGit) listed() bool {
	for _, c := range f.calls {
		if len(c) > 0 && c[0] == "worktree" {
			return true
		}
	}
	return false
}

// gitPath writes a path the way git prints it: with forward slashes.
func gitPath(p string) string { return filepath.ToSlash(p) }

// porcelainEntry is one entry of `git worktree list --porcelain`, with a
// blank line after it. extra lines (bare, prunable ...) follow the branch line.
func porcelainEntry(path string, extra ...string) string {
	lines := append([]string{"worktree " + gitPath(path), "HEAD 0123456789abcdef0123456789abcdef01234567", "branch refs/heads/x"}, extra...)
	return strings.Join(lines, "\n") + "\n\n"
}

// seedIDs writes empty-ish records with the IDs into dir.
func seedIDs(t *testing.T, dir string, ids ...int) {
	t.Helper()
	for _, id := range ids {
		seed(t, dir, fmt.Sprintf("%04d-q%d.md", id, id), question(id, fmt.Sprintf("Q%d", id)))
	}
}

// siblingWith makes a sibling worktree directory whose default store holds the
// IDs, and returns its path.
func siblingWith(t *testing.T, ids ...int) string {
	t.Helper()
	dir := t.TempDir()
	seedIDs(t, filepath.Join(dir, "decisions"), ids...)
	return dir
}

func upTo(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func TestParseWorktrees(t *testing.T) {
	const head = "HEAD 0123456789abcdef0123456789abcdef01234567\n"
	tests := []struct {
		name string
		out  string
		want []worktreeEntry
	}{
		{"empty output", "", nil},
		{"only blank lines", "\n\n", nil},
		{
			"main and two linked worktrees",
			"worktree /repo\n" + head + "branch refs/heads/main\n\n" +
				"worktree /repo-wt/a\n" + head + "branch refs/heads/a\n\n" +
				"worktree /repo-wt/b\n" + head + "branch refs/heads/b\n\n",
			[]worktreeEntry{{Path: "/repo"}, {Path: "/repo-wt/a"}, {Path: "/repo-wt/b"}},
		},
		{
			"a bare entry",
			"worktree /srv/repo.git\nbare\n\nworktree /srv/wt\n" + head + "branch refs/heads/a\n\n",
			[]worktreeEntry{{Path: "/srv/repo.git", Bare: true}, {Path: "/srv/wt"}},
		},
		{
			"a prunable entry, with its reason",
			"worktree /repo\n" + head + "branch refs/heads/main\n\n" +
				"worktree /gone\n" + head + "detached\nprunable gitdir file points to non-existent location\n\n",
			[]worktreeEntry{{Path: "/repo"}, {Path: "/gone", Prunable: true}},
		},
		{
			"a prunable entry without a reason",
			"worktree /gone\n" + head + "detached\nprunable\n\n",
			[]worktreeEntry{{Path: "/gone", Prunable: true}},
		},
		{
			"detached and locked entries",
			"worktree /a\n" + head + "detached\n\nworktree /b\n" + head + "branch refs/heads/b\nlocked\n\nworktree /c\n" + head + "detached\nlocked in use\n\n",
			[]worktreeEntry{{Path: "/a"}, {Path: "/b"}, {Path: "/c"}},
		},
		{
			"Windows paths with forward slashes and a drive letter",
			"worktree C:/Users/me/repo\n" + head + "branch refs/heads/main\n\nworktree D:/work trees/wt\n" + head + "branch refs/heads/a\n\n",
			[]worktreeEntry{{Path: "C:/Users/me/repo"}, {Path: "D:/work trees/wt"}},
		},
		{
			"CRLF line ends",
			"worktree C:/r\r\n" + head + "branch refs/heads/main\r\n\r\nworktree C:/w\r\n" + head + "bare\r\n\r\n",
			[]worktreeEntry{{Path: "C:/r"}, {Path: "C:/w", Bare: true}},
		},
		{
			"no trailing blank line",
			"worktree /a\n" + head + "branch refs/heads/a\n\nworktree /b\n" + head + "branch refs/heads/b",
			[]worktreeEntry{{Path: "/a"}, {Path: "/b"}},
		},
		{
			"several trailing blank lines",
			"worktree /a\n" + head + "branch refs/heads/a\n\n\n\n",
			[]worktreeEntry{{Path: "/a"}},
		},
		{
			"a path git quotes",
			"worktree \"/tmp/caf\\303\\251\"\n" + head + "branch refs/heads/a\n\n",
			[]worktreeEntry{{Path: "/tmp/café"}},
		},
		{
			"a marker word inside a path is not a marker",
			"worktree /srv/bare\n" + head + "branch refs/heads/bare\n\n",
			[]worktreeEntry{{Path: "/srv/bare"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorktrees(tt.out)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseWorktrees =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestSiblingFloor(t *testing.T) {
	t.Run("one past the highest ID in a sibling", func(t *testing.T) {
		root := t.TempDir()
		sib := siblingWith(t, upTo(7)...)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 8 {
			t.Errorf("floor = %d, want 8", got)
		}
	})

	t.Run("the largest of several siblings", func(t *testing.T) {
		root := t.TempDir()
		a, b := siblingWith(t, 1, 2), siblingWith(t, 1, 2, 3, 9)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(a) + porcelainEntry(b)}
		if got := siblingFloor(root, g.run); got != 10 {
			t.Errorf("floor = %d, want 10", got)
		}
	})

	t.Run("no sibling gives 0", func(t *testing.T) {
		root := t.TempDir()
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root)}
		if got := siblingFloor(root, g.run); got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
	})

	t.Run("siblings with no records give 0", func(t *testing.T) {
		root := t.TempDir()
		sib := t.TempDir() // no store directory at all
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
	})

	t.Run("the current worktree is left out", func(t *testing.T) {
		root := t.TempDir()
		seedIDs(t, filepath.Join(root, "decisions"), upTo(20)...)
		sib := siblingWith(t, 1, 2, 3)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 4 {
			t.Errorf("floor = %d, want 4 (the sibling's), not 21", got)
		}
	})

	t.Run("root that is not the top level gives 0 and lists nothing", func(t *testing.T) {
		top := t.TempDir()
		root := filepath.Join(top, "sub")
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		sib := siblingWith(t, upTo(7)...)
		g := &fakeGit{top: gitPath(top), worktrees: porcelainEntry(top) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
		if g.listed() {
			t.Errorf("git worktree list was run for a root that is not the top level: %v", g.calls)
		}
	})

	t.Run("a top level that is not the root's directory gives 0", func(t *testing.T) {
		root, other := t.TempDir(), t.TempDir()
		sib := siblingWith(t, upTo(7)...)
		g := &fakeGit{top: gitPath(other), worktrees: porcelainEntry(other) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
	})

	t.Run("a top level that does not exist gives 0", func(t *testing.T) {
		root := t.TempDir()
		sib := siblingWith(t, upTo(7)...)
		gone := filepath.Join(root, "gone")
		g := &fakeGit{top: gitPath(gone), worktrees: porcelainEntry(gone) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
	})

	t.Run("git failing gives 0", func(t *testing.T) {
		root := t.TempDir()
		g := &fakeGit{err: errors.New("exec: \"git\": executable file not found in $PATH")}
		if got := siblingFloor(root, g.run); got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
	})

	t.Run("the worktree list failing gives 0", func(t *testing.T) {
		root := t.TempDir()
		calls := 0
		run := func(ctx context.Context, dir string, args ...string) (string, error) {
			calls++
			if args[0] == "rev-parse" {
				return gitPath(root) + "\n", nil
			}
			return "", errors.New("exit status 128")
		}
		if got := siblingFloor(root, run); got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
		if calls != 2 {
			t.Errorf("git ran %d times, want 2", calls)
		}
	})

	t.Run("no root gives 0 and runs no git", func(t *testing.T) {
		g := &fakeGit{top: "/", worktrees: porcelainEntry("/x")}
		if got := siblingFloor("", g.run); got != 0 || len(g.calls) != 0 {
			t.Errorf("floor = %d after %v, want 0 and no git", got, g.calls)
		}
	})

	t.Run("the git calls are bounded and aimed at the root", func(t *testing.T) {
		root := t.TempDir()
		var seen [][]string
		run := func(ctx context.Context, dir string, args ...string) (string, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Errorf("git %v runs with no deadline", args)
			}
			if dir != root {
				t.Errorf("git %v runs for %q, want %q", args, dir, root)
			}
			seen = append(seen, slices.Clone(args))
			if args[0] == "rev-parse" {
				return gitPath(root) + "\n", nil
			}
			return porcelainEntry(root), nil
		}
		siblingFloor(root, run)
		want := [][]string{{"rev-parse", "--show-toplevel"}, {"worktree", "list", "--porcelain"}}
		if !reflect.DeepEqual(seen, want) {
			t.Errorf("git ran %v, want %v", seen, want)
		}
	})

	t.Run("a sibling with a custom store path in its config", func(t *testing.T) {
		root := t.TempDir()
		sib := t.TempDir()
		if err := os.WriteFile(filepath.Join(sib, ConfigFile), []byte(`{"store": "records/adj"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		seedIDs(t, filepath.Join(sib, "records", "adj"), 1, 2, 11)
		seedIDs(t, filepath.Join(sib, "decisions"), 40) // not its store
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 12 {
			t.Errorf("floor = %d, want 12", got)
		}
	})

	t.Run("a sibling with no config uses the default store", func(t *testing.T) {
		root := t.TempDir()
		sib := siblingWith(t, 1, 2, 3, 4, 5)
		if _, err := os.Stat(filepath.Join(sib, ConfigFile)); err == nil {
			t.Fatal("the sibling has a config")
		}
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 6 {
			t.Errorf("floor = %d, want 6", got)
		}
	})

	t.Run("a sibling with an invalid config is skipped and the others count", func(t *testing.T) {
		root := t.TempDir()
		bad := siblingWith(t, upTo(30)...)
		if err := os.WriteFile(filepath.Join(bad, ConfigFile), []byte(`{"store": "decisions", "nope": 1}`), 0o644); err != nil {
			t.Fatal(err)
		}
		good := siblingWith(t, 1, 2, 3)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(bad) + porcelainEntry(good)}
		if got := siblingFloor(root, g.run); got != 4 {
			t.Errorf("floor = %d, want 4", got)
		}
	})

	t.Run("a sibling whose store is not a directory is skipped", func(t *testing.T) {
		root := t.TempDir()
		bad := t.TempDir()
		if err := os.WriteFile(filepath.Join(bad, "decisions"), []byte("not a directory"), 0o644); err != nil {
			t.Fatal(err)
		}
		good := siblingWith(t, 1, 2)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(bad) + porcelainEntry(good)}
		if got := siblingFloor(root, g.run); got != 3 {
			t.Errorf("floor = %d, want 3", got)
		}
	})

	t.Run("a sibling with an unreadable store directory is skipped", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows has no directory read permission bit to take away")
		}
		root := t.TempDir()
		bad := siblingWith(t, upTo(30)...)
		dir := filepath.Join(bad, "decisions")
		if err := os.Chmod(dir, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0o755) })
		if _, err := os.ReadDir(dir); err == nil {
			t.Skip("the directory is readable anyway (running as root)")
		}
		good := siblingWith(t, 1, 2, 3)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(bad) + porcelainEntry(good)}
		if got := siblingFloor(root, g.run); got != 4 {
			t.Errorf("floor = %d, want 4", got)
		}
	})

	t.Run("a bare entry is skipped", func(t *testing.T) {
		root := t.TempDir()
		bare := siblingWith(t, upTo(50)...)
		good := siblingWith(t, 1, 2)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(bare, "bare") + porcelainEntry(good)}
		if got := siblingFloor(root, g.run); got != 3 {
			t.Errorf("floor = %d, want 3", got)
		}
	})

	t.Run("a prunable entry whose directory is gone is skipped, one that is there counts", func(t *testing.T) {
		root := t.TempDir()
		gone := filepath.Join(t.TempDir(), "removed-worktree")
		present := siblingWith(t, 1, 2, 3, 4)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) +
			porcelainEntry(gone, "prunable gitdir file points to non-existent location") +
			porcelainEntry(present, "prunable gitdir file points to non-existent location")}
		if got := siblingFloor(root, g.run); got != 5 {
			t.Errorf("floor = %d, want 5", got)
		}
	})

	t.Run("detached and locked worktrees count", func(t *testing.T) {
		root := t.TempDir()
		a, b := siblingWith(t, 1, 2), siblingWith(t, 1, 2, 3, 4, 5, 6)
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(a, "detached") + porcelainEntry(b, "locked")}
		if got := siblingFloor(root, g.run); got != 7 {
			t.Errorf("floor = %d, want 7", got)
		}
	})

	t.Run("a file that does not parse still reserves its ID", func(t *testing.T) {
		root := t.TempDir()
		sib := t.TempDir()
		dir := filepath.Join(sib, "decisions")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "0009-broken.md"), []byte("not a record"), 0o644); err != nil {
			t.Fatal(err)
		}
		g := &fakeGit{top: gitPath(root), worktrees: porcelainEntry(root) + porcelainEntry(sib)}
		if got := siblingFloor(root, g.run); got != 10 {
			t.Errorf("floor = %d, want 10", got)
		}
	})

	t.Run("the top level is compared as a directory, so a symbolic link to the root is the root", func(t *testing.T) {
		real := t.TempDir()
		link := filepath.Join(t.TempDir(), "link")
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("cannot make a symbolic link: %v", err)
		}
		sib := siblingWith(t, 1, 2, 3)
		g := &fakeGit{top: gitPath(real), worktrees: porcelainEntry(real) + porcelainEntry(sib)}
		if got := siblingFloor(link, g.run); got != 4 {
			t.Errorf("floor = %d, want 4", got)
		}
	})
}

// A hung git must not hold the store lock for long: the floor is 0 once the
// deadline passes.
func TestSiblingFloorGivesUpWhenGitHangs(t *testing.T) {
	old := siblingGitTimeout
	siblingGitTimeout = 30 * time.Millisecond
	t.Cleanup(func() { siblingGitTimeout = old })

	root := t.TempDir()
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	run := func(ctx context.Context, dir string, args ...string) (string, error) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-stop:
			return "", errors.New("test over")
		}
	}
	done := make(chan int, 1)
	go func() { done <- siblingFloor(root, run) }()
	select {
	case got := <-done:
		if got != 0 {
			t.Errorf("floor = %d, want 0", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("siblingFloor did not give up on a git that hangs")
	}
}

// siblingsOf opens the store at root with a fake git that lists the siblings.
func storeWithSiblings(t *testing.T, root string, siblings ...string) (*Store, *fakeGit) {
	t.Helper()
	list := porcelainEntry(root)
	for _, s := range siblings {
		list += porcelainEntry(s)
	}
	g := &fakeGit{top: gitPath(root), worktrees: list}
	s := mustOpen(t, root)
	s.git = g.run
	return s, g
}

func TestCreateTakesAnIDPastTheSiblings(t *testing.T) {
	root := t.TempDir()
	seedIDs(t, filepath.Join(root, "decisions"), 1, 2, 3)
	sib := siblingWith(t, upTo(7)...)
	s, _ := storeWithSiblings(t, root, sib)

	got, err := s.Create(question(0, "Fresh"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 8 {
		t.Errorf("ID = %d, want 8", got.ID)
	}
	if _, err := os.Stat(filepath.Join(root, "decisions", "0008-fresh.md")); err != nil {
		t.Error(err)
	}
	// The sibling's store is only read.
	if n := names(t, filepath.Join(sib, "decisions")); len(n) != 7 {
		t.Errorf("sibling store = %v, want its 7 files", n)
	}
}

func TestCreateWithoutSiblingsTakesTheNextLocalID(t *testing.T) {
	root := t.TempDir()
	seedIDs(t, filepath.Join(root, "decisions"), 1, 2, 3)
	s, g := storeWithSiblings(t, root)

	got, err := s.Create(question(0, "Fresh"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 4 {
		t.Errorf("ID = %d, want 4", got.ID)
	}
	if !g.listed() {
		t.Error("git worktree list was not run")
	}
}

// The local highest ID wins when it is above the siblings'.
func TestCreateKeepsTheLocalHighestWhenItIsAboveTheSiblings(t *testing.T) {
	root := t.TempDir()
	seedIDs(t, filepath.Join(root, "decisions"), upTo(12)...)
	s, _ := storeWithSiblings(t, root, siblingWith(t, 1, 2))

	got, err := s.Create(question(0, "Fresh"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 13 {
		t.Errorf("ID = %d, want 13", got.ID)
	}
}

// The floor adds to the dangling-link rule (C155, S51); it does not replace it.
func TestCreateStillSkipsIDsNamedByLinksAboveTheSiblingFloor(t *testing.T) {
	for _, tt := range []struct {
		name string
		link func(r *model.Record)
	}{
		{"supersedes", func(r *model.Record) { r.Supersedes = []int{8} }},
		{"narrows", func(r *model.Record) { r.Narrows = []int{8} }},
		{"narrowed_by", func(r *model.Record) { r.NarrowedBy = []model.Narrowing{{ID: 8, Scope: "x"}} }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "decisions")
			seedIDs(t, dir, 1, 2, 3)
			r := question(4, "Links")
			tt.link(&r)
			seed(t, dir, "0004-links.md", r)
			s, _ := storeWithSiblings(t, root, siblingWith(t, upTo(7)...))

			got, err := s.Create(question(0, "Fresh"))
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != 9 {
				t.Errorf("ID = %d, want 9 (8 is the sibling floor but a link names it)", got.ID)
			}
		})
	}
}

// Create asks git only once, whatever it has to retry.
func TestCreateComputesTheFloorOnce(t *testing.T) {
	root := t.TempDir()
	seedIDs(t, filepath.Join(root, "decisions"), 1)
	s, g := storeWithSiblings(t, root, siblingWith(t, 1, 2, 3))
	taken := 0
	s.beforeLink = func(path string) {
		if taken < 2 { // another writer takes the name twice
			taken++
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := s.Create(question(0, "Fresh"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 6 { // 4 taken, 5 taken, 6 free
		t.Errorf("ID = %d, want 6", got.ID)
	}
	n := 0
	for _, c := range g.calls {
		if c[0] == "rev-parse" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("git rev-parse ran %d times, want 1", n)
	}
}

// The floor is read under the store's lock, so a sibling's ask cannot be missed
// between the scan and the link of this one.
func TestSiblingFloorIsReadWithTheStoreLockHeld(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seedIDs(t, dir, 1)
	s := mustOpen(t, root)
	ran := 0
	heldDuring := func(op string) func(context.Context, string, ...string) (string, error) {
		return func(ctx context.Context, dir2 string, args ...string) (string, error) {
			ran++
			unlock, err := acquire(filepath.Join(dir, lockName), 20*time.Millisecond, nil)
			if err == nil {
				unlock()
				t.Errorf("%s: the lock was free while git ran %v", op, args)
			}
			return "", errors.New("no git")
		}
	}

	s.git = heldDuring("Create")
	if _, err := s.Create(question(0, "Fresh")); err != nil {
		t.Fatal(err)
	}
	if ran != 1 {
		t.Fatalf("Create ran git %d times, want 1", ran)
	}

	// A collision for Renumber.
	b := question(1, "B")
	b.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", b)
	s.git = heldDuring("Renumber")
	if _, err := s.Renumber(renumberDate, false); err != nil {
		t.Fatal(err)
	}
	if ran != 2 {
		t.Fatalf("Renumber ran git %d times, want 1", ran-1)
	}
}

// With no collision Renumber reads and changes nothing, git included.
func TestRenumberWithoutACollisionRunsNoGit(t *testing.T) {
	root := t.TempDir()
	seedIDs(t, filepath.Join(root, "decisions"), 1, 2)
	s, g := storeWithSiblings(t, root, siblingWith(t, upTo(9)...))
	if _, err := s.Renumber(renumberDate, false); err != nil {
		t.Fatal(err)
	}
	if len(g.calls) != 0 {
		t.Errorf("git ran %v with nothing to renumber", g.calls)
	}
}

// A record that is invalid whatever its ID makes no git call either.
func TestCreateOfAnInvalidRecordRunsNoGit(t *testing.T) {
	root := t.TempDir()
	s, g := storeWithSiblings(t, root, siblingWith(t, 1))
	if _, err := s.Create(model.Record{}); err == nil {
		t.Fatal("Create accepted an invalid record")
	}
	if len(g.calls) != 0 {
		t.Errorf("git ran %v for an invalid record", g.calls)
	}
}

// collisionRoot makes a store with a duplicate ID 1 (two records) and the
// local highest ID 3.
func collisionRoot(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	b := question(1, "B")
	b.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", b)
	seedIDs(t, dir, 2, 3)
	return root, dir
}

func TestRenumberMovesPastTheSiblings(t *testing.T) {
	root, dir := collisionRoot(t)
	sib := siblingWith(t, upTo(9)...)
	s, _ := storeWithSiblings(t, root, sib)
	var checked []string
	s.validated = func(name string, id int) { checked = append(checked, fmt.Sprintf("%s=%d", name, id)) }

	got, err := s.Renumber(renumberDate, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Moves) != 1 || got.Moves[0].NewID != 10 || got.Moves[0].NewName != "0010-b.md" {
		t.Fatalf("moves = %+v, want 0001-b.md to 0010-b.md", got.Moves)
	}
	if r := mustRead(t, filepath.Join(dir, "0010-b.md")); r.ID != 10 {
		t.Errorf("0010-b.md has id %d", r.ID)
	}
	// The in-memory check saw the ID that was written.
	if want := []string{"0001-b.md=10"}; !reflect.DeepEqual(checked, want) {
		t.Errorf("pre-check validated %v, want %v", checked, want)
	}
}

// Several moves climb from the floor one at a time, in the check and in the
// writes.
func TestRenumberSeveralMovesClimbFromTheSiblingFloor(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	for i, name := range []string{"0001-a.md", "0001-b.md", "0001-c.md"} {
		r := question(1, string(rune('A'+i)))
		r.Created = fmt.Sprintf("2026-10-0%d", i+1)
		seed(t, dir, name, r)
	}
	s, _ := storeWithSiblings(t, root, siblingWith(t, upTo(9)...))
	var checked []int
	s.validated = func(name string, id int) { checked = append(checked, id) }

	got, err := s.Renumber(renumberDate, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Moves) != 2 || got.Moves[0].NewID != 10 || got.Moves[1].NewID != 11 {
		t.Errorf("moves = %+v, want ids 10 and 11", got.Moves)
	}
	if !reflect.DeepEqual(checked, []int{10, 11}) {
		t.Errorf("pre-check validated %v, want [10 11]", checked)
	}
}

// The floor adds to C155: a link above the floor is still skipped.
func TestRenumberSkipsALinkedIDAboveTheSiblingFloor(t *testing.T) {
	root, dir := collisionRoot(t)
	r := mustRead(t, filepath.Join(dir, "0003-q3.md"))
	r.Narrows = []int{10}
	if err := model.WriteFile(filepath.Join(dir, "0003-q3.md"), r); err != nil {
		t.Fatal(err)
	}
	s, _ := storeWithSiblings(t, root, siblingWith(t, upTo(9)...))

	got, err := s.Renumber(renumberDate, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Moves) != 1 || got.Moves[0].NewID != 11 {
		t.Errorf("moves = %+v, want the move to 11 (10 is named by a link)", got.Moves)
	}
}

// With no sibling the move is where it was: one past the local highest.
func TestRenumberWithoutSiblingsMovesToTheNextLocalID(t *testing.T) {
	root, _ := collisionRoot(t)
	s, _ := storeWithSiblings(t, root)
	got, err := s.Renumber(renumberDate, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Moves) != 1 || got.Moves[0].NewID != 4 {
		t.Errorf("moves = %+v, want the move to 4", got.Moves)
	}
}
