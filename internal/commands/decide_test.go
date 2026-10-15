// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// A change to a record that is not open fails with an error that names the
// record's status, still matches model.ErrInvalidTransition, and is not a
// usage error (so the CLI exits 1).
func TestClosedRecordErrorMatchesInvalidTransition(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 4, 23, 30, 0, 0, time.UTC)
	rec, err := Ask(st, AskInput{Title: "Pick a store"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Reject(st, rec.ID, ReasonInput{Why: "no"}, now); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		change  func() error
		wantErr string
	}{
		{"decide", func() error {
			_, err := Decide(st, rec.ID, DecideInput{Text: "x", HasText: true, Why: "w"}, now)
			return err
		}, "record 1 is rejected; only an open record can be decided"},
		{"withdraw", func() error {
			_, err := Withdraw(st, rec.ID, ReasonInput{Why: "w"}, now)
			return err
		}, "record 1 is rejected; only an open record can be withdrawn"},
	}
	for _, tt := range tests {
		err := tt.change()
		if err == nil {
			t.Errorf("%s: no error on a rejected record", tt.name)
			continue
		}
		if !errors.Is(err, model.ErrInvalidTransition) {
			t.Errorf("%s: err = %v, want one matching model.ErrInvalidTransition", tt.name, err)
		}
		if !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("%s: err = %q, want it to contain %q", tt.name, err, tt.wantErr)
		}
		var ue *UsageError
		if errors.As(err, &ue) {
			t.Errorf("%s: err = %v is a usage error, want an ordinary failure", tt.name, err)
		}
	}
}
