// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package store

// The tests of lock_unix.go that depend on unix permission bits and errno
// values: they have no Windows counterpart, so they are not built there (S36).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// readOnlyLock makes a store directory whose lock file exists and cannot be
// opened for writing, and returns its path. It skips the test where the
// permission cannot be denied (running as root, or a file system that ignores
// the mode bits).
func readOnlyLock(t *testing.T) (dir, path string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "decisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, lockName)
	if err := os.WriteFile(path, nil, 0o444); err != nil {
		t.Fatal(err)
	}
	if f, err := os.OpenFile(path, os.O_RDWR, 0); err == nil {
		f.Close()
		t.Skip("a read-only file can be opened for writing here (root?)")
	}
	return dir, path
}

// A lock file that is there but read-only (checked out from git by another
// user, or chmod 444) must not stop an ask: flock needs no write access, so
// acquire opens it read-only when the read-write open fails with EACCES. The
// lock still excludes: a second writer finds it held, and Create works.
func TestLockFallsBackToAReadOnlyOpenOfAnExistingLockFile(t *testing.T) {
	dir, path := readOnlyLock(t)

	unlock, err := acquire(path, time.Second, nil)
	if err != nil {
		t.Fatalf("acquire on a read-only lock file: %v", err)
	}
	if _, err := acquire(path, 50*time.Millisecond, nil); err == nil || !strings.Contains(err.Error(), "still held") {
		t.Errorf("a second acquire while the first holds the lock = %v, want 'still held'", err)
	}
	unlock()
	unlock2, err := acquire(path, time.Second, nil)
	if err != nil {
		t.Fatalf("acquire after the release: %v", err)
	}
	unlock2()

	s := mustOpen(t, filepath.Dir(dir))
	if got, err := s.Create(question(0, "Through a read-only lock")); err != nil || got.ID != 1 {
		t.Errorf("Create = %+v, %v; want ID 1", got, err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o444 || fi.Size() != 0 {
		t.Errorf("lock file after the calls: %v, %v; want it unchanged (mode 0444, empty)", fi, err)
	}
}

// The fallback is only for a lock file that exists. When the directory cannot
// be written, so the lock file cannot be made, the error is the permission
// error of the create, as before.
func TestLockStillFailsWhenTheLockFileCannotBeMade(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decisions")
	if err := os.MkdirAll(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	path := filepath.Join(dir, lockName)
	if f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644); err == nil {
		f.Close()
		t.Skip("a read-only directory can be written here (root?)")
	}
	_, err := acquire(path, time.Second, nil)
	if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), path) {
		t.Errorf("acquire = %v, want a permission error that names %s", err, path)
	}
}

// Only EACCES gets the read-only open. A lock path that is a directory fails
// the read-write open with EISDIR, and a read-only open of a directory would
// succeed and lock the directory, so the failure must stand.
func TestLockPathThatIsADirectoryStillFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := acquire(path, time.Second, nil); !errors.Is(err, syscall.EISDIR) {
		t.Errorf("acquire on a directory = %v, want EISDIR", err)
	}
}
