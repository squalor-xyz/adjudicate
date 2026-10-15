// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"encoding/json"
	"io"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// jsonRecord is a record in the JSON shape documented in docs/json.md: the
// model's fields, plus a body key when the record has a body (C88).
type jsonRecord struct {
	model.Record
	Body string `json:"body,omitempty"`
}

// listJSON is the object adj list --json prints (C86).
type listJSON struct {
	Records []jsonRecord `json:"records"`
}

// RecordJSON writes one record as a JSON object.
func RecordJSON(w io.Writer, r model.Record) error {
	return encodeJSON(w, jsonRecord{Record: r, Body: r.Body})
}

// RecordsJSON writes {"records": [...]} in the order given, with an empty
// array when there are no records.
func RecordsJSON(w io.Writer, recs []model.Record) error {
	return encodeJSON(w, listJSON{Records: jsonRecords(recs)})
}

// jsonRecords converts recs to their JSON shape; no records is an empty,
// non-nil slice, which encodes as [].
func jsonRecords(recs []model.Record) []jsonRecord {
	out := []jsonRecord{}
	for _, r := range recs {
		out = append(out, jsonRecord{Record: r, Body: r.Body})
	}
	return out
}

// encodeJSON writes v with two-space indent, HTML characters unescaped, and
// the final newline that Encode adds.
func encodeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
