// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/commands"
)

// maxSnippetBytes keeps the snippet cheap to paste into an agent's AGENTS.md.
const maxSnippetBytes = 1800

func TestAIPrintsTheSnippet(t *testing.T) {
	// No repo is needed: the directory has no config file.
	r := run(testEnv(t.TempDir()), "", "ai")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	if r.stdout != commands.AgentSnippet() {
		t.Errorf("stdout is not the snippet:\n%s", r.stdout)
	}
	if !strings.HasPrefix(r.stdout, "## ") || !strings.HasSuffix(r.stdout, "\n") || strings.HasSuffix(r.stdout, "\n\n") {
		t.Errorf("snippet should start with a ## heading and end with one newline:\n%q", r.stdout)
	}
	if len(r.stdout) > maxSnippetBytes {
		t.Errorf("snippet is %d bytes, want at most %d: it is pasted into AGENTS.md", len(r.stdout), maxSnippetBytes)
	}
}

func TestAISnippetNamesTheReadCommandsAndSaysOnlyTheOwnerDecides(t *testing.T) {
	snippet := run(testEnv(t.TempDir()), "", "ai").stdout
	for _, want := range []string{"adj check", "adj ask", "adj inbox", "adj context", "adj show", "adj withdraw"} {
		if !strings.Contains(snippet, want) {
			t.Errorf("snippet does not name %q:\n%s", want, snippet)
		}
	}
	// C27: the owner-only rule, and the commands an agent must leave alone.
	for _, want := range []string{"only the owner decides", "never run `adj decide`", "`adj reject`"} {
		if !strings.Contains(snippet, want) {
			t.Errorf("snippet does not say %q:\n%s", want, snippet)
		}
	}
	// The exit codes and the environment variable that attributes a question.
	for _, want := range []string{"ADJ_ASKED_BY", "Exit codes: 0 success, 1 ", "2 usage error"} {
		if !strings.Contains(snippet, want) {
			t.Errorf("snippet does not mention %q:\n%s", want, snippet)
		}
	}
	// No ask-for-decision flow is advertised: no agent line starts a decide.
	for _, line := range strings.Split(snippet, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "adj decide") {
			t.Errorf("snippet line %q tells an agent to decide", line)
		}
	}
}

var (
	snippetCommand = regexp.MustCompile(`\badj ([a-z][a-z-]*)`)
	snippetSpan    = regexp.MustCompile("`([^`]+)`")
	snippetFlag    = regexp.MustCompile(`--[a-z][a-z-]*`)
	snippetEnv     = regexp.MustCompile(`\bADJ_[A-Z_]+`)
)

// helpHasFlag reports whether the usage text lists flag as a whole word: --key
// is not found in --key-prefix, nor --reason in --reasoning.
func helpHasFlag(help, flag string) bool {
	return regexp.MustCompile(`(^|[^a-z-])` + regexp.QuoteMeta(flag) + `([^a-z-]|$)`).MatchString(help)
}

func TestHelpHasFlagMatchesWholeWords(t *testing.T) {
	help := "  --key-prefix P   a prefix\n  --reasoning TEXT why\n"
	for flag, want := range map[string]bool{"--key-prefix": true, "--reasoning": true, "--key": false, "--reason": false, "--prefix": false} {
		if got := helpHasFlag(help, flag); got != want {
			t.Errorf("helpHasFlag(%q) = %v, want %v", flag, got, want)
		}
	}
}

// The snippet cannot drift from the CLI: every command it names exists, every
// flag in a command it shows exists on that command, and every environment
// variable it names is in some command's usage.
func TestAISnippetOnlyNamesRealCommandsFlagsAndVariables(t *testing.T) {
	env := testEnv(t.TempDir())
	snippet := run(env, "", "ai").stdout
	helps := map[string]string{} // command name -> its --help text
	help := func(cmd string) string {
		if h, ok := helps[cmd]; ok {
			return h
		}
		r := run(env, "", cmd, "--help")
		if r.code != 0 || !strings.Contains(r.stdout, "usage: adj "+cmd) {
			t.Errorf("snippet names `adj %s`, but `adj %s --help` exits %d with %q", cmd, cmd, r.code, r.stdout)
		}
		helps[cmd] = r.stdout
		return r.stdout
	}

	for _, m := range snippetCommand.FindAllStringSubmatch(snippet, -1) {
		help(m[1])
	}
	var named []string
	for cmd := range helps {
		named = append(named, cmd)
	}
	sort.Strings(named)
	for _, want := range []string{"ask", "check", "context", "inbox", "list", "show", "withdraw"} {
		if _, ok := helps[want]; !ok {
			t.Errorf("snippet names only %v, want %q among them", named, want)
		}
	}

	// A flag in any code span belongs to the nearest `adj CMD` span before it
	// (so "or pass `--by NAME`" is checked against the command it follows).
	flags := 0
	cmd := ""
	for _, m := range snippetSpan.FindAllStringSubmatch(snippet, -1) {
		if fields := strings.Fields(m[1]); len(fields) >= 2 && fields[0] == "adj" {
			cmd = fields[1]
		}
		for _, flag := range snippetFlag.FindAllString(m[1], -1) {
			flags++
			if cmd == "" {
				t.Errorf("`%s`: flag %s comes before any `adj CMD`", m[1], flag)
			} else if !helpHasFlag(help(cmd), flag) {
				t.Errorf("`%s` (after `adj %s`): `adj %s --help` does not list %s", m[1], cmd, cmd, flag)
			}
		}
	}
	if flags < 10 {
		t.Errorf("checked only %d flags in the snippet's commands, want at least 10", flags)
	}

	var allHelp strings.Builder
	for _, h := range helps {
		allHelp.WriteString(h)
	}
	vars := snippetEnv.FindAllString(snippet, -1)
	if len(vars) == 0 {
		t.Error("snippet names no ADJ_ environment variable")
	}
	for _, v := range vars {
		if !strings.Contains(allHelp.String(), v) {
			t.Errorf("snippet names %s, but no command it names mentions it in its usage", v)
		}
	}
}

// The snippet says what to do for each result of `adj check`, and the words
// are the ones check prints and documents.
func TestAISnippetGuidesEachCheckResult(t *testing.T) {
	env := testEnv(t.TempDir())
	snippet := run(env, "", "ai").stdout
	checkHelp := run(env, "", "check", "--help").stdout
	for _, word := range []string{"decided", "open", "closed", "not found"} {
		if !strings.Contains(snippet, "`"+word+"`") {
			t.Errorf("snippet does not give the check result `%s`", word)
		}
		if !strings.Contains(snippet, "If `"+word+"`") {
			t.Errorf("snippet gives no guidance after \"If `%s`\"", word)
		}
		if !strings.Contains(checkHelp, word) {
			t.Errorf("adj check --help does not mention %s", word)
		}
	}
	// A closed question may have been rejected: read why, and do not re-ask.
	_, afterClosed, _ := strings.Cut(snippet, "If `closed`")
	closed, _, _ := strings.Cut(afterClosed, "If `not found`")
	for _, want := range []string{"rejected", "`adj show ID`", "do not re-ask a rejected question"} {
		if !strings.Contains(closed, want) {
			t.Errorf("the guidance for `closed` does not say %q:\n%s", want, closed)
		}
	}
}

// check exits 0 for every completed check, as the snippet says, and line 1 is
// the result it names.
func TestAISnippetCheckExitsZeroForEveryResult(t *testing.T) {
	snippet := run(testEnv(t.TempDir()), "", "ai").stdout
	if !strings.Contains(snippet, "the exit code is 0 for all four") {
		t.Fatalf("snippet no longer says check exits 0 for all four results:\n%s", snippet)
	}
	root := seedCheck(t) // 2 decided storage.cache, 1 open storage.engine, 3 rejected ui.theme
	tests := []struct{ query, want string }{
		{"storage.cache", "decided"},
		{"storage.engine", "open"},
		{"ui.theme", "closed"},
		{"no-such-topic", "not found"},
	}
	for _, tt := range tests {
		r := run(testEnv(root), "", "check", tt.query)
		if first, _, _ := strings.Cut(r.stdout, "\n"); r.code != 0 || first != tt.want {
			t.Errorf("check %q: exit %d, first line %q; want exit 0 and %q", tt.query, r.code, first, tt.want)
		}
	}
}

// The snippet's exit-code line gives three failures that exit 1 and a usage
// error that exits 2; each really does.
func TestAISnippetExitCodesAreTheRealOnes(t *testing.T) {
	snippet := run(testEnv(t.TempDir()), "", "ai").stdout
	if want := "Exit codes: 0 success, 1 failed (no repo, unknown ID, record not open), 2 usage error."; !strings.Contains(snippet, want) {
		t.Fatalf("snippet no longer says %q:\n%s", want, snippet)
	}
	root := seedCheck(t) // record 2 is decided
	tests := []struct {
		name string
		env  Env
		args []string
		want int
	}{
		{"no repo", testEnv(t.TempDir()), []string{"check", "x"}, 1},
		{"unknown ID", testEnv(root), []string{"show", "99"}, 1},
		{"record not open", testEnv(root), []string{"withdraw", "2", "--why", "late"}, 1},
		{"usage error", testEnv(root), []string{"check"}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if r := run(tt.env, "", tt.args...); r.code != tt.want {
				t.Errorf("%v: exit %d, want %d; stderr:\n%s", tt.args, r.code, tt.want, r.stderr)
			}
		})
	}
}

// What an agent may and may not do: withdraw only its own question, say who
// withdrew it, leave the owner's variable alone, and leave init to the owner.
func TestAISnippetLimitsWhatAgentsDo(t *testing.T) {
	snippet := run(testEnv(t.TempDir()), "", "ai").stdout
	for _, want := range []string{
		"Withdraw your own question when you no longer need it: `adj withdraw ID --why \"Reason\" --by NAME`",
		"never another agent's or the owner's",
		"no repo, tell the owner; do not run `adj init`",
	} {
		if !strings.Contains(snippet, want) {
			t.Errorf("snippet does not say %q:\n%s", want, snippet)
		}
	}
	if strings.Contains(snippet, "ADJ_DECIDED_BY") {
		t.Error("snippet names ADJ_DECIDED_BY, the owner's variable")
	}
}

// S12 gate note: supersede changes a decision, so it is on the never-run list
// (C27). The drift test checks that a command exists, not that it is listed.
func TestAISnippetNeverRunListNamesSupersede(t *testing.T) {
	snippet := run(testEnv(t.TempDir()), "", "ai").stdout
	_, after, ok := strings.Cut(snippet, "never run ")
	if !ok {
		t.Fatalf("snippet has no \"never run\" list:\n%s", snippet)
	}
	list, _, _ := strings.Cut(after, ".")
	for _, want := range []string{"`adj decide`", "`adj reject`", "`adj supersede`"} {
		if !strings.Contains(list, want) {
			t.Errorf("the never-run list %q does not name %s", list, want)
		}
	}
	// Only the owner supersedes: no agent line tells an agent to.
	for _, line := range strings.Split(snippet, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "adj supersede") {
			t.Errorf("snippet line %q tells an agent to supersede", line)
		}
	}
}

func TestAIHelpAndBadArguments(t *testing.T) {
	env := testEnv(t.TempDir())
	for _, flag := range []string{"--help", "-h"} {
		r := run(env, "", "ai", flag)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj ai\n") || r.stderr != "" {
			t.Errorf("ai %s: exit %d, stdout %q, stderr %q; want exit 0 and usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"positional argument", []string{"ai", "extra"}, "no arguments"},
		{"unknown flag", []string{"ai", "--nope"}, "nope"},
		{"--root is not an ai flag", []string{"ai", "--root", "x"}, "root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(env, "", tt.args...)
			if r.code != 2 || r.stdout != "" {
				t.Errorf("exit %d, stdout %q; want exit 2 and no stdout", r.code, r.stdout)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj ai\n") {
				t.Errorf("stderr should contain %q and the ai usage:\n%s", tt.wantErr, r.stderr)
			}
		})
	}
}

func TestAIListedInTheMainUsage(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(r.stdout, "adj ai ") {
		t.Errorf("main usage does not list adj ai:\n%s", r.stdout)
	}
}

// S07 gate: "who decided" reads oddly for a withdrawal; the asker withdraws.
func TestWithdrawAndRejectUsageDescribeWhoActed(t *testing.T) {
	env := testEnv(t.TempDir())
	w := run(env, "", "withdraw", "--help").stdout
	if strings.Contains(w, "who decided") || !strings.Contains(w, "--by NAME      who withdrew it;") {
		t.Errorf("withdraw usage should describe --by as who withdrew it:\n%s", w)
	}
	rj := run(env, "", "reject", "--help").stdout
	if !strings.Contains(rj, "--by NAME      who decided;") {
		t.Errorf("reject usage should keep describing --by as who decided:\n%s", rj)
	}
}
