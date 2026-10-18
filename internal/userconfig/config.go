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
)

// Config is the user config's complete JSON shape.
type Config struct {
	Repos []string `json:"repos"`
}

// Path resolves the config using the injected environment, including on macOS.
func Path(getenv func(string) string) (string, error) {
	base := getenv("XDG_CONFIG_HOME")
	if base == "" {
		home := getenv("HOME")
		if home == "" {
			return "", errors.New("XDG_CONFIG_HOME and HOME are both unset or empty")
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
		return Config{}, fmt.Errorf("%s: invalid user config: %w", path, err)
	}
	if cfg == nil {
		return Config{}, fmt.Errorf("%s: invalid user config: must be a JSON object", path)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return Config{}, fmt.Errorf("%s: invalid user config: data after the JSON object", path)
	}
	for i, repo := range cfg.Repos {
		if !filepath.IsAbs(repo) {
			return Config{}, fmt.Errorf("%s: invalid user config: repo %q must be absolute", path, repo)
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
