// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// recordWith is the text of an open record file whose frontmatter holds the
// given extra members (JSON, no braces), after the known ones (S46).
func recordWith(extra string) []byte {
	return []byte(`---
{
  "id": 4,
  "title": "Logging",
  "status": "open",
  "created": "2026-10-01",
  "history": [{"date":"2026-10-01","event":"ask"}],
  ` + extra + `
}
---
Body text.
`)
}

const futureTop = `"zeta": {"a": [1, 2]}, "alpha": true, "mid": "<x & y>"`

// A record's unknown top-level keys are kept, written back after the known
// keys in sorted order, and the bytes do not change on another round trip.
func TestRecordKeepsUnknownTopLevelKeysAcrossRoundTrips(t *testing.T) {
	r, err := Unmarshal("0004-logging.md", recordWith(futureTop))
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	if got, want := r.UnknownKeys(), []string{"alpha", "mid", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("UnknownKeys() = %v, want %v", got, want)
	}
	first, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(first)
	// Known keys first, in their usual order, then the unknown ones sorted.
	last := -1
	for _, want := range []string{`"history": [`, `"alpha": true`, `"mid": "<x & y>"`, `"zeta": {`} {
		i := strings.Index(text, want)
		if i <= last {
			t.Errorf("%s is missing or not after the previous key in:\n%s", want, text)
		}
		last = i
	}
	if !strings.HasSuffix(text, "---\nBody text.\n") {
		t.Errorf("Marshal() lost the body:\n%s", text)
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

// A record with no unknown key has none, whatever else it holds.
func TestRecordWithoutUnknownKeysHasNone(t *testing.T) {
	r, err := Parse("x.md", recordWith(`"key": "k"`))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.UnknownKeys(); len(got) != 0 {
		t.Errorf("UnknownKeys() = %v, want none", got)
	}
	if got := r.UnknownJSON(); got != "" {
		t.Errorf("UnknownJSON() = %q, want empty", got)
	}
}

// Records that differ only in an unknown key are not equal: renumber compares
// with reflect.DeepEqual and the type stays comparable that way (S44, S46).
func TestRecordsDifferingOnlyInAnUnknownKeyAreNotEqual(t *testing.T) {
	parse := func(extra string) Record {
		r, err := Parse("x.md", recordWith(extra))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	a, a2, b, c := parse(`"x": 1`), parse(`"x": 1`), parse(`"x": 2`), parse(`"y": 1`)
	if !reflect.DeepEqual(a, a2) {
		t.Error("records with the same unknown key compare unequal")
	}
	if reflect.DeepEqual(a, b) || reflect.DeepEqual(a, c) {
		t.Error("records with different unknown keys compare equal")
	}
}

// Keys that differ from a known key only in letter case follow the S44 rule:
// the exact one is the known key, the others are unknown keys that are kept.
// A body key is not a known key (the body is the text after the frontmatter)
// and is kept like any other.
func TestRecordKeysFoldingToOneKnownKeyAreNotDropped(t *testing.T) {
	tests := []struct {
		name     string
		extra    string
		key      string
		unknown  []string
		contains []string
	}{
		{"exact key wins", `"Key": "b", "key": "a"`, "a", []string{"Key"}, []string{`"key": "a"`, `"Key": "b"`}},
		{"only another case: that is the key", `"KEY": "a"`, "a", nil, []string{`"key": "a"`}},
		{"none exact: the last wins, the other is kept", `"KEY": "x", "Key": "y"`, "y", []string{"KEY"}, []string{`"key": "y"`, `"KEY": "x"`}},
		{"a key repeated as spelled: the last wins", `"key": "a", "key": "b"`, "b", nil, []string{`"key": "b"`}},
		{"body", `"body": "text", "Body": 5`, "", []string{"Body", "body"}, []string{`"body": "text"`, `"Body": 5`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Unmarshal("x.md", recordWith(tt.extra))
			if err != nil {
				t.Fatal(err)
			}
			if r.Key != tt.key || !reflect.DeepEqual(r.UnknownKeys(), tt.unknown) {
				t.Errorf("key %q, unknown keys %v; want key %q, unknown keys %v", r.Key, r.UnknownKeys(), tt.key, tt.unknown)
			}
			if r.Body != "Body text.\n" {
				t.Errorf("Body = %q, want the text after the frontmatter", r.Body)
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
			if n := strings.Count(string(first), `"key"`); n > 1 {
				t.Errorf("Marshal() wrote key %d times:\n%s", n, first)
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

// Known keys stay strict about type, and so do the nested records that are not
// tolerant: an option and a recommendation (only the top level, the history
// entry and was are).
func TestKnownRecordKeysStayStrict(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"id of the wrong type", `---
{"id":"4","title":"T","status":"open","created":"2026-10-01","colour":"red"}
---
`, "id"},
		{"tags of the wrong type", `---
{"id":4,"title":"T","status":"open","created":"2026-10-01","tags":"x","colour":"red"}
---
`, "tags"},
		{"unknown key in an option", `---
{"id":4,"title":"T","status":"open","created":"2026-10-01","options":[{"label":"a","colour":"red"}]}
---
`, `unknown field "colour"`},
		{"unknown key in a recommendation", `---
{"id":4,"title":"T","status":"open","created":"2026-10-01","recommendation":{"text":"a","colour":"red"}}
---
`, `unknown field "colour"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("x.md", []byte(tt.data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Parse() = %v, want an error mentioning %q", err, tt.want)
			}
			if err != nil {
				for _, leak := range []string{"recordJSON", "map[", "RawMessage"} {
					if strings.Contains(err.Error(), leak) {
						t.Errorf("error %q mentions %q", err, leak)
					}
				}
			}
		})
	}
	if _, err := Parse("x.md", []byte("---\n[]\n---\n")); err == nil {
		t.Error("Parse(frontmatter that is an array) = nil, want an error")
	}
}

const (
	amendWas = `{"date":"2026-10-02","event":"amend","note":"title, summary a","was":{` +
		`"zeta":{"k":[1]},"title":"Old","alpha":"<a>",` +
		`"options":[{"url":"u","label":"a","summary":"Old A","weight":2},{"label":"b","tradeoffs":"Old B"}]}}`
	amendFuture = `{"date":"2026-10-03","event":"amend","was":{"key":"old-key","tags":["x"]}}`
)

// A known amend entry's was keeps keys this version does not know, at its top
// and inside each options entry, and writes them back after the known keys in
// sorted order; a was that has only such keys is still an old text.
func TestWasKeepsUnknownKeysAcrossRoundTrips(t *testing.T) {
	data := openFile(askEntry, amendWas, amendFuture)
	r, err := Unmarshal("0004-logging.md", data)
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	w := r.History[1].Was
	if w == nil || w.Title == nil || *w.Title != "Old" || len(w.Options) != 2 {
		t.Fatalf("Was = %+v, want the known title and two options", w)
	}
	if got, want := w.UnknownKeys(), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Was.UnknownKeys() = %v, want %v", got, want)
	}
	if got, want := w.Options[0].UnknownKeys(), []string{"url", "weight"}; !reflect.DeepEqual(got, want) {
		t.Errorf("OptionWas.UnknownKeys() = %v, want %v", got, want)
	}
	if got := w.Options[1].UnknownKeys(); len(got) != 0 {
		t.Errorf("second OptionWas.UnknownKeys() = %v, want none", got)
	}
	if got := r.History[1].UnknownKeys(); len(got) != 0 {
		t.Errorf("entry UnknownKeys() = %v, want none: the keys are in was", got)
	}
	if got, want := r.History[2].Was.UnknownKeys(), []string{"key", "tags"}; !reflect.DeepEqual(got, want) {
		t.Errorf("only-unknown Was.UnknownKeys() = %v, want %v", got, want)
	}

	first, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	text := string(first)
	// Known keys first, then unknown ones sorted: at the top of was ...
	last := -1
	for _, want := range []string{`"title": "Old"`, `"options": [`, `"alpha": "<a>"`, `"zeta": {`} {
		i := strings.Index(text, want)
		if i <= last {
			t.Errorf("%s is missing or not after the previous key in:\n%s", want, text)
		}
		last = i
	}
	// ... and in an option: label, summary, tradeoffs, then url and weight.
	if i, j, k := strings.Index(text, `"summary": "Old A"`), strings.Index(text, `"url": "u"`), strings.Index(text, `"weight": 2`); i < 0 || j < i || k < j {
		t.Errorf("option keys are not label, summary, url, weight in:\n%s", text)
	}
	again, err := Unmarshal("0004-logging.md", first)
	if err != nil {
		t.Fatalf("second Unmarshal() = %v", err)
	}
	if !reflect.DeepEqual(again, r) {
		t.Errorf("parse, marshal, parse gave\n %+v\nwant\n %+v", again, r)
	}
	if second, _ := Marshal(again); string(second) != text {
		t.Errorf("second Marshal() differs from the first:\n%s\nwant\n%s", second, text)
	}
}

// Two was texts that differ only in an unknown key are not equal.
func TestWasDifferingOnlyInAnUnknownKeyAreNotEqual(t *testing.T) {
	parse := func(was string) HistoryEntry {
		r, err := Parse("x.md", openFile(askEntry, `{"date":"2026-10-02","event":"amend","was":`+was+`}`))
		if err != nil {
			t.Fatal(err)
		}
		return r.History[1]
	}
	a, a2 := parse(`{"title":"T","x":1}`), parse(`{"title":"T","x":1}`)
	if !reflect.DeepEqual(a, a2) {
		t.Error("was with the same unknown key compare unequal")
	}
	for _, other := range []string{`{"title":"T","x":2}`, `{"title":"T"}`, `{"title":"T","y":1}`} {
		if reflect.DeepEqual(a, parse(other)) {
			t.Errorf("was %s compares equal to one with another unknown key", other)
		}
	}
	o1 := parse(`{"options":[{"label":"a","summary":"S","x":1}]}`)
	for _, other := range []string{`{"options":[{"label":"a","summary":"S","x":2}]}`, `{"options":[{"label":"a","summary":"S"}]}`} {
		if reflect.DeepEqual(o1, parse(other)) {
			t.Errorf("was %s compares equal to one with another unknown option key", other)
		}
	}
}

// Keys of a was and of its options that fold to a known key follow the S44
// rule too.
func TestWasKeysFoldingToOneKnownKeyAreNotDropped(t *testing.T) {
	r, err := Unmarshal("x.md", openFile(askEntry,
		`{"date":"2026-10-02","event":"amend","was":{"Title":"b","title":"a","OPTIONS":[{"Label":"L","SUMMARY":"s","summary":"t"}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	w := r.History[1].Was
	if *w.Title != "a" || !reflect.DeepEqual(w.UnknownKeys(), []string{"Title"}) {
		t.Errorf("Was = %+v with unknown keys %v; want title a and unknown key Title", w, w.UnknownKeys())
	}
	if o := w.Options[0]; o.Label != "L" || *o.Summary != "t" || !reflect.DeepEqual(o.UnknownKeys(), []string{"SUMMARY"}) {
		t.Errorf("OptionWas = %+v with unknown keys %v; want label L, summary t, unknown key SUMMARY", o, o.UnknownKeys())
	}
	first, err := Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"title": "a"`, `"Title": "b"`, `"summary": "t"`, `"SUMMARY": "s"`} {
		if !strings.Contains(string(first), want) {
			t.Errorf("Marshal() lost %s:\n%s", want, first)
		}
	}
	again, err := Unmarshal("x.md", first)
	if err != nil || !reflect.DeepEqual(again, r) {
		t.Fatalf("read back = %+v, %v; want the record written", again, err)
	}
}

// Known keys of was stay strict about type; an amend entry's was must be an
// object, and a decide entry has none.
func TestWasKnownKeysStayStrict(t *testing.T) {
	tests := []struct {
		name  string
		entry string
	}{
		{"title of the wrong type", `{"date":"2026-10-02","event":"amend","was":{"title":5,"x":1}}`},
		{"options of the wrong type", `{"date":"2026-10-02","event":"amend","was":{"options":"a","x":1}}`},
		{"option summary of the wrong type", `{"date":"2026-10-02","event":"amend","was":{"options":[{"label":"a","summary":5,"x":1}]}}`},
		{"option label of the wrong type", `{"date":"2026-10-02","event":"amend","was":{"options":[{"label":5}]}}`},
		{"was that is not an object", `{"date":"2026-10-02","event":"amend","was":"text"}`},
		{"was on a decide entry", `{"date":"2026-10-02","event":"decide","was":"text"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse("x.md", openFile(tt.entry))
			if err == nil {
				t.Fatal("Parse() = nil, want an error")
			}
			if !strings.Contains(err.Error(), "history entry") {
				t.Errorf("error %q does not name the history entry", err)
			}
			for _, leak := range []string{"wasJSON", "optionWasJSON", "knownEntry", "map[", "RawMessage", "Go value of type", "field ."} {
				if strings.Contains(err.Error(), leak) {
					t.Errorf("error %q mentions %q", err, leak)
				}
			}
		})
	}
}

// A was or an option's old text that has only keys this version does not know
// is still an old text: a newer version may amend a field this one lacks, and
// the entry must not make this version refuse the record.
func TestValidateAcceptsAWasOfUnknownKeysOnly(t *testing.T) {
	_, err := Unmarshal("x.md", openFile(askEntry, amendFuture,
		`{"date":"2026-10-04","event":"amend","was":{"options":[{"label":"a","colour":"red"}]}}`))
	// Option a does not exist, but Validate does not look at the options of
	// the record here: it asks that an old option text has some text.
	if err != nil {
		t.Errorf("Unmarshal() = %v, want nil", err)
	}
	// An empty was and an option with no old text of any kind stay errors.
	for _, entry := range []string{
		`{"date":"2026-10-02","event":"amend","was":{}}`,
		`{"date":"2026-10-02","event":"amend","was":{"options":[{"label":"a"}]}}`,
	} {
		if _, err := Unmarshal("x.md", openFile(askEntry, entry)); err == nil {
			t.Errorf("Unmarshal(%s) = nil, want an error", entry)
		}
	}
}

// Every command that rewrites a record in the model (decide, reject, withdraw,
// amend, supersede, renumber) keeps the unknown parts as they were.
func TestTransitionsKeepUnknownTopLevelAndWasKeys(t *testing.T) {
	const want1, want2 = `"alpha": true`, `"zeta": {`
	open := func() Record {
		r, err := Unmarshal("x.md", recordWithHistory(`"options":[{"label":"a"}],`+futureTop))
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	steps := []struct {
		name string
		do   func(*Record) error
	}{
		{"decide", func(r *Record) error { return Decide(r, "a", "because", "owner", "2026-11-01") }},
		{"reject", func(r *Record) error { return Reject(r, "no", "owner", "2026-11-01", Provenance{}) }},
		{"withdraw", func(r *Record) error { return Withdraw(r, "no", "owner", "2026-11-01", Provenance{}) }},
		{"amend", func(r *Record) error { return Amend(r, Amendment{Title: ptr("Logging, corrected")}, "2026-11-02") }},
		{"renumber", func(r *Record) error { return Renumber(r, 9, "2026-11-02") }},
	}
	for _, tt := range steps {
		t.Run(tt.name, func(t *testing.T) {
			r := open()
			if err := tt.do(&r); err != nil {
				t.Fatal(err)
			}
			data, err := Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{want1, want2, `"mid": "<x & y>"`} {
				if !strings.Contains(string(data), want) {
					t.Errorf("file lost %s:\n%s", want, data)
				}
			}
			again, err := Unmarshal("x.md", data)
			if err != nil || !reflect.DeepEqual(again.UnknownKeys(), []string{"alpha", "mid", "zeta"}) {
				t.Errorf("read back unknown keys = %v, %v; want alpha, mid, zeta", again.UnknownKeys(), err)
			}
		})
	}

	t.Run("supersede", func(t *testing.T) {
		old, err := Unmarshal("x.md", []byte(`---
{"id":4,"title":"Logging","status":"decided","created":"2026-10-01","decided_at":"2026-10-02","decision":"d","rationale":"r",`+
			`"history":[`+askEntry+`],`+futureTop+`}
---
`))
		if err != nil {
			t.Fatal(err)
		}
		if err := Supersede(&old, 9, "2026-11-02"); err != nil {
			t.Fatal(err)
		}
		data, err := Marshal(old)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{want1, want2} {
			if !strings.Contains(string(data), want) {
				t.Errorf("file lost %s:\n%s", want, data)
			}
		}
	})

	t.Run("amend keeps the was keys of older entries", func(t *testing.T) {
		r, err := Unmarshal("x.md", openFile(askEntry, amendWas, amendFuture))
		if err != nil {
			t.Fatal(err)
		}
		before := slices.Clone(r.History)
		if err := Amend(&r, Amendment{Title: ptr("Logging, corrected")}, "2026-11-02"); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(r.History[:3], before) {
			t.Errorf("old entries = %+v, want %+v", r.History[:3], before)
		}
		data, err := Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"zeta": {`, `"url": "u"`, `"weight": 2`, `"tags": [`, `"key": "old-key"`} {
			if !strings.Contains(string(data), want) {
				t.Errorf("file lost %s:\n%s", want, data)
			}
		}
	})
}

// recordWithHistory is an open record file with the ask entry and the given
// extra members after the known ones; members may set the known keys too.
func recordWithHistory(members string) []byte {
	return []byte(`---
{"id":4,"title":"Logging","status":"open","created":"2026-10-01","history":[` + askEntry + `],` + members + `}
---
`)
}

// The JSON of a record that the readers print holds the unknown keys beside
// the known ones, and a body key from the frontmatter never repeats the body.
func TestMarshalJSONAndJSONWithBody(t *testing.T) {
	r, err := Parse("x.md", recordWith(`"zeta": 1, "alpha": [true]`))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(plain), `,"alpha":[true],"zeta":1}`) || strings.Contains(string(plain), "Body text") {
		t.Errorf("json.Marshal(r) = %s, want the known keys, then alpha and zeta, and no body", plain)
	}
	with, err := r.MarshalJSONWithBody()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(with), `,"alpha":[true],"zeta":1,"body":"Body text.\n"}`) {
		t.Errorf("MarshalJSONWithBody() = %s, want the unknown keys, then the body", with)
	}

	clash, err := Parse("x.md", recordWith(`"body": "from the frontmatter", "alpha": 1`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := clash.MarshalJSONWithBody()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(got), `"body"`); n != 1 || !strings.Contains(string(got), `"body":"Body text.\n"`) {
		t.Errorf("MarshalJSONWithBody() = %s, want one body key holding the body", got)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Errorf("MarshalJSONWithBody() is not JSON: %v", err)
	}
	// With no body there is no body key at all: body means the text after the
	// frontmatter, so a frontmatter key of that name is left out (C235).
	clash.Body = ""
	got, _ = clash.MarshalJSONWithBody()
	if strings.Contains(string(got), `"body"`) || !strings.Contains(string(got), `"alpha":1`) {
		t.Errorf("MarshalJSONWithBody() without a body = %s, want no body key and the other unknown keys", got)
	}
	// The file keeps the key on a rewrite.
	if file, err := Marshal(clash); err != nil || !strings.Contains(string(file), `"body": "from the frontmatter"`) {
		t.Errorf("Marshal() = %s, %v; want the frontmatter body key kept", file, err)
	}
}

// The lists of known keys that the decoders use are the JSON names of the
// structs, so a field added to one cannot be treated as an unknown key.
func TestKnownKeyListsMatchTheStructs(t *testing.T) {
	names := func(v any) []string {
		var out []string
		typ := reflect.TypeOf(v)
		for i := 0; i < typ.NumField(); i++ {
			tag := typ.Field(i).Tag.Get("json")
			if tag == "-" || !typ.Field(i).IsExported() {
				continue
			}
			out = append(out, strings.Split(tag, ",")[0])
		}
		return out
	}
	for _, tt := range []struct {
		name string
		got  []string
		v    any
	}{
		{"recordKeys", recordKeys, Record{}},
		{"wasKeys", wasKeys, Was{}},
		{"optionWasKeys", optionWasKeys, OptionWas{}},
		{"entryKeys", entryKeys, HistoryEntry{}},
	} {
		if want := names(tt.v); !reflect.DeepEqual(slices.Sorted(slices.Values(tt.got)), slices.Sorted(slices.Values(want))) {
			t.Errorf("%s = %v, want the JSON names %v", tt.name, tt.got, want)
		}
	}
}
