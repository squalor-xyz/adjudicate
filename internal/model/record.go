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
	EventNarrow    Event = "narrow"   // the record was narrowed by another, or narrows another; the note is "by N: SCOPE" or "narrows N" (S51)
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

// Known reports whether e is one of the nine events this version knows. An
// event that is not known is still accepted in a history entry, so a store
// that a newer version wrote can be read and changed; adj lint warns about it
// (S44).
func (e Event) Known() bool {
	switch e {
	case EventAsk, EventDecide, EventReject, EventWithdraw, EventSupersede, EventRenumber, EventAmend, EventImport, EventNarrow:
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
func (h HistoryEntry) UnknownKeys() []string { return keysOf(h.extra) }

// keysOf returns the sorted keys of extra, the JSON object of the unknown keys
// of a record, an entry, a was or an option in a was ("" for none).
func keysOf(extra string) []string {
	if extra == "" {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(extra), &m); err != nil { // cannot fail: extra is made here
		return nil
	}
	return slices.Sorted(maps.Keys(m))
}

// objectField is one key of a JSON object as written, with its raw value.
type objectField struct {
	key string
	raw json.RawMessage
}

// objectFields reads the keys of a JSON object in the order written. null is
// reported, so the value can be the zero one; any other value that is not an
// object is an error.
func objectFields(data []byte) (fields []objectField, null bool, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, false, err
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
		return nil, false, fmt.Errorf("want a JSON object, got a %s", kind)
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, false, err
		}
		f := objectField{key: kt.(string)} // an object's key is a string
		if err := dec.Decode(&f.raw); err != nil {
			return nil, false, err
		}
		fields = append(fields, f)
	}
	return fields, false, nil
}

// splitObject splits the JSON object in data into the values of the keys in
// names, under those names, and the other keys, kept as their JSON object text
// (compact, keys sorted; "" when there are none). A key matches a name in any
// letter case, as encoding/json does. When keys differ only in letter case from
// one name, the one spelled exactly is the known key (else the last, as
// encoding/json does) and the others are unknown keys (C214, S46); a key
// repeated as spelled is a duplicate, and the last one wins. null is reported,
// so the value can be the zero one.
func splitObject(data []byte, names []string) (known map[string]json.RawMessage, extra string, null bool, err error) {
	fields, null, err := objectFields(data)
	if err != nil || null {
		return nil, "", null, err
	}
	known, unknown := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	isKnown := make([]bool, len(fields))
	for _, name := range names {
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
			// A key spelled otherwise is kept and reported by lint.
			if f.key != fields[winner].key {
				unknown[f.key] = f.raw
			}
		}
	}
	for i, f := range fields {
		if !isKnown[i] {
			unknown[f.key] = f.raw
		}
	}
	if len(unknown) > 0 {
		text, err := encodeJSON(unknown)
		if err != nil {
			return nil, "", false, err
		}
		extra = string(text)
	}
	return known, extra, false, nil
}

// withExtra adds the members of extra, a JSON object text from splitObject, to
// the JSON object out, after its own members.
func withExtra(out []byte, extra string) []byte {
	if len(extra) <= 2 { // none, or "{}"
		return out
	}
	members := extra[1 : len(extra)-1]
	if string(out) == "{}" {
		return []byte("{" + members + "}")
	}
	out = append(bytes.TrimSuffix(out, []byte("}")), ',')
	out = append(out, members...)
	return append(out, '}')
}

// renameStruct gives a type error of the decoder that is not exported the name
// of the type it is for, so the message does not name the helper type.
func renameStruct(err error, from, to string) error {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) && te.Struct == from {
		fixed := *te
		fixed.Struct = to
		return &fixed
	}
	return err
}

// decodeError names the history entry in err and keeps the types that decode
// it out of the message.
func decodeError(err error) error {
	return fmt.Errorf("history entry: %w", renameStruct(err, "knownEntry", "HistoryEntry"))
}

// UnmarshalJSON decodes the known keys as encoding/json does (a key matches
// one of them in any letter case) and keeps every other key (splitObject).
// A was of a known event is decoded as a Was, which keeps its own unknown keys
// (S46); for an event this version does not know, was is kept as written
// (C213).
func (h *HistoryEntry) UnmarshalJSON(data []byte) error {
	known, extra, null, err := splitObject(data, entryKeys)
	if err != nil {
		return fmt.Errorf("history entry: %w", err)
	}
	if null { // as for any struct, null leaves the zero entry
		*h = HistoryEntry{}
		return nil
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
	*h = HistoryEntry{Date: k.Date, Event: k.Event, Note: k.Note, extra: extra}
	if hasWas {
		if k.Event.Known() {
			if err := json.Unmarshal(wasRaw, &h.Was); err != nil {
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
	if h.Was == nil && h.rawWas != "" {
		out = withExtra(out, "{"+`"was":`+h.rawWas+"}")
	}
	return withExtra(out, h.extra), nil
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
//
// Keys this version does not know (an amend of a field a newer version added)
// are kept apart, in extra, and written back after the known keys in sorted
// order, as for a HistoryEntry (S46); extra is a string so Was stays
// comparable with reflect.DeepEqual.
type Was struct {
	Title     *string     `json:"title,omitempty"`
	Context   *string     `json:"context,omitempty"`
	Rationale *string     `json:"rationale,omitempty"`
	Options   []OptionWas `json:"options,omitempty"`

	extra string // the keys this version does not know, as a JSON object
}

// wasKeys are the keys of a Was that this version knows.
var wasKeys = []string{"title", "context", "rationale", "options"}

// wasJSON is Was's known keys, for encoding and decoding without the methods.
type wasJSON Was

// empty reports whether w holds no old text. A key this version does not know
// is old text of a field it does not know (C233).
func (w Was) empty() bool {
	return w.Title == nil && w.Context == nil && w.Rationale == nil && len(w.Options) == 0 && w.extra == ""
}

// UnknownKeys returns the keys of w that this version does not know, sorted;
// those inside an option are in OptionWas.UnknownKeys.
func (w Was) UnknownKeys() []string { return keysOf(w.extra) }

// UnmarshalJSON decodes the known keys and keeps the others (splitObject).
func (w *Was) UnmarshalJSON(data []byte) error {
	known, extra, null, err := splitObject(data, wasKeys)
	if err != nil {
		return err
	}
	if null {
		*w = Was{}
		return nil
	}
	text, err := encodeJSON(known)
	if err != nil {
		return err
	}
	var k wasJSON
	if err := json.Unmarshal(text, &k); err != nil {
		return renameStruct(err, "wasJSON", "Was")
	}
	*w = Was(k)
	w.extra = extra
	return nil
}

// MarshalJSON writes the known keys, then the unknown ones sorted.
func (w Was) MarshalJSON() ([]byte, error) {
	out, err := encodeJSON(wasJSON(w))
	if err != nil {
		return nil, err
	}
	return withExtra(out, w.extra), nil
}

// OptionWas is the old summary and/or tradeoffs of the option with Label. Keys
// this version does not know are kept as for a Was (S46).
type OptionWas struct {
	Label     string  `json:"label"`
	Summary   *string `json:"summary,omitempty"`
	Tradeoffs *string `json:"tradeoffs,omitempty"`

	extra string // the keys this version does not know, as a JSON object
}

// optionWasKeys are the keys of an OptionWas that this version knows.
var optionWasKeys = []string{"label", "summary", "tradeoffs"}

// optionWasJSON is OptionWas's known keys, for encoding and decoding without
// the methods.
type optionWasJSON OptionWas

// UnknownKeys returns the keys of o that this version does not know, sorted.
func (o OptionWas) UnknownKeys() []string { return keysOf(o.extra) }

// UnmarshalJSON decodes the known keys and keeps the others (splitObject).
func (o *OptionWas) UnmarshalJSON(data []byte) error {
	known, extra, null, err := splitObject(data, optionWasKeys)
	if err != nil {
		return err
	}
	if null {
		*o = OptionWas{}
		return nil
	}
	text, err := encodeJSON(known)
	if err != nil {
		return err
	}
	var k optionWasJSON
	if err := json.Unmarshal(text, &k); err != nil {
		return renameStruct(err, "optionWasJSON", "OptionWas")
	}
	*o = OptionWas(k)
	o.extra = extra
	return nil
}

// MarshalJSON writes the known keys, then the unknown ones sorted.
func (o OptionWas) MarshalJSON() ([]byte, error) {
	out, err := encodeJSON(optionWasJSON(o))
	if err != nil {
		return nil, err
	}
	return withExtra(out, o.extra), nil
}

// Narrowing is one link from a decided record to a decided record that narrows
// it (S51): ID is the narrowing record and Scope says, in one line, what of the
// decision it narrows. Unlike a supersede the narrowed record stays decided and
// in force; the scope is the part that the narrowing record changes.
type Narrowing struct {
	ID    int    `json:"id"`
	Scope string `json:"scope"`
}

// Record is one question or decision. Field order follows the PROJECT.md
// Core concepts table (minus kind) so encoding/json output is deterministic.
// Dates are time.DateOnly strings (YYYY-MM-DD). AskedBy and DecidedBy are
// free-text attribution, not authentication (C27). RecordedBy is who entered
// the answer when that is not DecidedBy, and Via how the owner gave it (S52).
type Record struct {
	ID             int             `json:"id"`
	Title          string          `json:"title"`
	Status         Status          `json:"status"`
	Key            string          `json:"key,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	AskedBy        string          `json:"asked_by,omitempty"`
	DecidedBy      string          `json:"decided_by,omitempty"`
	RecordedBy     string          `json:"recorded_by,omitempty"`
	Via            string          `json:"via,omitempty"`
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
	NarrowedBy     []Narrowing     `json:"narrowed_by,omitempty"`
	Narrows        []int           `json:"narrows,omitempty"`
	Refs           []string        `json:"refs,omitempty"`
	History        []HistoryEntry  `json:"history,omitempty"`

	// Body is the Markdown after the frontmatter, kept verbatim. It is not a
	// frontmatter field (json:"-"); see file.go.
	Body string `json:"-"`

	extra string // the top-level keys this version does not know, as a JSON object
}

// recordKeys are the frontmatter keys of a Record that this version knows.
var recordKeys = []string{
	"id", "title", "status", "key", "tags", "asked_by", "decided_by", "recorded_by", "via", "created",
	"decided_at", "context", "options", "recommendation", "decision", "rationale", "consequences",
	"supersedes", "superseded_by", "narrowed_by", "narrows", "refs", "history",
}

// recordJSON is Record's known keys, for encoding and decoding without the
// methods below.
type recordJSON Record

// UnknownKeys returns the top-level keys of r that this version does not know,
// sorted; none for a record without any. A newer version may write keys that
// this one keeps and writes back unchanged (S46).
func (r Record) UnknownKeys() []string { return keysOf(r.extra) }

// UnknownJSON returns the unknown top-level keys of r as a JSON object (compact,
// keys sorted), or "" when there are none. Two records that differ only in what
// a newer version wrote have different texts.
func (r Record) UnknownJSON() string { return r.extra }

// UnmarshalJSON decodes the known keys, as encoding/json does but with the
// letter-case rule of splitObject, and keeps every other top-level key apart,
// so that rewriting the record never drops what a newer version added (S46).
// The known keys stay strict about type, and so do the objects inside them (an
// option, the recommendation) except the history, whose entries are tolerant
// (S44).
func (r *Record) UnmarshalJSON(data []byte) error {
	known, extra, null, err := splitObject(data, recordKeys)
	if err != nil {
		return err
	}
	if null {
		*r = Record{}
		return nil
	}
	text, err := encodeJSON(known)
	if err != nil {
		return err
	}
	var k recordJSON
	dec := json.NewDecoder(bytes.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&k); err != nil {
		return renameStruct(err, "recordJSON", "Record")
	}
	*r = Record(k)
	r.extra = extra
	return nil
}

// MarshalJSON writes the known keys in the order of the struct, then the
// unknown ones in sorted key order, so the bytes are the same on every write.
func (r Record) MarshalJSON() ([]byte, error) {
	out, err := encodeJSON(recordJSON(r))
	if err != nil {
		return nil, err
	}
	return withExtra(out, r.extra), nil
}

// MarshalJSONWithBody is MarshalJSON with the body added as a "body" key when
// the record has one, as the read commands print a record (docs/json.md). A
// frontmatter key of that name is an unknown key like any other in the file
// (C234), but it is always left out here (C235): body in this JSON is the text
// after the frontmatter, never a frontmatter value.
func (r Record) MarshalJSONWithBody() ([]byte, error) {
	if r.extra != "" {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(r.extra), &m); err != nil { // cannot fail: extra is made here
			return nil, err
		}
		delete(m, "body")
		r.extra = ""
		if len(m) > 0 {
			text, err := encodeJSON(m)
			if err != nil {
				return nil, err
			}
			r.extra = string(text)
		}
	}
	out, err := r.MarshalJSON()
	if err != nil || r.Body == "" {
		return out, err
	}
	body, err := encodeJSON(r.Body)
	if err != nil {
		return nil, err
	}
	return append(bytes.TrimSuffix(out, []byte("}")), `,"body":`+string(body)+"}"...), nil
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
// superseded needs superseded_by. decided_by is never required (C27), and
// neither are recorded_by and via, which must not be blank when set (S52).
// A recommendation has an option label or free text, not both (C167).
// narrowed_by and narrows list other records, each once, and a narrowing has a
// one-line scope that is not blank (S51); the status they sit on is lint's
// business, as for superseded_by.
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

	if r.RecordedBy != "" && strings.TrimSpace(r.RecordedBy) == "" {
		bad("recorded_by", "must not be blank when set")
	}
	if r.Via != "" && strings.TrimSpace(r.Via) == "" {
		bad("via", "must not be blank when set")
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
	seenBy := map[int]bool{}
	for _, n := range r.NarrowedBy {
		switch {
		case n.ID <= 0 || n.ID == r.ID:
			bad("narrowed_by", "%d is not another record's id", n.ID)
		case seenBy[n.ID]:
			bad("narrowed_by", "%d is listed twice", n.ID)
		}
		seenBy[n.ID] = true
		switch {
		case strings.TrimSpace(n.Scope) == "":
			bad("narrowed_by", "the scope for %d must not be blank", n.ID)
		case strings.ContainsAny(n.Scope, "\r\n"):
			bad("narrowed_by", "the scope for %d must be one line", n.ID)
		}
	}
	seenNarrows := map[int]bool{}
	for _, id := range r.Narrows {
		switch {
		case id <= 0 || id == r.ID:
			bad("narrows", "%d is not another record's id", id)
		case seenNarrows[id]:
			bad("narrows", "%d is listed twice", id)
		}
		seenNarrows[id] = true
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
				case o.Summary == nil && o.Tradeoffs == nil && o.extra == "":
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
