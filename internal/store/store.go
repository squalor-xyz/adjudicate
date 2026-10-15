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

	"github.com/squalor-xyz/adjudicate/internal/model"
)

var (
	// ErrNotFound is matched by the error Get and Update return when no record
	// file in the store has the ID.
	ErrNotFound = errors.New("record not found")

	// ErrDuplicateID is matched by the error Get and Update return when more
	// than one record file in the store has the ID. Finding and fixing such
	// collisions is lint's and renumber's job (C26).
	ErrDuplicateID = errors.New("more than one record has this id")
)

// maxSlug is the longest slug in a file name, in bytes.
const maxSlug = 60

// untitled is the slug of a title that has no ASCII letter or digit.
const untitled = "untitled"

// tmpPattern names the temporary files the store writes next to a record. The
// names start with "." so they are never taken for record files.
const tmpPattern = ".adj-*.tmp"

// Store is one repo's record directory.
type Store struct {
	dir string

	// beforeLink, if set, is called by Create with the target path before the
	// temporary file is written and linked into place. Tests use it to play a
	// second writer.
	beforeLink func(path string)
}

// Open opens the store of the repo at root: root must be an existing
// directory, and the config in it, if any, is loaded. The store directory
// does not have to exist yet; Create makes it.
func Open(root string) (*Store, error) {
	// A missing root is an error; a root that is a file fails in LoadConfig.
	if _, err := os.Stat(root); err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(root)
	if err != nil {
		return nil, err
	}
	return &Store{dir: filepath.Join(root, cfg.Store)}, nil
}

// Dir returns the store directory.
func (s *Store) Dir() string { return s.dir }

// entry is a record file found by scanning the store directory.
type entry struct {
	id   int
	name string
}

// scan lists the record files in the store directory, sorted by ID and then
// by file name (os.ReadDir's order). A record file is a regular file named
// <digits>-<text>.md with at least four digits and an ID above zero; anything
// else is ignored. A missing store directory is an empty store. The ID comes
// from the file name, so a file that does not parse still holds its ID.
func (s *Store) scan() ([]entry, error) {
	des, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []entry
	for _, de := range des {
		if !de.Type().IsRegular() {
			continue
		}
		if id, ok := fileID(de.Name()); ok {
			out = append(out, entry{id, de.Name()})
		}
	}
	// os.ReadDir sorts by file name, and a stable sort keeps that order for
	// files that share an ID.
	slices.SortStableFunc(out, func(a, b entry) int { return cmp.Compare(a.id, b.id) })
	return out, nil
}

// fileID returns the ID in a record file name, or false if name is not one.
func fileID(name string) (int, bool) {
	rest, ok := strings.CutSuffix(name, ".md")
	if !ok {
		return 0, false
	}
	digits, text, ok := strings.Cut(rest, "-")
	if !ok || text == "" || len(digits) < 4 {
		return 0, false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	id, err := strconv.Atoi(digits)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// read loads one record file and checks that its ID matches its file name.
func (s *Store) read(e entry) (model.Record, error) {
	path := filepath.Join(s.dir, e.name)
	r, err := model.ReadFile(path)
	if err != nil {
		return model.Record{}, err
	}
	if r.ID != e.id {
		return model.Record{}, fmt.Errorf("%s: %w: id %d does not match the file name", path, model.ErrInvalid, r.ID)
	}
	return r, nil
}

// List returns every record in the store sorted by ID, and by file name if two
// files share an ID, so the order is the same on every call. The first file
// that cannot be read or is invalid is an error that names it.
func (s *Store) List() ([]model.Record, error) {
	es, err := s.scan()
	if err != nil {
		return nil, err
	}
	out := make([]model.Record, 0, len(es))
	for _, e := range es {
		r, err := s.read(e)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// find returns the one record file with the ID: ErrNotFound if there is none,
// ErrDuplicateID if there are several.
func (s *Store) find(id int) (entry, error) {
	es, err := s.scan()
	if err != nil {
		return entry{}, err
	}
	var hits []entry
	for _, e := range es {
		if e.id == id {
			hits = append(hits, e)
		}
	}
	switch len(hits) {
	case 0:
		return entry{}, fmt.Errorf("%w: id %d", ErrNotFound, id)
	case 1:
		return hits[0], nil
	}
	var files []string
	for _, e := range hits {
		files = append(files, e.name)
	}
	return entry{}, fmt.Errorf("%w: id %d is in %s", ErrDuplicateID, id, strings.Join(files, ", "))
}

// Get returns the record with the ID. The error matches ErrNotFound if no file
// has the ID and ErrDuplicateID if several do.
func (s *Store) Get(id int) (model.Record, error) {
	e, err := s.find(id)
	if err != nil {
		return model.Record{}, err
	}
	return s.read(e)
}

// createAttempts is how often Create tries to take a file name before it
// gives up: each attempt that finds the name taken rescans for the next ID.
const createAttempts = 3

// recordPerm is the permission bits of a new record file. Update keeps the
// bits of the file it replaces.
const recordPerm fs.FileMode = 0o644

// Create stores r as a new record and returns it with its ID. The ID is one
// past the highest ID in the store (1 for an empty store); the ID in r is
// ignored. The file is <ID padded to four digits>-<slug of the title>.md with
// mode 0644, and the store directory is made if it is missing. The record is
// validated, and an invalid one is an error matching model.ErrInvalid with
// nothing written.
//
// Create never overwrites a file: if the name is taken it rescans and tries
// the next ID, up to three times, then returns an error matching os.ErrExist
// that names the file. There is no lock, so two creates running at once with
// different titles can both succeed with the same ID; lint finds such
// collisions and renumber fixes them (C26).
func (s *Store) Create(r model.Record) (model.Record, error) {
	var path string
	for range createAttempts {
		es, err := s.scan()
		if err != nil {
			return model.Record{}, err
		}
		r.ID = 1
		if len(es) > 0 {
			r.ID = es[len(es)-1].id + 1
		}
		if err := r.Validate(); err != nil {
			return model.Record{}, err
		}
		if err := os.MkdirAll(s.dir, 0o755); err != nil {
			return model.Record{}, err
		}
		path = filepath.Join(s.dir, fmt.Sprintf("%04d-%s.md", r.ID, slug(r.Title)))
		if s.beforeLink != nil {
			s.beforeLink(path)
		}
		err = s.writeNew(path, r)
		if err == nil {
			return r, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return model.Record{}, err
		}
	}
	return model.Record{}, fmt.Errorf("create: gave up after %d attempts: %w", createAttempts, &fs.PathError{Op: "create", Path: path, Err: os.ErrExist})
}

// Update rewrites the record with r.ID at its existing path; the file is not
// renamed if the title changed. The error matches ErrNotFound if no file has
// the ID, ErrDuplicateID if several do, and model.ErrInvalid if r is invalid;
// in each case the existing file is untouched.
func (s *Store) Update(r model.Record) error {
	e, err := s.find(r.ID)
	if err != nil {
		return err
	}
	return s.replace(filepath.Join(s.dir, e.name), r)
}

// writeNew writes r to a new file at path with mode 0644 and fails with an
// error matching os.ErrExist that names path, leaving the file there
// untouched, if path exists. The record is written to a temporary file first
// and linked into place, so a reader never sees half a record and a name
// is never taken twice. A file system without hard links is not supported.
func (s *Store) writeNew(path string, r model.Record) error {
	tmp, err := s.writeTemp(r, recordPerm)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Link(tmp, path); err != nil {
		var le *os.LinkError
		if errors.As(err, &le) {
			return &fs.PathError{Op: "link", Path: path, Err: le.Err}
		}
		return err
	}
	return nil
}

// replace writes r over the file at path, keeping the file's permission bits
// (0644 if it does not exist). The record is written to a temporary file,
// synced and renamed over path. After a crash the file is the old record or
// the new one; the directory is not synced, so which one is not guaranteed.
// An invalid record leaves the old file untouched.
func (s *Store) replace(path string, r model.Record) error {
	perm := recordPerm
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	tmp, err := s.writeTemp(r, perm)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// writeTemp writes r to a new temporary file in the store directory with the
// permission bits perm, syncs it and returns its path. The record is
// validated first, so an invalid one creates nothing. On error no file is
// left behind.
func (s *Store) writeTemp(r model.Record, perm fs.FileMode) (string, error) {
	data, err := model.Marshal(r)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(s.dir, tmpPattern)
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Chmod(perm) // CreateTemp makes 0600
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return "", err
	}
	return tmp, nil
}

// slug turns a title into the text of a file name: lowercase ASCII letters and
// digits, with every other run of characters (spaces, punctuation, non-ASCII)
// becoming one "-". It is cut to 60 bytes with no "-" at either end, and a
// title with no letter or digit gives "untitled".
func slug(title string) string {
	var b strings.Builder
	dash := false
	for _, c := range title {
		switch {
		case c >= 'A' && c <= 'Z':
			c += 'a' - 'A'
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		default:
			dash = true
			continue
		}
		if dash && b.Len() > 0 {
			b.WriteByte('-')
		}
		dash = false
		b.WriteRune(c)
	}
	out := b.String()
	if len(out) > maxSlug {
		out = out[:maxSlug]
	}
	out = strings.TrimRight(out, "-")
	if out == "" {
		return untitled
	}
	return out
}
