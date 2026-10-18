// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/model"
)

// InboxText writes the open questions of adj inbox in the order given. Each
// entry is its ID and title, then, indented, its created date, its refs when
// set (C171), its context, its options (with summary and tradeoffs) and its
// recommendation with the reasoning (a free-text recommendation shows as its
// text); the rest of the record is left to adj show. Entries are separated by
// one blank line. No records prints nothing.
func InboxText(w io.Writer, recs []model.Record) error {
	var b strings.Builder
	for i, r := range recs {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d  %s\n", r.ID, r.Title)
		writeField(&b, "  ", "Created", r.Created)
		writeField(&b, "  ", "Refs", strings.Join(r.Refs, ", "))
		writeField(&b, "  ", "Context", r.Context)
		writeOptions(&b, "  ", r.Options)
		if rec := r.Recommendation; rec != nil {
			writeField(&b, "  ", "Recommendation", recommended(rec))
			writeField(&b, "  ", "Reasoning", rec.Reasoning)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// InboxAllText writes a root header and the existing record blocks per repo.
func InboxAllText(w io.Writer, result commands.AllInbox) error {
	for i, repo := range result.Repos {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, repo.Root); err != nil {
			return err
		}
		if err := InboxText(w, repo.Records); err != nil {
			return err
		}
	}
	return nil
}

// InboxAllJSON writes non-null arrays, including the per-repo warnings.
func InboxAllJSON(w io.Writer, result commands.AllInbox) error {
	type repoJSON struct {
		Root    string       `json:"root"`
		Records []jsonRecord `json:"records"`
	}
	out := struct {
		Repos  []repoJSON            `json:"repos"`
		Errors []commands.InboxError `json:"errors"`
	}{Repos: []repoJSON{}, Errors: []commands.InboxError{}}
	for _, repo := range result.Repos {
		out.Repos = append(out.Repos, repoJSON{Root: repo.Root, Records: jsonRecords(repo.Records)})
	}
	out.Errors = append(out.Errors, result.Errors...)
	return encodeJSON(w, out)
}
