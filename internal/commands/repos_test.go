// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/store"
	"github.com/squalor-xyz/adjudicate/internal/userconfig"
)

func TestReposAddRejectsNonRepo(t *testing.T) {
	for _, mode := range []string{"missing", "directory"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			config := filepath.Join(root, "user", "config.json")
			if mode == "directory" {
				if err := os.Mkdir(filepath.Join(root, ".adjudicate.json"), 0755); err != nil {
					t.Fatal(err)
				}
			}
			err := ReposAdd(config, root)
			if err == nil || !strings.Contains(err.Error(), ".adjudicate.json") {
				t.Fatalf("non-repo error: %v", err)
			}
			if _, err := os.Stat(config); !os.IsNotExist(err) {
				t.Fatalf("non-repo wrote config: %v", err)
			}
		})
	}
}

func TestReposWritesLock(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "dragonfly", "freebsd", "illumos", "linux", "netbsd", "openbsd", "windows":
	default:
		t.Skip("platform has no flock or LockFileEx")
	}

	for _, op := range []string{"add", "remove"} {
		t.Run(op, func(t *testing.T) {
			base := t.TempDir()
			repo := filepath.Join(base, "repo")
			if err := os.Mkdir(repo, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, ".adjudicate.json"), []byte("{}"), 0644); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(base, "config.json")
			cfg := userconfig.Config{Repos: []string{}}
			if op == "remove" {
				cfg.Repos = []string{repo}
			}
			if err := userconfig.Save(path, cfg); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			lock := path + ".lock"
			release, err := store.AcquireLock(lock, 30*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { release() }()
			operation := ReposAdd
			if op == "remove" {
				operation = ReposRemove
			}
			started := time.Now()
			err = operation(path, repo)
			if err == nil || !strings.Contains(err.Error(), lock) {
				t.Errorf("%s while held: %v; want error naming %s", op, err, lock)
			}
			if time.Since(started) < 30*time.Millisecond {
				t.Error("writer did not wait for held lock")
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || string(after) != string(before) {
				t.Errorf("locked write changed config: %s, %v", after, readErr)
			}
			release()
			release = func() {}
			if err := operation(path, repo); err != nil {
				t.Fatal(err)
			}
			fi, err := os.Stat(lock)
			if err != nil || fi.Size() != 0 {
				t.Fatalf("lock must remain empty: %v, %v", fi, err)
			}
			// Check persistence before another acquisition could recreate the file.
			unlock, err := store.AcquireLock(lock, 30*time.Millisecond)
			if err != nil {
				t.Fatalf("write leaked lock: %v", err)
			}
			unlock()
			got, err := ReposList(path)
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if op == "remove" {
				want = 0
			}
			if len(got) != want || (want == 1 && got[0] != repo) {
				t.Fatalf("repo list = %v, want count %d", got, want)
			}
		})
	}
}

func TestReposRemoveMissingConfigWritesNothing(t *testing.T) {
	for _, existingDir := range []bool{false, true} {
		t.Run(fmt.Sprint(existingDir), func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "user")
			if existingDir {
				if err := os.Mkdir(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			err := ReposRemove(filepath.Join(dir, "config.json"), filepath.Join(base, "repo"))
			if err == nil || !strings.Contains(err.Error(), "not listed") {
				t.Fatalf("remove error = %v; want not listed", err)
			}
			if existingDir {
				entries, err := os.ReadDir(dir)
				if err != nil || len(entries) != 0 {
					t.Fatalf("remove left files: %v, %v", entries, err)
				}
			} else if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("remove created config directory: %v", err)
			}
		})
	}
}
