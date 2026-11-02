// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The records of a store that a newer version wrote (S46): 1 is decided, with
// unknown top-level keys and an amend entry whose was has unknown keys, also
// inside an option; 2 is open with one unknown key; 3 is a plain decided
// record.
const (
	futureDecided = `---
{
  "id": 1,
  "title": "Cache layer",
  "status": "decided",
  "created": "2026-10-01",
  "decided_at": "2026-10-02",
  "decided_by": "owner",
  "options": [
    {
      "label": "redis",
      "summary": "New summary"
    }
  ],
  "decision": "redis",
  "rationale": "fast",
  "history": [
    {
      "date": "2026-10-01",
      "event": "ask"
    },
    {
      "date": "2026-10-03",
      "event": "amend",
      "note": "summary redis",
      "was": {
        "options": [
          {
            "label": "redis",
            "summary": "Old summary",
            "url": "https://redis.io"
          }
        ],
        "tags": [
          "x"
        ]
      }
    }
  ],
  "alpha": true,
  "zeta": {
    "k": [
      1
    ]
  }
}
---
The body.
`
	futureOpen = `---
{
  "id": 2,
  "title": "Queue",
  "status": "open",
  "created": "2026-10-01",
  "context": "Which queue?",
  "options": [
    {
      "label": "a",
      "summary": "A"
    }
  ],
  "colour": "red"
}
---
`
	plainDecided = `---
{"id":3,"title":"Replacement","status":"decided","created":"2026-10-01","decided_at":"2026-10-02","decided_by":"owner","decision":"x","rationale":"y","history":[{"date":"2026-10-01","event":"ask"}]}
---
`
)

// The unknown top-level members of the records, as the files above write them
// (known keys first, then these, sorted), and the unknown keys inside a was.
const (
	openTail    = "  ],\n  \"colour\": \"red\"\n}\n---\n"
	decidedTail = "  \"alpha\": true,\n  \"zeta\": {\n    \"k\": [\n      1\n    ]\n  }\n}\n---\nThe body.\n"
	wasUnknown  = "          {\n            \"label\": \"redis\",\n            \"summary\": \"Old summary\",\n            \"url\": \"https://redis.io\"\n          }\n        ],\n        \"tags\": [\n          \"x\"\n        ]\n"
)

func futureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), []byte(`{"store": "decisions"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{
		"0001-cache-layer.md": futureDecided, "0002-queue.md": futureOpen, "0003-replacement.md": plainDecided,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// show --json, list --json, inbox --json and check --json carry the unknown
// keys beside the known ones; the was of an amend entry keeps its own too.
func TestJSONOutputIncludesUnknownRecordAndWasKeys(t *testing.T) {
	env := testEnv(futureRoot(t))

	r := run(env, "", "show", "1", "--json")
	if r.code != 0 {
		t.Fatalf("show: exit %d, stderr:\n%s", r.code, r.stderr)
	}
	obj := decodeObject(t, r.stdout)
	if obj["alpha"] != true || !reflect.DeepEqual(obj["zeta"], map[string]any{"k": []any{1.0}}) {
		t.Errorf("show --json lacks the unknown keys alpha and zeta:\n%s", r.stdout)
	}
	if obj["id"] != 1.0 || obj["body"] != "The body.\n" {
		t.Errorf("show --json = %v, want the known keys and the body", obj)
	}
	was := obj["history"].([]any)[1].(map[string]any)["was"].(map[string]any)
	if !reflect.DeepEqual(was["tags"], []any{"x"}) {
		t.Errorf("was = %v, want the unknown key tags", was)
	}
	if opt := was["options"].([]any)[0].(map[string]any); opt["url"] != "https://redis.io" {
		t.Errorf("was option = %v, want the unknown key url", opt)
	}

	for _, args := range [][]string{{"list", "--json"}, {"inbox", "--json"}, {"check", "queue", "--json"}} {
		r := run(env, "", args...)
		if r.code != 0 {
			t.Fatalf("%v: exit %d, stderr:\n%s", args, r.code, r.stderr)
		}
		if !strings.Contains(r.stdout, `"colour": "red"`) {
			t.Errorf("%v does not carry the unknown key colour:\n%s", args, r.stdout)
		}
	}
	if r := run(env, "", "list", "--json"); !strings.Contains(r.stdout, `"alpha": true`) {
		t.Errorf("list --json lacks alpha:\n%s", r.stdout)
	}
}

// adj show text and adj context print the known fields only.
func TestTextAndContextLeaveOutUnknownKeys(t *testing.T) {
	env := testEnv(futureRoot(t))
	for _, args := range [][]string{
		{"show", "1"}, {"show", "2"}, {"list"}, {"inbox"}, {"check", "cache"},
		{"context"}, {"context", "--format", "json"},
	} {
		r := run(env, "", args...)
		if r.code != 0 {
			t.Fatalf("%v: exit %d, stderr:\n%s", args, r.code, r.stderr)
		}
		for _, leak := range []string{"alpha", "zeta", "colour", "https://redis.io", "tags"} {
			if strings.Contains(r.stdout, leak) {
				t.Errorf("%v prints %q:\n%s", args, leak, r.stdout)
			}
		}
	}
	// The amend entry is shown from its known fields.
	r := run(env, "", "show", "1")
	if !strings.Contains(r.stdout, "was summary redis: Old summary") {
		t.Errorf("show 1 lacks the old summary:\n%s", r.stdout)
	}
}

// Every command that rewrites a record writes the unknown parts back as they
// were: the file after the command still holds each unknown member and the
// unknown keys of every was.
func TestRewritingCommandsKeepUnknownKeys(t *testing.T) {
	tests := []struct {
		name string
		file string
		args []string
		keep []string
	}{
		{"decide", "0002-queue.md", []string{"decide", "2", "a", "--why", "because", "--by", "owner"}, []string{openTail}},
		{"reject", "0002-queue.md", []string{"reject", "2", "--why", "no", "--by", "owner"}, []string{openTail}},
		{"withdraw", "0002-queue.md", []string{"withdraw", "2", "--why", "no"}, []string{openTail}},
		{"amend an open record", "0002-queue.md", []string{"amend", "2", "--title", "Queue, corrected"}, []string{openTail}},
		{"amend a decided record", "0001-cache-layer.md", []string{"amend", "1", "--rationale", "faster"}, []string{
			decidedTail, wasUnknown}},
		{"supersede, the old record", "0001-cache-layer.md", []string{"supersede", "1", "--by", "3"}, []string{
			decidedTail, wasUnknown}},
		{"narrow, the old record", "0001-cache-layer.md", []string{"narrow", "1", "--by", "3", "--scope", "s"}, []string{
			decidedTail, wasUnknown}},
		{"narrow, the new record", "0001-cache-layer.md", []string{"narrow", "3", "--by", "1", "--scope", "s"}, []string{
			decidedTail, wasUnknown}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := futureRoot(t)
			path := filepath.Join(root, "decisions", tt.file)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if r := run(testEnv(root), "", tt.args...); r.code != 0 {
				t.Fatalf("%v: exit %d, stderr:\n%s", tt.args, r.code, r.stderr)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) == string(before) {
				t.Fatal("the command changed nothing")
			}
			for _, want := range tt.keep {
				if !strings.Contains(string(after), want) {
					t.Errorf("file lost %s:\n%s", want, after)
				}
			}
		})
	}
}

// Lint warns about the unknown keys and exits 0; the JSON report lists the
// warnings (S46).
func TestLintWarnsAboutUnknownRecordAndWasKeys(t *testing.T) {
	root := futureRoot(t)
	r := run(testEnv(root), "", "lint")
	if r.code != 0 {
		t.Errorf("exit code = %d, want 0 (warnings only); stderr:\n%s", r.code, r.stderr)
	}
	want := `decisions/0001-cache-layer.md: record 1: warning: unknown-record-key: unknown keys "alpha", "zeta"
decisions/0001-cache-layer.md: record 1: warning: unknown-was-key: history entry 1: unknown was keys "options.redis.url", "tags"
decisions/0002-queue.md: record 2: warning: unknown-record-key: unknown key "colour"
0 errors, 3 warnings
`
	if r.stdout != want {
		t.Errorf("stdout =\n%s\nwant\n%s", r.stdout, want)
	}

	j := run(testEnv(root), "", "lint", "--json")
	if j.code != 0 {
		t.Errorf("--json exit code = %d, want 0; stderr:\n%s", j.code, j.stderr)
	}
	top := decodeObject(t, j.stdout)
	if top["errors"] != 0.0 || top["warnings"] != 3.0 {
		t.Errorf("errors, warnings = %v, %v; want 0, 3", top["errors"], top["warnings"])
	}
	var checks []string
	for _, f := range recordObjects(t, j.stdout, "findings") {
		if f["severity"] != "warning" {
			t.Errorf("finding %v has severity %v, want warning", f["check"], f["severity"])
		}
		checks = append(checks, f["check"].(string))
	}
	if want := []string{"unknown-record-key", "unknown-was-key", "unknown-record-key"}; !reflect.DeepEqual(checks, want) {
		t.Errorf("checks = %v, want %v", checks, want)
	}
}

// A frontmatter key named body is never printed by the read commands: body in
// their JSON is the text after the frontmatter (C235).
func TestJSONLeavesOutAFrontmatterBodyKey(t *testing.T) {
	root := futureRoot(t)
	path := filepath.Join(root, "decisions", "0002-queue.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), `"colour": "red"`, `"body": "fake body", "colour": "red"`, 1)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	env := testEnv(root)
	for _, args := range [][]string{{"show", "2", "--json"}, {"list", "--json"}, {"inbox", "--json"}, {"check", "queue", "--json"}} {
		r := run(env, "", args...)
		if r.code != 0 {
			t.Fatalf("%v: exit %d, stderr:\n%s", args, r.code, r.stderr)
		}
		if strings.Contains(r.stdout, "fake body") {
			t.Errorf("%v prints the frontmatter body key:\n%s", args, r.stdout)
		}
		if !strings.Contains(r.stdout, `"colour": "red"`) {
			t.Errorf("%v lost the other unknown key:\n%s", args, r.stdout)
		}
	}
	// A rewrite keeps the key in the file.
	if r := run(env, "", "amend", "2", "--title", "Queue, corrected"); r.code != 0 {
		t.Fatalf("amend: exit %d, stderr:\n%s", r.code, r.stderr)
	}
	if after, _ := os.ReadFile(path); !strings.Contains(string(after), `"body": "fake body"`) {
		t.Errorf("file lost the body key:\n%s", after)
	}
}
