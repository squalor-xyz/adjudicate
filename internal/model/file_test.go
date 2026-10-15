// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fullRecord sets every field, with a multi-line Markdown body that contains
// lines looking like the frontmatter fence and a closing brace.
func fullRecord() Record {
	return Record{
		ID:        3,
		Title:     "Store format",
		Status:    StatusSuperseded,
		Key:       "store.format",
		Tags:      []string{"storage", "format"},
		AskedBy:   "agent",
		DecidedBy: "owner",
		Created:   "2026-10-01",
		DecidedAt: "2026-10-02",
		Context:   "Records must diff cleanly in a pull request; --- inside a value is not a fence.",
		Options: []Option{
			{Label: "toml", Summary: "TOML frontmatter", Tradeoffs: "no standard library parser"},
			{Label: "json", Summary: "JSON frontmatter", Tradeoffs: "less pleasant to hand edit"},
		},
		Recommendation: &Recommendation{Label: "json", Reasoning: "encoding/json is in the standard library"},
		Decision:       "json",
		Rationale:      "The standard library has no TOML parser.",
		Consequences:   "Records are read and written with encoding/json.",
		Supersedes:     []int{1, 2},
		SupersededBy:   7,
		Refs:           []string{"S04", "PR 12"},
		History: []HistoryEntry{
			{Date: "2026-10-01", Event: EventAsk},
			{Date: "2026-10-02", Event: EventDecide, Note: "owner"},
			{Date: "2026-10-07", Event: EventSupersede, Note: "by 7"},
		},
		Body: "## Notes\n\nSee the table.\n\n---\n\n}\n",
	}
}

func TestRoundTripKeepsEveryField(t *testing.T) {
	type roundTripCase struct {
		name string
		rec  Record
	}
	tests := []roundTripCase{{"fully populated", fullRecord()}}
	for _, s := range allStatuses {
		tests = append(tests, roundTripCase{"minimal " + string(s), validRecord(s)})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Marshal(tt.rec)
			if err != nil {
				t.Fatalf("Marshal() = %v, want nil", err)
			}
			got, err := Unmarshal("record.md", data)
			if err != nil {
				t.Fatalf("Unmarshal() = %v, want nil", err)
			}
			if !reflect.DeepEqual(got, tt.rec) {
				t.Fatalf("round trip changed the record\n got %+v\nwant %+v", got, tt.rec)
			}
		})
	}
}

func TestMarshalIsDeterministicAndMatchesGolden(t *testing.T) {
	first, err := Marshal(fullRecord())
	if err != nil {
		t.Fatalf("Marshal() = %v, want nil", err)
	}
	second, err := Marshal(fullRecord())
	if err != nil {
		t.Fatalf("Marshal() = %v, want nil", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two Marshal calls on the same record differ")
	}
	if !bytes.HasSuffix(first, []byte("\n")) || bytes.HasSuffix(first, []byte("\n\n")) {
		t.Fatalf("output must end with exactly one newline, ends with %q", first[max(0, len(first)-4):])
	}
	golden, err := os.ReadFile(filepath.Join("testdata", "full.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, golden) {
		t.Fatalf("Marshal output does not match testdata/full.md\n got:\n%s\nwant:\n%s", first, golden)
	}
}

func TestMarshalMinimalRecordOmitsEmptyOptionalFields(t *testing.T) {
	data, err := Marshal(validRecord(StatusOpen))
	if err != nil {
		t.Fatalf("Marshal() = %v, want nil", err)
	}
	want := "---\n" +
		"{\n" +
		"  \"id\": 1,\n" +
		"  \"title\": \"Pick a store\",\n" +
		"  \"status\": \"open\",\n" +
		"  \"created\": \"2026-10-01\"\n" +
		"}\n" +
		"---\n"
	if string(data) != want {
		t.Fatalf("minimal record\n got %q\nwant %q", data, want)
	}
}

func TestMarshalOutputIsReadable(t *testing.T) {
	r := validRecord(StatusOpen)
	r.Title = "Use <this> & that"
	r.Context = "a > b && c < d"
	data, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() = %v, want nil", err)
	}
	for _, want := range []string{"Use <this> & that", "a > b && c < d", "\n  \"title\": "} {
		if !strings.Contains(string(data), want) {
			t.Errorf("output does not contain %q:\n%s", want, data)
		}
	}
	if strings.Contains(string(data), `\u`) {
		t.Errorf("output has escaped characters:\n%s", data)
	}
}

func TestBodyRoundTripsVerbatim(t *testing.T) {
	bodies := []string{
		"",
		"Plain text.\n",
		"Line one\n---\nLine two\n",
		"}\n---\n}\n",
		"---\n---\n",
		"# Heading\n\n```json\n{\"a\": 1}\n```\n",
		"\n\nleading and trailing blank lines\n\n",
	}
	for i, body := range bodies {
		t.Run(fmt.Sprintf("body %d", i), func(t *testing.T) {
			r := validRecord(StatusOpen)
			r.Body = body
			data, err := Marshal(r)
			if err != nil {
				t.Fatalf("Marshal() = %v, want nil", err)
			}
			got, err := Unmarshal("body.md", data)
			if err != nil {
				t.Fatalf("Unmarshal() = %v, want nil", err)
			}
			if got.Body != body {
				t.Fatalf("body changed\n got %q\nwant %q", got.Body, body)
			}
		})
	}
}

func TestMarshalAddsFinalNewlineToBody(t *testing.T) {
	r := validRecord(StatusOpen)
	r.Body = "no newline"
	data, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() = %v, want nil", err)
	}
	if !bytes.HasSuffix(data, []byte("no newline\n")) {
		t.Fatalf("output %q does not end with the body plus one newline", data)
	}
	got, err := Unmarshal("body.md", data)
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	if got.Body != "no newline\n" {
		t.Fatalf("body = %q, want %q", got.Body, "no newline\n")
	}
}

func TestGoldenFileDecodesToFullRecord(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "full.md"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unmarshal("full.md", data)
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, fullRecord()) {
		t.Fatalf("testdata/full.md decodes to\n got %+v\nwant %+v", got, fullRecord())
	}
}

func TestMalformedFilesReturnErrorNamingFile(t *testing.T) {
	tests := []struct {
		file   string
		reason string // the error must also say what is wrong
	}{
		{"no_opening.md", "no opening --- line"},
		{"unterminated.md", "not closed by a --- line"},
		{"bad_json.md", "invalid character"},
		{"unknown_key.md", `unknown field "kind"`},
		{"invalid_record.md", "rationale"},
		{"bad_history_date.md", "history"},
		{"bad_history_event.md", "history"},
		{"opening_fence_text.md", "no opening --- line"},
		{"trailing_json.md", "data after the JSON object"},
		{"empty_frontmatter.md", "frontmatter is empty"},
		{"type_error.md", "cannot unmarshal string"},
		{"bom.md", "file starts with a byte order mark"},
		{"crlf.md", "CRLF line endings are not supported (LF only)"},
		{"crlf_mixed.md", "CRLF line endings are not supported (LF only)"},
		{"crlf_fence.md", "CRLF line endings are not supported (LF only)"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "malformed", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			_, err = Unmarshal(tt.file, data)
			if err == nil {
				t.Fatal("Unmarshal() = nil, want error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("error %v does not match ErrInvalid", err)
			}
			if !strings.Contains(err.Error(), tt.file) {
				t.Errorf("error %q does not name the file %q", err, tt.file)
			}
			if !strings.Contains(err.Error(), tt.reason) {
				t.Errorf("error %q does not contain %q", err, tt.reason)
			}
		})
	}
}

func TestParseKeepsInvalidRecordForLint(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "malformed", "invalid_record.md"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse("invalid_record.md", data)
	if err != nil {
		t.Fatalf("Parse() = %v, want the record without Validate", err)
	}
	if got.ID != 2 || got.Status != StatusDecided || got.Decision != "keep it" {
		t.Fatalf("Parse() = %+v, want the id, status and decision from the file", got)
	}
	if _, err := Unmarshal("invalid_record.md", data); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Unmarshal() = %v, want an ErrInvalid error", err)
	}

	bad, err := os.ReadFile(filepath.Join("testdata", "malformed", "bad_json.md"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse("bad_json.md", bad); err == nil {
		t.Fatal("Parse() of invalid JSON = nil, want error")
	}
}

func TestWriteFileThenReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "3-store-format.md")
	if err := WriteFile(path, fullRecord()); err != nil {
		t.Fatalf("WriteFile() = %v, want nil", err)
	}
	got, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, fullRecord()) {
		t.Fatalf("ReadFile() = %+v, want the written record", got)
	}
}

func TestWriteFileRefusesInvalidRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.md")
	err := WriteFile(path, Record{})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("WriteFile() = %v, want an ErrInvalid error", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Fatal("WriteFile created a file for an invalid record")
	}
}

func TestReadFileMissingFileIsError(t *testing.T) {
	if _, err := ReadFile(filepath.Join(t.TempDir(), "absent.md")); err == nil {
		t.Fatal("ReadFile() of a missing file = nil, want error")
	}
}

func TestReadFileRejectsInvalidRecord(t *testing.T) {
	path := filepath.Join("testdata", "malformed", "invalid_record.md")
	_, err := ReadFile(path)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("ReadFile() = %v, want an ErrInvalid error", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("ReadFile() error %q does not name the path %q", err, path)
	}
}

// unmarshalFixture returns the error Unmarshal gives for a malformed fixture.
func unmarshalFixture(t *testing.T, file string) error {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "malformed", file))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Unmarshal(file, data)
	if err == nil {
		t.Fatalf("Unmarshal(%s) = nil, want error", file)
	}
	return err
}

func TestJSONErrorsAreReachableWithErrorsAs(t *testing.T) {
	t.Run("syntax error", func(t *testing.T) {
		err := unmarshalFixture(t, "bad_json.md")
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Fatalf("errors.As(%v, *json.SyntaxError) = false", err)
		}
	})
	t.Run("type error", func(t *testing.T) {
		err := unmarshalFixture(t, "type_error.md")
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("errors.As(%v, *json.UnmarshalTypeError) = false", err)
		}
	})
}

func TestClosingFenceAtEndOfFileHasEmptyBody(t *testing.T) {
	// No newline after the closing fence: the file ends right at "---".
	data := []byte("---\n{\n  \"id\": 1,\n  \"title\": \"Pick a store\",\n  \"status\": \"open\",\n  \"created\": \"2026-10-01\"\n}\n---")
	got, err := Unmarshal("eof.md", data)
	if err != nil {
		t.Fatalf("Unmarshal() = %v, want nil", err)
	}
	if want := validRecord(StatusOpen); !reflect.DeepEqual(got, want) {
		t.Fatalf("Unmarshal() = %+v, want %+v with an empty body", got, want)
	}
}
