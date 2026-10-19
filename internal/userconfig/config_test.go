// SPDX-License-Identifier: GPL-3.0-or-later

package userconfig

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/squalor-xyz/adjudicate/internal/store"
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
				// Windows has no unix permission bits: files report 0666.
				if runtime.GOOS != "windows" {
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

func TestPathRelativeEnvironment(t *testing.T) {
	for _, mode := range []string{"relative-xdg", "relative-home"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			if mode == "relative-home" {
				home = "relative-home"
			}
			got, err := Path(func(k string) string {
				if k == "XDG_CONFIG_HOME" {
					return "relative-xdg"
				}
				if k == "HOME" {
					return home
				}
				return ""
			})
			if mode == "relative-home" {
				if err == nil || !strings.Contains(err.Error(), "HOME") {
					t.Fatalf("relative HOME: path %q, err %v", got, err)
				}
				return
			}
			want := filepath.Join(home, ".config", "adjudicate", "config.json")
			if err != nil || got != want {
				t.Fatalf("path = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestPathRelativeXDGEmptyHOME(t *testing.T) {
	_, err := Path(func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return "relative-xdg"
		}
		return ""
	})
	const want = "HOME is unset or empty and XDG_CONFIG_HOME is unset, empty or relative"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v; want %q", err, want)
	}
}

func TestUpdateLoadsUnderLock(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "dragonfly", "freebsd", "illumos", "linux", "netbsd", "openbsd", "windows":
	default:
		t.Skip("platform has no flock or LockFileEx")
	}
	base := t.TempDir()
	path := filepath.Join(base, "config.json")
	repoA, repoB := filepath.Join(base, "a"), filepath.Join(base, "b")
	if err := Save(path, Config{}); err != nil {
		t.Fatal(err)
	}
	release, err := store.AcquireLock(path+".lock", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { release() }()
	entered := make(chan struct{})
	lock := func(path string, timeout time.Duration) (func(), error) {
		close(entered)
		return store.AcquireLock(path, timeout)
	}
	done := make(chan error, 1)
	go func() {
		done <- (Updater{Lock: lock}).Update(path, func(cfg *Config) (bool, error) { return cfg.Add(repoB), nil })
	}()
	<-entered
	// Another writer changes the real config before Update can acquire the lock.
	saveErr := Save(path, Config{Repos: []string{repoA}})
	release()
	release = func() {}
	updateErr := <-done
	if saveErr != nil {
		t.Fatal(saveErr)
	}
	if updateErr != nil {
		t.Fatal(updateErr)
	}
	cfg, err := Load(path)
	want := []string{repoA, repoB}
	if err != nil || !reflect.DeepEqual(cfg.Repos, want) {
		t.Fatalf("repos = %v, %v; want %v", cfg.Repos, err, want)
	}
}

func TestUpdateShortLockTimeout(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "dragonfly", "freebsd", "illumos", "linux", "netbsd", "openbsd", "windows":
	default:
		t.Skip("platform has no flock or LockFileEx")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	release, err := store.AcquireLock(path+".lock", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	started := time.Now()
	err = (Updater{Lock: func(path string, _ time.Duration) (func(), error) {
		return store.AcquireLock(path, 30*time.Millisecond)
	}}).Update(path, func(c *Config) (bool, error) { return c.Add(filepath.Dir(path)), nil })
	elapsed := time.Since(started)
	if err == nil || !strings.Contains(err.Error(), path+".lock") {
		t.Fatalf("held lock error = %v", err)
	}
	if elapsed >= time.Second {
		t.Errorf("short lock timeout took %v, want under 1s", elapsed)
	}
	if elapsed < 30*time.Millisecond {
		t.Errorf("lock returned too soon: %v", elapsed)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("held lock wrote config: %v", err)
	}
}

func TestUpdateProductionTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	sentinel := errors.New("lock probe")
	err := (Updater{Lock: func(_ string, timeout time.Duration) (func(), error) {
		if timeout != 5*time.Second {
			t.Errorf("production timeout = %v, want 5s", timeout)
		}
		return nil, sentinel
	}}).Update(path, func(*Config) (bool, error) { t.Fatal("change called without lock"); return false, nil })
	if !errors.Is(err, sentinel) {
		t.Fatalf("lock error = %v, want probe", err)
	}
}
