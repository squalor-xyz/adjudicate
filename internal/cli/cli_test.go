// SPDX-License-Identifier: GPL-3.0-or-later

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// hasUsageLine reports whether s has a line that starts with "usage: adj".
func hasUsageLine(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "usage: adj") {
			return true
		}
	}
	return false
}

func TestRunHelpPrintsUsageToStdout(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "long flag", args: []string{"--help"}},
		{name: "short flag", args: []string{"-h"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if !hasUsageLine(stdout.String()) {
				t.Errorf("stdout has no line starting %q:\n%s", "usage: adj", stdout.String())
			}
			if stderr.Len() != 0 {
				t.Errorf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRunUsageErrorsPrintUsageToStderr(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantErrText   string
		forbidErrText string
	}{
		{name: "no arguments", args: nil, forbidErrText: "unknown command"},
		{name: "unknown subcommand", args: []string{"frobnicate"}, wantErrText: `unknown command "frobnicate"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(tt.args, strings.NewReader(""), &stdout, &stderr)
			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if !hasUsageLine(stderr.String()) {
				t.Errorf("stderr has no line starting %q:\n%s", "usage: adj", stderr.String())
			}
			if tt.wantErrText != "" && !strings.Contains(stderr.String(), tt.wantErrText) {
				t.Errorf("stderr does not contain %q:\n%s", tt.wantErrText, stderr.String())
			}
			if tt.forbidErrText != "" && strings.Contains(stderr.String(), tt.forbidErrText) {
				t.Errorf("stderr contains %q, want no such text:\n%s", tt.forbidErrText, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("stdout = %q, want empty", stdout.String())
			}
		})
	}
}
