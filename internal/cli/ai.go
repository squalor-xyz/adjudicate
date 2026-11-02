// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/squalor-xyz/adjudicate/internal/commands"
)

const aiUsage = `usage: adj ai
       adj ai skill [--output PATH | --install] [--force]

Print a short AGENTS.md snippet that tells an agent how to use adj: check
before asking, ask, read the decisions, and leave deciding to the owner. Paste
it into the AGENTS.md of a repo that uses adjudicate. It reads no repo and
takes no flags. adj ai skill prints or installs the longer agent skill (see
adj ai skill --help).
`

const aiSkillUsage = `usage: adj ai skill [--output PATH | --install [--root DIR]] [--force]

Print the agent skill: SKILL.md, which tells an agent when to check, how to ask
and read decisions, and what is the owner's alone. The text is part of this
binary, so it matches this version. Printing reads no repo.

  --output PATH  write SKILL.md to PATH instead, creating parent directories;
                 stdout lists the path written; reads no repo
  --install      write SKILL.md and reference.md into .claude/skills/adj/,
                 .agents/skills/adj/ and .grok/skills/adj/ under the repo root;
                 stdout lists the paths written. It needs a repo (the nearest
                 .adjudicate.json). To put the skill elsewhere use --output
  --root DIR     with --install only: the repo root; default: found from the
                 working directory upwards (the nearest .adjudicate.json)
  --force        replace a file that holds different text. Without it a file
                 that differs exits 1 and nothing is written. A file that
                 already holds the text is left alone and not listed

--install and --output cannot be combined, and --force goes with either.
`

func (c *invocation) runAI(args []string) int {
	fs := flag.NewFlagSet("ai", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	if len(args) > 0 && args[0] == "skill" {
		return c.runAISkill(args[1:])
	}
	pos, code, done := c.parse("ai", aiUsage, fs, args)
	if done {
		return code
	}
	if len(pos) > 0 {
		return c.usageError("ai", aiUsage, fmt.Errorf("ai takes no arguments, got %q", pos[0]))
	}
	fmt.Fprint(c.stdout, commands.AgentSnippet())
	return exitOK
}

// runAISkill is adj ai skill: print SKILL.md, write it to a file, or install
// the skill files under the repo root (S54). Printing and --output read no
// repo; --install needs one, which is why only it takes --root.
func (c *invocation) runAISkill(args []string) int {
	fs, root := newFlagSet("ai skill")
	output := fs.String("output", "", "")
	install := fs.Bool("install", false, "")
	force := fs.Bool("force", false, "")

	pos, code, done := c.parse("ai skill", aiSkillUsage, fs, args)
	if done {
		return code
	}
	usage := func(err error) int { return c.usageError("ai skill", aiSkillUsage, err) }
	switch {
	case len(pos) > 0:
		return usage(fmt.Errorf("ai skill takes no arguments, got %q", pos[0]))
	case fsSet(fs, "output") && strings.TrimSpace(*output) == "":
		return usage(fmt.Errorf("--output needs a path"))
	case *install && fsSet(fs, "output"):
		return usage(fmt.Errorf("--install and --output cannot be combined"))
	case fsSet(fs, "root") && !*install:
		return usage(fmt.Errorf("--root goes with --install; printing and --output read no repo"))
	case *force && !*install && !fsSet(fs, "output"):
		return usage(fmt.Errorf("--force goes with --install or --output; printing replaces nothing"))
	}

	switch {
	case *install:
		dir, err := c.rootFor(*root, false)
		if err != nil {
			// The error names the problem; it does not tell the reader what
			// to run, because that is for the owner to decide.
			return c.fail("ai skill", err)
		}
		var files []commands.TextFile
		for _, d := range commands.SkillInstallDirs {
			for _, f := range commands.SkillFiles() {
				files = append(files, commands.TextFile{Path: filepath.Join(dir, filepath.FromSlash(d), f.Name), Text: f.Text})
			}
		}
		return c.writeSkill(files, *force)
	case fsSet(fs, "output"):
		return c.writeSkill([]commands.TextFile{{Path: *output, Text: commands.AgentSkill()}}, *force)
	}
	fmt.Fprint(c.stdout, commands.AgentSkill())
	return exitOK
}

// writeSkill writes files and lists the paths it wrote on stdout.
func (c *invocation) writeSkill(files []commands.TextFile, force bool) int {
	written, err := commands.WriteTextFiles(files, force)
	if err != nil {
		return c.fail("ai skill", err)
	}
	for _, p := range written {
		fmt.Fprintln(c.stdout, p)
	}
	return exitOK
}
