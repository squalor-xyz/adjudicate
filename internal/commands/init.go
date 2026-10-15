// SPDX-License-Identifier: GPL-3.0-or-later

// Package commands holds what each adj command does to a repo's store, apart
// from argument parsing, output and exit codes, which belong to package cli.
package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/squalor-xyz/adjudicate/internal/store"
)

// defaultConfig is what Init writes: the store key spelled out, so the owner
// sees the one setting there is (C56).
const defaultConfig = "{\n  \"store\": \"" + store.DefaultStore + "\"\n}\n"

// Init makes root a repo root: it writes the config file there if there is
// none, and creates the store directory the config names. root must be an
// existing directory. An existing config is kept byte for byte (it must be
// valid) and existing records are never touched, so a second Init changes
// nothing. It reports whether it wrote the config.
func Init(root string) (wroteConfig bool, err error) {
	fi, err := os.Stat(root)
	if err != nil {
		return false, err
	}
	if !fi.IsDir() {
		return false, fmt.Errorf("%s: not a directory", root)
	}
	path := filepath.Join(root, store.ConfigFile)
	switch fi, err := os.Stat(path); {
	case err == nil && !fi.Mode().IsRegular():
		return false, fmt.Errorf("%s: not a regular file", path)
	case err == nil:
		// Keep the owner's config.
	case os.IsNotExist(err):
		if err := writeNewFile(path, defaultConfig); err != nil {
			return false, err
		}
		wroteConfig = true
	default:
		return false, err
	}
	cfg, err := store.LoadConfig(root)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Join(root, cfg.Store), 0o755); err != nil {
		if wroteConfig {
			os.Remove(path) // never leave behind a config this run wrote
		}
		return false, err
	}
	return wroteConfig, nil
}

// writeNewFile creates path with mode 0644 and fails if it exists.
func writeNewFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(content)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(path)
	}
	return err
}
