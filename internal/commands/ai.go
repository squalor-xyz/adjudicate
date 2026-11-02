// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// agentSnippet is the AGENTS.md snippet that adj ai prints (S12). It is a
// Markdown file so it reads as it will be pasted; the cli tests check that
// every command, flag and variable it names exists.
//
//go:embed ai.md
var agentSnippet string

// AgentSnippet returns the AGENTS.md snippet that tells an agent how to use
// adj: check before asking, ask, read decisions, and leave deciding to the
// owner (C27). It ends with one newline.
func AgentSnippet() string { return agentSnippet }

// agentSkill and agentSkillReference are the two files of the agent skill
// that adj ai skill prints and installs (S54). They are embedded so the text
// matches the binary that carries it; the cli tests check that every command,
// flag and variable they name exists.
//
//go:embed skill.md
var agentSkill string

//go:embed skill_reference.md
var agentSkillReference string

// SkillFile is one file of the agent skill: its name inside the skill folder
// and its text.
type SkillFile struct {
	Name string
	Text string
}

// SkillFiles returns the files of the agent skill, SKILL.md first. Each text
// ends with one newline.
func SkillFiles() []SkillFile {
	return []SkillFile{
		{Name: "SKILL.md", Text: agentSkill},
		{Name: "reference.md", Text: agentSkillReference},
	}
}

// AgentSkill returns the text of SKILL.md, the skill's entry file.
func AgentSkill() string { return agentSkill }

// SkillInstallDirs are the folders, relative to the repo root, that
// adj ai skill --install writes the skill files into (S54): one per agent
// tool that reads skills.
var SkillInstallDirs = []string{".claude/skills/adj", ".agents/skills/adj", ".grok/skills/adj"}

// TextFile is a file to write: its path and its whole text.
type TextFile struct {
	Path string
	Text string
}

// plannedWrite is a file WriteTextFiles will write, with what undoes it.
type plannedWrite struct {
	TextFile
	mode    os.FileMode
	prev    []byte   // the text being replaced; nil when the file is new
	created []string // directories the write makes, outermost first
}

// WriteTextFiles writes files, creating parent directories (0755); a new file
// gets mode 0644 and a replaced one keeps its permission bits. It looks at
// every destination before it writes any. A destination that is not a regular
// file is an error. One that holds the same text is left alone. One that
// holds different text is an error naming every such file unless force is
// set, and nothing is written in that case. It returns the paths it wrote, in
// order, so a destination that already had the text is not among them. If a
// write fails, the files and directories written before it are put back as
// they were (a failure to do so is added to the error) and no path is
// returned.
func WriteTextFiles(files []TextFile, force bool) ([]string, error) {
	var plan []plannedWrite
	var differs []string
	for _, f := range files {
		fi, err := os.Lstat(f.Path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			created, err := missingDirs(filepath.Dir(f.Path))
			if err != nil {
				return nil, err
			}
			plan = append(plan, plannedWrite{TextFile: f, mode: 0o644, created: created})
			continue
		case err != nil:
			return nil, err
		case !fi.Mode().IsRegular():
			return nil, fmt.Errorf("%s is not a regular file", f.Path)
		}
		have, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, err
		}
		if string(have) == f.Text {
			continue
		}
		if !force {
			differs = append(differs, f.Path)
			continue
		}
		plan = append(plan, plannedWrite{TextFile: f, mode: fi.Mode().Perm(), prev: have})
	}
	if len(differs) > 0 {
		return nil, fmt.Errorf("%d file(s) hold different text; --force replaces them:\n  %s",
			len(differs), strings.Join(differs, "\n  "))
	}

	var written []string
	for i, w := range plan {
		var err error
		if w.prev == nil {
			err = os.MkdirAll(filepath.Dir(w.Path), 0o755)
		}
		if err == nil {
			err = writeFileAtomic(w.Path, []byte(w.Text), w.mode)
		}
		if err != nil {
			err = fmt.Errorf("%w; nothing was installed", err)
			if rerr := rollback(plan[:i], plan[i]); rerr != nil {
				err = fmt.Errorf("%w; %v", err, rerr)
			}
			return nil, err
		}
		written = append(written, w.Path)
	}
	return written, nil
}

// missingDirs lists the directories of dir that do not exist yet, outermost
// first.
func missingDirs(dir string) ([]string, error) {
	var missing []string
	for {
		fi, err := os.Stat(dir)
		if err == nil {
			if !fi.IsDir() {
				return nil, fmt.Errorf("%s is not a directory", dir)
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		missing = append([]string{dir}, missing...)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return missing, nil
}

// rollback undoes done, the writes that succeeded before failed (whose own
// directories may have been made already), newest first: a new file is
// removed, a replaced one gets its old text back, and a directory the install
// made is removed once it is empty.
func rollback(done []plannedWrite, failed plannedWrite) error {
	var problems []string
	for i := len(done) - 1; i >= 0; i-- {
		w := done[i]
		var err error
		if w.prev == nil {
			err = os.Remove(w.Path)
		} else {
			err = writeFileAtomic(w.Path, w.prev, w.mode)
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("could not restore %s: %v", w.Path, err))
		}
	}
	seen := map[string]bool{}
	var dirs []string
	for _, w := range append(append([]plannedWrite(nil), done...), failed) {
		for _, d := range w.created {
			if !seen[d] {
				seen[d] = true
				dirs = append(dirs, d)
			}
		}
	}
	// Longest first, so a directory is emptied before its parent is tried. A
	// directory that still holds something else is not ours to remove.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		os.Remove(d)
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// writeFileAtomic writes text to path through a temporary file in the same
// directory and a rename, so path holds the old or the new text, never part.
func writeFileAtomic(path string, text []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".adj-skill-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := bytes.NewReader(text).WriteTo(f); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
