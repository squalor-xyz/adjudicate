// SPDX-License-Identifier: GPL-3.0-or-later

package render

import "io"

// errorJSON is the object a failed --json command prints (S42, docs/json.md).
type errorJSON struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Command *string `json:"command"`
}

// ErrorJSON writes {"error": {"code", "message", "command"}} with the same
// encoder settings as the other JSON. An empty command is written as null: the
// failure came before a command was known.
func ErrorJSON(w io.Writer, code, message, command string) error {
	body := errorBody{Code: code, Message: message}
	if command != "" {
		body.Command = &command
	}
	return encodeJSON(w, errorJSON{Error: body})
}
