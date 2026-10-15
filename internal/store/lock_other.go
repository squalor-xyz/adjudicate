// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !(darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd)

package store

import "time"

// acquire is the fallback for systems without syscall.Flock, Windows among
// them (C114): there is no lock. Create keeps its bounded retry for a name
// that is taken (C64), which catches two writers that choose the same file
// name but not two that choose the same ID with different names; lint and
// renumber find and fix those (C26).
func acquire(path string, timeout time.Duration, busy func()) (unlock func(), err error) {
	return func() {}, nil
}
