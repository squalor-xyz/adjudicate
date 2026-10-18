// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// budgetRec is one decided record of seedBudget as adj context prints it.
type budgetRec struct {
	id    int
	title string
	key   string
}

// The four decided records of seedBudget, by ID. They are decided on
// different days, so the newest first order (budgetNewest) is not the ID
// order, and 1 and 4 share a day, so the tie goes to the higher ID:
//
//	1 Alpha  a.one    tag x  decided 2026-03-02
//	2 Bravo  b.two    tag x  decided 2026-02-20
//	3 Gamma  a.three  tag y  decided 2026-03-03
//	4 Delta  b.four   tag x  decided 2026-03-02
var (
	budgetRecs = map[int]budgetRec{
		1: {1, "Alpha", "a.one"},
		2: {2, "Bravo", "b.two"},
		3: {3, "Gamma", "a.three"},
		4: {4, "Delta", "b.four"},
	}
	budgetNewest = []int{3, 4, 1, 2}
)

func seedBudget(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	steps := [][]string{
		{"ask", "Alpha", "--key", "a.one", "--tag", "x"},
		{"ask", "Bravo", "--key", "b.two", "--tag", "x"},
		{"ask", "Gamma", "--key", "a.three", "--tag", "y"},
		{"ask", "Delta", "--key", "b.four", "--tag", "x"},
		{"decide", "1", "--text", "yes", "--why", "why", "--decided-at", "2026-03-02"},
		{"decide", "2", "--text", "yes", "--why", "why", "--decided-at", "2026-02-20"},
		{"decide", "3", "--text", "yes", "--why", "why", "--decided-at", "2026-03-03"},
		{"decide", "4", "--text", "yes", "--why", "why", "--decided-at", "2026-03-02"},
	}
	for _, args := range steps {
		if r := run(testEnv(root), "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	return root
}

// mdWant is the text output for the kept IDs (in ID order) and the number of
// records the budget left out, written out by hand.
func mdWant(kept []int, omitted int) string {
	ids := append([]int(nil), kept...)
	sort.Ints(ids)
	var b strings.Builder
	for _, id := range ids {
		r := budgetRecs[id]
		fmt.Fprintf(&b, "- %d %s [%s]: yes\n  Rationale: why\n", r.id, r.title, r.key)
	}
	if omitted > 0 {
		if len(ids) > 0 {
			b.WriteString("\n")
		}
		noun := "decisions"
		if omitted == 1 {
			noun = "decision"
		}
		fmt.Fprintf(&b, "%d %s omitted\n", omitted, noun)
	}
	return b.String()
}

// jsonWant is the JSON output with the omitted key, written out by hand.
func jsonWant(kept []int, omitted int) string {
	ids := append([]int(nil), kept...)
	sort.Ints(ids)
	var entries []string
	for _, id := range ids {
		r := budgetRecs[id]
		entries = append(entries, fmt.Sprintf("    {\n      \"id\": %d,\n      \"title\": %q,\n      \"key\": %q,\n      \"decision\": \"yes\",\n      \"rationale\": \"why\"\n    }", r.id, r.title, r.key))
	}
	list := "[]"
	if len(entries) > 0 {
		list = "[\n" + strings.Join(entries, ",\n") + "\n  ]"
	}
	return fmt.Sprintf("{\n  \"decisions\": %s,\n  \"omitted\": %d\n}\n", list, omitted)
}

// With a budget the newest decisions are kept, in ID order, and the output is
// at most N bytes, the omitted line included. At each size of the table the
// output is exactly the one that the table lists, and one byte less gives the
// next row down.
func TestContextMaxBytesKeepsTheNewestInIDOrder(t *testing.T) {
	root := seedBudget(t)
	before := snapshot(t, root)
	formats := []struct {
		name string
		args []string
		want func(kept []int, omitted int) string
	}{
		{"md", []string{"context"}, mdWant},
		{"json", []string{"context", "--format", "json"}, jsonWant},
	}
	for _, f := range formats {
		t.Run(f.name, func(t *testing.T) {
			// Row k keeps the k newest records.
			var sizes []int
			for k := 0; k <= len(budgetNewest); k++ {
				sizes = append(sizes, len(f.want(budgetNewest[:k], len(budgetNewest)-k)))
			}
			for k := 1; k < len(sizes); k++ {
				if sizes[k] <= sizes[k-1] {
					t.Fatalf("sizes of rows %v are not increasing; the table below would be ambiguous", sizes)
				}
			}
			for k := 0; k <= len(budgetNewest); k++ {
				kept, omitted := budgetNewest[:k], len(budgetNewest)-k
				want := f.want(kept, omitted)
				for _, n := range []int{sizes[k], sizes[k] + 1} {
					if k < len(budgetNewest) && n >= sizes[k+1] {
						continue
					}
					r := run(testEnv(root), "", append(append([]string{}, f.args...), "--max-bytes", strconv.Itoa(n))...)
					if r.code != 0 || r.stderr != "" {
						t.Fatalf("k=%d N=%d: exit %d, stderr %q; want exit 0 and no stderr", k, n, r.code, r.stderr)
					}
					if r.stdout != want {
						t.Errorf("k=%d N=%d: stdout =\n%s\nwant\n%s", k, n, r.stdout, want)
					}
					if len(r.stdout) > n {
						t.Errorf("k=%d N=%d: %d bytes, more than the budget", k, n, len(r.stdout))
					}
				}
			}
			// A budget well above the full output changes nothing but the
			// omitted key of the JSON.
			r := run(testEnv(root), "", append(append([]string{}, f.args...), "--max-bytes", "100000")...)
			if want := f.want(budgetNewest, 0); r.code != 0 || r.stdout != want {
				t.Errorf("big budget: exit %d, stdout =\n%s\nwant\n%s", r.code, r.stdout, want)
			}
		})
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Error("context --max-bytes changed the repo")
	}
}

// A budget that fits all records: md has no omitted line and is the output
// without a budget, byte for byte; the JSON adds omitted: 0 and nothing else.
func TestContextMaxBytesThatFitsEverything(t *testing.T) {
	root := seedBudget(t)
	plain := run(testEnv(root), "", "context")
	if plain.stdout != mdWant(budgetNewest, 0) {
		t.Fatalf("context without a budget =\n%s\nwant\n%s", plain.stdout, mdWant(budgetNewest, 0))
	}
	n := strconv.Itoa(len(plain.stdout))
	if r := run(testEnv(root), "", "context", "--max-bytes", n); r.code != 0 || r.stdout != plain.stdout {
		t.Errorf("md --max-bytes %s: exit %d, stdout =\n%s\nwant the output without a budget", n, r.code, r.stdout)
	}

	// Space around the number is trimmed.
	if r := run(testEnv(root), "", "context", "--max-bytes", " "+n+" "); r.code != 0 || r.stdout != plain.stdout {
		t.Errorf("md --max-bytes %q: exit %d, stdout =\n%s\nwant the output without a budget", " "+n+" ", r.code, r.stdout)
	}

	plainJSON := run(testEnv(root), "", "context", "--format", "json")
	if strings.Contains(plainJSON.stdout, "omitted") {
		t.Errorf("json without a budget has an omitted key:\n%s", plainJSON.stdout)
	}
	withKey := strings.Replace(plainJSON.stdout, "\n  ]\n}\n", "\n  ],\n  \"omitted\": 0\n}\n", 1)
	r := run(testEnv(root), "", "context", "--format", "json", "--max-bytes", strconv.Itoa(len(withKey)))
	if r.code != 0 || r.stdout != withKey {
		t.Errorf("json --max-bytes %d: exit %d, stdout =\n%s\nwant\n%s", len(withKey), r.code, r.stdout, withKey)
	}
	// One byte less leaves the oldest record out.
	r = run(testEnv(root), "", "context", "--format", "json", "--max-bytes", strconv.Itoa(len(withKey)-1))
	if want := jsonWant([]int{3, 4, 1}, 1); r.code != 0 || r.stdout != want {
		t.Errorf("json, one byte short: exit %d, stdout =\n%s\nwant\n%s", r.code, r.stdout, want)
	}
}

// The budget is applied after the filters: records a filter leaves out are
// not counted as omitted.
func TestContextMaxBytesWorksWithFilters(t *testing.T) {
	root := seedBudget(t)
	tests := []struct {
		name   string
		args   []string
		newest []int // the records the filters leave, newest first
	}{
		{name: "tag", args: []string{"--tag", "x"}, newest: []int{4, 1, 2}},
		{name: "key prefix", args: []string{"--key-prefix", "a."}, newest: []int{3, 1}},
		{name: "tag and key prefix", args: []string{"--tag", "x", "--key-prefix", "b."}, newest: []int{4, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k := 0; k < len(tt.newest); k++ {
				want := mdWant(tt.newest[:k], len(tt.newest)-k)
				args := append(append([]string{"context"}, tt.args...), "--max-bytes", strconv.Itoa(len(want)))
				r := run(testEnv(root), "", args...)
				if r.code != 0 || r.stdout != want {
					t.Errorf("%v: exit %d, stdout =\n%s\nwant\n%s", args, r.code, r.stdout, want)
				}
				jwant := jsonWant(tt.newest[:k], len(tt.newest)-k)
				args = append(append([]string{"context", "--format", "json"}, tt.args...), "--max-bytes", strconv.Itoa(len(jwant)))
				r = run(testEnv(root), "", args...)
				if r.code != 0 || r.stdout != jwant {
					t.Errorf("%v: exit %d, stdout =\n%s\nwant\n%s", args, r.code, r.stdout, jwant)
				}
			}
		})
	}
}

// A budget below the smallest output is a failed operation: nothing on stdout,
// exit 1, and an error that says how many bytes are needed. The smallest
// output is the omitted line alone (md), or the JSON with no decisions.
func TestContextMaxBytesBelowTheOmittedLine(t *testing.T) {
	root := seedBudget(t)
	for _, tt := range []struct {
		name string
		args []string
		line string
	}{
		{"md", []string{"context"}, mdWant(nil, 4)},
		{"json", []string{"context", "--format", "json"}, jsonWant(nil, 4)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := run(testEnv(root), "", append(append([]string{}, tt.args...), "--max-bytes", strconv.Itoa(len(tt.line)))...)
			if r.code != 0 || r.stdout != tt.line || r.stderr != "" {
				t.Errorf("exactly the omitted output: exit %d, stdout %q, stderr %q; want exit 0 and %q", r.code, r.stdout, r.stderr, tt.line)
			}
			n := len(tt.line) - 1
			r = run(testEnv(root), "", append(append([]string{}, tt.args...), "--max-bytes", strconv.Itoa(n))...)
			if r.code != 1 || r.stdout != "" {
				t.Errorf("one byte less: exit %d, stdout %q; want exit 1 and no stdout", r.code, r.stdout)
			}
			if !strings.Contains(r.stderr, "--max-bytes "+strconv.Itoa(n)) || !strings.Contains(r.stderr, strconv.Itoa(len(tt.line))) {
				t.Errorf("stderr = %q; want it to name the budget %d and the %d bytes needed", r.stderr, n, len(tt.line))
			}
			if strings.Contains(r.stderr, "usage: adj context") {
				t.Errorf("a failed budget is not a usage error: %q", r.stderr)
			}
		})
	}
}

// No decision at all: nothing is omitted, md prints nothing even for one byte,
// and the JSON is the empty list with omitted 0.
func TestContextMaxBytesWithNoDecisions(t *testing.T) {
	root := seedBudget(t)
	r := run(testEnv(root), "", "context", "--tag", "legal", "--max-bytes", "1")
	if r.code != 0 || r.stdout != "" || r.stderr != "" {
		t.Errorf("md: exit %d, stdout %q, stderr %q; want exit 0 and no output", r.code, r.stdout, r.stderr)
	}
	want := jsonWant(nil, 0)
	r = run(testEnv(root), "", "context", "--tag", "legal", "--format", "json", "--max-bytes", strconv.Itoa(len(want)))
	if r.code != 0 || r.stdout != want {
		t.Errorf("json: exit %d, stdout %q; want exit 0 and %q", r.code, r.stdout, want)
	}
	r = run(testEnv(root), "", "context", "--tag", "legal", "--format", "json", "--max-bytes", strconv.Itoa(len(want)-1))
	if r.code != 1 || r.stdout != "" {
		t.Errorf("json, one byte short: exit %d, stdout %q; want exit 1 and no stdout", r.code, r.stdout)
	}
}

// For every budget the output is within it, holds the newest k records for the
// largest k that fits, and says how many are left out.
func TestContextMaxBytesEveryBudget(t *testing.T) {
	root := seedBudget(t)
	full := len(jsonWant(budgetNewest, 0))
	lastKept := -1
	for n := 1; n <= full+2; n++ {
		r := run(testEnv(root), "", "context", "--format", "json", "--max-bytes", strconv.Itoa(n))
		if r.code == 1 && r.stdout == "" {
			if lastKept != -1 {
				t.Errorf("N=%d fails after a smaller budget worked", n)
			}
			continue
		}
		if r.code != 0 || len(r.stdout) > n {
			t.Fatalf("N=%d: exit %d, %d bytes; want exit 0 within the budget", n, r.code, len(r.stdout))
		}
		var out struct {
			Decisions []struct {
				ID int `json:"id"`
			} `json:"decisions"`
			Omitted *int `json:"omitted"`
		}
		if err := json.Unmarshal([]byte(r.stdout), &out); err != nil || out.Omitted == nil {
			t.Fatalf("N=%d: not the decisions object with omitted: %v\n%s", n, err, r.stdout)
		}
		k := len(out.Decisions)
		if k+*out.Omitted != len(budgetNewest) || k < lastKept {
			t.Errorf("N=%d: kept %d, omitted %d (before: kept %d)", n, k, *out.Omitted, lastKept)
		}
		var got, want []int
		for _, d := range out.Decisions {
			got = append(got, d.ID)
		}
		want = append(want, budgetNewest[:k]...)
		sort.Ints(want)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("N=%d: kept %v, want the newest %d in ID order: %v", n, got, k, want)
		}
		lastKept = k
	}
	if lastKept != len(budgetNewest) {
		t.Errorf("the largest budget keeps %d records, want all %d", lastKept, len(budgetNewest))
	}
}

func TestContextMaxBytesBadValuesAreUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"zero", []string{"--max-bytes", "0"}},
		{"negative", []string{"--max-bytes", "-1"}},
		{"not a number", []string{"--max-bytes", "abc"}},
		{"empty", []string{"--max-bytes", ""}},
		{"blank", []string{"--max-bytes", "  "}},
		{"a fraction", []string{"--max-bytes", "1.5"}},
		{"too big for an int", []string{"--max-bytes", "99999999999999999999"}},
		{"no value", []string{"--max-bytes"}},
		{"equals form", []string{"--max-bytes=0"}},
		{"valid then invalid, the last counts", []string{"--max-bytes", "500", "--max-bytes", "0"}},
		{"with json", []string{"--format", "json", "--max-bytes", "abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// No repo: the value is refused before the store is looked for.
			r := run(testEnv(t.TempDir()), "", append([]string{"context"}, tt.args...)...)
			if r.code != 2 || r.stdout != "" {
				t.Errorf("exit %d, stdout %q; want exit 2 and no stdout", r.code, r.stdout)
			}
			if !strings.Contains(r.stderr, "--max-bytes") || !strings.Contains(r.stderr, "usage: adj context") {
				t.Errorf("stderr = %q; want it to name --max-bytes and show the context usage", r.stderr)
			}
		})
	}
	// Twice, the last value counts, as for --root.
	root := seedBudget(t)
	if r := run(testEnv(root), "", "context", "--max-bytes", "0", "--max-bytes", "500"); r.code != 0 || r.stdout != mdWant(budgetNewest, 0) {
		t.Errorf("invalid then valid: exit %d, stdout %q; want exit 0 and the full output", r.code, r.stdout)
	}
}

func TestContextUsageNamesMaxBytes(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "context", "--help")
	for _, want := range []string{"\n  --max-bytes N ", "newest", "omitted"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("context usage does not mention %q:\n%s", want, r.stdout)
		}
	}
}
