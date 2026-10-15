// SPDX-License-Identifier: GPL-3.0-or-later

// Package query selects records. Its functions are pure: they take records in
// memory and do no file I/O.
package query

import (
	"slices"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// Filter is the conditions a record must meet; a zero field is no condition.
// Every condition given must hold (AND). Status is one of the five statuses;
// Tags must all be on the record; Key must equal the record's key exactly;
// KeyPrefix must be a prefix of the record's key (a record with no key never
// matches it).
type Filter struct {
	Status    model.Status
	Tags      []string
	Key       string
	KeyPrefix string
}

// match reports whether r meets every condition of f.
func (f Filter) match(r model.Record) bool {
	if f.Status != "" && r.Status != f.Status {
		return false
	}
	if f.Key != "" && r.Key != f.Key {
		return false
	}
	if f.KeyPrefix != "" && !strings.HasPrefix(r.Key, f.KeyPrefix) {
		return false
	}
	for _, tag := range f.Tags {
		if !slices.Contains(r.Tags, tag) {
			return false
		}
	}
	return true
}

// Select returns the records of recs that meet f, in their order.
func Select(recs []model.Record, f Filter) []model.Record {
	var out []model.Record
	for _, r := range recs {
		if f.match(r) {
			out = append(out, r)
		}
	}
	return out
}
