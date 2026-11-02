// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// withUnknown returns r as read back from a file that also holds the given
// members (JSON, no braces) as top-level keys this version does not know (S46).
func withUnknown(t *testing.T, r model.Record, members string) model.Record {
	t.Helper()
	data, err := model.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "{\n", "{\n  "+members+",\n", 1)
	got, err := model.Parse("x.md", []byte(text))
	if err != nil {
		t.Fatalf("Parse(%s) = %v", text, err)
	}
	return got
}

// IsStaleCopy compares the unknown top-level keys too (S46, C32): they are part
// of "the same record", and nothing in this version can have changed them since
// the crash. Without it a keeper with other unknown keys would look like a
// stale copy, and a stale copy whose keys were lost would be moved again.
func TestIsStaleCopyComparesUnknownTopLevelKeys(t *testing.T) {
	stale, moved := File{ID: 3, Name: "0003-foo.md"}, File{ID: 5, Name: "0005-foo.md"}
	decided := func(r *model.Record) { // what adj decide does after the crash
		r.Status, r.Decision, r.Rationale, r.DecidedAt, r.DecidedBy = model.StatusDecided, "a", "because", "2026-11-03", "owner"
		r.History = append(r.History, model.HistoryEntry{Date: "2026-11-03", Event: model.EventDecide})
	}
	for _, edited := range []bool{false, true} {
		name := "no edit since"
		if edited {
			name = "edited since"
		}
		t.Run(name, func(t *testing.T) {
			staleRec := withUnknown(t, askedRecord(3), `"x": 1, "y": {"z": [true]}`)
			movedWith := func(members string) model.Record {
				r := movedBy(t, askedRecord(3), 5)
				if edited {
					decided(&r)
				}
				if members == "" {
					return r
				}
				return withUnknown(t, r, members)
			}
			tests := []struct {
				name  string
				moved model.Record
				want  bool
			}{
				{"the same unknown keys", movedWith(`"y": {"z": [true]}, "x": 1`), true},
				{"another value", movedWith(`"x": 2, "y": {"z": [true]}`), false},
				{"a key missing", movedWith(`"x": 1`), false},
				{"an extra key", movedWith(`"x": 1, "y": {"z": [true]}, "w": 0`), false},
				{"none", movedWith(""), false},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					if got := IsStaleCopy(stale, staleRec, moved, tt.moved); got != tt.want {
						t.Errorf("IsStaleCopy = %v, want %v", got, tt.want)
					}
				})
			}
			// The moved record has keys the stale one lacks.
			if IsStaleCopy(stale, askedRecord(3), moved, movedWith(`"x": 1`)) {
				t.Error("IsStaleCopy = true for a moved record with unknown keys the stale one lacks")
			}
		})
	}
}

// Renumber still finds a stale copy when the records carry unknown keys, and
// still moves a different record that has them.
func TestRenumberRefusesAStaleCopyWithUnknownKeys(t *testing.T) {
	root := fixture(t, "collision-interrupted")
	dir := filepath.Join(root, "decisions")
	const stale = `---
{"id":2,"title":"Beta","status":"open","created":"2026-10-02","future":{"a":1},"history":[{"date":"2026-10-02","event":"ask"}]}
---
Beta body.
`
	movedWith := func(future string) string {
		return `---
{"id":4,"title":"Beta","status":"open","created":"2026-10-02",` + future + `"history":[{"date":"2026-10-02","event":"ask"},{"date":"2026-11-02","event":"renumber","note":"from 2"}]}
---
Beta body.
`
	}

	writeFile(t, filepath.Join(dir, "0002-beta.md"), stale)
	writeFile(t, filepath.Join(dir, "0004-beta.md"), movedWith(`"future":{"a":1},`))
	before := tree(t, root)
	got, err := mustOpen(t, root).Renumber(renumberDate, false)
	if !errors.Is(err, ErrStaleCopy) {
		t.Fatalf("Renumber = %+v, %v; want ErrStaleCopy", got.Moves, err)
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("a refused Renumber changed the store")
	}

	// The same files with another unknown value: 0002-beta.md is a live record
	// that only looks like the copy, so it moves.
	writeFile(t, filepath.Join(dir, "0004-beta.md"), movedWith(`"future":{"a":2},`))
	got, err = mustOpen(t, root).Renumber(renumberDate, false)
	if err != nil {
		t.Fatalf("Renumber = %+v, %v; want the move of 0002-beta.md", got.Moves, err)
	}
	if len(got.Moves) != 1 || got.Moves[0].OldName != "0002-beta.md" {
		t.Errorf("moves = %+v, want 0002-beta.md moved", got.Moves)
	}
}

// A record that moves keeps its unknown top-level keys and the unknown keys of
// its was, byte for byte, with the renumber entry added.
func TestRenumberMovesARecordWithUnknownKeys(t *testing.T) {
	root := fixture(t, "collision")
	dir := filepath.Join(root, "decisions")
	writeFile(t, filepath.Join(dir, "0002-beta.md"), `---
{
  "id": 2,
  "title": "Beta",
  "status": "open",
  "created": "2026-10-02",
  "options": [
    {
      "label": "a",
      "summary": "New"
    }
  ],
  "history": [
    {
      "date": "2026-10-02",
      "event": "ask"
    },
    {
      "date": "2026-10-03",
      "event": "amend",
      "note": "summary a",
      "was": {
        "options": [
          {
            "label": "a",
            "summary": "Old",
            "url": "u"
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
Beta body.
`)
	if _, err := mustOpen(t, root).Renumber(renumberDate, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "0004-beta.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		`"url": "u"`, `"tags": [`, `"alpha": true,`, `"zeta": {`, `"note": "from 2"`, `"id": 4,`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("0004-beta.md lost %s:\n%s", want, text)
		}
	}
	rec := mustRead(t, filepath.Join(dir, "0004-beta.md"))
	if got, want := rec.UnknownKeys(), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("unknown keys = %v, want %v", got, want)
	}
	if got, want := rec.History[1].Was.UnknownKeys(), []string{"tags"}; !reflect.DeepEqual(got, want) {
		t.Errorf("was unknown keys = %v, want %v", got, want)
	}
}
