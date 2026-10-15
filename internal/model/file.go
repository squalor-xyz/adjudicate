// SPDX-License-Identifier: GPL-3.0-or-later

package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// A record file is a "---" line, the record as indented JSON (encoding/json,
// C25), a "---" line, then the Markdown body verbatim (C48, C49). encoding/json
// never writes a raw newline inside a string, so the closing fence is the
// first later line that is exactly "---".

// fence is the line that opens and closes the JSON frontmatter (C48).
const fence = "---"

// bom is the UTF-8 byte order mark. A record file must not start with it (C48).
const bom = "\xef\xbb\xbf"

// errCRLF is why a file whose frontmatter uses CRLF line endings is rejected:
// LF only (C48). The body is kept verbatim and may contain CR (C49).
const errCRLF = "CRLF line endings are not supported (LF only)"

// Marshal encodes r as a record file. It validates r first and returns an
// error matching ErrInvalid, writing nothing, if r breaks the model's rules
// (C50). The output always ends with a newline; a non-empty body that lacks
// one gets one (C52).
func Marshal(r Record) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(fence + "\n")
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	buf.WriteString(fence + "\n")
	buf.WriteString(r.Body)
	if r.Body != "" && !strings.HasSuffix(r.Body, "\n") {
		buf.WriteByte('\n')
	}
	return buf.Bytes(), nil
}

// Unmarshal decodes a record file and validates the record (C50). name is
// used in error messages. Every error matches ErrInvalid (C53).
func Unmarshal(name string, data []byte) (Record, error) {
	r, err := Parse(name, data)
	if err != nil {
		return Record{}, err
	}
	if err := r.Validate(); err != nil {
		return Record{}, fmt.Errorf("%s: %w", name, err)
	}
	return r, nil
}

// Parse decodes a record file without validating the record (C51), so lint
// can report a record that Validate rejects. A missing fence, a byte order
// mark, CRLF line endings, a JSON syntax or type error, an unknown key or data
// after the JSON are still errors, matching ErrInvalid (C53). The JSON error
// stays reachable with errors.As.
func Parse(name string, data []byte) (Record, error) {
	front, body, err := splitFile(data)
	if err != nil {
		return Record{}, fmt.Errorf("%s: %w: %v", name, ErrInvalid, err)
	}
	var r Record
	dec := json.NewDecoder(bytes.NewReader(front))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		if errors.Is(err, io.EOF) {
			err = errors.New("frontmatter is empty")
		}
		return Record{}, fmt.Errorf("%s: %w: frontmatter: %w", name, ErrInvalid, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Record{}, fmt.Errorf("%s: %w: frontmatter: data after the JSON object", name, ErrInvalid)
	}
	r.Body = string(body)
	return r, nil
}

// splitFile returns the frontmatter (the bytes between the fences) and the
// body (everything after the closing fence line, verbatim).
func splitFile(data []byte) (front, body []byte, err error) {
	if bytes.HasPrefix(data, []byte(bom)) {
		return nil, nil, errors.New("file starts with a byte order mark")
	}
	open := fence + "\n"
	if bytes.HasPrefix(data, []byte(fence+"\r\n")) {
		return nil, nil, errors.New(errCRLF)
	}
	if !bytes.HasPrefix(data, []byte(open)) {
		return nil, nil, errors.New("no opening --- line")
	}
	rest := data[len(open):]
	for pos := 0; pos < len(rest); {
		end, next := len(rest), len(rest)
		if i := bytes.IndexByte(rest[pos:], '\n'); i >= 0 {
			end, next = pos+i, pos+i+1
		}
		// A line ending in CR is CRLF. Only frontmatter lines reach this check;
		// the body is kept verbatim.
		if bytes.HasSuffix(rest[pos:end], []byte("\r")) {
			return nil, nil, errors.New(errCRLF)
		}
		if string(rest[pos:end]) == fence {
			return rest[:pos], rest[next:], nil
		}
		pos = next
	}
	return nil, nil, errors.New("frontmatter is not closed by a --- line")
}

// ReadFile reads and validates the record file at path.
func ReadFile(path string) (Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	return Unmarshal(path, data)
}

// WriteFile validates r and writes it to path, replacing any file there (C54).
// It does not create directories.
func WriteFile(path string, r Record) error {
	data, err := Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
