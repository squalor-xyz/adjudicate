// SPDX-License-Identifier: GPL-3.0-or-later

// Package userconfig manages the per-user repo list, independently of any repo.
package userconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/store"
)

// Config is the user config's complete JSON shape.
type Config struct {
	Repos []string `json:"repos"`
}

// ErrConfig matches every error that says the user config, or the environment
// that locates it, is wrong: bad JSON, a relative repo path, no usable HOME
// (S42: the CLI gives them the code user_config). A file that cannot be read
// is not one.
var ErrConfig = errors.New("invalid user config")

// configError is an ErrConfig whose text is its own.
type configError string

func (e configError) Error() string { return string(e) }

func (e configError) Is(target error) bool { return target == ErrConfig }

// Path resolves the config using the injected environment, including on macOS.
func Path(getenv func(string) string) (string, error) {
	base := getenv("XDG_CONFIG_HOME")
	if !filepath.IsAbs(base) {
		home := getenv("HOME")
		if home == "" {
			return "", configError("HOME is unset or empty and XDG_CONFIG_HOME is unset, empty or relative")
		}
		if !filepath.IsAbs(home) {
			return "", configError("HOME must be an absolute path")
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "adjudicate", "config.json"), nil
}

// Load treats a missing file as an empty list and rejects unknown keys,
// non-object JSON, trailing data, and non-absolute repo paths.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{Repos: []string{}}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg *Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("%s: %w: %w", path, ErrConfig, err)
	}
	if cfg == nil {
		return Config{}, fmt.Errorf("%s: %w: must be a JSON object", path, ErrConfig)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("%s: %w: data after the JSON object", path, ErrConfig)
	}
	for i, repo := range cfg.Repos {
		if !filepath.IsAbs(repo) {
			return Config{}, fmt.Errorf("%s: %w: repo %q must be absolute", path, ErrConfig, repo)
		}
		cfg.Repos[i] = filepath.Clean(repo)
	}
	if cfg.Repos == nil {
		cfg.Repos = []string{}
	}
	return *cfg, nil
}

// Save writes a complete config using a same-directory temporary file and
// rename. It cleans up the temporary file on failures as well as on success.
func Save(path string, cfg Config) (err error) {
	if cfg.Repos == nil {
		cfg.Repos = []string{}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0644); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Add appends an unlisted path, preserving order; false means no change.
func (c *Config) Add(repo string) bool {
	for _, p := range c.Repos {
		if p == repo {
			return false
		}
	}
	c.Repos = append(c.Repos, repo)
	return true
}

// Remove removes a listed path; false means it was not listed.
func (c *Config) Remove(repo string) bool {
	found := false
	out := []string{}
	for _, p := range c.Repos {
		if p == repo {
			found = true
		} else {
			out = append(out, p)
		}
	}
	if found {
		c.Repos = out
	}
	return found
}

// Updater permits a caller to supply a lock function. The zero value uses
// the platform lock; tests supply a shorter wait without shared state.
type Updater struct {
	Lock func(path string, timeout time.Duration) (func(), error)
}

// Update serializes the complete read-modify-write operation with other repo
// writers. The lock remains beside config.json and is never removed. A false
// result from change preserves the existing config bytes.
func Update(path string, change func(*Config) (bool, error)) error {
	return (Updater{}).Update(path, change)
}

// Update performs the same locked update using u's lock function.
func (u Updater) Update(path string, change func(*Config) (bool, error)) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	lock := u.Lock
	if lock == nil {
		lock = store.AcquireLock
	}
	unlock, err := lock(path+".lock", 5*time.Second)
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := Load(path)
	if err != nil {
		return err
	}
	changed, err := change(&cfg)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return Save(path, cfg)
}
