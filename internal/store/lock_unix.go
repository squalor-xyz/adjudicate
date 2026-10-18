// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// lockPoll is how long acquire sleeps between attempts while the lock is held.
const lockPoll = 10 * time.Millisecond

// acquire takes an exclusive advisory flock on the file at path, creating it
// if it is missing (or, if it exists and cannot be opened for writing (EACCES),
// through a read-only open, see openLockFile), and returns the function that
// releases it (C114). The attempts do not block (LOCK_NB): acquire polls for up
// to timeout and then returns an error that names path, so a writer stuck
// behind a hung process fails instead of hanging. busy, if not nil, is
// called each time the lock is found held. The lock goes with the file
// descriptor, so it is also released when the process exits; the lock file
// stays, because removing it would let a waiting writer and a new one lock
// different files.
//
// The build tag lists the systems whose syscall package has Flock: the "unix"
// tag also covers aix and solaris, which do not (illumos does have it).
func acquire(path string, timeout time.Duration, busy func()) (unlock func(), err error) {
	f, err := openLockFile(path, os.OpenFile, func(err error) bool { return errors.Is(err, syscall.EACCES) })
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	fd := int(f.Fd())
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				syscall.Flock(fd, syscall.LOCK_UN)
				f.Close()
			}, nil
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			f.Close()
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		if busy != nil {
			busy()
		}
		if !time.Now().Before(deadline) {
			f.Close()
			return nil, fmt.Errorf("lock %s: still held by another process after %v; nothing was written", path, timeout)
		}
		time.Sleep(lockPoll)
	}
}
