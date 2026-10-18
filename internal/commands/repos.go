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
	return reposAdd(configPath, repo, userconfig.Updater{})
}

func reposAdd(configPath, repo string, updater userconfig.Updater) error {
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
	return updater.Update(configPath, func(cfg *userconfig.Config) (bool, error) {
		return cfg.Add(repo), nil
	})
}

// ReposRemove unregisters a path without requiring that the repo still exists.
// An unlisted path fails without writing the config.
func ReposRemove(configPath, repo string) error {
	return reposRemove(configPath, repo, userconfig.Updater{})
}

func reposRemove(configPath, repo string, updater userconfig.Updater) error {
	// A missing config cannot contain the repo. Avoid creating a directory or
	// lock for a removal that cannot change anything; existing configs are still
	// read and changed only under Update's lock.
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return fmt.Errorf("repo %q is not listed in %s", repo, configPath)
	}
	return updater.Update(configPath, func(cfg *userconfig.Config) (bool, error) {
		if !cfg.Remove(filepath.Clean(repo)) {
			return false, fmt.Errorf("repo %q is not listed in %s", repo, configPath)
		}
		return true, nil
	})
}

// ReposList returns stored paths in insertion order without probing the repos.
func ReposList(configPath string) ([]string, error) {
	cfg, err := userconfig.Load(configPath)
	return cfg.Repos, err
}
