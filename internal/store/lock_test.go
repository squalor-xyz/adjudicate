// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd

package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// waitFor is how long a test waits for another goroutine before it gives up.
// A passing test never waits this long.
const waitFor = 5 * time.Second

// created is what a goroutine's Create returned.
type created struct {
	rec model.Record
	err error
}

// createInBackground runs s.Create(r) in a goroutine and returns the channel
// that gets its result.
func createInBackground(s *Store, r model.Record) <-chan created {
	done := make(chan created, 1)
	go func() {
		rec, err := s.Create(r)
		done <- created{rec, err}
	}()
	return done
}

// waitingBehind arranges for s to report, on the returned channel, that it
// found the lock held by someone else.
func waitingBehind(s *Store) <-chan struct{} {
	busy := make(chan struct{}, 1)
	s.lockBusy = func() {
		select {
		case busy <- struct{}{}:
		default:
		}
	}
	return busy
}

// holdUntilBlocked is for use inside a hook of the writer that holds the lock:
// it starts second.Create and returns once second is waiting for the lock. If
// second finishes or never waits, the lock did not do its job and the test
// fails; the result is put back on the channel for the caller.
func holdUntilBlocked(t *testing.T, busy <-chan struct{}, done chan created, what string) {
	t.Helper()
	select {
	case <-busy:
	case res := <-done:
		t.Errorf("%s finished (%+v, err %v) while the first writer held the lock", what, res.rec.ID, res.err)
		done <- res
	case <-time.After(waitFor):
		t.Errorf("%s neither waited for the lock nor finished", what)
	}
}

// receive returns the result of a background Create, or fails the test.
func receive(t *testing.T, done <-chan created) created {
	t.Helper()
	select {
	case res := <-done:
		return res
	case <-time.After(waitFor):
		t.Fatal("the second Create never finished")
		return created{}
	}
}

// Two writers on one directory with different titles each scan, then link. The
// lock holds the second back from its scan until the first has linked, so
// they take different IDs (C114). Without the lock the hook shows both
// scanning before either links.
func TestCreateHoldsTheLockFromScanToLink(t *testing.T) {
	root := t.TempDir()
	first, second := mustOpen(t, root), mustOpen(t, root)
	busy := waitingBehind(second)
	done := make(chan created, 1)
	first.beforeLink = func(path string) {
		first.beforeLink = nil
		go func() {
			rec, err := second.Create(question(0, "Second title"))
			done <- created{rec, err}
		}()
		holdUntilBlocked(t, busy, done, "the second Create")
	}

	got, err := first.Create(question(0, "First title"))
	if err != nil {
		t.Fatal(err)
	}
	other := receive(t, done)
	if other.err != nil {
		t.Fatal(other.err)
	}
	if got.ID != 1 || other.rec.ID != 2 {
		t.Errorf("IDs = %d and %d, want 1 and 2", got.ID, other.rec.ID)
	}
	if want := []string{"0001-first-title.md", "0002-second-title.md"}; !reflect.DeepEqual(names(t, first.Dir()), want) {
		t.Errorf("files = %v, want %v", names(t, first.Dir()), want)
	}
}

// The lock file is in the store directory, is not a record, and does not stop
// the next Create from counting.
func TestCreateKeepsTheLockFileInTheStoreDirectory(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	if _, err := s.Create(question(0, "One")); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(s.Dir(), ".adj.lock")); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("lock file .adj.lock in the store directory: %v, %v", fi, err)
	}
	files, err := s.Files()
	if err != nil || len(files) != 1 || files[0].Name != "0001-one.md" {
		t.Errorf("Files = %+v, %v; want only 0001-one.md", files, err)
	}
	recs, err := s.List()
	if err != nil || len(recs) != 1 {
		t.Errorf("List = %d records, %v; want 1", len(recs), err)
	}
	if got, err := s.Create(question(0, "Two")); err != nil || got.ID != 2 {
		t.Errorf("second Create = %d, %v; want ID 2", got.ID, err)
	}
}

// Create leaves nothing holding the lock, whichever way it ends.
func TestCreateReleasesTheLock(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(s *Store)
		rec     model.Record
		wantErr bool
	}{
		{"created", func(*Store) {}, question(0, "t"), false},
		{"the name is taken every time", func(s *Store) {
			s.beforeLink = func(path string) { writeFile(t, path, "taken\n") }
		}, question(0, "t"), true},
		// Valid for the ID it is checked with first, invalid for the ID the
		// empty store gives it: the error comes with the lock held.
		{"invalid for the ID it gets", func(*Store) {}, func() model.Record {
			r := question(0, "t")
			r.Supersedes = []int{1}
			return r
		}(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			s := mustOpen(t, root)
			tt.prepare(s)
			_, err := s.Create(tt.rec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			other := mustOpen(t, root)
			other.lockTimeout = 300 * time.Millisecond
			unlock, err := other.lock()
			if err != nil {
				t.Fatalf("the lock is still held after Create: %v", err)
			}
			unlock()
		})
	}
}

// A lock held by someone else for longer than the writer will wait is an
// error that names the lock file, and nothing is written (C32).
func TestCreateFailsWhenTheLockStaysHeld(t *testing.T) {
	root := t.TempDir()
	holder, s := mustOpen(t, root), mustOpen(t, root)
	unlock, err := holder.lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	s.lockTimeout = 50 * time.Millisecond

	start := time.Now()
	_, err = s.Create(question(0, "Never written"))
	if err == nil {
		t.Fatal("Create succeeded while another process held the lock")
	}
	lockPath := filepath.Join(s.Dir(), ".adj.lock")
	if !strings.Contains(err.Error(), lockPath) {
		t.Errorf("err = %v, want it to name %s", err, lockPath)
	}
	if !strings.Contains(err.Error(), "still held by another process after") || strings.Contains(err.Error(), "adj process") {
		t.Errorf("err = %v, want \"still held by another process after\" (the holder need not be adj)", err)
	}
	if time.Since(start) > waitFor {
		t.Errorf("Create waited %v for a 50ms timeout", time.Since(start))
	}
	if got := names(t, s.Dir()); len(got) != 0 {
		t.Errorf("files = %v, want no record and no temporary file", got)
	}
}

// Renumber allocates IDs by scan-then-link too, so it takes the same lock: a
// Create that starts while a move is between its scan and its link waits and
// takes the ID after the moved one (C114).
func TestRenumberHoldsTheLockWhileItMovesRecords(t *testing.T) {
	root := fixture(t, "collision")
	first, second := mustOpen(t, root), mustOpen(t, root)
	busy := waitingBehind(second)
	done := make(chan created, 1)
	first.beforeLink = func(path string) {
		first.beforeLink = nil
		go func() {
			rec, err := second.Create(question(0, "Newcomer"))
			done <- created{rec, err}
		}()
		holdUntilBlocked(t, busy, done, "Create")
	}

	res, err := first.Renumber(renumberDate, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moves) != 3 || res.Moves[2].NewID != 6 {
		t.Fatalf("moves = %+v, want three, the last to ID 6", res.Moves)
	}
	other := receive(t, done)
	if other.err != nil {
		t.Fatal(other.err)
	}
	if other.rec.ID != 7 {
		t.Errorf("Create got ID %d, want 7 (after the renumbered records)", other.rec.ID)
	}
	want := []string{"0001-intro.md", "0002-alpha.md", "0003-zzz-early.md", "0004-beta.md", "0005-mmm-mid.md", "0006-aaa-late.md", "0007-newcomer.md"}
	if !reflect.DeepEqual(names(t, first.Dir()), want) {
		t.Errorf("files = %v, want %v", names(t, first.Dir()), want)
	}
}

// A renumber that cannot get the lock changes nothing and names the lock file.
func TestRenumberFailsWhenTheLockStaysHeld(t *testing.T) {
	root := fixture(t, "collision")
	holder, s := mustOpen(t, root), mustOpen(t, root)
	unlock, err := holder.lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	s.lockTimeout = 50 * time.Millisecond
	before := tree(t, root)

	res, err := s.Renumber(renumberDate, false)
	lockPath := filepath.Join(s.Dir(), ".adj.lock")
	if err == nil || !strings.Contains(err.Error(), lockPath) {
		t.Fatalf("err = %v, want one that names %s", err, lockPath)
	}
	if len(res.Moves) != 0 {
		t.Errorf("moves = %+v, want none", res.Moves)
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("Renumber changed the store without the lock")
	}
}

// Renumber releases the lock when a move fails, as it does when all succeed.
func TestRenumberReleasesTheLock(t *testing.T) {
	for _, fail := range []bool{false, true} {
		root := fixture(t, "collision")
		s := mustOpen(t, root)
		if fail {
			s.beforeRemove = func(oldPath, newPath string) error { return errors.New("disk on fire") }
		}
		_, err := s.Renumber(renumberDate, false)
		if (err != nil) != fail {
			t.Fatalf("fail=%v: err = %v", fail, err)
		}
		other := mustOpen(t, root)
		other.lockTimeout = 300 * time.Millisecond
		unlock, err := other.lock()
		if err != nil {
			t.Fatalf("fail=%v: the lock is still held after Renumber: %v", fail, err)
		}
		unlock()
	}
}

// With no collision Renumber changes nothing, so it takes no lock either: no
// lock file appears, and a store directory that is not there is not made.
func TestRenumberWithoutACollisionTakesNoLock(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	if _, err := mustOpen(t, root).Renumber(renumberDate, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".adj.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("lock file after a Renumber with nothing to do: %v", err)
	}

	empty := t.TempDir()
	if _, err := mustOpen(t, empty).Renumber(renumberDate, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(empty, "decisions")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("store directory after a Renumber of a missing store: %v", err)
	}
}
