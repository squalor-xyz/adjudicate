// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package store

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

// lockPoll is how long acquire sleeps between attempts while the lock is held.
const lockPoll = 10 * time.Millisecond

// The kernel32 calls and flags of the lock, called through the standard
// library's syscall package: golang.org/x/sys is not a dependency (S36).
const (
	lockfileFailImmediately = 0x1 // LOCKFILE_FAIL_IMMEDIATELY
	lockfileExclusiveLock   = 0x2 // LOCKFILE_EXCLUSIVE_LOCK

	errorLockViolation = syscall.Errno(33) // ERROR_LOCK_VIOLATION: another handle holds the range
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

// lockFileEx locks the whole file behind h (the range is the largest a file
// can have, which is also valid beyond the end of the file, where the empty
// lock file is) exclusively, without waiting.
func lockFileEx(h syscall.Handle) error {
	var ol syscall.Overlapped
	r, _, err := procLockFileEx.Call(uintptr(h), lockfileExclusiveLock|lockfileFailImmediately,
		0, ^uintptr(0)&0xFFFFFFFF, ^uintptr(0)&0xFFFFFFFF, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}

func unlockFileEx(h syscall.Handle) error {
	var ol syscall.Overlapped
	r, _, err := procUnlockFileEx.Call(uintptr(h), 0,
		^uintptr(0)&0xFFFFFFFF, ^uintptr(0)&0xFFFFFFFF, uintptr(unsafe.Pointer(&ol)))
	if r == 0 {
		return err
	}
	return nil
}

// acquire takes an exclusive lock with LockFileEx on the file at path,
// creating it if it is missing, and returns the function that releases it
// (C114, S36). It behaves like the unix flock, except that the lock is
// mandatory (it changes nothing for the empty lock file): the attempts do not
// block (LOCKFILE_FAIL_IMMEDIATELY), acquire polls for up to timeout and then
// returns an error that names path, so a writer stuck behind a hung process
// fails instead of hanging. busy, if not nil, is called each time the lock is
// found held. The lock belongs to the open handle, so it is also released
// when the process exits; the lock file stays, because removing it would let
// a waiting writer and a new one lock different files.
func acquire(path string, timeout time.Duration, busy func()) (unlock func(), err error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	h := syscall.Handle(f.Fd())
	deadline := time.Now().Add(timeout)
	for {
		err := lockFileEx(h)
		if err == nil {
			return func() {
				unlockFileEx(h)
				f.Close()
			}, nil
		}
		if !errors.Is(err, errorLockViolation) {
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
