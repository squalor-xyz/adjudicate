// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// AmendInput is the raw input of adj amend (C166): the new text of the fields
// it may change, each given only when its Has flag is set. Summaries and
// Tradeoffs are LABEL:TEXT as typed, split at the first colon; an empty TEXT
// clears the summary or tradeoffs. Title, Context and Rationale are kept as
// typed and must not be blank.
type AmendInput struct {
	Title        string
	HasTitle     bool
	Context      string
	HasContext   bool
	Rationale    string
	HasRationale bool
	Summaries    []string
	Tradeoffs    []string
}

// labeled parses LABEL:TEXT values into a map by label, trimmed as ask does. A
// label given twice is an error.
func labeled(flagName string, values []string) (map[string]string, error) {
	var out map[string]string
	for _, v := range values {
		label, text, err := splitLabeled(flagName, v, "LABEL:TEXT")
		if err != nil {
			return nil, err
		}
		if _, dup := out[label]; dup {
			return nil, usagef("--%s given twice for option %q", flagName, label)
		}
		if out == nil {
			out = map[string]string{}
		}
		out[label] = text
	}
	return out, nil
}

// amendment turns the input into the model's Amendment, or returns a
// *UsageError that says what is wrong. Nothing is read or written.
func (in AmendInput) amendment() (model.Amendment, error) {
	var a model.Amendment
	if !in.HasTitle && !in.HasContext && !in.HasRationale && len(in.Summaries) == 0 && len(in.Tradeoffs) == 0 {
		return a, usagef("give at least one of --title, --context (or --stdin), --rationale, --summary, --tradeoffs")
	}
	if in.HasTitle {
		switch {
		case strings.TrimSpace(in.Title) == "":
			return a, usagef("--title must not be blank")
		case strings.ContainsAny(in.Title, "\r\n"):
			return a, usagef("--title must be one line")
		}
		a.Title = &in.Title
	}
	if in.HasContext {
		if strings.TrimSpace(in.Context) == "" {
			return a, usagef("--context must not be blank")
		}
		a.Context = &in.Context
	}
	if in.HasRationale {
		if strings.TrimSpace(in.Rationale) == "" {
			return a, usagef("--rationale must not be blank")
		}
		a.Rationale = &in.Rationale
	}
	var err error
	if a.Summaries, err = labeled("summary", in.Summaries); err != nil {
		return a, err
	}
	if a.Tradeoffs, err = labeled("tradeoffs", in.Tradeoffs); err != nil {
		return a, err
	}
	return a, nil
}

// Check reports a *UsageError if the input cannot amend any record: nothing
// to change, a blank title, context or rationale, a title on several lines, or
// a malformed or repeated LABEL:TEXT. Whether a label is one of the record's
// options needs the record, so Amend checks that. The CLI runs Check first, so
// bad arguments are reported before the store is looked for.
func (in AmendInput) Check() error {
	_, err := in.amendment()
	return err
}

// noChangeError is an amend refused because every text it was given is what
// the record already says. Unwrap keeps the model's error.
type noChangeError struct{ id int }

func (e *noChangeError) Error() string {
	return fmt.Sprintf("record %d: nothing to change: every text given is what the record already says", e.id)
}

func (e *noChangeError) Unwrap() error { return model.ErrNoChange }

// openRationaleError is an amend refused because it names a rationale and the
// record is open, so it has none. Unwrap keeps the model's error.
type openRationaleError struct{ id int }

func (e *openRationaleError) Error() string {
	return fmt.Sprintf("record %d is open and has no rationale to amend", e.id)
}

func (e *openRationaleError) Unwrap() error { return model.ErrInvalidTransition }

// Amend corrects the text of the record with id, dated now (C166). It works on
// a record in any status and changes only the title, context, option summaries
// and tradeoffs and rationale that in names, and appends one amend history entry
// that keeps the old text of every field it changed. It returns the record as
// stored. Bad input, or a label that is not one of the record's options, is a
// *UsageError. A missing record, a rationale on an open record and an amend
// that changes nothing are ordinary errors. Nothing is written on error.
func Amend(st *store.Store, id int, in AmendInput, now time.Time) (model.Record, error) {
	am, err := in.amendment()
	if err != nil {
		return model.Record{}, err
	}
	rec, err := st.Get(id)
	if err != nil {
		return model.Record{}, err
	}
	for _, c := range []struct {
		flagName string
		texts    map[string]string
	}{{"summary", am.Summaries}, {"tradeoffs", am.Tradeoffs}} {
		for _, label := range slices.Sorted(maps.Keys(c.texts)) {
			if !hasOption(rec, label) {
				return model.Record{}, usagef("--%s: record %d has no option %q (only the text of an option can be amended, not its label)", c.flagName, id, label)
			}
		}
	}
	if err := model.Amend(&rec, am, now.Format(time.DateOnly)); err != nil {
		switch {
		case errors.Is(err, model.ErrNoChange):
			return model.Record{}, &noChangeError{id}
		case errors.Is(err, model.ErrInvalidTransition):
			return model.Record{}, &openRationaleError{id}
		}
		return model.Record{}, err
	}
	if err := st.Update(rec); err != nil {
		return model.Record{}, err
	}
	return rec, nil
}
