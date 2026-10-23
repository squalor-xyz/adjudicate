// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// jsonFlagSet is the flag set of a command that has --json and a --key that
// takes a value, as list does; formatFlagSet is context's --format.
func jsonFlagSet() *flag.FlagSet {
	fs, _ := newFlagSet("list")
	fs.Bool("json", false, "")
	fs.String("key", "", "")
	return fs
}

func formatFlagSet() *flag.FlagSet {
	fs, _ := newFlagSet("context")
	fs.String("format", "md", "")
	return fs
}

func TestWantsJSON(t *testing.T) {
	tests := []struct {
		name string
		fs   *flag.FlagSet
		args []string
		want bool
	}{
		{"--json", jsonFlagSet(), []string{"--json"}, true},
		{"-json", jsonFlagSet(), []string{"-json"}, true},
		{"--json=true", jsonFlagSet(), []string{"--json=true"}, true},
		{"--json=1", jsonFlagSet(), []string{"--json=1"}, true},
		{"--json=false", jsonFlagSet(), []string{"--json=false"}, false},
		{"--json then --json=false: the last wins", jsonFlagSet(), []string{"--json", "--json=false"}, false},
		{"--json=false then --json", jsonFlagSet(), []string{"--json=false", "--json"}, true},
		{"after a positional", jsonFlagSet(), []string{"3", "--json"}, true},
		{"after --", jsonFlagSet(), []string{"--", "--json"}, false},
		{"before and after --", jsonFlagSet(), []string{"--json", "--", "x"}, true},
		{"the value of --key", jsonFlagSet(), []string{"--key", "--json"}, false},
		{"the value of --key=", jsonFlagSet(), []string{"--key=--json"}, false},
		{"--key then --json", jsonFlagSet(), []string{"--key", "k", "--json"}, true},
		{"three dashes is a bad flag", jsonFlagSet(), []string{"---json"}, false},
		{"three dashes before the command", nil, []string{"---json"}, false},
		{"three-dash --format", formatFlagSet(), []string{"---format=json"}, false},
		{"a positional that is not a flag", jsonFlagSet(), []string{"json"}, false},
		{"no flag", jsonFlagSet(), []string{"3"}, false},
		{"no arguments", jsonFlagSet(), nil, false},
		{"a bad flag does not hide --json", jsonFlagSet(), []string{"--bogus", "--json"}, true},
		{"a command with no --json flag", formatFlagSet(), []string{"--json"}, false},
		{"--format json", formatFlagSet(), []string{"--format", "json"}, true},
		{"--format=json", formatFlagSet(), []string{"--format=json"}, true},
		{"-format json", formatFlagSet(), []string{"-format", "json"}, true},
		{"--format md", formatFlagSet(), []string{"--format", "md"}, false},
		{"--format=md", formatFlagSet(), []string{"--format=md"}, false},
		{"--format json then md", formatFlagSet(), []string{"--format", "json", "--format", "md"}, false},
		{"--format md then json", formatFlagSet(), []string{"--format=md", "--format=json"}, true},
		{"--format json after --", formatFlagSet(), []string{"--", "--format", "json"}, false},
		{"--format with no value", formatFlagSet(), []string{"--format"}, false},
		{"--format is not a flag of list", jsonFlagSet(), []string{"--format", "json"}, false},
		{"before the command: --json", nil, []string{"--bogus", "--json"}, true},
		{"before the command: --root takes a value", nil, []string{"--root", "--json"}, false},
		{"before the command: --root=", nil, []string{"--root=x", "--json"}, true},
		{"before the command: --json=false", nil, []string{"--json=false"}, false},
		{"--json=maybe is on", jsonFlagSet(), []string{"--json=maybe"}, true},
		{"--json= is on", jsonFlagSet(), []string{"--json="}, true},
		{"--json=0 is off", jsonFlagSet(), []string{"--json=0"}, false},
		{"--json=F is off", jsonFlagSet(), []string{"--json=F"}, false},
		{"--json=maybe then --json=false: the last wins", jsonFlagSet(), []string{"--json=maybe", "--json=false"}, false},
		{"--json=false then --json=maybe: the last wins", jsonFlagSet(), []string{"--json=false", "--json=maybe"}, true},
		{"before the command: --json=maybe", nil, []string{"--json=maybe"}, true},
		{"before the command: --json=", nil, []string{"--json="}, true},
		{"before the command: --json=0", nil, []string{"--json=0"}, false},
		{"before the command: --json=F", nil, []string{"--json=F"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wantsJSON(tt.fs, tt.args); got != tt.want {
				t.Errorf("wantsJSON(%q) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

// errorCase is one failing run: the setup, the arguments with --json, the exit
// code, the error code and the command the object names ("" is null).
type errorCase struct {
	name  string
	setup func(t *testing.T) Env
	args  []string
	plain []string // the same run without JSON; default: args without --json
	// plainText: the run without JSON fails with other text (the budget
	// message names the size of the other format), so only its exit code and
	// empty stdout are compared.
	plainText bool
	exit      int
	code      string
	command   string
}

// failureStdout reports whether stdout is what a failed run prints: nothing,
// or, when args ask for JSON (--json, --format json), only the error object.
func failureStdout(stdout string, args []string) bool {
	jsonRun := false
	for i, a := range args {
		if a == "--json" || a == "--format=json" || (a == "--format" && i+1 < len(args) && args[i+1] == "json") {
			jsonRun = true
		}
	}
	if !jsonRun {
		return stdout == ""
	}
	var obj map[string]any
	return json.Unmarshal([]byte(stdout), &obj) == nil && len(obj) == 1 && obj["error"] != nil
}

func withoutJSON(args []string) []string {
	var out []string
	for _, a := range args {
		if a != "--json" {
			out = append(out, a)
		}
	}
	return out
}

// seededEnv is a repo with the inbox fixture's seven records.
func seededEnv(t *testing.T) Env { return testEnv(seedInbox(t)) }

func writeStoreFile(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "decisions", name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func errorCases() []errorCase {
	return []errorCase{
		{
			name:    "not found",
			setup:   seededEnv,
			args:    []string{"show", "999", "--json"},
			exit:    1,
			code:    "not_found",
			command: "show",
		},
		{
			name: "duplicate id",
			setup: func(t *testing.T) Env {
				root := seedInbox(t)
				data, err := os.ReadFile(filepath.Join(root, "decisions", "0001-pick-a-store.md"))
				if err != nil {
					t.Fatal(err)
				}
				writeStoreFile(t, root, "0001-pick-a-store-copy.md", string(data))
				return testEnv(root)
			},
			args:    []string{"show", "1", "--json"},
			exit:    1,
			code:    "duplicate_id",
			command: "show",
		},
		{
			name: "malformed record file",
			setup: func(t *testing.T) Env {
				root := initRoot(t)
				writeStoreFile(t, root, "0001-bad.md", "not a record\n")
				return testEnv(root)
			},
			args:    []string{"list", "--json"},
			exit:    1,
			code:    "invalid_record",
			command: "list",
		},
		{
			name:    "no repo",
			setup:   func(t *testing.T) Env { return testEnv(t.TempDir()) },
			args:    []string{"list", "--json"},
			exit:    1,
			code:    "no_repo",
			command: "list",
		},
		{
			name: "unknown config key",
			setup: func(t *testing.T) Env {
				root := initRoot(t)
				if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), []byte(`{"bogus": 1}`), 0o644); err != nil {
					t.Fatal(err)
				}
				return testEnv(root)
			},
			args:    []string{"lint", "--json"},
			exit:    1,
			code:    "invalid_config",
			command: "lint",
		},
		{
			name:    "bad flag",
			setup:   seededEnv,
			args:    []string{"show", "--bogus", "--json"},
			exit:    2,
			code:    "usage",
			command: "show",
		},
		{
			// S50: a --json=VALUE that is not a bool is a usage error and
			// still JSON mode. The same run with the last --json=false is
			// off and fails with the same stderr.
			name:    "unparsable --json value",
			setup:   seededEnv,
			args:    []string{"show", "1", "--json=maybe"},
			plain:   []string{"show", "1", "--json=maybe", "--json=false"},
			exit:    2,
			code:    "usage",
			command: "show",
		},
		{
			name:    "usage error from the command",
			setup:   seededEnv,
			args:    []string{"list", "--status", "bogus", "--json"},
			exit:    2,
			code:    "usage",
			command: "list",
		},
		{
			name:    "usage error before the store is read",
			setup:   seededEnv,
			args:    []string{"check", "--json"},
			exit:    2,
			code:    "usage",
			command: "check",
		},
		{
			name:    "bad flag before the command",
			setup:   seededEnv,
			args:    []string{"--bogus", "--json"},
			exit:    2,
			code:    "usage",
			command: "",
		},
		{
			name:    "unknown command",
			setup:   seededEnv,
			args:    []string{"frobnicate", "--json"},
			exit:    2,
			code:    "usage",
			command: "",
		},
		{
			name:    "bad flag after a leading --root",
			setup:   seededEnv,
			args:    []string{"--root", "x", "--bogus", "--json"},
			exit:    2,
			code:    "usage",
			command: "",
		},
		{
			name:    "bad user config",
			setup:   badUserConfigEnv,
			args:    []string{"inbox", "--all", "--json"},
			exit:    1,
			code:    "user_config",
			command: "inbox",
		},
		{
			name:    "repos list with a bad user config",
			setup:   badUserConfigEnv,
			args:    []string{"repos", "list", "--json"},
			exit:    1,
			code:    "user_config",
			command: "repos",
		},
		{
			name: "no home for the user config",
			setup: func(t *testing.T) Env {
				env := seededEnv(t)
				env.Getenv = func(string) string { return "" }
				return env
			},
			args:    []string{"inbox", "--all", "--json"},
			exit:    1,
			code:    "user_config",
			command: "inbox",
		},
		{
			name:      "budget too small",
			setup:     seededEnv,
			args:      []string{"context", "--format", "json", "--max-bytes", "1"},
			plain:     []string{"context", "--max-bytes", "1"},
			plainText: true,
			exit:      1,
			code:      "budget",
			command:   "context",
		},
		{
			name:    "bad flag for context --format=json",
			setup:   seededEnv,
			args:    []string{"context", "--bogus", "--format=json"},
			plain:   []string{"context", "--bogus"},
			exit:    2,
			code:    "usage",
			command: "context",
		},
		{
			name: "store directory that is a file",
			setup: func(t *testing.T) Env {
				root := initRoot(t)
				dir := filepath.Join(root, "decisions")
				if err := os.RemoveAll(dir); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dir, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return testEnv(root)
			},
			args:    []string{"list", "--json"},
			exit:    1,
			code:    "io",
			command: "list",
		},
		{
			name: "unreadable record file",
			setup: func(t *testing.T) Env {
				if runtime.GOOS == "windows" {
					t.Skip("file permission bits do not make a file unreadable on Windows")
				}
				root := seedInbox(t)
				path := filepath.Join(root, "decisions", "0002-cache-layer.md")
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
				if _, err := os.ReadFile(path); err == nil {
					t.Skip("the file is readable anyway (running as root)")
				}
				t.Cleanup(func() { os.Chmod(path, 0o644) })
				return testEnv(root)
			},
			args:    []string{"show", "2", "--json"},
			exit:    1,
			code:    "io",
			command: "show",
		},
	}
}

// badUserConfigEnv is a repo plus a user config that is not valid JSON.
func badUserConfigEnv(t *testing.T) Env {
	env, path, _ := reposEnv(t)
	writeReposConfig(t, path, `{"repos": [`)
	return env
}

// The error object: exactly one object on stdout, with the keys code, message
// and command in that order, and the stderr text that adj printed before the
// object existed.
func TestJSONFailuresPrintTheErrorObject(t *testing.T) {
	for _, tt := range errorCases() {
		t.Run(tt.name, func(t *testing.T) {
			env := tt.setup(t)
			got := run(env, "", tt.args...)
			if got.code != tt.exit {
				t.Fatalf("exit = %d, want %d\nstdout %q\nstderr %q", got.code, tt.exit, got.stdout, got.stderr)
			}
			var obj map[string]map[string]any
			dec := json.NewDecoder(strings.NewReader(got.stdout))
			if err := dec.Decode(&obj); err != nil {
				t.Fatalf("stdout is not one JSON object: %v\n%q", err, got.stdout)
			}
			if dec.More() {
				t.Fatalf("stdout has more than one value: %q", got.stdout)
			}
			if len(obj) != 1 || obj["error"] == nil {
				t.Fatalf("top-level keys = %v, want only error: %q", obj, got.stdout)
			}
			inner := obj["error"]
			if !reflect.DeepEqual(keysOf(inner), []string{"code", "command", "message"}) {
				t.Fatalf("error keys = %v: %q", keysOf(inner), got.stdout)
			}
			if inner["code"] != tt.code {
				t.Errorf("code = %v, want %q", inner["code"], tt.code)
			}
			var wantCommand any
			if tt.command != "" {
				wantCommand = tt.command
			}
			if inner["command"] != wantCommand {
				t.Errorf("command = %v, want %v", inner["command"], wantCommand)
			}
			message, _ := inner["message"].(string)
			if message == "" || strings.Contains(message, "usage: adj") || strings.HasPrefix(message, "adj ") {
				t.Errorf("message = %q: want the error text without the adj prefix or the usage", message)
			}
			// Two-space indent, keys in order, one final newline.
			wantStart := "{\n  \"error\": {\n    \"code\": \"" + tt.code + "\",\n    \"message\": "
			if !strings.HasPrefix(got.stdout, wantStart) || !strings.HasSuffix(got.stdout, "\n  }\n}\n") {
				t.Errorf("stdout layout = %q", got.stdout)
			}
			if !strings.Contains(got.stdout, "\n    \"command\": ") {
				t.Errorf("stdout has no command line: %q", got.stdout)
			}

			// stderr is the text from before: "adj NAME: MESSAGE", then the
			// usage for a usage error; and it is what the run prints without
			// --json.
			prefix := "adj: "
			if tt.command != "" {
				prefix = "adj " + tt.command + ": "
			}
			if !strings.HasPrefix(got.stderr, prefix+message+"\n") {
				t.Errorf("stderr = %q, want it to start %q", got.stderr, prefix+message+"\n")
			}
			if tt.exit == 1 && got.stderr != prefix+message+"\n" {
				t.Errorf("stderr = %q, want just the message line", got.stderr)
			}
			if tt.exit == 2 && !strings.Contains(got.stderr, "usage: adj") {
				t.Errorf("stderr has no usage text: %q", got.stderr)
			}
			plainArgs := tt.plain
			if plainArgs == nil {
				plainArgs = withoutJSON(tt.args)
			}
			plain := run(env, "", plainArgs...)
			if plain.code != tt.exit || plain.stdout != "" || (!tt.plainText && plain.stderr != got.stderr) {
				t.Errorf("without --json: %+v\nwant exit %d, empty stdout and stderr %q", plain, tt.exit, got.stderr)
			}
		})
	}
}

// The stderr text of the failures, as it was before the error object (S42).
func TestJSONFailuresKeepTheirStderrText(t *testing.T) {
	root := seedInbox(t)
	env := testEnv(root)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"show", "999", "--json"}, "adj show: record not found: id 999\n"},
		{[]string{"--bogus", "--json"}, "adj: unknown flag \"--bogus\" before the command; only --root, --help and -h are accepted there\n" + usage},
		{[]string{"show", "--bogus", "--json"}, "adj show: flag provided but not defined: -bogus\n" + showUsage},
		{[]string{"context", "--format=json", "--max-bytes", "1"}, "adj context: --max-bytes 1 is too small: the shortest output is 38 bytes\n"},
	}
	for _, tt := range tests {
		got := run(env, "", tt.args...)
		if got.stderr != tt.want {
			t.Errorf("%q: stderr = %q, want %q", tt.args, got.stderr, tt.want)
		}
	}
	nr := run(testEnv(t.TempDir()), "", "list", "--json")
	if !strings.HasPrefix(nr.stderr, "adj list: no repo root found: no .adjudicate.json in ") || !strings.HasSuffix(nr.stderr, "; run `adj init` first\n") {
		t.Errorf("no repo: stderr = %q", nr.stderr)
	}
}

// A failure that is not in JSON mode prints nothing on stdout, as before.
func TestFailuresWithoutJSONPrintNothingOnStdout(t *testing.T) {
	env := seededEnv(t)
	for _, args := range [][]string{
		{"show", "999"},
		{"show", "--bogus"},
		{"list", "--json=false", "--status", "bogus"},
		{"--bogus"},
		{"context", "--format", "md", "--max-bytes", "1"},
		{"ask", "--json"}, // ask has no --json: a usage error with no object
		{"show", "--", "--json"},
	} {
		got := run(env, "", args...)
		if got.code == 0 || got.stdout != "" {
			t.Errorf("%q: %+v, want a failure with nothing on stdout", args, got)
		}
	}
}

// Reports and results are not failures: no error object, same exit codes.
func TestReportsAreNotErrorObjects(t *testing.T) {
	t.Run("lint with an error finding", func(t *testing.T) {
		env := testEnv(lintRoot(t, "mixed"))
		got := run(env, "", "lint", "--json")
		if got.code != 1 || got.stderr != "" {
			t.Fatalf("lint = %+v, want exit 1 and no stderr", got)
		}
		obj := decodeObject(t, got.stdout)
		if !reflect.DeepEqual(keysOf(obj), []string{"errors", "findings", "warnings"}) {
			t.Errorf("lint keys = %v", keysOf(obj))
		}
	})
	t.Run("check not found", func(t *testing.T) {
		got := run(seededEnv(t), "", "check", "zzz-nothing", "--json")
		if got.code != 0 || got.stderr != "" {
			t.Fatalf("check = %+v", got)
		}
		if obj := decodeObject(t, got.stdout); obj["result"] != "not found" || obj["error"] != nil {
			t.Errorf("check object = %v", obj)
		}
	})
	t.Run("inbox --all with a missing repo", func(t *testing.T) {
		env, path, repo := reposEnv(t)
		missing := filepath.Join(t.TempDir(), "missing")
		writeReposConfig(t, path, `{"repos":[`+quoteRepo(repo)+`,`+quoteRepo(missing)+`]}`)
		got := run(env, "", "inbox", "--all", "--json")
		if got.code != 1 || !strings.Contains(got.stderr, "warning") {
			t.Fatalf("inbox --all = %+v", got)
		}
		obj := decodeObject(t, got.stdout)
		if !reflect.DeepEqual(keysOf(obj), []string{"errors", "repos"}) {
			t.Errorf("inbox --all keys = %v", keysOf(obj))
		}
	})
	t.Run("context --max-bytes that fits only the omitted line", func(t *testing.T) {
		got := run(seededEnv(t), "", "context", "--format", "json", "--max-bytes", "40")
		if got.code != 0 || decodeObject(t, got.stdout)["error"] != nil {
			t.Errorf("context = %+v", got)
		}
	})
}

// A command that succeeds prints no error object.
func TestSuccessPrintsNoErrorObject(t *testing.T) {
	got := run(seededEnv(t), "", "show", "1", "--json")
	if got.code != 0 || decodeObject(t, got.stdout)["error"] != nil {
		t.Errorf("show = %+v", got)
	}
}

// docs/json.md documents the object and every code (S42).
func TestErrorObjectMatchesDocs(t *testing.T) {
	const heading = "## Errors"
	got := run(testEnv(t.TempDir()), "", "list", "--json")
	obj := decodeObject(t, got.stdout)
	example := decodeObject(t, docsJSONExample(t, heading))
	for name, o := range map[string]map[string]any{"output": obj, "docs": example} {
		if !reflect.DeepEqual(keysOf(o), []string{"error"}) {
			t.Fatalf("%s top keys = %v", name, keysOf(o))
		}
		inner, _ := o["error"].(map[string]any)
		if !reflect.DeepEqual(keysOf(inner), []string{"code", "command", "message"}) {
			t.Fatalf("%s error keys = %v", name, keysOf(inner))
		}
	}
	// The docs table lists exactly the codes the tests above produce, plus
	// io, the catch-all.
	want := map[string]bool{"usage": true}
	for _, tt := range errorCases() {
		want[tt.code] = true
	}
	docCodes := docsTableKeys(t, heading)
	if len(docCodes) != len(want) {
		t.Errorf("docs codes = %v, want %d codes %v", docCodes, len(want), want)
	}
	for _, c := range docCodes {
		if !want[c] {
			t.Errorf("docs list code %q that no test produces", c)
		}
	}
}
