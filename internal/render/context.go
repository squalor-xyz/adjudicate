// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// ContextText writes the decided records of adj context in the order given,
// as a Markdown list. Each entry is "- ID title [key]: decision", with the key
// left out when the record has none, and the first line of the rationale on
// the next line, indented. A decision with line breaks continues on indented
// lines. Nothing else of a record is shown; adj show has it. No records
// prints nothing.
func ContextText(w io.Writer, recs []model.Record) error {
	return ContextTextOmitted(w, recs, 0)
}

// ContextTextOmitted is ContextText for a budgeted run (--max-bytes): omitted
// is the number of decided records the budget left out. When it is above 0 the
// output ends with the line "N decisions omitted" ("1 decision omitted" for
// one), after a blank line if any record is printed, so that Markdown does not
// read it as part of the last entry. With 0 it is ContextText.
func ContextTextOmitted(w io.Writer, recs []model.Record, omitted int) error {
	var b strings.Builder
	for _, r := range recs {
		fmt.Fprintf(&b, "- %d %s", r.ID, r.Title)
		if r.Key != "" {
			fmt.Fprintf(&b, " [%s]", r.Key)
		}
		fmt.Fprintf(&b, ": %s\n", cont(trimBreaks(r.Decision), "    "))
		writeField(&b, "  ", "Rationale", firstLine(r.Rationale))
	}
	if omitted > 0 {
		if len(recs) > 0 {
			b.WriteString("\n")
		}
		noun := "decisions"
		if omitted == 1 {
			noun = "decision"
		}
		fmt.Fprintf(&b, "%d %s omitted\n", omitted, noun)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// contextEntry is one decision in the adj context --format json output.
type contextEntry struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Key       string `json:"key,omitempty"`
	Decision  string `json:"decision"`
	Rationale string `json:"rationale,omitempty"`
}

// contextJSON is the object adj context --format json prints.
type contextJSON struct {
	Decisions []contextEntry `json:"decisions"`
	// Omitted is set only on a budgeted run (--max-bytes), where it is
	// printed even when 0.
	Omitted *int `json:"omitted,omitempty"`
}

// ContextJSON writes {"decisions": [...]} in the order given (docs/json.md),
// with an empty array when there are no records.
func ContextJSON(w io.Writer, recs []model.Record) error {
	return writeContextJSON(w, recs, nil)
}

// ContextJSONOmitted is ContextJSON for a budgeted run (--max-bytes): the
// object also has "omitted", the number of decided records the budget left
// out, 0 included (docs/json.md).
func ContextJSONOmitted(w io.Writer, recs []model.Record, omitted int) error {
	return writeContextJSON(w, recs, &omitted)
}

func writeContextJSON(w io.Writer, recs []model.Record, omitted *int) error {
	out := contextJSON{Decisions: []contextEntry{}, Omitted: omitted}
	for _, r := range recs {
		out.Decisions = append(out.Decisions, contextEntry{
			ID:        r.ID,
			Title:     r.Title,
			Key:       r.Key,
			Decision:  trimBreaks(r.Decision),
			Rationale: firstLine(r.Rationale),
		})
	}
	return encodeJSON(w, out)
}

// firstLine returns the first line of s that is not blank, without the space
// around it; it is "" when s has no such line.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
