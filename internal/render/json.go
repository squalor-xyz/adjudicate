// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"encoding/json"
	"io"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// jsonRecord is a record in the JSON shape documented in docs/json.md: the
// model's fields, the keys it does not know (S46), then a body key when the
// record has a body (C88).
type jsonRecord struct {
	model.Record
}

// MarshalJSON writes the record with its body. Without it the embedded
// Record's own method would be used, which leaves the body out.
func (j jsonRecord) MarshalJSON() ([]byte, error) { return j.Record.MarshalJSONWithBody() }

// listJSON is the object adj list --json prints (C86).
type listJSON struct {
	Records []jsonRecord `json:"records"`
}

// RecordJSON writes one record as a JSON object.
func RecordJSON(w io.Writer, r model.Record) error {
	return encodeJSON(w, jsonRecord{Record: r})
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
		out = append(out, jsonRecord{Record: r})
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
