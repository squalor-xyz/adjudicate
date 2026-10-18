// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/query"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// ListInput is the raw input of adj list: the filters as typed. HasStatus and
// HasKey say whether the flag was given, so an explicit empty value is refused
// rather than read as no filter (C83).
type ListInput struct {
	Status    string
	HasStatus bool
	Tags      []string
	Refs      []string
	Key       string
	HasKey    bool
}

// Check reports a *UsageError if a filter cannot be used: a status that is not
// one of the five, or a blank tag, ref or key (C91). The CLI runs it before the
// store is looked for.
func (in ListInput) Check() error {
	if in.HasStatus && !model.Status(strings.TrimSpace(in.Status)).Valid() {
		return usagef("--status must be one of open, decided, rejected, withdrawn or superseded; got %q", in.Status)
	}
	for _, tag := range in.Tags {
		if strings.TrimSpace(tag) == "" {
			return usagef("--tag must not be blank")
		}
	}
	for _, ref := range in.Refs {
		if strings.TrimSpace(ref) == "" {
			return usagef("--ref must not be blank")
		}
	}
	if in.HasKey && strings.TrimSpace(in.Key) == "" {
		return usagef("--key must not be blank")
	}
	return nil
}

// List returns the records of st that meet every filter in in, in ID order.
// Tags and status are trimmed as ask and decide trim their values; the key is
// matched as stored (C91). Refs match exactly as typed. Bad input is a
// *UsageError and nothing is read.
func List(st *store.Store, in ListInput) ([]model.Record, error) {
	if err := in.Check(); err != nil {
		return nil, err
	}
	recs, err := st.List()
	if err != nil {
		return nil, err
	}
	f := query.Filter{Refs: in.Refs}
	if in.HasStatus {
		f.Status = model.Status(strings.TrimSpace(in.Status))
	}
	for _, tag := range in.Tags {
		f.Tags = append(f.Tags, strings.TrimSpace(tag))
	}
	if in.HasKey {
		f.Key = in.Key
	}
	return query.Select(recs, f), nil
}
