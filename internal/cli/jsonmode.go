// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"errors"
	"flag"
	"strconv"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
	"github.com/squalor-xyz/adjudicate/internal/userconfig"
)

// The error codes of the JSON error object (S42, docs/json.md).
const (
	codeUsage         = "usage"
	codeNoRepo        = "no_repo"
	codeInvalidConfig = "invalid_config"
	codeNotFound      = "not_found"
	codeDuplicateID   = "duplicate_id"
	codeInvalidRecord = "invalid_record"
	codeUserConfig    = "user_config"
	codeBudget        = "budget"
	codeIO            = "io"
)

// wantsJSON reports whether args ask for JSON output, read from the raw
// arguments, because a bad flag stops flag parsing before --json is seen. It
// follows parseInterspersed: "--" ends the flags, and a flag that takes a value
// owns the next argument, so `--key --json` is a key. JSON is asked for by
// --json, -json or --json=true (the last one given wins, so --json=false
// turns it off) when fs has a json flag, and by --format json or
// --format=json when fs has a format flag (context). A nil fs is a failure
// before a command is known: it takes --json, and --root as its only flag
// with a value.
func wantsJSON(fs *flag.FlagSet, args []string) bool {
	hasJSON := fs == nil || fs.Lookup("json") != nil
	hasFormat := fs != nil && fs.Lookup("format") != nil
	var jsonFlag, formatJSON bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		if len(a) < 2 || a[0] != '-' {
			continue
		}
		// The flag package takes one or two leading dashes; "---json" is a
		// bad flag, not --json.
		trimmed := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
		name, value, hasValue := strings.Cut(trimmed, "=")
		if strings.HasPrefix(name, "-") {
			continue
		}
		takes := !hasValue && flagTakesValue(fs, name)
		switch {
		case name == "json" && hasJSON:
			if !hasValue {
				jsonFlag = true
			} else {
				b, err := strconv.ParseBool(value)
				jsonFlag = err == nil && b
			}
		case name == "format" && hasFormat:
			if takes && i+1 < len(args) {
				value = args[i+1]
			}
			formatJSON = value == "json"
		}
		if takes && i+1 < len(args) {
			i++
		}
	}
	return jsonFlag || formatJSON
}

// flagTakesValue reports whether the flag called name owns the next argument
// when written without "=value". With no flag set only --root does.
func flagTakesValue(fs *flag.FlagSet, name string) bool {
	if fs == nil {
		return name == "root"
	}
	return takesValue(fs, name)
}

// errorCode names the failure err for the JSON error object. It reads the
// error chain, never its text. A usage error does not come through here: the
// callers that know it is one say so.
func errorCode(err error) string {
	switch {
	case errors.Is(err, store.ErrNoRoot):
		return codeNoRepo
	case errors.Is(err, store.ErrInvalidConfig):
		return codeInvalidConfig
	case errors.Is(err, store.ErrNotFound):
		return codeNotFound
	case errors.Is(err, store.ErrDuplicateID):
		return codeDuplicateID
	case errors.Is(err, model.ErrInvalid):
		return codeInvalidRecord
	case errors.Is(err, userconfig.ErrConfig):
		return codeUserConfig
	case errors.Is(err, commands.ErrBudget):
		return codeBudget
	}
	return codeIO
}

// jsonFailure prints the error object on stdout when the run is in JSON mode.
// command is "" for a failure before dispatch. The text on stderr is the
// caller's.
func (c *invocation) jsonFailure(command, code string, err error) {
	if !c.json {
		return
	}
	// A stdout that cannot be written to leaves nothing more to report.
	_ = render.ErrorJSON(c.stdout, code, err.Error(), command)
}
