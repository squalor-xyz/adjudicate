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
	"strings"
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

	// Contract is the JSON contract version this repo pins (S45), or nil for
	// no pin. In the file it is optional and a positive whole number.
	Contract *int `json:"contract,omitempty"`
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
	return parseConfig(path, data)
}

// parseConfig decodes and checks the content of the config file at path.
func parseConfig(path string, data []byte) (Config, error) {
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
	if cfg.Contract != nil && *cfg.Contract < 1 {
		return Config{}, fmt.Errorf("%s: %w: contract %d must be a positive whole number", path, ErrInvalidConfig, *cfg.Contract)
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

// SetContract pins the JSON contract version in root's config file, or removes
// the pin when contract is nil (S45). The file must exist and be valid; it is
// never created here. Only the keys the file has are written back, `store`
// first and `contract` after it, indented as adj init writes them, so the
// store path stays as typed. A file that already has the wanted pin is left
// byte for byte as it is (changed is false). The write goes through a
// temporary file and a rename, keeps the file's permission bits, and goes
// through a symbolic link to the file that the link names.
func SetContract(root string, contract *int) (changed bool, err error) {
	if contract != nil && *contract < 1 {
		return false, fmt.Errorf("contract %d must be a positive whole number", *contract)
	}
	path := filepath.Join(root, ConfigFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	cfg, err := parseConfig(path, data)
	if err != nil {
		return false, err
	}
	if (cfg.Contract == nil) == (contract == nil) && (contract == nil || *cfg.Contract == *contract) {
		return false, nil
	}
	// parseConfig accepted the file, so it is one object and its store, if
	// any, is a string.
	var raw struct {
		Store *string `json:"store"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("%s: %w: %w", path, ErrInvalidConfig, err)
	}
	var fields []string
	if raw.Store != nil {
		fields = append(fields, `  "store": `+jsonString(*raw.Store))
	}
	if contract != nil {
		fields = append(fields, fmt.Sprintf(`  "contract": %d`, *contract))
	}
	out := "{}\n"
	if len(fields) > 0 {
		out = "{\n" + strings.Join(fields, ",\n") + "\n}\n"
	}
	if err := writeFileAtomic(path, []byte(out)); err != nil {
		return false, err
	}
	return true, nil
}

// jsonString is s as a JSON string, with HTML characters unescaped.
func jsonString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return strings.TrimSuffix(b.String(), "\n")
}

// renameFile is os.Rename; a test replaces it to make the last step fail.
var renameFile = os.Rename

// writeFileAtomic replaces the file at path with data through a temporary file
// in the same directory and a rename, keeping the file's permission bits. A
// symbolic link is followed, so the link stays and the file it names changes.
func writeFileAtomic(path string, data []byte) (err error) {
	if target, err := filepath.EvalSymlinks(path); err == nil {
		path = target
	}
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".adjudicate-*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	if err = f.Chmod(fi.Mode().Perm()); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	err = renameFile(f.Name(), path)
	return err
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
