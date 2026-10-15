// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// ErrAmbiguousLinks is matched by the error Renumber returns when a record
// links to an ID that more than one file has and the caller did not accept
// that the link stays with the record that keeps the ID (C147).
// ErrStaleCopy is matched by the error Renumber returns when a record file is
// the stale copy that an interrupted earlier Renumber left behind (C148):
// another file holds the same record under a new ID.
var ErrStaleCopy = errors.New("stale copy left by an interrupted renumber")

var ErrAmbiguousLinks = errors.New("links to a colliding id are ambiguous")

// Move is one record file that Renumber gave a new ID.
type Move struct {
	OldID, NewID     int
	OldName, NewName string // file names
	KeptName         string // the file that kept OldID
}

// Link is a supersedes or superseded_by link to an ID that more than one file
// had. Renumber leaves such links as they are (C147).
type Link struct {
	File  string // the file name that holds the link
	ID    int    // that file's ID
	Field string // "supersedes" or "superseded_by"
	To    int    // the colliding ID

	KeptName string // the file that keeps To, and so the one the link now names
}

// RenumberResult is what Renumber did. After an error, Moves still lists the
// moves that were completed.
type RenumberResult struct {
	Moves []Move
	Links []Link // links left in place because keepLinks was set
}

// renumberAttempts is how often Renumber tries a new file name for one
// record before it gives up.
const renumberAttempts = 10

// NameText returns the text of a record file name after the ID and the first
// "-": 0003-foo.md gives foo.md. A renumbered record keeps it (C146).
func NameText(name string) string {
	_, text, _ := strings.Cut(name, "-")
	return text
}

// RenumberedFrom returns the ID a record was renumbered from, if its last
// history entry is a renumber entry with the note "from N" (model.Renumber).
func RenumberedFrom(r model.Record) (int, bool) {
	if len(r.History) == 0 {
		return 0, false
	}
	last := r.History[len(r.History)-1]
	if last.Event != model.EventRenumber {
		return 0, false
	}
	n, ok := strings.CutPrefix(last.Note, "from ")
	if !ok {
		return 0, false
	}
	id, err := strconv.Atoi(n)
	return id, err == nil && id > 0
}

// IsStaleCopy reports whether stale is the old file of a renumber that was
// interrupted before it was removed: moved, a file with another ID and the
// same name text, holds a record whose last history entry says it was
// renumbered from stale's ID (C148).
func IsStaleCopy(stale, moved File, movedRec model.Record) bool {
	from, ok := RenumberedFrom(movedRec)
	return ok && moved.ID != stale.ID && from == stale.ID && NameText(moved.Name) == NameText(stale.Name)
}

// Renumber resolves every ID that more than one record file has (C26). Of the
// files that share an ID, the record created first keeps it, and the file name
// breaks a tie (C146); each other file gets the next free ID (one past the
// highest in the store), taking colliding IDs in ascending order. The record is
// written to a new file named with the new ID and the old file's text, with the
// old file's permission bits and one renumber history entry dated date; then
// the old file is removed (C148).
//
// Links are never rewritten: a link to a colliding ID cannot be told apart
// from a link to the other record (C147). If any record links to one, Renumber
// returns an error matching ErrAmbiguousLinks and changes nothing, unless
// keepLinks is set; the links are then left as they are, so they name the
// record that kept the ID, and the result lists them. History notes are not
// rewritten either (C149).
//
// A new ID is one past the highest in the store and never one that a link in
// the store names without a file, which would resolve that link to the moved
// record (C155).
//
// With no collision nothing is read and nothing changes. Otherwise every record
// file must be readable, since any of them may hold a link; a colliding record
// must have a valid created date, which picks the keeper; no colliding file may
// be the stale copy of an interrupted earlier Renumber (ErrStaleCopy: Renumber
// would move it again and the record would exist twice, C148); and every record
// that moves must be valid with the ID it will get. All of that is checked
// before the first file is written, so a refusal changes nothing. A move that
// fails midway is undone if it can be; moves completed before it stay.
func (s *Store) Renumber(date string, keepLinks bool) (RenumberResult, error) {
	var res RenumberResult
	files, err := s.Files()
	if err != nil {
		return res, err
	}
	groups := map[int][]File{}
	var shared []int // colliding IDs, ascending
	for _, f := range files {
		groups[f.ID] = append(groups[f.ID], f)
		if len(groups[f.ID]) == 2 {
			shared = append(shared, f.ID)
		}
	}
	if len(shared) == 0 {
		return res, nil
	}

	recs := map[string]model.Record{} // by file name
	for _, f := range files {
		data, err := os.ReadFile(f.Path)
		if err != nil {
			return res, fmt.Errorf("renumber: %w", err)
		}
		rec, err := model.Parse(f.Name, data)
		if err != nil {
			return res, fmt.Errorf("renumber: %w", err)
		}
		recs[f.Name] = rec
	}

	for _, id := range shared {
		for _, f := range groups[id] {
			for _, g := range files {
				if IsStaleCopy(f, g, recs[g.Name]) {
					return res, fmt.Errorf("renumber: %w: %s has the same record renumbered from id %d, so %s is the "+
						"stale copy; check both, delete %s, then run again", ErrStaleCopy, g.Name, f.ID, f.Name, f.Name)
				}
			}
			if _, err := time.Parse(time.DateOnly, recs[f.Name].Created); err != nil {
				return res, fmt.Errorf("renumber: %s: created %q is not a YYYY-MM-DD date, so renumber cannot tell "+
					"which record is older; fix it first", f.Name, recs[f.Name].Created)
			}
		}
	}

	// Pick the file that keeps each colliding ID; the rest move.
	keeper := map[int]File{}
	type pending struct {
		file File
		kept File
	}
	var moving []pending
	for _, id := range shared {
		group := slices.Clone(groups[id])
		slices.SortStableFunc(group, func(a, b File) int {
			if c := cmp.Compare(recs[a.Name].Created, recs[b.Name].Created); c != 0 {
				return c
			}
			return cmp.Compare(a.Name, b.Name)
		})
		keeper[id] = group[0]
		for _, f := range group[1:] {
			moving = append(moving, pending{f, group[0]})
		}
	}

	links := collidingLinks(files, recs, groups, keeper)
	if len(links) > 0 && !keepLinks {
		return res, ambiguousLinks(links)
	}

	// Check every move in memory first, with the ID it will get: the IDs go
	// up by one record at a time, as the files are written.
	avoid := linkedIDs(files, recs, groups)
	highest := files[len(files)-1].ID
	for _, m := range moving {
		rec := recs[m.file.Name]
		rec.ID = m.file.ID // the ID in the file name is the record's identity (C59)
		id := freeID(highest, 0, avoid)
		if err := model.Renumber(&rec, id, date); err != nil {
			return res, fmt.Errorf("renumber: %s: %w", m.file.Name, err)
		}
		highest = id
	}

	for _, m := range moving {
		move, err := s.moveFile(m.file, m.kept, recs[m.file.Name], date, avoid)
		if err != nil {
			return res, err
		}
		res.Moves = append(res.Moves, move)
	}
	res.Links = links
	return res, nil
}

// collidingLinks lists the supersedes and superseded_by links, in any record
// file, to an ID that more than one file has, in file order. A link listed
// twice in one record is listed once.
func collidingLinks(files []File, recs map[string]model.Record, groups map[int][]File, keeper map[int]File) []Link {
	var out []Link
	for _, f := range files {
		rec := recs[f.Name]
		add := func(field string, to int) {
			if len(groups[to]) < 2 {
				return
			}
			l := Link{File: f.Name, ID: f.ID, Field: field, To: to, KeptName: keeper[to].Name}
			if !slices.Contains(out, l) {
				out = append(out, l)
			}
		}
		for _, to := range rec.Supersedes {
			add("supersedes", to)
		}
		add("superseded_by", rec.SupersededBy)
	}
	return out
}

// linkedIDs returns the IDs that a supersedes or superseded_by link in the
// store names although no file has them.
func linkedIDs(files []File, recs map[string]model.Record, groups map[int][]File) map[int]bool {
	out := map[int]bool{}
	for _, f := range files {
		rec := recs[f.Name]
		for _, to := range append(slices.Clone(rec.Supersedes), rec.SupersededBy) {
			if to > 0 && len(groups[to]) == 0 {
				out[to] = true
			}
		}
	}
	return out
}

// freeID returns the next ID for a moved record: one past highest, at least
// floor, and not in avoid.
func freeID(highest, floor int, avoid map[int]bool) int {
	id := max(floor, highest+1)
	for avoid[id] {
		id++
	}
	return id
}

// ambiguousLinks is the error for links that Renumber will not leave
// unchecked.
func ambiguousLinks(links []Link) error {
	parts := make([]string, len(links))
	for i, l := range links {
		parts[i] = fmt.Sprintf("%s %s %d (stays with %s)", l.File, l.Field, l.To, l.KeptName)
	}
	return fmt.Errorf("%w: %s; renumber cannot tell which record each one means", ErrAmbiguousLinks, strings.Join(parts, "; "))
}

// moveFile writes rec, the record in file old, under the next free ID and
// removes old. The new file is complete before the old one goes, so a crash
// between the two leaves both and loses nothing (C148).
func (s *Store) moveFile(old, kept File, rec model.Record, date string, avoid map[int]bool) (Move, error) {
	perm := recordPerm
	if fi, err := os.Stat(old.Path); err == nil {
		perm = fi.Mode().Perm()
	}
	text := NameText(old.Name)

	floor := 0 // the lowest ID not tried yet
	var newName, newPath string
	for range renumberAttempts {
		es, err := s.scan()
		if err != nil {
			return Move{}, err
		}
		highest := 0
		if len(es) > 0 {
			highest = es[len(es)-1].id
		}
		newID := freeID(highest, floor, avoid)
		rec.ID = old.ID
		moved := rec
		if err := model.Renumber(&moved, newID, date); err != nil {
			return Move{}, fmt.Errorf("renumber: %s: %w", old.Name, err)
		}
		newName = fmt.Sprintf("%04d-%s", newID, text)
		newPath = filepath.Join(s.dir, newName)
		err = s.writeNewMode(newPath, moved, perm)
		if errors.Is(err, os.ErrExist) {
			floor = newID + 1 // another writer, or a directory, has the name
			continue
		}
		if err != nil {
			return Move{}, fmt.Errorf("renumber: %s: %w", old.Name, err)
		}
		syncDir(s.dir) // best effort: the new name should be on disk before the old one goes (C148)
		return s.dropOld(old, kept, newName, newPath, newID)
	}
	return Move{}, fmt.Errorf("renumber: %s: gave up after %d attempts: %w", old.Name, renumberAttempts,
		&fs.PathError{Op: "create", Path: newPath, Err: os.ErrExist})
}

// syncDir flushes the directory entries of dir. An error is ignored: not every
// platform can sync a directory, and the file itself is already synced.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}

// dropOld removes the old file once the new one is in place. If it cannot, it
// takes the new file away again, and if it cannot do that either the error
// says both files are there.
func (s *Store) dropOld(old, kept File, newName, newPath string, newID int) (Move, error) {
	var err error
	if s.beforeRemove != nil {
		err = s.beforeRemove(old.Path, newPath)
	}
	if err == nil {
		err = os.Remove(old.Path)
	}
	if err != nil {
		if rerr := os.Remove(newPath); rerr != nil {
			return Move{}, fmt.Errorf("renumber: cannot remove %s: %v; %s was written for the record "+
				"and cannot be taken away again (%v), so both files are there: delete %s once you have checked them",
				old.Name, err, newName, rerr, newName)
		}
		return Move{}, fmt.Errorf("renumber: cannot remove %s: %v; %s was removed again and the record is left as it was",
			old.Name, err, newName)
	}
	return Move{OldID: old.ID, NewID: newID, OldName: old.Name, NewName: newName, KeptName: kept.Name}, nil
}
