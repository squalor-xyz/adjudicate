// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// unknownRoot is a temp copy of the lint fixture "unknown-history": record 1
// is open with an unknown event, record 2 is decided with an unknown key on its
// decide entry, record 3 is open with both (S44).
func unknownRoot(t *testing.T) string {
	t.Helper()
	return lintRoot(t, "unknown-history")
}

// Lint warns about an unknown event and an unknown history key, and warnings
// alone exit 0 (S44, C140).
func TestLintWarnsAboutUnknownHistoryEventsAndKeys(t *testing.T) {
	r := run(testEnv(unknownRoot(t)), "", "lint")
	if r.code != 0 {
		t.Errorf("exit code = %d, want 0 (warnings only); stderr:\n%s", r.code, r.stderr)
	}
	want := `decisions/0001-future-event.md: record 1: warning: unknown-event: history entry 1: unknown event "future-thing"
decisions/0002-extra-key.md: record 2: warning: unknown-history-key: history entry 1: unknown key "by"
decisions/0003-both.md: record 3: warning: unknown-event: history entry 1: unknown event "future-thing"
decisions/0003-both.md: record 3: warning: unknown-event: history entry 2: unknown event "other-thing"
decisions/0003-both.md: record 3: warning: unknown-history-key: history entry 1: unknown keys "alpha", "zeta"
0 errors, 5 warnings
`
	if r.stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, want)
	}
}

func TestLintJSONListsUnknownHistoryFindingsAsWarnings(t *testing.T) {
	r := run(testEnv(unknownRoot(t)), "", "lint", "--json")
	if r.code != 0 {
		t.Errorf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	top := decodeObject(t, r.stdout)
	if top["errors"] != 0.0 || top["warnings"] != 5.0 {
		t.Errorf("errors, warnings = %v, %v; want 0, 5", top["errors"], top["warnings"])
	}
	var checks []string
	for _, f := range recordObjects(t, r.stdout, "findings") {
		if f["severity"] != "warning" {
			t.Errorf("finding %v has severity %v, want warning", f["check"], f["severity"])
		}
		checks = append(checks, f["check"].(string))
	}
	want := []string{"unknown-event", "unknown-history-key", "unknown-event", "unknown-event", "unknown-history-key"}
	if !reflect.DeepEqual(checks, want) {
		t.Errorf("checks = %v, want %v", checks, want)
	}
}

// The readers that List used to refuse accept the store.
func TestReadCommandsAcceptUnknownHistoryEventsAndKeys(t *testing.T) {
	env := testEnv(unknownRoot(t))
	r := run(env, "", "list", "--json")
	if r.code != 0 {
		t.Fatalf("list: exit %d, stderr %q; want 0", r.code, r.stderr)
	}
	if got := len(recordObjects(t, r.stdout, "records")); got != 3 {
		t.Errorf("list lists %d records, want 3", got)
	}
	for _, args := range [][]string{{"inbox"}, {"check", "extra"}, {"context"}} {
		if r := run(env, "", args...); r.code != 0 {
			t.Errorf("%v: exit %d, stderr %q; want 0", args, r.code, r.stderr)
		}
	}
}

// show --json keeps the unknown keys of an entry (they are part of the
// record); show text prints the stored event name and no unknown key.
func TestShowKeepsUnknownHistoryKeysInJSONButNotInText(t *testing.T) {
	env := testEnv(unknownRoot(t))
	r := run(env, "", "show", "3", "--json")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	history, ok := decodeObject(t, r.stdout)["history"].([]any)
	if !ok || len(history) != 3 {
		t.Fatalf("history = %v, want three entries:\n%s", history, r.stdout)
	}
	entry := history[1].(map[string]any)
	if got, want := keysOf(entry), []string{"alpha", "date", "event", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("entry keys = %v, want %v", got, want)
	}
	if entry["zeta"] != 1.0 || entry["event"] != "future-thing" {
		t.Errorf("entry = %v, want zeta 1 and event future-thing", entry)
	}

	text := run(env, "", "show", "3")
	if text.code != 0 || !strings.Contains(text.stdout, "2026-10-02 future-thing\n") {
		t.Errorf("show 3: exit %d, stdout:\n%s\nwant the stored event name in the history", text.code, text.stdout)
	}
	if strings.Contains(text.stdout, "alpha") || strings.Contains(text.stdout, "zeta") {
		t.Errorf("show 3 prints an unknown key:\n%s", text.stdout)
	}
}

// Deciding a record whose history holds an unknown event and key keeps both
// as they were, and adds the new entry (S44).
func TestDecideKeepsAnUnknownHistoryEntry(t *testing.T) {
	root := unknownRoot(t)
	path := filepath.Join(root, "decisions", "0003-both.md")
	if r := run(testEnv(root), "", "decide", "3", "--text", "go", "--why", "because", "--by", "owner"); r.code != 0 {
		t.Fatalf("decide: exit %d, stderr:\n%s", r.code, r.stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"event": "future-thing"`, `"zeta": 1`, `"alpha": {`, `"event": "other-thing"`, `"event": "decide"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("file lost %s:\n%s", want, data)
		}
	}
	show := run(testEnv(root), "", "show", "3", "--json")
	history := decodeObject(t, show.stdout)["history"].([]any)
	if len(history) != 4 || history[3].(map[string]any)["event"] != "decide" {
		t.Errorf("history = %v, want the three old entries and a decide entry", history)
	}
}
