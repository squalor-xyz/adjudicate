// SPDX-License-Identifier: GPL-3.0-or-later

package lint_test

import (
	"reflect"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/lint"
)

// adj amend appends an amend history entry that holds the old text; lint
// accepts it, finds a broken one, and still tells a stale copy from a keeper.
func TestRunAndTheAmendEvent(t *testing.T) {
	stale := "0004-beta.md has this record renumbered from id 2: this file is a stale copy left by an interrupted renumber; check both, delete it"
	tests := []struct {
		name string
		want []lint.Finding
	}{
		{"amended", nil},
		{"amend-broken", []lint.Finding{
			finding(1, "0001-no-old-text.md", errSev, lint.CheckFields, "history: entry 1: an amend entry needs the old text in was"),
			finding(2, "0002-old-text-on-ask.md", errSev, lint.CheckFields, "history: entry 0: only an amend entry has was"),
			finding(3, "0003-option-without-label.md", errSev, lint.CheckFields, "history: entry 1: was.options: an option has no label"),
		}},
		{"renumber-leftover-amended", []lint.Finding{
			finding(2, "0002-beta.md", errSev, lint.CheckRenumberLeftover, stale),
		}},
		{"renumber-keeper-amended", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, root := open(t, tt.name)
			got, err := lint.Run(st, root)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings =\n%s\nwant\n%s", dump(got), dump(tt.want))
			}
		})
	}
}
