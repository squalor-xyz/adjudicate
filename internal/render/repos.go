// SPDX-License-Identifier: GPL-3.0-or-later

package render

import (
	"fmt"
	"io"
)

// ReposText writes one stored repo path per line.
func ReposText(w io.Writer, repos []string) error {
	for _, repo := range repos {
		if _, err := fmt.Fprintln(w, repo); err != nil {
			return err
		}
	}
	return nil
}

// ReposJSON writes the repo list, with an empty array rather than null.
func ReposJSON(w io.Writer, repos []string) error {
	if repos == nil {
		repos = []string{}
	}
	return encodeJSON(w, struct {
		Repos []string `json:"repos"`
	}{repos})
}
