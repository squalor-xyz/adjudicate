// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/lint"
)

// lintRoot returns a temp copy of the fixture store internal/lint/testdata/
// <name>, so a command that wrote to its root could not touch the fixture.
func lintRoot(t *testing.T, name string) string {
	t.Helper()
	return copyTree(t, filepath.Join("..", "lint", "testdata", name))
}

const mixedText = `decisions/0001-pick-a-store.md: record 1: warning: decided-by: decided_by is missing
decisions/0002-cache-layer.md: record 2: error: fields: rationale: is required once decided
decisions/0003-old-queue.md: record 3: error: dangling-link: superseded_by 9 names no record
2 errors, 1 warning
`

// Each defect class exits 1 and names the record and the file; a clean store
// and a store with only warnings exit 0 (C32); lint prints findings only on
// stdout and never fails to load a store that List refuses.
func TestLintReportsEachDefectClass(t *testing.T) {
	tests := []struct {
		fixture string
		code    int
		want    string   // the whole stdout, if set
		lines   []string // lines that stdout must contain
	}{
		{fixture: "clean", code: 0, want: ""},
		{fixture: "missing-decided-by", code: 0, want: `decisions/0001-nobody.md: record 1: warning: decided-by: decided_by is missing
decisions/0002-blank.md: record 2: warning: decided-by: decided_by is missing
0 errors, 2 warnings
`},
		{fixture: "mixed", code: 1, want: mixedText},
		{fixture: "cycle-two", code: 1, want: `decisions/0001-first.md: record 1: error: supersede-cycle: supersede cycle among records 1, 2
decisions/0002-second.md: record 2: error: supersede-cycle: supersede cycle among records 1, 2
2 errors, 0 warnings
`},
		{fixture: "cycle-long", code: 1, lines: []string{
			"decisions/0003-link-3.md: record 3: error: supersede-cycle: supersede cycle among records 1, 2, 3, 4",
			"4 errors, 0 warnings",
		}},
		{fixture: "cycle-supersedes", code: 1, lines: []string{
			"decisions/0002-second.md: record 2: error: supersede-cycle: supersede cycle among records 1, 2",
			"2 errors, 0 warnings",
		}},
		{fixture: "missing-fields", code: 1, lines: []string{
			"decisions/0001-no-answer.md: record 1: error: fields: decision: is required once decided",
			"decisions/0002-no-title.md: record 2: error: fields: title: is required",
			"decisions/0003-no-why.md: record 3: error: fields: rationale: is required once rejected",
			"decisions/0004-no-link.md: record 4: error: fields: superseded_by: is required once superseded",
			"decisions/0005-no-status.md: record 5: error: fields: status: unknown status \"\"",
			"decisions/0006-no-id.md: record 6: error: fields: id: must be a positive integer, got 0",
			"10 errors, 0 warnings",
		}},
		{fixture: "dangling-links", code: 1, lines: []string{
			"decisions/0001-first.md: record 1: error: dangling-link: supersedes 7 names no record",
			"decisions/0002-second.md: record 2: error: dangling-link: superseded_by 8 names no record",
			"3 errors, 0 warnings",
		}},
		{fixture: "key-conflict", code: 1, lines: []string{
			"decisions/0001-first.md: record 1: error: key-conflict: key \"storage.db\" is also decided by records 2, 3",
			"3 errors, 0 warnings",
		}},
		{fixture: "supersede-state", code: 1, lines: []string{
			"decisions/0001-first.md: record 1: error: supersede-state: superseded_by 2 is set but the status is decided",
			"2 errors, 0 warnings",
		}},
		{fixture: "id-mismatch", code: 1, lines: []string{
			"decisions/0002-wrong-id.md: record 2: error: id-mismatch: id 7 does not match the ID 2 in the file name",
			"decisions/0003-self-link.md: record 3: error: fields: superseded_by: 3 is not another record's id",
			"5 errors, 0 warnings",
		}},
		{fixture: "cycle-missing", code: 1, lines: []string{
			"decisions/0001-first.md: record 1: error: supersede-cycle: supersede cycle among records 1, 2",
			"6 errors, 0 warnings",
		}},
		{fixture: "parse-errors", code: 1, lines: []string{
			"decisions/0002-unknown-key.md: record 2: error: parse: frontmatter: json: unknown field \"colour\"",
			"5 errors, 0 warnings",
		}},
		{fixture: "broken-file", code: 1, lines: []string{
			"decisions/0002-garbled.md: record 2: error: parse: no opening --- line",
			"decisions/0004-unclosed.md: record 4: error: parse: frontmatter is not closed by a --- line",
			"2 errors, 0 warnings",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			r := run(testEnv(lintRoot(t, tt.fixture)), "", "lint")
			if r.code != tt.code {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", r.code, tt.code, r.stderr)
			}
			if r.stderr != "" {
				t.Errorf("stderr = %q, want empty: findings go to stdout", r.stderr)
			}
			if tt.lines == nil && r.stdout != tt.want {
				t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, tt.want)
			}
			got := strings.Split(r.stdout, "\n")
			for _, line := range tt.lines {
				found := false
				for _, g := range got {
					found = found || g == line
				}
				if !found {
					t.Errorf("stdout has no line %q:\n%s", line, r.stdout)
				}
			}
		})
	}
}

// lint --json is the findings with the counts; the documented example is the
// real output for the "mixed" fixture, so the docs cannot drift (C104).
func TestLintJSONMatchesTheDocumentedExample(t *testing.T) {
	r := run(testEnv(lintRoot(t, "mixed")), "", "lint", "--json")
	if r.code != 1 {
		t.Errorf("exit code = %d, want 1 (the store has errors); stderr:\n%s", r.code, r.stderr)
	}
	if want := docsJSONExample(t, "## `adj lint --json`"); r.stdout != want {
		t.Errorf("lint --json =\n%s\nwant the docs/json.md example\n%s", r.stdout, want)
	}
}

func TestLintJSONCountsAndExitCodes(t *testing.T) {
	tests := []struct {
		fixture           string
		code              int
		errors, warnings  float64
		wantEmptyFindings bool
	}{
		{"clean", 0, 0, 0, true},
		{"missing-decided-by", 0, 0, 2, false},
		{"mixed", 1, 2, 1, false},
		{"cycle-long", 1, 4, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			r := run(testEnv(lintRoot(t, tt.fixture)), "", "lint", "--json")
			if r.code != tt.code {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", r.code, tt.code, r.stderr)
			}
			top := decodeObject(t, r.stdout)
			if top["errors"] != tt.errors || top["warnings"] != tt.warnings {
				t.Errorf("errors, warnings = %v, %v; want %v, %v", top["errors"], top["warnings"], tt.errors, tt.warnings)
			}
			findings, ok := top["findings"].([]any)
			if !ok {
				t.Fatalf("findings is not an array in:\n%s", r.stdout)
			}
			if len(findings) != int(tt.errors+tt.warnings) || (len(findings) == 0) != tt.wantEmptyFindings {
				t.Errorf("%d findings for %v errors and %v warnings", len(findings), tt.errors, tt.warnings)
			}
		})
	}
	// A clean store is an empty array, never null.
	r := run(testEnv(lintRoot(t, "clean")), "", "lint", "--json")
	if want := "{\n  \"errors\": 0,\n  \"warnings\": 0,\n  \"findings\": []\n}\n"; r.stdout != want {
		t.Errorf("clean store: lint --json = %q, want %q", r.stdout, want)
	}
}

// Every finding object has exactly the keys in the docs table, and its
// severity and check are values the docs name.
func TestLintFindingsUseOnlyDocumentedKeys(t *testing.T) {
	want := docsTableKeys(t, "## Keys of a lint finding")
	r := run(testEnv(lintRoot(t, "mixed")), "", "lint", "--json")
	findings := recordObjects(t, r.stdout, "findings")
	if len(findings) != 3 {
		t.Fatalf("%d findings, want 3:\n%s", len(findings), r.stdout)
	}
	for _, f := range findings {
		if got := keysOf(f); !reflect.DeepEqual(got, want) {
			t.Errorf("finding %v has keys %v, docs table has %v", f["id"], got, want)
		}
		if sev := f["severity"]; sev != "error" && sev != "warning" {
			t.Errorf("finding %v has severity %v", f["id"], sev)
		}
	}
}

// The docs and the usage text name every check, so a new check cannot ship
// undocumented (lint.Checks is the list the lint tests pin to fixtures).
func TestLintDocsAndUsageNameEveryCheck(t *testing.T) {
	section, err := sectionUnder(docsText(t), "## Keys of a lint finding")
	if err != nil {
		t.Fatal(err)
	}
	help := run(testEnv(t.TempDir()), "", "lint", "--help").stdout
	for _, check := range lint.Checks {
		if !strings.Contains(section, "`"+check+"`") {
			t.Errorf("docs/json.md does not name the check `%s` in its lint finding section", check)
		}
		if !strings.Contains(help, check) {
			t.Errorf("adj lint --help does not name the check %s", check)
		}
	}
}

// copyTree copies the directory src to a new temp dir and returns it.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// lint only reads: a broken store is the same after it runs.
func TestLintChangesNothing(t *testing.T) {
	root := lintRoot(t, "broken-file")
	before := snapshot(t, root)
	if len(before) == 0 {
		t.Fatal("the copy of the fixture is empty")
	}
	run(testEnv(root), "", "lint")
	run(testEnv(root), "", "lint", "--json")
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Errorf("lint changed the store:\nbefore %v\nafter  %v", before, after)
	}
}

func TestLintHelpAndBadArguments(t *testing.T) {
	env := testEnv(lintRoot(t, "clean"))
	for _, flag := range []string{"--help", "-h"} {
		r := run(env, "", "lint", flag)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj lint") || r.stderr != "" {
			t.Errorf("lint %s: exit %d, stdout %q, stderr %q; want exit 0 and usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"positional argument", []string{"lint", "extra"}, "no arguments"},
		{"unknown flag", []string{"lint", "--nope"}, "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(env, "", tt.args...)
			if r.code != 2 || r.stdout != "" {
				t.Errorf("exit %d, stdout %q; want exit 2 and no stdout", r.code, r.stdout)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj lint") {
				t.Errorf("stderr should contain %q and the lint usage:\n%s", tt.wantErr, r.stderr)
			}
		})
	}
}

// No repo is a failed run (exit 1), not a clean store.
func TestLintWithoutARepo(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "lint")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, "adj init") {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1, no stdout and a hint to run adj init", r.code, r.stdout, r.stderr)
	}
}

func TestLintListedInTheMainUsage(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(r.stdout, "adj lint ") {
		t.Errorf("main usage does not list adj lint:\n%s", r.stdout)
	}
}
