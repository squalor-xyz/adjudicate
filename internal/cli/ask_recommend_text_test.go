// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// recommendationOf decodes the "recommendation" object of the one record in a
// --json output: the object itself (show), or the first record of an array.
func recommendationOf(t *testing.T, stdout string, arrays ...string) map[string]string {
	t.Helper()
	recs := recordObjects(t, stdout, arrays...)
	if len(recs) != 1 {
		t.Fatalf("want one record object, got %d in:\n%s", len(recs), stdout)
	}
	raw, ok := recs[0]["recommendation"]
	if !ok {
		t.Fatalf("record has no recommendation in:\n%s", stdout)
	}
	data, _ := json.Marshal(raw)
	var out map[string]string
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("recommendation %s is not an object of strings: %v", data, err)
	}
	return out
}

// --recommend-text stores a recommendation that is not an option label
// (C167): the text and the reasoning, no label, with or without options.
func TestAskRecommendText(t *testing.T) {
	const text = "Log to stderr as JSON lines"
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"no options", []string{"ask", "Where do logs go", "--recommend-text", text, "--reasoning", "agents parse it"}},
		{"options, none recommended", []string{"ask", "Where do logs go", "--option", "file:A file", "--option", "stderr:Stderr",
			"--recommend-text", text, "--reasoning", "agents parse it"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			if r := run(testEnv(root), "", tt.args...); r.code != 0 || r.stdout != "1\n" || r.stderr != "" {
				t.Fatalf("ask: exit %d, stdout %q, stderr %q; want exit 0 and the ID", r.code, r.stdout, r.stderr)
			}
			recs := records(t, root)
			if len(recs) != 1 {
				t.Fatalf("records = %+v, want one", recs)
			}
			want := &model.Recommendation{Text: text, Reasoning: "agents parse it"}
			if !reflect.DeepEqual(recs[0].Recommendation, want) {
				t.Errorf("recommendation = %+v, want %+v (text, no label)", recs[0].Recommendation, want)
			}

			// show, text and JSON.
			r := run(testEnv(root), "", "show", "1")
			if want := "Recommendation: " + text + "\nReasoning: agents parse it\n"; !strings.Contains(r.stdout, want) {
				t.Errorf("show 1 =\n%s\nwant it to contain\n%s", r.stdout, want)
			}
			wantJSON := map[string]string{"text": text, "reasoning": "agents parse it"}
			r = run(testEnv(root), "", "show", "1", "--json")
			if got := recommendationOf(t, r.stdout); !reflect.DeepEqual(got, wantJSON) {
				t.Errorf("show 1 --json recommendation = %v, want %v", got, wantJSON)
			}

			// inbox, text and JSON.
			r = run(testEnv(root), "", "inbox")
			if want := "  Recommendation: " + text + "\n  Reasoning: agents parse it\n"; !strings.HasSuffix(r.stdout, want) {
				t.Errorf("inbox =\n%s\nwant it to end with\n%s", r.stdout, want)
			}
			r = run(testEnv(root), "", "inbox", "--json")
			if got := recommendationOf(t, r.stdout, "records"); !reflect.DeepEqual(got, wantJSON) {
				t.Errorf("inbox --json recommendation = %v, want %v", got, wantJSON)
			}

			// check lists an open record in the same record shape.
			r = run(testEnv(root), "", "check", "--json", "logs")
			if got := recommendationOf(t, r.stdout, "open"); !reflect.DeepEqual(got, wantJSON) {
				t.Errorf("check --json recommendation = %v, want %v", got, wantJSON)
			}
		})
	}
}

// The text is kept as typed, like --reasoning and decide --text; only a blank
// one is refused.
func TestAskRecommendTextKeepsTheTextAsTyped(t *testing.T) {
	root := initRoot(t)
	r := run(testEnv(root), "", "ask", "T", "--recommend-text", "  line one\nline two  ")
	if r.code != 0 {
		t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
	}
	if rec := records(t, root)[0].Recommendation; rec == nil || rec.Text != "  line one\nline two  " || rec.Label != "" || rec.Reasoning != "" {
		t.Errorf("recommendation = %+v, want the text as typed and nothing else", rec)
	}
}

func TestAskRecommendExclusive(t *testing.T) {
	root := initRoot(t)
	before := snapshot(t, root)
	r := run(testEnv(root), "", "ask", "T", "--option", "a:A", "--recommend", "a", "--recommend-text", "or something else")
	if r.code != 2 {
		t.Errorf("exit code = %d, want 2; stderr:\n%s", r.code, r.stderr)
	}
	if !strings.Contains(r.stderr, "--recommend and --recommend-text are exclusive") || !strings.Contains(r.stderr, "usage: adj ask") {
		t.Errorf("stderr does not say the flags are exclusive, with the usage:\n%s", r.stderr)
	}
	if r.stdout != "" {
		t.Errorf("stdout = %q, want empty", r.stdout)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
	}
}

func TestAskRecommendTextBadInputIsAUsageError(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"blank text", []string{"T", "--recommend-text", "  "}, "--recommend-text must not be blank"},
		{"explicit empty text", []string{"T", "--recommend-text", ""}, "--recommend-text must not be blank"},
		{"exclusive even when --recommend is empty", []string{"T", "--recommend", "", "--recommend-text", "x"}, "exclusive"},
		{"exclusive even when --recommend is blank", []string{"T", "--recommend", " ", "--recommend-text", "x"}, "exclusive"},
		{"reasoning with neither", []string{"T", "--reasoning", "because"}, "--recommend-text"},
		{"recommend is still an option label", []string{"T", "--option", "a:A", "--recommend", "free text"}, `"free text"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			before := snapshot(t, root)
			r := run(testEnv(root), "", append([]string{"ask"}, tt.args...)...)
			if r.code != 2 || !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("exit code = %d, stderr:\n%s\nwant exit 2 and %q", r.code, r.stderr, tt.wantErr)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

// decide treats a record with a free-text recommendation like any other open
// record: --text and an option label both work, and the recommendation stays.
func TestDecideTextAfterRecommendText(t *testing.T) {
	root := initRoot(t)
	if r := run(testEnv(root), "", "ask", "Pick a store", "--option", "a:Use A",
		"--recommend-text", "Use neither", "--reasoning", "both are slow"); r.code != 0 {
		t.Fatalf("ask: exit %d: %s", r.code, r.stderr)
	}
	before := records(t, root)[0]
	r := run(testEnv(root), "", "decide", "1", "--text", "Use neither", "--why", "agreed")
	if r.code != 0 || r.stdout != "1\n" || r.stderr != "" {
		t.Fatalf("decide: exit %d, stdout %q, stderr %q; want exit 0 and the ID", r.code, r.stdout, r.stderr)
	}
	want := before
	want.Status = model.StatusDecided
	want.Decision = "Use neither"
	want.Rationale = "agreed"
	want.DecidedAt = "2026-03-04"
	want.History = append(slices.Clip(before.History), model.HistoryEntry{Date: "2026-03-04", Event: model.EventDecide})
	if got := records(t, root); len(got) != 1 || !reflect.DeepEqual(got[0], want) {
		t.Errorf("records = %+v\nwant one %+v", got, want)
	}
	if got := records(t, root)[0].Recommendation; got == nil || got.Text != "Use neither" {
		t.Errorf("recommendation after decide = %+v, want it kept", got)
	}
}

// --ref (S27) and --recommend-text (S28) work together: the record keeps the
// refs in order and the text recommendation, and inbox shows both.
func TestAskRefWithRecommendText(t *testing.T) {
	root := initRoot(t)
	r := run(testEnv(root), "", "ask", "Where do logs go", "--ref", "S27", "--ref", "C167",
		"--recommend-text", "Log to stderr", "--reasoning", "agents parse it")
	if r.code != 0 || r.stdout != "1\n" || r.stderr != "" {
		t.Fatalf("ask: exit %d, stdout %q, stderr %q; want exit 0 and the ID", r.code, r.stdout, r.stderr)
	}
	rec := records(t, root)[0]
	if want := []string{"S27", "C167"}; !reflect.DeepEqual(rec.Refs, want) {
		t.Errorf("refs = %q, want %q", rec.Refs, want)
	}
	if want := (&model.Recommendation{Text: "Log to stderr", Reasoning: "agents parse it"}); !reflect.DeepEqual(rec.Recommendation, want) {
		t.Errorf("recommendation = %+v, want %+v", rec.Recommendation, want)
	}
	inbox := run(testEnv(root), "", "inbox")
	if want := "  Refs: S27, C167\n"; !strings.Contains(inbox.stdout, want) || !strings.Contains(inbox.stdout, "  Recommendation: Log to stderr\n") {
		t.Errorf("inbox =\n%s\nwant the refs and the text recommendation", inbox.stdout)
	}
}
