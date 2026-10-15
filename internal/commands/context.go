// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/query"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// ContextInput is the raw input of adj context: the filters as typed.
// HasKeyPrefix says whether the flag was given, so an explicit empty value is
// refused rather than read as no filter (C83).
type ContextInput struct {
	Tags         []string
	KeyPrefix    string
	HasKeyPrefix bool
}

// Check reports a *UsageError if a filter cannot be used: a blank tag or a
// blank key prefix (C91). The CLI runs it before the store is looked for.
func (in ContextInput) Check() error {
	for _, tag := range in.Tags {
		if strings.TrimSpace(tag) == "" {
			return usagef("--tag must not be blank")
		}
	}
	if in.HasKeyPrefix && strings.TrimSpace(in.KeyPrefix) == "" {
		return usagef("--key-prefix must not be blank")
	}
	return nil
}

// Context returns the decided records of st that meet every filter in in, in
// ID order. Only status decided counts: open, rejected, withdrawn and
// superseded records are not context. Tags are trimmed as in List; the key
// prefix is matched as typed against the stored key. Bad input is a
// *UsageError and nothing is read.
func Context(st *store.Store, in ContextInput) ([]model.Record, error) {
	if err := in.Check(); err != nil {
		return nil, err
	}
	recs, err := st.List()
	if err != nil {
		return nil, err
	}
	f := query.Filter{Status: model.StatusDecided}
	for _, tag := range in.Tags {
		f.Tags = append(f.Tags, strings.TrimSpace(tag))
	}
	if in.HasKeyPrefix {
		f.KeyPrefix = in.KeyPrefix
	}
	return query.Select(recs, f), nil
}
