// SPDX-License-Identifier: GPL-3.0-or-later

package query

import (
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// Outcome is what a search found: the words adj check prints.
type Outcome string

const (
	Decided  Outcome = "decided"
	Open     Outcome = "open"
	Closed   Outcome = "closed"
	NotFound Outcome = "not found"
)

// Result is what Search found. Query is the search text with surrounding space
// removed. Decided, Open and Closed are never nil.
type Result struct {
	Query   string
	Outcome Outcome
	Decided []model.Record // matching decided records, and the decided end of a superseded match's chain
	Open    []model.Record // matching records with status open
	Closed  []model.Record // matching records with status rejected, withdrawn or superseded
}

// Search looks for q in the title, key, tags and decision text of recs. A
// record matches when any of those contains q, ignoring case (C105). A match
// is listed by its status: decided, open, or closed for a rejected, withdrawn
// or superseded record (C106). For a superseded match, superseded_by is
// followed to the end of its chain; if the last record is decided it is listed
// as decided as well, once, and the superseded record stays in Closed (C113).
// The outcome is Decided if Decided is not empty, else Open, else Closed, else
// NotFound (C107). Each list keeps the order of recs. A query that is blank
// once trimmed matches nothing.
func Search(recs []model.Record, q string) Result {
	q = strings.TrimSpace(q)
	res := Result{Query: q, Outcome: NotFound, Decided: []model.Record{}, Open: []model.Record{}, Closed: []model.Record{}}
	if q == "" {
		return res
	}
	needle := strings.ToLower(q)
	decided := map[int]bool{} // indexes into recs
	for i, r := range recs {
		if !mentions(r, needle) {
			continue
		}
		switch r.Status {
		case model.StatusDecided:
			decided[i] = true
		case model.StatusOpen:
			res.Open = append(res.Open, r)
		default:
			res.Closed = append(res.Closed, r)
			if r.Status == model.StatusSuperseded {
				if end, ok := decidedEnd(recs, i); ok {
					decided[end] = true
				}
			}
		}
	}
	for i, r := range recs {
		if decided[i] {
			res.Decided = append(res.Decided, r)
		}
	}
	switch {
	case len(res.Decided) > 0:
		res.Outcome = Decided
	case len(res.Open) > 0:
		res.Outcome = Open
	case len(res.Closed) > 0:
		res.Outcome = Closed
	}
	return res
}

// decidedEnd follows superseded_by from recs[start] through superseded records
// and returns the index of the decided record the chain ends in. It reports
// false when the chain ends anywhere else: no link, an ID that is not in recs,
// a record that is neither superseded nor decided, or an ID seen before (a
// cycle). Each record is visited once, so it always stops.
func decidedEnd(recs []model.Record, start int) (int, bool) {
	seen := map[int]bool{recs[start].ID: true}
	cur := start
	for {
		next := recs[cur].SupersededBy
		if next == 0 || seen[next] {
			return 0, false
		}
		seen[next] = true
		i := indexOf(recs, next)
		if i < 0 {
			return 0, false
		}
		switch recs[i].Status {
		case model.StatusDecided:
			return i, true
		case model.StatusSuperseded:
			cur = i
		default:
			return 0, false
		}
	}
}

// indexOf returns the index of the first record of recs with the ID, or -1.
func indexOf(recs []model.Record, id int) int {
	for i, r := range recs {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// mentions reports whether the title, key, a tag or the decision of r contains
// needle, which is already lower case.
func mentions(r model.Record, needle string) bool {
	fields := append([]string{r.Title, r.Key, r.Decision}, r.Tags...)
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), needle) {
			return true
		}
	}
	return false
}
