// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/contract"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

// The bump rule (S45): contract.Version moves on any change to a shape pinned
// by this package's tests that is not purely additive. testdata/contract/
// shapes.json records, for each JSON-printing command, every key path of its
// output with the JSON types seen there, for the version in the file. The
// test below compares the real output with it, so a key that is removed,
// renamed or retyped while contract.Version still names the recorded version
// fails. Adding a key never fails.
//
// To record after a bump or an addition:
//
//	ADJ_RECORD_SHAPES=1 go test ./internal/cli -run TestRecordContractShapes
//
// The recording refuses to overwrite a file of the current version with shapes
// that lost something: bump contract.Version (and write the line for it under
// "Contract version" in docs/json.md) first.

const shapesFile = "testdata/contract/shapes.json"

// shapes maps a command to its key paths, and each path to the JSON types seen
// at it, sorted and joined with "|": "$.records[].id" is "number". An array is
// the type "array" at its own path, and its elements are at path+"[]".
type shapes map[string]map[string]string

// shapesRecord is the file: the version the shapes belong to, then the shapes.
type shapesRecord struct {
	Version int    `json:"version"`
	Shapes  shapes `json:"shapes"`
}

// addTypes records v and everything inside it in seen, under path.
func addTypes(seen map[string]map[string]bool, path string, v any) {
	var kind string
	switch x := v.(type) {
	case nil:
		kind = "null"
	case bool:
		kind = "boolean"
	case float64:
		kind = "number"
	case string:
		kind = "string"
	case []any:
		kind = "array"
		for _, e := range x {
			addTypes(seen, path+"[]", e)
		}
	case map[string]any:
		kind = "object"
		for k, e := range x {
			addTypes(seen, path+"."+k, e)
		}
	default:
		panic(fmt.Sprintf("unexpected JSON value %T", v))
	}
	if seen[path] == nil {
		seen[path] = map[string]bool{}
	}
	seen[path][kind] = true
}

// shapeOf merges the JSON objects in outputs into one path -> types map.
func shapeOf(t *testing.T, outputs ...string) map[string]string {
	t.Helper()
	seen := map[string]map[string]bool{}
	for _, out := range outputs {
		var v any
		if err := json.Unmarshal([]byte(out), &v); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, out)
		}
		addTypes(seen, "$", v)
	}
	flat := map[string]string{}
	for path, kinds := range seen {
		var list []string
		for k := range kinds {
			list = append(list, k)
		}
		sort.Strings(list)
		flat[path] = strings.Join(list, "|")
	}
	return flat
}

// shapeFixtureRepo is a repo whose records between them have every key a
// record can have, and one of each status that lint, check and context treat
// differently:
//
//	1 open        options with tradeoffs, a label recommendation, key, tags, refs, context, amended
//	2 decided     a free-text recommendation, recorded_by and via, consequences and a body, amended
//	3 superseded  by 4
//	4 decided     supersedes 3
//	5 rejected
//	6 decided     no decided_by (a lint warning), narrowed by 2 (S51)
//
// and 2 narrows 6, so narrowed_by and narrows are recorded: they are optional
// keys, added within contract version 1.
func shapeFixtureRepo(t *testing.T) string {
	t.Helper()
	root := initRoot(t)
	env := testEnv(root)
	for _, args := range [][]string{
		{"ask", "Pick a store", "--option", "a:Use A", "--option", "b:Use B", "--tradeoffs", "a:Younger project",
			"--recommend", "a", "--reasoning", "A is proven", "--key", "storage.engine", "--tag", "infra", "--tag", "perf",
			"--ref", "S1", "--ref", "S2", "--context", "why", "--by", "claude"},
		{"ask", "Cache layer", "--option", "redis:Use Redis", "--recommend-text", "Use Redis, with a TTL",
			"--reasoning", "Hosts share it", "--key", "storage.cache", "--tag", "infra"},
		{"ask", "Old queue", "--tag", "infra", "--key", "queue.old"},
		{"ask", "New queue", "--tag", "infra", "--key", "queue.new"},
		{"ask", "Theme", "--tag", "infra"},
		{"ask", "No owner", "--tag", "infra"},
		{"decide", "2", "redis", "--why", "fast", "--by", "owner", "--recorded-by", "claude", "--via", "chat"},
		{"decide", "3", "--text", "Use X", "--why", "ok", "--by", "owner"},
		{"decide", "4", "--text", "Use Y", "--why", "ok", "--by", "owner"},
		{"supersede", "3", "--by", "4"},
		{"reject", "5", "--why", "not needed"},
		{"decide", "6", "--text", "Use Z", "--why", "ok"},
		{"narrow", "6", "--by", "2", "--scope", "only for the sidebar"},
		{"amend", "1", "--title", "Pick a data store", "--context", "why now", "--summary", "a:Use engine A", "--tradeoffs", "a:Younger project, fewer docs"},
		{"amend", "2", "--rationale", "fast and cheap"},
	} {
		if r := run(env, "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := st.Get(2)
	if err != nil {
		t.Fatal(err)
	}
	rec.Consequences, rec.Body = "More to run.", "Notes.\n"
	if err := st.Update(rec); err != nil {
		t.Fatal(err)
	}
	return root
}

// currentShapes is the shape of the real output of every JSON-printing
// command, on the fixture repo.
func currentShapes(t *testing.T) shapes {
	t.Helper()
	root := shapeFixtureRepo(t)
	env := testEnv(root)
	out := func(args ...string) string {
		t.Helper()
		r := run(env, "", args...)
		if r.stdout == "" {
			t.Fatalf("%v printed nothing: exit %d: %s", args, r.code, r.stderr)
		}
		return r.stdout
	}
	got := shapes{}

	var shows []string
	for id := 1; id <= 6; id++ {
		shows = append(shows, out("show", fmt.Sprint(id), "--json"))
	}
	got["show"] = shapeOf(t, shows...)
	got["list"] = shapeOf(t, out("list", "--json"))
	got["inbox"] = shapeOf(t, out("inbox", "--json"))
	got["check"] = shapeOf(t, out("check", "--json", "infra"), out("check", "--json", "nothing-matches-this"))
	got["context"] = shapeOf(t, out("context", "--format", "json"))
	got["context --max-bytes"] = shapeOf(t, out("context", "--format", "json", "--max-bytes", "100000"))
	got["lint"] = shapeOf(t, out("lint", "--json"))

	pinned := initRoot(t)
	writeConfig(t, pinned, pinConfig(1))
	got["contract"] = shapeOf(t, out("contract", "--json"), run(testEnv(pinned), "", "contract", "--json").stdout)
	got["version"] = shapeOf(t, out("version", "--json"))
	got["error"] = shapeOf(t, out("show", "999", "--json"), out("--bogus", "--json"))

	renv, cfgPath, repo := reposEnv(t)
	for _, args := range [][]string{{"init"}, {"ask", "An open question", "--option", "a:A"}} {
		if r := run(renv, "", args...); r.code != 0 {
			t.Fatalf("%v: exit %d: %s", args, r.code, r.stderr)
		}
	}
	missing := filepath.Join(filepath.Dir(repo), "missing")
	writeReposConfig(t, cfgPath, `{"repos":[`+quoteRepo(repo)+`,`+quoteRepo(missing)+`]}`)
	got["inbox --all"] = shapeOf(t, run(renv, "", "inbox", "--all", "--json").stdout)
	got["repos list"] = shapeOf(t, run(renv, "", "repos", "list", "--json").stdout)
	return got
}

// shapeLosses lists what recorded has that current lacks or has with other
// types: each is a change that is not purely additive. A path current adds is
// not one.
func shapeLosses(recorded, current shapes) []string {
	var out []string
	for cmd, paths := range recorded {
		for path, types := range paths {
			now, ok := current[cmd][path]
			switch {
			case !ok:
				out = append(out, fmt.Sprintf("%s: %s (%s) is gone", cmd, path, types))
			case now != types:
				out = append(out, fmt.Sprintf("%s: %s was %s and is now %s", cmd, path, types, now))
			}
		}
	}
	sort.Strings(out)
	return out
}

func readShapes(t *testing.T) shapesRecord {
	t.Helper()
	data, err := os.ReadFile(shapesFile)
	if err != nil {
		t.Fatal(err)
	}
	var rec shapesRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("%s: %v", shapesFile, err)
	}
	return rec
}

// The recorded shapes belong to contract.Version, and the output still has all
// of them. Removing, renaming or retyping a key fails here until
// contract.Version is raised, the shapes are recorded again for the new
// version and docs/json.md has a line for it.
func TestContractShapesAreTiedToTheVersion(t *testing.T) {
	rec := readShapes(t)
	if rec.Version != contract.Version {
		t.Fatalf("%s records contract version %d but contract.Version is %d: record the shapes for the new version "+
			"(ADJ_RECORD_SHAPES=1 go test ./internal/cli -run TestRecordContractShapes) and describe it under \"Contract version\" in docs/json.md",
			shapesFile, rec.Version, contract.Version)
	}
	if lost := shapeLosses(rec.Shapes, currentShapes(t)); len(lost) > 0 {
		t.Errorf("the JSON output lost or retyped keys that contract version %d promises; "+
			"a change that is not purely additive needs a new contract.Version:\n  %s", contract.Version, strings.Join(lost, "\n  "))
	}
}

// The rule itself: a key that is removed, renamed or retyped is a loss, a key
// that is added is not.
func TestShapeLossesFindsWhatIsNotAdditive(t *testing.T) {
	recorded := shapes{"list": {"$": "object", "$.records": "array", "$.records[].id": "number", "$.records[].key": "string"}}
	tests := []struct {
		name    string
		current shapes
		want    []string
	}{
		{"the same", recorded, nil},
		{"a key added", shapes{"list": {"$": "object", "$.records": "array", "$.records[].id": "number", "$.records[].key": "string", "$.records[].new": "string"}}, nil},
		{"a command added", shapes{"list": recorded["list"], "other": {"$": "object"}}, nil},
		{"a key removed", shapes{"list": {"$": "object", "$.records": "array", "$.records[].id": "number"}},
			[]string{"list: $.records[].key (string) is gone"}},
		{"a key renamed", shapes{"list": {"$": "object", "$.records": "array", "$.records[].id": "number", "$.records[].name": "string"}},
			[]string{"list: $.records[].key (string) is gone"}},
		{"a key retyped", shapes{"list": {"$": "object", "$.records": "array", "$.records[].id": "string", "$.records[].key": "string"}},
			[]string{"list: $.records[].id was number and is now string"}},
		{"a type widened", shapes{"list": {"$": "object", "$.records": "array", "$.records[].id": "null|number", "$.records[].key": "string"}},
			[]string{"list: $.records[].id was number and is now null|number"}},
		{"a command removed", shapes{}, []string{"list: $ (object) is gone", "list: $.records (array) is gone",
			"list: $.records[].id (number) is gone", "list: $.records[].key (string) is gone"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shapeLosses(recorded, tt.current)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("losses = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShapeOfMergesTypesAcrossOutputs(t *testing.T) {
	got := shapeOf(t, `{"a": null, "b": [{"c": 1}]}`, `{"a": "x", "b": []}`)
	want := map[string]string{"$": "object", "$.a": "null|string", "$.b": "array", "$.b[]": "object", "$.b[].c": "number"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("shape = %v, want %v", got, want)
	}
}

// The shapes the docs promise are the shapes the fixture holds: every command
// the fixture records has its section or is named in docs/json.md.
func TestRecordedShapesCoverTheDocumentedCommands(t *testing.T) {
	rec := readShapes(t)
	doc := docsText(t)
	for cmd := range rec.Shapes {
		if cmd == "error" { // the error object has its own section, "Errors"
			continue
		}
		if !strings.Contains(doc, "adj "+strings.Fields(cmd)[0]) {
			t.Errorf("docs/json.md does not mention the command of the recorded shape %q", cmd)
		}
	}
	for _, want := range []string{"show", "list", "inbox", "check", "context", "context --max-bytes", "lint", "inbox --all",
		"repos list", "contract", "version", "error"} {
		if len(rec.Shapes[want]) == 0 {
			t.Errorf("no shape is recorded for %q", want)
		}
	}
}

// ADJ_RECORD_SHAPES=1 writes the file. It refuses to replace the shapes of the
// current version with shapes that lost something.
func TestRecordContractShapes(t *testing.T) {
	if os.Getenv("ADJ_RECORD_SHAPES") == "" {
		t.Skip("set ADJ_RECORD_SHAPES=1 to record " + shapesFile)
	}
	cur := currentShapes(t)
	if old, err := os.ReadFile(shapesFile); err == nil {
		var prev shapesRecord
		if err := json.Unmarshal(old, &prev); err != nil {
			t.Fatal(err)
		}
		if lost := shapeLosses(prev.Shapes, cur); prev.Version == contract.Version && len(lost) > 0 {
			t.Fatalf("not recording: contract version %d would lose\n  %s\nraise contract.Version first", prev.Version, strings.Join(lost, "\n  "))
		}
	}
	data, err := json.MarshalIndent(shapesRecord{Version: contract.Version, Shapes: cur}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shapesFile, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
