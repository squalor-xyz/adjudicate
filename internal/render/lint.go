// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"fmt"
	"io"

	"github.com/squalor-xyz/adjudicate/internal/lint"
)

// LintText writes one line per finding, "FILE: record ID: SEVERITY: CHECK:
// MESSAGE", then a summary line such as "2 errors, 1 warning". A report with
// no finding writes nothing.
func LintText(w io.Writer, findings []lint.Finding) error {
	if len(findings) == 0 {
		return nil
	}
	for _, f := range findings {
		if _, err := fmt.Fprintf(w, "%s: record %d: %s: %s: %s\n", f.File, f.ID, f.Severity, f.Check, f.Message); err != nil {
			return err
		}
	}
	errs, warns := lint.Counts(findings)
	_, err := fmt.Fprintf(w, "%s, %s\n", plural(errs, "error"), plural(warns, "warning"))
	return err
}

// plural formats a count and its noun: "1 error", "0 errors".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// lintJSON is the object adj lint --json prints.
type lintJSON struct {
	Errors   int            `json:"errors"`
	Warnings int            `json:"warnings"`
	Findings []lint.Finding `json:"findings"`
}

// LintJSON writes {"errors", "warnings", "findings"}; no findings is an empty
// array.
func LintJSON(w io.Writer, findings []lint.Finding) error {
	errs, warns := lint.Counts(findings)
	if findings == nil {
		findings = []lint.Finding{}
	}
	return encodeJSON(w, lintJSON{Errors: errs, Warnings: warns, Findings: findings})
}
