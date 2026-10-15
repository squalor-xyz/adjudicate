// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"testing"
)

func TestCheckInputRefusesABlankQuery(t *testing.T) {
	for _, q := range []string{"", " ", "\t\n "} {
		err := CheckInput{Query: q}.Check()
		var ue *UsageError
		if !errors.As(err, &ue) {
			t.Errorf("Check(%q) = %v, want a *UsageError", q, err)
		}
	}
}

func TestCheckInputAcceptsAQuery(t *testing.T) {
	for _, q := range []string{"cache", " cache layer ", "-x", "a"} {
		if err := (CheckInput{Query: q}).Check(); err != nil {
			t.Errorf("Check(%q) = %v, want nil", q, err)
		}
	}
}

// Check validates before it reads: a nil store is never touched for a blank
// query.
func TestCheckRefusesABlankQueryBeforeReadingTheStore(t *testing.T) {
	_, err := Check(nil, CheckInput{Query: "  "})
	var ue *UsageError
	if !errors.As(err, &ue) {
		t.Errorf("Check(nil, blank) = %v, want a *UsageError", err)
	}
}
