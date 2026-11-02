// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomicFailsWithoutLeavingATempFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "SKILL.md")
	// A non-empty directory cannot be replaced by a file, on any system.
	if err := os.MkdirAll(filepath.Join(target, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(target, []byte("text\n"), 0o644); err == nil {
		t.Fatal("writeFileAtomic replaced a directory without an error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "SKILL.md" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("the directory holds %v after the failed write, want only SKILL.md", names)
	}
}

func TestWriteFileAtomicWritesTheTextAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := writeFileAtomic(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "two\n" {
		t.Errorf("file = %q, %v; want the second text", got, err)
	}
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o644 {
			t.Errorf("mode = %v, %v; want 0644 (the mode of the last write)", fi.Mode(), err)
		}
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("%d entries after two writes, want 1 (no temp file left)", len(entries))
	}
}

func TestMissingDirs(t *testing.T) {
	base := t.TempDir()
	got, err := missingDirs(filepath.Join(base, "a", "b", "c"))
	want := []string{filepath.Join(base, "a"), filepath.Join(base, "a", "b"), filepath.Join(base, "a", "b", "c")}
	if err != nil || strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("missingDirs = %v, %v; want %v, outermost first", got, err, want)
	}
	if got, err := missingDirs(base); err != nil || len(got) != 0 {
		t.Errorf("missingDirs(existing) = %v, %v; want none", got, err)
	}
	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// Where a missing path is reported as missing even though an ancestor is
	// a file (Windows), the walk must still refuse.
	if _, err := missingDirs(file); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("missingDirs(file) error = %v, want \"not a directory\"", err)
	}
}

func TestWriteTextFilesRefusesWhenAParentIsAFile(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, ".claude"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := []TextFile{
		{Path: filepath.Join(base, ".agents", "SKILL.md"), Text: "a\n"},
		{Path: filepath.Join(base, ".claude", "skills", "SKILL.md"), Text: "b\n"},
	}
	written, err := WriteTextFiles(files, true)
	if err == nil || len(written) != 0 {
		t.Fatalf("WriteTextFiles = %v, %v; want an error and nothing written", written, err)
	}
	if _, err := os.Stat(filepath.Join(base, ".agents")); err == nil {
		t.Error("the first file was written although the second cannot be")
	}
}
