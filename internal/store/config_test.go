// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
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
		name   string
		config []string // dirs (relative to the temp root, "." for the root) holding a config file
		start  string   // relative to the temp root
		stop   string   // relative to the temp root; the last dir examined
		want   string   // relative to the temp root; "" means ErrNoRoot
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
			got, err := findRoot(start, filepath.Join(tmp, filepath.FromSlash(tt.stop)))
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
	if got, err := findRoot(filepath.Join(tmp, "a"), tmp); !errors.Is(err, ErrNoRoot) {
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
	if got, err := findRoot(b, a); !errors.Is(err, ErrNoRoot) {
		t.Fatalf("got %q, %v; want ErrNoRoot", got, err)
	}
}
