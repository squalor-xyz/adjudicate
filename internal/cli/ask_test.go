// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// initRoot returns a temp root that adj init has already set up.
func initRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if r := run(testEnv(root), "", "init"); r.code != 0 {
		t.Fatalf("init: exit %d: %s", r.code, r.stderr)
	}
	return root
}

// records lists the records in the store of the repo at root.
func records(t *testing.T, root string) []model.Record {
	t.Helper()
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	recs, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	return recs
}

func TestAskCreatesAnOpenRecord(t *testing.T) {
	root := initRoot(t)
	r := run(testEnv(root), "", "ask", "Pick a storage engine",
		"--option", " a : Use A ",
		"--option", "b:Use B: faster, but newer",
		"--tradeoffs", "b:Younger project: fewer tools",
		"--recommend", "a",
		"--reasoning", "A is proven",
		"--key", "storage.engine",
		"--tag", "t1", "--tag", "t2",
		"--context", "why we need to choose",
		"--by", "claude-opus")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	if r.stdout != "1\n" {
		t.Errorf("stdout = %q, want the ID and a newline", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	want := model.Record{
		ID:      1,
		Title:   "Pick a storage engine",
		Status:  model.StatusOpen,
		Key:     "storage.engine",
		Tags:    []string{"t1", "t2"},
		AskedBy: "claude-opus",
		Created: "2026-03-04",
		Context: "why we need to choose",
		Options: []model.Option{
			{Label: "a", Summary: "Use A"},
			{Label: "b", Summary: "Use B: faster, but newer", Tradeoffs: "Younger project: fewer tools"},
		},
		Recommendation: &model.Recommendation{Label: "a", Reasoning: "A is proven"},
		History:        []model.HistoryEntry{{Date: "2026-03-04", Event: model.EventAsk}},
	}
	got := records(t, root)
	if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("records = %+v\nwant one %+v", got, want)
	}
	file := filepath.Join(root, "decisions", "0001-pick-a-storage-engine.md")
	if _, err := os.Stat(file); err != nil {
		t.Errorf("record file: %v", err)
	}
}

func TestAskTitleOnlyAndSecondIDs(t *testing.T) {
	root := initRoot(t)
	env := testEnv(root)
	for i, title := range []string{"First", "Second"} {
		r := run(env, "", "ask", title)
		if r.code != 0 {
			t.Fatalf("ask %q: exit %d: %s", title, r.code, r.stderr)
		}
		if want := string(rune('1'+i)) + "\n"; r.stdout != want {
			t.Errorf("ask %q: stdout = %q, want %q", title, r.stdout, want)
		}
	}
	recs := records(t, root)
	if len(recs) != 2 || recs[0].Title != "First" || recs[1].Title != "Second" {
		t.Fatalf("records = %+v", recs)
	}
	if recs[1].Status != model.StatusOpen || recs[1].Options != nil || recs[1].Recommendation != nil {
		t.Errorf("title-only record = %+v, want open with no options or recommendation", recs[1])
	}
}

func TestAskAcceptsFlagsOnEitherSideOfTheTitle(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"flags after", []string{"ask", "Title", "--key", "k", "--tag", "x", "--option", "a:A"}},
		{"flags before", []string{"ask", "--key", "k", "--tag", "x", "--option", "a:A", "Title"}},
		{"flags around", []string{"ask", "--key", "k", "Title", "--tag", "x", "--option", "a:A"}},
		{"after a double dash", []string{"ask", "--key", "k", "--tag", "x", "--option", "a:A", "--", "Title"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 0 {
				t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
			}
			recs := records(t, root)
			if len(recs) != 1 {
				t.Fatalf("records = %+v, want one", recs)
			}
			g := recs[0]
			if g.Title != "Title" || g.Key != "k" || !reflect.DeepEqual(g.Tags, []string{"x"}) ||
				len(g.Options) != 1 || g.Options[0].Label != "a" {
				t.Errorf("record = %+v", g)
			}
		})
	}
}

func TestAskContextFromStdin(t *testing.T) {
	tests := []struct {
		name        string
		stdin       string
		wantContext string
	}{
		{"one line", "because\n", "because"},
		{"several lines keep inner newlines", "line one\n\nline three\n\n", "line one\n\nline three"},
		{"no trailing newline", "bare", "bare"},
		{"leading space is kept", "  indented\n\n", "  indented"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			r := run(testEnv(root), tt.stdin, "ask", "Title", "--stdin")
			if r.code != 0 {
				t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
			}
			recs := records(t, root)
			if len(recs) != 1 || recs[0].Context != tt.wantContext {
				t.Errorf("records = %+v, want context %q", recs, tt.wantContext)
			}
		})
	}
}

func TestAskTrimsTagsAndRecommendAndDropsRepeatedTags(t *testing.T) {
	root := initRoot(t)
	r := run(testEnv(root), "", "ask", "T", "--option", "a:A",
		"--recommend", " a ", "--tag", " t1 ", "--tag", "t2", "--tag", "t1", "--tag", "t2 ")
	if r.code != 0 {
		t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
	}
	recs := records(t, root)
	if len(recs) != 1 {
		t.Fatalf("records = %+v, want one", recs)
	}
	if want := []string{"t1", "t2"}; !reflect.DeepEqual(recs[0].Tags, want) {
		t.Errorf("tags = %q, want %q", recs[0].Tags, want)
	}
	if rec := recs[0].Recommendation; rec == nil || rec.Label != "a" {
		t.Errorf("recommendation = %+v, want label a", rec)
	}
}

// failReader fails the test if anything reads it.
type failReader struct{ t *testing.T }

func (f failReader) Read([]byte) (int, error) {
	f.t.Error("stdin was read")
	return 0, io.EOF
}

func TestAskStdinIsNotReadWhenTheRepoIsNotInitialized(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := RunWith(testEnv(root), []string{"ask", "T", "--stdin"}, failReader{t}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "adj init") {
		t.Errorf("exit code = %d, stderr = %q; want 1 and a hint to run adj init", code, stderr.String())
	}
	if got := snapshot(t, root); len(got) != 0 {
		t.Errorf("tree changed: %q", got)
	}
}

func TestAskStdinIsNotReadOnBadArguments(t *testing.T) {
	root := initRoot(t)
	var stdout, stderr bytes.Buffer
	code := RunWith(testEnv(root), []string{"ask", "T", "--stdin", "--recommend", "z"}, failReader{t}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
}

func TestAskAskedBySource(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"flag", "", []string{"--by", "flag-name"}, "flag-name"},
		{"environment", "env-name", nil, "env-name"},
		{"flag beats environment", "env-name", []string{"--by", "flag-name"}, "flag-name"},
		{"neither", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			env := testEnv(root)
			env.Getenv = func(k string) string {
				if k == "ADJ_ASKED_BY" {
					return tt.env
				}
				return ""
			}
			r := run(env, "", append([]string{"ask", "Title"}, tt.args...)...)
			if r.code != 0 {
				t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
			}
			if recs := records(t, root); len(recs) != 1 || recs[0].AskedBy != tt.want {
				t.Errorf("records = %+v, want asked_by %q", recs, tt.want)
			}
		})
	}
}

func TestAskRejectsBadInputAndWritesNothing(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		args    []string
		wantErr string
	}{
		{"recommend is not an option", "",
			[]string{"T", "--option", "a:A", "--recommend", "z"}, `"z"`},
		{"recommend with no options", "",
			[]string{"T", "--recommend", "a"}, `"a"`},
		{"option without a colon", "",
			[]string{"T", "--option", "just-a-label"}, "LABEL:SUMMARY"},
		{"option with an empty label", "",
			[]string{"T", "--option", ":summary"}, "label"},
		{"duplicate option label", "",
			[]string{"T", "--option", "a:A", "--option", "a:Again"}, `"a"`},
		{"tradeoffs without a colon", "",
			[]string{"T", "--option", "a:A", "--tradeoffs", "a"}, "LABEL:TEXT"},
		{"tradeoffs for an unknown option", "",
			[]string{"T", "--option", "a:A", "--tradeoffs", "q:text"}, `"q"`},
		{"tradeoffs twice for one option", "",
			[]string{"T", "--option", "a:A", "--tradeoffs", "a:x", "--tradeoffs", "a:y"}, `"a"`},
		{"reasoning without recommend", "",
			[]string{"T", "--option", "a:A", "--reasoning", "because"}, "--recommend"},
		{"no title", "", []string{}, "TITLE"},
		{"blank title", "", []string{"   "}, "title"},
		{"two titles", "", []string{"One", "Two"}, "quote"},
		{"multi-line title", "", []string{"One\nTwo"}, "title"},
		{"blank tag", "", []string{"T", "--tag", " "}, "tag"},
		{"blank recommend", "",
			[]string{"T", "--option", "a:A", "--recommend", " "}, "--recommend"},
		{"context and stdin", "ctx",
			[]string{"T", "--context", "x", "--stdin"}, "--stdin"},
		{"unknown flag", "", []string{"T", "--nope"}, "nope"},
		{"flag without a value", "", []string{"T", "--key"}, "key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			before := snapshot(t, root)
			r := run(testEnv(root), tt.stdin, append([]string{"ask"}, tt.args...)...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantErr, r.stderr)
			}
			if !strings.Contains(r.stderr, "usage: adj ask") {
				t.Errorf("stderr has no ask usage:\n%s", r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestAskBeforeInitSaysRunInit(t *testing.T) {
	// An explicit root with no config: nothing is discovered or created.
	root := t.TempDir()
	r := run(testEnv(root), "", "ask", "Title", "--option", "a:A")
	if r.code != 1 {
		t.Errorf("exit code = %d, want 1", r.code)
	}
	if !strings.Contains(r.stderr, "adj init") {
		t.Errorf("stderr does not tell the user to run adj init:\n%s", r.stderr)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if got := snapshot(t, root); len(got) != 0 {
		t.Errorf("ask before init created %q", got)
	}
}

func TestAskTreatsAConfigDirectoryAsNotInitialized(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".adjudicate.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	r := run(testEnv(root), "", "ask", "Title")
	if r.code != 1 || !strings.Contains(r.stderr, "adj init") {
		t.Errorf("exit code = %d, stderr = %q; want 1 and a hint to run adj init", r.code, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed: %q", after)
	}
}

func TestAskBadInputBeatsMissingInit(t *testing.T) {
	root := t.TempDir()
	r := run(testEnv(root), "", "ask", "T", "--recommend", "z")
	if r.code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error before the init check)", r.code)
	}
}

func TestAskFindsTheRootFromTheWorkingDirectory(t *testing.T) {
	// The walk up from the working directory is bounded by Env.Stop, which is
	// inside the temp dir, so it never reaches this repo's own config.
	tmp := t.TempDir()
	if r := run(testEnv(tmp), "", "init"); r.code != 0 {
		t.Fatalf("init: exit %d: %s", r.code, r.stderr)
	}
	deep := filepath.Join(tmp, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	env := testEnv("")
	env.Stop = tmp
	env.Getwd = func() (string, error) { return deep, nil }

	if r := run(env, "", "ask", "Found it"); r.code != 0 {
		t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
	}
	if recs := records(t, tmp); len(recs) != 1 || recs[0].Title != "Found it" {
		t.Errorf("records = %+v", recs)
	}
	if got := snapshot(t, deep); len(got) != 0 {
		t.Errorf("working directory changed: %q", got)
	}
}

func TestAskNeverSearchesAboveStop(t *testing.T) {
	// A repo above the stop directory is out of reach: the walk ends at
	// work, so ask finds no root even though outer holds a config.
	outer := t.TempDir()
	if r := run(testEnv(outer), "", "init"); r.code != 0 {
		t.Fatalf("init: exit %d: %s", r.code, r.stderr)
	}
	work := filepath.Join(outer, "work")
	deep := filepath.Join(work, "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, outer)
	env := testEnv("")
	env.Stop = work
	env.Getwd = func() (string, error) { return deep, nil }

	r := run(env, "", "ask", "Lost")
	if r.code != 1 {
		t.Errorf("exit code = %d, want 1", r.code)
	}
	if !strings.Contains(r.stderr, "adj init") {
		t.Errorf("stderr does not tell the user to run adj init:\n%s", r.stderr)
	}
	if after := snapshot(t, outer); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed: %q", after)
	}
}

func TestAskRootFlagNamesTheRepo(t *testing.T) {
	root := initRoot(t)
	other := t.TempDir() // the injected default; the flag must win
	r := run(testEnv(other), "", "ask", "Title", "--root", root)
	if r.code != 0 {
		t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
	}
	if recs := records(t, root); len(recs) != 1 {
		t.Errorf("records = %+v, want one", recs)
	}
	if got := snapshot(t, other); len(got) != 0 {
		t.Errorf("injected root changed: %q", got)
	}
}

func TestAskUsesTheConfiguredStore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), []byte(`{"store": "log/adr"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := run(testEnv(root), "", "ask", "Where"); r.code != 0 {
		t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "log", "adr", "0001-where.md")); err != nil {
		t.Error(err)
	}
}

func TestAskHelp(t *testing.T) {
	root := initRoot(t)
	before := snapshot(t, root)
	r := run(testEnv(root), "", "ask", "--help")
	if r.code != 0 {
		t.Errorf("exit code = %d, want 0", r.code)
	}
	if !strings.Contains(r.stdout, "usage: adj ask") {
		t.Errorf("stdout = %q, want the ask usage", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("help changed the tree")
	}
}

func TestUsageListsInitAndAsk(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	for _, want := range []string{"adj init", "adj ask"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("usage does not mention %q:\n%s", want, r.stdout)
		}
	}
}
