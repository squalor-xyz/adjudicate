// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"fmt"
	"slices"
)

// ErrInvalidTransition is returned when a record's current status does not
// allow the requested transition. It also matches ErrInvalid.
var ErrInvalidTransition = fmt.Errorf("%w: transition not allowed", ErrInvalid)

// apply runs a transition: it requires the record to be in status from,
// changes a copy with mutate, appends one history entry, validates the
// result and only then stores it in r. On any error r is left untouched.
func apply(r *Record, from, to Status, event Event, date, note string, mutate func(*Record)) error {
	if r.Status != from {
		return fmt.Errorf("%w: cannot %s a record that is %s (needs %s)", ErrInvalidTransition, event, r.Status, from)
	}
	if !validDate(date) {
		return fmt.Errorf("%w: date: %q is not a YYYY-MM-DD date", ErrInvalid, date)
	}
	next := *r
	next.History = append(slices.Clip(r.History), HistoryEntry{Date: date, Event: event, Note: note})
	next.Status = to
	mutate(&next)
	if err := next.Validate(); err != nil {
		return fmt.Errorf("cannot %s record %d: %w", event, r.ID, err)
	}
	*r = next
	return nil
}

// Decide moves an open record to decided, recording the chosen option label
// or free-text decision, the owner's rationale, who decided (free text; may
// be empty) and the date (YYYY-MM-DD).
func Decide(r *Record, decision, rationale, decidedBy, date string) error {
	return apply(r, StatusOpen, StatusDecided, EventDecide, date, "", func(n *Record) {
		n.Decision, n.Rationale, n.DecidedBy, n.DecidedAt = decision, rationale, decidedBy, date
	})
}

// Reject moves an open record to rejected: the question will not be answered.
func Reject(r *Record, rationale, decidedBy, date string) error {
	return apply(r, StatusOpen, StatusRejected, EventReject, date, "", func(n *Record) {
		n.Rationale, n.DecidedBy, n.DecidedAt = rationale, decidedBy, date
	})
}

// Withdraw moves an open record to withdrawn: the asker no longer needs an answer.
func Withdraw(r *Record, rationale, decidedBy, date string) error {
	return apply(r, StatusOpen, StatusWithdrawn, EventWithdraw, date, "", func(n *Record) {
		n.Rationale, n.DecidedBy, n.DecidedAt = rationale, decidedBy, date
	})
}

// Supersede moves a decided record to superseded and links it to the record
// that replaces it. It changes only r; setting the new record's Supersedes
// link is the caller's job. The history note is "by <id>".
func Supersede(r *Record, by int, date string) error {
	return apply(r, StatusDecided, StatusSuperseded, EventSupersede, date, fmt.Sprintf("by %d", by), func(n *Record) {
		n.SupersededBy = by
	})
}

// LinkSupersedes records on r, a decided record, that it replaces the record
// with ID old: it adds old to Supersedes (once) and appends one supersede
// history entry with the note "supersedes <old>", dated date (YYYY-MM-DD). r
// stays decided (C135). It is the other half of Supersede, which changes only the
// old record (C46); the supersede command applies both.
func LinkSupersedes(r *Record, old int, date string) error {
	return apply(r, StatusDecided, StatusDecided, EventSupersede, date, fmt.Sprintf("supersedes %d", old), func(n *Record) {
		if !slices.Contains(n.Supersedes, old) {
			n.Supersedes = append(slices.Clip(n.Supersedes), old)
		}
	})
}
