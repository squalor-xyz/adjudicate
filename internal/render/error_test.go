// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"bytes"
	"testing"
)

func TestErrorJSON(t *testing.T) {
	tests := []struct {
		name, code, message, command, want string
	}{
		{"a command", "not_found", "record not found: id 9", "show",
			"{\n  \"error\": {\n    \"code\": \"not_found\",\n    \"message\": \"record not found: id 9\",\n    \"command\": \"show\"\n  }\n}\n"},
		{"no command is null", "usage", "unknown command \"x\"", "",
			"{\n  \"error\": {\n    \"code\": \"usage\",\n    \"message\": \"unknown command \\\"x\\\"\",\n    \"command\": null\n  }\n}\n"},
		{"HTML is not escaped", "io", "a <b> & c", "list",
			"{\n  \"error\": {\n    \"code\": \"io\",\n    \"message\": \"a <b> & c\",\n    \"command\": \"list\"\n  }\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := ErrorJSON(&buf, tt.code, tt.message, tt.command); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
