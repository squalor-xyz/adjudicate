// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/lint"
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

var importNow = time.Date(2026, 3, 4, 23, 30, 0, 0, time.UTC)

const importDate = "2026-03-04"

// adrRoot copies the ADR fixture folder (internal/adr/testdata/<fixture>) into
// <root>/docs/adr of a new temp repo root and returns both paths.
func adrRoot(t *testing.T, fixture string) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("..", "adr", "testdata", fixture)
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, d.Name()), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, dir
}

// adrFiles writes the files (name -> text) to <root>/docs/adr of a new repo.
func adrFiles(t *testing.T, files map[string]string) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	dir = filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, dir
}

func openStore(t *testing.T, root string) *store.Store {
	t.Helper()
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func listAll(t *testing.T, st *store.Store) []model.Record {
	t.Helper()
	recs, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

// imported is the history of an imported record.
func imported(file string) []model.HistoryEntry {
	return []model.HistoryEntry{{Date: importDate, Event: model.EventImport, Note: "from docs/adr/" + file}}
}

func refs(file string, number string) []string {
	return []string{"docs/adr/" + file, "ADR-" + number}
}

func importOK(t *testing.T, st *store.Store, in ImportADRInput) ImportADRResult {
	t.Helper()
	res, err := ImportADR(st, in, importNow)
	if err != nil {
		t.Fatalf("ImportADR: %v", err)
	}
	return res
}

func wantRecords(t *testing.T, got, want []model.Record) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d records, want %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("record %d:\n got %+v\nwant %+v", want[i].ID, got[i], want[i])
		}
	}
}

// Each adr-tools (Nygard) status becomes a record status: accepted is decided
// (the Decision section is the decision, the Consequences section the
// rationale), proposed is open, rejected is rejected, deprecated is
// withdrawn, superseded is superseded with the links to the ADR that replaced
// it, found by the number or the file name in its status.
func TestImportADRNygardMapsEachStatus(t *testing.T) {
	root, dir := adrRoot(t, "nygard")
	st := openStore(t, root)
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, Tags: []string{"adr"}, DecidedBy: "tester"})

	f1, f2, f3, f4 := "0001-record-architecture-decisions.md", "0002-use-postgresql.md", "0003-adopt-trunk-based-development.md", "0004-use-cockroachdb.md"
	f5, f6, f7, f8 := "0005-use-mongodb.md", "0006-keep-subversion.md", "0007-ship-weekly.md", "0008-use-semantic-versioning-crlf.md"
	tags := []string{"adr"}
	want := []model.Record{
		{ID: 1, Title: "Record architecture decisions", Status: model.StatusDecided, Tags: tags, DecidedBy: "tester",
			Created: "2016-02-12", DecidedAt: "2016-02-12",
			Context:   "We need to record the architectural decisions made on this project.",
			Decision:  "We will use Architecture Decision Records, as described by Michael Nygard.",
			Rationale: "See Michael Nygard's article, linked above. For a lightweight ADR toolset, see Nat Pryce's adr-tools.",
			Refs:      refs(f1, "0001"), History: imported(f1)},
		{ID: 2, Title: "Use PostgreSQL", Status: model.StatusSuperseded, Tags: tags, DecidedBy: "tester",
			Created: "2017-03-01", DecidedAt: "2017-03-01",
			Context:      "The service needs a relational store with transactions.",
			Decision:     "We will store all service data in PostgreSQL.",
			Rationale:    "We run and patch PostgreSQL ourselves, and a single node limits availability.",
			SupersededBy: 4,
			Refs:         refs(f2, "0002"), History: imported(f2)},
		{ID: 3, Title: "Adopt trunk-based development", Status: model.StatusOpen, Tags: tags,
			Created: "2018-05-20",
			Context: "Long-lived branches make merges painful and hide integration problems.",
			Recommendation: &model.Recommendation{
				Text:      "We will merge to the main branch at least daily and use feature flags.",
				Reasoning: "Unfinished work ships behind flags, which need cleaning up.",
			},
			Refs: refs(f3, "0003"), History: imported(f3)},
		{ID: 4, Title: "Use CockroachDB", Status: model.StatusDecided, Tags: tags, DecidedBy: "tester",
			Created: "2019-07-09", DecidedAt: "2019-07-09",
			Context:    "A single PostgreSQL node cannot give the availability the service now needs.",
			Decision:   "We will store all service data in CockroachDB.",
			Rationale:  "Operations learn a new database; the wire protocol stays PostgreSQL's.",
			Supersedes: []int{2},
			Refs:       refs(f4, "0004"), History: imported(f4)},
		{ID: 5, Title: "Use MongoDB", Status: model.StatusRejected, Tags: tags, DecidedBy: "tester",
			Created: "2019-08-01", DecidedAt: "2019-08-01",
			Context:      "A document store was suggested for the event data.",
			Rationale:    "We will not use MongoDB.", // the decision text; the consequences keep their own field
			Consequences: "Event data stays in the relational store, so it needs a schema.",
			Refs:         refs(f5, "0005"), History: imported(f5)},
		{ID: 6, Title: "Keep Subversion", Status: model.StatusWithdrawn, Tags: tags, DecidedBy: "tester",
			Created: "2012-01-15", DecidedAt: "2012-01-15",
			Context:      "The code lives in a Subversion repository.",
			Rationale:    "We will keep the code in Subversion.",
			Consequences: "No pull requests; reviews happen by mail.",
			Refs:         refs(f6, "0006"), History: imported(f6)},
		{ID: 7, Title: "Ship weekly", Status: model.StatusDecided, Tags: tags, DecidedBy: "tester",
			Created: importDate, DecidedAt: importDate, // no Date line: the run date
			Context:   "Releases are rare and large.",
			Decision:  "We will cut a release every Friday.",
			Rationale: "Each release is small; the Friday freeze starts at noon.",
			Refs:      refs(f7, "0007"), History: imported(f7)},
		{ID: 8, Title: "Use semantic versioning", Status: model.StatusDecided, Tags: tags, DecidedBy: "tester",
			Created: "2020-02-29", DecidedAt: "2020-02-29",
			Context:   "Consumers need to tell breaking changes from fixes.\nThey read the version number to do so.",
			Decision:  "We will version releases as MAJOR.MINOR.PATCH.",
			Rationale: "A breaking change always raises MAJOR.",
			Refs:      refs(f8, "0008"), History: imported(f8)},
	}
	wantRecords(t, listAll(t, st), want)

	// The README in the folder is not an ADR: eight files, eight lines.
	if len(res.ADRs) != 8 {
		t.Fatalf("result has %d ADRs, want 8: %+v", len(res.ADRs), res.ADRs)
	}
	for i, a := range res.ADRs {
		wantFile := "docs/adr/" + []string{f1, f2, f3, f4, f5, f6, f7, f8}[i]
		if a.File != wantFile || a.Action != ImportCreated || a.ID != i+1 || a.Status != want[i].Status {
			t.Errorf("result %d = %+v, want %s created as id %d (%s)", i, a, wantFile, i+1, want[i].Status)
		}
	}
	// The one thing to report: the ADR with no date.
	if len(res.Warnings) != 1 || res.Warnings[0].File != "docs/adr/"+f7 || !strings.Contains(res.Warnings[0].Message, "no date") ||
		!strings.Contains(res.Warnings[0].Message, importDate) {
		t.Errorf("warnings = %+v, want one for %s that says there is no date and the run date is used", res.Warnings, f7)
	}
}

// MADR: the chosen option is the decision and its "because" the rationale;
// the Considered Options become options and the Consequences subsections the
// consequences. "superseded by ADR-0005" is found by number.
func TestImportADRMADRMapsEachStatus(t *testing.T) {
	root, dir := adrRoot(t, "madr")
	st := openStore(t, root)
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "tester"})

	opts := func(labels ...string) []model.Option {
		var out []model.Option
		for _, l := range labels {
			out = append(out, model.Option{Label: l})
		}
		return out
	}
	f := []string{
		"", "0001-use-markdown-architectural-decision-records.md", "0002-use-plain-junit5-for-testing.md", "0003-use-spring-boot.md",
		"0004-evaluate-graalvm.md", "0005-use-quarkus.md", "0006-use-xml-configuration.md", "0007-keep-java-8.md",
	}
	want := []model.Record{
		{ID: 1, Title: "Use Markdown Architectural Decision Records", Status: model.StatusDecided, DecidedBy: "tester",
			Created: "2022-11-30", DecidedAt: "2022-11-30",
			Context: "We want to record architectural decisions made in this project.\n" +
				"Which format and structure should these records follow?",
			Options:   opts("MADR 3.0.0", "Michael Nygard's template", "Sustainable Architectural Decisions", "Other templates"),
			Decision:  "MADR 3.0.0",
			Rationale: "it is lean and the decision drivers are met.",
			Consequences: "* Good, because the records are plain text next to the code\n" +
				"* Bad, because the template is long for small decisions",
			Refs: refs(f[1], "0001"), History: imported(f[1])},
		{ID: 2, Title: "Use plain JUnit5 for testing", Status: model.StatusDecided, DecidedBy: "tester",
			Created: "2019-04-23", DecidedAt: "2019-04-23",
			Context:   "We want to write unit tests in Java. Which test framework should we use?",
			Options:   opts("JUnit5", "TestNG", "Hamcrest"),
			Decision:  "JUnit5",
			Rationale: "it is the standard and the team knows it.",
			Consequences: "Positive Consequences:\n* Tests run in every IDE\n\n" +
				"Negative Consequences:\n* Parametrised tests need an extension",
			Refs: refs(f[2], "0002"), History: imported(f[2])},
		{ID: 3, Title: "Use Spring Boot", Status: model.StatusSuperseded, DecidedBy: "tester",
			Created: "2021-01-10", DecidedAt: "2021-01-10",
			Context:   "Which application framework should the services use?",
			Options:   opts("Spring Boot", "Micronaut"),
			Decision:  "Spring Boot",
			Rationale: "the team has years of experience with it.", SupersededBy: 5,
			Refs: refs(f[3], "0003"), History: imported(f[3])},
		{ID: 4, Title: "Evaluate GraalVM native images", Status: model.StatusOpen,
			Created: "2023-02-14",
			Context: "Start-up time matters for our functions. Should we build native images?",
			Options: opts("Native images with GraalVM", "Keep the JVM"),
			// The chosen option of a proposal is the recommendation, by label.
			Recommendation: &model.Recommendation{Label: "Native images with GraalVM", Reasoning: "start-up drops from seconds to milliseconds."},
			Refs:           refs(f[4], "0004"), History: imported(f[4])},
		{ID: 5, Title: "Use Quarkus", Status: model.StatusDecided, DecidedBy: "tester",
			Created: "2022-09-01", DecidedAt: "2022-09-01",
			Context:      "Which application framework should the services use, now that start-up time matters?",
			Options:      opts("Quarkus", "Spring Boot"),
			Decision:     "Quarkus",
			Rationale:    "it starts fast and fits our native-image plan.",
			Consequences: "* Good, because start-up is fast\n* Bad, because few engineers know it yet",
			Supersedes:   []int{3},
			Refs:         refs(f[5], "0005"), History: imported(f[5])},
		{ID: 6, Title: "Use XML configuration", Status: model.StatusRejected, DecidedBy: "tester",
			Created: "2020-06-01", DecidedAt: "2020-06-01",
			Context:   "How should the services be configured?",
			Options:   opts("XML files", "YAML files"),
			Rationale: "XML is too verbose for hand editing.",
			Refs:      refs(f[6], "0006"), History: imported(f[6])},
		{ID: 7, Title: "Keep Java 8", Status: model.StatusWithdrawn, DecidedBy: "tester",
			Created: "2018-03-03", DecidedAt: "2018-03-03",
			Context:   "Which Java version do we run?",
			Rationale: "We stay on Java 8 until the vendor ends support.",
			Refs:      refs(f[7], "0007"), History: imported(f[7])},
	}
	wantRecords(t, listAll(t, st), want)
	// adr-template.md is not an ADR; every ADR has a date.
	if len(res.ADRs) != 7 || len(res.Warnings) != 0 {
		t.Errorf("result = %+v, warnings %+v; want 7 ADRs and no warning", res.ADRs, res.Warnings)
	}
}

// decided_by is --by, else the ADR's deciders, else blank (lint then warns
// that a decided record has none, C27).
func TestImportADRDecidedBy(t *testing.T) {
	tests := []struct {
		name, by string
		want     map[int]string
	}{
		{"no --by: the deciders field, else blank", "", map[int]string{1: "Alice Example, Bob Example", 2: "Dana Example, Eli Example", 3: "", 5: "Alice Example", 6: "", 7: ""}},
		{"--by wins", "owner", map[int]string{1: "owner", 2: "owner", 3: "owner", 5: "owner", 6: "owner", 7: "owner"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dir := adrRoot(t, "madr")
			st := openStore(t, root)
			importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: tt.by})
			for _, r := range listAll(t, st) {
				if want, ok := tt.want[r.ID]; ok && r.DecidedBy != want {
					t.Errorf("record %d decided_by = %q, want %q", r.ID, r.DecidedBy, want)
				}
				if r.Status == model.StatusOpen && r.DecidedBy != "" {
					t.Errorf("open record %d has decided_by %q", r.ID, r.DecidedBy)
				}
				if r.AskedBy != "" {
					t.Errorf("record %d asked_by = %q, want blank", r.ID, r.AskedBy)
				}
			}
		})
	}
}

// Lint finds nothing on the store of either fixture folder; with no decided_by
// it finds only the warnings that C27 asks for.
func TestImportADRLeavesALintCleanStore(t *testing.T) {
	for _, fixture := range []string{"nygard", "madr"} {
		t.Run(fixture, func(t *testing.T) {
			root, dir := adrRoot(t, fixture)
			st := openStore(t, root)
			importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "tester"})
			findings, err := lint.Run(st, root)
			if err != nil || len(findings) != 0 {
				t.Errorf("lint = %+v, %v; want nothing", findings, err)
			}
		})
		t.Run(fixture+" without decided_by", func(t *testing.T) {
			root, dir := adrRoot(t, fixture)
			st := openStore(t, root)
			importOK(t, st, ImportADRInput{Dir: dir, Root: root})
			findings, err := lint.Run(st, root)
			if err != nil {
				t.Fatal(err)
			}
			errs, warns := lint.Counts(findings)
			// Only the nygard ADRs lack deciders; the MADR ones that are
			// decided have them.
			if errs != 0 || (fixture == "nygard" && warns == 0) {
				t.Errorf("lint = %d errors, %d warnings (%+v); want only warnings, and some for nygard", errs, warns, findings)
			}
			for _, f := range findings {
				if f.Check != lint.CheckDecidedBy {
					t.Errorf("unexpected finding %+v", f)
				}
			}
		})
	}
}

// Records take the next free IDs, not the ADR numbers, and the links use the
// IDs the records got.
func TestImportADRLinksUseRecordIDsNotADRNumbers(t *testing.T) {
	root, dir := adrRoot(t, "nygard")
	st := openStore(t, root)
	for _, title := range []string{"First", "Second"} {
		if _, err := Ask(st, AskInput{Title: title}, importNow); err != nil {
			t.Fatal(err)
		}
	}
	importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "tester"})
	postgres, err := st.Get(4) // ADR 2
	if err != nil {
		t.Fatal(err)
	}
	cockroach, err := st.Get(6) // ADR 4
	if err != nil {
		t.Fatal(err)
	}
	if postgres.Title != "Use PostgreSQL" || postgres.SupersededBy != 6 {
		t.Errorf("record 4 = %q superseded_by %d, want Use PostgreSQL superseded by 6", postgres.Title, postgres.SupersededBy)
	}
	if cockroach.Title != "Use CockroachDB" || !reflect.DeepEqual(cockroach.Supersedes, []int{4}) {
		t.Errorf("record 6 = %q supersedes %v, want Use CockroachDB superseding [4]", cockroach.Title, cockroach.Supersedes)
	}
}

// ADRs are written in number order, not file-name order, so IDs follow the
// order the decisions were made in.
func TestImportADRWritesInNumberOrder(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{
		"10-ten.md":     "# 10. Ten\n\nDate: 2020-01-10\n\n## Status\n\nProposed\n",
		"2-two.md":      "# 2. Two\n\nDate: 2020-01-02\n\n## Status\n\nProposed\n",
		"adr-0003-a.md": "# Three\n\nStatus: proposed\n",
	})
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	var titles []string
	for _, r := range listAll(t, st) {
		titles = append(titles, r.Title)
	}
	if want := []string{"Two", "Three", "Ten"}; !reflect.DeepEqual(titles, want) {
		t.Errorf("titles in ID order = %q, want %q", titles, want)
	}
}

// A second import of the same folder skips the files already imported: no
// record is added or changed, and the result says so. A new file is imported.
func TestImportADRSecondRunSkipsImportedFiles(t *testing.T) {
	root, dir := adrRoot(t, "nygard")
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "tester"})
	before := listAll(t, st)

	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "someone else"})
	if len(res.ADRs) != 8 {
		t.Fatalf("result has %d ADRs, want 8", len(res.ADRs))
	}
	for i, a := range res.ADRs {
		if a.Action != ImportSkipped || a.ID != i+1 {
			t.Errorf("result %d = %+v, want skipped as id %d", i, a, i+1)
		}
	}
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %+v, want none", res.Warnings)
	}
	wantRecords(t, listAll(t, st), before)

	extra := "# 9. Use Go\n\nDate: 2021-01-01\n\n## Status\n\nAccepted\n\n## Decision\n\nWe use Go.\n\n## Consequences\n\nOne toolchain.\n"
	if err := os.WriteFile(filepath.Join(dir, "0009-use-go.md"), []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	res = importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "tester"})
	created := 0
	for _, a := range res.ADRs {
		if a.Action == ImportCreated {
			created++
			if a.File != "docs/adr/0009-use-go.md" || a.ID != 9 {
				t.Errorf("created %+v, want 0009-use-go.md as id 9", a)
			}
		}
	}
	if created != 1 || len(listAll(t, st)) != 9 {
		t.Errorf("created %d, store has %d records; want 1 and 9", created, len(listAll(t, st)))
	}
}

// A file is "already imported" by the ref that names it; the same file name in
// another folder is another ADR.
func TestImportADRSkipsByFileRefOnly(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nStatus: proposed\n"})
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	other := filepath.Join(root, "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "0001-x.md"), []byte("# 1. X\n\nStatus: proposed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := importOK(t, st, ImportADRInput{Dir: other, Root: root})
	if len(res.ADRs) != 1 || res.ADRs[0].Action != ImportCreated || res.ADRs[0].File != "other/0001-x.md" {
		t.Errorf("result = %+v, want other/0001-x.md created", res.ADRs)
	}
	if got := listAll(t, st); len(got) != 2 || !reflect.DeepEqual(got[1].Refs, []string{"other/0001-x.md", "ADR-0001"}) {
		t.Errorf("records = %+v, want two, the second with the ref other/0001-x.md", got)
	}
}

// The folder may be the repo root itself: refs are then just the file name.
func TestImportADRRefIsRelativeToTheRepoRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "0001-x.md"), []byte("# 1. X\n\nStatus: proposed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: root, Root: root})
	recs := listAll(t, st)
	if len(recs) != 1 || !reflect.DeepEqual(recs[0].Refs, []string{"0001-x.md", "ADR-0001"}) ||
		recs[0].History[0].Note != "from 0001-x.md" {
		t.Errorf("records = %+v, want ref 0001-x.md", recs)
	}
}

func TestImportADRFolderOutsideTheRepoIsRefused(t *testing.T) {
	root := t.TempDir()
	outside, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nStatus: proposed\n"})
	_ = outside
	st := openStore(t, root)
	_, err := ImportADR(st, ImportADRInput{Dir: dir, Root: root}, importNow)
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("error = %v, want one that says the folder is outside the repo", err)
	}
	var ue *UsageError
	if errors.As(err, &ue) {
		t.Errorf("error is a usage error, want an ordinary failure")
	}
	if len(listAll(t, st)) != 0 {
		t.Error("a record was written")
	}
}

// The folder and the root may name the same place by different paths (a
// symbolic link, such as macOS's /var for /private/var).
func TestImportADRFolderThroughASymbolicLink(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nStatus: proposed\n"})
	link := filepath.Join(t.TempDir(), "repo")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: link})
	if got := listAll(t, st)[0].Refs[0]; got != "docs/adr/0001-x.md" {
		t.Errorf("ref = %q, want docs/adr/0001-x.md", got)
	}
}

func TestImportADRRefIsASlashPath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001-x.md"), []byte("# 1. X\n\nStatus: proposed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	if got := listAll(t, st)[0].Refs[0]; got != "a/b/0001-x.md" {
		t.Errorf("ref = %q, want a/b/0001-x.md", got)
	}
}

func storeFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(filepath.Join(root, "decisions"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Every file is checked before any record is written: one bad file means no
// record at all, and the error names each bad file and why.
func TestImportADRWritesNothingIfAnyFileIsBad(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{
		"0001-good.md":        "# 1. Good\n\nStatus: proposed\n",
		"0002-no-status.md":   "# 2. No status\n\nDate: 2020-01-01\n",
		"0003-bad-status.md":  "# 3. Bad status\n\nStatus: maybe\n",
		"0004-no-decision.md": "# 4. No decision\n\n## Status\n\nAccepted\n\n## Consequences\n\nSome.\n",
		"0005-no-reason.md":   "# 5. No reason\n\n## Status\n\nAccepted\n\n## Decision\n\nDo it.\n",
		"0006-no-title.md":    "## Status\n\nProposed\n",
		"0007-good.md":        "# 7. Good again\n\nStatus: rejected\n\n## Context\n\nbecause\n",
	})
	st := openStore(t, root)
	_, err := ImportADR(st, ImportADRInput{Dir: dir, Root: root}, importNow)
	if err == nil {
		t.Fatal("ImportADR succeeded, want an error")
	}
	for _, want := range []string{
		"docs/adr/0002-no-status.md", "status",
		"docs/adr/0003-bad-status.md", `unknown status "maybe"`,
		"docs/adr/0004-no-decision.md", "no Decision",
		"docs/adr/0005-no-reason.md", "rationale",
		"docs/adr/0006-no-title.md", "title",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	for _, notWant := range []string{"0001-good.md", "0007-good.md"} {
		if strings.Contains(err.Error(), notWant) {
			t.Errorf("error names the good file %s:\n%v", notWant, err)
		}
	}
	// The problems are in file order, not in the order they were found (a file
	// that does not parse is found before a record that cannot be built).
	var fe *ADRFilesError
	if !errors.As(err, &fe) || !slices.IsSorted(fe.Problems) || len(fe.Problems) != 5 {
		t.Errorf("error = %#v, want an *ADRFilesError with five problems in file order", err)
	}
	if files := storeFiles(t, root); len(files) != 0 {
		t.Errorf("the store was touched: %q", files)
	}
}

func TestImportADRUnreadableFileWritesNothing(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("file permission bits do not stop this reader here")
	}
	root, dir := adrFiles(t, map[string]string{
		"0001-good.md": "# 1. Good\n\nStatus: proposed\n",
		"0002-lock.md": "# 2. Locked\n\nStatus: proposed\n",
	})
	if err := os.Chmod(filepath.Join(dir, "0002-lock.md"), 0); err != nil {
		t.Fatal(err)
	}
	st := openStore(t, root)
	_, err := ImportADR(st, ImportADRInput{Dir: dir, Root: root}, importNow)
	if err == nil || !strings.Contains(err.Error(), "docs/adr/0002-lock.md") {
		t.Fatalf("error = %v, want one that names docs/adr/0002-lock.md", err)
	}
	if files := storeFiles(t, root); len(files) != 0 {
		t.Errorf("the store was touched: %q", files)
	}
}

func TestImportADRFolderProblems(t *testing.T) {
	root := t.TempDir()
	st := openStore(t, root)
	file := filepath.Join(root, "0001-x.md")
	if err := os.WriteFile(file, []byte("# 1. X\n\nStatus: proposed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, dir := range map[string]string{"missing": filepath.Join(root, "nope"), "a file": file} {
		if _, err := ImportADR(st, ImportADRInput{Dir: dir, Root: root}, importNow); err == nil {
			t.Errorf("%s: ImportADR succeeded, want an error", name)
		}
	}
	if files := storeFiles(t, root); len(files) != 0 {
		t.Errorf("the store was touched: %q", files)
	}
}

// A folder with no ADR file is not an error and creates nothing, not even the
// store directory.
func TestImportADRWithoutADRFiles(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{
		"README.md":        "# Architecture decisions\n",
		"adr-template.md":  "# {title}\n",
		"0001-notes.txt":   "not markdown\n",
		".0002-hidden.md":  "# 2. Hidden\n\nStatus: proposed\n",
		"nested-0003.md":   "# 3. Not numbered first\n\nStatus: proposed\n",
		"0004-a-dir.md.bk": "x\n",
	})
	if err := os.Mkdir(filepath.Join(dir, "0005-subfolder.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	st := openStore(t, root)
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	if len(res.ADRs) != 0 || len(res.Warnings) != 0 {
		t.Errorf("result = %+v, want nothing", res)
	}
	if files := storeFiles(t, root); len(files) != 0 {
		t.Errorf("the store was touched: %q", files)
	}
}

// With DryRun nothing is written, and the result and the warnings are what a
// real run would give, except that no record has an ID yet.
func TestImportADRDryRunWritesNothing(t *testing.T) {
	root, dir := adrRoot(t, "nygard")
	st := openStore(t, root)
	dry := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "tester", DryRun: true})
	if files := storeFiles(t, root); len(files) != 0 {
		t.Fatalf("the store was touched: %q", files)
	}
	if len(dry.ADRs) != 8 {
		t.Fatalf("dry run lists %d ADRs, want 8", len(dry.ADRs))
	}
	real := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "tester"})
	for i, a := range dry.ADRs {
		r := real.ADRs[i]
		if a.Action != ImportWouldCreate || a.ID != 0 || a.File != r.File || a.Status != r.Status {
			t.Errorf("dry %d = %+v, real %+v; want would-create, no ID, same file and status", i, a, r)
		}
	}
	if !reflect.DeepEqual(dry.Warnings, real.Warnings) {
		t.Errorf("dry warnings %+v differ from real %+v", dry.Warnings, real.Warnings)
	}
	// After the real run a dry run reports the files as skipped.
	again := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DryRun: true})
	for _, a := range again.ADRs {
		if a.Action != ImportSkipped {
			t.Errorf("dry run after import: %+v, want skipped", a)
		}
	}
}

func TestImportADRTagsAreCheckedAndCleaned(t *testing.T) {
	for _, tags := range [][]string{{""}, {"ok", "  "}} {
		err := ImportADRInput{Dir: "d", Root: "r", Tags: tags}.Check()
		var ue *UsageError
		if !errors.As(err, &ue) || !strings.Contains(err.Error(), "tag") {
			t.Errorf("Check with tags %q = %v, want a usage error about the tag", tags, err)
		}
	}
	if err := (ImportADRInput{Dir: "d", Root: "r", Tags: []string{"a"}}).Check(); err != nil {
		t.Errorf("Check = %v, want nil", err)
	}
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nStatus: proposed\n"})
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root, Tags: []string{" a ", "b", "a"}})
	if got := listAll(t, st)[0].Tags; !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("tags = %q, want [a b]", got)
	}
}

// A superseded ADR is linked only to a target in the same import that ends up
// decided or superseded; otherwise it is imported as decided and the run says
// so, because a superseded record needs the record that replaced it.
func TestImportADRSupersededWithoutAUsableTarget(t *testing.T) {
	older := "# 1. Old\n\nDate: 2020-01-01\n\n## Status\n\n%s\n\n## Decision\n\nOld way.\n\n## Consequences\n\nSlow.\n"
	newer := func(n, status string) string {
		return "# " + n + ". New\n\nDate: 2021-01-01\n\n## Status\n\n" + status + "\n\n## Decision\n\nNew way.\n\n## Consequences\n\nFast.\n"
	}
	tests := []struct {
		name  string
		files map[string]string
		want  string // text of the warning on 0001-old.md
	}{
		{"target not in the folder",
			map[string]string{"0001-old.md": fmt.Sprintf(older, "Superseded by [9. Gone](0009-gone.md)")},
			"0009-gone.md"},
		{"target is only a number nobody has",
			map[string]string{"0001-old.md": fmt.Sprintf(older, "Superseded by ADR-0009")},
			"ADR-0009"},
		{"target is proposed",
			map[string]string{"0001-old.md": fmt.Sprintf(older, "Superseded by [2. New](0002-new.md)"), "0002-new.md": newer("2", "Proposed")},
			"proposed"},
		{"target is rejected",
			map[string]string{"0001-old.md": fmt.Sprintf(older, "Superseded by [2. New](0002-new.md)"), "0002-new.md": newer("2", "Rejected")},
			"rejected"},
		{"no target named",
			map[string]string{"0001-old.md": fmt.Sprintf(older, "Superseded")},
			"no replacement"},
		{"two ADRs have the number",
			map[string]string{"0001-old.md": fmt.Sprintf(older, "Superseded by ADR-0002"), "0002-a.md": newer("2", "Accepted"), "adr-0002-b.md": newer("2", "Accepted")},
			"more than one"},
		{"superseded by itself",
			map[string]string{"0001-old.md": fmt.Sprintf(older, "Superseded by [1. Old](0001-old.md)")},
			"itself"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dir := adrFiles(t, tt.files)
			st := openStore(t, root)
			res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
			old, err := st.Get(1)
			if err != nil {
				t.Fatal(err)
			}
			if old.Status != model.StatusDecided || old.SupersededBy != 0 {
				t.Errorf("record 1 = %s superseded_by %d, want decided with no link", old.Status, old.SupersededBy)
			}
			var got *ImportWarning
			for i, w := range res.Warnings {
				if w.File == "docs/adr/0001-old.md" {
					got = &res.Warnings[i]
				}
			}
			if got == nil || !strings.Contains(got.Message, tt.want) || !strings.Contains(got.Message, "decided") {
				t.Errorf("warnings = %+v, want one on 0001-old.md that mentions %q and says it is imported as decided", res.Warnings, tt.want)
			}
			if findings, err := lint.Run(st, root); err != nil || len(findings) != 0 {
				t.Errorf("lint = %+v, %v; want nothing", findings, err)
			}
		})
	}
}

// Two ADRs that supersede each other would make a supersede cycle; both are
// imported as decided, with a warning each.
// A link that names the file works when the number alone would not: two ADRs
// share the number 2, and ADR 0000 has no number to name.
func TestImportADRLinkByFileNameWhenNumbersClash(t *testing.T) {
	adrText := func(title, status string) string {
		return "# " + title + "\n\nDate: 2020-01-01\n\n## Status\n\n" + status + "\n\n## Decision\n\nD.\n\n## Consequences\n\nC.\n"
	}
	root, dir := adrFiles(t, map[string]string{
		"0001-old.md":   adrText("1. Old", "Superseded by [2. A](0002-a.md)"),
		"0002-a.md":     adrText("2. A", "Accepted"),
		"adr-0002-b.md": adrText("2. B", "Accepted"),
		"0003-older.md": adrText("3. Older", "Superseded by [Zero](0000-zero.md)"),
		"0000-zero.md":  adrText("Zero", "Accepted"),
	})
	st := openStore(t, root)
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
	if len(res.Warnings) != 0 {
		t.Errorf("warnings = %+v, want none", res.Warnings)
	}
	byTitle := map[string]model.Record{}
	for _, r := range listAll(t, st) {
		byTitle[r.Title] = r
	}
	if old, a := byTitle["Old"], byTitle["A"]; old.Status != model.StatusSuperseded || old.SupersededBy != a.ID || !reflect.DeepEqual(a.Supersedes, []int{old.ID}) {
		t.Errorf("Old = %s by %d, A (id %d) supersedes %v; want Old superseded by A", old.Status, old.SupersededBy, a.ID, a.Supersedes)
	}
	if older, zero := byTitle["Older"], byTitle["Zero"]; older.Status != model.StatusSuperseded || older.SupersededBy != zero.ID {
		t.Errorf("Older = %s by %d, Zero is id %d; want Older superseded by Zero", older.Status, older.SupersededBy, zero.ID)
	}
	if b := byTitle["B"]; len(b.Supersedes) != 0 {
		t.Errorf("B supersedes %v, want nothing", b.Supersedes)
	}
}

func TestImportADRSupersedeCycleIsBroken(t *testing.T) {
	text := func(n, other string) string {
		return "# " + n + ". T" + n + "\n\nDate: 2020-01-01\n\n## Status\n\nSuperseded by [" + other + ". T](000" + other + "-t.md)\n\n## Decision\n\nD.\n\n## Consequences\n\nC.\n"
	}
	root, dir := adrFiles(t, map[string]string{"0001-t.md": text("1", "2"), "0002-t.md": text("2", "1")})
	st := openStore(t, root)
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
	for _, r := range listAll(t, st) {
		if r.Status != model.StatusDecided || r.SupersededBy != 0 || len(r.Supersedes) != 0 {
			t.Errorf("record %d = %s superseded_by %d supersedes %v, want decided with no links", r.ID, r.Status, r.SupersededBy, r.Supersedes)
		}
	}
	if len(res.Warnings) != 2 || !strings.Contains(res.Warnings[0].Message, "cycle") {
		t.Errorf("warnings = %+v, want two that mention a cycle", res.Warnings)
	}
	if findings, err := lint.Run(st, root); err != nil || len(findings) != 0 {
		t.Errorf("lint = %+v, %v; want nothing", findings, err)
	}
}

// A chain A <- B <- C: B is superseded and supersedes A, C supersedes B.
func TestImportADRSupersedeChain(t *testing.T) {
	text := func(n, status string) string {
		return "# " + n + ". T" + n + "\n\nDate: 2020-01-0" + n + "\n\n## Status\n\n" + status + "\n\n## Decision\n\nD" + n + ".\n\n## Consequences\n\nC.\n"
	}
	root, dir := adrFiles(t, map[string]string{
		"0001-a.md": text("1", "Superseded by [2. B](0002-b.md)"),
		"0002-b.md": text("2", "Superseded by [3. C](0003-c.md)"),
		"0003-c.md": text("3", "Accepted"),
	})
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
	recs := listAll(t, st)
	type links struct {
		status model.Status
		by     int
		sup    []int
	}
	want := []links{{model.StatusSuperseded, 2, nil}, {model.StatusSuperseded, 3, []int{1}}, {model.StatusDecided, 0, []int{2}}}
	for i, r := range recs {
		if got := (links{r.Status, r.SupersededBy, r.Supersedes}); !reflect.DeepEqual(got, want[i]) {
			t.Errorf("record %d = %+v, want %+v", r.ID, got, want[i])
		}
	}
	if findings, err := lint.Run(st, root); err != nil || len(findings) != 0 {
		t.Errorf("lint = %+v, %v; want nothing", findings, err)
	}
}

// Several ADRs may be replaced by one: the new record lists them all.
func TestImportADRSeveralADRsReplacedByOne(t *testing.T) {
	old := func(n string) string {
		return "# " + n + ". Old" + n + "\n\nDate: 2020-01-01\n\n## Status\n\nSuperseded by ADR-0003\n\n## Decision\n\nD.\n\n## Consequences\n\nC.\n"
	}
	root, dir := adrFiles(t, map[string]string{
		"0001-a.md": old("1"), "0002-b.md": old("2"),
		"0003-c.md": "# 3. New\n\nDate: 2021-01-01\n\n## Status\n\nAccepted\n\n## Decision\n\nD.\n\n## Consequences\n\nC.\n",
	})
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
	c, err := st.Get(3)
	if err != nil || !reflect.DeepEqual(c.Supersedes, []int{1, 2}) {
		t.Errorf("record 3 = %+v, %v; want supersedes [1 2]", c, err)
	}
}

func TestImportADRUnparseableDateUsesTheRunDate(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nDate: last spring\n\nStatus: proposed\n"})
	st := openStore(t, root)
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	if r := listAll(t, st)[0]; r.Created != importDate {
		t.Errorf("created = %q, want %q", r.Created, importDate)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Message, `"last spring"`) {
		t.Errorf("warnings = %+v, want one that quotes the date as written", res.Warnings)
	}
}

// The rationale is the first of the justification, the consequences and the
// context that the ADR has; rejected and withdrawn records also try the
// decision text first because a "we will not" is the reason.
func TestImportADRRationaleFallbacks(t *testing.T) {
	tests := []struct {
		name, text, wantRationale, wantConsequences string
	}{
		{"consequences", "## Decision\n\nDo it.\n\n## Consequences\n\nIt costs.\n\n## Context\n\nWhy.\n", "It costs.", ""},
		{"context", "## Decision\n\nDo it.\n\n## Context\n\nWhy.\n", "Why.", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nDate: 2020-01-01\n\n## Status\n\nAccepted\n\n" + tt.text})
			st := openStore(t, root)
			importOK(t, st, ImportADRInput{Dir: dir, Root: root})
			r := listAll(t, st)[0]
			if r.Rationale != tt.wantRationale || r.Consequences != tt.wantConsequences {
				t.Errorf("rationale %q, consequences %q; want %q, %q", r.Rationale, r.Consequences, tt.wantRationale, tt.wantConsequences)
			}
		})
	}
}

// A store that cannot take a record leaves the earlier records in place and
// the error names the file that failed; the result lists what was done.
type failingCreate struct {
	adrStore
	after int
	n     int
}

func (f *failingCreate) Create(r model.Record) (model.Record, error) {
	if f.n++; f.n > f.after {
		return model.Record{}, errors.New("disk full")
	}
	return f.adrStore.Create(r)
}

func TestImportADRReportsWhatWasWrittenBeforeAFailure(t *testing.T) {
	root, dir := adrRoot(t, "nygard")
	real := openStore(t, root)
	res, err := importADR(&failingCreate{adrStore: real, after: 2}, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"}, importNow)
	if err == nil || !strings.Contains(err.Error(), "docs/adr/0003-adopt-trunk-based-development.md") || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("error = %v, want one that names the third file and the cause", err)
	}
	if len(res.ADRs) != 2 || res.ADRs[1].File != "docs/adr/0002-use-postgresql.md" || res.ADRs[1].Action != ImportCreated {
		t.Errorf("result = %+v, want the two created ADRs", res.ADRs)
	}
	if got := len(listAll(t, real)); got != 2 {
		t.Errorf("store has %d records, want 2", got)
	}
}

// If a superseded record cannot be written with its link, the run stops with
// an error that says the link is missing; the replacing record is not linked
// to a record that is not superseded.
type failingUpdate struct {
	adrStore
	failID int
}

func (f *failingUpdate) Update(r model.Record) error {
	if r.ID == f.failID {
		return errors.New("read-only")
	}
	return f.adrStore.Update(r)
}

func TestImportADRStopsIfALinkCannotBeWritten(t *testing.T) {
	root, dir := adrRoot(t, "nygard")
	real := openStore(t, root)
	_, err := importADR(&failingUpdate{adrStore: real, failID: 2}, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"}, importNow)
	if err == nil || !strings.Contains(err.Error(), "docs/adr/0002-use-postgresql.md") || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("error = %v, want one that names the file and the cause", err)
	}
	four, e := real.Get(4)
	if e != nil || len(four.Supersedes) != 0 {
		t.Errorf("record 4 = %+v, %v; want no supersedes link when record 2 could not be written", four, e)
	}
}

// ---- Fix round 1 ----

// A file counts as imported only if a record has an import history entry
// "from FILE": a record someone asked by hand with the file as a ref is not an
// import, and the ADR is imported as a new record.
func TestImportADRAskedRecordWithTheFileRefIsNotAnImport(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nStatus: proposed\n"})
	st := openStore(t, root)
	asked, err := Ask(st, AskInput{Title: "Asked by hand", Refs: []string{"docs/adr/0001-x.md"}}, importNow)
	if err != nil {
		t.Fatal(err)
	}
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	if len(res.ADRs) != 1 || res.ADRs[0].Action != ImportCreated || res.ADRs[0].ID != asked.ID+1 {
		t.Fatalf("result = %+v, want the ADR created as id %d", res.ADRs, asked.ID+1)
	}
	if got := listAll(t, st); len(got) != 2 || got[0].Title != "Asked by hand" || got[1].Title != "X" {
		t.Errorf("records = %+v, want the asked record and the imported one", got)
	}
	// And now that it is imported, a run skips it.
	if res := importOK(t, st, ImportADRInput{Dir: dir, Root: root}); res.ADRs[0].Action != ImportSkipped || res.ADRs[0].ID != 2 {
		t.Errorf("second run = %+v, want skipped as id 2", res.ADRs)
	}
}

func plainADR(n, title, status string) string {
	return "# " + n + ". " + title + "\n\nDate: 2020-01-01\n\n## Status\n\n" + status + "\n\n## Decision\n\nD" + n + ".\n\n## Consequences\n\nC" + n + ".\n"
}

// If the run stops after some records were created, the error lists each link
// not yet written as the adj supersede command that makes it, and says which
// links cannot be named yet.
func TestImportADRInterruptedRunListsTheLinksNotWritten(t *testing.T) {
	files := map[string]string{
		"0001-a.md": plainADR("1", "A", "Superseded by [2. B](0002-b.md)"),
		"0002-b.md": plainADR("2", "B", "Accepted"),
		"0003-c.md": plainADR("3", "C", "Superseded by [4. D](0004-d.md)"),
		"0004-d.md": plainADR("4", "D", "Accepted"),
	}
	t.Run("a record cannot be created", func(t *testing.T) {
		root, dir := adrFiles(t, files)
		real := openStore(t, root)
		_, err := importADR(&failingCreate{adrStore: real, after: 3}, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"}, importNow)
		if err == nil {
			t.Fatal("want an error")
		}
		for _, want := range []string{"docs/adr/0004-d.md", "disk full", "adj supersede 1 --by 2", "docs/adr/0003-c.md", "was not created"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not contain %q:\n%v", want, err)
			}
		}
		if strings.Contains(err.Error(), "adj supersede 3 --by") {
			t.Errorf("error names a supersede command for a record that does not exist:\n%v", err)
		}
	})
	t.Run("a superseded record cannot be written", func(t *testing.T) {
		root, dir := adrFiles(t, files)
		real := openStore(t, root)
		_, err := importADR(&failingUpdate{adrStore: real, failID: 1}, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"}, importNow)
		if err == nil {
			t.Fatal("want an error")
		}
		for _, want := range []string{"docs/adr/0001-a.md", "read-only", "adj supersede 1 --by 2", "adj supersede 3 --by 4"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not contain %q:\n%v", want, err)
			}
		}
		for _, r := range listAll(t, real) {
			if r.Status == model.StatusSuperseded {
				t.Errorf("record %d is superseded, want none written", r.ID)
			}
		}
	})
}

// The link is written old record first and never rolled back: if the new
// record then cannot be written, the old one stays superseded, the decision in
// force is still the new record, and the error says how to finish.
func TestImportADRNewRecordCannotBeWrittenLeavesTheOldOneSuperseded(t *testing.T) {
	files := map[string]string{
		"0001-a.md": plainADR("1", "A", "Superseded by [2. B](0002-b.md)"),
		"0002-b.md": plainADR("2", "B", "Accepted"),
		"0003-c.md": plainADR("3", "C", "Superseded by [4. D](0004-d.md)"),
		"0004-d.md": plainADR("4", "D", "Accepted"),
	}
	root, dir := adrFiles(t, files)
	real := openStore(t, root)
	_, err := importADR(&failingUpdate{adrStore: real, failID: 4}, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"}, importNow)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"docs/adr/0004-d.md", "read-only", "record 3 is left superseded by record 4", "no link back", "decision in force is still record 4"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	// The half-written link cannot be made with adj supersede (the old record
	// is superseded already), so it is not offered as a command.
	if strings.Contains(err.Error(), "adj supersede") {
		t.Errorf("error offers an adj supersede command for a link that is half written:\n%v", err)
	}
	// The first link was written whole; the second is half written.
	one, _ := real.Get(1)
	two, _ := real.Get(2)
	three, _ := real.Get(3)
	four, _ := real.Get(4)
	if one.Status != model.StatusSuperseded || one.SupersededBy != 2 || !reflect.DeepEqual(two.Supersedes, []int{1}) {
		t.Errorf("first link: record 1 = %s by %d, record 2 supersedes %v; want written", one.Status, one.SupersededBy, two.Supersedes)
	}
	if three.Status != model.StatusSuperseded || three.SupersededBy != 4 || len(four.Supersedes) != 0 {
		t.Errorf("second link: record 3 = %s by %d, record 4 supersedes %v; want 3 left superseded and 4 without the link", three.Status, three.SupersededBy, four.Supersedes)
	}
}

// An ADR file that says it is superseded, whose record is still decided
// because the file was edited after the import (adr-tools' `adr new -s`), gets
// a warning with the adj supersede command; nothing is written.
func TestImportADRRerunWarnsAboutALinkTheFilesNowHave(t *testing.T) {
	const old = "docs/adr/0001-old.md"
	oldFile := func(status string) string { return plainADR("1", "Old", status) }
	t.Run("the replacement is new in this run", func(t *testing.T) {
		root, dir := adrFiles(t, map[string]string{"0001-old.md": oldFile("Accepted")})
		st := openStore(t, root)
		importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		write(t, dir, "0001-old.md", oldFile("Superseded by [2. New](0002-new.md)"))
		write(t, dir, "0002-new.md", plainADR("2", "New", "Accepted\n\nSupersedes [1. Old](0001-old.md)"))
		before := listAll(t, st)
		res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		if len(res.Warnings) != 1 || res.Warnings[0].File != old || !strings.Contains(res.Warnings[0].Message, "adj supersede 1 --by 2") {
			t.Errorf("warnings = %+v, want one on %s with `adj supersede 1 --by 2`", res.Warnings, old)
		}
		after := listAll(t, st)
		if after[0].Status != model.StatusDecided || after[0].SupersededBy != 0 || len(after[1].Supersedes) != 0 || !reflect.DeepEqual(after[0], before[0]) {
			t.Errorf("the skipped record was changed: %+v", after[0])
		}
	})
	t.Run("both records were imported already", func(t *testing.T) {
		root, dir := adrFiles(t, map[string]string{"0001-old.md": oldFile("Accepted"), "0002-new.md": plainADR("2", "New", "Accepted")})
		st := openStore(t, root)
		importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		write(t, dir, "0001-old.md", oldFile("Superseded by ADR-0002"))
		res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		if len(res.Warnings) != 1 || res.Warnings[0].File != old || !strings.Contains(res.Warnings[0].Message, "adj supersede 1 --by 2") {
			t.Errorf("warnings = %+v, want one on %s with `adj supersede 1 --by 2`", res.Warnings, old)
		}
	})
	t.Run("a dry run names the file instead of an ID", func(t *testing.T) {
		root, dir := adrFiles(t, map[string]string{"0001-old.md": oldFile("Accepted")})
		st := openStore(t, root)
		importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		write(t, dir, "0001-old.md", oldFile("Superseded by [2. New](0002-new.md)"))
		write(t, dir, "0002-new.md", plainADR("2", "New", "Accepted"))
		res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DryRun: true})
		if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Message, "adj supersede 1 --by") || !strings.Contains(res.Warnings[0].Message, "docs/adr/0002-new.md") {
			t.Errorf("warnings = %+v, want the command and the new file", res.Warnings)
		}
	})
	t.Run("no warning when the replacement is new and not decided", func(t *testing.T) {
		root, dir := adrFiles(t, map[string]string{"0001-old.md": oldFile("Accepted")})
		st := openStore(t, root)
		importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		write(t, dir, "0001-old.md", oldFile("Superseded by [2. New](0002-new.md)"))
		write(t, dir, "0002-new.md", plainADR("2", "New", "Proposed"))
		if res := importOK(t, st, ImportADRInput{Dir: dir, Root: root}); len(res.Warnings) != 0 {
			t.Errorf("warnings = %+v, want none", res.Warnings)
		}
	})
	t.Run("no warning when the replacement record is not decided", func(t *testing.T) {
		root, dir := adrFiles(t, map[string]string{"0001-old.md": oldFile("Accepted"), "0002-new.md": plainADR("2", "New", "Proposed")})
		st := openStore(t, root)
		importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		write(t, dir, "0001-old.md", oldFile("Superseded by [2. New](0002-new.md)"))
		if res := importOK(t, st, ImportADRInput{Dir: dir, Root: root}); len(res.Warnings) != 0 {
			t.Errorf("warnings = %+v, want none", res.Warnings)
		}
	})
	t.Run("no warning when the record is superseded already", func(t *testing.T) {
		root, dir := adrFiles(t, map[string]string{"0001-old.md": oldFile("Superseded by [2. New](0002-new.md)"), "0002-new.md": plainADR("2", "New", "Accepted")})
		st := openStore(t, root)
		importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		if res := importOK(t, st, ImportADRInput{Dir: dir, Root: root}); len(res.Warnings) != 0 {
			t.Errorf("warnings = %+v, want none", res.Warnings)
		}
	})
	t.Run("an imported file that no longer parses does not fail the run", func(t *testing.T) {
		root, dir := adrFiles(t, map[string]string{"0001-old.md": oldFile("Accepted")})
		st := openStore(t, root)
		importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		write(t, dir, "0001-old.md", "not an ADR any more\n")
		write(t, dir, "0002-new.md", plainADR("2", "New", "Accepted"))
		res := importOK(t, st, ImportADRInput{Dir: dir, Root: root, DecidedBy: "t"})
		if len(res.ADRs) != 2 || res.ADRs[0].Action != ImportSkipped || res.ADRs[1].Action != ImportCreated || len(res.Warnings) != 0 {
			t.Errorf("result = %+v, warnings %+v; want 0001 skipped, 0002 created, no warning", res.ADRs, res.Warnings)
		}
	})
}

func write(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A date after the run date is no more believable than one that is not a date:
// the run date is used, with a warning. A well-shaped but impossible date is
// called that.
func TestImportADRDatesThatCannotBeUsed(t *testing.T) {
	tests := []struct {
		date, want string
	}{
		{"2026-03-05", "after the run date"},
		{"2999-01-01", "after the run date"},
		{"2024-02-30", "not a real date"},
		{"2024-13-01", "not a real date"},
		{"last spring", "not YYYY-MM-DD"},
	}
	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nDate: " + tt.date + "\n\nStatus: proposed\n"})
			st := openStore(t, root)
			res := importOK(t, st, ImportADRInput{Dir: dir, Root: root})
			if r := listAll(t, st)[0]; r.Created != importDate {
				t.Errorf("created = %q, want the run date", r.Created)
			}
			if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0].Message, tt.want) || !strings.Contains(res.Warnings[0].Message, `"`+tt.date+`"`) ||
				!strings.Contains(res.Warnings[0].Message, importDate) {
				t.Errorf("warnings = %+v, want one that quotes %q, says %q and names the run date", res.Warnings, tt.date, tt.want)
			}
		})
	}
	// The run date itself is fine.
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nDate: " + importDate + "\n\nStatus: proposed\n"})
	if res := importOK(t, openStore(t, root), ImportADRInput{Dir: dir, Root: root}); len(res.Warnings) != 0 {
		t.Errorf("warnings for the run date = %+v, want none", res.Warnings)
	}
}

// An ADR file that is a symbolic link is not read; it is reported.
func TestImportADRWarnsAboutSymbolicLinks(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "# 1. X\n\nDate: 2020-01-01\n\nStatus: proposed\n"})
	if err := os.Symlink("0001-x.md", filepath.Join(dir, "0002-link.md")); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	if err := os.Symlink("0001-x.md", filepath.Join(dir, "notes.md")); err != nil { // not an ADR name: silent
		t.Fatal(err)
	}
	st := openStore(t, root)
	res := importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	if len(res.ADRs) != 1 || len(listAll(t, st)) != 1 {
		t.Errorf("result = %+v, want only 0001-x.md imported", res.ADRs)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].File != "docs/adr/0002-link.md" || res.Warnings[0].Message != "symbolic link skipped" {
		t.Errorf("warnings = %+v, want one: docs/adr/0002-link.md: symbolic link skipped", res.Warnings)
	}
}

// The record is valid by construction: build checks everything Validate does,
// so Create cannot fail halfway through an import for a bad record. Every
// record built from the fixtures and from awkward text validates.
type validatingStore struct {
	t    *testing.T
	recs []model.Record
}

func (s *validatingStore) List() ([]model.Record, error) { return nil, nil }
func (s *validatingStore) Update(model.Record) error     { return nil }
func (s *validatingStore) Create(r model.Record) (model.Record, error) {
	s.t.Helper()
	r.ID = len(s.recs) + 1
	if err := r.Validate(); err != nil {
		s.t.Errorf("built record %q is invalid: %v", r.Title, err)
	}
	s.recs = append(s.recs, r)
	return r, nil
}

func TestImportADRBuiltRecordsAreValid(t *testing.T) {
	awkward := map[string]string{
		"0001-blank-decision.md":   "# 1. T\n\nStatus: proposed\n\n## Decision\n\n   \n\n## Considered Options\n\n* a\n",
		"0002-unicode.md":          "# 2. T\u2028x \u00e9\n\nStatus: accepted\n\n## Decision\n\n\u00e9\n\n## Context\n\nc\n",
		"0003-label-not-option.md": "# 3. T\n\nStatus: proposed\n\n## Considered Options\n\n* a\n\n## Decision Outcome\n\nChosen option: \"b\", because x\n",
		"0004-year-zero.md":        "# 4. T\n\nDate: 0001-01-01\n\nStatus: accepted\n\n## Decision\n\nd\n\n## Context\n\nc\n",
		"0005-both.md":             "# 5. T\n\nStatus: rejected\n\n## Context\n\nc\n",
		"0006-title-markup.md":     "# 6. **T** `x` <!-- c -->\n\nStatus: deprecated\n\n## Decision\n\nd\n",
	}
	for _, fixture := range []string{"nygard", "madr"} {
		root, dir := adrRoot(t, fixture)
		st := &validatingStore{t: t}
		if _, err := importADR(st, ImportADRInput{Dir: dir, Root: root}, importNow); err != nil {
			t.Fatalf("%s: %v", fixture, err)
		}
		if len(st.recs) == 0 {
			t.Errorf("%s: no record was built", fixture)
		}
	}
	root, dir := adrFiles(t, awkward)
	st := &validatingStore{t: t}
	if _, err := importADR(st, ImportADRInput{Dir: dir, Root: root}, importNow); err != nil {
		t.Fatal(err)
	}
	if len(st.recs) != len(awkward) {
		t.Errorf("built %d records, want %d", len(st.recs), len(awkward))
	}
}

// A MADR chosen option written as a link names the option by its text.
func TestImportADRChosenOptionLinkIsTheOptionLabel(t *testing.T) {
	root, dir := adrFiles(t, map[string]string{"0001-x.md": "---\nstatus: proposed\n---\n# X\n\n## Considered Options\n\n* [Postgres](https://pg.example/a,b)\n* MySQL\n\n" +
		"## Decision Outcome\n\nChosen option: [Postgres](https://pg.example/a,b), because it is boring.\n"})
	st := openStore(t, root)
	importOK(t, st, ImportADRInput{Dir: dir, Root: root})
	rec := listAll(t, st)[0]
	want := &model.Recommendation{Label: "Postgres", Reasoning: "it is boring."}
	if !reflect.DeepEqual(rec.Recommendation, want) {
		t.Errorf("recommendation = %+v, want %+v", rec.Recommendation, want)
	}
}
