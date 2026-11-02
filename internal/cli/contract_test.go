// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/contract"
)

const (
	plainConfig  = "{\n  \"store\": \"decisions\"\n}\n"
	pinnedConfig = "{\n  \"store\": \"decisions\",\n  \"contract\": 1\n}\n"
)

// writeConfig replaces root's .adjudicate.json with text.
func writeConfig(t *testing.T, root, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".adjudicate.json"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readConfig returns root's .adjudicate.json.
func readConfig(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".adjudicate.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// pinConfig is a config that pins contract n.
func pinConfig(n int) string {
	return fmt.Sprintf("{\n  \"store\": \"decisions\",\n  \"contract\": %d\n}\n", n)
}

// errorObject decodes the error object a failed JSON run printed and returns
// its code, command and message.
func errorObject(t *testing.T, stdout string) (code string, command any, message string) {
	t.Helper()
	inner, ok := decodeObject(t, stdout)["error"].(map[string]any)
	if !ok {
		t.Fatalf("stdout is not an error object:\n%s", stdout)
	}
	code, _ = inner["code"].(string)
	message, _ = inner["message"].(string)
	return code, inner["command"], message
}

func TestContractPrintsTheBuiltInVersion(t *testing.T) {
	root := initRoot(t)
	r := run(testEnv(root), "", "contract")
	if r.code != 0 || r.stdout != "adj-json v1\n" || r.stderr != "" {
		t.Errorf("contract = %+v, want exit 0 and %q", r, "adj-json v1\n")
	}

	const unpinned = "{\n  \"contract\": \"adj-json v1\",\n  \"version\": 1,\n  \"pinned\": null\n}\n"
	r = run(testEnv(root), "", "contract", "--json")
	if r.code != 0 || r.stdout != unpinned || r.stderr != "" {
		t.Errorf("contract --json = %+v, want exit 0 and\n%s", r, unpinned)
	}

	// The flag may also be spelled -json.
	if got := run(testEnv(root), "", "contract", "-json"); got.code != 0 || got.stdout != unpinned {
		t.Errorf("contract -json = %+v", got)
	}
}

func TestContractJSONReportsThePin(t *testing.T) {
	for _, n := range []int{1, 2} {
		root := initRoot(t)
		writeConfig(t, root, pinConfig(n))
		want := fmt.Sprintf("{\n  \"contract\": \"adj-json v1\",\n  \"version\": 1,\n  \"pinned\": %d\n}\n", n)
		if r := run(testEnv(root), "", "contract", "--json"); r.code != 0 || r.stdout != want {
			t.Errorf("pin %d: contract --json = %+v, want\n%s", n, r, want)
		}
		// The text form is the built-in version, whatever the pin.
		if r := run(testEnv(root), "", "contract"); r.code != 0 || r.stdout != "adj-json v1\n" {
			t.Errorf("pin %d: contract = %+v", n, r)
		}
	}
}

// adj contract needs no repo: the built-in version is the same everywhere, and
// ork asks for it before it has set a pin. With no repo the pin is null.
func TestContractNeedsNoRepo(t *testing.T) {
	bare := t.TempDir()
	env := Env{Stop: bare, Getwd: func() (string, error) { return bare, nil }, Getenv: func(string) string { return "" }}
	const want = "{\n  \"contract\": \"adj-json v1\",\n  \"version\": 1,\n  \"pinned\": null\n}\n"
	for name, args := range map[string][]string{
		"found from the working directory": {"contract", "--json"},
		"an explicit root with no config":  {"contract", "--json", "--root", bare},
		"a leading root with no config":    {"--root", bare, "contract", "--json"},
	} {
		if r := run(env, "", args...); r.code != 0 || r.stdout != want {
			t.Errorf("%s: %+v, want exit 0 and\n%s", name, r, want)
		}
	}
}

func TestContractFailsOnAnInvalidConfig(t *testing.T) {
	root := initRoot(t)
	writeConfig(t, root, `{"contract": 0}`)
	r := run(testEnv(root), "", "contract", "--json")
	if r.code != 1 {
		t.Fatalf("exit = %d, want 1: %+v", r.code, r)
	}
	if code, command, _ := errorObject(t, r.stdout); code != "invalid_config" || command != "contract" {
		t.Errorf("error object code %q command %v, want invalid_config and contract", code, command)
	}
	// The text form reads no config: it says what this adj speaks, whatever
	// the repo holds.
	if r := run(testEnv(root), "", "contract"); r.code != 0 || r.stdout != "adj-json v1\n" {
		t.Errorf("contract in a repo with an invalid config = %+v, want exit 0 and adj-json v1", r)
	}
}

func TestContractPinAndUnpin(t *testing.T) {
	root := initRoot(t)
	env := testEnv(root)
	if got := readConfig(t, root); got != plainConfig {
		t.Fatalf("init wrote %q", got)
	}

	step := func(label string, want string, args ...string) {
		t.Helper()
		before := snapshot(t, root)
		r := run(env, "", args...)
		if r.code != 0 || r.stdout != "" || r.stderr != "" {
			t.Fatalf("%s: %+v, want exit 0 and no output", label, r)
		}
		if got := readConfig(t, root); got != want {
			t.Fatalf("%s: config = %q, want %q", label, got, want)
		}
		after := snapshot(t, root)
		for name := range after {
			if name != ".adjudicate.json" && before[name] != after[name] {
				t.Errorf("%s: %s changed", label, name)
			}
		}
		if len(before) != len(after) {
			t.Errorf("%s: files before %d, after %d (a temporary file was left?)", label, len(before), len(after))
		}
	}
	step("pin 1", pinnedConfig, "contract", "pin", "1")
	step("pin 1 again leaves the bytes alone", pinnedConfig, "contract", "pin", "1")
	step("pin 2 replaces the number", pinConfig(2), "contract", "pin", "2")
	step("unpin returns the file to its first form", plainConfig, "contract", "unpin")
	step("unpin of an unpinned file is a no-op", plainConfig, "contract", "unpin")
	step("a leading --root works", pinnedConfig, "--root", root, "contract", "pin", "1")
	step("--root after the command works", plainConfig, "contract", "unpin", "--root", root)

	// A hand-formatted file that already has the pin stays as it is.
	writeConfig(t, root, `{"store":"decisions","contract":1}`)
	step("a pin already there keeps its layout", `{"store":"decisions","contract":1}`, "contract", "pin", "1")

	// The pin is read back by contract --json and by the store.
	if r := run(env, "", "contract", "--json"); !strings.Contains(r.stdout, "\"pinned\": 1\n") {
		t.Errorf("contract --json after pin = %s", r.stdout)
	}
}

func TestContractPinKeepsTheStorePath(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "{\n  \"store\": \"docs/log\"\n}\n")
	if r := run(testEnv(root), "", "contract", "pin", "3"); r.code != 0 {
		t.Fatalf("pin: %+v", r)
	}
	if got, want := readConfig(t, root), "{\n  \"store\": \"docs/log\",\n  \"contract\": 3\n}\n"; got != want {
		t.Errorf("config = %q, want %q", got, want)
	}
	if r := run(testEnv(root), "", "contract", "unpin"); r.code != 0 {
		t.Fatalf("unpin: %+v", r)
	}
	if got, want := readConfig(t, root), "{\n  \"store\": \"docs/log\"\n}\n"; got != want {
		t.Errorf("config = %q, want %q", got, want)
	}
}

func TestContractPinCreatesAMissingConfigLikeInit(t *testing.T) {
	t.Run("with an explicit root", func(t *testing.T) {
		root := t.TempDir()
		r := run(testEnv(root), "", "contract", "pin", "1")
		if r.code != 0 || r.stdout != "" {
			t.Fatalf("pin: %+v", r)
		}
		if got := readConfig(t, root); got != pinnedConfig {
			t.Errorf("config = %q, want %q", got, pinnedConfig)
		}
		if fi, err := os.Stat(filepath.Join(root, "decisions")); err != nil || !fi.IsDir() {
			t.Errorf("the store directory was not made: %v", err)
		}
		// A repo made this way is a repo like any other.
		if r := run(testEnv(root), "", "list"); r.code != 0 {
			t.Errorf("list: %+v", r)
		}
	})
	t.Run("with --root", func(t *testing.T) {
		root := t.TempDir()
		env := Env{Stop: root, Getwd: func() (string, error) { return "", fmt.Errorf("no cwd") }, Getenv: func(string) string { return "" }}
		if r := run(env, "", "contract", "pin", "1", "--root", root); r.code != 0 {
			t.Fatalf("pin: %+v", r)
		}
		if got := readConfig(t, root); got != pinnedConfig {
			t.Errorf("config = %q, want %q", got, pinnedConfig)
		}
	})
	t.Run("with no root and no repo, nothing is created", func(t *testing.T) {
		bare := t.TempDir()
		env := Env{Stop: bare, Getwd: func() (string, error) { return bare, nil }, Getenv: func(string) string { return "" }}
		r := run(env, "", "contract", "pin", "1")
		if r.code != 1 || !strings.Contains(r.stderr, "no repo root found") {
			t.Fatalf("pin: %+v, want exit 1 and no repo", r)
		}
		if entries, _ := os.ReadDir(bare); len(entries) != 0 {
			t.Errorf("pin created %d entries in a directory that is not a repo", len(entries))
		}
	})
	t.Run("an existing repo is found from a parent", func(t *testing.T) {
		root := initRoot(t)
		sub := filepath.Join(root, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		env := Env{Stop: root, Getwd: func() (string, error) { return sub, nil }, Getenv: func(string) string { return "" }}
		if r := run(env, "", "contract", "pin", "1"); r.code != 0 {
			t.Fatalf("pin: %+v", r)
		}
		if got := readConfig(t, root); got != pinnedConfig {
			t.Errorf("config = %q, want %q", got, pinnedConfig)
		}
		if _, err := os.Stat(filepath.Join(sub, ".adjudicate.json")); err == nil {
			t.Error("pin made a config in the subdirectory")
		}
	})
}

func TestContractUnpinNeedsARepo(t *testing.T) {
	root := t.TempDir()
	r := run(testEnv(root), "", "contract", "unpin")
	if r.code != 1 || !strings.Contains(r.stderr, "no repo root found") {
		t.Errorf("unpin = %+v, want exit 1 and no repo", r)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Errorf("unpin created %d entries", len(entries))
	}
}

func TestContractPinAndUnpinRefuseAnInvalidConfig(t *testing.T) {
	for _, args := range [][]string{{"contract", "pin", "1"}, {"contract", "unpin"}} {
		root := initRoot(t)
		const bad = `{"stor": "x"}`
		writeConfig(t, root, bad)
		r := run(testEnv(root), "", args...)
		if r.code != 1 || !strings.Contains(r.stderr, "invalid config") {
			t.Errorf("%v: %+v, want exit 1 and an invalid config message", args, r)
		}
		if got := readConfig(t, root); got != bad {
			t.Errorf("%v: config = %q, want it untouched", args, got)
		}
		if entries, _ := os.ReadDir(root); len(entries) != 2 { // the config and the store
			t.Errorf("%v: directory holds %d entries, want 2", args, len(entries))
		}
	}
}

func TestContractUsageErrorsWriteNothing(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"pin 0", []string{"contract", "pin", "0"}},
		{"pin -1", []string{"contract", "pin", "-1"}},
		{"pin v1", []string{"contract", "pin", "v1"}},
		{"pin 1.5", []string{"contract", "pin", "1.5"}},
		{"pin blank", []string{"contract", "pin", ""}},
		{"pin +1", []string{"contract", "pin", "+1"}},
		{"pin 01", []string{"contract", "pin", "01"}},
		{"pin too large", []string{"contract", "pin", "99999999999999999999"}},
		{"pin without a number", []string{"contract", "pin"}},
		{"pin with two numbers", []string{"contract", "pin", "1", "2"}},
		{"unpin with an argument", []string{"contract", "unpin", "1"}},
		{"unknown subcommand", []string{"contract", "bump"}},
		{"show with an argument", []string{"contract", "now", "--json"}},
		{"pin with --json", []string{"contract", "pin", "1", "--json"}},
		{"unpin with --json", []string{"contract", "unpin", "--json"}},
		{"unknown flag", []string{"contract", "--bogus"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := initRoot(t)
			before := snapshot(t, root)
			r := run(testEnv(root), "", tt.args...)
			if r.code != 2 {
				t.Fatalf("%v = %+v, want exit 2", tt.args, r)
			}
			if !strings.Contains(r.stderr, "usage: adj contract") {
				t.Errorf("stderr has no contract usage:\n%s", r.stderr)
			}
			if got := snapshot(t, root); !reflect.DeepEqual(got, before) {
				t.Errorf("a usage error changed the repo: %v -> %v", before, got)
			}
		})
	}
}

func TestContractHelpAndMainUsage(t *testing.T) {
	r := run(testEnv(t.TempDir()), "", "contract", "--help")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "usage: adj contract") || r.stderr != "" {
		t.Errorf("contract --help = %+v", r)
	}
	for _, want := range []string{"pin", "unpin", "--json", "--root", "adj-json"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("contract usage does not mention %q:\n%s", want, r.stdout)
		}
	}
	main := run(testEnv(t.TempDir()), "", "--help")
	if !strings.Contains(main.stdout, "adj contract") {
		t.Errorf("the main usage does not list adj contract:\n%s", main.stdout)
	}
}

func TestVersionJSON(t *testing.T) {
	old := versionOverride
	versionOverride = "v9.9.9-test"
	t.Cleanup(func() { versionOverride = old })

	const want = "{\n  \"version\": \"v9.9.9-test\",\n  \"contract\": \"adj-json v1\"\n}\n"
	for _, args := range [][]string{{"version", "--json"}, {"--version", "--json"}} {
		r := run(testEnv(t.TempDir()), "", args...)
		if r.code != 0 || r.stdout != want || r.stderr != "" {
			t.Errorf("%v = %+v, want exit 0 and\n%s", args, r, want)
		}
	}
	// The text form is the version alone, as before.
	if r := run(testEnv(t.TempDir()), "", "version"); r.stdout != "v9.9.9-test\n" {
		t.Errorf("version = %q", r.stdout)
	}
	// It reads no repo: a pin that does not match changes nothing.
	root := initRoot(t)
	writeConfig(t, root, pinConfig(7))
	if r := run(testEnv(root), "", "version", "--json"); r.code != 0 || r.stdout != want {
		t.Errorf("version --json in a pinned repo = %+v", r)
	}
}

// The examples in docs/json.md are the real output (the docs are the
// contract), and their key tables name the keys.
func TestContractAndVersionJSONMatchTheDocs(t *testing.T) {
	old := versionOverride
	versionOverride = "v1.0.0"
	t.Cleanup(func() { versionOverride = old })

	root := initRoot(t)
	tests := []struct {
		heading string
		args    []string
		keys    []string
	}{
		{"## `adj contract --json`", []string{"contract", "--json"}, []string{"contract", "pinned", "version"}},
		{"## `adj version --json`", []string{"version", "--json"}, []string{"contract", "version"}},
	}
	for _, tt := range tests {
		t.Run(tt.args[0], func(t *testing.T) {
			r := run(testEnv(root), "", tt.args...)
			if want := docsJSONExample(t, tt.heading); r.stdout != want {
				t.Errorf("%v =\n%s\nwant the docs/json.md example\n%s", tt.args, r.stdout, want)
			}
			if got := docsTableKeys(t, tt.heading); !reflect.DeepEqual(got, tt.keys) {
				t.Errorf("key table = %v, want %v", got, tt.keys)
			}
		})
	}
}

// "Contract version" has one line per version, from 1 to the version this adj
// speaks, so raising the number cannot skip the line that says what changed.
func TestContractVersionSectionHasALinePerVersion(t *testing.T) {
	section, err := sectionUnder(docsText(t), "## Contract version")
	if err != nil {
		t.Fatal(err)
	}
	var versions []string
	for _, line := range strings.Split(section, "\n") {
		if rest, ok := strings.CutPrefix(line, "- `adj-json v"); ok {
			v, _, _ := strings.Cut(rest, "`")
			versions = append(versions, v)
		}
	}
	var want []string
	for v := 1; v <= contract.Version; v++ {
		want = append(want, fmt.Sprint(v))
	}
	if !reflect.DeepEqual(versions, want) {
		t.Errorf("docs/json.md \"Contract version\" has lines for versions %v, want %v", versions, want)
	}
	for _, line := range strings.Split(docsText(t), "\n") {
		if strings.HasPrefix(line, "| `contract_mismatch` |") {
			return
		}
	}
	t.Error("the error code table has no contract_mismatch row")
}
