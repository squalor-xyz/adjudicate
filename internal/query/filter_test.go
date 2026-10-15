// SPDX-License-Identifier: GPL-3.0-or-later

package query

import (
	"reflect"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// sample is four records in ID order that differ in status, tags and key, so
// each filter keeps some of them and drops the rest.
var sample = []model.Record{
	{ID: 1, Title: "Storage", Status: model.StatusOpen, Key: "storage.engine", Tags: []string{"infra"}},
	{ID: 2, Title: "Cache", Status: model.StatusDecided, Key: "storage.cache", Tags: []string{"infra", "perf"}},
	{ID: 3, Title: "Theme", Status: model.StatusOpen, Key: "ui.theme", Tags: []string{"perf"}},
	{ID: 4, Title: "Logging", Status: model.StatusRejected},
}

// idsOf returns the IDs of recs in order. An empty result is []int{}, so it
// compares equal to an expected empty list.
func idsOf(recs []model.Record) []int {
	ids := []int{}
	for _, r := range recs {
		ids = append(ids, r.ID)
	}
	return ids
}

func TestSelect(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		want   []int
	}{
		{name: "no filter keeps every record", filter: Filter{}, want: []int{1, 2, 3, 4}},
		{name: "status", filter: Filter{Status: model.StatusOpen}, want: []int{1, 3}},
		{name: "status with no record", filter: Filter{Status: model.StatusSuperseded}, want: []int{}},
		{name: "one tag", filter: Filter{Tags: []string{"infra"}}, want: []int{1, 2}},
		{name: "every tag must match", filter: Filter{Tags: []string{"infra", "perf"}}, want: []int{2}},
		{name: "a repeated tag is the same tag", filter: Filter{Tags: []string{"perf", "perf"}}, want: []int{2, 3}},
		{name: "tag with no record", filter: Filter{Tags: []string{"legal"}}, want: []int{}},
		{name: "key is an exact match", filter: Filter{Key: "storage.engine"}, want: []int{1}},
		{name: "key is not a prefix match", filter: Filter{Key: "storage"}, want: []int{}},
		{name: "key prefix", filter: Filter{KeyPrefix: "storage"}, want: []int{1, 2}},
		{name: "key prefix is a plain string prefix, not a dotted segment", filter: Filter{KeyPrefix: "stor"}, want: []int{1, 2}},
		{name: "key prefix equal to a whole key", filter: Filter{KeyPrefix: "ui.theme"}, want: []int{3}},
		{name: "key prefix is case sensitive", filter: Filter{KeyPrefix: "Storage"}, want: []int{}},
		{name: "key prefix is not a substring match", filter: Filter{KeyPrefix: "cache"}, want: []int{}},
		{name: "key prefix never matches a record with no key", filter: Filter{KeyPrefix: "l"}, want: []int{}},
		{name: "key prefix longer than the key", filter: Filter{KeyPrefix: "storage.engine.x"}, want: []int{}},
		{name: "key prefix and tag", filter: Filter{KeyPrefix: "storage", Tags: []string{"perf"}}, want: []int{2}},
		{name: "key prefix and exact key", filter: Filter{KeyPrefix: "storage", Key: "ui.theme"}, want: []int{}},
		{name: "status and tag", filter: Filter{Status: model.StatusOpen, Tags: []string{"perf"}}, want: []int{3}},
		{name: "status and key", filter: Filter{Status: model.StatusOpen, Key: "storage.cache"}, want: []int{}},
		{name: "all three", filter: Filter{Status: model.StatusDecided, Tags: []string{"infra"}, Key: "storage.cache"}, want: []int{2}},
		{name: "all three, no record meets them", filter: Filter{Status: model.StatusDecided, Tags: []string{"perf"}, Key: "ui.theme"}, want: []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := idsOf(Select(sample, tt.filter)); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Select IDs = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectKeepsTheInputOrder(t *testing.T) {
	in := []model.Record{sample[3], sample[0]}
	if got := idsOf(Select(in, Filter{})); !reflect.DeepEqual(got, []int{4, 1}) {
		t.Errorf("Select IDs = %v, want [4 1]", got)
	}
}
