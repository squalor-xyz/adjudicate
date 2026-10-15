// SPDX-License-Identifier: GPL-3.0-or-later

// Package cli dispatches the adj command line. It takes its I/O as
// parameters so tests can run it in-process.
package cli

import (
	"fmt"
	"io"
)

// Exit codes shared by every adj command (C32).
const (
	exitOK    = 0 // success
	exitFail  = 1 // a failed operation or a failed check
	exitUsage = 2 // a usage error
)

const usage = `usage: adj <command> [arguments]

adjudicate records project decisions as plain text.
No commands are implemented yet.
`

// Run executes one adj invocation and returns the process exit code. args
// excludes the program name. Help goes to stdout with exit 0; a missing or
// unknown command prints usage to stderr with exit 2.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "--help", "-h":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "adj: unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}
