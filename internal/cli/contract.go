// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/contract"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

const contractUsage = `usage: adj contract [--json] [--root DIR]
       adj contract pin M [--root DIR]
       adj contract unpin [--root DIR]

Print the version of the JSON contract this adj speaks, "adj-json v1": the
revision of the shapes in docs/json.md. With --json, print {"contract",
"version", "pinned"}, where pinned is the version this repo's .adjudicate.json
pins, or null.

A repo may pin the contract version it was written for. adj never serves an
older shape: when the pin is not the version this adj speaks, show, list,
inbox, check and context --format json (and inbox --all --json, for that repo)
refuse with code contract_mismatch, commands without JSON work as before, and
adj lint warns (contract-pin).

  pin M      write "contract": M into .adjudicate.json, keeping its other
             keys; M is a whole number of at least 1. A root with no config
             gets one, and the store directory, as adj init would. Without
             --root the repo is found from the working directory upwards
  unpin      remove the pin; a repo without one is left as it is
  --json     print JSON instead of text (not with pin or unpin)
  --root DIR the repo root; default: found from the working directory
             upwards. Printing the version needs no repo
`

func (c *invocation) runContract(args []string) int {
	fs, root := newFlagSet("contract")
	asJSON := fs.Bool("json", false, "")
	pos, code, done := c.parse("contract", contractUsage, fs, args)
	if done {
		return code
	}
	if len(pos) == 0 {
		return c.contractShow(*root, *asJSON)
	}
	switch pos[0] {
	case "pin", "unpin":
		if fsSet(fs, "json") {
			return c.usageError("contract", contractUsage, fmt.Errorf("%s takes no --json", pos[0]))
		}
	default:
		return c.usageError("contract", contractUsage, fmt.Errorf("unknown subcommand %q", pos[0]))
	}
	if pos[0] == "unpin" {
		if len(pos) > 1 {
			return c.usageError("contract", contractUsage, fmt.Errorf("unpin takes no arguments, got %q", pos[1]))
		}
		dir, err := c.rootFor(*root, false)
		if err != nil {
			return c.rootError("contract", err)
		}
		if err := commands.ContractUnpin(dir); err != nil {
			return c.fail("contract", err)
		}
		return exitOK
	}
	if len(pos) != 2 {
		return c.usageError("contract", contractUsage, fmt.Errorf("pin takes one version, got %d arguments", len(pos)-1))
	}
	n, err := parseContractVersion(pos[1])
	if err != nil {
		return c.usageError("contract", contractUsage, err)
	}
	// A root that was named is used as it is, and gets a config if it has
	// none; otherwise the repo must already exist, as for every other command.
	named := *root != "" || c.env.Root != ""
	dir, err := c.rootFor(*root, named)
	if err != nil {
		return c.rootError("contract", err)
	}
	if err := commands.ContractPin(dir, n); err != nil {
		return c.commandError("contract", contractUsage, err)
	}
	return exitOK
}

// contractShow prints the contract this adj speaks and, with --json, the
// repo's pin. It needs no repo: with none found the pin is null.
func (c *invocation) contractShow(flagRoot string, asJSON bool) int {
	var pinned *int
	dir, err := c.rootFor(flagRoot, false)
	switch {
	case errors.Is(err, store.ErrNoRoot):
	case err != nil:
		return c.fail("contract", err)
	case asJSON:
		if pinned, err = commands.ContractPinned(dir); err != nil {
			return c.fail("contract", err)
		}
	}
	if asJSON {
		err = render.ContractJSON(c.stdout, pinned)
	} else {
		err = render.ContractText(c.stdout)
	}
	if err != nil {
		return c.fail("contract", err)
	}
	return exitOK
}

// parseContractVersion reads the M of adj contract pin M: a whole number of at
// least 1, written in digits alone.
func parseContractVersion(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || strconv.Itoa(n) != s {
		return 0, fmt.Errorf("the contract version must be a whole number of at least 1, such as %d; got %q", contract.Version, s)
	}
	return n, nil
}

// guardContract is the one guard in front of every command with JSON output:
// a repo that pins a contract version other than the one this adj speaks gets
// no JSON, because adj never serves an older shape (S45). It runs once the
// store is open and before any record is read. In text mode (wantJSON false)
// it lets the command through. When it refuses, it has reported the failure
// (exit 1, code contract_mismatch) and refused is true.
func (c *invocation) guardContract(name string, st *store.Store, wantJSON bool) (code int, refused bool) {
	if !wantJSON {
		return 0, false
	}
	if err := contract.Check(st.Contract()); err != nil {
		return c.fail(name, err), true
	}
	return 0, false
}
