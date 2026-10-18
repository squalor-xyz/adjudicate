// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// adrFixture copies the ADR fixture folder internal/adr/testdata/<name> into
// <root>/docs/adr and returns that folder.
func adrFixture(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join("..", "adr", "testdata", name)
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
	return dir
}

const nygardImportOut = `docs/adr/0001-record-architecture-decisions.md -> 1 (decided)
docs/adr/0002-use-postgresql.md -> 2 (superseded)
docs/adr/0003-adopt-trunk-based-development.md -> 3 (open)
docs/adr/0004-use-cockroachdb.md -> 4 (decided)
docs/adr/0005-use-mongodb.md -> 5 (rejected)
docs/adr/0006-keep-subversion.md -> 6 (withdrawn)
docs/adr/0007-ship-weekly.md -> 7 (decided)
docs/adr/0008-use-semantic-versioning-crlf.md -> 8 (decided)
`

// The whole flow: import the adr-tools folder, read the result back with the
// read commands, lint it, import again.
func TestImportADRImportsAFolderAndLintIsCleanAfter(t *testing.T) {
	root := initRoot(t)
	dir := adrFixture(t, root, "nygard")

	r := run(testEnv(root), "", "import-adr", dir, "--by", "tester", "--tag", "adr")
	if r.code != 0 || r.stdout != nygardImportOut {
		t.Fatalf("exit %d, stdout\n%s\nwant exit 0 and\n%s", r.code, r.stdout, nygardImportOut)
	}
	// The one ADR with no date is reported, on stderr.
	wantErr := "adj import-adr: warning: docs/adr/0007-ship-weekly.md: no date; using the run date 2026-03-04\n"
	if r.stderr != wantErr {
		t.Errorf("stderr = %q, want %q", r.stderr, wantErr)
	}

	if l := run(testEnv(root), "", "lint"); l.code != 0 || l.stdout != "" {
		t.Errorf("lint: exit %d, stdout %q, stderr %q; want a clean store", l.code, l.stdout, l.stderr)
	}
	// The columns are aligned; compare the fields.
	list := run(testEnv(root), "", "list", "--tag", "adr")
	wantList := []string{"1 decided Record architecture decisions", "2 superseded Use PostgreSQL", "3 open Adopt trunk-based development",
		"4 decided Use CockroachDB", "5 rejected Use MongoDB", "6 withdrawn Keep Subversion", "7 decided Ship weekly", "8 decided Use semantic versioning"}
	var gotList []string
	for _, line := range strings.Split(strings.TrimSuffix(list.stdout, "\n"), "\n") {
		gotList = append(gotList, strings.Join(strings.Fields(line), " "))
	}
	if list.code != 0 || !reflect.DeepEqual(gotList, wantList) {
		t.Errorf("list: exit %d, rows %q; want %q", list.code, gotList, wantList)
	}
	show := run(testEnv(root), "", "show", "2")
	for _, want := range []string{"superseded", "Superseded by: 4", "ADR-0002", "docs/adr/0002-use-postgresql.md", "import", "from docs/adr/0002-use-postgresql.md"} {
		if !strings.Contains(show.stdout, want) {
			t.Errorf("show 2 does not contain %q:\n%s", want, show.stdout)
		}
	}
	// The old decision is out of the context; its replacement is in.
	ctx := run(testEnv(root), "", "context")
	if strings.Contains(ctx.stdout, "Use PostgreSQL") || !strings.Contains(ctx.stdout, "Use CockroachDB") {
		t.Errorf("context should hold the replacement and not the superseded record:\n%s", ctx.stdout)
	}
	// check finds an imported decision by the ADR number kept as a ref.
	if c := run(testEnv(root), "", "check", "ADR-0004"); !strings.HasPrefix(c.stdout, "decided\n") {
		t.Errorf("check ADR-0004:\n%s", c.stdout)
	}

	before := snapshot(t, root)
	r = run(testEnv(root), "", "import-adr", dir, "--by", "tester")
	var wantSkip strings.Builder
	for i, line := range strings.Split(strings.TrimSuffix(nygardImportOut, "\n"), "\n") {
		file, _, _ := strings.Cut(line, " -> ")
		wantSkip.WriteString(file + ": already imported as " + string(rune('1'+i)) + "\n")
	}
	if r.code != 0 || r.stdout != wantSkip.String() || r.stderr != "" {
		t.Errorf("second import: exit %d, stdout\n%s\nstderr %q; want exit 0 and\n%s", r.code, r.stdout, r.stderr, wantSkip.String())
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("the second import changed the tree")
	}
}

func TestImportADRMADRFolder(t *testing.T) {
	root := initRoot(t)
	dir := adrFixture(t, root, "madr")
	r := run(testEnv(root), "", "import-adr", dir)
	want := `docs/adr/0001-use-markdown-architectural-decision-records.md -> 1 (decided)
docs/adr/0002-use-plain-junit5-for-testing.md -> 2 (decided)
docs/adr/0003-use-spring-boot.md -> 3 (superseded)
docs/adr/0004-evaluate-graalvm.md -> 4 (open)
docs/adr/0005-use-quarkus.md -> 5 (decided)
docs/adr/0006-use-xml-configuration.md -> 6 (rejected)
docs/adr/0007-keep-java-8.md -> 7 (withdrawn)
`
	if r.code != 0 || r.stdout != want || r.stderr != "" {
		t.Errorf("exit %d, stdout\n%s\nstderr %q; want exit 0 and\n%s", r.code, r.stdout, r.stderr, want)
	}
	// The deciders of the ADR are decided_by; the open ADR shows in the inbox with its options.
	if s := run(testEnv(root), "", "show", "1"); !strings.Contains(s.stdout, "Alice Example, Bob Example") {
		t.Errorf("show 1 does not name the deciders:\n%s", s.stdout)
	}
	inbox := run(testEnv(root), "", "inbox")
	for _, want := range []string{"Evaluate GraalVM native images", "Native images with GraalVM", "Keep the JVM"} {
		if !strings.Contains(inbox.stdout, want) {
			t.Errorf("inbox does not contain %q:\n%s", want, inbox.stdout)
		}
	}
}

func TestImportADRDryRun(t *testing.T) {
	root := initRoot(t)
	dir := adrFixture(t, root, "nygard")
	before := snapshot(t, root)
	r := run(testEnv(root), "", "import-adr", dir, "--dry-run")
	want := strings.NewReplacer(" -> 1 (", " -> would create (", " -> 2 (", " -> would create (", " -> 3 (", " -> would create (",
		" -> 4 (", " -> would create (", " -> 5 (", " -> would create (", " -> 6 (", " -> would create (",
		" -> 7 (", " -> would create (", " -> 8 (", " -> would create (").Replace(nygardImportOut)
	if r.code != 0 || r.stdout != want {
		t.Errorf("exit %d, stdout\n%s\nwant exit 0 and\n%s", r.code, r.stdout, want)
	}
	if !strings.Contains(r.stderr, "warning: docs/adr/0007-ship-weekly.md: no date") {
		t.Errorf("a dry run should report the warnings; stderr:\n%s", r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("--dry-run changed the tree")
	}
}

// decided_by: --by, else the ADR's deciders, else $ADJ_DECIDED_BY, else blank.
func TestImportADRDecidedByOrder(t *testing.T) {
	// The MADR fixture: record 1 has deciders ("Alice Example, Bob Example"),
	// record 3 has none. The nygard fixture has no deciders at all.
	tests := []struct {
		name, fixture string
		args          []string
		env           string
		id            int
		want          string
	}{
		{"--by beats the deciders and the environment", "madr", []string{"--by", "flag"}, "env", 1, "flag"},
		{"--by beats the environment", "madr", []string{"--by", "flag"}, "env", 3, "flag"},
		{"the deciders beat the environment", "madr", nil, "env", 1, "Alice Example, Bob Example"},
		{"the environment where the ADR has no deciders", "madr", nil, "env", 3, "env"},
		{"the environment for an adr-tools ADR", "nygard", nil, "env", 1, "env"},
		{"the deciders when no environment", "madr", nil, "", 1, "Alice Example, Bob Example"},
		{"blank when nothing names anyone", "madr", nil, "", 3, ""},
		{"blank for an adr-tools ADR when nothing names anyone", "nygard", nil, "", 1, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			dir := adrFixture(t, root, tt.fixture)
			env := testEnv(root)
			env.Getenv = func(k string) string {
				if k == "ADJ_DECIDED_BY" {
					return tt.env
				}
				return ""
			}
			if r := run(env, "", append([]string{"import-adr", dir}, tt.args...)...); r.code != 0 {
				t.Fatalf("exit %d: %s", r.code, r.stderr)
			}
			if got := recordByID(t, root, tt.id).DecidedBy; got != tt.want {
				t.Errorf("decided_by of record %d = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}

// A relative folder is relative to the working directory, and --root may come
// before the command.
func TestImportADRRelativeFolderAndLeadingRoot(t *testing.T) {
	root := initRoot(t)
	adrFixture(t, root, "madr")
	env := testEnv(root)
	env.Getwd = func() (string, error) { return filepath.Join(root, "docs"), nil }
	r := run(env, "", "--root", root, "import-adr", "adr")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "docs/adr/0001-use-markdown-architectural-decision-records.md -> 1 (decided)\n") {
		t.Errorf("exit %d, stdout\n%s\nstderr %s", r.code, r.stdout, r.stderr)
	}
}

func TestImportADRBadFileWritesNothing(t *testing.T) {
	root := initRoot(t)
	dir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"0001-good.md": "# 1. Good\n\nStatus: proposed\n",
		"0002-bad.md":  "# 2. Bad\n\nDate: 2020-01-01\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "", "import-adr", dir)
	if r.code != 1 || r.stdout != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and no stdout", r.code, r.stdout)
	}
	if !strings.Contains(r.stderr, "adj import-adr:") || !strings.Contains(r.stderr, "docs/adr/0002-bad.md: no status") || strings.Contains(r.stderr, "0001-good.md") {
		t.Errorf("stderr should name the bad file and its problem only:\n%s", r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("the tree changed")
	}
}

// If a record cannot be written after others were, what was done is printed
// before the error, so the owner knows what changed; the exit code is 1.
func TestImportADRPrintsWhatWasDoneBeforeAFailure(t *testing.T) {
	root := initRoot(t)
	dir := adrFixture(t, root, "nygard")
	// A folder where record 2's file would go: the store cannot take that name.
	if err := os.Mkdir(filepath.Join(root, "decisions", "0002-use-postgresql.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := run(testEnv(root), "", "import-adr", dir)
	if r.code != 1 || r.stdout != "docs/adr/0001-record-architecture-decisions.md -> 1 (decided)\n" {
		t.Errorf("exit %d, stdout %q; want exit 1 and the line for the first record only", r.code, r.stdout)
	}
	if !strings.Contains(r.stderr, "docs/adr/0002-use-postgresql.md") {
		t.Errorf("stderr should name the file that failed:\n%s", r.stderr)
	}
	if got := recordByID(t, root, 1); got.Title != "Record architecture decisions" {
		t.Errorf("record 1 = %+v", got)
	}
}

func TestImportADRFailures(t *testing.T) {
	root := initRoot(t)
	adrFixture(t, root, "nygard")
	outside := t.TempDir()
	tests := []struct {
		name    string
		env     Env
		args    []string
		wantErr string
	}{
		{"folder does not exist", testEnv(root), []string{"import-adr", filepath.Join(root, "nope")}, "nope"},
		{"folder outside the repo", testEnv(root), []string{"import-adr", outside}, "outside the repo root"},
		{"no repo", testEnv(t.TempDir()), []string{"import-adr", outside}, "adj init"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, root)
			r := run(tt.env, "", tt.args...)
			if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit 1, no stdout and %q", r.code, r.stdout, r.stderr, tt.wantErr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("the tree changed")
			}
		})
	}
}

// A folder with no ADR file is not an error and prints nothing.
func TestImportADRFolderWithoutADRs(t *testing.T) {
	root := initRoot(t)
	dir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# ADRs\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "", "import-adr", dir)
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 0 and nothing printed", r.code, r.stdout, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("the tree changed")
	}
}

func TestImportADRUsageErrors(t *testing.T) {
	root := initRoot(t)
	dir := adrFixture(t, root, "nygard")
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no folder", []string{"import-adr"}, "DIR is required"},
		{"two folders", []string{"import-adr", dir, dir}, "one DIR"},
		{"blank folder", []string{"import-adr", "  "}, "DIR must not be blank"},
		{"blank tag", []string{"import-adr", dir, "--tag", " "}, "tag must not be blank"},
		{"unknown flag", []string{"import-adr", dir, "--nope"}, "nope"},
		{"--root twice", []string{"--root", root, "import-adr", dir, "--root", root}, "--root given twice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj import-adr") {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit 2 with %q and the usage", r.code, r.stdout, r.stderr, tt.wantErr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("the tree changed")
			}
		})
	}
}

func TestImportADRHelp(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "import-adr", "--help")
	if r.code != 0 || r.stderr != "" || !strings.HasPrefix(r.stdout, "usage: adj import-adr DIR") {
		t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	for _, want := range []string{"--tag TAG", "a tag for every record", "--by NAME", "ADJ_DECIDED_BY", "--dry-run      write nothing", "--root DIR", "Nygard", "MADR"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("help does not mention %q:\n%s", want, r.stdout)
		}
	}
	// The rules that the text wraps across lines, compared without the wraps:
	// the rationale order, the supersede target, a failure partway, symbolic
	// links, dates and the warning on a second run.
	flat := strings.Join(strings.Fields(r.stdout), " ")
	for _, want := range []string{
		"The rationale is the MADR justification, else the Consequences section, else the Context",
		"for a rejected or withdrawn ADR the justification, then the Decision text, then the Consequences, then the Context",
		"when the ADR that replaced it is in the same run and is accepted or itself superseded",
		"the records created so far stay in the store and are printed",
		"symbolic link",
		"after the run date",
		"the command to run, adj supersede OLD --by NEW",
		"has an import history entry",
		"--by NAME decided_by of the answered records; default: the ADR's deciders field (MADR), else $ADJ_DECIDED_BY, else left blank",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("help does not say %q:\n%s", want, r.stdout)
		}
	}
}

func TestImportADRListedInTheMainUsage(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(r.stdout, "adj import-adr ") {
		t.Errorf("main usage does not list adj import-adr:\n%s", r.stdout)
	}
}

// Import is the owner's: the agent snippet does not name it (it is at its size
// limit, and an agent has no use for it).
func TestImportADRIsNotInTheAgentSnippet(t *testing.T) {
	if r := run(testEnv(t.TempDir()), "", "ai"); strings.Contains(r.stdout, "import") {
		t.Errorf("the snippet names import:\n%s", r.stdout)
	}
}

// Both the JSON of a show and docs/json.md know the import event.
func TestImportADREventIsInTheJSONDocs(t *testing.T) {
	doc := docsText(t)
	if !strings.Contains(doc, "`import`") || !strings.Contains(doc, "adj import-adr") {
		t.Error("docs/json.md does not name the import event and the command that writes it")
	}
	root := initRoot(t)
	dir := adrFixture(t, root, "madr")
	if r := run(testEnv(root), "", "import-adr", dir); r.code != 0 {
		t.Fatalf("import: %s", r.stderr)
	}
	r := run(testEnv(root), "", "show", "1", "--json")
	if !strings.Contains(r.stdout, `"event": "import"`) || !strings.Contains(r.stdout, `"note": "from docs/adr/0001-use-markdown-architectural-decision-records.md"`) {
		t.Errorf("show --json lacks the import history entry:\n%s", r.stdout)
	}
}

// A record someone asked with the file as a ref is not an import of it.
func TestImportADRAskedRecordWithTheFileRefDoesNotSkipTheFile(t *testing.T) {
	root := initRoot(t)
	dir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001-x.md"), []byte("# 1. X\n\nDate: 2020-01-01\n\nStatus: proposed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := run(testEnv(root), "", "ask", "By hand", "--ref", "docs/adr/0001-x.md"); r.code != 0 {
		t.Fatalf("ask: %s", r.stderr)
	}
	r := run(testEnv(root), "", "import-adr", dir)
	if r.code != 0 || r.stdout != "docs/adr/0001-x.md -> 2 (open)\n" {
		t.Errorf("exit %d, stdout %q; want the ADR created as record 2", r.code, r.stdout)
	}
}

// After an ADR file was edited to say it is superseded, a second import says
// which command links the records (it writes nothing).
func TestImportADRRerunPrintsTheSupersedeCommand(t *testing.T) {
	root := initRoot(t)
	dir := filepath.Join(root, "docs", "adr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := func(n, status string) []byte {
		return []byte("# " + n + ". T" + n + "\n\nDate: 2020-01-01\n\n## Status\n\n" + status + "\n\n## Decision\n\nD.\n\n## Consequences\n\nC.\n")
	}
	for name, data := range map[string][]byte{"0001-a.md": text("1", "Accepted"), "0002-b.md": text("2", "Accepted")} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if r := run(testEnv(root), "", "import-adr", dir, "--by", "o"); r.code != 0 {
		t.Fatalf("first import: %s", r.stderr)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001-a.md"), text("1", "Superseded by [2. T2](0002-b.md)"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "", "import-adr", dir, "--by", "o")
	want := "adj import-adr: warning: docs/adr/0001-a.md: record 1 is still decided, but the file now says it is superseded by docs/adr/0002-b.md; run `adj supersede 1 --by 2`\n"
	if r.code != 0 || r.stderr != want {
		t.Errorf("exit %d, stderr %q; want exit 0 and %q", r.code, r.stderr, want)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("the second import changed the tree")
	}
}
