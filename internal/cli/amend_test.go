// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// amendDate is the day the amends in these tests run; the records were asked
// and decided on 2026-03-04 (the clock every other test uses).
const amendDate = "2026-10-17"

// seedAmend makes a repo with three records:
//
//	1 open      "Pick a store", a multi-line context, options a (with tradeoffs) and b
//	2 decided   "Cache layer", option redis with tradeoffs, decided by owner
//	3 rejected  "Theme"
func seedAmend(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--option", "b:Use B", "--tradeoffs", "a:slower",
			"--recommend", "a", "--reasoning", "A is proven", "--key", "storage.engine", "--tag", "infra",
			"--by", "claude-opus", "--context", "first line\nsecond line"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--tradeoffs", "redis:one more service",
			"--key", "storage.cache", "--context", "why a cache"},
		{"ask", "Theme", "--option", "dark:Dark"},
		{"decide", "2", "redis", "--why", "fast\nand shared", "--by", "owner"},
		{"reject", "3", "--why", "not needed"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	return root
}

// amendOK runs `adj amend ID args...` on the day amendDate, expects exit 0, the
// ID on stdout and nothing on stderr, and expects the stored record to be what
// it was before with change applied and exactly one more history entry
// (dated amendDate) with the note and the old text. It does not rename the
// file.
func amendOK(t *testing.T, root string, id int, stdin string, args []string, change func(*model.Record), note string, was model.Was) {
	t.Helper()
	before := recordByID(t, root, id)
	treeBefore := snapshot(t, root)
	r := run(envOn(t, root, amendDate), stdin, append([]string{"amend", itoa(id)}, args...)...)
	if r.code != 0 {
		t.Fatalf("amend %d %v: exit %d, want 0; stderr:\n%s", id, args, r.code, r.stderr)
	}
	if r.stdout != itoa(id)+"\n" || r.stderr != "" {
		t.Errorf("output = (%q, %q), want the ID and no stderr", r.stdout, r.stderr)
	}
	want := before
	want.Options = append([]model.Option(nil), before.Options...)
	change(&want)
	want.History = append(append([]model.HistoryEntry(nil), before.History...),
		model.HistoryEntry{Date: amendDate, Event: model.EventAmend, Note: note, Was: &was})
	if got := recordByID(t, root, id); !reflect.DeepEqual(got, want) {
		t.Errorf("record %d after amend\n got %+v\nwant %+v", id, got, want)
	}
	after := snapshot(t, root)
	if len(after) != len(treeBefore) {
		t.Errorf("amend added or removed files:\nbefore %v\nafter  %v", keysOfSnapshot(treeBefore), keysOfSnapshot(after))
	}
	for name := range treeBefore {
		if _, ok := after[name]; !ok {
			t.Errorf("amend renamed or removed %s", name)
		}
	}
}

func keysOfSnapshot(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func strp(s string) *string { return &s }

func TestAmendTitle(t *testing.T) {
	for _, id := range []int{1, 2, 3} { // open, decided, rejected
		root := seedAmend(t)
		old := recordByID(t, root, id).Title
		amendOK(t, root, id, "", []string{"--title", "A better title"},
			func(r *model.Record) { r.Title = "A better title" }, "title", model.Was{Title: strp(old)})
	}
}

func TestAmendContext(t *testing.T) {
	t.Run("flag, open and decided", func(t *testing.T) {
		for _, id := range []int{1, 2} {
			root := seedAmend(t)
			old := recordByID(t, root, id).Context
			amendOK(t, root, id, "", []string{"--context", "new\ncontext"},
				func(r *model.Record) { r.Context = "new\ncontext" }, "context", model.Was{Context: strp(old)})
		}
	})
	t.Run("a context added to a record that had none", func(t *testing.T) {
		root := seedAmend(t)
		amendOK(t, root, 3, "", []string{"--context", "now with a context"},
			func(r *model.Record) { r.Context = "now with a context" }, "context", model.Was{Context: strp("")})
	})
	t.Run("standard input, trailing line breaks dropped as ask does", func(t *testing.T) {
		root := seedAmend(t)
		old := recordByID(t, root, 1).Context
		amendOK(t, root, 1, "from\nstdin\n\n", []string{"--stdin"},
			func(r *model.Record) { r.Context = "from\nstdin" }, "context", model.Was{Context: strp(old)})
	})
	t.Run("the old multi-line text is in the file", func(t *testing.T) {
		root := seedAmend(t)
		amendOK(t, root, 1, "", []string{"--context", "x"}, func(r *model.Record) { r.Context = "x" }, "context",
			model.Was{Context: strp("first line\nsecond line")})
		if got := recordByID(t, root, 1).History[1].Was; got == nil || *got.Context != "first line\nsecond line" {
			t.Errorf("history after reading the file again: %+v", got)
		}
	})
}

func TestAmendOptionSummary(t *testing.T) {
	for _, tc := range []struct {
		id    int
		label string
		idx   int
		old   string
	}{{1, "b", 1, "Use B"}, {2, "redis", 0, "Use Redis"}} { // open, decided
		root := seedAmend(t)
		amendOK(t, root, tc.id, "", []string{"--summary", tc.label + ":Use it: with colons, trimmed "},
			func(r *model.Record) { r.Options[tc.idx].Summary = "Use it: with colons, trimmed" },
			"summary "+tc.label, model.Was{Options: []model.OptionWas{{Label: tc.label, Summary: strp(tc.old)}}})
	}
}

func TestAmendTradeoffs(t *testing.T) {
	t.Run("changed, open and decided", func(t *testing.T) {
		for _, tc := range []struct {
			id    int
			label string
			old   string
		}{{1, "a", "slower"}, {2, "redis", "one more service"}} {
			root := seedAmend(t)
			amendOK(t, root, tc.id, "", []string{"--tradeoffs", tc.label + ":it differs"},
				func(r *model.Record) { r.Options[0].Tradeoffs = "it differs" },
				"tradeoffs "+tc.label, model.Was{Options: []model.OptionWas{{Label: tc.label, Tradeoffs: strp(tc.old)}}})
		}
	})
	t.Run("cleared with an empty text", func(t *testing.T) {
		root := seedAmend(t)
		amendOK(t, root, 1, "", []string{"--tradeoffs", "a:"},
			func(r *model.Record) { r.Options[0].Tradeoffs = "" },
			"tradeoffs a", model.Was{Options: []model.OptionWas{{Label: "a", Tradeoffs: strp("slower")}}})
	})
	t.Run("set on an option that had none", func(t *testing.T) {
		root := seedAmend(t)
		amendOK(t, root, 1, "", []string{"--tradeoffs", "b:new"},
			func(r *model.Record) { r.Options[1].Tradeoffs = "new" },
			"tradeoffs b", model.Was{Options: []model.OptionWas{{Label: "b", Tradeoffs: strp("")}}})
	})
}

func TestAmendRationale(t *testing.T) {
	for _, id := range []int{2, 3} { // decided, rejected: the records that have one
		root := seedAmend(t)
		old := recordByID(t, root, id).Rationale
		amendOK(t, root, id, "", []string{"--rationale", "a clearer\nreason"},
			func(r *model.Record) { r.Rationale = "a clearer\nreason" }, "rationale", model.Was{Rationale: strp(old)})
	}
}

// One call, several fields: one history entry that keeps every old text.
func TestAmendSeveralFieldsAddsOneEntry(t *testing.T) {
	root := seedAmend(t)
	before := recordByID(t, root, 2)
	amendOK(t, root, 2, "", []string{"--title", "Cache", "--rationale", "because", "--context", "ctx",
		"--summary", "redis:Redis", "--tradeoffs", "redis:none", "--title", "Cache!"},
		func(r *model.Record) {
			r.Title, r.Rationale, r.Context = "Cache!", "because", "ctx"
			r.Options[0].Summary, r.Options[0].Tradeoffs = "Redis", "none"
		}, "title, context, summary redis, tradeoffs redis, rationale", model.Was{
			Title: strp(before.Title), Context: strp(before.Context), Rationale: strp(before.Rationale),
			Options: []model.OptionWas{{Label: "redis", Summary: strp("Use Redis"), Tradeoffs: strp("one more service")}},
		})
}

// A field given its current value is not a change: it is not in the entry.
func TestAmendLeavesAnUnchangedFieldOutOfTheEntry(t *testing.T) {
	root := seedAmend(t)
	amendOK(t, root, 1, "", []string{"--title", "Pick a store", "--summary", "a:Use A", "--context", "changed"},
		func(r *model.Record) { r.Context = "changed" }, "context", model.Was{Context: strp("first line\nsecond line")})
}

func TestAmendRefusesOtherFields(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"status", []string{"--status", "decided"}, "status"},
		{"decision", []string{"--decision", "b"}, "decision"},
		{"decided by", []string{"--decided-by", "me"}, "decided-by"},
		{"decided at", []string{"--decided-at", "2026-01-01"}, "decided-at"},
		{"by", []string{"--by", "me"}, "by"},
		{"asked by", []string{"--asked-by", "me"}, "asked-by"},
		{"id", []string{"--id", "7"}, "id"},
		{"supersedes", []string{"--supersedes", "2"}, "supersedes"},
		{"superseded by", []string{"--superseded-by", "2"}, "superseded-by"},
		{"key", []string{"--key", "k"}, "key"},
		{"tag", []string{"--tag", "t"}, "tag"},
		{"ref", []string{"--ref", "r"}, "ref"},
		{"created", []string{"--created", "2026-01-01"}, "created"},
		{"option label", []string{"--label", "a:z"}, "label"},
		{"adding an option", []string{"--option", "c:Use C"}, "option"},
		{"recommendation", []string{"--recommend", "b"}, "recommend"},
		{"a flag next to a good one", []string{"--title", "ok", "--key", "k"}, "key"},
		{"renaming an option through --summary", []string{"--summary", "c:Use C"}, `"c"`},
		{"renaming an option through --tradeoffs", []string{"--tradeoffs", "c:x"}, `"c"`},
		{"a label that differs only in case", []string{"--summary", "A:x"}, `"A"`},
	}
	root := seedAmend(t) // every case is a refusal, so the store is shared and checked after each
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, root)
			r := run(envOn(t, root, amendDate), "", append([]string{"amend", "1"}, tt.args...)...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj amend") {
				t.Errorf("stderr should name %q and print the amend usage:\n%s", tt.wantErr, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("the store changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestAmendNoChange(t *testing.T) {
	t.Run("the text is what the record has: an error, exit 1", func(t *testing.T) {
		for _, args := range [][]string{
			{"--title", "Pick a store"},
			{"--context", "first line\nsecond line"},
			{"--summary", "a: Use A "},
			{"--tradeoffs", "b:"},
			{"--title", "Pick a store", "--tradeoffs", "a:slower"},
		} {
			root := seedAmend(t)
			before := snapshot(t, root)
			r := run(envOn(t, root, amendDate), "", append([]string{"amend", "1"}, args...)...)
			if r.code != 1 {
				t.Errorf("%v: exit code = %d, want 1; stderr:\n%s", args, r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, "record 1: nothing to change: every text given is what the record already says") || r.stdout != "" {
				t.Errorf("%v: output = (%q, %q), want a nothing-to-change error", args, r.stdout, r.stderr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("%v: the store changed:\nbefore %q\nafter  %q", args, before, after)
			}
		}
	})
	t.Run("no field given at all is a usage error", func(t *testing.T) {
		root := seedAmend(t)
		before := snapshot(t, root)
		r := run(envOn(t, root, amendDate), "", "amend", "1")
		if r.code != 2 || !strings.Contains(r.stderr, "give at least one") || !strings.Contains(r.stderr, "usage: adj amend") {
			t.Errorf("result = %+v, want exit 2, a message and the usage", r)
		}
		if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
			t.Errorf("the store changed")
		}
	})
}

func TestAmendRationaleOnOpen(t *testing.T) {
	for _, args := range [][]string{
		{"--rationale", "why"},
		{"--title", "Another title", "--rationale", "why"}, // the real change beside it is not written either
	} {
		root := seedAmend(t)
		before := snapshot(t, root)
		r := run(envOn(t, root, amendDate), "", append([]string{"amend", "1"}, args...)...)
		if r.code != 1 {
			t.Errorf("%v: exit code = %d, want 1; stderr:\n%s", args, r.code, r.stderr)
		}
		if !strings.Contains(r.stderr, "record 1 is open") || !strings.Contains(r.stderr, "rationale") || r.stdout != "" {
			t.Errorf("%v: output = (%q, %q), want an error that says the record is open and has no rationale", args, r.stdout, r.stderr)
		}
		if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
			t.Errorf("%v: the store changed", args)
		}
	}
}

func TestAmendBlankAndBadValues(t *testing.T) {
	tests := []struct {
		name    string
		stdin   string
		args    []string
		wantErr string
	}{
		{"blank title", "", []string{"--title", "  "}, "--title must not be blank"},
		{"empty title", "", []string{"--title", ""}, "--title must not be blank"},
		{"multi-line title", "", []string{"--title", "a\nb"}, "--title must be one line"},
		{"blank context", "", []string{"--context", " \n "}, "--context must not be blank"},
		{"empty context", "", []string{"--context", ""}, "--context must not be blank"},
		{"blank context from stdin", "  \n\n", []string{"--stdin"}, "context must not be blank"},
		{"empty stdin", "", []string{"--stdin"}, "context must not be blank"},
		{"context and stdin", "x", []string{"--context", "x", "--stdin"}, "exclusive"},
		{"blank rationale", "", []string{"--rationale", "\t"}, "--rationale must not be blank"},
		{"summary without a colon", "", []string{"--summary", "a"}, "LABEL:TEXT"},
		{"summary with an empty label", "", []string{"--summary", ":x"}, "label is empty"},
		{"summary twice for one option", "", []string{"--summary", "redis:x", "--summary", "redis:y"}, `"redis"`},
		{"tradeoffs without a colon", "", []string{"--tradeoffs", "a"}, "LABEL:TEXT"},
		{"tradeoffs twice for one option", "", []string{"--tradeoffs", "redis:x", "--tradeoffs", "redis:y"}, `"redis"`},
		{"unknown flag", "", []string{"--nope", "x"}, "nope"},
		{"flag without a value", "", []string{"--title"}, "title"},
	}
	root := seedAmend(t) // every case is a refusal, so the store is shared and checked after each
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, root)
			r := run(envOn(t, root, amendDate), tt.stdin, append([]string{"amend", "2"}, tt.args...)...)
			if r.code != 2 {
				t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj amend") {
				t.Errorf("stderr should contain %q and the usage:\n%s", tt.wantErr, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("the store changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestAmendBadArguments(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		code    int
		wantErr string
	}{
		{"no ID", []string{"amend", "--title", "x"}, 2, "ID is required"},
		{"ID is not a number", []string{"amend", "x", "--title", "y"}, 2, "ID must be a positive integer"},
		{"ID is zero", []string{"amend", "0", "--title", "y"}, 2, "ID must be a positive integer"},
		{"extra argument", []string{"amend", "1", "2", "--title", "y"}, 2, "want one ID"},
		{"unknown record", []string{"amend", "99", "--title", "y"}, 1, "record not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := seedAmend(t)
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != tt.code || !strings.Contains(r.stderr, tt.wantErr) || r.stdout != "" {
				t.Errorf("result = %+v, want exit %d with %q", r, tt.code, tt.wantErr)
			}
			if tt.code == 2 && !strings.Contains(r.stderr, "usage: adj amend") {
				t.Errorf("a usage error should print the usage:\n%s", r.stderr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("the store changed")
			}
		})
	}
	t.Run("the standard input is not read on bad arguments or before init", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := RunWith(testEnv(seedAmend(t)), []string{"amend", "1", "--stdin", "--title", " "}, failReader{t}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("exit = %d, want 2 without reading stdin", code)
		}
		code = RunWith(testEnv(t.TempDir()), []string{"amend", "1", "--stdin"}, failReader{t}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "adj init") {
			t.Errorf("exit = %d, stderr = %q; want 1 and a hint to run adj init, without reading stdin", code, stderr.String())
		}
	})
	t.Run("before init", func(t *testing.T) {
		r := run(testEnv(t.TempDir()), "", "amend", "1", "--title", "x")
		if r.code != 1 || !strings.Contains(r.stderr, "adj init") {
			t.Errorf("result = %+v, want exit 1 and a hint to run adj init", r)
		}
	})
	t.Run("--root before the command and after it", func(t *testing.T) {
		root := seedAmend(t)
		if r := run(testEnv(t.TempDir()), "", "--root", root, "amend", "3", "--title", "Renamed"); r.code != 0 {
			t.Errorf("leading --root: %+v", r)
		}
		if r := run(testEnv(t.TempDir()), "", "amend", "3", "--title", "Renamed again", "--root", root); r.code != 0 {
			t.Errorf("trailing --root: %+v", r)
		}
		if got := recordByID(t, root, 3).Title; got != "Renamed again" {
			t.Errorf("title = %q", got)
		}
	})
	t.Run("help", func(t *testing.T) {
		r := run(testEnv(t.TempDir()), "", "amend", "--help")
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj amend") || r.stderr != "" {
			t.Errorf("amend --help = %+v", r)
		}
	})
}

func TestUsageListsAmend(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(r.stdout, "adj amend") {
		t.Errorf("usage does not mention adj amend:\n%s", r.stdout)
	}
}

// A record file with a duplicate ID behaves as it does for the other commands
// that go through store.Get and store.Update: the amend fails, names the
// problem and changes no file.
func TestAmendOnADuplicateID(t *testing.T) {
	root := seedAmend(t)
	dir := filepath.Join(root, "decisions")
	data, err := os.ReadFile(filepath.Join(dir, "0001-pick-a-store.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0001-pick-a-store-copy.md"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	r := run(envOn(t, root, amendDate), "", "amend", "1", "--title", "New title")
	d := run(envOn(t, root, amendDate), "", "decide", "1", "a", "--why", "x")
	if r.code != 1 || !strings.Contains(r.stderr, "more than one record has this id") {
		t.Errorf("amend = %+v, want exit 1 and the duplicate-ID error", r)
	}
	if d.code != r.code || strings.Replace(d.stderr, "decide", "amend", 1) != r.stderr {
		t.Errorf("amend fails differently from decide:\namend  %+v\ndecide %+v", r, d)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("the store changed")
	}
}

// adj show prints the old text under the amend entry, multi-line text indented.
func TestAmendShowDisplaysTheOldText(t *testing.T) {
	root := seedAmend(t)
	if r := run(envOn(t, root, amendDate), "", "amend", "1", "--title", "Pick a datastore",
		"--context", "short", "--tradeoffs", "a:", "--summary", "b:Use B!"); r.code != 0 {
		t.Fatalf("amend: %+v", r)
	}
	r := run(testEnv(root), "", "show", "1")
	if r.code != 0 {
		t.Fatalf("show: %+v", r)
	}
	want := `History:
  2026-03-04 ask
  2026-10-17 amend: title, context, tradeoffs a, summary b
    was title: Pick a store
    was context:
      first line
      second line
    was tradeoffs a: slower
    was summary b: Use B
`
	if !strings.Contains(r.stdout, want) {
		t.Errorf("show does not print the old text as expected\n got:\n%s\nwant it to contain:\n%s", r.stdout, want)
	}
}

// The new history entry is in the JSON of show, with the documented keys.
func TestAmendHistoryEntryIsDocumentedJSON(t *testing.T) {
	root := seedAmend(t)
	if r := run(envOn(t, root, amendDate), "", "amend", "2", "--title", "T2", "--context", "c2", "--rationale", "r2",
		"--summary", "redis:s2", "--tradeoffs", "redis:t2"); r.code != 0 {
		t.Fatalf("amend: %+v", r)
	}
	r := run(testEnv(root), "", "show", "2", "--json")
	hist, ok := decodeObject(t, r.stdout)["history"].([]any)
	if !ok || len(hist) != 3 {
		t.Fatalf("history = %v, want ask, decide and amend", hist)
	}
	entry := hist[2].(map[string]any)
	tag := func(v any) []string {
		var out []string
		typ := reflect.TypeOf(v)
		for i := 0; i < typ.NumField(); i++ {
			name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
			if name == "" { // unexported: the unknown keys of a history entry (S44)
				continue
			}
			out = append(out, name)
		}
		sort.Strings(out)
		return out
	}
	if got, want := keysOf(entry), []string{"date", "event", "note", "was"}; !reflect.DeepEqual(got, want) {
		t.Errorf("amend entry keys = %v, want %v", got, want)
	}
	was := entry["was"].(map[string]any)
	if got, want := keysOf(was), []string{"context", "options", "rationale", "title"}; !reflect.DeepEqual(got, want) {
		t.Errorf("was keys = %v, want %v", got, want)
	}
	opt := was["options"].([]any)[0].(map[string]any)
	if got, want := keysOf(opt), []string{"label", "summary", "tradeoffs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("was option keys = %v, want %v", got, want)
	}

	// docs/json.md documents exactly the keys the model has.
	doc := docsText(t)
	if got, want := docsTableKeys(t, "## Keys of a history entry"), tag(model.HistoryEntry{}); !reflect.DeepEqual(got, want) {
		t.Errorf("history entry key table = %v, want the model's keys %v", got, want)
	}
	section, err := sectionUnder(doc, "## Keys of a history entry")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range append(tag(model.Was{}), tag(model.OptionWas{})...) {
		if !strings.Contains(section, "`"+k+"`") {
			t.Errorf("the history entry section does not name the old-text key %q", k)
		}
	}
	// The documented example is a valid amend entry that the model decodes
	// without an unknown key.
	dec := json.NewDecoder(strings.NewReader(docsJSONExample(t, "## Keys of a history entry")))
	dec.DisallowUnknownFields()
	var example model.HistoryEntry
	if err := dec.Decode(&example); err != nil || example.Event != model.EventAmend || example.Was == nil ||
		*example.Was.Title != "Pick a store" || *example.Was.Options[0].Tradeoffs != "slower" {
		t.Errorf("the history entry example does not decode as the amend entry it describes: %+v, %v", example, err)
	}
	for _, w := range []string{"`amend`", "`was`"} {
		if !strings.Contains(doc, w) {
			t.Errorf("docs/json.md does not mention %s", w)
		}
	}
}

// adj lint accepts a store with amended records: nothing is reported.
func TestAmendedStoreLintsClean(t *testing.T) {
	root := seedAmend(t)
	for _, args := range [][]string{
		{"amend", "1", "--title", "T2", "--tradeoffs", "a:"},
		{"amend", "2", "--rationale", "r2", "--context", "multi\nline"},
	} {
		if r := run(envOn(t, root, amendDate), "", args...); r.code != 0 {
			t.Fatalf("%v: %+v", args, r)
		}
	}
	r := run(testEnv(root), "", "lint")
	// record 2 was decided with decided_by "owner": no warning either
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("lint = %+v, want nothing", r)
	}
}
