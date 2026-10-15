// SPDX-License-Identifier: GPL-3.0-or-later

// Package cli dispatches the adj command line. It takes its I/O as
// parameters so tests can run it in-process.
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

const usage = `usage: adj <command> [arguments]

adjudicate records project decisions as plain text.

Commands:
  adj init   create the config file and the store directory
  adj ask    record an open question with options and a recommendation

Run adj <command> --help for a command's flags.
`

const initUsage = `usage: adj init [--root DIR]

Create the config file (.adjudicate.json) and the store directory in DIR, or
in the working directory. Safe to run again: an existing config and the
records in the store are left as they are.

  --root DIR   the repo root; default: the working directory
`

const askUsage = `usage: adj ask TITLE [flags]

Record an open question. Prints the new record's ID.

  --option LABEL:SUMMARY    an answer on offer; repeat for each option
  --tradeoffs LABEL:TEXT    the trade-offs of the option LABEL; once per option
  --recommend LABEL         the option you recommend
  --reasoning TEXT          why you recommend it (needs --recommend)
  --key KEY                 the decision key, such as storage.engine
  --tag TAG                 a tag; repeat for several
  --context TEXT            why the question is asked
  --stdin                   read the context from standard input instead
  --by NAME                 who asks; default: $ADJ_ASKED_BY, else left blank
  --root DIR                the repo root; default: found from the working
                            directory upwards (the nearest .adjudicate.json)

The flags may come before or after TITLE. LABEL:SUMMARY and LABEL:TEXT split
at the first colon, so the text may hold colons.
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
// excludes the program name. Help goes to stdout with exit 0; a missing or
// unknown command prints usage to stderr with exit 2.
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
	switch args[0] {
	case "--help", "-h":
		fmt.Fprint(stdout, usage)
		return exitOK
	case "init":
		return c.runInit(args[1:])
	case "ask":
		return c.runAsk(args[1:])
	default:
		fmt.Fprintf(stderr, "adj: unknown command %q\n", args[0])
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
}

// invocation is one Run: the environment and the streams.
type invocation struct {
	env            Env
	stdin          io.Reader
	stdout, stderr io.Writer
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

func (c *invocation) runInit(args []string) int {
	fs, root := newFlagSet("init")
	pos, code, done := c.parse("init", initUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("init", initUsage, fmt.Errorf("unexpected argument %q", pos[0]))
	}
	dir, err := c.rootFor(*root, true)
	if err != nil {
		return c.rootError("init", err)
	}
	wrote, err := commands.Init(dir)
	if err != nil {
		return c.fail("init", err)
	}
	if wrote {
		fmt.Fprintf(c.stdout, "initialized %s\n", dir)
	} else {
		fmt.Fprintf(c.stdout, "already initialized %s\n", dir)
	}
	return exitOK
}

func (c *invocation) runAsk(args []string) int {
	fs, root := newFlagSet("ask")
	var options, tradeoffs, tags stringList
	fs.Var(&options, "option", "")
	fs.Var(&tradeoffs, "tradeoffs", "")
	fs.Var(&tags, "tag", "")
	recommend := fs.String("recommend", "", "")
	reasoning := fs.String("reasoning", "", "")
	key := fs.String("key", "", "")
	context := fs.String("context", "", "")
	fromStdin := fs.Bool("stdin", false, "")
	by := fs.String("by", "", "")

	pos, code, done := c.parse("ask", askUsage, fs, args)
	if done {
		return code
	}
	switch len(pos) {
	case 0:
		return c.usageError("ask", askUsage, errors.New("TITLE is required"))
	case 1:
	default:
		return c.usageError("ask", askUsage, fmt.Errorf("want one TITLE, got %d arguments (quote a title that has spaces)", len(pos)))
	}
	in := commands.AskInput{
		Title:     pos[0],
		Key:       *key,
		Tags:      tags,
		Context:   *context,
		AskedBy:   *by,
		Options:   options,
		Tradeoffs: tradeoffs,
		Recommend: *recommend,
		Reasoning: *reasoning,
	}
	if in.AskedBy == "" {
		in.AskedBy = c.env.Getenv("ADJ_ASKED_BY")
	}
	if *fromStdin && fsSet(fs, "context") {
		return c.usageError("ask", askUsage, errors.New("--context and --stdin are exclusive"))
	}
	if err := in.Check(); err != nil {
		return c.askError(err)
	}

	dir, err := c.rootFor(*root, false)
	if err != nil {
		return c.rootError("ask", err)
	}
	st, err := store.Open(dir)
	if err != nil {
		return c.fail("ask", err)
	}
	// Standard input is read last, so a usage error or a missing init fails
	// before anything is consumed.
	if *fromStdin {
		data, err := io.ReadAll(c.stdin)
		if err != nil {
			return c.fail("ask", err)
		}
		in.Context = strings.TrimRight(string(data), "\r\n")
	}
	rec, err := commands.Ask(st, in, c.env.Now())
	if err != nil {
		return c.askError(err)
	}
	fmt.Fprintf(c.stdout, "%d\n", rec.ID)
	return exitOK
}

// askError maps an error from the ask command to its exit code.
func (c *invocation) askError(err error) int {
	var ue *commands.UsageError
	if errors.As(err, &ue) {
		return c.usageError("ask", askUsage, err)
	}
	return c.fail("ask", err)
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
