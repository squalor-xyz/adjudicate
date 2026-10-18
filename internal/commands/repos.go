// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/squalor-xyz/adjudicate/internal/store"
	"github.com/squalor-xyz/adjudicate/internal/userconfig"
)

// ReposAdd registers an absolute repo path. A duplicate is a byte-preserving
// no-op; the repo must hold a regular .adjudicate.json file.
func ReposAdd(configPath, repo string) error {
	cfg, err := userconfig.Load(configPath)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(repo) {
		return fmt.Errorf("repo %q must be absolute", repo)
	}
	repo = filepath.Clean(repo)
	marker := filepath.Join(repo, store.ConfigFile)
	fi, err := os.Stat(marker)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file", marker)
	}
	if !cfg.Add(repo) {
		return nil
	}
	return userconfig.Save(configPath, cfg)
}

// ReposRemove unregisters a path without requiring that the repo still exists.
// An unlisted path fails without writing the config.
func ReposRemove(configPath, repo string) error {
	cfg, err := userconfig.Load(configPath)
	if err != nil {
		return err
	}
	if !cfg.Remove(filepath.Clean(repo)) {
		return fmt.Errorf("repo %q is not listed in %s", repo, configPath)
	}
	return userconfig.Save(configPath, cfg)
}

// ReposList returns stored paths in insertion order without probing the repos.
func ReposList(configPath string) ([]string, error) {
	cfg, err := userconfig.Load(configPath)
	return cfg.Repos, err
}
