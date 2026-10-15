// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// InboxText writes the open questions of adj inbox in the order given. Each
// entry is its ID and title, then, indented, its created date, its context,
// its options (with summary and tradeoffs) and its recommendation with the
// reasoning; the rest of the record is left to adj show. Entries are separated
// by one blank line. No records prints nothing.
func InboxText(w io.Writer, recs []model.Record) error {
	var b strings.Builder
	for i, r := range recs {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%d  %s\n", r.ID, r.Title)
		writeField(&b, "  ", "Created", r.Created)
		writeField(&b, "  ", "Context", r.Context)
		writeOptions(&b, "  ", r.Options)
		if rec := r.Recommendation; rec != nil {
			writeField(&b, "  ", "Recommendation", rec.Label)
			writeField(&b, "  ", "Reasoning", rec.Reasoning)
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}
