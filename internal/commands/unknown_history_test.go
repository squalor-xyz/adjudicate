// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/store"
)

// unknownHistoryStore returns a store with open record 1 whose history holds
// an event this version does not know and a decide-like entry with a key it
// does not know (S44), and the path of its file.
func unknownHistoryStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "0001-logging.md")
	text := `---
{"id":1,"title":"Logging","status":"open","created":"2026-10-01","history":[{"date":"2026-10-01","event":"ask"},{"date":"2026-10-02","event":"future-thing","note":"x","zeta":[1,2]},{"date":"2026-10-03","event":"review","by":"someone"}]}
---
`
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return st, path
}

func TestChangesKeepAnUnknownHistoryEntryAndItsKeys(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		change func(*store.Store) error
		event  string // the new entry's event
	}{
		{"decide", func(st *store.Store) error {
			_, err := Decide(st, 1, DecideInput{Text: "files", HasText: true, Why: "simple", DecidedBy: "owner"}, now)
			return err
		}, `"event": "decide"`},
		{"amend", func(st *store.Store) error {
			_, err := Amend(st, 1, AmendInput{Title: "Logging, corrected", HasTitle: true}, now)
			return err
		}, `"event": "amend"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, path := unknownHistoryStore(t)
			if err := tt.change(st); err != nil {
				t.Fatalf("%s = %v, want nil", tt.name, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			for _, want := range []string{
				`"event": "future-thing"`, `"note": "x"`, `"zeta": [`, `"event": "review"`, `"by": "someone"`, tt.event,
			} {
				if !strings.Contains(text, want) {
					t.Errorf("file lost %s:\n%s", want, text)
				}
			}
			rec, err := st.Get(1)
			if err != nil {
				t.Fatal(err)
			}
			if len(rec.History) != 4 {
				t.Errorf("history has %d entries, want the three old ones and one new", len(rec.History))
			}
		})
	}
}
