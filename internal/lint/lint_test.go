// SPDX-License-Identifier: GPL-3.0-or-later

package lint_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/lint"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// open opens the fixture store testdata/<name>.
func open(t *testing.T, name string) (*store.Store, string) {
	t.Helper()
	root := filepath.Join("testdata", name)
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return st, root
}

// finding builds the finding that a fixture file should give.
func finding(id int, file string, sev lint.Severity, check, msg string) lint.Finding {
	return lint.Finding{ID: id, File: "decisions/" + file, Severity: sev, Check: check, Message: msg}
}

const (
	errSev  = lint.SeverityError
	warnSev = lint.SeverityWarning
)

// fixtureCases is one fixture store per defect class, with the exact report
// (sorted by record ID, then check) that lint must give for it.
var fixtureCases = []struct {
	name string
	want []lint.Finding
}{
	{"clean", nil},
	{"missing-fields", []lint.Finding{
		finding(1, "0001-no-answer.md", errSev, lint.CheckFields, "rationale: is required once decided"),
		finding(1, "0001-no-answer.md", errSev, lint.CheckFields, "decided_at: is required once decided"),
		finding(1, "0001-no-answer.md", errSev, lint.CheckFields, "decision: is required once decided"),
		finding(2, "0002-no-title.md", errSev, lint.CheckFields, "title: is required"),
		finding(2, "0002-no-title.md", errSev, lint.CheckFields, "created: is required"),
		finding(3, "0003-no-why.md", errSev, lint.CheckFields, "rationale: is required once rejected"),
		finding(3, "0003-no-why.md", errSev, lint.CheckFields, "decided_at: is required once rejected"),
		finding(4, "0004-no-link.md", errSev, lint.CheckFields, "superseded_by: is required once superseded"),
		finding(5, "0005-no-status.md", errSev, lint.CheckFields, `status: unknown status ""`),
		finding(6, "0006-no-id.md", errSev, lint.CheckFields, "id: must be a positive integer, got 0"),
	}},
	{"dangling-links", []lint.Finding{
		finding(1, "0001-first.md", errSev, lint.CheckDanglingLink, "supersedes 7 names no record"),
		finding(1, "0001-first.md", errSev, lint.CheckDanglingLink, "supersedes 9 names no record"),
		finding(2, "0002-second.md", errSev, lint.CheckDanglingLink, "superseded_by 8 names no record"),
	}},
	{"cycle-two", []lint.Finding{
		finding(1, "0001-first.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2"),
		finding(2, "0002-second.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2"),
	}},
	{"cycle-long", []lint.Finding{
		finding(1, "0001-link-1.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2, 3, 4"),
		finding(2, "0002-link-2.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2, 3, 4"),
		finding(3, "0003-link-3.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2, 3, 4"),
		finding(4, "0004-link-4.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2, 3, 4"),
	}},
	{"cycle-supersedes", []lint.Finding{
		finding(1, "0001-first.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2"),
		finding(2, "0002-second.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2"),
	}},
	{"key-conflict", []lint.Finding{
		finding(1, "0001-first.md", errSev, lint.CheckKeyConflict, `key "storage.db" is also decided by records 2, 3`),
		finding(2, "0002-second.md", errSev, lint.CheckKeyConflict, `key "storage.db" is also decided by records 1, 3`),
		finding(3, "0003-third.md", errSev, lint.CheckKeyConflict, `key "storage.db" is also decided by records 1, 2`),
	}},
	{"missing-decided-by", []lint.Finding{
		finding(1, "0001-nobody.md", warnSev, lint.CheckDecidedBy, "decided_by is missing"),
		finding(2, "0002-blank.md", warnSev, lint.CheckDecidedBy, "decided_by is missing"),
	}},
	{"supersede-state", []lint.Finding{
		finding(1, "0001-first.md", errSev, lint.CheckSupersedeState, "superseded_by 2 is set but the status is decided"),
		finding(3, "0003-third.md", errSev, lint.CheckSupersedeState, "superseded_by 2 is set but the status is rejected"),
	}},
	{"broken-file", []lint.Finding{
		finding(2, "0002-garbled.md", errSev, lint.CheckParse, "no opening --- line"),
		finding(4, "0004-unclosed.md", errSev, lint.CheckParse, "frontmatter is not closed by a --- line"),
	}},
	{"parse-errors", []lint.Finding{
		finding(1, "0001-bad-json.md", errSev, lint.CheckParse, "frontmatter: unexpected EOF"),
		finding(2, "0002-unknown-key.md", errSev, lint.CheckParse, `frontmatter: json: unknown field "colour"`),
		finding(3, "0003-type-error.md", errSev, lint.CheckParse, "frontmatter: json: cannot unmarshal string into Go struct field Record.id of type int"),
		finding(4, "0004-empty.md", errSev, lint.CheckParse, "frontmatter: frontmatter is empty"),
		finding(5, "0005-trailing.md", errSev, lint.CheckParse, "frontmatter: data after the JSON object"),
	}},
	// The ID in the file name is the record's identity (C59): an id that
	// differs from it is an error, and a link to the file's own ID is a link
	// to itself even though the record's id is another.
	{"id-mismatch", []lint.Finding{
		finding(2, "0002-wrong-id.md", errSev, lint.CheckIDMismatch, "id 7 does not match the ID 2 in the file name"),
		finding(3, "0003-self-link.md", errSev, lint.CheckFields, "superseded_by: 3 is not another record's id"),
		finding(3, "0003-self-link.md", errSev, lint.CheckIDMismatch, "id 5 does not match the ID 3 in the file name"),
		// A link to the record's own id is Validate's finding, not a dangling one.
		finding(4, "0004-self-by-id.md", errSev, lint.CheckFields, "superseded_by: 9 is not another record's id"),
		finding(4, "0004-self-by-id.md", errSev, lint.CheckIDMismatch, "id 9 does not match the ID 4 in the file name"),
	}},
	// Two files with one ID (C26): every file gets the finding, and the
	// message names all of them. A link to the shared ID is not dangling.
	{"duplicate-id", []lint.Finding{
		finding(2, "0002-first.md", errSev, lint.CheckDuplicateID, "more than one file has id 2: 0002-first.md, 0002-second.md"),
		finding(2, "0002-second.md", errSev, lint.CheckDuplicateID, "more than one file has id 2: 0002-first.md, 0002-second.md"),
		finding(4, "0004-a.md", errSev, lint.CheckDuplicateID, "more than one file has id 4: 0004-a.md, 0004-b.md, 0004-c.md"),
		finding(4, "0004-b.md", errSev, lint.CheckDuplicateID, "more than one file has id 4: 0004-a.md, 0004-b.md, 0004-c.md"),
		finding(4, "0004-c.md", errSev, lint.CheckDuplicateID, "more than one file has id 4: 0004-a.md, 0004-b.md, 0004-c.md"),
	}},
	// A crash between writing the new file and removing the old one leaves
	// the old file behind (C148): the record exists twice, so the stale copy is
	// an error of its own, wherever it still collides or not.
	{"renumber-leftover", []lint.Finding{
		finding(2, "0002-alpha.md", errSev, lint.CheckDuplicateID, "more than one file has id 2: 0002-alpha.md, 0002-beta.md"),
		finding(2, "0002-beta.md", errSev, lint.CheckDuplicateID, "more than one file has id 2: 0002-alpha.md, 0002-beta.md"),
		finding(2, "0002-beta.md", errSev, lint.CheckRenumberLeftover, "0004-beta.md has this record renumbered from id 2: this file is a stale copy left by an interrupted renumber; check both, delete it"),
	}},
	{"renumber-leftover-alone", []lint.Finding{
		finding(2, "0002-beta.md", errSev, lint.CheckRenumberLeftover, "0004-beta.md has this record renumbered from id 2: this file is a stale copy left by an interrupted renumber; check both, delete it"),
	}},
	// The moved record may be edited after the crash (here it is decided):
	// the old file is still the stale copy.
	{"renumber-leftover-edited", []lint.Finding{
		finding(2, "0002-beta.md", errSev, lint.CheckRenumberLeftover, "0004-beta.md has this record renumbered from id 2: this file is a stale copy left by an interrupted renumber; check both, delete it"),
	}},
	// A stale file written by hand or an older version (empty arrays, no final
	// newline in the body) is the same record as the canonical moved file.
	{"renumber-leftover-noncanonical", []lint.Finding{
		finding(2, "0002-beta.md", errSev, lint.CheckRenumberLeftover, "0004-beta.md has this record renumbered from id 2: this file is a stale copy left by an interrupted renumber; check both, delete it"),
	}},
	{"renumber-leftover-noncanonical-edited", []lint.Finding{
		finding(2, "0002-beta.md", errSev, lint.CheckRenumberLeftover, "0004-beta.md has this record renumbered from id 2: this file is a stale copy left by an interrupted renumber; check both, delete it"),
	}},
	// 0003-foo.md and 0005-foo.md share the name text and the record in 0005
	// was renumbered from 3, but it is another record than the one in
	// 0003-foo.md (two hand-named files, 0003-foo.md and 00003-foo.md, once
	// collided and one was moved): the keeper is live, not a stale copy.
	{"renumber-keeper", nil},
	// A link listed twice is one finding, not two identical ones.
	{"duplicate-supersedes", []lint.Finding{
		finding(1, "0001-first.md", errSev, lint.CheckDanglingLink, "supersedes 7 names no record"),
	}},
	// A cycle that runs through an ID with no file names only the records
	// that exist; dangling-link reports the missing one.
	{"cycle-missing", []lint.Finding{
		finding(1, "0001-first.md", errSev, lint.CheckDanglingLink, "supersedes 9 names no record"),
		finding(1, "0001-first.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2"),
		finding(2, "0002-second.md", errSev, lint.CheckDanglingLink, "superseded_by 9 names no record"),
		finding(2, "0002-second.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 1, 2"),
		// A loop through a missing ID with only one record that exists is
		// just the dangling links.
		finding(3, "0003-lonely.md", errSev, lint.CheckDanglingLink, "supersedes 8 names no record"),
		finding(3, "0003-lonely.md", errSev, lint.CheckDanglingLink, "superseded_by 8 names no record"),
	}},
	// Findings of several checks come in ID order, then check name, whatever
	// order the checks run in.
	{"order", []lint.Finding{
		finding(1, "0001-first.md", warnSev, lint.CheckDecidedBy, "decided_by is missing"),
		finding(1, "0001-first.md", errSev, lint.CheckKeyConflict, `key "k" is also decided by record 2`),
		finding(2, "0002-second.md", errSev, lint.CheckDanglingLink, "supersedes 9 names no record"),
		finding(2, "0002-second.md", errSev, lint.CheckKeyConflict, `key "k" is also decided by record 1`),
		finding(3, "0003-third.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 3, 4"),
		finding(4, "0004-fourth.md", errSev, lint.CheckSupersedeCycle, "supersede cycle among records 3, 4"),
	}},
	// A free-text recommendation (C167) is no finding, with or without
	// options; a label that is not an option stays one, and so do a label with
	// text, a blank text and a recommendation with neither.
	{"recommendations", []lint.Finding{
		finding(3, "0003-label-not-an-option.md", errSev, lint.CheckFields, `recommendation: label "z" is not one of the options`),
		finding(4, "0004-label-and-text.md", errSev, lint.CheckFields, "recommendation: has both a label and text; want one"),
		finding(5, "0005-blank-text.md", errSev, lint.CheckFields, "recommendation: text must not be blank"),
		finding(6, "0006-empty-recommendation.md", errSev, lint.CheckFields, "recommendation: needs a label or text"),
	}},
	// A history entry with an event or a key this version does not know is read
	// and kept (S44): lint warns, one finding per entry and check, and the
	// warnings alone do not fail the lint.
	{"unknown-history", []lint.Finding{
		finding(1, "0001-future-event.md", warnSev, lint.CheckUnknownEvent, `history entry 1: unknown event "future-thing"`),
		finding(2, "0002-extra-key.md", warnSev, lint.CheckUnknownHistoryKey, `history entry 1: unknown key "by"`),
		finding(3, "0003-both.md", warnSev, lint.CheckUnknownEvent, `history entry 1: unknown event "future-thing"`),
		finding(3, "0003-both.md", warnSev, lint.CheckUnknownEvent, `history entry 2: unknown event "other-thing"`),
		finding(3, "0003-both.md", warnSev, lint.CheckUnknownHistoryKey, `history entry 1: unknown keys "alpha", "zeta"`),
	}},
	// A blank event is still an error, and only that: it is not also an unknown
	// event (S44).
	{"blank-history-event", []lint.Finding{
		finding(1, "0001-blank-event.md", errSev, lint.CheckFields, "history: entry 1: event is required"),
	}},
	// An unknown event's was is kept raw: it is an unknown event and nothing
	// more, whatever the was holds (S44, C213).
	{"unknown-event-was", []lint.Finding{
		finding(1, "0001-was-raw.md", warnSev, lint.CheckUnknownEvent, `history entry 1: unknown event "future-thing"`),
		finding(1, "0001-was-raw.md", warnSev, lint.CheckUnknownEvent, `history entry 2: unknown event "other-thing"`),
	}},
	{"mixed", []lint.Finding{
		finding(1, "0001-pick-a-store.md", warnSev, lint.CheckDecidedBy, "decided_by is missing"),
		finding(2, "0002-cache-layer.md", errSev, lint.CheckFields, "rationale: is required once decided"),
		finding(3, "0003-old-queue.md", errSev, lint.CheckDanglingLink, "superseded_by 9 names no record"),
	}},
}

func TestRunReportsEachDefectClass(t *testing.T) {
	for _, tt := range fixtureCases {
		t.Run(tt.name, func(t *testing.T) {
			st, root := open(t, tt.name)
			got, err := lint.Run(st, root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings =\n%s\nwant\n%s", dump(got), dump(tt.want))
			}
		})
	}
}

func dump(fs []lint.Finding) string {
	var b strings.Builder
	for _, f := range fs {
		b.WriteString("  " + f.File + " [" + string(f.Severity) + "] " + f.Check + ": " + f.Message + "\n")
	}
	return b.String()
}

// Every check has a fixture that makes it report, so none can be dropped
// without a test failing.
func TestEveryCheckHasAFixture(t *testing.T) {
	seen := map[string]bool{}
	for _, tt := range fixtureCases {
		for _, f := range tt.want {
			seen[f.Check] = true
		}
	}
	var got []string
	for c := range seen {
		got = append(got, c)
	}
	sort.Strings(got)
	want := append([]string(nil), lint.Checks...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("checks with a fixture = %v, want every check %v", got, want)
	}
}

// Lint is for the stores that List refuses (C61): it reports a broken or
// invalid file instead of failing to load it.
func TestRunWorksWhereListFails(t *testing.T) {
	for _, name := range []string{"broken-file", "missing-fields", "parse-errors", "id-mismatch"} {
		t.Run(name, func(t *testing.T) {
			st, root := open(t, name)
			if _, err := st.List(); err == nil {
				t.Fatal("List succeeded on this fixture; it should be strict (C61)")
			}
			got, err := lint.Run(st, root)
			if err != nil || len(got) == 0 {
				t.Errorf("Run = %v, %v; want findings and no error", got, err)
			}
		})
	}
}

// A file that cannot be read is a parse finding for its record, not an error
// that stops the lint.
func TestRunReportsAnUnreadableFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "0001-locked.md")
	if err := os.WriteFile(path, []byte("---\n{}\n---\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(path); err == nil {
		t.Skip("the file is readable despite mode 000 (running as root, or on Windows, which has no such mode)")
	}
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := lint.Run(st, root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got) != 1 || got[0].ID != 1 || got[0].File != "decisions/0001-locked.md" || got[0].Check != lint.CheckParse || got[0].Severity != errSev {
		t.Errorf("findings =\n%swant one parse error for record 1", dump(got))
	}
}

func TestCounts(t *testing.T) {
	fs := []lint.Finding{
		{Severity: errSev}, {Severity: warnSev}, {Severity: errSev}, {Severity: errSev},
	}
	if e, w := lint.Counts(fs); e != 3 || w != 1 {
		t.Errorf("Counts = %d errors, %d warnings; want 3 and 1", e, w)
	}
	if e, w := lint.Counts(nil); e != 0 || w != 0 {
		t.Errorf("Counts(nil) = %d, %d; want 0, 0", e, w)
	}
}
