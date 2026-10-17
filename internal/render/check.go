// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/query"
)

// CheckText writes the result of adj check: the outcome (decided, open, closed
// or not found) on the first line, then one line per matching record, "ID
// status  title": decided records first, then open, then closed ones. A decided
// record is followed by its decision and rationale, indented. A closed record
// (rejected, withdrawn or superseded) is followed by its rationale, so the
// reason it was closed is in the answer; a superseded one has the ID of the
// record that replaced it first. Each matching record shows its refs when set
// (C171); adj show prints the other fields.
func CheckText(w io.Writer, res query.Result) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", res.Outcome)
	for _, r := range res.Decided {
		checkLine(&b, r)
		writeField(&b, "  ", "Decision", r.Decision)
		writeField(&b, "  ", "Rationale", r.Rationale)
	}
	for _, r := range res.Open {
		checkLine(&b, r)
	}
	for _, r := range res.Closed {
		checkLine(&b, r)
		if r.SupersededBy != 0 {
			writeField(&b, "  ", "Superseded by", strconv.Itoa(r.SupersededBy))
		}
		writeField(&b, "  ", "Rationale", r.Rationale)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func checkLine(b *strings.Builder, r model.Record) {
	fmt.Fprintf(b, "%d  %s  %s\n", r.ID, r.Status, r.Title)
	writeField(b, "  ", "Refs", strings.Join(r.Refs, ", "))
}

// checkJSON is the object adj check --json prints.
type checkJSON struct {
	Query   string       `json:"query"`
	Result  string       `json:"result"`
	Decided []jsonRecord `json:"decided"`
	Open    []jsonRecord `json:"open"`
	Closed  []jsonRecord `json:"closed"`
}

// CheckJSON writes the result of adj check as one JSON object (docs/json.md):
// the trimmed query, the outcome, and the decided, open and closed records in
// the record shape of adj list. The three arrays are empty, never null.
func CheckJSON(w io.Writer, res query.Result) error {
	return encodeJSON(w, checkJSON{
		Query:   res.Query,
		Result:  string(res.Outcome),
		Decided: jsonRecords(res.Decided),
		Open:    jsonRecords(res.Open),
		Closed:  jsonRecords(res.Closed),
	})
}
