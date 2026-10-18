// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/render"
	"github.com/squalor-xyz/adjudicate/internal/store"
)

func TestRefFilters(t *testing.T) {
	for _, command := range []string{"list", "context"} {
		t.Run(command, func(t *testing.T) {
			type refCase struct {
				name string
				refs []string
				args []string
				want bool
			}
			tests := []refCase{
				{"exact", []string{"S20"}, nil, true},
				{"prefix excluded", []string{"S200"}, nil, false},
				{"case excluded", []string{"s20"}, nil, false},
				{"no refs", nil, nil, false},
				{"stored spaces preserved", []string{" S20 "}, nil, false},
				{"filter spaces preserved", []string{"S20"}, []string{"--ref", " S20 "}, false},
				{"spaces match", []string{"S20", " S20 "}, []string{"--ref", " S20 "}, true},
				{"all refs", []string{"S20", "C12"}, []string{"--ref", "C12"}, true},
				{"missing second ref", []string{"S20"}, []string{"--ref", "C12"}, false},
				{"duplicate ref", []string{"S20"}, []string{"--ref", "S20"}, true},
				{"tag matches", []string{"S20"}, []string{"--tag", "topic"}, true},
				{"tag excludes", []string{"S20"}, []string{"--tag", "other"}, false},
			}
			if command == "list" {
				tests = append(tests,
					refCase{"status matches", []string{"S20"}, []string{"--status", "decided"}, true},
					refCase{"status excludes", []string{"S20"}, []string{"--status", "open"}, false},
					refCase{"key matches", []string{"S20"}, []string{"--key", "area.topic"}, true},
					refCase{"key excludes", []string{"S20"}, []string{"--key", "other"}, false},
				)
			} else {
				tests = append(tests,
					refCase{"prefix matches", []string{"S20"}, []string{"--key-prefix", "area"}, true},
					refCase{"prefix excludes", []string{"S20"}, []string{"--key-prefix", "other"}, false},
				)
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					root := initRoot(t)
					st, err := store.Open(root)
					if err != nil {
						t.Fatal(err)
					}
					rec := model.Record{Title: "Ref record", Status: model.StatusDecided, Created: "2026-10-18", DecidedAt: "2026-10-18", Decision: "yes", Rationale: "reason", Key: "area.topic", Tags: []string{"topic"}, Refs: tt.refs}
					if _, err := st.Create(rec); err != nil {
						t.Fatal(err)
					}
					args := []string{command, "--ref", "S20"}
					if command == "list" {
						args = append(args, "--json")
					} else {
						args = append(args, "--format", "json")
					}
					r := run(testEnv(root), "", append(args, tt.args...)...)
					if r.code != 0 || r.stderr != "" {
						t.Fatalf("exit %d, stderr %s", r.code, r.stderr)
					}
					key := "records"
					if command == "context" {
						key = "decisions"
					}
					var out map[string][]json.RawMessage
					if err := json.Unmarshal([]byte(r.stdout), &out); err != nil {
						t.Fatal(err)
					}
					want := 0
					if tt.want {
						want = 1
					}
					if len(out) != 1 || out[key] == nil || len(out[key]) != want {
						t.Fatalf("output %s; want only %s with %d entries", r.stdout, key, want)
					}
					if tt.want {
						var got map[string]any
						if err := json.Unmarshal(out[key][0], &got); err != nil {
							t.Fatal(err)
						}
						if got["id"] != float64(1) {
							t.Errorf("id = %v, want 1", got["id"])
						}
						if command == "list" {
							shown := run(testEnv(root), "", "show", "1", "--json")
							var show map[string]any
							if err := json.Unmarshal([]byte(shown.stdout), &show); err != nil {
								t.Fatal(err)
							}
							if !reflect.DeepEqual(got, show) {
								t.Errorf("list record %s differs from show %s", out[key][0], shown.stdout)
							}
						}
					}
				})
			}
		})
	}
}

func TestRefBlankBeforeStore(t *testing.T) {
	for _, command := range []string{"list", "context"} {
		for _, value := range []string{"", " \t "} {
			t.Run(command+fmt.Sprintf("/%q", value), func(t *testing.T) {
				r := run(testEnv(t.TempDir()), "", command, "--ref", value)
				if r.code != 2 || r.stdout != "" || !strings.Contains(r.stderr, "--ref must not be blank") {
					t.Errorf("exit %d, stdout %q, stderr %q; want blank ref usage error", r.code, r.stdout, r.stderr)
				}
			})
		}
	}
}

func TestRefHelp(t *testing.T) {
	for _, command := range []string{"list", "context"} {
		t.Run(command, func(t *testing.T) {
			r := run(testEnv(t.TempDir()), "", command, "--help")
			if r.code != 0 || !strings.Contains(r.stdout, "--ref") {
				t.Errorf("help missing --ref: exit %d, %s", r.code, r.stdout)
			}
		})
	}
}

func TestContextRefBeforeBudget(t *testing.T) {
	root := initRoot(t)
	st, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	rec := model.Record{Title: "Kept", Status: model.StatusDecided, Created: "2026-10-18", DecidedAt: "2026-10-18", Decision: "yes", Rationale: "reason", Refs: []string{"S20"}}
	if _, err := st.Create(rec); err != nil {
		t.Fatal(err)
	}
	var want strings.Builder
	rec.ID = 1
	if err := render.ContextTextOmitted(&want, []model.Record{rec}, 0); err != nil {
		t.Fatal(err)
	}
	rec.Title = "Unrelated newer record"
	rec.Refs = []string{"S200"}
	rec.ID = 0
	if _, err := st.Create(rec); err != nil {
		t.Fatal(err)
	}
	r := run(testEnv(root), "", "context", "--ref", "S20", "--max-bytes", strconv.Itoa(want.Len()))
	if r.code != 0 || r.stdout != want.String() || r.stderr != "" {
		t.Errorf("exit %d, output %q, stderr %q; want %q", r.code, r.stdout, r.stderr, want.String())
	}
}
