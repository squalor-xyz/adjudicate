// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func reposEnv(t *testing.T) (Env, string, string) {
	t.Helper()
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	if err := os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".adjudicate.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	env := Env{Root: repo, Stop: base, Getwd: func() (string, error) { return base, nil }, Getenv: func(k string) string {
		if k == "XDG_CONFIG_HOME" {
			return filepath.Join(base, "xdg")
		}
		if k == "HOME" {
			return filepath.Join(base, "home")
		}
		return ""
	}}
	return env, filepath.Join(base, "xdg", "adjudicate", "config.json"), repo
}

func writeReposConfig(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReposPathResolution(t *testing.T) {
	for _, mode := range []string{"xdg", "unset", "empty", "neither"} {
		t.Run(mode, func(t *testing.T) {
			env, path, repo := reposEnv(t)
			base := filepath.Dir(repo)
			env.Getenv = func(k string) string {
				if k == "XDG_CONFIG_HOME" && mode == "xdg" {
					return filepath.Join(base, "xdg")
				}
				if k == "HOME" && mode != "neither" {
					return filepath.Join(base, "home")
				}
				return ""
			}
			r := run(env, "", "repos", "add")
			if mode == "neither" {
				if r.code != 1 || !strings.Contains(r.stderr, "HOME") || r.stdout != "" {
					t.Fatalf("missing environment: %+v", r)
				}
				return
			}
			if mode != "xdg" {
				path = filepath.Join(base, "home", ".config", "adjudicate", "config.json")
			}
			if r.code != 0 || r.stdout != "" || r.stderr != "" {
				t.Fatalf("add: %+v", r)
			}
			b, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(b), quoteRepo(repo)) {
				t.Fatalf("config %s: %s, %v", path, b, err)
			}
		})
	}
}

func TestReposOperations(t *testing.T) {
	for _, op := range []string{"default", "flag-root", "leading-root", "explicit", "relative", "cleaned", "discover", "duplicate", "remove", "remove-relative", "unlisted", "invalid-repo", "missing-text", "missing-json", "stale", "root-unused", "append", "remove-only", "unlisted-existing", "symlink", "interspersed-json"} {
		t.Run(op, func(t *testing.T) {
			env, path, repo := reposEnv(t)
			args := []string{"repos", "add"}
			wantCode, wantOut := 0, ""
			wantFile := ""
			before := ""
			switch op {
			case "flag-root":
				env.Root = filepath.Join(filepath.Dir(repo), "absent")
				args = append(args, "--root", repo)
			case "leading-root":
				env.Root = ""
				args = []string{"--root", repo, "repos", "add"}
			case "explicit":
				env.Root = ""
				args = append(args, repo)
			case "relative":
				args = append(args, "repo")
			case "cleaned":
				args = append(args, repo+"/../repo/.")
			case "discover":
				env.Root = ""
				env.Getwd = func() (string, error) { return repo, nil }
			case "duplicate":
				before = "{\"repos\": [" + quoteRepo(repo) + "]}\n"
				writeReposConfig(t, path, before)
				wantFile = before
			case "remove", "remove-relative":
				before = "{\"repos\": [" + quoteRepo(repo) + ", " + quoteRepo(filepath.Dir(repo)) + "]}"
				writeReposConfig(t, path, before)
				args = []string{"repos", "remove", repo}
				if op == "remove-relative" {
					args[2] = "repo"
				}
				wantFile = filepath.Dir(repo)
			case "unlisted":
				args = []string{"repos", "remove", repo}
				wantCode = 1
			case "invalid-repo":
				args = append(args, filepath.Dir(repo))
				wantCode = 1
			case "missing-text":
				args = []string{"repos", "list"}
			case "missing-json":
				args = []string{"repos", "list", "--json"}
				wantOut = "{\n  \"repos\": []\n}\n"
			case "stale":
				before = "{\"repos\":[" + quoteRepo(repo+"/gone") + "]}"
				writeReposConfig(t, path, before)
				args = []string{"repos", "list"}
				wantOut = filepath.Join(repo, "gone") + "\n"
				wantFile = before
			case "append":
				before = "{\"repos\":[" + quoteRepo(filepath.Dir(repo)) + "]}"
				writeReposConfig(t, path, before)
			case "remove-only":
				before = "{\"repos\":[" + quoteRepo(repo) + "]}"
				writeReposConfig(t, path, before)
				args = []string{"repos", "remove", repo}
			case "unlisted-existing":
				before = "{\"repos\":[" + quoteRepo(filepath.Dir(repo)) + "]}"
				writeReposConfig(t, path, before)
				args = []string{"repos", "remove", repo}
				wantCode = 1
				wantFile = before
			case "symlink":
				alias := filepath.Join(filepath.Dir(repo), "alias")
				if err := os.Symlink(repo, alias); err != nil {
					t.Fatal(err)
				}
				repo = alias
				args = append(args, alias)
			case "interspersed-json":
				args = []string{"repos", "--root", repo + "/gone", "list", "--json"}
				wantOut = "{\n  \"repos\": []\n}\n"
			case "root-unused":
				args = []string{"repos", "list", "--root", repo + "/gone"}
			}
			r := run(env, "", args...)
			if r.code != wantCode || r.stdout != wantOut {
				t.Fatalf("%v: got %+v; want exit %d stdout %q", args, r, wantCode, wantOut)
			}
			if wantCode == 1 {
				if !strings.Contains(r.stderr, "adj repos:") {
					t.Fatalf("error: %q", r.stderr)
				}
			} else if r.stderr != "" {
				t.Fatalf("stderr: %s", r.stderr)
			}
			data, err := os.ReadFile(path)
			switch op {
			case "invalid-repo", "unlisted", "missing-text", "missing-json", "root-unused", "interspersed-json":
				if !os.IsNotExist(err) {
					t.Fatalf("unexpected config: %s, %v", data, err)
				}
			case "duplicate", "stale", "unlisted-existing":
				if err != nil || string(data) != wantFile {
					t.Fatalf("config changed: %s, %v", data, err)
				}
			default:
				var cfg struct {
					Repos []string `json:"repos"`
				}
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(data, &cfg); err != nil {
					t.Fatal(err)
				}
				expected := repo
				if op == "remove" || op == "remove-relative" {
					expected = wantFile
				}
				wantRepos := []string{expected}
				if op == "append" {
					wantRepos = []string{filepath.Dir(repo), repo}
				}
				if op == "remove-only" {
					wantRepos = []string{}
				}
				if !reflect.DeepEqual(cfg.Repos, wantRepos) {
					t.Fatalf("stored repos: %q, want %q", cfg.Repos, wantRepos)
				}
				list := run(env, "", "repos", "list")
				wantList := ""
				for _, p := range wantRepos {
					wantList += p + "\n"
				}
				if list.code != 0 || list.stdout != wantList || list.stderr != "" {
					t.Fatalf("list: %+v", list)
				}
			}
		})
	}
}

func quoteRepo(s string) string { b, _ := json.Marshal(s); return string(b) }

func TestReposMalformed(t *testing.T) {
	for _, bad := range []string{"{", `{"typo":[]}`, `{"repos":["relative"]}`, `null`, `[]`, `{} {}`, `{"repos":[1]}`} {
		for _, op := range []string{"add", "remove", "list"} {
			t.Run(op+"/"+bad, func(t *testing.T) {
				env, path, repo := reposEnv(t)
				writeReposConfig(t, path, bad)
				args := []string{"repos", op}
				if op != "list" {
					args = append(args, repo)
				}
				r := run(env, "", args...)
				if r.code != 1 || !strings.Contains(r.stderr, path) || !strings.Contains(r.stderr, "invalid user config") || r.stdout != "" {
					t.Fatalf("malformed config: %+v", r)
				}
				b, err := os.ReadFile(path)
				if err != nil || string(b) != bad {
					t.Fatalf("malformed config overwritten: %s, %v", b, err)
				}
			})
		}
	}
}

func TestReposUsage(t *testing.T) {
	for _, args := range [][]string{{"repos"}, {"repos", "unknown"}, {"repos", "add", "a", "b"}, {"repos", "remove"}, {"repos", "remove", "a", "b"}, {"repos", "list", "extra"}, {"repos", "add", ""}, {"repos", "remove", " "}, {"repos", "add", "--json"}, {"repos", "list", "--bogus"}} {
		t.Run(strings.Join(args, "/"), func(t *testing.T) {
			env, path, _ := reposEnv(t)
			r := run(env, "", args...)
			if r.code != 2 || !strings.Contains(r.stderr, "usage: adj repos") || r.stdout != "" {
				t.Fatalf("usage: %+v", r)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("usage wrote config: %v", err)
			}
		})
	}
}
