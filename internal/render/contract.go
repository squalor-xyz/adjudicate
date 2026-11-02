// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"fmt"
	"io"

	"github.com/squalor-xyz/adjudicate/internal/contract"
)

// ContractText writes the contract this binary speaks, "adj-json v1".
func ContractText(w io.Writer) error {
	_, err := fmt.Fprintln(w, contract.String())
	return err
}

// contractJSON is the object adj contract --json prints (S45): the contract
// as text, its number, and the repo's pin, null when it has none.
type contractJSON struct {
	Contract string `json:"contract"`
	Version  int    `json:"version"`
	Pinned   *int   `json:"pinned"`
}

// ContractJSON writes {"contract", "version", "pinned"}.
func ContractJSON(w io.Writer, pinned *int) error {
	return encodeJSON(w, contractJSON{Contract: contract.String(), Version: contract.Version, Pinned: pinned})
}

// versionJSON is the object adj version --json prints (S45).
type versionJSON struct {
	Version  string `json:"version"`
	Contract string `json:"contract"`
}

// VersionJSON writes {"version", "contract"}: the build's version text and the
// contract it speaks.
func VersionJSON(w io.Writer, version string) error {
	return encodeJSON(w, versionJSON{Version: version, Contract: contract.String()})
}
