// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || windows

package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// openLockFile is the part of acquire that decides how the lock file is
// opened (C164). It takes the open function and the test for a permission
// error as parameters so that the read-only fallback can be tested on every
// system, with a fake open that is denied; the real permission errors differ
// by system (EACCES, ERROR_ACCESS_DENIED), and lock_unix_test.go and
// lock_windows_test.go try them for real.

var errDenied = errors.New("fake: access denied")

func isFakeDenied(err error) bool { return errors.Is(err, errDenied) }

// denyWrite is an open function that refuses any open for writing with
// errDenied and opens the rest for real; it records the flags it was called
// with.
func denyWrite(flags *[]int) func(string, int, os.FileMode) (*os.File, error) {
	return func(name string, flag int, perm os.FileMode) (*os.File, error) {
		*flags = append(*flags, flag)
		if flag&(os.O_RDWR|os.O_WRONLY) != 0 {
			return nil, &os.PathError{Op: "open", Path: name, Err: errDenied}
		}
		return os.OpenFile(name, flag, perm)
	}
}

func TestOpenLockFileOpensReadWriteAndCreatesWhenItCan(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	f, err := openLockFile(path, os.OpenFile, isFakeDenied)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if !fileExists(path) {
		t.Error("the lock file was not created")
	}
}

func TestOpenLockFileFallsBackToReadOnlyWhenWritingIsDenied(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var flags []int
	f, err := openLockFile(path, denyWrite(&flags), isFakeDenied)
	if err != nil {
		t.Fatalf("openLockFile = %v, want the read-only open of the existing file", err)
	}
	defer f.Close()
	if len(flags) != 2 || flags[1] != os.O_RDONLY {
		t.Errorf("open flags = %v, want a read-write open and then a read-only one", flags)
	}
}

// The fallback is only for a lock file that exists: the read-only open never
// creates it, and the error of the first open stands.
func TestOpenLockFileKeepsTheOriginalErrorWhenTheFileIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	var flags []int
	_, err := openLockFile(path, denyWrite(&flags), isFakeDenied)
	if !errors.Is(err, errDenied) {
		t.Errorf("openLockFile = %v, want the error of the read-write open", err)
	}
	if fileExists(path) {
		t.Error("the fallback created the lock file")
	}
}

// Only a permission error gets the read-only open.
func TestOpenLockFileDoesNotFallBackOnOtherErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	other := errors.New("fake: disk on fire")
	calls := 0
	open := func(name string, flag int, perm os.FileMode) (*os.File, error) {
		calls++
		return nil, &os.PathError{Op: "open", Path: name, Err: other}
	}
	if _, err := openLockFile(path, open, isFakeDenied); !errors.Is(err, other) {
		t.Errorf("openLockFile = %v, want the original error", err)
	}
	if calls != 1 {
		t.Errorf("open was called %d times, want 1 (no read-only attempt)", calls)
	}
}

// A read-only open of a directory succeeds, and locking it would lock the
// directory: a directory is never taken as the lock file.
func TestOpenLockFileDoesNotFallBackToADirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), lockName)
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	var flags []int
	f, err := openLockFile(path, denyWrite(&flags), isFakeDenied)
	if err == nil {
		f.Close()
		t.Fatal("openLockFile returned a directory")
	}
	if !errors.Is(err, errDenied) {
		t.Errorf("openLockFile = %v, want the error of the read-write open", err)
	}
}
