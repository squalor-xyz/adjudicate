// SPDX-License-Identifier: GPL-3.0-or-later

// Package render writes records for people (text) and for agents (JSON). It
// does no file I/O of its own: the caller passes the writer.
package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// RecordText writes one record as labelled lines, in a fixed order. An empty
// field is left out. A value with line breaks goes under its label, indented,
// and the body goes last, verbatim. Trailing line breaks of a value are not
// shown; the record is not changed.
func RecordText(w io.Writer, r model.Record) error {
	var b strings.Builder
	field := func(label, value string) { writeField(&b, "", label, value) }

	fmt.Fprintf(&b, "ID: %d\n", r.ID)
	field("Title", r.Title)
	field("Status", string(r.Status))
	field("Key", r.Key)
	field("Tags", strings.Join(r.Tags, ", "))
	field("Asked by", r.AskedBy)
	field("Created", r.Created)
	field("Context", r.Context)
	writeOptions(&b, "", r.Options)
	if rec := r.Recommendation; rec != nil {
		field("Recommendation", recommended(rec))
		field("Reasoning", rec.Reasoning)
	}
	field("Decided by", r.DecidedBy)
	field("Decided at", r.DecidedAt)
	field("Decision", r.Decision)
	field("Rationale", r.Rationale)
	field("Consequences", r.Consequences)
	if len(r.Supersedes) > 0 {
		field("Supersedes", joinInts(r.Supersedes))
	}
	if r.SupersededBy != 0 {
		field("Superseded by", strconv.Itoa(r.SupersededBy))
	}
	field("Refs", strings.Join(r.Refs, ", "))
	if len(r.History) > 0 {
		b.WriteString("History:\n")
		for _, h := range r.History {
			if note := trimBreaks(h.Note); note == "" {
				fmt.Fprintf(&b, "  %s %s\n", h.Date, h.Event)
			} else {
				fmt.Fprintf(&b, "  %s %s: %s\n", h.Date, h.Event, cont(note, "    "))
			}
		}
	}
	if r.Body != "" {
		b.WriteString("Body:\n")
		b.WriteString(r.Body)
		if !strings.HasSuffix(r.Body, "\n") {
			b.WriteByte('\n')
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// recommended is what a recommendation shows after "Recommendation:": its free
// text (C167), or else the option label.
func recommended(rec *model.Recommendation) string {
	if rec.Text != "" {
		return rec.Text
	}
	return rec.Label
}

// RecordsText writes one line per record: ID, status and title, in aligned
// columns. A tab in a title becomes a space so the columns hold.
func RecordsText(w io.Writer, recs []model.Record) error {
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, r := range recs {
		title := strings.ReplaceAll(r.Title, "\t", " ")
		fmt.Fprintf(tw, "%d\t%s\t%s\n", r.ID, r.Status, title)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// trimBreaks drops the trailing line breaks of a value for display; the
// record is not changed.
func trimBreaks(s string) string {
	return strings.TrimRight(s, "\n")
}

// writeField writes "label: value" at indent pad. A value with line breaks goes
// under its label, indented two more; a value that is empty once its trailing
// line breaks are dropped is left out.
func writeField(b *strings.Builder, pad, label, value string) {
	value = trimBreaks(value)
	switch {
	case value == "":
	case strings.Contains(value, "\n"):
		fmt.Fprintf(b, "%s%s:\n%s  %s\n", pad, label, pad, cont(value, pad+"  "))
	default:
		fmt.Fprintf(b, "%s%s: %s\n", pad, label, value)
	}
}

// writeOptions writes an "Options:" line at indent pad and each option under
// it, two deeper: "label: summary", then its tradeoffs, if any, two deeper
// again. Trailing line breaks of a summary or tradeoffs are dropped. It writes
// nothing when there are no options.
func writeOptions(b *strings.Builder, pad string, opts []model.Option) {
	if len(opts) == 0 {
		return
	}
	fmt.Fprintf(b, "%sOptions:\n", pad)
	for _, o := range opts {
		if summary := trimBreaks(o.Summary); summary == "" {
			fmt.Fprintf(b, "%s  %s\n", pad, o.Label)
		} else {
			fmt.Fprintf(b, "%s  %s: %s\n", pad, o.Label, cont(summary, pad+"    "))
		}
		if tradeoffs := trimBreaks(o.Tradeoffs); tradeoffs != "" {
			fmt.Fprintf(b, "%s    tradeoffs: %s\n", pad, cont(tradeoffs, pad+"      "))
		}
	}
}

// cont indents the lines after the first of s by pad.
func cont(s, pad string) string {
	return strings.ReplaceAll(s, "\n", "\n"+pad)
}

// joinInts formats ns as a comma-separated list.
func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ", ")
}
