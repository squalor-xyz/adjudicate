// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/userconfig"
)

const reposUsage = `usage: adj repos add [PATH] [--root DIR]
       adj repos remove PATH [--root DIR]
       adj repos list [--json] [--root DIR]

Manage the user repo list in $XDG_CONFIG_HOME/adjudicate/config.json,
falling back to $HOME/.config/adjudicate/config.json.

Add defaults to the current repo root; PATH must hold .adjudicate.json.
Paths are absolute and cleaned, without resolving symlinks. Adding a listed
path is a no-op. Removing an unlisted path exits 1. List prints stored paths
in insertion order without checking whether they still exist.

  --json       list as {"repos": [...]} instead of one path per line
  --root DIR   default PATH for add; otherwise accepted but unused
`

func (c *invocation) runRepos(args []string) int {
	fs, root := newFlagSet("repos")
	asJSON := fs.Bool("json", false, "")
	pos, code, done := c.parse("repos", reposUsage, fs, args)
	if done {
		return code
	}
	if len(pos) == 0 {
		return c.usageError("repos", reposUsage, fmt.Errorf("a subcommand is required"))
	}
	op := pos[0]
	paths := pos[1:]
	switch op {
	case "add":
		if len(paths) > 1 {
			return c.usageError("repos", reposUsage, fmt.Errorf("add takes at most one PATH"))
		}
	case "remove":
		if len(paths) != 1 {
			return c.usageError("repos", reposUsage, fmt.Errorf("remove needs one PATH"))
		}
	case "list":
		if len(paths) != 0 {
			return c.usageError("repos", reposUsage, fmt.Errorf("list takes no PATH"))
		}
	default:
		return c.usageError("repos", reposUsage, fmt.Errorf("unknown subcommand %q", op))
	}
	if op != "list" && fsSet(fs, "json") {
		return c.usageError("repos", reposUsage, fmt.Errorf("--json is only accepted by list"))
	}
	if len(paths) > 0 && strings.TrimSpace(paths[0]) == "" {
		return c.usageError("repos", reposUsage, fmt.Errorf("PATH must not be blank"))
	}
	configPath, err := userconfig.Path(c.env.Getenv)
	if err != nil {
		return c.fail("repos", err)
	}
	var repo string
	if op != "list" {
		if len(paths) == 0 {
			repo, err = c.rootFor(*root, false)
		} else {
			repo = paths[0]
		}
		if err != nil {
			return c.rootError("repos", err)
		}
		if !filepath.IsAbs(repo) {
			cwd, e := c.env.Getwd()
			if e != nil {
				return c.fail("repos", e)
			}
			repo = filepath.Join(cwd, repo)
		}
		repo = filepath.Clean(repo)
	}
	switch op {
	case "add":
		err = commands.ReposAdd(configPath, repo)
	case "remove":
		err = commands.ReposRemove(configPath, repo)
	case "list":
		var repos []string
		repos, err = commands.ReposList(configPath)
		if err == nil {
			if *asJSON {
				err = render.ReposJSON(c.stdout, repos)
			} else {
				err = render.ReposText(c.stdout, repos)
			}
		}
	}
	if err != nil {
		return c.fail("repos", err)
	}
	return exitOK
}
