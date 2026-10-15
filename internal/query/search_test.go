// SPDX-License-Identifier: GPL-3.0-or-later

package query

import (
	"reflect"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// searchSample is records that differ in status and in which field a query can
// hit: title, key, tag or decision text. Records 9 to 11 are a supersede chain
// that ends in a decided record, 12 and 13 a cycle, 14 a dangling link, 15 a
// chain that ends in an open record and 16 a record that replaces itself.
var searchSample = []model.Record{
	{ID: 1, Title: "Pick a store", Status: model.StatusOpen, Key: "storage.engine", Tags: []string{"infra"},
		Options: []model.Option{{Label: "zeta-label", Summary: "zeta-summary", Tradeoffs: "zeta-tradeoffs"}},
		Context: "zeta-context", Body: "zeta-body\n"},
	{ID: 2, Title: "Cache layer", Status: model.StatusDecided, Key: "storage.cache", Tags: []string{"perf", "Backend"}, Decision: "Use Redis", Rationale: "because it is fast"},
	{ID: 3, Title: "Theme", Status: model.StatusRejected, Key: "ui.theme", Tags: []string{"ui"}},
	{ID: 4, Title: "Logging format", Status: model.StatusOpen, Tags: []string{"ops"}},
	{ID: 5, Title: "Queue", Status: model.StatusWithdrawn, Key: "infra.queue"},
	{ID: 6, Title: "Old database", Status: model.StatusSuperseded, SupersededBy: 7, Key: "storage.db", Decision: "postgres", Tags: []string{"legacy"}},
	{ID: 7, Title: "Database", Status: model.StatusDecided, Key: "storage.db", Decision: "sqlite"},
	{ID: 8, Title: "Deploy target", Status: model.StatusOpen, Tags: []string{"infra"}},
	{ID: 9, Title: "Chain first", Status: model.StatusSuperseded, SupersededBy: 10, Decision: "alpha-choice"},
	{ID: 10, Title: "Chain second", Status: model.StatusSuperseded, SupersededBy: 11, Decision: "beta-choice"},
	{ID: 11, Title: "Chain third", Status: model.StatusDecided, Decision: "gamma-choice"},
	{ID: 12, Title: "Loop one", Status: model.StatusSuperseded, SupersededBy: 13},
	{ID: 13, Title: "Loop two", Status: model.StatusSuperseded, SupersededBy: 12},
	{ID: 14, Title: "Dangling", Status: model.StatusSuperseded, SupersededBy: 99},
	{ID: 15, Title: "Ends open", Status: model.StatusSuperseded, SupersededBy: 8},
	{ID: 16, Title: "Self", Status: model.StatusSuperseded, SupersededBy: 16},
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantOutcome Outcome
		wantDecided []int
		wantOpen    []int
		wantClosed  []int
	}{
		{name: "title of a decided record", query: "Cache layer", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "key of a decided record", query: "storage.cache", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "tag of a decided record", query: "perf", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "decision text of a decided record", query: "redis", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "title of an open record", query: "Pick a store", wantOutcome: Open, wantOpen: []int{1}},
		{name: "key of an open record", query: "storage.engine", wantOutcome: Open, wantOpen: []int{1}},
		{name: "tag of an open record", query: "ops", wantOutcome: Open, wantOpen: []int{4}},
		{name: "only open records match: open with all their IDs", query: "deploy", wantOutcome: Open, wantOpen: []int{8}},
		{name: "no match", query: "kubernetes", wantOutcome: NotFound},
		{name: "case does not matter in the query", query: "CACHE LAYER", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "case does not matter in the record", query: "backend", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "a substring of a word matches", query: "ache lay", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "a tag is matched as a substring too", query: "back", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "surrounding space is ignored", query: "  cache layer \n", wantOutcome: Decided, wantDecided: []int{2}},
		{name: "the whole query is one substring, not a set of words", query: "cache redis", wantOutcome: NotFound},
		{name: "decided and open both match: decided wins, both are listed", query: "storage", wantOutcome: Decided, wantDecided: []int{2, 7}, wantOpen: []int{1}, wantClosed: []int{6}},
		{name: "open and closed match: open wins", query: "infra", wantOutcome: Open, wantOpen: []int{1, 8}, wantClosed: []int{5}},
		{name: "a rejected record is closed", query: "theme", wantOutcome: Closed, wantClosed: []int{3}},
		{name: "a rejected record's key is searched", query: "ui.theme", wantOutcome: Closed, wantClosed: []int{3}},
		{name: "a withdrawn record is closed", query: "queue", wantOutcome: Closed, wantClosed: []int{5}},
		{name: "a superseded record is closed, and its replacement is decided", query: "postgres", wantOutcome: Decided, wantDecided: []int{7}, wantClosed: []int{6}},
		{name: "a superseded record's tag is searched", query: "legacy", wantOutcome: Decided, wantDecided: []int{7}, wantClosed: []int{6}},
		{name: "the replacement and the old record both match: decided once, closed once", query: "database", wantOutcome: Decided, wantDecided: []int{7}, wantClosed: []int{6}},
		{name: "a chain is followed to its decided end", query: "alpha-choice", wantOutcome: Decided, wantDecided: []int{11}, wantClosed: []int{9}},
		{name: "a chain is followed from its middle", query: "beta-choice", wantOutcome: Decided, wantDecided: []int{11}, wantClosed: []int{10}},
		{name: "every record of a chain that matches is closed, the end is decided once", query: "choice", wantOutcome: Decided, wantDecided: []int{11}, wantClosed: []int{9, 10}},
		{name: "a cycle ends the search: closed only", query: "loop", wantOutcome: Closed, wantClosed: []int{12, 13}},
		{name: "a record that replaces itself ends the search", query: "self", wantOutcome: Closed, wantClosed: []int{16}},
		{name: "a dangling superseded_by: closed only", query: "dangling", wantOutcome: Closed, wantClosed: []int{14}},
		{name: "a chain that ends in a record that is not decided: closed only", query: "ends open", wantOutcome: Closed, wantClosed: []int{15}},
		{name: "a blank query matches nothing", query: "   ", wantOutcome: NotFound},
		{name: "an empty query matches nothing", query: "", wantOutcome: NotFound},
		{name: "the rationale is not searched", query: "because", wantOutcome: NotFound},
		{name: "an option label is not searched", query: "zeta-label", wantOutcome: NotFound},
		{name: "an option summary is not searched", query: "zeta-summary", wantOutcome: NotFound},
		{name: "option tradeoffs are not searched", query: "zeta-tradeoffs", wantOutcome: NotFound},
		{name: "the context is not searched", query: "zeta-context", wantOutcome: NotFound},
		{name: "the body is not searched", query: "zeta-body", wantOutcome: NotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Search(searchSample, tt.query)
			if got.Outcome != tt.wantOutcome {
				t.Errorf("Outcome = %q, want %q", got.Outcome, tt.wantOutcome)
			}
			if ids := idsOf(got.Decided); !reflect.DeepEqual(ids, orEmpty(tt.wantDecided)) {
				t.Errorf("Decided IDs = %v, want %v", ids, tt.wantDecided)
			}
			if ids := idsOf(got.Open); !reflect.DeepEqual(ids, orEmpty(tt.wantOpen)) {
				t.Errorf("Open IDs = %v, want %v", ids, tt.wantOpen)
			}
			if ids := idsOf(got.Closed); !reflect.DeepEqual(ids, orEmpty(tt.wantClosed)) {
				t.Errorf("Closed IDs = %v, want %v", ids, tt.wantClosed)
			}
		})
	}
}

// orEmpty turns nil into []int{} so it compares equal to idsOf's empty result.
func orEmpty(ids []int) []int {
	if ids == nil {
		return []int{}
	}
	return ids
}

func TestSearchOutcomeWhenOnlyOpenRecordsMatch(t *testing.T) {
	got := Search(searchSample, "logging")
	if got.Outcome != Open || !reflect.DeepEqual(idsOf(got.Open), []int{4}) || len(got.Decided) != 0 || len(got.Closed) != 0 {
		t.Errorf("Search = %+v, want outcome open with record 4 only", got)
	}
}

func TestSearchOutcomeNamesAreTheCLIWords(t *testing.T) {
	if Decided != "decided" || Open != "open" || Closed != "closed" || NotFound != "not found" {
		t.Errorf("outcomes = %q, %q, %q, %q; want decided, open, closed, not found", Decided, Open, Closed, NotFound)
	}
}

func TestSearchReportsTheTrimmedQuery(t *testing.T) {
	if got := Search(searchSample, "  Cache layer \n").Query; got != "Cache layer" {
		t.Errorf("Query = %q, want %q", got, "Cache layer")
	}
}

func TestSearchKeepsTheInputOrderAndDoesNotChangeIt(t *testing.T) {
	in := []model.Record{searchSample[6], searchSample[1], searchSample[0]}
	before := append([]model.Record(nil), in...)
	got := Search(in, "storage")
	if !reflect.DeepEqual(idsOf(got.Decided), []int{7, 2}) || !reflect.DeepEqual(idsOf(got.Open), []int{1}) {
		t.Errorf("Decided = %v, Open = %v; want [7 2] and [1]", idsOf(got.Decided), idsOf(got.Open))
	}
	if !reflect.DeepEqual(in, before) {
		t.Error("Search changed its input")
	}
}

// A decided record reached by following superseded_by is listed in the order
// of the input, with the records that matched directly.
func TestSearchListsAFollowedRecordInInputOrder(t *testing.T) {
	in := []model.Record{
		{ID: 1, Title: "Cache", Status: model.StatusDecided},
		{ID: 2, Title: "Old store", Status: model.StatusSuperseded, SupersededBy: 3, Key: "cache.old"},
		{ID: 3, Title: "Store", Status: model.StatusDecided},
		{ID: 4, Title: "Cache two", Status: model.StatusDecided},
	}
	got := Search(in, "cache")
	if want := []int{1, 3, 4}; !reflect.DeepEqual(idsOf(got.Decided), want) {
		t.Errorf("Decided IDs = %v, want %v", idsOf(got.Decided), want)
	}
	if want := []int{2}; !reflect.DeepEqual(idsOf(got.Closed), want) {
		t.Errorf("Closed IDs = %v, want %v", idsOf(got.Closed), want)
	}
}

func TestSearchNoRecords(t *testing.T) {
	got := Search(nil, "anything")
	if got.Outcome != NotFound || got.Decided == nil || got.Open == nil || got.Closed == nil || len(got.Decided)+len(got.Open)+len(got.Closed) != 0 {
		t.Errorf("Search(nil) = %+v, want not found with empty non-nil lists", got)
	}
}
