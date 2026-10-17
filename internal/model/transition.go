// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"fmt"
	"maps"
	"slices"
	"strings"
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
	return DecideAt(r, decision, rationale, decidedBy, date, date)
}

// DecideAt moves an open record to decided, setting decidedAt as the date the
// decision took effect and date as the date this transition was recorded.
// When those dates differ, the history entry notes the backdated date.
func DecideAt(r *Record, decision, rationale, decidedBy, decidedAt, date string) error {
	note := ""
	if decidedAt != date {
		note = fmt.Sprintf("backdated to %s", decidedAt)
	}
	return apply(r, StatusOpen, StatusDecided, EventDecide, date, note, func(n *Record) {
		n.Decision, n.Rationale, n.DecidedBy, n.DecidedAt = decision, rationale, decidedBy, decidedAt
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

// ErrNoChange is matched by the error Amend returns when no text it was given
// differs from the record's.
var ErrNoChange = fmt.Errorf("%w: nothing to change", ErrInvalid)

// Amendment is what Amend changes. A nil pointer leaves the field alone;
// Summaries and Tradeoffs map an option label to its new text.
type Amendment struct {
	Title, Context, Rationale *string
	Summaries, Tradeoffs      map[string]string
}

// Amend corrects text on r, a record in any status, and appends one amend
// history entry dated date (YYYY-MM-DD) whatever the number of fields changed
// (C166). The entry's Was keeps the old text of every field that changed, and
// its note names them: title, context, then for each option in order summary
// and tradeoffs, then rationale. A field given the text the record already has
// is not a change and is not in the entry. Nothing else changes: not the
// status, decision, decided_by, dates, ID, links, key, tags, refs, an option's
// label or the recommendation.
//
// An open record has no rationale, so Rationale on it is refused
// (ErrInvalidTransition); a label in Summaries or Tradeoffs that no option has
// is an error matching ErrInvalid; an amend that changes nothing returns
// ErrNoChange. The result is validated on a copy (a title must stay one
// non-blank line, a rationale must stay non-blank once answered), so on error
// r is untouched (C45).
func Amend(r *Record, a Amendment, date string) error {
	if !validDate(date) {
		return fmt.Errorf("%w: date: %q is not a YYYY-MM-DD date", ErrInvalid, date)
	}
	if a.Rationale != nil && r.Status == StatusOpen {
		return fmt.Errorf("%w: cannot amend the rationale of record %d: it is open and has none", ErrInvalidTransition, r.ID)
	}
	for _, texts := range []map[string]string{a.Summaries, a.Tradeoffs} {
		for _, label := range slices.Sorted(maps.Keys(texts)) {
			if !slices.ContainsFunc(r.Options, func(o Option) bool { return o.Label == label }) {
				return fmt.Errorf("%w: options: record %d has no option %q", ErrInvalid, r.ID, label)
			}
		}
	}

	next := *r
	next.Options = slices.Clone(r.Options)
	var was Was
	var changed []string
	if a.Title != nil && *a.Title != r.Title {
		was.Title, next.Title = ptr(r.Title), *a.Title
		changed = append(changed, "title")
	}
	if a.Context != nil && *a.Context != r.Context {
		was.Context, next.Context = ptr(r.Context), *a.Context
		changed = append(changed, "context")
	}
	for i, o := range r.Options {
		ow := OptionWas{Label: o.Label}
		if text, ok := a.Summaries[o.Label]; ok && text != o.Summary {
			ow.Summary, next.Options[i].Summary = ptr(o.Summary), text
			changed = append(changed, "summary "+o.Label)
		}
		if text, ok := a.Tradeoffs[o.Label]; ok && text != o.Tradeoffs {
			ow.Tradeoffs, next.Options[i].Tradeoffs = ptr(o.Tradeoffs), text
			changed = append(changed, "tradeoffs "+o.Label)
		}
		if ow.Summary != nil || ow.Tradeoffs != nil {
			was.Options = append(was.Options, ow)
		}
	}
	if a.Rationale != nil && *a.Rationale != r.Rationale {
		was.Rationale, next.Rationale = ptr(r.Rationale), *a.Rationale
		changed = append(changed, "rationale")
	}
	if len(changed) == 0 {
		return fmt.Errorf("cannot amend record %d: %w", r.ID, ErrNoChange)
	}
	next.History = append(slices.Clip(r.History), HistoryEntry{Date: date, Event: EventAmend, Note: strings.Join(changed, ", "), Was: &was})
	if err := next.Validate(); err != nil {
		return fmt.Errorf("cannot amend record %d: %w", r.ID, err)
	}
	*r = next
	return nil
}

// ptr returns a pointer to a copy of s: the old text must not alias a field
// that Amend is about to overwrite.
func ptr(s string) *string { return &s }

// Renumber gives r a new ID and appends one renumber history entry with the
// note "from <old ID>", dated date (YYYY-MM-DD). It works on a record in any
// status and changes nothing else, links included: renumbering resolves two
// files with one ID (C26), and a link to the old ID cannot be told apart from
// a link to the other record (C147). newID must be positive and differ from
// the current ID, and date must be a YYYY-MM-DD date. The result is validated
// on a copy (the history entry's date included), so on error r is untouched
// (C45).
func Renumber(r *Record, newID int, date string) error {
	if newID == r.ID {
		return fmt.Errorf("%w: id: record %d is already %d", ErrInvalid, r.ID, newID)
	}
	next := *r
	next.ID = newID
	next.History = append(slices.Clip(r.History), HistoryEntry{Date: date, Event: EventRenumber, Note: fmt.Sprintf("from %d", r.ID)})
	if err := next.Validate(); err != nil {
		return fmt.Errorf("cannot renumber record %d: %w", r.ID, err)
	}
	*r = next
	return nil
}
