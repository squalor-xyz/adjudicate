// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// siblingGitTimeout bounds all the git calls of one siblingFloor, so a hung git
// cannot hold the store lock for long. A variable so a test can shorten it.
var siblingGitTimeout = 3 * time.Second

// gitRunner runs git with args in the repo at root and returns its standard
// output. A non-zero exit is an error. Production uses runGit; tests inject
// output so that they need no git.
type gitRunner func(ctx context.Context, root string, args ...string) (string, error)

// runGit runs the git binary on PATH as `git -C root args...`. Its standard
// error is not shown to anyone; a failure, a missing binary or the deadline
// is an error.
func runGit(ctx context.Context, root string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	// A wrapper script may leave a child holding the pipe after git is killed;
	// do not wait for it past the deadline.
	cmd.WaitDelay = 100 * time.Millisecond
	out, err := cmd.Output()
	return string(out), err
}

// siblingFloor returns the lowest ID a new record in the store at root may take
// without colliding with a sibling worktree of the same git repository: one
// past the highest record ID in any other worktree's store (S43), or 0 when
// there is nothing to clear. It never fails: git missing or failing, a root
// that is not the top level of its worktree, or a sibling that cannot be read,
// all leave the floor as it is, so behaviour is then exactly what it was
// without siblings. Nothing is reported for what is skipped.
//
// root is looked into only when it is the top level of its worktree
// (`git rev-parse --show-toplevel`). That keeps a store below the top level, or
// a test directory inside some other checkout, from reading that checkout's
// worktrees.
//
// Only file names are read, as Create reads them, and the sibling's lock is
// not taken: an ask running in a sibling at the same moment can still collide.
func siblingFloor(root string, run gitRunner) int {
	if root == "" || run == nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), siblingGitTimeout)
	defer cancel()

	out, err := run(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return 0
	}
	// Git prints forward slashes on Windows, and the root may be spelled
	// differently (a symbolic link, a short name, another case): compare the
	// two as directories.
	top, err := os.Stat(filepath.FromSlash(strings.TrimRight(out, "\r\n")))
	if err != nil {
		return 0
	}
	here, err := os.Stat(root)
	if err != nil || !os.SameFile(top, here) {
		return 0
	}

	out, err = run(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return 0
	}
	highest := 0
	for _, w := range parseWorktrees(out) {
		if w.Bare {
			continue
		}
		path := filepath.FromSlash(w.Path)
		if fi, err := os.Stat(path); err != nil || os.SameFile(fi, here) {
			continue // gone (a prunable entry), or this worktree
		}
		highest = max(highest, highestID(path))
	}
	if highest == 0 {
		return 0
	}
	return highest + 1
}

// highestID returns the highest record ID in the store of the repo at root, or
// 0 if there is none or it cannot be read: a root with an invalid config, a
// store that is not a directory or cannot be listed. A root with no config has
// the default store, as an older branch does.
func highestID(root string) int {
	s, err := Open(root)
	if err != nil {
		return 0
	}
	es, err := s.scan()
	if err != nil || len(es) == 0 {
		return 0
	}
	return es[len(es)-1].id
}

// worktreeEntry is one worktree of `git worktree list --porcelain`.
type worktreeEntry struct {
	Path     string // as git prints it: forward slashes on Windows
	Bare     bool
	Prunable bool
}

// parseWorktrees reads the output of `git worktree list --porcelain`: entries
// separated by blank lines, each starting with a `worktree PATH` line and
// followed by attribute lines (`HEAD`, `branch`, `detached`, `bare`, `locked`,
// `prunable`). A path git had to quote (a name with unusual bytes) is
// unquoted. Anything else is ignored.
func parseWorktrees(out string) []worktreeEntry {
	var entries []worktreeEntry
	var cur *worktreeEntry
	flush := func() {
		if cur != nil {
			entries = append(entries, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			flush()
			continue
		}
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			flush()
			if strings.HasPrefix(path, `"`) {
				if p, err := strconv.Unquote(path); err == nil {
					path = p
				}
			}
			cur = &worktreeEntry{Path: path}
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case line == "bare":
			cur.Bare = true
		case line == "prunable" || strings.HasPrefix(line, "prunable "):
			cur.Prunable = true
		}
	}
	flush()
	return entries
}

// siblingFloor is the floor for this store: see the function of that name.
func (s *Store) siblingFloor() int {
	run := s.git
	if run == nil {
		run = runGit
	}
	return siblingFloor(s.root, run)
}
