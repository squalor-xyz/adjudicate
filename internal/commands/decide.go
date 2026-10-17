// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// DecideInput is the raw input of adj decide. The answer is one option label
// (Option, with HasOption) or free text (Text, with HasText), never both. Why
// is the reason for it, and DecidedBy who decided (C27, C70).
type DecideInput struct {
	Option       string
	HasOption    bool
	Text         string
	HasText      bool
	Why          string
	DecidedBy    string
	DecidedAt    string
	HasDecidedAt bool
}

// ReasonInput is the raw input of adj reject and adj withdraw: the reason and
// who closed the question (C27, C70).
type ReasonInput struct {
	Why       string
	DecidedBy string
}

// Check reports a *UsageError if the input cannot answer a record whatever
// the record is: both or neither of OPTION and --text, a blank answer or no
// reason, or an invalid --decided-at date. Whether OPTION is one of the
// record's options needs the record, so Decide checks that.
func (in DecideInput) Check() error {
	switch {
	case in.HasOption && in.HasText:
		return usagef("give an OPTION or --text, not both")
	case !in.HasOption && !in.HasText:
		return usagef("give an OPTION or --text")
	case in.HasOption && strings.TrimSpace(in.Option) == "":
		return usagef("OPTION must not be blank")
	case in.HasText && strings.TrimSpace(in.Text) == "":
		return usagef("--text must not be blank")
	}
	if err := checkWhy(in.Why); err != nil {
		return err
	}
	if in.HasDecidedAt || in.DecidedAt != "" {
		if _, err := time.Parse(time.DateOnly, in.DecidedAt); err != nil {
			return usagef("--decided-at must be a valid YYYY-MM-DD date, got %q", in.DecidedAt)
		}
	}
	return nil
}

// CheckAt reports a *UsageError when the input date is after the run date.
func (in DecideInput) CheckAt(now time.Time) error {
	if err := in.Check(); err != nil {
		return err
	}
	if (in.HasDecidedAt || in.DecidedAt != "") && in.DecidedAt > now.Format(time.DateOnly) {
		return usagef("--decided-at %s is later than the run date %s", in.DecidedAt, now.Format(time.DateOnly))
	}
	return nil
}

// Check reports a *UsageError if there is no reason. Reject and Withdraw run
// it too.
func (in ReasonInput) Check() error {
	return checkWhy(in.Why)
}

// checkWhy reports a *UsageError if the reason is missing or blank.
func checkWhy(why string) error {
	if strings.TrimSpace(why) == "" {
		return usagef("--why is required and must not be blank")
	}
	return nil
}

// Decide answers the open record with id, setting decided_at to the requested
// date or to now, and dating its history at now. Its decision is the option
// label or free text, and its rationale is in.Why. It returns the record as
// stored. Bad input is a *UsageError, and so is an OPTION that the record does
// not offer, when the record is open. A missing record, or one that is not
// open, is an ordinary error, and a record that is not open is reported before
// OPTION is checked. Nothing is written on error.
func Decide(st *store.Store, id int, in DecideInput, now time.Time) (model.Record, error) {
	if err := in.CheckAt(now); err != nil {
		return model.Record{}, err
	}
	runDate := now.Format(time.DateOnly)
	decidedAt := runDate
	if in.HasDecidedAt || in.DecidedAt != "" {
		decidedAt = in.DecidedAt
	}
	return change(st, id, "decided", func(r *model.Record) error {
		decision := in.Text
		if in.HasOption {
			label := strings.TrimSpace(in.Option)
			// Only an open record has options to check. On any other record the
			// transition below reports its status, so that error comes first.
			if r.Status == model.StatusOpen && !hasOption(*r, label) {
				return usagef("OPTION %q is not one of the options of record %d", label, r.ID)
			}
			decision = label
		}
		return model.DecideAt(r, decision, in.Why, in.DecidedBy, decidedAt, runDate)
	})
}

// Reject closes the open record with id as rejected, dated now: the question
// will not be answered. The reason is stored as its rationale.
func Reject(st *store.Store, id int, in ReasonInput, now time.Time) (model.Record, error) {
	if err := in.Check(); err != nil {
		return model.Record{}, err
	}
	date := now.Format(time.DateOnly)
	return change(st, id, "rejected", func(r *model.Record) error {
		return model.Reject(r, in.Why, in.DecidedBy, date)
	})
}

// Withdraw closes the open record with id as withdrawn, dated now: the asker
// no longer needs an answer. The reason is stored as its rationale.
func Withdraw(st *store.Store, id int, in ReasonInput, now time.Time) (model.Record, error) {
	if err := in.Check(); err != nil {
		return model.Record{}, err
	}
	date := now.Format(time.DateOnly)
	return change(st, id, "withdrawn", func(r *model.Record) error {
		return model.Withdraw(r, in.Why, in.DecidedBy, date)
	})
}

// notOpenError is a transition refused because the record is not open. Its
// message names the record's status and the change that needs an open record.
// Unwrap keeps the model's error, so errors.Is(err, model.ErrInvalidTransition)
// still holds.
type notOpenError struct {
	id     int
	status model.Status
	verb   string
	cause  error
}

func (e *notOpenError) Error() string {
	return fmt.Sprintf("record %d is %s; only an open record can be %s", e.id, e.status, e.verb)
}

func (e *notOpenError) Unwrap() error { return e.cause }

// change reads the record with id, applies transition to it and stores the
// result. A transition that fails leaves the stored record as it was. verb is
// the status the transition moves to, for the message when the record is not
// open.
func change(st *store.Store, id int, verb string, transition func(*model.Record) error) (model.Record, error) {
	rec, err := st.Get(id)
	if err != nil {
		return model.Record{}, err
	}
	if err := transition(&rec); err != nil {
		if errors.Is(err, model.ErrInvalidTransition) {
			return model.Record{}, &notOpenError{id: id, status: rec.Status, verb: verb, cause: err}
		}
		return model.Record{}, err
	}
	if err := st.Update(rec); err != nil {
		return model.Record{}, err
	}
	return rec, nil
}

// hasOption reports whether the record offers an option with the label.
func hasOption(r model.Record, label string) bool {
	for _, o := range r.Options {
		if o.Label == label {
			return true
		}
	}
	return false
}
