// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"errors"
	"flag"
	"io"
	"reflect"
	"strings"
	"testing"
)

// newFlags returns a FlagSet with a string flag key, a bool flag stdin and a
// repeatable flag tag, the shapes the commands use.
func newFlags() (fs *flag.FlagSet, key *string, stdin *bool, tags *stringList) {
	fs = flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	key = fs.String("key", "", "")
	stdin = fs.Bool("stdin", false, "")
	tags = new(stringList)
	fs.Var(tags, "tag", "")
	return fs, key, stdin, tags
}

func TestParseInterspersed(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPos   []string
		wantKey   string
		wantStdin bool
		wantTags  []string
	}{
		{name: "no arguments"},
		{
			name:    "flags before the positional",
			args:    []string{"--key", "k", "Title"},
			wantPos: []string{"Title"}, wantKey: "k",
		},
		{
			name:    "flags after the positional",
			args:    []string{"Title", "--key", "k", "--tag", "a"},
			wantPos: []string{"Title"}, wantKey: "k", wantTags: []string{"a"},
		},
		{
			name:    "flags on both sides",
			args:    []string{"--tag", "a", "Title", "--tag", "b"},
			wantPos: []string{"Title"}, wantTags: []string{"a", "b"},
		},
		{
			name:    "several positionals keep their order",
			args:    []string{"one", "--key", "k", "two", "three"},
			wantPos: []string{"one", "two", "three"}, wantKey: "k",
		},
		{
			name:    "equals form and single dash",
			args:    []string{"Title", "--key=k", "-tag", "a", "-tag=b"},
			wantPos: []string{"Title"}, wantKey: "k", wantTags: []string{"a", "b"},
		},
		{
			name:    "a bool flag does not take the next argument",
			args:    []string{"--stdin", "Title"},
			wantPos: []string{"Title"}, wantStdin: true,
		},
		{
			name:    "a bool flag after the positional",
			args:    []string{"Title", "--stdin"},
			wantPos: []string{"Title"}, wantStdin: true,
		},
		{
			name:    "explicit bool value",
			args:    []string{"Title", "--stdin=false"},
			wantPos: []string{"Title"},
		},
		{
			name:    "double dash ends the flags",
			args:    []string{"--key", "k", "--", "--weird", "-x"},
			wantPos: []string{"--weird", "-x"}, wantKey: "k",
		},
		{
			name:    "double dash is a value when a flag takes it",
			args:    []string{"--key", "--", "Title"},
			wantPos: []string{"Title"}, wantKey: "--",
		},
		{
			name:    "a flag value may look like a flag",
			args:    []string{"Title", "--key", "-x"},
			wantPos: []string{"Title"}, wantKey: "-x",
		},
		{
			name:    "a lone dash is a positional",
			args:    []string{"-", "--key", "k"},
			wantPos: []string{"-"}, wantKey: "k",
		},
		{
			name:    "the last repeated value of a plain flag wins",
			args:    []string{"--key", "a", "Title", "--key", "b"},
			wantPos: []string{"Title"}, wantKey: "b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, key, stdin, tags := newFlags()
			pos, err := parseInterspersed(fs, tt.args)
			if err != nil {
				t.Fatalf("parseInterspersed(%q) error = %v", tt.args, err)
			}
			if !reflect.DeepEqual(pos, tt.wantPos) {
				t.Errorf("positionals = %q, want %q", pos, tt.wantPos)
			}
			if *key != tt.wantKey {
				t.Errorf("key = %q, want %q", *key, tt.wantKey)
			}
			if *stdin != tt.wantStdin {
				t.Errorf("stdin = %v, want %v", *stdin, tt.wantStdin)
			}
			if !reflect.DeepEqual([]string(*tags), tt.wantTags) {
				t.Errorf("tags = %q, want %q", []string(*tags), tt.wantTags)
			}
		})
	}
}

func TestParseInterspersedErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string // substring; "" means errors.Is(err, flag.ErrHelp)
	}{
		{name: "unknown flag", args: []string{"Title", "--nope"}, wantErr: "nope"},
		{name: "unknown flag before positional", args: []string{"--nope", "x", "Title"}, wantErr: "nope"},
		{name: "missing value at the end", args: []string{"Title", "--key"}, wantErr: "key"},
		{name: "help", args: []string{"Title", "-h"}},
		{name: "long help", args: []string{"--help"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, _, _, _ := newFlags()
			_, err := parseInterspersed(fs, tt.args)
			if err == nil {
				t.Fatalf("parseInterspersed(%q) error = nil, want an error", tt.args)
			}
			if tt.wantErr == "" {
				if !errors.Is(err, flag.ErrHelp) {
					t.Errorf("error = %v, want flag.ErrHelp", err)
				}
				return
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestStringListKeepsEveryValueInOrder(t *testing.T) {
	var l stringList
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(&l, "o", "")
	args := []string{"-o", "a:one", "-o", "b:two:three", "-o", "a:one"}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	want := []string{"a:one", "b:two:three", "a:one"}
	if !reflect.DeepEqual([]string(l), want) {
		t.Errorf("list = %q, want %q", []string(l), want)
	}
	if got := l.String(); got != strings.Join(want, ",") {
		t.Errorf("String() = %q, want %q", got, strings.Join(want, ","))
	}
	var nilList *stringList
	if got := nilList.String(); got != "" {
		t.Errorf("nil String() = %q, want empty", got)
	}
}
