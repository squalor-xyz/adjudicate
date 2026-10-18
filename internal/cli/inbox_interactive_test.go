// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

func interactiveRoot(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	for _, title := range []string{"First", "Second"} {
		if r := run(testEnv(root), "", "ask", title, "--option", "a:Use A"); r.code != 0 {
			t.Fatal(r)
		}
	}
	return root
}

func TestInteractiveOptionAndSkip(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "environment", true: "flag"}[explicit], func(t *testing.T) {
			root := interactiveRoot(t)
			env := testEnv(root)
			env.Getenv = func(k string) string {
				if k == "ADJ_DECIDED_BY" {
					return "Env Owner"
				}
				return ""
			}
			args := []string{"inbox", "-i"}
			by := "Env Owner"
			if explicit {
				args = append(args, "--by", "Flag Owner")
				by = "Flag Owner"
			}
			before := records(t, root)
			want := append([]model.Record(nil), before...)
			if err := model.Decide(&want[0], "a", "  rationale as typed  ", by, fixedNow().Format("2006-01-02")); err != nil {
				t.Fatal(err)
			}
			r := run(env, "d\n a \n  rationale as typed  \ns\n", args...)
			if r.code != 0 || r.stderr != "" {
				t.Errorf("walk: %+v", r)
			}
			if got := records(t, root); !reflect.DeepEqual(got, want) {
				t.Errorf("records = %+v; want %+v", got, want)
			}
		})
	}
}

func TestInteractiveMatchesOneShot(t *testing.T) {
	for _, tc := range []struct {
		name, script, summary string
		args                  []string
	}{
		{"text", "t\n  typed answer  \n  because  \nq\n", "1 decided, 0 rejected, 0 withdrawn, 0 skipped, 0 failed\n", []string{"decide", "1", "--text", "  typed answer  "}},
		{"reject", "r\n  because  \nq\n", "0 decided, 1 rejected, 0 withdrawn, 0 skipped, 0 failed\n", []string{"reject", "1"}},
		{"withdraw", "w\n  because  \nq\n", "0 decided, 0 rejected, 1 withdrawn, 0 skipped, 0 failed\n", []string{"withdraw", "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, control := interactiveRoot(t), interactiveRoot(t)
			args := append(tc.args, "--why", "  because  ", "--by", "Owner")
			if r := run(testEnv(control), "", args...); r.code != 0 {
				t.Fatal(r)
			}
			r := run(testEnv(root), tc.script, "inbox", "-i", "--by", "Owner")
			if r.code != 0 || r.stderr != "" {
				t.Errorf("walk: %+v", r)
			}
			if !strings.HasSuffix(r.stdout, "\n"+tc.summary) {
				t.Errorf("summary: stdout %q; want final line %q", r.stdout, tc.summary)
			}
			if got, want := records(t, root), records(t, control); !reflect.DeepEqual(got, want) {
				t.Errorf("records = %+v; want %+v", got, want)
			}
		})
	}
}

func TestInteractiveReprompts(t *testing.T) {
	for _, tc := range []struct{ name, script, diagnostic, prompt string }{
		{"label", "d\nmissing\na\nwhy\nq\n", `OPTION "missing" is not one of the options of record 1`, "Option label: "},
		{"rationale", "d\na\n   \nwhy\nq\n", "Rationale must not be blank", "Rationale: "},
		{"text", "t\n   \nanswer\nwhy\nq\n", "Decision text must not be blank", "Decision text: "},
		{"action", "bad\nd\na\nwhy\nq\n", "Unknown action", "Action ["},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := interactiveRoot(t)
			r := run(testEnv(root), tc.script, "inbox", "-i")
			wantPrompts := 2
			if tc.name == "action" {
				wantPrompts = 3
			}
			if r.code != 0 || !strings.Contains(r.stderr, tc.diagnostic) || strings.Count(r.stdout, tc.prompt) != wantPrompts {
				t.Errorf("reprompt: %+v", r)
			}
			decision := "a"
			if tc.name == "text" {
				decision = "answer"
			}
			got := records(t, root)
			if got[0].Decision != decision || got[0].Rationale != "why" || len(got[0].History) != 2 || got[1].Status != model.StatusOpen {
				t.Errorf("records: %+v", got)
			}
		})
	}
}

func TestInteractiveQuitAndEOF(t *testing.T) {
	for _, script := range []string{"q\n", "", "d\n", "d\na\n", "t\nanswer\n", "r\n", "w\n", "d\na\npartial"} {
		t.Run(strings.ReplaceAll(script, "\n", "/"), func(t *testing.T) {
			root := interactiveRoot(t)
			before := snapshot(t, root)
			r := run(testEnv(root), script, "inbox", "-i")
			if r.code != 0 || r.stderr != "" || strings.Contains(r.stdout, "2  Second") {
				t.Errorf("quit: %+v", r)
			}
			if got := snapshot(t, root); !reflect.DeepEqual(got, before) {
				t.Errorf("quit wrote files: %v", got)
			}
		})
	}
}

// The callback runs after the first record is displayed, just before stdin is read.
type interactiveReader struct {
	before func()
	reader io.Reader
}

func (r *interactiveReader) Read(p []byte) (int, error) {
	if r.before != nil {
		f := r.before
		r.before = nil
		f()
	}
	return r.reader.Read(p)
}

func TestInteractiveChangedStatus(t *testing.T) {
	root := interactiveRoot(t)
	var out, errOut bytes.Buffer
	input := &interactiveReader{reader: strings.NewReader("d\na\nwhy\ns\n"), before: func() {
		if !strings.Contains(out.String(), "1  First") {
			t.Fatal("callback before display")
		}
		if r := run(testEnv(root), "", "decide", "1", "a", "--why", "other owner"); r.code != 0 {
			t.Fatal(r)
		}
	}}
	code := RunWith(testEnv(root), []string{"inbox", "-i"}, input, &out, &errOut)
	if code != 1 || errOut.String() != "adj inbox: record 1 is decided; only an open record can be decided\n" || !strings.Contains(out.String(), "2  Second") || !strings.HasSuffix(out.String(), "0 decided, 0 rejected, 0 withdrawn, 1 skipped, 1 failed\n") {
		t.Errorf("changed status: exit %d, out %q, err %q", code, out.String(), errOut.String())
	}
	got := records(t, root)
	if got[0].Rationale != "other owner" || len(got[0].History) != 2 || got[1].Status != model.StatusOpen {
		t.Errorf("records: %+v", got)
	}
}

func TestInteractiveEmpty(t *testing.T) {
	if r := run(testEnv(initRoot(t)), "", "inbox", "-i"); r != (result{}) {
		t.Errorf("empty: %+v", r)
	}
}

func TestInteractiveUsage(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		message string
	}{
		{[]string{"-i", "--json"}, "-i cannot be combined with --json or --all"},
		{[]string{"-i", "--all"}, "-i cannot be combined with --json or --all"},
		{[]string{"-i", "argument"}, "inbox takes no arguments"},
		{[]string{"--by", "Owner"}, "--by requires -i"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			r := run(testEnv(initRoot(t)), "", append([]string{"inbox"}, tc.args...)...)
			if r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, tc.message) {
				t.Errorf("usage: %+v", r)
			}
		})
	}
}

func TestInteractiveGolden(t *testing.T) {
	root := interactiveRoot(t)
	r := run(testEnv(root), "?\nd\na\nwhy\ns\n", "inbox", "-i")
	const want = `1  First
  Created: 2026-03-04
  Options:
    a: Use A
Action [d=option, t=text, r=reject, w=withdraw, s=skip, q=quit, ?=help]: d: decide by option label (blank returns to action; no options: use t); t: decide with free text; r: reject; w: withdraw; s: skip; q: quit; ?: help
Action [d=option, t=text, r=reject, w=withdraw, s=skip, q=quit, ?=help]: Option label: Rationale: 
2  Second
  Created: 2026-03-04
  Options:
    a: Use A
Action [d=option, t=text, r=reject, w=withdraw, s=skip, q=quit, ?=help]: 
1 decided, 0 rejected, 0 withdrawn, 1 skipped, 0 failed
`
	if r != (result{stdout: want}) {
		t.Errorf("golden: %+v; want stdout %q", r, want)
	}
}

type interactiveReadError struct{}

func (interactiveReadError) Read([]byte) (int, error) { return 0, errors.New("input failed") }
func TestInteractiveReadError(t *testing.T) {
	root := interactiveRoot(t)
	before := snapshot(t, root)
	var out, errOut bytes.Buffer
	code := RunWith(testEnv(root), []string{"inbox", "-i"}, interactiveReadError{}, &out, &errOut)
	if code != 1 || !strings.Contains(errOut.String(), "input failed") {
		t.Errorf("read error: %d %q", code, errOut.String())
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, before) {
		t.Errorf("read error wrote: %v", got)
	}
}

func TestInteractiveNoOptionsReturnsToAction(t *testing.T) {
	root := initRoot(t)
	if r := run(testEnv(root), "", "ask", "Free", "--recommend-text", "Use text"); r.code != 0 {
		t.Fatal(r)
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "d\ns\n", "inbox", "-i")
	want := result{
		stdout: "1  Free\n  Created: 2026-03-04\n  Recommendation: Use text\n" +
			inboxActionPrompt + inboxActionPrompt + "\n0 decided, 0 rejected, 0 withdrawn, 1 skipped, 0 failed\n",
		stderr: "adj inbox: record 1 has no options; use t to decide with text\n",
	}
	if r != want {
		t.Errorf("no-options action: %+v; want %+v", r, want)
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, before) {
		t.Errorf("refused option action wrote files: %v; want %v", got, before)
	}
}

func TestInteractiveBlankLabelReturnsToAction(t *testing.T) {
	root := initRoot(t)
	if r := run(testEnv(root), "", "ask", "First", "--option", "a:Use A"); r.code != 0 {
		t.Fatal(r)
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "d\n \t \ns\n", "inbox", "-i")
	want := result{stdout: "1  First\n  Created: 2026-03-04\n  Options:\n    a: Use A\n" +
		inboxActionPrompt + "Option label: " + inboxActionPrompt + "\n0 decided, 0 rejected, 0 withdrawn, 1 skipped, 0 failed\n"}
	if r != want {
		t.Errorf("blank-label action: %+v; want %+v", r, want)
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, before) {
		t.Errorf("cancelled option action wrote files: %v; want %v", got, before)
	}
}
