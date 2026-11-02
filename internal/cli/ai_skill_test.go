// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/commands"
)

// The skill is the long form of the AGENTS.md snippet: SKILL.md says when and
// how, reference.md holds the detail. They are kept short because an agent
// reads them in full.
const (
	maxSkillBytes     = 6000
	maxReferenceBytes = 9000
)

// snippetSHA256 pins `adj ai`: S54 added a subcommand beside it without changing
// a byte of the snippet (C125). S51 changes it on purpose, to put `adj narrow`
// among the commands an agent never runs; the pin moved with it.
const snippetSHA256 = "b8607235b925b9758275d8f4971ff51d3afc8696b3b31151d48e7839a9d6437e"

func skillFile(t *testing.T, name string) string {
	t.Helper()
	for _, f := range commands.SkillFiles() {
		if f.Name == name {
			return f.Text
		}
	}
	t.Fatalf("the skill has no file %s", name)
	return ""
}

// wantDirs are the folders the install writes, spelled out so a change to the
// list in commands shows here.
var wantDirs = []string{".claude/skills/adj", ".agents/skills/adj", ".grok/skills/adj"}

// wantInstalled maps each path the install writes, relative to the root, to
// its text.
func wantInstalled() map[string]string {
	want := map[string]string{}
	for _, dir := range wantDirs {
		for _, f := range commands.SkillFiles() {
			want[dir+"/"+f.Name] = f.Text
		}
	}
	return want
}

func TestAISnippetMatchesItsPin(t *testing.T) {
	out := run(testEnv(t.TempDir()), "", "ai").stdout
	sum := sha256.Sum256([]byte(out))
	if got := hex.EncodeToString(sum[:]); got != snippetSHA256 {
		t.Errorf("adj ai output has sha256 %s, want %s: a change to the snippet moves the pin on purpose (C125)", got, snippetSHA256)
	}
}

func TestAISkillPrintsSKILLmd(t *testing.T) {
	// No repo is needed: the directory has no config file, and the working
	// directory fails any use.
	r := run(testEnv(t.TempDir()), "", "ai", "skill")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q; want exit 0 and no stderr", r.code, r.stderr)
	}
	if r.stdout != skillFile(t, "SKILL.md") || r.stdout != commands.AgentSkill() {
		t.Errorf("stdout is not SKILL.md:\n%s", r.stdout)
	}
	if !strings.HasPrefix(r.stdout, "---\nname: adj\ndescription: ") || !strings.HasSuffix(r.stdout, "\n") || strings.HasSuffix(r.stdout, "\n\n") {
		t.Errorf("SKILL.md should start with frontmatter naming the skill adj and end with one newline:\n%q", r.stdout)
	}
	if len(r.stdout) > maxSkillBytes {
		t.Errorf("SKILL.md is %d bytes, want at most %d", len(r.stdout), maxSkillBytes)
	}
	if ref := skillFile(t, "reference.md"); len(ref) == 0 || len(ref) > maxReferenceBytes {
		t.Errorf("reference.md is %d bytes, want 1 to %d", len(ref), maxReferenceBytes)
	}
	// The skill is a folder: SKILL.md first, then the reference file.
	var names []string
	for _, f := range commands.SkillFiles() {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "SKILL.md,reference.md" {
		t.Errorf("skill files = %v, want SKILL.md and reference.md", names)
	}
	// The pointer to the reference file comes before the first section.
	if intro, _, _ := strings.Cut(r.stdout, "\n## "); !strings.Contains(intro, "`reference.md`") {
		t.Errorf("the start of SKILL.md does not point to reference.md:\n%s", intro)
	}
}

func TestAISkillHelp(t *testing.T) {
	env := testEnv(t.TempDir())
	for _, flag := range []string{"--help", "-h"} {
		r := run(env, "", "ai", "skill", flag)
		if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj ai skill") || r.stderr != "" {
			t.Errorf("ai skill %s: exit %d, stdout %q, stderr %q; want exit 0 and usage on stdout", flag, r.code, r.stdout, r.stderr)
		}
	}
	// The ai usage points to the subcommand; its first line stays.
	if r := run(env, "", "ai", "--help"); !strings.HasPrefix(r.stdout, "usage: adj ai\n       adj ai skill ") {
		t.Errorf("ai --help should list adj ai skill after its first line:\n%s", r.stdout)
	}
	if r := run(env, "", "--help"); !strings.Contains(r.stdout, "adj ai") || !strings.Contains(r.stdout, "skill") {
		t.Errorf("main usage should mention the skill:\n%s", r.stdout)
	}
}

func TestAISkillBadArguments(t *testing.T) {
	root := initRoot(t)
	out := filepath.Join(t.TempDir(), "SKILL.md")
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"positional argument", []string{"ai", "skill", "extra"}, "no arguments"},
		{"unknown flag", []string{"ai", "skill", "--nope"}, "nope"},
		{"--root without --install", []string{"ai", "skill", "--root", root}, "--install"},
		{"leading --root without --install", []string{"--root", root, "ai", "skill"}, "--install"},
		{"--root with --output", []string{"ai", "skill", "--output", out, "--root", root}, "--install"},
		{"--force alone", []string{"ai", "skill", "--force"}, "--force"},
		{"--install with --output", []string{"ai", "skill", "--install", "--output", out}, "--output"},
		{"blank --output", []string{"ai", "skill", "--output", " "}, "--output"},
		{"--json", []string{"ai", "skill", "--json"}, "json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 || r.stdout != "" {
				t.Errorf("exit %d, stdout %q; want exit 2 and no stdout", r.code, r.stdout)
			}
			if !strings.Contains(r.stderr, tt.wantErr) || !strings.Contains(r.stderr, "usage: adj ai skill") {
				t.Errorf("stderr should contain %q and the ai skill usage:\n%s", tt.wantErr, r.stderr)
			}
			if !equalSnapshots(before, snapshot(t, root)) {
				t.Error("a usage error changed the repo")
			}
			if _, err := os.Stat(out); err == nil {
				t.Error("a usage error wrote the output file")
			}
		})
	}
}

func equalSnapshots(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

func TestAISkillOutputWritesSKILLmd(t *testing.T) {
	env := testEnv(t.TempDir()) // no repo: --output reads none
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "SKILL.md")

	r := run(env, "", "ai", "skill", "--output", path)
	if r.code != 0 || r.stdout != path+"\n" || r.stderr != "" {
		t.Fatalf("first run: exit %d, stdout %q, stderr %q; want exit 0 and the path", r.code, r.stdout, r.stderr)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != commands.AgentSkill() {
		t.Fatalf("file = %q, %v; want SKILL.md", got, err)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o644 {
			t.Errorf("mode = %v, %v; want 0644", fi.Mode(), err)
		}
	}
	// The reference file is for --install; --output writes the one file.
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("--output wrote %d files in %s, want 1", len(entries), filepath.Dir(path))
	}

	// A second run with the same text changes nothing and lists nothing.
	before := snapshot(t, dir)
	r = run(env, "", "ai", "skill", "--output", path)
	if r.code != 0 || r.stdout != "" || r.stderr != "" || !equalSnapshots(before, snapshot(t, dir)) {
		t.Errorf("second run: exit %d, stdout %q, stderr %q; want exit 0 and nothing written", r.code, r.stdout, r.stderr)
	}

	// Different text is not replaced without --force.
	if err := os.WriteFile(path, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = run(env, "", "ai", "skill", "--output", path)
	if got, _ := os.ReadFile(path); r.code != 1 || r.stdout != "" || string(got) != "mine\n" {
		t.Errorf("differing file: exit %d, stdout %q, file %q; want exit 1 and the file kept", r.code, r.stdout, got)
	}
	if !strings.Contains(r.stderr, path) || !strings.Contains(r.stderr, "--force") {
		t.Errorf("stderr should name the file and --force:\n%s", r.stderr)
	}
	r = run(env, "", "ai", "skill", "--output", path, "--force")
	if got, _ := os.ReadFile(path); r.code != 0 || r.stdout != path+"\n" || string(got) != commands.AgentSkill() {
		t.Errorf("--force: exit %d, stdout %q, file %q; want exit 0 and SKILL.md", r.code, r.stdout, got)
	}
}

func TestAISkillOutputFailsWhenPathIsADirectory(t *testing.T) {
	dir := t.TempDir()
	r := run(testEnv(t.TempDir()), "", "ai", "skill", "--output", dir, "--force")
	if r.code != 1 || r.stdout != "" || r.stderr == "" {
		t.Errorf("exit %d, stdout %q, stderr %q; want exit 1 and an error", r.code, r.stdout, r.stderr)
	}
}

func TestAISkillInstallWritesThreeFolders(t *testing.T) {
	root := initRoot(t)
	r := run(testEnv(root), "", "ai", "skill", "--install")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("exit %d, stderr %q; want exit 0", r.code, r.stderr)
	}
	want := wantInstalled()
	if len(want) != 6 {
		t.Fatalf("expected 6 files, the install table has %d", len(want))
	}
	if strings.Join(commands.SkillInstallDirs, ",") != strings.Join(wantDirs, ",") {
		t.Errorf("SkillInstallDirs = %v, want %v", commands.SkillInstallDirs, wantDirs)
	}
	var lines []string
	for _, dir := range wantDirs {
		for _, f := range commands.SkillFiles() {
			lines = append(lines, filepath.Join(root, filepath.FromSlash(dir), f.Name))
		}
	}
	if r.stdout != strings.Join(lines, "\n")+"\n" {
		t.Errorf("stdout should list the paths written:\n%s\nwant\n%s", r.stdout, strings.Join(lines, "\n"))
	}
	for rel, text := range want {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || string(got) != text {
			t.Errorf("%s = %q, %v; want the skill text", rel, got, err)
		}
	}
	for _, rel := range []string{".claude/skills/adj/SKILL.md", ".agents/skills/adj/SKILL.md", ".grok/skills/adj/SKILL.md"} {
		if got, _ := os.ReadFile(filepath.Join(root, rel)); string(got) != commands.AgentSkill() {
			t.Errorf("%s is not SKILL.md", rel)
		}
	}

	// A second run changes nothing, lists nothing and keeps the file times.
	before := snapshot(t, root)
	info := map[string]os.FileInfo{}
	for rel := range want {
		fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		info[rel] = fi
	}
	r = run(testEnv(root), "", "ai", "skill", "--install")
	if r.code != 0 || r.stdout != "" || r.stderr != "" || !equalSnapshots(before, snapshot(t, root)) {
		t.Errorf("second run: exit %d, stdout %q, stderr %q; want exit 0 and nothing written", r.code, r.stdout, r.stderr)
	}
	for rel, fi := range info {
		if now, _ := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !now.ModTime().Equal(fi.ModTime()) {
			t.Errorf("%s was rewritten by the second run", rel)
		}
	}
}

func TestAISkillInstallRootFromFlag(t *testing.T) {
	root := initRoot(t)
	for _, args := range [][]string{
		{"ai", "skill", "--install", "--root", root},
		{"--root", root, "ai", "skill", "--install"},
	} {
		other := t.TempDir() // Env.Root names a directory that holds no repo
		r := run(testEnv(other), "", args...)
		if r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
		if _, err := os.Stat(filepath.Join(root, ".claude/skills/adj/SKILL.md")); err != nil {
			t.Errorf("%v: nothing installed under --root: %v", args, err)
		}
		if got := snapshot(t, other); len(got) != 0 {
			t.Errorf("%v: wrote into Env.Root instead of --root: %v", args, got)
		}
		os.RemoveAll(filepath.Join(root, ".claude"))
		os.RemoveAll(filepath.Join(root, ".agents"))
		os.RemoveAll(filepath.Join(root, ".grok"))
	}
}

func TestAISkillInstallNeedsARepo(t *testing.T) {
	empty := t.TempDir()
	r := run(testEnv(empty), "", "ai", "skill", "--install")
	if r.code != 1 || r.stdout != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and no stdout", r.code, r.stdout)
	}
	if !strings.Contains(r.stderr, ".adjudicate.json") {
		t.Errorf("stderr should name the missing config:\n%s", r.stderr)
	}
	// The error names the problem; it does not tell the reader to run init.
	if strings.Contains(r.stderr, "adj init") {
		t.Errorf("stderr should not suggest `adj init`:\n%s", r.stderr)
	}
	if got := snapshot(t, empty); len(got) != 0 {
		t.Errorf("the install wrote %v without a repo", got)
	}
}

func TestAISkillInstallDifferingDestinationNeedsForce(t *testing.T) {
	root := initRoot(t)
	if r := run(testEnv(root), "", "ai", "skill", "--install"); r.code != 0 {
		t.Fatalf("install: exit %d: %s", r.code, r.stderr)
	}
	// One file differs; one folder is gone, one file is gone: with the
	// refusal nothing is written, not even what is missing.
	differs := filepath.Join(root, ".agents/skills/adj/SKILL.md")
	if err := os.WriteFile(differs, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".grok")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".claude/skills/adj/reference.md")); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)

	r := run(testEnv(root), "", "ai", "skill", "--install")
	if r.code != 1 || r.stdout != "" {
		t.Errorf("exit %d, stdout %q; want exit 1 and no stdout", r.code, r.stdout)
	}
	if !strings.Contains(r.stderr, differs) || !strings.Contains(r.stderr, "--force") {
		t.Errorf("stderr should name the differing file and --force:\n%s", r.stderr)
	}
	if strings.Contains(r.stderr, "usage:") {
		t.Errorf("a differing destination is a failure, not a usage error:\n%s", r.stderr)
	}
	if !equalSnapshots(before, snapshot(t, root)) {
		t.Error("a refused install changed the repo")
	}

	r = run(testEnv(root), "", "ai", "skill", "--install", "--force")
	if r.code != 0 {
		t.Fatalf("--force: exit %d: %s", r.code, r.stderr)
	}
	// Only what changed is listed: the replaced file and the two missing ones.
	wantLines := []string{
		filepath.Join(root, ".claude/skills/adj/reference.md"),
		differs,
		filepath.Join(root, ".grok/skills/adj/SKILL.md"),
		filepath.Join(root, ".grok/skills/adj/reference.md"),
	}
	if r.stdout != strings.Join(wantLines, "\n")+"\n" {
		t.Errorf("--force stdout:\n%s\nwant\n%s", r.stdout, strings.Join(wantLines, "\n"))
	}
	for rel, text := range wantInstalled() {
		if got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err != nil || string(got) != text {
			t.Errorf("%s = %q, %v after --force; want the skill text", rel, got, err)
		}
	}
}

func TestAISkillInstallRefusesADestinationThatIsNotAFile(t *testing.T) {
	root := initRoot(t)
	// SKILL.md is a directory in the last folder; the first two are free.
	if err := os.MkdirAll(filepath.Join(root, ".grok/skills/adj/SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	for _, args := range [][]string{{"ai", "skill", "--install"}, {"ai", "skill", "--install", "--force"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, ".grok/skills/adj/SKILL.md") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1 naming the directory", args, r.code, r.stdout, r.stderr)
		}
		if !equalSnapshots(before, snapshot(t, root)) {
			t.Errorf("%v changed the repo", args)
		}
	}
}

// This repo's own install of the skill (.claude/, .agents/ and .grok/ under
// the repo root) is the text of this binary. After a change to the skill, run
// `adj ai skill --install --force` in the repo root.
func TestRepoOwnSkillInstallMatchesTheBinary(t *testing.T) {
	root := filepath.Join("..", "..")
	for rel, text := range wantInstalled() {
		got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("%s: %v; run adj ai skill --install in the repo root", rel, err)
		} else if string(got) != text {
			t.Errorf("%s differs from the binary's skill text; run adj ai skill --install --force in the repo root", rel)
		}
	}
}

// A destination file that is itself a symbolic link (or a directory) is
// refused, with or without --force: replacing it would swap the link for a
// file. Only that last element is checked; a symbolic link in a parent folder
// (.claude, say) is followed, as people link those on purpose.
func TestAISkillInstallRefusesASymbolicLink(t *testing.T) {
	root := initRoot(t)
	target := filepath.Join(t.TempDir(), "elsewhere.md")
	if err := os.WriteFile(target, []byte(commands.AgentSkill()), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, ".agents/skills/adj/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	before := snapshot(t, root)
	for _, args := range [][]string{{"ai", "skill", "--install"}, {"ai", "skill", "--install", "--force"}} {
		r := run(testEnv(root), "", args...)
		if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, link) || !strings.Contains(r.stderr, "not a regular file") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit 1 naming the link", args, r.code, r.stdout, r.stderr)
		}
		if !equalSnapshots(before, snapshot(t, root)) {
			t.Errorf("%v changed the repo", args)
		}
	}
}

// A symbolic link in a parent folder is followed: people link .claude or
// .agents on purpose. Only the destination file itself is checked.
func TestAISkillInstallFollowsASymbolicLinkInAParentFolder(t *testing.T) {
	root := initRoot(t)
	shared := filepath.Join(root, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shared, filepath.Join(root, ".grok")); err != nil {
		t.Skipf("cannot make a symbolic link here: %v", err)
	}
	r := run(testEnv(root), "", "ai", "skill", "--install")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	if got, err := os.ReadFile(filepath.Join(shared, "skills/adj/SKILL.md")); err != nil || string(got) != commands.AgentSkill() {
		t.Errorf("the skill was not written through the linked folder: %q, %v", got, err)
	}
	if r := run(testEnv(root), "", "ai", "skill", "--install"); r.code != 0 || r.stdout != "" {
		t.Errorf("second run: exit %d, stdout %q; want exit 0 and nothing written", r.code, r.stdout)
	}
}

// A write that fails after others succeeded leaves no partial install: new
// files are removed and replaced files get their old text back.
func TestAISkillInstallRollsBackWhenAWriteFails(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs unix permission bits and a user they bind")
	}
	root := initRoot(t)
	old := filepath.Join(root, ".claude/skills/adj/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("local edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(root, ".grok/skills/adj")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	before := snapshot(t, root)

	r := run(testEnv(root), "", "ai", "skill", "--install", "--force")
	if r.code != 1 || r.stderr == "" {
		t.Fatalf("exit %d, stderr %q; want exit 1 and an error", r.code, r.stderr)
	}
	if !equalSnapshots(before, snapshot(t, root)) {
		t.Errorf("a failed install left files behind:\nbefore %v\nafter  %v", before, snapshot(t, root))
	}
	if got, _ := os.ReadFile(old); string(got) != "local edit\n" {
		t.Errorf("the replaced file was not restored: %q", got)
	}
}

// --- the text of the skill ---

var (
	fenceLine  = regexp.MustCompile("^\\s*```")
	skillToken = regexp.MustCompile(`\badj (ai skill|[a-z][a-z-]*)|--[a-z][a-z-]*|\bADJ_[A-Z_]+`)
)

// codeSegments returns the code in a Markdown text: each line of a fenced
// block and each inline span, in order.
func codeSegments(text string) []string {
	var segs []string
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		if fenceLine.MatchString(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			segs = append(segs, line)
			continue
		}
		for _, m := range snippetSpan.FindAllStringSubmatch(line, -1) {
			segs = append(segs, m[1])
		}
	}
	return segs
}

// The skill cannot drift from the CLI: every command it names in code exists,
// every flag after a command exists on that command, and every environment
// variable is in some named command's usage (C124, the same check as for the
// snippet).
func TestAISkillOnlyNamesRealCommandsFlagsAndVariables(t *testing.T) {
	env := testEnv(t.TempDir())
	helps := map[string]string{}
	help := func(cmd string) string {
		if h, ok := helps[cmd]; ok {
			return h
		}
		r := run(env, "", append(strings.Fields(cmd), "--help")...)
		if r.code != 0 || !strings.Contains(r.stdout, "usage: adj "+cmd) {
			t.Errorf("the skill names `adj %s`, but `adj %s --help` exits %d with %q", cmd, cmd, r.code, r.stdout)
		}
		helps[cmd] = r.stdout
		return r.stdout
	}
	for _, f := range commands.SkillFiles() {
		cmd, flags, vars := "", 0, []string{}
		for _, seg := range codeSegments(f.Text) {
			for _, tok := range skillToken.FindAllString(seg, -1) {
				switch {
				case strings.HasPrefix(tok, "adj "):
					cmd = strings.TrimPrefix(tok, "adj ")
					help(cmd)
				case strings.HasPrefix(tok, "ADJ_"):
					vars = append(vars, tok)
				default:
					flags++
					if cmd == "" {
						t.Errorf("%s: `%s`: flag %s comes before any `adj CMD`", f.Name, seg, tok)
					} else if !helpHasFlag(help(cmd), tok) {
						t.Errorf("%s: `%s` (after `adj %s`): `adj %s --help` does not list %s", f.Name, seg, cmd, cmd, tok)
					}
				}
			}
		}
		if f.Name == "SKILL.md" && flags < 8 {
			t.Errorf("checked only %d flags in %s, want at least 8", flags, f.Name)
		}
		if f.Name == "reference.md" && flags < 30 {
			t.Errorf("checked only %d flags in %s, want at least 30", flags, f.Name)
		}
		var all strings.Builder
		for _, h := range helps {
			all.WriteString(h)
		}
		for _, v := range vars {
			if !strings.Contains(all.String(), v) {
				t.Errorf("%s names %s, but no command it names mentions it in its usage", f.Name, v)
			}
		}
	}
	// The same command names appear in both files.
	for _, want := range []string{"ask", "check", "context", "show", "list", "inbox", "amend", "withdraw"} {
		if _, ok := helps[want]; !ok {
			t.Errorf("the skill and its reference do not name `adj %s`", want)
		}
	}
	// `adj inbox -i` is named as owner-only; the short flag must exist.
	if !strings.Contains(help("inbox"), "-i ") {
		t.Error("adj inbox --help does not list -i")
	}
}

// Section returns the text of the "## name" section of a Markdown text, up to
// the next "## " heading.
func section(text, name string) string {
	_, after, ok := strings.Cut(text, "\n## "+name+"\n")
	if !ok {
		return ""
	}
	body, _, _ := strings.Cut(after, "\n## ")
	return body
}

// The owner-only line (C27): the skill lists the commands an agent never
// runs, and no code outside that section tells an agent to run one of them.
func TestAISkillStatesTheOwnerOnlyCommands(t *testing.T) {
	skill := commands.AgentSkill()
	owner := section(skill, "Owner only")
	if owner == "" {
		t.Fatalf("SKILL.md has no \"## Owner only\" section:\n%s", skill)
	}
	for _, want := range []string{
		"`adj decide`", "`adj reject`", "`adj supersede`", "`adj narrow`", "`adj inbox -i`",
		"`adj renumber`", "`adj init`", "`adj import-adr`",
		"never run",
	} {
		if !strings.Contains(owner, want) {
			t.Errorf("the owner-only section does not say %s:\n%s", want, owner)
		}
	}
	// Duplicate ID and no repo: tell the owner.
	for _, want := range []string{"tell the owner", "duplicate ID", "no repo"} {
		if !strings.Contains(skill, want) {
			t.Errorf("SKILL.md does not say %q", want)
		}
	}
	ownerOnly := regexp.MustCompile(`^adj (decide|reject|supersede|narrow|renumber|init|import-adr|inbox -i|repos)\b`)
	rest := strings.Replace(skill, owner, "", 1)
	ref := commands.SkillFiles()[1].Text
	ref = strings.Replace(ref, section(ref, "When to tell the owner"), "", 1)
	for _, f := range []string{rest, ref} {
		for _, seg := range codeSegments(f) {
			if ownerOnly.MatchString(strings.TrimSpace(seg)) {
				t.Errorf("code %q outside the owner-only section tells an agent to run an owner command", seg)
			}
		}
	}
	// Flags that only the owner's commands take are not in the skill.
	all := skill + commands.SkillFiles()[1].Text
	for _, bad := range []string{"--decided-at", "--recorded-by", "--via", "ADJ_DECIDED_BY"} {
		if strings.Contains(all, bad) {
			t.Errorf("the skill names %s, which is the owner's", bad)
		}
	}
}

// The skill gives what the snippet cannot: each check result, the flags that
// narrow a read, and the shape of a good question.
func TestAISkillCoversTheWorkflow(t *testing.T) {
	skill := commands.AgentSkill()
	for _, want := range []string{
		"`decided`", "`open`", "`closed`", "`not found`",
		"--key-prefix", "--ref ", "--max-bytes", "--json", "--format json",
		"--recommend-text", "--recommend ", "--tradeoffs", "--reasoning", "--key ", "ADJ_ASKED_BY",
		"adj withdraw", "adj amend", "adj context", "adj show", "adj list", "adj check",
		"Exit codes", "reference.md",
	} {
		if !strings.Contains(skill, want) {
			t.Errorf("SKILL.md does not mention %q", want)
		}
	}
	fm, _, _ := strings.Cut(strings.TrimPrefix(skill, "---\n"), "\n---\n")
	for _, want := range []string{"name: adj", "description: ", "decision", "owner"} {
		if !strings.Contains(fm, want) {
			t.Errorf("the frontmatter does not contain %q:\n%s", want, fm)
		}
	}
	if strings.Count(fm, "\n") != 1 {
		t.Errorf("the frontmatter should be two lines, name and description:\n%s", fm)
	}
}

// The reference's JSON keys are the ones docs/json.md pins.
func TestAISkillReferenceJSONKeysAreInDocsJSON(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "json.md"))
	if err != nil {
		t.Fatal(err)
	}
	pinned := string(data)
	ref := skillFile(t, "reference.md")
	keys := regexp.MustCompile(`"([a-z_]+)"`).FindAllStringSubmatch(ref, -1)
	if len(keys) < 15 {
		t.Fatalf("the reference quotes only %d JSON keys, want at least 15", len(keys))
	}
	for _, m := range keys {
		// docs/json.md shows keys quoted in its examples and in backticks in
		// its key tables.
		if !strings.Contains(pinned, `"`+m[1]+`"`) && !strings.Contains(pinned, "`"+m[1]+"`") {
			t.Errorf("the reference quotes the key %q, which docs/json.md does not", m[1])
		}
	}
	// A record has more keys than the short list an agent first thinks of.
	for _, key := range []string{"context", "refs", "supersedes", "superseded_by", "consequences", "body"} {
		if !strings.Contains(ref, `"`+key+`"`) {
			t.Errorf("the reference does not quote the record key %q", key)
		}
	}
	for _, code := range []string{"no_repo", "not_found", "duplicate_id", "usage", "budget"} {
		if !strings.Contains(ref, code) {
			t.Errorf("the reference does not list the error code %s", code)
		}
	}
}

// The reference says which blank values of `adj ask` exit 2; each one does.
func TestAISkillReferenceBlankValueClaimsHold(t *testing.T) {
	ref := skillFile(t, "reference.md")
	var claim string
	for _, line := range strings.Split(ref, "\n") {
		if strings.HasPrefix(line, "- A blank ") {
			claim = line
		}
	}
	if claim == "" {
		t.Fatal("the reference has no \"- A blank ...\" line for adj ask")
	}
	flags := snippetFlag.FindAllString(claim, -1)
	if len(flags) < 4 {
		t.Fatalf("the blank-value line names %d flags, want at least 4: %s", len(flags), claim)
	}
	root := initRoot(t)
	for _, flag := range flags {
		args := []string{"ask", "Title", "--option", "a:A", flag, " "}
		if flag == "--reasoning" {
			// Blank reasoning is only refused when there is no recommendation.
			args = append(args, "--recommend", "a")
		}
		if r := run(testEnv(root), "", args...); r.code != 2 {
			t.Errorf("the reference says a blank %s exits 2, but %v exits %d", flag, args, r.code)
		}
	}
	// What the reference says about --reasoning instead.
	if !strings.Contains(ref, "`--reasoning` without `--recommend` or `--recommend-text` exits 2") {
		t.Error("the reference does not say that --reasoning without a recommendation exits 2")
	}
	r := run(testEnv(root), "", "ask", "T", "--option", "a:A", "--reasoning", "why")
	if r.code != 2 {
		t.Errorf("--reasoning without a recommendation exits %d, want 2", r.code)
	}
}

// Two wording points that were wrong: the omitted line appears only when a
// decision is left out, and a superseded match leads to a decided record, so
// check's result is `decided`, not `closed`.
func TestAISkillWordingOfMaxBytesAndSupersededMatches(t *testing.T) {
	skill := commands.AgentSkill()
	ref := skillFile(t, "reference.md")
	for name, text := range map[string]string{"SKILL.md": skill, "reference.md": ref} {
		if !strings.Contains(text, "when some do not fit") {
			t.Errorf("%s does not say that the omitted line comes when some decisions do not fit", name)
		}
		if strings.Contains(text, "and ends with `N decisions omitted`") {
			t.Errorf("%s says --max-bytes output always ends with the omitted line", name)
		}
	}
	var decided, closed string
	for _, line := range strings.Split(skill, "\n") {
		switch {
		case strings.HasPrefix(line, "- `decided`"):
			decided = line
		case strings.HasPrefix(line, "- `closed`"):
			closed = line
		}
	}
	if !strings.Contains(decided, "superseded") {
		t.Errorf("the `decided` bullet does not say a superseded match leads to the decision that replaced it:\n%s", decided)
	}
	if strings.Contains(closed, "replaced it") || strings.Contains(closed, "also lists") {
		t.Errorf("the `closed` bullet still claims a superseded match lists its replacement:\n%s", closed)
	}
	// The behaviour the wording rests on (C113).
	root := seedCheck(t) // record 5 is superseded by decided record 6
	r := run(testEnv(root), "", "check", "Old database")
	if first, _, _ := strings.Cut(r.stdout, "\n"); first != "decided" {
		t.Errorf("check of a superseded record with a decided replacement prints %q first, want decided", first)
	}
}

// Replaced files keep their permission bits (C229).
func TestAISkillInstallForceKeepsTheModeOfAReplacedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no unix permission bits to keep")
	}
	root := initRoot(t)
	path := filepath.Join(root, ".agents/skills/adj/SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("local edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil { // not lowered by the umask
		t.Fatal(err)
	}
	r := run(testEnv(root), "", "ai", "skill", "--install", "--force")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.stderr)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != commands.AgentSkill() {
		t.Errorf("file = %q, %v; want SKILL.md", got, err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, %v; want 0600 kept", fi.Mode(), err)
	}
	// A file that is new gets 0644.
	if fi, err := os.Stat(filepath.Join(root, ".grok/skills/adj/SKILL.md")); err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("new file mode = %v, %v; want 0644", fi.Mode(), err)
	}
}
