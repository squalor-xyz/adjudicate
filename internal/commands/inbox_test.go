// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

func TestOldestFirst(t *testing.T) {
	rec := func(id int, created string) model.Record {
		return model.Record{ID: id, Created: created}
	}
	tests := []struct {
		name string
		in   []model.Record
		want []int
	}{
		{name: "empty", in: nil, want: []int{}},
		{name: "by created", in: []model.Record{rec(1, "2026-03-05"), rec(2, "2026-03-03"), rec(3, "2026-03-04")}, want: []int{2, 3, 1}},
		{name: "same date: lower ID first, whatever the input order", in: []model.Record{rec(9, "2026-03-03"), rec(10, "2026-03-03"), rec(2, "2026-03-03")}, want: []int{2, 9, 10}},
		{name: "date before ID", in: []model.Record{rec(1, "2026-03-05"), rec(2, "2026-03-03")}, want: []int{2, 1}},
		{name: "month and year order", in: []model.Record{rec(1, "2027-01-01"), rec(2, "2026-12-31"), rec(3, "2026-02-01")}, want: []int{3, 2, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := append([]model.Record(nil), tt.in...)
			out := oldestFirst(tt.in)
			got := []int{}
			for _, r := range out {
				got = append(got, r.ID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("IDs = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.in, before) {
				t.Errorf("oldestFirst changed its input: %v, was %v", tt.in, before)
			}
		})
	}
}

// Records that share an ID (C60) and a date are equal for the sort: they keep
// the order the store listed them in (file-name order).
func TestOldestFirstKeepsStoreOrderOfEqualRecords(t *testing.T) {
	titles := func(recs []model.Record) []string {
		var out []string
		for _, r := range recs {
			out = append(out, r.Title)
		}
		return out
	}
	t.Run("two records with the same ID and date", func(t *testing.T) {
		in := []model.Record{
			{ID: 3, Created: "2026-03-03", Title: "0003-b.md"},
			{ID: 3, Created: "2026-03-03", Title: "0003-a.md"},
		}
		want := []string{"0003-b.md", "0003-a.md"}
		if got := titles(oldestFirst(in)); !reflect.DeepEqual(got, want) {
			t.Errorf("order = %v, want %v", got, want)
		}
	})
	t.Run("many equal records among others", func(t *testing.T) {
		// 30 records, three IDs, one date: each ID's records stay in input order.
		var in []model.Record
		want := make([][]string, 3)
		for i := 0; i < 30; i++ {
			title := fmt.Sprintf("file-%02d", i)
			in = append(in, model.Record{ID: i%3 + 1, Created: "2026-03-03", Title: title})
			want[i%3] = append(want[i%3], title)
		}
		wantAll := append(append(want[0], want[1]...), want[2]...)
		if got := titles(oldestFirst(in)); !reflect.DeepEqual(got, wantAll) {
			t.Errorf("order = %v, want %v", got, wantAll)
		}
	})
}
