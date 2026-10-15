// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import "fmt"

// UsageError is an error in the arguments a command was given, as opposed to
// a failed operation. The CLI maps it to exit code 2 (C34).
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

func usagef(format string, args ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, args...)}
}
