// SPDX-License-Identifier: GPL-3.0-or-later

// Command adj is the adjudicate command line.
package main

import (
	"os"

	"github.com/squalor-xyz/adjudicate/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
