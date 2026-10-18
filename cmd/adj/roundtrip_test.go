// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// adjBinary is the path of the adj binary that TestMain built.
var adjBinary string

// adjFile is the name of the built binary: Windows runs only files with an
// executable extension.
var adjFile = func() string {
	if runtime.GOOS == "windows" {
		return "adj.exe"
	}
	return "adj"
}()

// buildAdj builds this package's command into dir and returns its path. It
// runs "go build" with the environment of the test run, so under
// scripts/env.sh (C33) the build's caches and temp files stay inside the repo.
func buildAdj(dir string) (string, error) {
	bin := filepath.Join(dir, adjFile)
	// The test runs in cmd/adj, so the module root is two levels up.
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/adj")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %v\n%s", err, out)
	}
	return bin, nil
}

// buildAdjWithVersion builds adj with the same ldflags override used for a
// release build (.github/workflows/release.yml).
func buildAdjWithVersion(dir, version string) (string, error) {
	bin := filepath.Join(dir, adjFile)
	cmd := exec.Command("go", "build", "-ldflags", "-X github.com/squalor-xyz/adjudicate/internal/cli.versionOverride="+version, "-o", bin, "./cmd/adj")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %v\n%s", err, out)
	}
	return bin, nil
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "adj-roundtrip-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "roundtrip_test:", err)
		os.Exit(1)
	}
	adjBinary, err = buildAdj(dir)
	code := 1
	if err != nil {
		fmt.Fprintln(os.Stderr, "roundtrip_test: build adj:", err)
	} else {
		code = m.Run()
	}
	os.RemoveAll(dir)
	os.Exit(code)
}

// adj runs the built binary in dir, with no ADJ_ variable but the two names
// the test sets, and returns its output and exit code. dir is always a temp
// dir: adj is never run against this repo.
func adj(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "ADJ_") {
			env = append(env, kv)
		}
	}
	env = append(env, "ADJ_ASKED_BY=agent", "ADJ_DECIDED_BY=owner")
	cmd := exec.Command(adjBinary, args...)
	cmd.Dir = dir
	cmd.Env = env
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		code = exit.ExitCode()
	default:
		t.Fatalf("adj %v: %v", args, err)
	}
	return out.String(), errOut.String(), code
}

// ok runs adj, requires exit 0, and returns stdout.
func ok(t *testing.T, dir string, args ...string) string {
	t.Helper()
	stdout, stderr, code := adj(t, dir, args...)
	if code != 0 {
		t.Fatalf("adj %v: exit %d, want 0; stderr:\n%s", args, code, stderr)
	}
	return stdout
}

// want fails the test unless out contains every substring.
func want(t *testing.T, step, out string, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if !strings.Contains(out, s) {
			t.Errorf("%s: output lacks %q:\n%s", step, s, out)
		}
	}
}

// wantNot fails the test if out contains any substring.
func wantNot(t *testing.T, step, out string, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if strings.Contains(out, s) {
			t.Errorf("%s: output has %q, want it gone:\n%s", step, s, out)
		}
	}
}

// TestV1AcceptanceRoundTrip walks the PROJECT.md acceptance criteria with the
// built binary in a fresh store: ask, inbox, decide, context, check; then a
// second decision that supersedes the first; then lint.
func TestV1AcceptanceRoundTrip(t *testing.T) {
	dir := t.TempDir()

	want(t, "init", ok(t, dir, "init"), "initialized")
	if _, err := os.Stat(filepath.Join(dir, ".adjudicate.json")); err != nil {
		t.Fatalf("init wrote no config: %v", err)
	}

	// ask prints the new ID; the question is in the inbox.
	if got := ok(t, dir, "ask", "Pick a cache", "--option", "mem:Keep it in memory", "--option", "disk:Keep it on disk",
		"--recommend", "mem", "--reasoning", "It is fast", "--key", "storage.cache", "--tag", "perf", "--ref", "S27"); got != "1\n" {
		t.Fatalf("first ask printed %q, want \"1\\n\"", got)
	}
	inbox := ok(t, dir, "inbox")
	want(t, "inbox after ask", inbox, "1  Pick a cache", "Refs: S27", "mem: Keep it in memory", "disk: Keep it on disk", "Recommendation: mem")
	want(t, "show --json after ask", ok(t, dir, "show", "1", "--json"), `"refs": [
    "S27"
  ]`)
	if out := ok(t, dir, "context"); out != "" {
		t.Errorf("context before any decision = %q, want nothing", out)
	}

	// decide moves it out of the inbox and into the context and check.
	if got := ok(t, dir, "decide", "1", "mem", "--why", "Speed matters more"); got != "1\n" {
		t.Fatalf("decide printed %q, want \"1\\n\"", got)
	}
	if out := ok(t, dir, "inbox"); out != "" {
		t.Errorf("inbox after decide = %q, want empty", out)
	}
	want(t, "context after decide", ok(t, dir, "context"), "- 1 Pick a cache [storage.cache]: mem", "Rationale: Speed matters more")
	checked := ok(t, dir, "check", "storage.cache")
	if first, _, _ := strings.Cut(checked, "\n"); first != "decided" {
		t.Errorf("check line 1 = %q, want \"decided\":\n%s", first, checked)
	}
	want(t, "check after decide", checked, "1  decided  Pick a cache", "Decision: mem")

	// A second decision on the same key supersedes the first.
	if got := ok(t, dir, "ask", "Pick a shared cache", "--option", "redis:Use Redis", "--key", "storage.cache"); got != "2\n" {
		t.Fatalf("second ask printed %q, want \"2\\n\"", got)
	}
	ok(t, dir, "decide", "2", "redis", "--why", "Several hosts share it")
	if got := ok(t, dir, "supersede", "1", "--by", "2"); got != "1\n" {
		t.Fatalf("supersede printed %q, want the old ID \"1\\n\"", got)
	}

	context := ok(t, dir, "context")
	want(t, "context after supersede", context, "- 2 Pick a shared cache [storage.cache]: redis")
	wantNot(t, "context after supersede", context, "Pick a cache", "Speed matters more")

	shown := ok(t, dir, "show", "1")
	want(t, "show of the superseded record", shown, "Status: superseded", "Superseded by: 2", "Decision: mem",
		"History:", " supersede: by 2")
	// The history is one "  DATE EVENT" line per entry, in order.
	if !regexp.MustCompile(`(?m)^History:\n  \d{4}-\d{2}-\d{2} ask\n  \d{4}-\d{2}-\d{2} decide\n  \d{4}-\d{2}-\d{2} supersede: by 2$`).MatchString(shown) {
		t.Errorf("show of the superseded record: history is not ask, decide, supersede: by 2:\n%s", shown)
	}
	want(t, "show of the replacement", ok(t, dir, "show", "2"), "Status: decided", "Supersedes: 1", " supersede: supersedes 1")

	checked = ok(t, dir, "check", "storage.cache")
	if first, _, _ := strings.Cut(checked, "\n"); first != "decided" {
		t.Errorf("check line 1 after supersede = %q, want \"decided\":\n%s", first, checked)
	}
	want(t, "check after supersede", checked, "2  decided  Pick a shared cache", "1  superseded  Pick a cache", "Superseded by: 2")

	// A clean store: lint prints nothing and exits 0.
	stdout, stderr, code := adj(t, dir, "lint")
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("lint: exit %d, stdout %q, stderr %q; want exit 0 and no output", code, stdout, stderr)
	}
}

func TestVersionLdflagsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	version := "v0.0.0-test"
	bin, err := buildAdjWithVersion(dir, version)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "--version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("adj --version: %v; stderr:\n%s", err, stderr.String())
	}
	if got := stdout.String(); got != version+"\n" {
		t.Errorf("adj --version = %q, want %q", got, version+"\n")
	}
	if stderr.Len() != 0 {
		t.Errorf("adj --version stderr = %q, want empty", stderr.String())
	}
}

// README.md has a "###" heading for every command that "adj --help" lists, so
// a new command cannot ship without usage docs.
func TestREADMEDocumentsEveryCommand(t *testing.T) {
	help := ok(t, t.TempDir(), "--help")
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	var headings []string
	for _, line := range strings.Split(string(readme), "\n") {
		if strings.HasPrefix(line, "### ") {
			headings = append(headings, line)
		}
	}
	listed := regexp.MustCompile(`(?m)^  adj ([a-z]+) `).FindAllStringSubmatch(help, -1)
	if len(listed) < 14 {
		t.Fatalf("adj --help lists %d commands, want at least 14:\n%s", len(listed), help)
	}
	for _, m := range listed {
		heading := regexp.MustCompile("`adj " + m[1] + "[ `]")
		found := false
		for _, h := range headings {
			found = found || heading.MatchString(h)
		}
		if !found {
			t.Errorf("README.md has no ### heading for `adj %s`", m[1])
		}
	}
}
