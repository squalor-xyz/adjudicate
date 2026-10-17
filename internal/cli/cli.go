// SPDX-License-Identifier: GPL-3.0-or-later

// Package cli dispatches the adj command line. It takes its I/O as
// parameters so tests can run it in-process. Each command's handler is in
// its own file: init.go, ask.go, decide.go, supersede.go, amend.go, show.go, list.go,
// inbox.go, check.go, context.go, lint.go, renumber.go, ai.go.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// Exit codes shared by every adj command (C34).
const (
	exitOK    = 0 // success
	exitFail  = 1 // a failed operation or a failed check
	exitUsage = 2 // a usage error
)

const usage = `usage: adj [--root DIR] <command> [arguments]

adjudicate records project decisions as plain text.

Commands:
  adj init      create the config file and the store directory
  adj ask       record an open question with options and a recommendation
  adj decide    answer an open question
  adj reject    close an open question without an answer
  adj withdraw  close an open question because no answer is needed
  adj supersede replace a decided record with another decided record
  adj amend     correct the text of a record, keeping the old text in its history
  adj show      print one record
  adj list      list records, optionally filtered
  adj inbox     list open questions, oldest first, with options
  adj check     find out whether a topic is decided, open or not found
  adj context   print the decided records as compact context for an agent
  adj lint      check the store for broken records, links, cycles and conflicts
  adj renumber  give records that share an ID new IDs
  adj ai        print an AGENTS.md snippet that tells agents how to use adj

Run adj <command> --help for a command's flags. --root DIR may come before the
command or after it, but not both; it is the only flag accepted before it.
`

// Env is what Run takes from the process. Tests replace it; the zero value
// means the real clock, environment and working directory.
type Env struct {
	// Root, if not empty, is the repo root used when a command is not given
	// --root. It is used as it is: nothing is searched for. Tests set it to a
	// temp dir so no command can find the repo that holds the test.
	Root string

	// Stop, if not empty, is the highest directory the search for the root
	// from the working directory may reach (store.FindRootWithin).
	Stop string

	Now    func() time.Time       // default: time.Now
	Getenv func(string) string    // default: os.Getenv
	Getwd  func() (string, error) // default: os.Getwd
}

// Run executes one adj invocation and returns the process exit code. args
// excludes the program name. A leading --root DIR is taken before dispatch and
// given to the command as if it followed the command name. Help goes to
// stdout with exit 0; a missing or unknown command prints usage to stderr with
// exit 2.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return RunWith(Env{}, args, stdin, stdout, stderr)
}

// RunWith is Run with the clock, environment variables, working directory and
// repo root taken from env.
func RunWith(env Env, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Getenv == nil {
		env.Getenv = os.Getenv
	}
	if env.Getwd == nil {
		env.Getwd = os.Getwd
	}
	c := &invocation{env: env, stdin: stdin, stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	if root, rest, ok, err := leadingRoot(args); err != nil {
		fmt.Fprintf(stderr, "adj: %v\n%s", err, usage)
		return exitUsage
	} else if ok {
		if len(rest) == 0 {
			fmt.Fprintf(stderr, "adj: --root %q needs a command\n%s", root, usage)
			return exitUsage
		}
		c.leadRoot, c.hasLeadRoot, args = root, true, rest
	}
	switch args[0] {
	case "--help", "-h":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "init":
		return c.runInit(args[1:])
	case "ask":
		return c.runAsk(args[1:])
	case "decide":
		return c.runDecide(args[1:])
	case "reject":
		return c.runReasonCommand("reject", rejectUsage, args[1:], commands.Reject)
	case "withdraw":
		return c.runReasonCommand("withdraw", withdrawUsage, args[1:], commands.Withdraw)
	case "supersede":
		return c.runSupersede(args[1:])
	case "amend":
		return c.runAmend(args[1:])
	case "show":
		return c.runShow(args[1:])
	case "list":
		return c.runList(args[1:])
	case "inbox":
		return c.runInbox(args[1:])
	case "check":
		return c.runCheck(args[1:])
	case "context":
		return c.runContext(args[1:])
	case "lint":
		return c.runLint(args[1:])
	case "renumber":
		return c.runRenumber(args[1:])
	case "ai":
		return c.runAI(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") && args[0] != "-" {
			fmt.Fprintf(stderr, "adj: unknown flag %q before the command; only --root, --help and -h are accepted there\n", args[0])
		} else {
			fmt.Fprintf(stderr, "adj: unknown command %q\n", args[0])
		}
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

// invocation is one Run: the environment and the streams.
type invocation struct {
	env            Env
	stdin          io.Reader
	stdout, stderr io.Writer

	// leadRoot is the value of a --root given before the command; it counts
	// only when hasLeadRoot is set (the value may be empty). parse hands it
	// to the command as if it had followed the command name.
	leadRoot    string
	hasLeadRoot bool
}

// leadingRoot takes a --root DIR, -root DIR, --root=DIR or -root=DIR from the
// front of args, the only flag accepted before the command. ok is false when
// args does not start with one. A bare --root takes the next argument as its
// value, whatever it looks like, as it does after a command. --root with no
// value, or given again before the command, is an error.
func leadingRoot(args []string) (root string, rest []string, ok bool, err error) {
	rest = args
	for len(rest) > 0 {
		name, value, hasValue := strings.Cut(rest[0], "=")
		if name != "--root" && name != "-root" {
			break
		}
		rest = rest[1:]
		if !hasValue {
			if len(rest) == 0 {
				return "", nil, false, errors.New("--root needs a value")
			}
			value, rest = rest[0], rest[1:]
		}
		if ok {
			return "", nil, false, fmt.Errorf("--root given twice before the command (%q and %q)", root, value)
		}
		root, ok = value, true
	}
	return root, rest, ok, nil
}

// newFlagSet returns a FlagSet that reports nothing itself: the command
// prints errors and usage in the shape C36 asks for. It has --root.
func newFlagSet(name string) (fs *flag.FlagSet, root *string) {
	fs = flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root = fs.String("root", "", "")
	return fs, root
}

// parse parses args for the command name. done is true when the command is
// over: help was asked (exit 0, usage on stdout) or the flags are wrong
// (exit 2, message and usage on stderr).
func (c *invocation) parse(name, usageText string, fs *flag.FlagSet, args []string) (pos []string, code int, done bool) {
	pos, err := parseInterspersed(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(c.stdout, usageText)
		return nil, exitOK, true
	}
	if err != nil {
		return nil, c.usageError(name, usageText, err), true
	}
	if c.hasLeadRoot {
		if fs.Lookup("root") == nil {
			return nil, c.usageError(name, usageText, fmt.Errorf("%s takes no --root", name)), true
		}
		if fsSet(fs, "root") {
			return nil, c.usageError(name, usageText, fmt.Errorf(
				"--root given twice: before the command (%q) and after it (%q)",
				c.leadRoot, fs.Lookup("root").Value.String())), true
		}
		if err := fs.Set("root", c.leadRoot); err != nil {
			return nil, c.usageError(name, usageText, err), true
		}
	}
	return pos, 0, false
}

// usageError prints "adj name: err" and the usage to stderr and returns the
// usage exit code.
func (c *invocation) usageError(name, usageText string, err error) int {
	fmt.Fprintf(c.stderr, "adj %s: %v\n%s", name, err, usageText)
	return exitUsage
}

// fail prints "adj name: err" to stderr and returns the failure exit code.
func (c *invocation) fail(name string, err error) int {
	fmt.Fprintf(c.stderr, "adj %s: %v\n", name, err)
	return exitFail
}

// commandError maps an error from a command's work on the store to its exit
// code: a *commands.UsageError is a usage error (exit 2), anything else a
// failed operation (exit 1).
func (c *invocation) commandError(name, usageText string, err error) int {
	var ue *commands.UsageError
	if errors.As(err, &ue) {
		return c.usageError(name, usageText, err)
	}
	return c.fail(name, err)
}

// rootFor returns the repo root for a command. An explicit root (the --root
// flag, else Env.Root) is used as it is: for a command other than init it
// must hold the config file. With none, init uses the working directory and
// the other commands search upwards from it. A repo that has not been
// initialised gives an error matching store.ErrNoRoot.
func (c *invocation) rootFor(flagRoot string, forInit bool) (string, error) {
	root := flagRoot
	if root == "" {
		root = c.env.Root
	}
	if root != "" {
		if !forInit {
			path := filepath.Join(root, store.ConfigFile)
			if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() {
				return "", fmt.Errorf("%w: no %s in %s", store.ErrNoRoot, store.ConfigFile, root)
			}
		}
		return root, nil
	}
	cwd, err := c.env.Getwd()
	if err != nil {
		return "", err
	}
	if forInit {
		return cwd, nil
	}
	return store.FindRootWithin(cwd, c.env.Stop)
}

// rootError reports a failure to find the root. A repo with no config tells
// the user to run adj init.
func (c *invocation) rootError(name string, err error) int {
	if errors.Is(err, store.ErrNoRoot) {
		return c.fail(name, fmt.Errorf("%v; run `adj init` first", err))
	}
	return c.fail(name, err)
}

// fsSet reports whether the flag name was given on the command line.
func fsSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}
