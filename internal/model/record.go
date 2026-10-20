// SPDX-License-Identifier: GPL-3.0-or-later

// Package model defines the adjudicate record, its validation, its lifecycle
// transitions and its file format. Its only file I/O is ReadFile and WriteFile
// in file.go.
package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"
)

// ErrInvalid is matched (errors.Is) by every error this package returns for
// a record or a transition input that breaks the model's rules.
var ErrInvalid = errors.New("invalid record")

// Status is a record's lifecycle state. Status alone carries the record kind
// (C30): open is a question; decided, rejected, withdrawn and superseded are
// answered.
type Status string

const (
	StatusOpen       Status = "open"
	StatusDecided    Status = "decided"
	StatusRejected   Status = "rejected"
	StatusWithdrawn  Status = "withdrawn"
	StatusSuperseded Status = "superseded"
)

// Valid reports whether s is one of the five statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusDecided, StatusRejected, StatusWithdrawn, StatusSuperseded:
		return true
	}
	return false
}

// Event names what a history entry records: the transition that was applied.
type Event string

const (
	EventAsk       Event = "ask" // written by the code that creates the record
	EventDecide    Event = "decide"
	EventReject    Event = "reject"
	EventWithdraw  Event = "withdraw"
	EventSupersede Event = "supersede"
	EventRenumber  Event = "renumber" // a collision on the ID was resolved (C26)
	EventAmend     Event = "amend"    // text fields were corrected; the entry's Was keeps the old text (C166)
	EventImport    Event = "import"   // the record was made from an ADR file by adj import-adr; the note is "from FILE" (S22)
)

// Option is one answer a question offers.
type Option struct {
	Label     string `json:"label"`
	Summary   string `json:"summary,omitempty"`
	Tradeoffs string `json:"tradeoffs,omitempty"`
}

// Recommendation is the asker's suggestion and why. It names one of the
// record's options (Label) or is free text (Text, C167); exactly one is set.
// Records written before Text existed have a Label, and read and write as they
// did.
type Recommendation struct {
	Label     string `json:"label,omitempty"`
	Text      string `json:"text,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

// Known reports whether e is one of the eight events this version knows. An
// event that is not known is still accepted in a history entry, so a store
// that a newer version wrote can be read and changed; adj lint warns about it
// (S44).
func (e Event) Known() bool {
	switch e {
	case EventAsk, EventDecide, EventReject, EventWithdraw, EventSupersede, EventRenumber, EventAmend, EventImport:
		return true
	}
	return false
}

// HistoryEntry is one dated line of a record's history. Date is
// time.DateOnly (YYYY-MM-DD).
//
// An entry may have an event or keys that this version does not know (S44).
// The event is kept in Event whatever it is. The keys are kept apart, in
// extra, and written back unchanged after the known keys in sorted order, so
// rewriting a record never drops what a newer version added. For an event this
// version does not know, was is not decoded either: it is kept as it was
// written, in rawWas, and Was stays nil (C213). extra and rawWas are strings
// (extra is the JSON object of the unknown keys, compact, keys sorted; both are
// "" when there is nothing), so that two entries that differ only in what a
// newer version wrote are not equal: renumber's stale-copy match compares
// entries with reflect.DeepEqual, and the type stays comparable with ==, where
// Was, a pointer, compares by address.
type HistoryEntry struct {
	Date  string `json:"date"`
	Event Event  `json:"event"`
	Note  string `json:"note,omitempty"`
	Was   *Was   `json:"was,omitempty"` // amend only: the text the amend replaced

	extra  string // the keys this version does not know, as a JSON object
	rawWas string // the was of an event this version does not know, as written
}

// entryKeys are the keys of a HistoryEntry that this version knows.
var entryKeys = []string{"date", "event", "note", "was"}

// knownEntry is HistoryEntry's known keys other than was, for encoding and
// decoding without the methods below.
type knownEntry struct {
	Date  string `json:"date"`
	Event Event  `json:"event"`
	Note  string `json:"note,omitempty"`
	Was   *Was   `json:"was,omitempty"`
}

// UnknownKeys returns the keys of the entry that this version does not know,
// sorted; none for an entry without any. A was kept raw is not one: was is a
// key this version knows, and the event is what it does not.
func (h HistoryEntry) UnknownKeys() []string {
	if h.extra == "" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(h.extra), &m); err != nil { // cannot fail: extra is made here
		return nil
	}
	return slices.Sorted(maps.Keys(m))
}

// entryField is one key of a history entry as written, with its raw value.
type entryField struct {
	key string
	raw json.RawMessage
}

// entryFields reads the keys of a JSON object in the order written. null is
// reported, so the entry can be the zero entry; any other value that is not an
// object is an error.
func entryFields(data []byte) (fields []entryField, null bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, false, fmt.Errorf("history entry: %w", err)
	}
	if tok == nil {
		return nil, true, nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		kind := "value"
		switch tok.(type) {
		case json.Delim:
			kind = "array"
		case string:
			kind = "string"
		case float64:
			kind = "number"
		case bool:
			kind = "boolean"
		}
		return nil, false, fmt.Errorf("history entry: want a JSON object, got a %s", kind)
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false, fmt.Errorf("history entry: %w", err)
		}
		f := entryField{key: kt.(string)} // an object's key is a string
		if err := dec.Decode(&f.raw); err != nil {
			return nil, false, fmt.Errorf("history entry: %w", err)
		}
		fields = append(fields, f)
	}
	return fields, false, nil
}

// decodeError names the history entry in err and keeps the types that decode
// it out of the message.
func decodeError(err error) error {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) && (te.Struct == "knownEntry" || te.Struct == "" && te.Field != "") {
		fixed := *te
		fixed.Struct = "HistoryEntry" // the date, event or note
		if te.Struct == "" {
			fixed.Struct = "Was" // a field of was
		}
		err = &fixed
	}
	return fmt.Errorf("history entry: %w", err)
}

// UnmarshalJSON decodes the known keys as encoding/json does (a key matches
// one of them in any letter case) and keeps every other key. When keys differ
// only in letter case from one known key, the one spelled exactly is the known
// key (else the last, as encoding/json does) and the others are unknown keys
// (C214).
// For a known event an unknown key inside was is still an error; for an event
// this version does not know, was is kept as written (C213).
func (h *HistoryEntry) UnmarshalJSON(data []byte) error {
	fields, null, err := entryFields(data)
	if err != nil {
		return err
	}
	if null { // as for any struct, null leaves the zero entry
		*h = HistoryEntry{}
		return nil
	}
	known, extra := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	isKnown := make([]bool, len(fields))
	for _, name := range entryKeys {
		winner := -1
		for i, f := range fields {
			if strings.EqualFold(f.key, name) && (f.key == name || winner < 0 || fields[winner].key != name) {
				winner = i
			}
		}
		if winner < 0 {
			continue
		}
		known[name] = fields[winner].raw
		isKnown[winner] = true
		for i, f := range fields {
			if !strings.EqualFold(f.key, name) {
				continue
			}
			isKnown[i] = true
			// A key spelled otherwise is kept and reported by lint; a key
			// repeated as spelled is a duplicate, and the last one wins.
			if f.key != fields[winner].key {
				extra[f.key] = f.raw
			}
		}
	}
	for i, f := range fields {
		if !isKnown[i] {
			extra[f.key] = f.raw
		}
	}
	wasRaw, hasWas := known["was"]
	delete(known, "was")
	text, err := encodeJSON(known)
	if err != nil {
		return err
	}
	var k knownEntry
	if err := json.Unmarshal(text, &k); err != nil {
		return decodeError(err)
	}
	*h = HistoryEntry{Date: k.Date, Event: k.Event, Note: k.Note}
	if hasWas {
		if k.Event.Known() {
			dec := json.NewDecoder(bytes.NewReader(wasRaw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&h.Was); err != nil {
				var te *json.UnmarshalTypeError
				if errors.As(err, &te) && te.Field == "" { // was itself is not an object
					err = fmt.Errorf("want an object, got a %s", te.Value)
				}
				return decodeError(fmt.Errorf("was: %w", err))
			}
		} else {
			raw, err := encodeJSON(wasRaw)
			if err != nil {
				return err
			}
			h.rawWas = string(raw)
		}
	}
	if len(extra) > 0 {
		text, err := encodeJSON(extra)
		if err != nil {
			return err
		}
		h.extra = string(text)
	}
	return nil
}

// MarshalJSON writes the known keys in their usual order (a was kept raw in
// its place), then the unknown keys in sorted key order, so the bytes are the
// same on every write.
func (h HistoryEntry) MarshalJSON() ([]byte, error) {
	out, err := encodeJSON(knownEntry{Date: h.Date, Event: h.Event, Note: h.Note, Was: h.Was})
	if err != nil {
		return nil, err
	}
	with := func(members string) { // members: key-value pairs, no braces
		out = append(bytes.TrimSuffix(out, []byte("}")), ',')
		out = append(out, members...)
		out = append(out, '}')
	}
	if h.Was == nil && h.rawWas != "" {
		with(`"was":` + h.rawWas)
	}
	if len(h.extra) > 2 { // more than "{}"
		with(h.extra[1 : len(h.extra)-1])
	}
	return out, nil
}

// encodeJSON encodes v compactly, without escaping HTML characters (a file
// keeps "<" and "&" as typed) and without the encoder's final newline. A map
// is written with its keys sorted.
func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// Was is the text an amend replaced: one pointer per field that changed, nil
// for a field that did not, so an old empty text (a pointer to "") is told
// apart from a field that was left alone.
type Was struct {
	Title     *string     `json:"title,omitempty"`
	Context   *string     `json:"context,omitempty"`
	Rationale *string     `json:"rationale,omitempty"`
	Options   []OptionWas `json:"options,omitempty"`
}

// empty reports whether w holds no old text.
func (w Was) empty() bool {
	return w.Title == nil && w.Context == nil && w.Rationale == nil && len(w.Options) == 0
}

// OptionWas is the old summary and/or tradeoffs of the option with Label.
type OptionWas struct {
	Label     string  `json:"label"`
	Summary   *string `json:"summary,omitempty"`
	Tradeoffs *string `json:"tradeoffs,omitempty"`
}

// Record is one question or decision. Field order follows the PROJECT.md
// Core concepts table (minus kind) so encoding/json output is deterministic.
// Dates are time.DateOnly strings (YYYY-MM-DD). AskedBy and DecidedBy are
// free-text attribution, not authentication (C27).
type Record struct {
	ID             int             `json:"id"`
	Title          string          `json:"title"`
	Status         Status          `json:"status"`
	Key            string          `json:"key,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	AskedBy        string          `json:"asked_by,omitempty"`
	DecidedBy      string          `json:"decided_by,omitempty"`
	Created        string          `json:"created"`
	DecidedAt      string          `json:"decided_at,omitempty"`
	Context        string          `json:"context,omitempty"`
	Options        []Option        `json:"options,omitempty"`
	Recommendation *Recommendation `json:"recommendation,omitempty"`
	Decision       string          `json:"decision,omitempty"`
	Rationale      string          `json:"rationale,omitempty"`
	Consequences   string          `json:"consequences,omitempty"`
	Supersedes     []int           `json:"supersedes,omitempty"`
	SupersededBy   int             `json:"superseded_by,omitempty"`
	Refs           []string        `json:"refs,omitempty"`
	History        []HistoryEntry  `json:"history,omitempty"`

	// Body is the Markdown after the frontmatter, kept verbatim. It is not a
	// frontmatter field (json:"-"); see file.go.
	Body string `json:"-"`
}

// validDate reports whether s is a YYYY-MM-DD date.
func validDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

// Validate checks the record against the model's rules and returns nil, or an
// error matching ErrInvalid that names every problem found.
//
// Always required: id > 0, title (one non-blank line), a known status and a
// created date. Every history entry needs a valid date and a non-blank event;
// an event this version does not know is accepted (S44).
// Once answered (decided, rejected, withdrawn, superseded):
// rationale and decided_at; decided and superseded also need decision, and
// superseded needs superseded_by. decided_by is never required (C27).
// A recommendation has an option label or free text, not both (C167).
func (r Record) Validate() error {
	var errs []error
	bad := func(field, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%w: %s: %s", ErrInvalid, field, fmt.Sprintf(format, args...)))
	}

	if r.ID <= 0 {
		bad("id", "must be a positive integer, got %d", r.ID)
	}
	if strings.TrimSpace(r.Title) == "" {
		bad("title", "is required")
	} else if strings.ContainsAny(r.Title, "\r\n") {
		bad("title", "must be one line")
	}
	if !r.Status.Valid() {
		bad("status", "unknown status %q", string(r.Status))
	}
	if r.Created == "" {
		bad("created", "is required")
	} else if !validDate(r.Created) {
		bad("created", "%q is not a YYYY-MM-DD date", r.Created)
	}

	answered := r.Status.Valid() && r.Status != StatusOpen
	if answered && strings.TrimSpace(r.Rationale) == "" {
		bad("rationale", "is required once %s", r.Status)
	}
	if answered && r.DecidedAt == "" {
		bad("decided_at", "is required once %s", r.Status)
	} else if r.DecidedAt != "" && !validDate(r.DecidedAt) {
		bad("decided_at", "%q is not a YYYY-MM-DD date", r.DecidedAt)
	}
	if (r.Status == StatusDecided || r.Status == StatusSuperseded) && strings.TrimSpace(r.Decision) == "" {
		bad("decision", "is required once %s", r.Status)
	}
	if r.Status == StatusSuperseded && r.SupersededBy <= 0 {
		bad("superseded_by", "is required once superseded")
	}
	if r.SupersededBy != 0 && (r.SupersededBy < 0 || r.SupersededBy == r.ID) {
		bad("superseded_by", "%d is not another record's id", r.SupersededBy)
	}
	for _, id := range r.Supersedes {
		if id <= 0 || id == r.ID {
			bad("supersedes", "%d is not another record's id", id)
		}
	}

	for i, h := range r.History {
		if !validDate(h.Date) {
			bad("history", "entry %d: %q is not a YYYY-MM-DD date", i, h.Date)
		}
		if strings.TrimSpace(string(h.Event)) == "" {
			bad("history", "entry %d: event is required", i)
		}
		// The was rules are for the events this version knows: an unknown
		// event may carry any was a newer version gave it (S44).
		switch {
		case h.Event == EventAmend && (h.Was == nil || h.Was.empty()):
			bad("history", "entry %d: an amend entry needs the old text in was", i)
		case h.Event != EventAmend && h.Event.Known() && h.Was != nil:
			bad("history", "entry %d: only an amend entry has was", i)
		}
		if h.Was != nil {
			for _, o := range h.Was.Options {
				switch {
				case o.Label == "":
					bad("history", "entry %d: was.options: an option has no label", i)
				case o.Summary == nil && o.Tradeoffs == nil:
					bad("history", "entry %d: was.options: option %q has no old summary or tradeoffs", i, o.Label)
				}
			}
		}
	}

	labels := make(map[string]bool, len(r.Options))
	for _, o := range r.Options {
		switch {
		case o.Label == "":
			bad("options", "an option has no label")
		case labels[o.Label]:
			bad("options", "duplicate label %q", o.Label)
		}
		labels[o.Label] = true
	}
	if rec := r.Recommendation; rec != nil {
		switch {
		case rec.Label != "" && rec.Text != "":
			bad("recommendation", "has both a label and text; want one")
		case rec.Label == "" && rec.Text != "":
			if strings.TrimSpace(rec.Text) == "" {
				bad("recommendation", "text must not be blank")
			}
		case rec.Label == "":
			bad("recommendation", "needs a label or text")
		case !labels[rec.Label]:
			bad("recommendation", "label %q is not one of the options", rec.Label)
		}
	}

	return errors.Join(errs...)
}
