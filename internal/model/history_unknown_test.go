// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"reflect"
	"strings"
	"testing"
)

// openFile is the text of an open record file whose history is the given
// JSON entries, one per argument (S44).
func openFile(entries ...string) []byte {
	return []byte(`---
{
  "id": 4,
  "title": "Logging",
  "status": "open",
  "created": "2026-10-01",
  "history": [` + strings.Join(entries, ",") + `]
}
---
`)
}

const (
	askEntry     = `{"date":"2026-10-01","event":"ask"}`
	futureEntry  = `{"date":"2026-10-02","event":"future-thing","note":"x"}`
	byOnDecide   = `{"date":"2026-10-03","event":"decide","by":"someone"}`
	futureExtras = `{"date":"2026-10-04","event":"future-thing","zeta":{"a":[1,2]},"alpha":true,"note":"n"}`
)

// A history entry whose event this version does not know is read and kept, so
// a later event kind does not make an older adj refuse the store (S44).
func TestParseAcceptsAnUnknownHistoryEvent(t *testing.T) {
	r, err := Unmarshal("0004-logging.md", openFile(askEntry, futureEntry))
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	if len(r.History) != 2 || r.History[1].Event != "future-thing" || r.History[1].Note != "x" {
		t.Fatalf("History = %+v, want the unknown entry kept with its event and note", r.History)
	}
	if r.History[1].Event.Known() {
		t.Error(`Event("future-thing").Known() = true, want false`)
	}
	if !EventDecide.Known() {
		t.Error("EventDecide.Known() = false, want true")
	}
}

// Unknown keys inside a history entry are kept, written back after the known
// keys in sorted order, and the bytes do not change on another round trip.
func TestHistoryEntryKeepsUnknownKeysAcrossRoundTrips(t *testing.T) {
	r, err := Unmarshal("0004-logging.md", openFile(askEntry, byOnDecide, futureExtras))
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	first, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(first)
	for _, want := range []string{`"by": "someone"`, `"alpha": true`, `"zeta": {`} {
		if !strings.Contains(text, want) {
			t.Errorf("Marshal() lost %s:\n%s", want, text)
		}
	}
	// Known keys first, in their usual order, then unknown ones sorted.
	order := []string{`"event": "future-thing"`, `"note": "n"`, `"alpha": true`, `"zeta": {`}
	last := -1
	for _, want := range order {
		i := strings.Index(text, want)
		if i <= last {
			t.Errorf("%s is not after the previous key in:\n%s", want, text)
		}
		last = i
	}
	again, err := Unmarshal("0004-logging.md", first)
	if err != nil {
		t.Fatalf("second Unmarshal() = %v", err)
	}
	if !reflect.DeepEqual(again, r) {
		t.Errorf("parse, marshal, parse gave\n %+v\nwant\n %+v", again, r)
	}
	second, err := Marshal(again)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != text {
		t.Errorf("second Marshal() differs from the first:\n%s\nwant\n%s", second, text)
	}
}

func TestHistoryEntryUnknownKeysListedInSortedOrder(t *testing.T) {
	r, err := Parse("x.md", openFile(askEntry, futureExtras, byOnDecide))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.History[0].UnknownKeys(); len(got) != 0 {
		t.Errorf("UnknownKeys() = %v, want none", got)
	}
	if got, want := r.History[1].UnknownKeys(), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("UnknownKeys() = %v, want %v", got, want)
	}
	if got, want := r.History[2].UnknownKeys(), []string{"by"}; !reflect.DeepEqual(got, want) {
		t.Errorf("UnknownKeys() = %v, want %v", got, want)
	}
}

// Two entries that differ only in an unknown key are not equal: renumber's
// stale-copy match compares entries with reflect.DeepEqual (S44).
func TestEntriesDifferingOnlyInAnUnknownKeyAreNotEqual(t *testing.T) {
	a, err := Parse("a.md", openFile(askEntry, `{"date":"2026-10-03","event":"decide","by":"one"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Parse("b.md", openFile(askEntry, `{"date":"2026-10-03","event":"decide","by":"two"}`))
	if err != nil {
		t.Fatal(err)
	}
	c, err := Parse("c.md", openFile(askEntry, `{"date":"2026-10-03","event":"decide"}`))
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(a.History[1], b.History[1]) || reflect.DeepEqual(a.History[1], c.History[1]) {
		t.Error("entries with different unknown keys compare equal")
	}
	a2, _ := Parse("a2.md", openFile(askEntry, `{"date":"2026-10-03","event":"decide","by":"one"}`))
	if !reflect.DeepEqual(a.History[1], a2.History[1]) {
		t.Error("entries with the same unknown key compare unequal")
	}
	// An entry with no unknown key has none to compare, whatever its source.
	if !reflect.DeepEqual(c.History[1], HistoryEntry{Date: "2026-10-03", Event: EventDecide}) {
		t.Errorf("entry without unknown keys = %+v, want a plain HistoryEntry", c.History[1])
	}
}

// Only a history entry is tolerant: an unknown top-level key still fails
// (owner decision 2026-10-19), and so does one inside was.
func TestUnknownTopLevelAndWasKeysStillFail(t *testing.T) {
	top := []byte("---\n{\"id\":4,\"title\":\"T\",\"status\":\"open\",\"created\":\"2026-10-01\",\"colour\":\"red\"}\n---\n")
	if _, err := Parse("top.md", top); err == nil || !strings.Contains(err.Error(), `unknown field "colour"`) {
		t.Errorf("Parse(unknown top-level key) = %v, want the unknown field error", err)
	}
	was := openFile(`{"date":"2026-10-02","event":"amend","was":{"title":"Old","colour":"red"}}`)
	if _, err := Parse("was.md", was); err == nil || !strings.Contains(err.Error(), `unknown field "colour"`) {
		t.Errorf("Parse(unknown key in was) = %v, want the unknown field error", err)
	}
}

// A key that spells a known one in another case is that key, as before.
func TestHistoryKeysMatchKnownKeysIgnoringCase(t *testing.T) {
	r, err := Parse("x.md", openFile(`{"Date":"2026-10-01","EVENT":"ask","Note":"n"}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := (HistoryEntry{Date: "2026-10-01", Event: EventAsk, Note: "n"}); !reflect.DeepEqual(r.History[0], want) {
		t.Errorf("entry = %+v, want %+v (no unknown keys)", r.History[0], want)
	}
}

func TestValidateAndUnknownHistoryEvents(t *testing.T) {
	r := validRecord(StatusOpen)

	r.History = []HistoryEntry{{Date: "2026-10-01", Event: "future-thing", Note: "x"}}
	if err := r.Validate(); err != nil {
		t.Errorf("unknown event: Validate() = %v, want nil", err)
	}
	r.History = []HistoryEntry{{Date: "2026-10-01", Event: "future-thing", Was: &Was{Title: ptr("old")}}}
	if err := r.Validate(); err != nil {
		t.Errorf("unknown event with was: Validate() = %v, want nil", err)
	}

	for _, tt := range []struct {
		name  string
		entry HistoryEntry
		want  string
	}{
		{"blank event", HistoryEntry{Date: "2026-10-01"}, "entry 0: event is required"},
		{"whitespace event", HistoryEntry{Date: "2026-10-01", Event: " \t"}, "entry 0: event is required"},
		{"unknown event, bad date", HistoryEntry{Date: "soon", Event: "future-thing"}, "entry 0:"},
		{"amend without was", HistoryEntry{Date: "2026-10-01", Event: EventAmend}, "an amend entry needs the old text in was"},
		{"decide with was", HistoryEntry{Date: "2026-10-01", Event: EventDecide, Was: &Was{Title: ptr("old")}}, "only an amend entry has was"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := validRecord(StatusOpen)
			r.History = []HistoryEntry{tt.entry}
			err := r.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

// A record with an unknown event and unknown keys can still be changed: the
// transitions keep the old entries as they were and add their own (S44).
func TestTransitionsKeepUnknownHistoryEntries(t *testing.T) {
	const day = "2026-11-01"
	tests := []struct {
		name  string
		apply func(*Record) error
		event Event
	}{
		{"decide", func(r *Record) error { return Decide(r, "yes", "because", "owner", day) }, EventDecide},
		{"reject", func(r *Record) error { return Reject(r, "no", "owner", day) }, EventReject},
		{"withdraw", func(r *Record) error { return Withdraw(r, "no", "owner", day) }, EventWithdraw},
		{"amend", func(r *Record) error { return Amend(r, Amendment{Title: ptr("Logging, corrected")}, day) }, EventAmend},
		{"renumber", func(r *Record) error { return Renumber(r, 9, day) }, EventRenumber},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Unmarshal("0004-logging.md", openFile(askEntry, byOnDecide, futureEntry, futureExtras))
			if err != nil {
				t.Fatal(err)
			}
			before := append([]HistoryEntry(nil), r.History...)
			if err := tt.apply(&r); err != nil {
				t.Fatalf("transition = %v, want nil", err)
			}
			if len(r.History) != 5 || r.History[4].Event != tt.event {
				t.Fatalf("History = %+v, want the four old entries and a %s entry", r.History, tt.event)
			}
			if !reflect.DeepEqual(r.History[:4], before) {
				t.Errorf("old entries = %+v, want them unchanged: %+v", r.History[:4], before)
			}
			data, err := Marshal(r)
			if err != nil {
				t.Fatalf("Marshal() = %v, want nil", err)
			}
			for _, want := range []string{`"by": "someone"`, `"event": "future-thing"`, `"alpha": true`} {
				if !strings.Contains(string(data), want) {
					t.Errorf("file lost %s:\n%s", want, data)
				}
			}
			back, err := Unmarshal("x.md", data)
			if err != nil || !reflect.DeepEqual(back, r) {
				t.Errorf("read back = %+v, %v; want the record written", back, err)
			}
		})
	}
}

// A file keeps "<", ">" and "&" as typed, in an entry's note and in an
// unknown key, as it does everywhere else in the record (C25).
func TestHistoryEntryDoesNotEscapeHTMLCharacters(t *testing.T) {
	entry := `{"date":"2026-10-02","event":"future-thing","note":"a < b & c > d","tag":"<x>&"}`
	r, err := Unmarshal("0004-logging.md", openFile(askEntry, entry))
	if err != nil {
		t.Fatal(err)
	}
	data, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"note": "a < b & c > d"`, `"tag": "<x>&"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("Marshal() changed %s:\n%s", want, data)
		}
	}
}

// A JSON null where an entry should be is the zero entry, which Validate
// refuses for its date, as before.
func TestNullHistoryEntryIsTheZeroEntry(t *testing.T) {
	r, err := Parse("x.md", openFile(askEntry, `null`))
	if err != nil {
		t.Fatalf("Parse() = %v, want nil", err)
	}
	if r.History[1] != (HistoryEntry{}) {
		t.Errorf("entry = %+v, want the zero entry", r.History[1])
	}
	if err := r.Validate(); err == nil {
		t.Error("Validate() = nil, want an error for the zero entry")
	}
}

const (
	wasTags    = `{"date":"2026-10-05","event":"future-thing","was":{"tags":["x"]}}`
	wasText    = `{"date":"2026-10-06","event":"other-thing","was":"text"}`
	wasOptions = `{"date":"2026-10-07","event":"third-thing","was":{"options":[{"label":""}]}}`
)

// An unknown event keeps its was as it is, whatever shape it has: a newer
// version may give it other keys or another type. It is not decoded, so the
// was rules do not apply, and it round-trips unchanged (S44, C213).
func TestUnknownEventKeepsAnyWasRaw(t *testing.T) {
	r, err := Unmarshal("0004-logging.md", openFile(askEntry, wasTags, wasText, wasOptions))
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	for i, h := range r.History[1:] {
		if h.Was != nil {
			t.Errorf("entry %d: Was = %+v, want nil (kept raw)", i+1, h.Was)
		}
		if keys := h.UnknownKeys(); len(keys) != 0 {
			t.Errorf("entry %d: UnknownKeys() = %v, want none: was is a known key name", i+1, keys)
		}
	}
	first, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"tags": [`, `"was": "text"`, `"options": [`} {
		if !strings.Contains(string(first), want) {
			t.Errorf("Marshal() lost %s:\n%s", want, first)
		}
	}
	again, err := Unmarshal("0004-logging.md", first)
	if err != nil || !reflect.DeepEqual(again, r) {
		t.Fatalf("read back = %+v, %v; want the record written", again, err)
	}
	second, _ := Marshal(again)
	if string(second) != string(first) {
		t.Errorf("second Marshal() differs from the first:\n%s\nwant\n%s", second, first)
	}
	// The raw was is part of equality.
	other, err := Parse("o.md", openFile(askEntry, strings.Replace(wasTags, `"x"`, `"y"`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(other.History[1], r.History[1]) {
		t.Error("entries with different raw was compare equal")
	}
}

// A record whose unknown entries carry such a was can still be changed.
func TestTransitionsKeepAnUnknownEventsWas(t *testing.T) {
	r, err := Unmarshal("0004-logging.md", openFile(askEntry, wasTags, wasText, wasOptions))
	if err != nil {
		t.Fatal(err)
	}
	before := append([]HistoryEntry(nil), r.History...)
	if err := Decide(&r, "yes", "because", "owner", "2026-11-01"); err != nil {
		t.Fatalf("Decide() = %v", err)
	}
	if err := Amend(&r, Amendment{Title: ptr("Logging, corrected")}, "2026-11-02"); err != nil {
		t.Fatalf("Amend() = %v", err)
	}
	if !reflect.DeepEqual(r.History[:4], before) {
		t.Errorf("old entries = %+v, want %+v", r.History[:4], before)
	}
	data, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"tags": [`, `"was": "text"`, `"options": [`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("file lost %s:\n%s", want, data)
		}
	}
}

// For a known event the was stays strict: a new key in an amend's was is an
// error, as is a was on a decide entry.
func TestKnownEventsKeepWasStrict(t *testing.T) {
	amend := openFile(`{"date":"2026-10-02","event":"amend","was":{"title":"Old","tags":["x"]}}`)
	if _, err := Parse("x.md", amend); err == nil || !strings.Contains(err.Error(), `unknown field "tags"`) {
		t.Errorf("Parse(amend with an unknown was key) = %v, want the unknown field error", err)
	}
	decide := openFile(`{"date":"2026-10-02","event":"decide","was":"text"}`)
	if _, err := Parse("x.md", decide); err == nil {
		t.Error("Parse(decide with a non-object was) = nil, want an error")
	}
}

// Keys that differ from a known key only in letter case: the exact one is the
// known key, the others are unknown keys that are kept and reported.
func TestKeysFoldingToOneKnownKeyAreNotDropped(t *testing.T) {
	tests := []struct {
		name     string
		entry    string
		note     string
		event    Event
		unknown  []string
		contains []string
	}{
		{"exact note wins", `{"date":"2026-10-01","event":"ask","Note":"b","note":"a"}`, "a", EventAsk,
			[]string{"Note"}, []string{`"note": "a"`, `"Note": "b"`}},
		{"exact event wins", `{"date":"2026-10-01","EVENT":"decide","event":"ask"}`, "", EventAsk,
			[]string{"EVENT"}, []string{`"event": "ask"`, `"EVENT": "decide"`}},
		{"a key repeated as spelled: the last wins", `{"date":"2026-10-01","event":"ask","note":"a","note":"b"}`, "b", EventAsk,
			nil, []string{`"note": "b"`}},
		{"none exact: the last wins, the other is kept", `{"date":"2026-10-01","event":"ask","NOTE":"x","Note":"y"}`, "y", EventAsk,
			[]string{"NOTE"}, []string{`"note": "y"`, `"NOTE": "x"`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Unmarshal("x.md", openFile(tt.entry))
			if err != nil {
				t.Fatal(err)
			}
			h := r.History[0]
			if h.Note != tt.note || h.Event != tt.event || !reflect.DeepEqual(h.UnknownKeys(), tt.unknown) {
				t.Errorf("entry = %+v with unknown keys %v; want note %q, event %q, unknown keys %v", h, h.UnknownKeys(), tt.note, tt.event, tt.unknown)
			}
			first, err := Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.contains {
				if !strings.Contains(string(first), want) {
					t.Errorf("Marshal() lost %s:\n%s", want, first)
				}
			}
			if n := strings.Count(string(first), `"note"`); n > 1 {
				t.Errorf("Marshal() wrote note %d times:\n%s", n, first)
			}
			again, err := Unmarshal("x.md", first)
			if err != nil || !reflect.DeepEqual(again, r) {
				t.Fatalf("read back = %+v, %v; want the record written", again, err)
			}
			if second, _ := Marshal(again); string(second) != string(first) {
				t.Errorf("second Marshal() differs:\n%s\nwant\n%s", second, first)
			}
		})
	}
}

// A history entry that does not decode gives an error that names the history
// entry, not the types that decode it.
func TestHistoryDecodeErrorsDoNotLeakInternalTypes(t *testing.T) {
	tests := []struct {
		name  string
		entry string
	}{
		{"not an object", `7`},
		{"an array", `[]`},
		{"a string", `"x"`},
		{"date of the wrong type", `{"date":5,"event":"ask"}`},
		{"was of the wrong type on an amend", `{"date":"2026-10-01","event":"amend","was":"text"}`},
		{"was field of the wrong type", `{"date":"2026-10-01","event":"amend","was":{"title":5}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("x.md", openFile(tt.entry))
			if err == nil {
				t.Fatal("Parse() = nil, want an error")
			}
			msg := err.Error()
			if !strings.Contains(msg, "history entry") {
				t.Errorf("error %q does not name the history entry", msg)
			}
			for _, leak := range []string{"knownEntry", "map[", "jsontext", "RawMessage", "Go value of type", "field ."} {
				if strings.Contains(msg, leak) {
					t.Errorf("error %q mentions %q", msg, leak)
				}
			}
		})
	}
}
