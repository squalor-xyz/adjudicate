// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/squalor-xyz/adjudicate/internal/store"
)

// ContractPinned returns the JSON contract version that the repo at root pins
// in its config (S45), or nil when it pins none. An invalid config is an
// error matching store.ErrInvalidConfig.
func ContractPinned(root string) (*int, error) {
	cfg, err := store.LoadConfig(root)
	if err != nil {
		return nil, err
	}
	return cfg.Contract, nil
}

// ContractPin pins version n (at least 1) in the config of the repo at root,
// keeping its other keys. A root with no config file is set up as adj init
// would: the config, with the pin, and the store directory. A config that is
// not valid is an error and is left as it is. Pinning the pinned version
// changes nothing.
func ContractPin(root string, n int) error {
	if n < 1 {
		return usagef("contract version must be a whole number of at least 1; got %d", n)
	}
	path := filepath.Join(root, store.ConfigFile)
	wrote := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		var err error
		if wrote, err = Init(root); err != nil {
			return err
		}
	}
	if _, err := store.SetContract(root, &n); err != nil {
		if wrote {
			os.Remove(path) // never leave behind a config this run wrote
		}
		return err
	}
	return nil
}

// ContractUnpin removes the pin from the config of the repo at root. The
// config must exist; one with no pin is left byte for byte as it is.
func ContractUnpin(root string) error {
	_, err := store.SetContract(root, nil)
	return err
}
