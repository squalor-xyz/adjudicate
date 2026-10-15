// SPDX-License-Identifier: GPL-3.0-or-later

// Package lint checks a store for integrity problems: records that break the
// model's rules, two files with one ID, links to records that do not exist,
// supersede cycles, two decided records on one key, and a decision with no
// decided_by (C08, C26, C27).
// It reads the record files itself with model.Parse, so a broken or invalid
// file becomes a finding where store.List would fail (C61).
package lint

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// Severity says whether a finding fails the lint: errors do, warnings do not.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// The checks a finding can come from; Checks lists them all.
const (
	// CheckParse: the file cannot be read or decoded, so nothing else is
	// known about it.
	CheckParse = "parse"
	// CheckFields: the record breaks a rule of model.Validate, such as a
	// missing required field or a bad date.
	CheckFields = "fields"
	// CheckDanglingLink: supersedes or superseded_by names an ID that no
	// record file has.
	CheckDanglingLink = "dangling-link"
	// CheckSupersedeState: superseded_by is set on a record whose status is
	// not superseded (model.Validate does not check it).
	CheckSupersedeState = "supersede-state"
	// CheckSupersedeCycle: the record is in a loop of supersede links.
	CheckSupersedeCycle = "supersede-cycle"
	// CheckKeyConflict: another decided record has the same key.
	CheckKeyConflict = "key-conflict"
	// CheckIDMismatch: the record's id is not the ID in its file name, which
	// store.List refuses (C59).
	CheckIDMismatch = "id-mismatch"
	// CheckDuplicateID: more than one record file has the ID in its name
	// (two branches or clones took the same number, C26; asks in one checkout
	// are locked against it, C114).
	CheckDuplicateID = "duplicate-id"
	// CheckRenumberLeftover: the file is the stale copy that an interrupted
	// adj renumber left behind (C148): another file has the same record,
	// renumbered from this file's ID.
	CheckRenumberLeftover = "renumber-leftover"
	// CheckDecidedBy: a decided record has no decided_by (a warning, C27).
	CheckDecidedBy = "decided-by"
)

// Checks lists every check name.
var Checks = []string{
	CheckParse, CheckFields, CheckIDMismatch, CheckDuplicateID, CheckRenumberLeftover, CheckDanglingLink,
	CheckSupersedeState, CheckSupersedeCycle, CheckKeyConflict, CheckDecidedBy,
}

// Finding is one problem in one record file. ID is the ID in the file name
// (C59), which a file that does not parse still has.
type Finding struct {
	ID       int      `json:"id"`
	File     string   `json:"file"`
	Severity Severity `json:"severity"`
	Check    string   `json:"check"`
	Message  string   `json:"message"`
}

// Counts returns how many of fs are errors and how many are warnings.
func Counts(fs []Finding) (errs, warns int) {
	for _, f := range fs {
		switch f.Severity {
		case SeverityError:
			errs++
		case SeverityWarning:
			warns++
		}
	}
	return errs, warns
}

// parsed is a record file that decoded.
type parsed struct {
	file string // slash-separated path relative to the root
	name string // the file name
	id   int    // the ID in the file name
	rec  model.Record
}

// Run lints every record file in st and returns the findings sorted by record
// ID, then check, then file; a store with no problem gives nil. File names are
// slash-separated and relative to root. A file that cannot be read or decoded
// is a finding. The error is for a store directory that cannot be scanned.
//
// A record's identity is the ID in its file name, as for the store (C59); a
// link is dangling only if no file has the ID, so a link to a file that does
// not parse is not. Two files with one ID are a duplicate-id finding on each
// of them (adj renumber fixes it, C26); a link to such an ID is not dangling.
func Run(st *store.Store, root string) ([]Finding, error) {
	files, err := st.Files()
	if err != nil {
		return nil, err
	}
	var out []Finding
	add := func(id int, file string, sev Severity, check, format string, args ...any) {
		out = append(out, Finding{ID: id, File: file, Severity: sev, Check: check, Message: fmt.Sprintf(format, args...)})
	}

	exists := map[int]bool{}
	sharing := map[int][]string{} // ID -> the file names that have it, in name order
	for _, f := range files {
		sharing[f.ID] = append(sharing[f.ID], f.Name)
	}
	var recs []parsed
	for _, f := range files {
		exists[f.ID] = true
		rel := relPath(root, f.Path)
		if names := sharing[f.ID]; len(names) > 1 {
			add(f.ID, rel, SeverityError, CheckDuplicateID, "more than one file has id %d: %s", f.ID, strings.Join(names, ", "))
		}
		data, err := os.ReadFile(f.Path)
		if err != nil {
			add(f.ID, rel, SeverityError, CheckParse, "cannot read the file: %s", reason(err))
			continue
		}
		rec, err := model.Parse(f.Name, data)
		if err != nil {
			msg := strings.TrimPrefix(err.Error(), f.Name+": ")
			msg = strings.TrimPrefix(msg, model.ErrInvalid.Error()+": ")
			add(f.ID, rel, SeverityError, CheckParse, "%s", msg)
			continue
		}
		recs = append(recs, parsed{file: rel, name: f.Name, id: f.ID, rec: rec})
	}

	for _, p := range recs {
		r := p.rec
		if err := r.Validate(); err != nil {
			for _, problem := range problems(err) {
				add(p.id, p.file, SeverityError, CheckFields, "%s", problem)
			}
		}
		if r.ID > 0 && r.ID != p.id {
			add(p.id, p.file, SeverityError, CheckIDMismatch, "id %d does not match the ID %d in the file name", r.ID, p.id)
		}
		if r.SupersededBy != 0 && r.Status.Valid() && r.Status != model.StatusSuperseded {
			add(p.id, p.file, SeverityError, CheckSupersedeState, "superseded_by %d is set but the status is %s", r.SupersededBy, r.Status)
		}
		// Links that Validate already refuses (not positive, or the record
		// itself) are fields findings, not dangling ones. Validate compares
		// with the record's id, so a link to the file's own ID is reported
		// here when the two differ.
		// A link listed twice is reported once.
		reported := map[string]bool{}
		dangling := func(name string, link int) {
			key := fmt.Sprintf("%s %d", name, link)
			if reported[key] {
				return
			}
			reported[key] = true
			switch {
			case link <= 0 || link == r.ID:
			case link == p.id:
				add(p.id, p.file, SeverityError, CheckFields, "%s: %d is not another record's id", name, link)
			case !exists[link]:
				add(p.id, p.file, SeverityError, CheckDanglingLink, "%s %d names no record", name, link)
			}
		}
		for _, link := range r.Supersedes {
			dangling("supersedes", link)
		}
		dangling("superseded_by", r.SupersededBy)
		if r.Status == model.StatusDecided && strings.TrimSpace(r.DecidedBy) == "" {
			add(p.id, p.file, SeverityWarning, CheckDecidedBy, "decided_by is missing")
		}
	}

	out = append(out, leftovers(recs)...)
	out = append(out, cycles(recs, exists)...)
	out = append(out, keyConflicts(recs)...)

	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Check != b.Check {
			return a.Check < b.Check
		}
		return a.File < b.File
	})
	return out, nil
}

// relPath returns path relative to root with slashes, or path itself if it
// cannot be made relative.
func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

// reason is the cause of a file error without the path, which the finding
// names already.
func reason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// problems splits the error from Record.Validate, an errors.Join, into one
// message per rule broken, without the "invalid record: " prefix.
func problems(err error) []string {
	errs := []error{err}
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		errs = j.Unwrap()
	}
	out := make([]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, strings.TrimPrefix(e.Error(), model.ErrInvalid.Error()+": "))
	}
	return out
}

// leftovers reports each file that an interrupted renumber left behind: its
// record also exists under another ID, in a file with the same name text whose
// last history entry says it was renumbered from this file's ID (C148).
func leftovers(recs []parsed) []Finding {
	var out []Finding
	for _, p := range recs {
		for _, q := range recs {
			if store.IsStaleCopy(store.File{ID: p.id, Name: p.name}, store.File{ID: q.id, Name: q.name}, q.rec) {
				out = append(out, Finding{
					ID: p.id, File: p.file, Severity: SeverityError, Check: CheckRenumberLeftover,
					Message: fmt.Sprintf("%s has this record renumbered from id %d: this file is a stale copy left by an interrupted renumber; delete it", q.name, p.id),
				})
			}
		}
	}
	return out
}

// cycles reports each record that is in a loop of supersede links. An edge
// runs from a record to the record that replaces it: from OLD to NEW for
// OLD's superseded_by, and for each X in NEW's supersedes from X to NEW. The
// records of a strongly connected component of more than one record are in a
// cycle; a record that only leads into a cycle is not. Only IDs that have a
// file are named or flagged.
func cycles(recs []parsed, exists map[int]bool) []Finding {
	edges := map[int][]int{}
	link := func(from, to int) {
		if from > 0 && to > 0 && from != to {
			edges[from] = append(edges[from], to)
		}
	}
	for _, p := range recs {
		link(p.id, p.rec.SupersededBy)
		for _, old := range p.rec.Supersedes {
			link(old, p.id)
		}
	}

	// Tarjan's algorithm.
	var (
		index   = map[int]int{}
		low     = map[int]int{}
		onStack = map[int]bool{}
		stack   []int
		next    int
		inCycle = map[int]string{} // record ID -> message
		visit   func(v int)
	)
	visit = func(v int) {
		index[v], low[v] = next, next
		next++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range edges[v] {
			if _, seen := index[w]; !seen {
				visit(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		var comp []int
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			comp = append(comp, w)
			if w == v {
				break
			}
		}
		if len(comp) < 2 {
			return
		}
		// A loop can run through an ID with no file (dangling-link reports
		// it); only records that exist are named.
		comp = slices.DeleteFunc(comp, func(id int) bool { return !exists[id] })
		if len(comp) < 2 {
			return
		}
		sort.Ints(comp)
		msg := "supersede cycle among records " + joinIDs(comp)
		for _, id := range comp {
			inCycle[id] = msg
		}
	}
	var nodes []int
	for v := range edges {
		nodes = append(nodes, v)
	}
	sort.Ints(nodes)
	for _, v := range nodes {
		if _, seen := index[v]; !seen {
			visit(v)
		}
	}

	var out []Finding
	for _, p := range recs {
		if msg, ok := inCycle[p.id]; ok {
			out = append(out, Finding{ID: p.id, File: p.file, Severity: SeverityError, Check: CheckSupersedeCycle, Message: msg})
		}
	}
	return out
}

// keyConflicts reports each decided record whose key (trimmed, case-sensitive)
// is also on another decided record. recs is in ID order, so the IDs named are
// too. Superseded, open, rejected and withdrawn records do not conflict, and
// neither do records with no key.
func keyConflicts(recs []parsed) []Finding {
	byKey := map[string][]parsed{}
	for _, p := range recs {
		if key := strings.TrimSpace(p.rec.Key); key != "" && p.rec.Status == model.StatusDecided {
			byKey[key] = append(byKey[key], p)
		}
	}
	var out []Finding
	for key, group := range byKey {
		for _, p := range group {
			var others []int
			for _, q := range group {
				if q.id != p.id && !slices.Contains(others, q.id) {
					others = append(others, q.id)
				}
			}
			if len(others) == 0 {
				continue
			}
			noun := "records"
			if len(others) == 1 {
				noun = "record"
			}
			out = append(out, Finding{
				ID: p.id, File: p.file, Severity: SeverityError, Check: CheckKeyConflict,
				Message: fmt.Sprintf("key %q is also decided by %s %s", key, noun, joinIDs(others)),
			})
		}
	}
	return out
}

// joinIDs formats IDs as "1, 2, 3".
func joinIDs(ids []int) string {
	s := make([]string, len(ids))
	for i, id := range ids {
		s[i] = strconv.Itoa(id)
	}
	return strings.Join(s, ", ")
}
