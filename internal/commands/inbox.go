// SPDX-License-Identifier: GPL-3.0-or-later

package commands

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/squalor-xyz/adjudicate/internal/model"
	"github.com/squalor-xyz/adjudicate/internal/query"
	"github.com/squalor-xyz/adjudicate/internal/store"
	"github.com/squalor-xyz/adjudicate/internal/userconfig"
)

// Inbox returns the open records of st, oldest first: by created date, then
// by ID. An empty inbox is an empty result, not an error. Store failures are
// the store's errors (C61).
func Inbox(st *store.Store) ([]model.Record, error) {
	recs, err := st.List()
	if err != nil {
		return nil, err
	}
	return oldestFirst(query.Select(recs, query.Filter{Status: model.StatusOpen})), nil
}

// oldestFirst returns a copy of recs ordered by created date (YYYY-MM-DD, so
// text order is date order), then by ID. The sort is stable, so records that
// share an ID (C60) keep the order given. recs is not changed.
func oldestFirst(recs []model.Record) []model.Record {
	out := slices.Clone(recs)
	slices.SortStableFunc(out, func(a, b model.Record) int {
		return cmp.Or(cmp.Compare(a.Created, b.Created), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// RepoInbox is a successfully read repo, including those with no open records.
type RepoInbox struct {
	Root    string
	Records []model.Record
}

// InboxError identifies a repo that could not be read.
type InboxError struct {
	Root  string `json:"root"`
	Error string `json:"error"`
}

// AllInbox keeps config order and collects failures without losing other repos.
type AllInbox struct {
	Repos  []RepoInbox
	Errors []InboxError
}

// InboxAll reads only the listed roots, never discovering a parent repo.
func InboxAll(getenv func(string) string) (AllInbox, error) {
	out := AllInbox{Repos: []RepoInbox{}, Errors: []InboxError{}}
	path, err := userconfig.Path(getenv)
	if err != nil {
		return out, err
	}
	cfg, err := userconfig.Load(path)
	if err != nil {
		return out, err
	}
	for _, root := range cfg.Repos {
		recs, err := repoInbox(root)
		if err != nil {
			out.Errors = append(out.Errors, InboxError{Root: root, Error: err.Error()})
			continue
		}
		out.Repos = append(out.Repos, RepoInbox{Root: root, Records: recs})
	}
	return out, nil
}

func repoInbox(root string) ([]model.Record, error) {
	marker := filepath.Join(root, store.ConfigFile)
	fi, err := os.Stat(marker)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", marker)
	}
	st, err := store.Open(root)
	if err != nil {
		return nil, err
	}
	return Inbox(st)
}
