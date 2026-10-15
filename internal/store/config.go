// SPDX-License-Identifier: GPL-3.0-or-later

// Package store reads and writes one repository's record files: it finds the
// repo root, loads the config, lists records in ID order and allocates the next
// ID by scanning the store (C26, C28, C29). It never looks at another repo.
package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// ConfigFile is the config file name. It sits at the repo root, and the
	// nearest ancestor that holds it is the root (FindRoot).
	ConfigFile = ".adjudicate.json"

	// DefaultStore is the store directory, relative to the root, when there is
	// no config file or it names no store (C29).
	DefaultStore = "decisions"
)

var (
	// ErrInvalidConfig is matched by every error LoadConfig returns for a
	// config file that cannot be used.
	ErrInvalidConfig = errors.New("invalid config")

	// ErrNoRoot is matched by the error FindRoot returns when no directory on
	// the way up holds a config file.
	ErrNoRoot = errors.New("no repo root found")
)

// Config is the content of ConfigFile. The file is one JSON object; keys not
// listed here are rejected so a typo does not pass silently.
type Config struct {
	// Store is the store directory relative to the root, with the default
	// applied. In the file it is optional.
	Store string `json:"store"`
}

// LoadConfig reads root/ConfigFile. A missing file is not an error: it gives
// the defaults. A file that is not one JSON object of known keys, or whose
// store is absolute or leaves the root, is an error matching ErrInvalidConfig.
func LoadConfig(root string) (Config, error) {
	path := filepath.Join(root, ConfigFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{Store: DefaultStore}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		if errors.Is(err, io.EOF) {
			err = errors.New("file is empty")
		}
		return Config{}, fmt.Errorf("%s: %w: %w", path, ErrInvalidConfig, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("%s: %w: data after the JSON object", path, ErrInvalidConfig)
	}
	if cfg.Store == "" {
		cfg.Store = DefaultStore
	}
	cfg.Store = filepath.Clean(filepath.FromSlash(cfg.Store))
	if !filepath.IsLocal(cfg.Store) {
		return Config{}, fmt.Errorf("%s: %w: store %q must be a path inside the repo root", path, ErrInvalidConfig, cfg.Store)
	}
	return cfg, nil
}

// FindRoot returns the repo root for a working directory: the nearest
// directory, start itself or an ancestor, that holds ConfigFile as a regular
// file. It returns an error matching ErrNoRoot if there is none.
func FindRoot(start string) (string, error) {
	return FindRootWithin(start, "")
}

// FindRootWithin is FindRoot with a limit: when stop is not empty the walk
// examines stop last and never goes above it, and a start that is not inside
// stop gives ErrNoRoot. Tests use it (through cli.Env.Stop) to keep the walk
// inside a temp directory.
func FindRootWithin(start, stop string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	if stop != "" {
		if stop, err = filepath.Abs(stop); err != nil {
			return "", err
		}
		if rel, err := filepath.Rel(stop, dir); err != nil || !filepath.IsLocal(rel) {
			return "", fmt.Errorf("%w: %s is not inside %s", ErrNoRoot, dir, stop)
		}
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, ConfigFile)); err == nil && fi.Mode().IsRegular() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if dir == stop || parent == dir {
			return "", fmt.Errorf("%w: no %s in %s or above", ErrNoRoot, ConfigFile, start)
		}
		dir = parent
	}
}
