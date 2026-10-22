// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// hasUsageLine reports whether s has a line that starts with "usage: adj".
func hasUsageLine(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "usage: adj") {
			return true
		}
	}
	return false
}

func TestRunHelpPrintsUsageToStdout(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "long flag", args: []string{"--help"}},
		{name: "short flag", args: []string{"-h"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if !hasUsageLine(stdout.String()) {
				t.Errorf("stdout has no line starting %q:\n%s", "usage: adj", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunUsageErrorsPrintUsageToStderr(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantErrText   string
		forbidErrText string
	}{
		{name: "no arguments", args: nil, forbidErrText: "unknown command"},
		{name: "unknown subcommand", args: []string{"frobnicate"}, wantErrText: `unknown command "frobnicate"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if !hasUsageLine(stderr.String()) {
				t.Errorf("stderr has no line starting %q:\n%s", "usage: adj", stderr.String())
			}
			if tt.wantErrText != "" && !strings.Contains(stderr.String(), tt.wantErrText) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantErrText, stderr.String())
			}
			if tt.forbidErrText != "" && strings.Contains(stderr.String(), tt.forbidErrText) {
				t.Errorf("stderr contains %q, want no such text:\n%s", tt.forbidErrText, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}

// decoyEnv is an Env whose default root is an initialised, empty repo that is
// not the one under test, so a command that ignores a --root before the
// command reads the decoy and shows it.
func decoyEnv(t *testing.T) Env {
	t.Helper()
	return testEnv(initRoot(t))
}

// sameResult fails the test unless got and want have the same exit code,
// stdout and stderr.
func sameResult(t *testing.T, label string, got, want result) {
	t.Helper()
	if got != want {
		t.Errorf("%s: differs from the flag after the command\n got: exit %d, stdout %q, stderr %q\nwant: exit %d, stdout %q, stderr %q",
			label, got.code, got.stdout, got.stderr, want.code, want.stdout, want.stderr)
	}
}

func TestRootBeforeCommand(t *testing.T) {
	root := seedInbox(t)
	env := decoyEnv(t)

	reads := []struct {
		name         string
		before, rest []string // before: the flag's spelling; rest: the command and its flags
		readsStore   bool     // the answer depends on the root, so the decoy root must answer differently
	}{
		{"list, separate value", []string{"--root", root}, []string{"list"}, true},
		{"list, single dash", []string{"-root", root}, []string{"list"}, true},
		{"list, flags after", []string{"--root", root}, []string{"list", "--status", "open", "--json"}, true},
		{"inbox --json, equals form", []string{"--root=" + root}, []string{"inbox", "--json"}, true},
		{"show", []string{"--root", root}, []string{"show", "1"}, true},
		{"check", []string{"--root", root}, []string{"check", "cache"}, true},
		{"context", []string{"--root", root}, []string{"context", "--format", "json"}, true},
		{"lint", []string{"--root", root}, []string{"lint"}, true},
		{"unknown id fails the same way", []string{"--root", root}, []string{"show", "99"}, false},
		{"command usage error is the same", []string{"--root", root}, []string{"list", "extra"}, false},
		{"command help is the same", []string{"--root", root}, []string{"list", "--help"}, false},
	}
	for _, tt := range reads {
		t.Run(tt.name, func(t *testing.T) {
			var after []string
			after = append(after, tt.rest[0], "--root", root)
			after = append(after, tt.rest[1:]...)
			want := run(env, "", after...)
			got := run(env, "", append(append([]string{}, tt.before...), tt.rest...)...)
			sameResult(t, "result", got, want)
			// The control must differ from what the decoy root answers, or the
			// comparison above would pass whether or not --root was applied.
			if tt.readsStore {
				if decoy := run(env, "", tt.rest...); decoy == want {
					t.Fatalf("control run is the same as the default root's answer; the case proves nothing: %+v", want)
				}
			}
		})
	}

	t.Run("ask", func(t *testing.T) {
		// Two identical empty repos: one asked with the flag before the
		// command, one with it after. The decoy root must stay empty.
		a, b := initRoot(t), initRoot(t)
		askArgs := []string{"Pick a store", "--option", "a:Use A", "--option", "b:Use B", "--recommend", "a", "--key", "storage.engine", "--tag", "infra"}
		want := run(env, "", append([]string{"ask", "--root", b}, askArgs...)...)
		got := run(env, "", append([]string{"--root", a, "ask"}, askArgs...)...)
		sameResult(t, "ask", got, want)
		if want.code != 0 || want.stdout != "1\n" {
			t.Fatalf("control ask: exit %d, stdout %q, stderr %q", want.code, want.stdout, want.stderr)
		}
		if !reflect.DeepEqual(snapshot(t, a), snapshot(t, b)) {
			t.Errorf("store written with --root before the command differs from the one written with it after")
		}
		if n := len(records(t, a)); n != 1 {
			t.Errorf("records in --root DIR = %d, want 1", n)
		}
		if n := len(records(t, env.Root)); n != 0 {
			t.Errorf("records in the default root = %d, want 0: --root before the command was ignored", n)
		}
	})

	t.Run("init", func(t *testing.T) {
		dir := t.TempDir()
		got := run(env, "", "--root", dir, "init")
		if got.code != 0 {
			t.Fatalf("exit %d, stderr %q", got.code, got.stderr)
		}
		if _, err := os.Stat(filepath.Join(dir, ".adjudicate.json")); err != nil {
			t.Errorf("init with --root before the command wrote no config in DIR: %v", err)
		}
	})
}

func TestRootBeforeCommandEdges(t *testing.T) {
	root := seedInbox(t)
	env := decoyEnv(t)
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string // substring; "" means stderr must be empty
		wantStdout string // substring; "" means stdout must be empty
		forbidErr  string // substring stderr must not contain
	}{
		{"no value, last argument", []string{"--root"}, 2, "--root needs a value", "", ""},
		{"second leading --root has no value", []string{"--root", root, "--root"}, 2, "--root needs a value", "", "twice"},
		{"second leading --root= has an empty value", []string{"--root", root, "--root="}, 2, "twice", "", ""},
		{"no value with equals is an empty root and a command is still needed", []string{"--root="}, 2, `--root "" needs a command`, "", ""},
		{"DIR but no command", []string{"--root", root}, 2, "needs a command", "", ""},
		{"ai takes no --root", []string{"--root", root, "ai"}, 2, "ai takes no --root", "", ""},
		{"ai --help is handled before the --root check", []string{"--root", root, "ai", "--help"}, 0, "", "usage: adj ai", ""},
		{"help after --root", []string{"--root", root, "--help"}, 0, "", "usage: adj", ""},
		{"short help after --root", []string{"--root", root, "-h"}, 0, "", "usage: adj", ""},
		{"unknown command after --root", []string{"--root", root, "frobnicate"}, 2, `unknown command "frobnicate"`, "", ""},
		{"value that looks like a flag is the DIR", []string{"--root", "--help", "list"}, 1, "--help", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(env, "", tt.args...)
			if r.code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", r.code, tt.wantCode, r.stderr)
			}
			if tt.wantStderr == "" && r.stderr != "" {
				t.Errorf("stderr = %q, want empty", r.stderr)
			}
			if tt.wantStderr != "" && !strings.Contains(r.stderr, tt.wantStderr) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantStderr, r.stderr)
			}
			if tt.forbidErr != "" && strings.Contains(r.stderr, tt.forbidErr) {
				t.Errorf("stderr contains %q, want no such text:\n%s", tt.forbidErr, r.stderr)
			}
			if tt.wantStdout == "" && r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if tt.wantStdout != "" && !strings.Contains(r.stdout, tt.wantStdout) {
				t.Errorf("stdout does not contain %q:\n%s", tt.wantStdout, r.stdout)
			}
		})
	}
}

func TestRootTwice(t *testing.T) {
	a, b := seedInbox(t), seedInbox(t)
	env := decoyEnv(t)
	tests := []struct {
		name string
		args []string
	}{
		{"before and after", []string{"--root", a, "list", "--root", b}},
		{"equals forms", []string{"--root=" + a, "list", "--root=" + b}},
		{"same directory twice", []string{"--root", a, "list", "--root", a}},
		{"before and after, ask", []string{"--root", a, "ask", "Never written", "--root", b}},
		{"twice before the command", []string{"--root", a, "--root", b, "list"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, a)
			r := run(env, "", tt.args...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			for _, want := range []string{"--root", "twice"} {
				if !strings.Contains(r.stderr, want) {
					t.Errorf("stderr does not contain %q:\n%s", want, r.stderr)
				}
			}
			if !hasUsageLine(r.stderr) {
				t.Errorf("stderr has no usage line:\n%s", r.stderr)
			}
			if !reflect.DeepEqual(before, snapshot(t, a)) {
				t.Errorf("the store changed")
			}
		})
	}

	t.Run("names both values", func(t *testing.T) {
		r := run(env, "", "--root", a, "list", "--root", b)
		// The message quotes each value (%q), which doubles a Windows
		// backslash.
		for _, want := range []string{strconv.Quote(a), strconv.Quote(b)} {
			if !strings.Contains(r.stderr, want) {
				t.Errorf("stderr does not name %q:\n%s", want, r.stderr)
			}
		}
	})

	t.Run("twice after the command still takes the last", func(t *testing.T) {
		// Not part of this slice: only a --root before the command is
		// checked against the one after it.
		r := run(env, "", "list", "--root", b, "--root", a)
		if r.code != 0 {
			t.Errorf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
		}
	})
}

func TestUnknownLeadingFlag(t *testing.T) {
	root := seedInbox(t)
	env := decoyEnv(t)
	tests := []struct {
		name string
		args []string
	}{
		{"--json", []string{"--json", "list"}},
		{"--status", []string{"--status", "open", "list"}},
		{"-v", []string{"-v", "list"}},
		{"unknown after --root", []string{"--root", root, "--json", "list"}},
		{"unknown before --root", []string{"--json", "--root", root, "list"}},
		{"bare dash", []string{"-", "list"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(env, "", tt.args...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !failureStdout(r.stdout, tt.args) {
				t.Errorf("stdout = %q, want empty (or the error object with --json)", r.stdout)
			}
			if !hasUsageLine(r.stderr) {
				t.Errorf("stderr has no usage line:\n%s", r.stderr)
			}
			if tt.name != "bare dash" && !strings.Contains(r.stderr, "only --root, --help and -h are accepted") {
				t.Errorf("stderr does not say which flags are accepted before the command:\n%s", r.stderr)
			}
		})
	}
}

func TestMainUsageShowsRootBeforeTheCommand(t *testing.T) {
	r := run(decoyEnv(t), "", "--help")
	if !strings.Contains(r.stdout, "usage: adj [--root DIR] <command>") {
		t.Errorf("usage does not show --root before the command:\n%s", r.stdout)
	}
	// S32: the usage names what the unknown-flag error names (--root, --help
	// and -h) and does not call --root the only flag accepted before the
	// command; adj --version is told apart as the version command.
	want := "--root DIR may come before the\ncommand or after it, but not both. The only other flags accepted before the\ncommand are --help and -h; adj --version prints the version.\n"
	if !strings.Contains(r.stdout, want) {
		t.Errorf("usage does not say %q:\n%s", want, r.stdout)
	}
}
