// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The commands whose JSON a repo's contract pin guards (S45): each one with
// the flag that asks for JSON, and the same run without it.
var guardedRuns = []struct {
	name        string
	command     string
	json, plain []string
}{
	{"show", "show", []string{"show", "1", "--json"}, []string{"show", "1"}},
	{"list", "list", []string{"list", "--json"}, []string{"list"}},
	{"inbox", "inbox", []string{"inbox", "--json"}, []string{"inbox"}},
	{"check", "check", []string{"check", "--json", "storage"}, []string{"check", "storage"}},
	{"context", "context", []string{"context", "--format", "json"}, []string{"context"}},
	{"context --format=json", "context", []string{"context", "--format=json", "--max-bytes", "400"}, []string{"context", "--max-bytes", "400"}},
}

// A repo that pins another contract version gets no JSON: each JSON command
// exits 1 and prints only the error object, code contract_mismatch, naming
// both versions; the same command in text mode still works (S45).
func TestAPinForAnotherContractRefusesTheJSONCommands(t *testing.T) {
	root := seedInbox(t)
	env := testEnv(root)
	// What each run prints with no pin, to compare with after.
	free := map[string]result{}
	for _, tt := range guardedRuns {
		free[tt.name+" json"] = run(env, "", tt.json...)
		free[tt.name+" plain"] = run(env, "", tt.plain...)
		if free[tt.name+" json"].code != 0 || free[tt.name+" plain"].code != 0 {
			t.Fatalf("%s fails without a pin: %+v", tt.name, free)
		}
	}
	writeConfig(t, root, pinConfig(2))
	before := snapshot(t, root)

	for _, tt := range guardedRuns {
		t.Run(tt.name, func(t *testing.T) {
			r := run(env, "", tt.json...)
			if r.code != 1 {
				t.Fatalf("exit = %d, want 1: %+v", r.code, r)
			}
			code, command, message := errorObject(t, r.stdout)
			if code != "contract_mismatch" || command != tt.command {
				t.Errorf("error object code %q command %v, want contract_mismatch and %q", code, command, tt.command)
			}
			for _, want := range []string{"adj-json v2", "adj-json v1", "ask the owner to run `adj contract pin 1`", "or to install an adj that speaks adj-json v2"} {
				if !strings.Contains(message, want) {
					t.Errorf("message %q does not contain %q", message, want)
				}
			}
			if obj := decodeObject(t, r.stdout); len(obj) != 1 {
				t.Errorf("stdout holds more than the error object: %s", r.stdout)
			}
			if want := "adj " + tt.command + ": " + message + "\n"; r.stderr != want {
				t.Errorf("stderr = %q, want %q", r.stderr, want)
			}
			// Text mode is not refused, and prints what it printed.
			if p := run(env, "", tt.plain...); p != free[tt.name+" plain"] {
				t.Errorf("without --json: %+v, want %+v", p, free[tt.name+" plain"])
			}
		})
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, before) {
		t.Error("a refused command changed the repo")
	}

	// A pin for this version is no pin at all: the bytes are what they were.
	writeConfig(t, root, pinConfig(1))
	for _, tt := range guardedRuns {
		if r := run(env, "", tt.json...); r != free[tt.name+" json"] {
			t.Errorf("%s with pin 1 = %+v, want the unpinned output %+v", tt.name, r, free[tt.name+" json"])
		}
	}
}

// The guard comes after the arguments are checked and before the store is
// read: a usage error is still exit 2, and a record that is not there is
// still a refusal, because the repo's pin is the first thing about it.
func TestTheGuardSitsAfterUsageAndBeforeTheStore(t *testing.T) {
	root := seedInbox(t)
	writeConfig(t, root, pinConfig(2))
	env := testEnv(root)

	r := run(env, "", "show", "0", "--json")
	if r.code != 2 {
		t.Errorf("show 0 --json = %+v, want a usage error", r)
	}
	if code, _, _ := errorObject(t, r.stdout); code != "usage" {
		t.Errorf("code = %q, want usage", code)
	}
	r = run(env, "", "show", "999", "--json")
	if r.code != 1 {
		t.Fatalf("show 999 --json = %+v", r)
	}
	if code, _, _ := errorObject(t, r.stdout); code != "contract_mismatch" {
		t.Errorf("code = %q, want contract_mismatch", code)
	}
}

// Commands with no JSON output are not guarded: a pinned repo still takes
// questions, answers and lint.
func TestAPinDoesNotStopTheCommandsWithoutJSON(t *testing.T) {
	root := seedInbox(t)
	writeConfig(t, root, pinConfig(2))
	env := testEnv(root)
	for _, args := range [][]string{
		{"ask", "Another question"},
		{"withdraw", "8", "--why", "no longer needed"},
		{"amend", "1", "--title", "Pick a data store"},
	} {
		if r := run(env, "", args...); r.code != 0 {
			t.Errorf("%v = %+v, want exit 0", args, r)
		}
	}
	if got := readConfig(t, root); got != pinConfig(2) {
		t.Errorf("config = %q, a command rewrote it", got)
	}
}

func TestInboxAllRefusesAPinnedRepoInJSONOnly(t *testing.T) {
	env, path, repo := reposEnv(t)
	other := filepath.Join(filepath.Dir(repo), "other")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	writeConfig(t, other, "{}")
	writeConfig(t, repo, "{\n  \"contract\": 2\n}\n")
	writeReposConfig(t, path, `{"repos":[`+quoteRepo(repo)+`,`+quoteRepo(other)+`]}`)

	r := run(env, "", "inbox", "--all", "--json")
	if r.code != 1 {
		t.Fatalf("exit = %d, want 1 (one repo refused): %+v", r.code, r)
	}
	var out struct {
		Repos []struct {
			Root string `json:"root"`
		} `json:"repos"`
		Errors []struct {
			Root  string `json:"root"`
			Error string `json:"error"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
		t.Fatalf("stdout is not the inbox --all object: %v\n%s", err, r.stdout)
	}
	if len(out.Repos) != 1 || out.Repos[0].Root != other {
		t.Errorf("repos = %+v, want only the unpinned repo %s", out.Repos, other)
	}
	if len(out.Errors) != 1 || out.Errors[0].Root != repo ||
		!strings.Contains(out.Errors[0].Error, "adj-json v2") || !strings.Contains(out.Errors[0].Error, "adj-json v1") {
		t.Errorf("errors = %+v, want the pinned repo with a contract message", out.Errors)
	}
	if !strings.Contains(r.stderr, repo) || !strings.Contains(r.stderr, "contract mismatch") {
		t.Errorf("stderr = %q, want a warning naming the repo", r.stderr)
	}

	// Text is not refused.
	r = run(env, "", "inbox", "--all")
	if r.code != 0 || !strings.Contains(r.stdout, repo) || !strings.Contains(r.stdout, other) {
		t.Errorf("inbox --all (text) = %+v, want both repos and exit 0", r)
	}
}

const pinWarning = ".adjudicate.json: warning: contract-pin: this repo pins adj-json v2 but this adj speaks adj-json v1, so its JSON commands refuse this repo; ask the owner to run `adj contract pin 1` or to install an adj that speaks adj-json v2\n"

// adj lint warns about a pin for another version, and still lints the store,
// so it stays the way to diagnose a repo that the JSON commands refuse.
func TestLintWarnsAboutAPinForAnotherContract(t *testing.T) {
	root := lintRoot(t, "clean")
	writeConfig(t, root, pinConfig(2))
	env := testEnv(root)

	r := run(env, "", "lint")
	if want := pinWarning + "0 errors, 1 warning\n"; r.code != 0 || r.stdout != want || r.stderr != "" {
		t.Errorf("lint = %+v, want exit 0 and\n%s", r, want)
	}

	r = run(env, "", "lint", "--json")
	if r.code != 0 {
		t.Fatalf("lint --json = %+v", r)
	}
	var out struct {
		Errors, Warnings int
		Findings         []map[string]any
	}
	if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
		t.Fatal(err)
	}
	if out.Errors != 0 || out.Warnings != 1 || len(out.Findings) != 1 {
		t.Fatalf("lint --json = %s, want one warning", r.stdout)
	}
	f := out.Findings[0]
	if f["check"] != "contract-pin" || f["severity"] != "warning" || f["file"] != ".adjudicate.json" || f["id"] != float64(0) {
		t.Errorf("finding = %v", f)
	}
	if msg, _ := f["message"].(string); !strings.Contains(msg, "adj-json v2") || !strings.Contains(msg, "adj-json v1") {
		t.Errorf("message = %q, want both versions", msg)
	}

	// The warning does not hide a store problem, and the store problem still
	// decides the exit code.
	mixed := lintRoot(t, "mixed")
	writeConfig(t, mixed, pinConfig(2))
	r = run(testEnv(mixed), "", "lint")
	if want := pinWarning + mixedText[:len(mixedText)-len("2 errors, 1 warning\n")] + "2 errors, 2 warnings\n"; r.code != 1 || r.stdout != want {
		t.Errorf("lint on a mixed store = %+v, want exit 1 and\n%s", r, want)
	}
}

func TestLintIsQuietAboutAPinForThisContract(t *testing.T) {
	for _, config := range []string{pinConfig(1), plainConfig} {
		root := lintRoot(t, "clean")
		writeConfig(t, root, config)
		if r := run(testEnv(root), "", "lint"); r.code != 0 || r.stdout != "" {
			t.Errorf("config %q: lint = %+v, want a clean report", config, r)
		}
	}
}

func TestLintUsageNamesTheContractPinCheck(t *testing.T) {
	if r := run(testEnv(t.TempDir()), "", "lint", "--help"); !strings.Contains(r.stdout, "contract-pin") {
		t.Errorf("lint usage does not name the contract-pin check:\n%s", r.stdout)
	}
}

// docs/json.md says what the pin does where each reader will look for it.
func TestDocsDescribeThePinAndItsRefusal(t *testing.T) {
	doc := docsText(t)
	for _, tt := range []struct {
		heading string
		wants   []string
	}{
		{"## Contract version", []string{"adj contract pin M", "adj contract unpin", "contract_mismatch", "`contract-pin`", "never serves an older shape"}},
		{"## `adj inbox --all --json`", []string{"contract mismatch", "`errors`"}},
		{"## Keys of a lint finding", []string{"- `contract-pin` (warning)", "`0` for a finding about the config file"}},
	} {
		section, err := sectionUnder(doc, tt.heading)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range tt.wants {
			if !strings.Contains(section, want) {
				t.Errorf("docs/json.md %q does not contain %q", tt.heading, want)
			}
		}
	}
}

// The lint text format is stated with its exception: a finding about the
// config file has no "record ID:" part (usage text and docs/json.md).
func TestLintTextFormatIsStatedWithTheIDZeroException(t *testing.T) {
	usage := run(testEnv(t.TempDir()), "", "lint", "--help").stdout
	if !strings.Contains(usage, "no \"record ID:\" for a finding about the config file") {
		t.Errorf("lint usage does not state the exception to the text format:\n%s", usage)
	}
	section, err := sectionUnder(docsText(t), "## `adj lint --json`")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(section, "(no `record ID:` for a finding about the config file)") {
		t.Errorf("docs/json.md does not state the exception where it gives the text format:\n%s", section)
	}
}
