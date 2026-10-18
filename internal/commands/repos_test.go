// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
