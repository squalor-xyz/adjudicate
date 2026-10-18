// SPDX-License-Identifier: GPL-3.0-or-later

package userconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAtomic(t *testing.T) {
	for _, mode := range []string{"create", "replace", "rename-failure"} {
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			path := filepath.Join(base, "adjudicate", "config.json")
			if mode != "create" {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
			}
			link := filepath.Join(base, "old")
			if mode == "replace" {
				if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Link(path, link); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "rename-failure" {
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			}
			err := Save(path, Config{Repos: []string{base}})
			if mode == "rename-failure" {
				if err == nil {
					t.Fatal("Save succeeded over a directory")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				cfg, err := Load(path)
				if err != nil || len(cfg.Repos) != 1 || cfg.Repos[0] != base {
					t.Fatalf("saved config: %+v, %v", cfg, err)
				}
				fi, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if fi.Mode().Perm() != 0644 {
					t.Fatalf("file mode %o", fi.Mode().Perm())
				}
				fi, err = os.Stat(filepath.Dir(path))
				if err != nil {
					t.Fatal(err)
				}
				if fi.Mode().Perm() & ^os.FileMode(0755) != 0 {
					t.Fatalf("dir mode %o", fi.Mode().Perm())
				}
			}
			if mode == "replace" {
				b, err := os.ReadFile(link)
				if err != nil || string(b) != "old" {
					t.Fatalf("old inode changed: %q, %v", b, err)
				}
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "config.json" {
				t.Fatalf("temporary files remain: %v", entries)
			}
		})
	}
}
