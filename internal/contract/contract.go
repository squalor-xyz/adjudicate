// SPDX-License-Identifier: GPL-3.0-or-later

// Package contract holds the version of adj's JSON contract (S45): the number
// that says which revision of the shapes in docs/json.md this binary speaks.
// A repo may pin a number in .adjudicate.json; adj refuses the JSON of a repo
// whose pin is not this number, and never serves an older shape.
package contract

import (
	"errors"
	"fmt"
)

const (
	// Name is the contract's name. Slicer's relay contract is a different
	// contract with its own number.
	Name = "adj-json"

	// Version is the contract version this binary speaks. It moves on any
	// change to a shape pinned by internal/cli/jsonshape_test.go that is not
	// purely additive (TestContractShapesAreTiedToTheVersion), and docs/json.md
	// says in "Contract version" what each version changed.
	Version = 1
)

// ErrMismatch is matched by the error Check returns for a pin that is not
// Version. The JSON error object calls it contract_mismatch.
var ErrMismatch = errors.New("contract mismatch")

// String returns the contract and its version, as adj contract prints it:
// "adj-json v1".
func String() string { return fmt.Sprintf("%s v%d", Name, Version) }

// Check returns nil when a repo that pins pin (nil: no pin) may be given this
// binary's JSON, and otherwise an error matching ErrMismatch that names both
// versions and the two ways out. It is the one guard that every command with
// JSON output goes through.
func Check(pin *int) error {
	if pin == nil || *pin == Version {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrMismatch, Detail(*pin))
}

// Detail says what a pin of pin means for this binary, for the refusal and for
// the lint warning: both versions and the two ways out, which are the owner's
// to take (a pin is repo config).
func Detail(pin int) string {
	return fmt.Sprintf("this repo pins %s v%d but this adj speaks %s, so its JSON commands refuse this repo; ask the owner to run `adj contract pin %d` or to install an adj that speaks %s v%d",
		Name, pin, String(), Version, Name, pin)
}
