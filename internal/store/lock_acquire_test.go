// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || windows

package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// acquire is tested here without a Store, on the lock file alone, on every
// system that has a lock (the unix flock one and the Windows LockFileEx one;
// C114, S36): a second acquire waits, reports each time it finds the lock
// held, and fails with the error that names the lock file; it succeeds once
// the first holder has released; the lock file is never deleted.
func TestAcquireWaitsThenFailsNamingTheLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	unlock, err := acquire(path, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	const timeout = 100 * time.Millisecond
	busy := 0
	start := time.Now()
	_, err = acquire(path, timeout, func() { busy++ })
	waited := time.Since(start)
	if err == nil {
		t.Fatal("a second acquire succeeded while the first holds the lock")
	}
	if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "still held by another process after") {
		t.Errorf("err = %v, want one that names %s and says \"still held by another process after\"", err, path)
	}
	if waited < timeout {
		t.Errorf("the second acquire gave up after %v, before its %v timeout", waited, timeout)
	}
	if waited > waitFor {
		t.Errorf("the second acquire waited %v for a %v timeout", waited, timeout)
	}
	// The poll is 10 ms, so a 100 ms wait finds the lock held several times.
	if busy < 2 {
		t.Errorf("busy was called %d times, want it called on each attempt that found the lock held", busy)
	}
}

func TestAcquireSucceedsAfterTheFirstHolderReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	unlock, err := acquire(path, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	unlock()
	unlock2, err := acquire(path, 50*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("acquire after the release: %v", err)
	}
	unlock2()
}

// A waiting writer takes the lock as soon as the holder lets go, not only at
// the end of its timeout.
func TestAcquireTakesTheLockWhenTheHolderReleasesWhileItWaits(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	unlock, err := acquire(path, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	waiting := make(chan struct{}, 1)
	got := make(chan error, 1)
	go func() {
		u, err := acquire(path, waitFor, func() {
			select {
			case waiting <- struct{}{}:
			default:
			}
		})
		if err == nil {
			u()
		}
		got <- err
	}()
	select {
	case <-waiting:
	case err := <-got:
		t.Fatalf("the second acquire finished (%v) while the first held the lock", err)
	case <-time.After(waitFor):
		t.Fatal("the second acquire never found the lock held")
	}
	unlock()
	select {
	case err := <-got:
		if err != nil {
			t.Errorf("the waiting acquire failed after the release: %v", err)
		}
	case <-time.After(waitFor):
		t.Fatal("the waiting acquire never took the lock after the release")
	}
}

// Unlocking leaves the lock file: removing it would let a waiting writer and
// a new one lock different files.
func TestAcquireNeverDeletesTheLockFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	unlock, err := acquire(path, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !fileExists(path) {
		t.Fatal("no lock file while the lock is held")
	}
	unlock()
	if !fileExists(path) {
		t.Error("the lock file is gone after unlock")
	}
	unlock2, err := acquire(path, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	unlock2()
	if !fileExists(path) {
		t.Error("the lock file is gone after the second unlock")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
