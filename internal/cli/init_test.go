// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// result is what one in-process adj run produced.
type result struct {
	code           int
	stdout, stderr string
}

// fixedNow is the clock every test uses: late in the day in UTC, so a date
// taken from the wrong place (or the wrong zone) shows.
func fixedNow() time.Time { return time.Date(2026, 3, 4, 23, 30, 0, 0, time.UTC) }

// testEnv is an Env that names root explicitly and fails any use of the real
// working directory or environment: tests live under the repo's own .cache,
// where walking up from the cwd could find the repo's own store.
func testEnv(root string) Env {
	return Env{
		Root:   root,
		Now:    fixedNow,
		Getenv: func(string) string { return "" },
		Getwd: func() (string, error) {
			return "", errors.New("test must not use the working directory")
		},
	}
}

// run calls RunWith with env, stdin text and args, and collects the output.
func run(env Env, stdin string, args ...string) result {
	var stdout, stderr bytes.Buffer
	code := RunWith(env, args, strings.NewReader(stdin), &stdout, &stderr)
	return result{code, stdout.String(), stderr.String()}
}

// snapshot maps every file and directory under root (slash-separated, dirs end
// in "/") to its content, so a test can tell that nothing changed. The store's
// lock file, which ask and renumber leave behind and which holds no data, is
// left out (C114).
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.Name() == ".adj.lock" {
			return nil
		}
		if d.IsDir() {
			out[rel+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		out[rel] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestInitCreatesStoreAndConfig(t *testing.T) {
	root := t.TempDir()
	r := run(testEnv(root), "", "init")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	want := map[string]string{
		".adjudicate.json": "{\n  \"store\": \"decisions\"\n}\n",
		"decisions/":       "",
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, want) {
		t.Errorf("tree after init = %q, want %q", got, want)
	}
	if !strings.HasPrefix(r.stdout, "initialized ") {
		t.Errorf("stdout = %q, want it to start with %q", r.stdout, "initialized ")
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
}

func TestInitTwiceLeavesRecordsAndConfigUntouched(t *testing.T) {
	root := t.TempDir()
	env := testEnv(root)
	if r := run(env, "", "init"); r.code != 0 {
		t.Fatalf("first init: exit %d: %s", r.code, r.stderr)
	}
	for _, title := range []string{"First question", "Second question"} {
		if r := run(env, "", "ask", title); r.code != 0 {
			t.Fatalf("ask %q: exit %d: %s", title, r.code, r.stderr)
		}
	}
	before := snapshot(t, root)
	if len(before) != 4 { // config, store dir and two records
		t.Fatalf("setup tree = %q, want config, store dir and two records", before)
	}

	r := run(env, "", "init")
	if r.code != 0 {
		t.Fatalf("second init: exit %d: %s", r.code, r.stderr)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
		t.Errorf("tree changed by second init:\nbefore %q\nafter  %q", before, after)
	}
	if !strings.HasPrefix(r.stdout, "already initialized ") {
		t.Errorf("stdout = %q, want it to say already initialized", r.stdout)
	}
}

func TestInitKeepsAnExistingConfig(t *testing.T) {
	root := t.TempDir()
	cfg := "{\"store\": \"adr/log\"}\n"
	if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	r := run(testEnv(root), "", "init")
	if r.code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr:\n%s", r.code, r.stderr)
	}
	want := map[string]string{
		".adjudicate.json": cfg,
		"adr/":             "",
		"adr/log/":         "",
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, want) {
		t.Errorf("tree after init = %q, want %q", got, want)
	}
}

func TestInitFailures(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, root string)
		args     []string
		wantCode int
		wantErr  string
	}{
		{
			name: "invalid config",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), []byte(`{"stor": "x"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			args: []string{"init"}, wantCode: 1, wantErr: ".adjudicate.json",
		},
		{
			name: "config is a directory",
			setup: func(t *testing.T, root string) {
				if err := os.Mkdir(filepath.Join(root, ".adjudicate.json"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			args: []string{"init"}, wantCode: 1, wantErr: "not a regular file",
		},
		{
			name:     "root does not exist",
			args:     []string{"init", "--root", "@missing"},
			wantCode: 1, wantErr: "missing",
		},
		{
			name: "root is a file",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "afile"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			args:     []string{"init", "--root", "@afile"},
			wantCode: 1, wantErr: "afile: not a directory",
		},
		{
			name: "store path is a file",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "decisions"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			// The config this run would have written is not left behind.
			args: []string{"init"}, wantCode: 1, wantErr: "decisions",
		},
		{
			name: "store path is a file, config already there",
			setup: func(t *testing.T, root string) {
				cfg := []byte(`{"store": "adr"}`)
				if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), cfg, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "adr"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			// An existing config is never removed.
			args: []string{"init"}, wantCode: 1, wantErr: "adr",
		},
		{
			name:     "extra argument",
			args:     []string{"init", "extra"},
			wantCode: 2, wantErr: "usage: adj init",
		},
		{
			name:     "unknown flag",
			args:     []string{"init", "--nope"},
			wantCode: 2, wantErr: "usage: adj init",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, root)
			}
			args := tt.args
			// An argument written @name stands for root/name.
			for i, a := range args {
				if name, ok := strings.CutPrefix(a, "@"); ok {
					args[i] = filepath.Join(root, name)
				}
			}
			before := snapshot(t, root)
			r := run(testEnv(root), "", args...)
			if r.code != tt.wantCode {
				t.Errorf("exit code = %d, want %d; stderr:\n%s", r.code, tt.wantCode, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.wantErr) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantErr, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty", r.stdout)
			}
			if after := snapshot(t, root); !reflect.DeepEqual(after, before) {
				t.Errorf("tree changed:\nbefore %q\nafter  %q", before, after)
			}
		})
	}
}

func TestInitHelp(t *testing.T) {
	root := t.TempDir()
	r := run(testEnv(root), "", "init", "-h")
	if r.code != 0 {
		t.Errorf("exit code = %d, want 0", r.code)
	}
	if !strings.Contains(r.stdout, "usage: adj init") {
		t.Errorf("stdout = %q, want the init usage", r.stdout)
	}
	if r.stderr != "" {
		t.Errorf("stderr = %q, want empty", r.stderr)
	}
	if got := snapshot(t, root); len(got) != 0 {
		t.Errorf("help created %q", got)
	}
}

func TestRootFlagOverridesEnvRoot(t *testing.T) {
	envRoot, flagRoot := t.TempDir(), t.TempDir()
	r := run(testEnv(envRoot), "", "init", "--root", flagRoot)
	if r.code != 0 {
		t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
	}
	if got := snapshot(t, envRoot); len(got) != 0 {
		t.Errorf("env root changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(flagRoot, ".adjudicate.json")); err != nil {
		t.Errorf("flag root has no config: %v", err)
	}
}

func TestInitUsesTheWorkingDirectoryWithoutAnExplicitRoot(t *testing.T) {
	cwd := t.TempDir()
	env := testEnv("")
	env.Getwd = func() (string, error) { return cwd, nil }
	if r := run(env, "", "init"); r.code != 0 {
		t.Fatalf("exit code = %d; stderr:\n%s", r.code, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".adjudicate.json")); err != nil {
		t.Errorf("working directory has no config: %v", err)
	}
}
