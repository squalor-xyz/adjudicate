// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A lock file that is there but has the read-only attribute (checked out by
// another user, or attrib +R) must not stop an ask: LockFileEx needs only a
// handle that can read, so acquire opens it read-only when the read-write open
// is denied (C164). The lock still excludes, and Create works.
func TestLockFallsBackToAReadOnlyOpenOfAReadOnlyLockFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "decisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, lockName)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// Go maps a mode without write bits to the read-only attribute.
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
	if f, err := os.OpenFile(path, os.O_RDWR, 0); err == nil {
		f.Close()
		t.Skip("a read-only file can be opened for writing here")
	}

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
		t.Errorf("lock file after the calls: %v, %v; want it unchanged (read-only, empty)", fi, err)
	}
}
