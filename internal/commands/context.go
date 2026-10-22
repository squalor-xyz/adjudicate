// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/query"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// ContextInput is the raw input of adj context: the filters as typed.
// HasKeyPrefix says whether the flag was given, so an explicit empty value is
// refused rather than read as no filter (C83). MaxBytes is the size budget of
// --max-bytes, used only when HasMaxBytes is set.
type ContextInput struct {
	Tags         []string
	Refs         []string
	KeyPrefix    string
	HasKeyPrefix bool
	MaxBytes     int
	HasMaxBytes  bool
}

// Check reports a *UsageError if a filter cannot be used: a blank tag, ref or
// key prefix (C91), or a budget below 1 byte. The CLI runs it before the store
// is looked for.
func (in ContextInput) Check() error {
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
	if in.HasKeyPrefix && strings.TrimSpace(in.KeyPrefix) == "" {
		return usagef("--key-prefix must not be blank")
	}
	if in.HasMaxBytes && in.MaxBytes < 1 {
		return usagef("--max-bytes must be a whole number of at least 1; got %d", in.MaxBytes)
	}
	return nil
}

// Context returns the decided records of st that meet every filter in in, in
// ID order. Only status decided counts: open, rejected, withdrawn and
// superseded records are not context. Tags are trimmed as in List; the key
// prefix is matched as typed against the stored key. Refs match exactly as
// typed. Bad input is a *UsageError and nothing is read.
func Context(st *store.Store, in ContextInput) ([]model.Record, error) {
	if err := in.Check(); err != nil {
		return nil, err
	}
	recs, err := st.List()
	if err != nil {
		return nil, err
	}
	f := query.Filter{Status: model.StatusDecided, Refs: in.Refs}
	for _, tag := range in.Tags {
		f.Tags = append(f.Tags, strings.TrimSpace(tag))
	}
	if in.HasKeyPrefix {
		f.KeyPrefix = in.KeyPrefix
	}
	return query.Select(recs, f), nil
}

// ContextMeasure returns the number of bytes the output has for the records
// kept (in ID order) and the number of records left out. The CLI builds it from
// the renderer of the chosen format, so the budget counts the real output.
type ContextMeasure func(kept []model.Record, omitted int) (int, error)

// FitContext chooses the records of recs (in ID order, as Context returns
// them) that a budget of max bytes allows (S35, C32). The newest decisions
// are kept: decided_at, latest first, then the higher ID first. Of that order
// the kept records are the first k, with k the largest for which measure(kept,
// omitted) is at most max, so an older record never stands in for a newer one
// that does not fit. kept is in ID order and omitted is the number
// left out. If even k = 0 does not fit, nothing can be printed within max and
// the error (not a usage error) names the budget and the smallest size. recs
// is not changed.
//
// If everything fits, that is the answer (one measurement). Otherwise some
// record is omitted for every k below len(recs), and in that range the output
// grows with k: one more entry adds more bytes (an entry is at least nine) than
// the omitted line can lose (one digit when the count drops from 10 to 9, one
// byte when "decisions" becomes "decision"; the blank line before it is only
// added). So the k that fit are a prefix 0..K of the range and a binary search
// finds K in about log2(n) measurements instead of n, which matters for a log
// of thousands of decisions. The k = len(recs) case is left out of that range
// on purpose: dropping the omitted line there can shrink the output.
func FitContext(recs []model.Record, max int, measure ContextMeasure) (kept []model.Record, omitted int, err error) {
	newest := append([]model.Record(nil), recs...)
	sort.SliceStable(newest, func(i, j int) bool {
		if newest[i].DecidedAt != newest[j].DecidedAt {
			return newest[i].DecidedAt > newest[j].DecidedAt
		}
		return newest[i].ID > newest[j].ID
	})
	n := len(newest)
	// try measures the k newest records, kept in ID order.
	try := func(k int) (kept []model.Record, size int, err error) {
		kept = append([]model.Record(nil), newest[:k]...)
		sort.SliceStable(kept, func(i, j int) bool { return kept[i].ID < kept[j].ID })
		size, err = measure(kept, n-k)
		return kept, size, err
	}
	kept, size, err := try(n)
	if err != nil {
		return nil, 0, err
	}
	if size <= max {
		return kept, 0, nil
	}
	smallest := size
	best, lo, hi := -1, 0, n-1
	var bestKept []model.Record
	for lo <= hi {
		mid := lo + (hi-lo)/2
		kept, size, err := try(mid)
		if err != nil {
			return nil, 0, err
		}
		if size <= max {
			best, bestKept, lo = mid, kept, mid+1
		} else {
			if size < smallest {
				smallest = size
			}
			hi = mid - 1
		}
	}
	if best < 0 {
		return nil, 0, &budgetError{max: max, smallest: smallest}
	}
	return bestKept, n - best, nil
}

// ErrBudget matches the error FitContext returns when not even the shortest
// output fits the budget (S42: the CLI gives it the code budget).
var ErrBudget = errors.New("--max-bytes is too small")

// budgetError is that error with its numbers; its text names both.
type budgetError struct{ max, smallest int }

func (e *budgetError) Error() string {
	return fmt.Sprintf("--max-bytes %d is too small: the shortest output is %d bytes", e.max, e.smallest)
}

func (e *budgetError) Is(target error) bool { return target == ErrBudget }
