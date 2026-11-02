// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitEnv points git at a throwaway home so the real git config (user, system,
// includes) is never read, for the test's own git calls and for the ones the
// store makes.
func gitEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, "gitconfig-that-does-not-exist"))
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH")
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// With the real git binary: two linked worktrees of one repository, one with
// its own store path in its config and one with no config, and a new record in
// the main worktree clears the highest ID in either (S43).
func TestCreateClearsSiblingWorktreesWithRealGit(t *testing.T) {
	requireGit(t)
	gitEnv(t)
	base := t.TempDir()
	main := filepath.Join(base, "main")
	if err := os.Mkdir(main, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, main, "init", "-q")
	mustGit(t, main, "commit", "-q", "--allow-empty", "-m", "first")
	if err := os.WriteFile(filepath.Join(main, ConfigFile), []byte(`{"store": "decisions"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	seedIDs(t, filepath.Join(main, "decisions"), 1, 2, 3)

	custom := filepath.Join(base, "custom")
	plain := filepath.Join(base, "plain")
	mustGit(t, main, "worktree", "add", "-q", custom, "-b", "custom")
	mustGit(t, main, "worktree", "add", "-q", plain, "-b", "plain")
	if err := os.WriteFile(filepath.Join(custom, ConfigFile), []byte(`{"store": "records"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	seedIDs(t, filepath.Join(custom, "records"), upTo(7)...)
	seedIDs(t, filepath.Join(plain, "decisions"), 1, 2)

	s := mustOpen(t, main)
	got, err := s.Create(question(0, "Fresh"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 8 {
		t.Errorf("ID = %d, want 8", got.ID)
	}

	// Seen from the sibling, the main worktree's new record counts too.
	s2 := mustOpen(t, custom)
	got2, err := s2.Create(question(0, "Second"))
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != 9 {
		t.Errorf("ID in the sibling = %d, want 9", got2.ID)
	}
}

// Outside a git repository nothing changes: the next local ID.
func TestCreateOutsideGitTakesTheNextLocalID(t *testing.T) {
	requireGit(t)
	gitEnv(t)
	root := t.TempDir()
	seedIDs(t, filepath.Join(root, "decisions"), 1, 2, 3)
	got, err := mustOpen(t, root).Create(question(0, "Fresh"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 4 {
		t.Errorf("ID = %d, want 4", got.ID)
	}
}

// A repo root below the top level is not looked into (S43): with the store in
// a subdirectory of the repository the floor is 0 even with siblings that have
// higher IDs.
func TestCreateBelowTheGitTopLevelTakesTheNextLocalID(t *testing.T) {
	requireGit(t)
	gitEnv(t)
	base := t.TempDir()
	main := filepath.Join(base, "main")
	if err := os.Mkdir(main, 0o755); err != nil {
		t.Fatal(err)
	}
	mustGit(t, main, "init", "-q")
	mustGit(t, main, "commit", "-q", "--allow-empty", "-m", "first")
	sibling := filepath.Join(base, "sibling")
	mustGit(t, main, "worktree", "add", "-q", sibling, "-b", "sibling")
	seedIDs(t, filepath.Join(sibling, "sub", "decisions"), upTo(9)...)
	seedIDs(t, filepath.Join(sibling, "decisions"), upTo(9)...)

	root := filepath.Join(main, "sub")
	seedIDs(t, filepath.Join(root, "decisions"), 1, 2, 3)
	got, err := mustOpen(t, root).Create(question(0, "Fresh"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 4 {
		t.Errorf("ID = %d, want 4", got.ID)
	}
}

// The unit tests of this package run in directories below this repository's
// own checkout, which has worktrees with their own records. The top-level
// guard must keep them away: git is asked for the top level and for nothing
// more, and the floor stays 0.
func TestTestsInsideTheRepositoryNeverReadItsWorktrees(t *testing.T) {
	requireGit(t)
	tmp := filepath.Join("..", "..", ".cache", "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(tmp, "guard")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	root, err = filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	var calls [][]string
	run := func(ctx context.Context, dir string, args ...string) (string, error) {
		calls = append(calls, args)
		return runGit(ctx, dir, args...)
	}
	if got := siblingFloor(root, run); got != 0 {
		t.Errorf("floor = %d, want 0", got)
	}
	if len(calls) == 0 {
		t.Fatal("git was not run")
	}
	if _, err := runGit(context.Background(), root, "rev-parse", "--show-toplevel"); err != nil {
		t.Skipf("this checkout is not in a git repository: %v", err)
	}
	for _, c := range calls {
		if c[0] == "worktree" {
			t.Errorf("git %v was run for a directory that is not the top level of its repository", c)
		}
	}
}
