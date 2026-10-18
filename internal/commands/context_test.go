// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"fmt"
	"math/bits"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

func TestContextInputCheckMaxBytes(t *testing.T) {
	tests := []struct {
		name string
		in   ContextInput
		bad  bool
	}{
		{name: "no budget", in: ContextInput{}},
		{name: "one byte", in: ContextInput{MaxBytes: 1, HasMaxBytes: true}},
		{name: "zero", in: ContextInput{MaxBytes: 0, HasMaxBytes: true}, bad: true},
		{name: "negative", in: ContextInput{MaxBytes: -5, HasMaxBytes: true}, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Check()
			var ue *UsageError
			if tt.bad != errors.As(err, &ue) {
				t.Fatalf("Check() = %v, want a usage error: %v", err, tt.bad)
			}
			if tt.bad && !strings.Contains(err.Error(), "--max-bytes") {
				t.Errorf("error %q does not name --max-bytes", err)
			}
		})
	}
}

// decidedOn is a decided record with only what FitContext looks at.
func decidedOn(id int, date string) model.Record {
	return model.Record{ID: id, Title: "T", Status: model.StatusDecided, DecidedAt: date}
}

func ids(recs []model.Record) []int {
	out := []int{}
	for _, r := range recs {
		out = append(out, r.ID)
	}
	return out
}

// fakeSize charges 10 bytes per kept record and 7 for the omitted line.
func fakeSize(kept []model.Record, omitted int) (int, error) {
	n := 10 * len(kept)
	if omitted > 0 {
		n += 7
	}
	return n, nil
}

func TestFitContext(t *testing.T) {
	// Newest first: 3 (03-20), then 5 and 1 (03-10, the higher ID first),
	// then 4 (03-05), then 2 (03-01).
	recs := []model.Record{
		decidedOn(1, "2026-03-10"),
		decidedOn(2, "2026-03-01"),
		decidedOn(3, "2026-03-20"),
		decidedOn(4, "2026-03-05"),
		decidedOn(5, "2026-03-10"),
	}
	tests := []struct {
		name        string
		max         int
		wantIDs     []int
		wantOmitted int
	}{
		{name: "everything fits", max: 100, wantIDs: []int{1, 2, 3, 4, 5}},
		{name: "everything fits exactly", max: 50, wantIDs: []int{1, 2, 3, 4, 5}},
		{name: "one byte short of everything", max: 49, wantIDs: []int{1, 3, 4, 5}, wantOmitted: 1},
		{name: "the omitted line counts", max: 40, wantIDs: []int{1, 3, 5}, wantOmitted: 2},
		{name: "decided_at decides, not the ID", max: 30, wantIDs: []int{3, 5}, wantOmitted: 3},
		{name: "a tie in decided_at goes to the higher ID", max: 17, wantIDs: []int{3}, wantOmitted: 4},
		{name: "nothing fits but the omitted line", max: 16, wantIDs: []int{}, wantOmitted: 5},
		{name: "exactly the omitted line", max: 7, wantIDs: []int{}, wantOmitted: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := ids(recs)
			kept, omitted, err := FitContext(recs, tt.max, fakeSize)
			if err != nil {
				t.Fatal(err)
			}
			if got := ids(kept); !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("kept %v, want %v", got, tt.wantIDs)
			}
			if omitted != tt.wantOmitted {
				t.Errorf("omitted %d, want %d", omitted, tt.wantOmitted)
			}
			if after := ids(recs); !reflect.DeepEqual(before, after) {
				t.Errorf("FitContext reordered its input: %v, was %v", after, before)
			}
		})
	}
}

func TestFitContextNothingFitsIsAnError(t *testing.T) {
	recs := []model.Record{decidedOn(1, "2026-03-10"), decidedOn(2, "2026-03-11")}
	_, _, err := FitContext(recs, 6, fakeSize)
	if err == nil {
		t.Fatal("FitContext with a budget below the omitted line: no error")
	}
	var ue *UsageError
	if errors.As(err, &ue) {
		t.Errorf("error %q is a usage error; want a failed operation (exit 1)", err)
	}
	if !strings.Contains(err.Error(), "--max-bytes 6") || !strings.Contains(err.Error(), "7") {
		t.Errorf("error %q does not name the budget and the smallest size", err)
	}
	// No records and a budget that fits nothing at all is fine when the output
	// is empty.
	zero := func([]model.Record, int) (int, error) { return 0, nil }
	if kept, omitted, err := FitContext(nil, 1, zero); err != nil || len(kept) != 0 || omitted != 0 {
		t.Errorf("FitContext(nil) = %v, %d, %v; want nothing and no error", kept, omitted, err)
	}
}

func TestFitContextMeasureError(t *testing.T) {
	boom := errors.New("boom")
	fail := func([]model.Record, int) (int, error) { return 0, boom }
	if _, _, err := FitContext([]model.Record{decidedOn(1, "2026-03-10")}, 100, fail); !errors.Is(err, boom) {
		t.Errorf("err = %v, want the measure's error", err)
	}
}

// textSize is the size of the md output for the records, as the renderer
// writes it (entries of about 30 bytes and more, then the omitted line).
func textSize(kept []model.Record, omitted int) (int, error) {
	n := 0
	for _, r := range kept {
		n += len(fmt.Sprintf("- %d %s: %s\n  Rationale: why\n", r.ID, r.Title, r.Decision))
	}
	if omitted > 0 {
		if len(kept) > 0 {
			n++
		}
		noun := "decisions"
		if omitted == 1 {
			noun = "decision"
		}
		n += len(fmt.Sprintf("%d %s omitted\n", omitted, noun))
	}
	return n, nil
}

// manyRecords makes n decided records with seeded dates (many ties), titles
// and decisions of varying length.
func manyRecords(rng *rand.Rand, n int) []model.Record {
	recs := make([]model.Record, n)
	for i := range recs {
		recs[i] = model.Record{
			ID: i + 1, Status: model.StatusDecided,
			Title:     strings.Repeat("t", 1+rng.Intn(40)),
			Decision:  strings.Repeat("d", 1+rng.Intn(60)),
			DecidedAt: fmt.Sprintf("2026-%02d-%02d", 1+rng.Intn(3), 1+rng.Intn(5)),
		}
	}
	return recs
}

// bruteForce is the definition: the k newest for the largest k that fits, by
// measuring every k.
func bruteForce(t *testing.T, recs []model.Record, max int) (keptIDs []int, omitted int, ok bool) {
	t.Helper()
	newest := append([]model.Record(nil), recs...)
	sort.SliceStable(newest, func(i, j int) bool {
		if newest[i].DecidedAt != newest[j].DecidedAt {
			return newest[i].DecidedAt > newest[j].DecidedAt
		}
		return newest[i].ID > newest[j].ID
	})
	for k := len(newest); k >= 0; k-- {
		kept := append([]model.Record(nil), newest[:k]...)
		size, _ := textSize(kept, len(newest)-k)
		if size <= max {
			ids := ids(kept)
			sort.Ints(ids)
			return ids, len(newest) - k, true
		}
	}
	return nil, 0, false
}

// FitContext finds the largest k without measuring every k; its answer is the
// brute-force one, for every budget from nothing to everything.
func TestFitContextMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewSource(35))
	for _, n := range []int{0, 1, 2, 3, 7, 50, 300} {
		recs := manyRecords(rng, n)
		full, _ := textSize(recs, 0)
		step := 1
		if full > 600 {
			step = full / 120
		}
		for max := 0; max <= full+3; max += step {
			wantIDs, wantOmitted, wantOK := bruteForce(t, recs, max)
			kept, omitted, err := FitContext(recs, max, textSize)
			if (err == nil) != wantOK {
				t.Fatalf("n=%d max=%d: err = %v, brute force fits: %v", n, max, err, wantOK)
			}
			if !wantOK {
				continue
			}
			got := ids(kept)
			if !sort.IntsAreSorted(got) || !reflect.DeepEqual(got, wantIDs) || omitted != wantOmitted {
				t.Fatalf("n=%d max=%d: kept %v omitted %d, brute force %v omitted %d", n, max, got, omitted, wantIDs, wantOmitted)
			}
		}
	}
}

// A budget that leaves most records out takes a few measurements, not one per
// record: the size grows with k, so FitContext halves the range.
func TestFitContextMeasuresLogarithmically(t *testing.T) {
	recs := manyRecords(rand.New(rand.NewSource(7)), 500)
	for _, max := range []int{40, 400, 2000, 9000} {
		calls := 0
		counting := func(kept []model.Record, omitted int) (int, error) {
			calls++
			return textSize(kept, omitted)
		}
		if _, _, err := FitContext(recs, max, counting); err != nil {
			t.Fatal(err)
		}
		// One call for "everything fits", then a binary search over 500.
		if limit := 2 + bits.Len(uint(len(recs))); calls > limit {
			t.Errorf("max=%d: %d measurements for %d records, want at most %d", max, calls, len(recs), limit)
		}
	}
}
