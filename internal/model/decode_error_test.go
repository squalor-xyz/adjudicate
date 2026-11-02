// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"strings"
	"testing"
)

// A history entry with a value of the wrong type gives an error in plain words:
// the key, the type it must have and the type it has. It names no Go type and
// no Go struct field (S39).
func TestHistoryTypeErrorsNameTheKeyAndTheExpectedType(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{"event a number", `{"date":"2026-10-01","event":5}`, `history entry: "event" must be a string, got a number`},
		{"event an object", `{"date":"2026-10-01","event":{}}`, `history entry: "event" must be a string, got an object`},
		{"date a number", `{"date":5,"event":"ask"}`, `history entry: "date" must be a string, got a number`},
		{"note a boolean", `{"date":"2026-10-01","event":"ask","note":true}`, `history entry: "note" must be a string, got a boolean`},
		{"note an array", `{"date":"2026-10-01","event":"ask","note":["a"]}`, `history entry: "note" must be a string, got an array`},
		{"was title a number", `{"date":"2026-10-01","event":"amend","was":{"title":5}}`, `history entry: was: "title" must be a string, got a number`},
		{"was option label a number", `{"date":"2026-10-01","event":"amend","was":{"options":[{"label":5}]}}`, `history entry: was: "label" must be a string, got a number`},
		{"was options a string", `{"date":"2026-10-01","event":"amend","was":{"options":"a"}}`, `history entry: was: "options" must be an array, got a string`},
		{"entry an array", `[]`, `history entry: want a JSON object, got an array`},
		{"entry a number", `7`, `history entry: want a JSON object, got a number`},
		{"was a string", `{"date":"2026-10-01","event":"amend","was":"text"}`, `history entry: was: want a JSON object, got a string`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("x.md", openFile(tt.entry))
			if err == nil {
				t.Fatal("Parse() = nil, want an error")
			}
			msg := err.Error()
			if !strings.Contains(msg, tt.want) {
				t.Errorf("error %q does not contain %q", msg, tt.want)
			}
			for _, leak := range []string{"Go struct field", "model.", "cannot unmarshal", "of type", "json:"} {
				if strings.Contains(msg, leak) {
					t.Errorf("error %q mentions %q", msg, leak)
				}
			}
		})
	}
}
