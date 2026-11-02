// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		file    *string // nil: no config file
		want    string
		wantErr bool
	}{
		{"no file gives the default", nil, DefaultStore, false},
		{"empty object gives the default", ptr(`{}`), DefaultStore, false},
		{"empty store gives the default", ptr(`{"store": ""}`), DefaultStore, false},
		{"relative path", ptr(`{"store": "docs/decisions"}`), filepath.Join("docs", "decisions"), false},
		{"trailing slash is cleaned", ptr(`{"store": "log/"}`), "log", false},
		{"dot is the root itself", ptr(`{"store": "."}`), ".", false},
		{"absolute path is rejected", ptr(`{"store": "` + filepath.ToSlash(os.TempDir()) + `"}`), "", true},
		{"parent escape is rejected", ptr(`{"store": "../elsewhere"}`), "", true},
		{"hidden parent escape is rejected", ptr(`{"store": "a/../../elsewhere"}`), "", true},
		{"unknown key is rejected", ptr(`{"stor": "x"}`), "", true},
		{"wrong type is rejected", ptr(`{"store": 3}`), "", true},
		{"not JSON is rejected", ptr(`store = "x"`), "", true},
		{"empty file is rejected", ptr(``), "", true},
		{"trailing data is rejected", ptr(`{} {}`), "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.file != nil {
				writeFile(t, filepath.Join(root, ConfigFile), *tt.file)
			}
			cfg, err := LoadConfig(root)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidConfig) {
					t.Fatalf("err = %v, want ErrInvalidConfig", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Store != tt.want {
				t.Errorf("Store = %q, want %q", cfg.Store, tt.want)
			}
		})
	}
}

func TestLoadConfigDirectoryIsNotAConfigFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ConfigFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(root); err == nil {
		t.Fatal("want an error when the config path is a directory")
	}
}

func ptr(s string) *string { return &s }

func TestFindRoot(t *testing.T) {
	// Every case builds its tree under its own t.TempDir() and stops the
	// upward walk at the temp root (stop), so nothing above it is read.
	tests := []struct {
		name string
		// config lists the dirs that hold a config file, relative to the
		// temp root ("." for the root itself).
		config []string
		start  string // relative to the temp root
		stop   string // relative to the temp root; the last dir examined
		want   string // relative to the temp root; "" means ErrNoRoot
	}{
		{"start dir holds the config", []string{"."}, ".", ".", "."},
		{"ancestor holds the config", []string{"."}, "a/b/c", ".", "."},
		{"nearest config wins", []string{".", "a"}, "a/b/c", ".", "a"},
		{"no config below the stop dir", nil, "a/b/c", ".", ""},
		{"walk stops at the stop dir", []string{"outer"}, "outer/inner/deep", "outer/inner", ""},
		{"stop dir itself is examined", []string{"outer/inner"}, "outer/inner/deep", "outer/inner", "outer/inner"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			for _, d := range tt.config {
				writeFile(t, filepath.Join(tmp, filepath.FromSlash(d), ConfigFile), "{}")
			}
			start := filepath.Join(tmp, filepath.FromSlash(tt.start))
			if err := os.MkdirAll(start, 0o755); err != nil {
				t.Fatal(err)
			}
			got, err := FindRootWithin(start, filepath.Join(tmp, filepath.FromSlash(tt.stop)))
			if tt.want == "" {
				if !errors.Is(err, ErrNoRoot) {
					t.Fatalf("got %q, %v; want ErrNoRoot", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(tmp, filepath.FromSlash(tt.want)); got != want {
				t.Errorf("root = %q, want %q", got, want)
			}
		})
	}
}

func TestFindRootIgnoresDirectoryNamedLikeConfig(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "a", ConfigFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := FindRootWithin(filepath.Join(tmp, "a"), tmp); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("got %q, %v; want ErrNoRoot", got, err)
	}
}

func TestFindRootStartMustBeInsideStop(t *testing.T) {
	tmp := t.TempDir()
	a, b := filepath.Join(tmp, "a"), filepath.Join(tmp, "b")
	for _, d := range []string{a, b} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(tmp, ConfigFile), "{}")
	// b is not below a, so a walk from b would never meet the stop dir and
	// would climb to tmp, where the config is. It must refuse instead.
	if got, err := FindRootWithin(b, a); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("got %q, %v; want ErrNoRoot", got, err)
	}
}

func intp(n int) *int { return &n }

func TestLoadConfigContract(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		want    *int
		wantErr bool
	}{
		{"no key is no pin", `{"store": "decisions"}`, nil, false},
		{"a positive pin", `{"store": "decisions", "contract": 1}`, intp(1), false},
		{"a later version", `{"contract": 12}`, intp(12), false},
		{"zero is refused", `{"contract": 0}`, nil, true},
		{"negative is refused", `{"contract": -1}`, nil, true},
		{"a string is refused", `{"contract": "v1"}`, nil, true},
		{"a fraction is refused", `{"contract": 1.5}`, nil, true},
		{"a boolean is refused", `{"contract": true}`, nil, true},
		{"a misspelt key is still refused", `{"contracts": 1}`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, ConfigFile), tt.file)
			cfg, err := LoadConfig(root)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidConfig) {
					t.Fatalf("err = %v, want ErrInvalidConfig", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if (cfg.Contract == nil) != (tt.want == nil) || (tt.want != nil && *cfg.Contract != *tt.want) {
				t.Errorf("Contract = %v, want %v", cfg.Contract, tt.want)
			}
			st, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if got := st.Contract(); (got == nil) != (tt.want == nil) || (tt.want != nil && *got != *tt.want) {
				t.Errorf("Store.Contract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetContract(t *testing.T) {
	const withStore = "{\n  \"store\": \"decisions\"\n}\n"
	tests := []struct {
		name        string
		file        string
		set         *int
		want        string
		wantChanged bool
	}{
		{"pin keeps store and puts contract after it", withStore, intp(1),
			"{\n  \"store\": \"decisions\",\n  \"contract\": 1\n}\n", true},
		{"pin to the pinned value is left alone, whatever the layout", `{"store":"decisions","contract":1}`, intp(1),
			`{"store":"decisions","contract":1}`, false},
		{"pin changes the number", "{\n  \"store\": \"docs/log\",\n  \"contract\": 1\n}\n", intp(3),
			"{\n  \"store\": \"docs/log\",\n  \"contract\": 3\n}\n", true},
		{"unpin removes the key", "{\n  \"store\": \"decisions\",\n  \"contract\": 2\n}\n", nil, withStore, true},
		{"unpin of an unpinned file is left alone", `{"store":"decisions"}`, nil, `{"store":"decisions"}`, false},
		{"a file with no store key gets none", `{}`, intp(2), "{\n  \"contract\": 2\n}\n", true},
		{"unpinning down to nothing leaves an empty object", `{"contract": 2}`, nil, "{}\n", true},
		{"a store path is kept as typed", `{"store": "a/./b/"}`, intp(1),
			"{\n  \"store\": \"a/./b/\",\n  \"contract\": 1\n}\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ConfigFile)
			writeFile(t, path, tt.file)
			changed, err := SetContract(root, tt.set)
			if err != nil {
				t.Fatal(err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("file =\n%q\nwant\n%q", got, tt.want)
			}
			if _, err := LoadConfig(root); err != nil {
				t.Errorf("the written file does not load: %v", err)
			}
		})
	}
}

func TestSetContractRefusesWithoutWriting(t *testing.T) {
	t.Run("a missing file", func(t *testing.T) {
		root := t.TempDir()
		if _, err := SetContract(root, intp(1)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v, want os.ErrNotExist", err)
		}
		if _, err := os.Stat(filepath.Join(root, ConfigFile)); err == nil {
			t.Error("SetContract created a config file")
		}
	})
	t.Run("an invalid file", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ConfigFile)
		writeFile(t, path, `{"stor": "x"}`)
		if _, err := SetContract(root, intp(1)); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("err = %v, want ErrInvalidConfig", err)
		}
		if got, _ := os.ReadFile(path); string(got) != `{"stor": "x"}` {
			t.Errorf("file changed to %q", got)
		}
	})
	t.Run("a non-positive pin", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, ConfigFile)
		writeFile(t, path, `{}`)
		for _, n := range []int{0, -1} {
			if _, err := SetContract(root, intp(n)); err == nil {
				t.Errorf("SetContract(%d) = nil, want an error", n)
			}
		}
		if got, _ := os.ReadFile(path); string(got) != `{}` {
			t.Errorf("file changed to %q", got)
		}
	})
}

func TestSetContractKeepsModeAndLeavesNoTempFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no unix permission bits")
	}
	// os.CreateTemp makes 0600, so the modes that tell a kept mode from the
	// temporary file's are the ones that differ from it.
	for _, mode := range []os.FileMode{0o644, 0o640, 0o664} {
		t.Run(mode.String(), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, ConfigFile)
			writeFile(t, path, `{"store": "decisions"}`)
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			if _, err := SetContract(root, intp(1)); err != nil {
				t.Fatal(err)
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().Perm() != mode {
				t.Errorf("mode = %v, want %v", fi.Mode().Perm(), mode)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Errorf("directory holds %d entries, want only the config", len(entries))
			}
		})
	}
}

func TestSetContractWritesThroughASymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	root := t.TempDir()
	target := filepath.Join(root, "real.json")
	writeFile(t, target, `{"store": "decisions"}`)
	link := filepath.Join(root, ConfigFile)
	if err := os.Symlink("real.json", link); err != nil {
		t.Fatal(err)
	}
	if _, err := SetContract(root, intp(1)); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the config is no longer a symlink: %v, %v", fi, err)
	}
	if got, _ := os.ReadFile(target); !strings.Contains(string(got), `"contract": 1`) {
		t.Errorf("target = %q, want the pin written through the link", got)
	}
}

// A write that fails at the rename leaves the config as it was and no
// temporary file behind.
func TestSetContractFailedWriteLeavesTheDirectoryAsItWas(t *testing.T) {
	old := renameFile
	renameFile = func(oldpath, newpath string) error { return errors.New("rename refused") }
	t.Cleanup(func() { renameFile = old })

	root := t.TempDir()
	path := filepath.Join(root, ConfigFile)
	writeFile(t, path, `{"store": "decisions"}`)
	changed, err := SetContract(root, intp(1))
	if err == nil || !strings.Contains(err.Error(), "rename refused") || changed {
		t.Fatalf("SetContract = %v, %v; want the rename error and no change", changed, err)
	}
	if got, _ := os.ReadFile(path); string(got) != `{"store": "decisions"}` {
		t.Errorf("config = %q, want it untouched", got)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Errorf("directory holds %d entries after a failed write, want only the config", len(entries))
	}
}
