// SPDX-License-Identifier: GPL-3.0-or-later

// Package model defines the adjudicate record, its validation and its
// lifecycle transitions. It does no file I/O.
package model

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrInvalid is matched (errors.Is) by every error this package returns for
// a record or a transition input that breaks the model's rules.
var ErrInvalid = errors.New("invalid record")

// Status is a record's lifecycle state. Status alone carries the record kind
// (C30): open is a question; decided, rejected, withdrawn and superseded are
// answered.
type Status string

const (
	StatusOpen       Status = "open"
	StatusDecided    Status = "decided"
	StatusRejected   Status = "rejected"
	StatusWithdrawn  Status = "withdrawn"
	StatusSuperseded Status = "superseded"
)

// Valid reports whether s is one of the five statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusDecided, StatusRejected, StatusWithdrawn, StatusSuperseded:
		return true
	}
	return false
}

// Event names what a history entry records: the transition that was applied.
type Event string

const (
	EventAsk       Event = "ask" // written by the code that creates the record
	EventDecide    Event = "decide"
	EventReject    Event = "reject"
	EventWithdraw  Event = "withdraw"
	EventSupersede Event = "supersede"
)

// Option is one answer a question offers.
type Option struct {
	Label     string `json:"label"`
	Summary   string `json:"summary,omitempty"`
	Tradeoffs string `json:"tradeoffs,omitempty"`
}

// Recommendation is the asker's suggested option and why.
type Recommendation struct {
	Label     string `json:"label"`
	Reasoning string `json:"reasoning,omitempty"`
}

// Valid reports whether e is one of the five events.
func (e Event) Valid() bool {
	switch e {
	case EventAsk, EventDecide, EventReject, EventWithdraw, EventSupersede:
		return true
	}
	return false
}

// HistoryEntry is one dated line of a record's history. Date is
// time.DateOnly (YYYY-MM-DD).
type HistoryEntry struct {
	Date  string `json:"date"`
	Event Event  `json:"event"`
	Note  string `json:"note,omitempty"`
}

// Record is one question or decision. Field order follows the PROJECT.md
// Core concepts table (minus kind) so encoding/json output is deterministic.
// Dates are time.DateOnly strings (YYYY-MM-DD). AskedBy and DecidedBy are
// free-text attribution, not authentication (C27).
type Record struct {
	ID             int             `json:"id"`
	Title          string          `json:"title"`
	Status         Status          `json:"status"`
	Key            string          `json:"key,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	AskedBy        string          `json:"asked_by,omitempty"`
	DecidedBy      string          `json:"decided_by,omitempty"`
	Created        string          `json:"created"`
	DecidedAt      string          `json:"decided_at,omitempty"`
	Context        string          `json:"context,omitempty"`
	Options        []Option        `json:"options,omitempty"`
	Recommendation *Recommendation `json:"recommendation,omitempty"`
	Decision       string          `json:"decision,omitempty"`
	Rationale      string          `json:"rationale,omitempty"`
	Consequences   string          `json:"consequences,omitempty"`
	Supersedes     []int           `json:"supersedes,omitempty"`
	SupersededBy   int             `json:"superseded_by,omitempty"`
	Refs           []string        `json:"refs,omitempty"`
	History        []HistoryEntry  `json:"history,omitempty"`
}

// validDate reports whether s is a YYYY-MM-DD date.
func validDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

// Validate checks the record against the model's rules and returns nil, or an
// error matching ErrInvalid that names every problem found.
//
// Always required: id > 0, title (one non-blank line), a known status and a
// created date. Every history entry needs a valid date and a known event.
// Once answered (decided, rejected, withdrawn, superseded):
// rationale and decided_at; decided and superseded also need decision, and
// superseded needs superseded_by. decided_by is never required (C27).
func (r Record) Validate() error {
	var errs []error
	bad := func(field, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%w: %s: %s", ErrInvalid, field, fmt.Sprintf(format, args...)))
	}

	if r.ID <= 0 {
		bad("id", "must be a positive integer, got %d", r.ID)
	}
	if strings.TrimSpace(r.Title) == "" {
		bad("title", "is required")
	} else if strings.ContainsAny(r.Title, "\r\n") {
		bad("title", "must be one line")
	}
	if !r.Status.Valid() {
		bad("status", "unknown status %q", string(r.Status))
	}
	if r.Created == "" {
		bad("created", "is required")
	} else if !validDate(r.Created) {
		bad("created", "%q is not a YYYY-MM-DD date", r.Created)
	}

	answered := r.Status.Valid() && r.Status != StatusOpen
	if answered && strings.TrimSpace(r.Rationale) == "" {
		bad("rationale", "is required once %s", r.Status)
	}
	if answered && r.DecidedAt == "" {
		bad("decided_at", "is required once %s", r.Status)
	} else if r.DecidedAt != "" && !validDate(r.DecidedAt) {
		bad("decided_at", "%q is not a YYYY-MM-DD date", r.DecidedAt)
	}
	if (r.Status == StatusDecided || r.Status == StatusSuperseded) && strings.TrimSpace(r.Decision) == "" {
		bad("decision", "is required once %s", r.Status)
	}
	if r.Status == StatusSuperseded && r.SupersededBy <= 0 {
		bad("superseded_by", "is required once superseded")
	}
	if r.SupersededBy != 0 && (r.SupersededBy < 0 || r.SupersededBy == r.ID) {
		bad("superseded_by", "%d is not another record's id", r.SupersededBy)
	}
	for _, id := range r.Supersedes {
		if id <= 0 || id == r.ID {
			bad("supersedes", "%d is not another record's id", id)
		}
	}

	for i, h := range r.History {
		if !validDate(h.Date) {
			bad("history", "entry %d: %q is not a YYYY-MM-DD date", i, h.Date)
		}
		if !h.Event.Valid() {
			bad("history", "entry %d: unknown event %q", i, string(h.Event))
		}
	}

	labels := make(map[string]bool, len(r.Options))
	for _, o := range r.Options {
		switch {
		case o.Label == "":
			bad("options", "an option has no label")
		case labels[o.Label]:
			bad("options", "duplicate label %q", o.Label)
		}
		labels[o.Label] = true
	}
	if rec := r.Recommendation; rec != nil {
		if !labels[rec.Label] {
			bad("recommendation", "label %q is not one of the options", rec.Label)
		}
	}

	return errors.Join(errs...)
}
