// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || windows

package store

import "os"

// openLockFile opens the lock file at path for acquire, creating it if it is
// missing. If the read-write open fails with a permission error (denied says
// which errors are) and the file exists, it is opened read-only instead (C164):
// both flock and LockFileEx need no write access, and a lock file that is
// read-only (checked out by another user, chmod 444, the read-only attribute
// on Windows) must not stop an ask. The read-only open never creates the file;
// if it is not there, or this fails too, or it is a directory (a read-only
// open of one succeeds, and locking it would lock the directory), the error of
// the first open stands. open is os.OpenFile outside the tests.
func openLockFile(path string, open func(name string, flag int, perm os.FileMode) (*os.File, error), denied func(error) bool) (*os.File, error) {
	f, err := open(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err == nil || !denied(err) {
		return f, err
	}
	ro, rerr := open(path, os.O_RDONLY, 0)
	if rerr != nil {
		return nil, err
	}
	if fi, serr := ro.Stat(); serr != nil || fi.IsDir() {
		ro.Close()
		return nil, err
	}
	return ro, nil
}
