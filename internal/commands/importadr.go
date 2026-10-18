// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/adr"
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// ImportADRInput is the input of adj import-adr. Dir is the ADR folder and
// Root the repo root, both absolute: the refs of the records name each ADR
// file by its path relative to Root, so Dir must be inside it.
type ImportADRInput struct {
	Dir, Root string
	Tags      []string // put on every record
	// DecidedBy is decided_by on every answered record; if it is empty the
	// ADR's own deciders field is used, else decided_by is left blank.
	DecidedBy string
	// DryRun reads and checks everything and reports what would be created,
	// but writes nothing.
	DryRun bool
}

// Check reports a *UsageError if the input cannot be used whatever the folder
// holds. The CLI runs it before the store is looked for.
func (in ImportADRInput) Check() error {
	for _, tag := range in.Tags {
		if strings.TrimSpace(tag) == "" {
			return usagef("tag must not be blank")
		}
	}
	return nil
}

// ImportAction is what an import did, or with DryRun would do, for one ADR file.
type ImportAction string

const (
	ImportCreated     ImportAction = "created"
	ImportSkipped     ImportAction = "skipped"      // a record with this file as a ref exists
	ImportWouldCreate ImportAction = "would create" // DryRun
)

// ImportedADR is the outcome for one ADR file.
type ImportedADR struct {
	File   string // the path relative to the repo root, slash-separated: the first ref
	Action ImportAction
	ID     int          // the record's ID; 0 for ImportWouldCreate
	Status model.Status // the record's status
}

// ImportWarning is something the importer decided on its own and the owner
// should know: a date it had to make up, a link it could not make.
type ImportWarning struct {
	File    string
	Message string
}

// ImportADRResult is what ImportADR did, in the order it did it: the
// outcomes by ADR number and the warnings by file.
type ImportADRResult struct {
	ADRs     []ImportedADR
	Warnings []ImportWarning
}

// ADRFilesError is the error for ADR files that cannot be imported. Nothing
// was written.
type ADRFilesError struct {
	Problems []string // "FILE: reason", one per file
}

func (e *ADRFilesError) Error() string {
	noun := "files cannot be imported"
	if len(e.Problems) == 1 {
		noun = "file cannot be imported"
	}
	return fmt.Sprintf("%d ADR %s, so nothing was written:\n  %s", len(e.Problems), noun, strings.Join(e.Problems, "\n  "))
}

// adrStore is the part of the store that import-adr uses; tests wrap the real
// store to make a write fail.
type adrStore interface {
	List() ([]model.Record, error)
	Create(r model.Record) (model.Record, error)
	Update(r model.Record) error
}

// ImportADR creates one record for each ADR file in in.Dir (S22).
//
// An ADR file is a file in the folder itself (no subfolder is read) whose
// name starts with a number, with an optional "adr-" before it. Every file is
// read and every record built and validated before the first record is
// written; if any file is bad nothing is written and the error is an
// *ADRFilesError that names each bad file. A file is already imported when a
// record has an import history entry "from FILE": it is skipped, so a second
// run imports only new files. A record someone asked by hand with the file as
// a ref does not count. A skipped file is also read, only for its "Superseded
// by" line: if its record is still decided and the file now says it is
// superseded by an ADR whose record is decided, a warning gives the adj
// supersede command; nothing is written for it. A file that is a symbolic
// link is not read and is reported.
//
// The files are imported in ADR number order (then file name), so the
// records' IDs follow the order the decisions were made in. The status word
// of the ADR gives the record's status: accepted is decided, proposed is
// open, rejected is rejected, deprecated is withdrawn and superseded is
// superseded when the ADR it names is in this run and ends up decided or
// superseded; otherwise the record is decided and a warning says why. The
// two records of a link are written after all records exist, the old one
// first. Unlike adj supersede (C136), a failure writing the new record does
// not put the old one back: the old record stays superseded and the decision
// in force is still the new record, which has no link back. Any failure after
// the first record was created lists the links not yet written as adj
// supersede commands.
func ImportADR(st *store.Store, in ImportADRInput, now time.Time) (ImportADRResult, error) {
	return importADR(st, in, now)
}

// adrItem is one ADR file to import.
type adrItem struct {
	adr    adr.ADR
	ref    string // the path relative to the repo root
	status model.Status
	target int // index of the item that replaces this one, or -1
	rec    model.Record
	warns  []string
}

func importADR(st adrStore, in ImportADRInput, now time.Time) (ImportADRResult, error) {
	var res ImportADRResult
	if err := in.Check(); err != nil {
		return res, err
	}
	relDir, err := relativeTo(in.Root, in.Dir)
	if err != nil {
		return res, err
	}
	des, err := os.ReadDir(in.Dir)
	if err != nil {
		return res, err
	}
	existing, err := st.List()
	if err != nil {
		return res, err
	}
	// A file is imported when a record has an import entry "from FILE".
	imported := map[string]model.Record{}
	for _, r := range existing {
		for _, h := range r.History {
			if file, ok := strings.CutPrefix(h.Note, "from "); ok && h.Event == model.EventImport {
				if _, dup := imported[file]; !dup {
					imported[file] = r
				}
			}
		}
	}

	// Read every file that is not imported yet.
	var items []*adrItem
	skipped := map[string]model.Record{}
	var problems []string
	var linkWarns []ImportWarning
	for _, de := range des {
		name := de.Name()
		if !adr.IsFile(name) {
			continue
		}
		ref := path.Join(relDir, name)
		if de.Type()&fs.ModeSymlink != 0 {
			linkWarns = append(linkWarns, ImportWarning{File: ref, Message: "symbolic link skipped"})
			continue
		}
		if !de.Type().IsRegular() {
			continue
		}
		if r, ok := imported[ref]; ok {
			// Already imported. The file is read again, but only for its
			// link: one that no longer parses is not a problem of this run.
			skipped[name] = r
			it := &adrItem{adr: adr.ADR{File: name, Number: adr.FileNumber(name)}, ref: ref, target: -1}
			if data, err := os.ReadFile(filepath.Join(in.Dir, name)); err == nil {
				if a, err := adr.Parse(name, data); err == nil {
					it.adr = a
				}
			}
			items = append(items, it)
			continue
		}
		data, err := os.ReadFile(filepath.Join(in.Dir, name))
		if err != nil {
			problems = append(problems, ref+": "+fileReason(err))
			continue
		}
		a, err := adr.Parse(name, data)
		if err != nil {
			problems = append(problems, ref+": "+strings.TrimPrefix(err.Error(), name+": "))
			continue
		}
		items = append(items, &adrItem{adr: a, ref: ref, target: -1})
	}
	slices.SortStableFunc(items, func(a, b *adrItem) int {
		return cmp.Or(cmp.Compare(a.adr.Number, b.adr.Number), strings.Compare(a.adr.File, b.adr.File))
	})

	var fresh []*adrItem // the items to create, in order
	for _, it := range items {
		if _, ok := skipped[it.adr.File]; ok {
			continue
		}
		fresh = append(fresh, it)
	}
	for _, it := range fresh {
		if err := it.build(in, now); err != nil {
			problems = append(problems, it.ref+": "+err.Error())
		}
	}
	if len(problems) > 0 {
		slices.Sort(problems) // each starts with the path: file order
		return res, &ADRFilesError{Problems: problems}
	}
	linkSuperseded(fresh)

	// Warnings, in the order of the items, then the symbolic links.
	for _, it := range items {
		for _, w := range it.warns {
			res.Warnings = append(res.Warnings, ImportWarning{File: it.ref, Message: w})
		}
	}
	res.Warnings = append(res.Warnings, linkWarns...)

	if in.DryRun {
		for _, it := range items {
			if r, ok := skipped[it.adr.File]; ok {
				res.ADRs = append(res.ADRs, ImportedADR{File: it.ref, Action: ImportSkipped, ID: r.ID, Status: r.Status})
				continue
			}
			res.ADRs = append(res.ADRs, ImportedADR{File: it.ref, Action: ImportWouldCreate, Status: it.finalStatus()})
		}
		res.Warnings = append(res.Warnings, staleLinkWarnings(items, skipped)...)
		return res, nil
	}

	// The links to write, in order.
	var links []pendingLink
	for _, it := range fresh {
		if it.target >= 0 {
			links = append(links, pendingLink{old: it, newer: fresh[it.target]})
		}
	}

	// Phase 1: every record, a superseded one as decided and with no links.
	// Create takes the ID lock for each, as ask does.
	for _, it := range items {
		if r, ok := skipped[it.adr.File]; ok {
			res.ADRs = append(res.ADRs, ImportedADR{File: it.ref, Action: ImportSkipped, ID: r.ID, Status: r.Status})
			continue
		}
		created, err := st.Create(it.rec)
		if err != nil {
			return res, fmt.Errorf("%s: %w%s", it.ref, err, unwrittenText(links, true))
		}
		it.rec = created
		res.ADRs = append(res.ADRs, ImportedADR{File: it.ref, Action: ImportCreated, ID: created.ID, Status: created.Status})
	}
	res.Warnings = append(res.Warnings, staleLinkWarnings(items, skipped)...)

	// Phase 2: the supersede links, the old record first. If the new record
	// then cannot be written the old one is left superseded, not put back: the
	// decision in force is still the new record (check and context agree), and
	// the error says what is missing.
	for k, l := range links {
		it, newer := l.old, l.newer
		it.rec.Status = model.StatusSuperseded
		it.rec.SupersededBy = newer.rec.ID
		if err := st.Update(it.rec); err != nil {
			it.rec.Status, it.rec.SupersededBy = model.StatusDecided, 0
			return res, fmt.Errorf("%s: record %d: cannot write its superseded_by link to record %d: %w%s",
				it.ref, it.rec.ID, newer.rec.ID, err, unwrittenText(links[k:], false))
		}
		for i := range res.ADRs {
			if res.ADRs[i].File == it.ref {
				res.ADRs[i].Status = model.StatusSuperseded
			}
		}
		newer.rec.Supersedes = append(newer.rec.Supersedes, it.rec.ID)
		if err := st.Update(newer.rec); err != nil {
			return res, fmt.Errorf("%s: record %d: cannot write its supersedes link to record %d: %w; record %d is left superseded by record %d, which has no link back, so the store is inconsistent (the decision in force is still record %d); add %d to the supersedes list of record %d by hand%s",
				newer.ref, newer.rec.ID, it.rec.ID, err, it.rec.ID, newer.rec.ID, newer.rec.ID, it.rec.ID, newer.rec.ID, unwrittenText(links[k+1:], false))
		}
	}
	return res, nil
}

// pendingLink is a supersede link between two ADRs of this run.
type pendingLink struct{ old, newer *adrItem }

// unwrittenText says which of the links are not in the store, for an error
// after some records were created. Where both records exist it gives the adj
// supersede command that makes the link; where the new record does not exist
// yet (phase 1 stopped before it) it names the files. Links whose old record
// does not exist are not listed: a new run imports it.
func unwrittenText(links []pendingLink, creating bool) string {
	var b strings.Builder
	for _, l := range links {
		switch {
		case l.old.rec.ID == 0:
		case l.newer.rec.ID == 0:
			fmt.Fprintf(&b, "\n  %s is superseded by %s, which was not created: run the import again after fixing the problem, and it will name the command", l.old.ref, l.newer.ref)
		default:
			fmt.Fprintf(&b, "\n  adj supersede %d --by %d", l.old.rec.ID, l.newer.rec.ID)
		}
	}
	if b.Len() == 0 {
		return ""
	}
	head := "\nthe supersede links not written:"
	if creating {
		head = "\nthe records created so far are in the store; the supersede links not written:"
	}
	return head + b.String()
}

// resolveLink finds the ADR that l names among items: by file name, else by a
// number that exactly one ADR has. found counts the matches by number;
// ambiguous says several ADRs have it.
func resolveLink(items []*adrItem, l *adr.Link) (j, found int, ambiguous bool) {
	j = -1
	for k, other := range items {
		if l.File != "" && other.adr.File == l.File {
			return k, 1, false
		}
	}
	if l.Number > 0 {
		for k, other := range items {
			if other.adr.Number == l.Number {
				j, found = k, found+1
			}
		}
	}
	return j, found, found > 1
}

// staleLinkWarnings warns about each skipped ADR whose record is still decided
// although its file says it is superseded by an ADR whose record is decided
// (or will be): the link cannot be written by an import, so the warning gives
// the owner's command. A run that created the new record knows its ID.
func staleLinkWarnings(items []*adrItem, skipped map[string]model.Record) []ImportWarning {
	var out []ImportWarning
	for i, it := range items {
		rec, ok := skipped[it.adr.File]
		if !ok || rec.Status != model.StatusDecided || it.adr.Status != adr.Superseded || it.adr.SupersededBy == nil {
			continue
		}
		j, found, ambiguous := resolveLink(items, it.adr.SupersededBy)
		if found == 0 || ambiguous || j == i {
			continue
		}
		target := items[j]
		newID := target.rec.ID
		if tr, ok := skipped[target.adr.File]; ok {
			if tr.Status != model.StatusDecided {
				continue
			}
			newID = tr.ID
		} else if target.finalStatus() != model.StatusDecided {
			continue
		}
		msg := fmt.Sprintf("record %d is still decided, but the file now says it is superseded by %s; run `adj supersede %d --by %d`", rec.ID, target.ref, rec.ID, newID)
		if newID == 0 {
			msg = fmt.Sprintf("record %d is still decided, but the file now says it is superseded by %s; once that file is imported, run `adj supersede %d --by <the ID of its record>`", rec.ID, target.ref, rec.ID)
		}
		out = append(out, ImportWarning{File: it.ref, Message: msg})
	}
	return out
}

// relativeTo returns dir relative to root, slash-separated, or an error if dir
// is not inside root. A path through a symbolic link is also tried resolved.
func relativeTo(root, dir string) (string, error) {
	inside := func(root, dir string) (string, bool) {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", false
		}
		return filepath.ToSlash(rel), true
	}
	if rel, ok := inside(filepath.Clean(root), filepath.Clean(dir)); ok {
		return rel, nil
	}
	if r, err := filepath.EvalSymlinks(root); err == nil {
		if d, err := filepath.EvalSymlinks(dir); err == nil {
			if rel, ok := inside(r, d); ok {
				return rel, nil
			}
		}
	}
	return "", fmt.Errorf("folder %s is outside the repo root %s: the records name each ADR file by its path from the root", dir, root)
}

// fileReason is the cause of a file error without the path, which the message
// names already.
func fileReason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return "cannot read the file: " + pe.Err.Error()
	}
	return "cannot read the file: " + err.Error()
}

// finalStatus is the status the record will have once its links are written.
func (it *adrItem) finalStatus() model.Status {
	if it.target >= 0 {
		return model.StatusSuperseded
	}
	return it.rec.Status
}

// build makes the record of the ADR, without ID and links. A superseded ADR is
// built as decided with the target unset; linkSuperseded settles it.
func (it *adrItem) build(in ImportADRInput, now time.Time) error {
	a := it.adr
	run := now.Format(time.DateOnly)
	created := a.Date
	switch {
	case created != "" && created <= run:
	case created != "":
		created = run
		it.warns = append(it.warns, fmt.Sprintf("the date %q is after the run date %s; using the run date", a.DateText, run))
	case a.DateText == "":
		created = run
		it.warns = append(it.warns, fmt.Sprintf("no date; using the run date %s", run))
	case dateShape.MatchString(a.DateText):
		created = run
		it.warns = append(it.warns, fmt.Sprintf("the date %q is not a real date; using the run date %s", a.DateText, run))
	default:
		created = run
		it.warns = append(it.warns, fmt.Sprintf("the date %q is not YYYY-MM-DD; using the run date %s", a.DateText, run))
	}

	var tags []string
	for _, tag := range in.Tags {
		if tag = strings.TrimSpace(tag); !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	var opts []model.Option
	for _, label := range a.Options {
		opts = append(opts, model.Option{Label: label})
	}
	refs := []string{it.ref}
	if a.Number > 0 {
		refs = append(refs, fmt.Sprintf("ADR-%04d", a.Number))
	}
	rec := model.Record{
		Title:   a.Title,
		Tags:    tags,
		Created: created,
		Context: a.Context,
		Options: opts,
		Refs:    refs,
		History: []model.HistoryEntry{{Date: run, Event: model.EventImport, Note: "from " + it.ref}},
	}

	switch a.Status {
	case adr.Proposed:
		rec.Status = model.StatusOpen
		if a.Decision != "" {
			rec.Recommendation = &model.Recommendation{Text: a.Decision, Reasoning: firstText(a.Justification, a.Consequences)}
			if slices.ContainsFunc(opts, func(o model.Option) bool { return o.Label == a.Decision }) {
				rec.Recommendation.Text, rec.Recommendation.Label = "", a.Decision
			}
		}
	default:
		switch a.Status {
		case adr.Accepted, adr.Superseded:
			rec.Status = model.StatusDecided
			if a.Decision == "" {
				return errors.New("no Decision section or Decision Outcome section to take the decision from")
			}
			rec.Decision = a.Decision
		case adr.Rejected:
			rec.Status = model.StatusRejected
		case adr.Deprecated:
			rec.Status = model.StatusWithdrawn
		}
		rec.DecidedAt = created
		rec.DecidedBy = in.DecidedBy
		if rec.DecidedBy == "" {
			rec.DecidedBy = a.Deciders
		}
		// The reason is the justification, else the consequences, else the
		// context; a rejected or withdrawn ADR's decision text ("we will
		// not ...") comes before the consequences.
		candidates := []string{a.Justification, a.Consequences, a.Context}
		if rec.Status == model.StatusRejected || rec.Status == model.StatusWithdrawn {
			candidates = []string{a.Justification, a.Decision, a.Consequences, a.Context}
		}
		rec.Rationale = firstText(candidates...)
		if rec.Rationale == "" {
			return errors.New("no rationale: the ADR has no justification, Consequences or Context to use")
		}
		if rec.Rationale != a.Consequences {
			rec.Consequences = a.Consequences
		}
	}
	// rec is valid by construction: the title is one non-blank line, the
	// dates are real, an answered record has a non-blank decision (if
	// decided) and rationale, and a recommendation names an option or has
	// text (TestImportADRBuiltRecordsAreValid).
	it.status = rec.Status
	it.rec = rec
	if a.Status == adr.Superseded {
		it.status = model.StatusSuperseded
	}
	return nil
}

// dateShape is the shape of a date: a real date has it, and so does 2024-02-30.
var dateShape = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}`)

// firstText returns the first text that is not blank.
func firstText(texts ...string) string {
	for _, t := range texts {
		if strings.TrimSpace(t) != "" {
			return t
		}
	}
	return ""
}

// linkSuperseded decides which superseded ADRs become superseded records: the
// ones whose replacement is another item of this run that ends up decided or
// superseded and that do not run in a loop. The others stay decided, each
// with a warning.
func linkSuperseded(items []*adrItem) {
	why := make([]string, len(items))
	for i, it := range items {
		if it.status != model.StatusSuperseded {
			continue
		}
		l := it.adr.SupersededBy
		if l == nil {
			why[i] = "is superseded but names no replacement"
			continue
		}
		name := l.File
		if name == "" {
			name = fmt.Sprintf("ADR-%04d", l.Number)
		}
		j, found, ambiguous := resolveLink(items, l)
		switch {
		case ambiguous:
			why[i] = fmt.Sprintf("is superseded by %s, but more than one ADR has that number", name)
		case found == 0:
			why[i] = fmt.Sprintf("is superseded by %s, which is not imported by this run", name)
		case j == i:
			why[i] = "is superseded by itself"
		case items[j].adr.Status != adr.Accepted && items[j].adr.Status != adr.Superseded:
			why[i] = fmt.Sprintf("is superseded by %s, which is %s", name, items[j].adr.Status)
		default:
			it.target = j
		}
	}
	// A loop of links: every ADR in it stays decided.
	var loops []int
	for i := range items {
		for k, steps := items[i].target, 0; k >= 0 && steps <= len(items); k, steps = items[k].target, steps+1 {
			if k == i {
				loops = append(loops, i)
				break
			}
		}
	}
	for _, i := range loops {
		why[i] = fmt.Sprintf("is superseded by %s, which leads back to it (a supersede cycle)", items[items[i].target].adr.File)
	}
	for i, it := range items {
		if why[i] == "" {
			continue
		}
		it.target = -1
		it.warns = append(it.warns, why[i]+"; imported as decided")
	}
}
