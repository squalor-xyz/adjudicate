// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// question returns a valid open record. Slices stay nil: an empty non-nil
// slice does not survive a write and read (omitempty), so DeepEqual would fail.
func question(id int, title string) model.Record {
	return model.Record{ID: id, Title: title, Status: model.StatusOpen, Created: "2026-10-01"}
}

// seed writes a record file named name into dir, creating dir.
func seed(t *testing.T, dir, name string, r model.Record) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := model.WriteFile(filepath.Join(dir, name), r); err != nil {
		t.Fatal(err)
	}
}

// names lists the entries of dir, sorted by os.ReadDir, without the lock file
// that Create and Renumber leave in a store directory (C114).
func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.Name() != lockName {
			out = append(out, e.Name())
		}
	}
	return out
}

func mustOpen(t *testing.T, root string) *Store {
	t.Helper()
	s, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCreateEmptyStore(t *testing.T) {
	tests := []struct {
		title string
		file  string
	}{
		{"Store format", "0001-store-format.md"},
		{"  Use Go: stdlib only (C24)!  ", "0001-use-go-stdlib-only-c24.md"},
		{"UPPER and lower 42", "0001-upper-and-lower-42.md"},
		{"snake_case--and   gaps", "0001-snake-case-and-gaps.md"},
		{"Café déjà vu", "0001-caf-d-j-vu.md"},
		{"???", "0001-untitled.md"},
		{"日本語", "0001-untitled.md"},
		{strings.Repeat("abcd ", 20), "0001-" + strings.TrimSuffix(strings.Repeat("abcd-", 12), "-") + ".md"},
		{strings.Repeat("a", 59) + " bcd", "0001-" + strings.Repeat("a", 59) + ".md"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			root := t.TempDir()
			s := mustOpen(t, root)
			got, err := s.Create(question(0, tt.title))
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != 1 {
				t.Errorf("ID = %d, want 1", got.ID)
			}
			dir := filepath.Join(root, "decisions")
			if want := []string{tt.file}; !reflect.DeepEqual(names(t, dir), want) {
				t.Errorf("files = %v, want %v", names(t, dir), want)
			}
			disk, err := model.ReadFile(filepath.Join(dir, tt.file))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(disk, got) {
				t.Errorf("file holds %+v, Create returned %+v", disk, got)
			}
		})
	}
}

func TestCreateAssignsIDIgnoringInput(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	got, err := s.Create(question(42, "Whatever"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 {
		t.Errorf("ID = %d, want 1 (the caller's ID is ignored)", got.ID)
	}
}

func TestCreateSequence(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	for want := 1; want <= 3; want++ {
		got, err := s.Create(question(0, "Same title"))
		if err != nil {
			t.Fatal(err)
		}
		if got.ID != want {
			t.Fatalf("ID = %d, want %d", got.ID, want)
		}
	}
	if want := []string{"0001-same-title.md", "0002-same-title.md", "0003-same-title.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v", names(t, s.Dir()), want)
	}
}

func TestNextID(t *testing.T) {
	tests := []struct {
		name string
		// files are the names of the empty files to write; a record's ID
		// comes from its name.
		files    []string
		wantID   int
		wantFile string
	}{
		{"one past the highest", []string{"0001-a.md", "0002-b.md", "0005-c.md"}, 6, "0006-t.md"},
		{"order of creation does not matter", []string{"0005-c.md", "0001-a.md", "0002-b.md"}, 6, "0006-t.md"},
		{"above 9999 keeps every digit", []string{"9999-a.md"}, 10000, "10000-t.md"},
		{"beyond that too", []string{"10000-a.md", "9999-b.md"}, 10001, "10001-t.md"},
		{"files that are not records are ignored", []string{
			"0002-a.md",
			"README.md", "notes.txt", "0100-x.txt", "0100.md", "100-x.md", "abc-x.md", "-x.md",
			"0100-x.md.bak", ".0100-x.md", "x0100-x.md", "0000-zero.md", "0100-.md",
		}, 3, "0003-t.md"},
		{"a record that fails to parse still holds its ID", []string{"0002-a.md", "0007-garbage.md"}, 8, "0008-t.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "decisions")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, f := range tt.files {
				writeFile(t, filepath.Join(dir, f), "not a record\n")
			}
			// A directory named like a record file is not a record either.
			if err := os.Mkdir(filepath.Join(dir, "0099-dir.md"), 0o755); err != nil {
				t.Fatal(err)
			}
			s := mustOpen(t, root)
			got, err := s.Create(question(0, "t"))
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", got.ID, tt.wantID)
			}
			if _, err := os.Stat(filepath.Join(dir, tt.wantFile)); err != nil {
				t.Errorf("want file %s: %v", tt.wantFile, err)
			}
		})
	}
}

func TestCreateRejectsInvalidRecordAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	s := mustOpen(t, root)
	_, err := s.Create(question(0, "   "))
	if !errors.Is(err, model.ErrInvalid) {
		t.Fatalf("err = %v, want model.ErrInvalid", err)
	}
	if _, err := os.Stat(filepath.Join(root, "decisions")); !os.IsNotExist(err) {
		t.Errorf("decisions/ exists after a failed Create (err = %v)", err)
	}
}

// A link in the store to an ID that has no file must not come to resolve to
// a new record: Create skips such IDs, as Renumber does (C155).
func TestCreateSkipsIDsNamedByDanglingLinks(t *testing.T) {
	tests := []struct {
		name     string
		link     func(r *model.Record)
		wantID   int
		wantFile string
	}{
		{"supersedes", func(r *model.Record) { r.Supersedes = []int{2} }, 3, "0003-t.md"},
		{"superseded_by", func(r *model.Record) { r.SupersededBy = 2 }, 3, "0003-t.md"},
		{"a run of them", func(r *model.Record) { r.Supersedes = []int{2, 3, 4} }, 5, "0005-t.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "decisions")
			a := question(1, "A")
			tt.link(&a)
			seed(t, dir, "0001-a.md", a)
			got, err := mustOpen(t, root).Create(question(0, "t"))
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", got.ID, tt.wantID)
			}
			if _, err := os.Stat(filepath.Join(dir, tt.wantFile)); err != nil {
				t.Errorf("want file %s: %v", tt.wantFile, err)
			}
		})
	}
}

// A file that cannot be read still reserves its ID by name (C59), but it can
// hold no link Create knows about; Create goes on without it, and still avoids
// the links of the files it can read, rather than failing every ask on a store
// with one broken file.
func TestCreateGoesOnPastAFileThatDoesNotParse(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	a := question(1, "A")
	a.Supersedes = []int{3}
	seed(t, dir, "0001-a.md", a)
	writeFile(t, filepath.Join(dir, "0002-broken.md"), "not a record\n")
	got, err := mustOpen(t, root).Create(question(0, "t"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 4 {
		t.Errorf("ID = %d, want 4 (past the broken file 0002 and the link from 0001 to 3)", got.ID)
	}
}

// The record is checked with the ID it gets, not with a stand-in: a link to 1 is
// fine on a record that becomes 2.
func TestCreateChecksTheRecordWithItsRealID(t *testing.T) {
	root := t.TempDir()
	seed(t, filepath.Join(root, "decisions"), "0001-a.md", question(1, "A"))
	r := question(0, "B")
	r.Supersedes = []int{1}
	got, err := mustOpen(t, root).Create(r)
	if err != nil || got.ID != 2 {
		t.Errorf("Create = %d, %v; want ID 2", got.ID, err)
	}
}

func TestStoreLocation(t *testing.T) {
	tests := []struct {
		name   string
		config *string
		want   string // relative to the root
	}{
		{"default", nil, "decisions"},
		{"configured relative path", ptr(`{"store": "docs/log"}`), filepath.Join("docs", "log")},
		{"configured root itself", ptr(`{"store": "."}`), "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if tt.config != nil {
				writeFile(t, filepath.Join(root, ConfigFile), *tt.config)
			}
			s := mustOpen(t, root)
			wantDir := filepath.Join(root, tt.want)
			if s.Dir() != wantDir {
				t.Errorf("Dir = %q, want %q", s.Dir(), wantDir)
			}
			if _, err := s.Create(question(0, "Where")); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(wantDir, "0001-where.md")); err != nil {
				t.Error(err)
			}
			// The default store is not used when the config names another.
			if tt.config != nil && tt.want != "decisions" {
				if _, err := os.Stat(filepath.Join(root, "decisions")); !os.IsNotExist(err) {
					t.Errorf("decisions/ exists (err = %v)", err)
				}
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "file")
	writeFile(t, filePath, "x")
	badCfg := filepath.Join(tmp, "badcfg")
	writeFile(t, filepath.Join(badCfg, ConfigFile), `{"store": "/abs"}`)

	if _, err := Open(filepath.Join(tmp, "missing")); err == nil {
		t.Error("Open of a missing root: want an error")
	}
	if _, err := Open(filePath); err == nil {
		t.Error("Open of a file: want an error")
	}
	if _, err := Open(badCfg); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Open with a bad config: err = %v, want ErrInvalidConfig", err)
	}
}

func TestListSortedByID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	// Written out of order. By file name "10000-..." sorts before "9999-...",
	// so a name sort is not an ID sort.
	seed(t, dir, "10000-e.md", question(10000, "E"))
	seed(t, dir, "0002-b.md", question(2, "B"))
	seed(t, dir, "9999-d.md", question(9999, "D"))
	seed(t, dir, "0010-c.md", question(10, "C"))
	seed(t, dir, "0001-a.md", question(1, "A"))
	writeFile(t, filepath.Join(dir, "README.md"), "# not a record\n")
	// ID 0 is not a record.
	writeFile(t, filepath.Join(dir, "0000-zero.md"), "not a record\n")
	s := mustOpen(t, root)

	var first []model.Record
	for i := 0; i < 3; i++ {
		got, err := s.List()
		if err != nil {
			t.Fatal(err)
		}
		var ids []int
		for _, r := range got {
			ids = append(ids, r.ID)
		}
		if want := []int{1, 2, 10, 9999, 10000}; !reflect.DeepEqual(ids, want) {
			t.Fatalf("IDs = %v, want %v", ids, want)
		}
		if i == 0 {
			first = got
		} else if !reflect.DeepEqual(got, first) {
			t.Fatalf("call %d differs from call 0", i)
		}
	}
}

func TestListEmpty(t *testing.T) {
	root := t.TempDir()
	s := mustOpen(t, root)
	got, err := s.List() // decisions/ does not exist yet
	if err != nil || len(got) != 0 {
		t.Fatalf("missing store dir: got %v, %v; want no records and no error", got, err)
	}
	if err := os.Mkdir(s.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err = s.List()
	if err != nil || len(got) != 0 {
		t.Fatalf("empty store dir: got %v, %v; want no records and no error", got, err)
	}
}

func TestListAndGetBadFiles(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content func(t *testing.T, path string)
		wantIs  error
	}{
		{"malformed file", "0003-bad.md", func(t *testing.T, p string) { writeFile(t, p, "no frontmatter\n") }, model.ErrInvalid},
		{"record ID differs from the file name", "0003-other.md", func(t *testing.T, p string) {
			if err := model.WriteFile(p, question(4, "Other")); err != nil {
				t.Fatal(err)
			}
		}, model.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "decisions")
			seed(t, dir, "0001-ok.md", question(1, "Ok"))
			tt.content(t, filepath.Join(dir, tt.file))
			s := mustOpen(t, root)

			_, err := s.List()
			if !errors.Is(err, tt.wantIs) || !strings.Contains(err.Error(), tt.file) {
				t.Errorf("List err = %v, want %v naming %s", err, tt.wantIs, tt.file)
			}
			_, err = s.Get(3)
			if !errors.Is(err, tt.wantIs) || !strings.Contains(err.Error(), tt.file) {
				t.Errorf("Get(3) err = %v, want %v naming %s", err, tt.wantIs, tt.file)
			}
			// A bad file does not stop Get of another record.
			if got, err := s.Get(1); err != nil || got.Title != "Ok" {
				t.Errorf("Get(1) = %+v, %v", got, err)
			}
		})
	}
}

func TestGet(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	want := question(2, "Two")
	want.Body = "Free text.\n"
	seed(t, dir, "0001-one.md", question(1, "One"))
	seed(t, dir, "0002-two.md", want)
	seed(t, dir, "9999-big.md", question(9999, "Big"))
	// ID 0 is not a record.
	writeFile(t, filepath.Join(dir, "0000-zero.md"), "not a record\n")
	s := mustOpen(t, root)

	for _, id := range []int{0, -1, 3, 10000} {
		if _, err := s.Get(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Get(%d) err = %v, want ErrNotFound", id, err)
		}
	}
	got, err := s.Get(2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Get(2) = %+v, want %+v", got, want)
	}
}

func TestDuplicateIDs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	seed(t, dir, "0002-z.md", question(2, "Z"))
	seed(t, dir, "0002-b.md", question(2, "B"))
	s := mustOpen(t, root)

	// Detecting and fixing collisions is lint's and renumber's job; the store
	// lists both files in a fixed order and refuses to pick one for Get/Update.
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, r := range got {
		titles = append(titles, r.Title)
	}
	if want := []string{"A", "B", "Z"}; !reflect.DeepEqual(titles, want) {
		t.Errorf("titles = %v, want %v", titles, want)
	}
	_, err = s.Get(2)
	if !errors.Is(err, ErrDuplicateID) || !strings.Contains(err.Error(), "0002-b.md") || !strings.Contains(err.Error(), "0002-z.md") {
		t.Errorf("Get(2) err = %v, want ErrDuplicateID naming both files", err)
	}
	if err := s.Update(question(2, "New")); !errors.Is(err, ErrDuplicateID) {
		t.Errorf("Update err = %v, want ErrDuplicateID", err)
	}
	// The next ID is still one past the highest.
	if r, err := s.Create(question(0, "N")); err != nil || r.ID != 3 {
		t.Errorf("Create = %+v, %v; want ID 3", r, err)
	}
}

func TestUpdate(t *testing.T) {
	root := t.TempDir()
	s := mustOpen(t, root)
	created, err := s.Create(question(0, "Original title"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(question(0, "Bystander")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(s.Dir(), "0002-bystander.md"))
	if err != nil {
		t.Fatal(err)
	}

	changed := created
	changed.Title = "A completely different title"
	changed.Tags = []string{"x"}
	changed.Body = "Added body.\n"
	if err := s.Update(changed); err != nil {
		t.Fatal(err)
	}

	// Same path as before: the title changed but the file was not renamed.
	if want := []string{"0001-original-title.md", "0002-bystander.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v", names(t, s.Dir()), want)
	}
	got, err := s.Get(1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, changed) {
		t.Errorf("Get(1) = %+v, want %+v", got, changed)
	}
	after, err := os.ReadFile(filepath.Join(s.Dir(), "0002-bystander.md"))
	if err != nil || string(after) != string(before) {
		t.Errorf("other record changed (err = %v)", err)
	}
}

func TestUpdateErrors(t *testing.T) {
	root := t.TempDir()
	s := mustOpen(t, root)
	created, err := s.Create(question(0, "Keep"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir(), "0001-keep.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	invalid := created
	invalid.Status = "bogus"
	tests := []struct {
		name string
		rec  model.Record
		is   error
	}{
		{"missing ID", question(7, "Nope"), ErrNotFound},
		{"zero ID", question(0, "Nope"), ErrNotFound},
		{"invalid record", invalid, model.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := s.Update(tt.rec); !errors.Is(err, tt.is) {
				t.Errorf("err = %v, want %v", err, tt.is)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != string(before) {
				t.Errorf("existing file changed (err = %v)", err)
			}
			if want := []string{"0001-keep.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
				t.Errorf("files = %v, want %v (no leftovers)", names(t, s.Dir()), want)
			}
		})
	}
}

func TestWriteNewNeverReplaces(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	if err := os.MkdirAll(s.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir(), "0001-a.md")
	if err := s.writeNew(path, question(1, "First")); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	err := s.writeNew(path, question(1, "Second"))
	if !errors.Is(err, os.ErrExist) || !strings.Contains(err.Error(), path) || strings.Contains(err.Error(), ".adj-") {
		t.Errorf("second writeNew err = %v, want os.ErrExist naming %s and not the temporary file", err, path)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Error("writeNew replaced an existing file")
	}
	if want := []string{"0001-a.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v (no leftovers)", names(t, s.Dir()), want)
	}
}

func TestReplaceLeavesNoTemporaryFile(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	if err := os.MkdirAll(s.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.Dir(), "0001-a.md")
	if err := s.writeNew(path, question(1, "First")); err != nil {
		t.Fatal(err)
	}
	// A second name for the old file keeps its content: a replace that renames
	// a new file over path leaves it alone, one that rewrites in place does not.
	alias := filepath.Join(t.TempDir(), "alias.md")
	if err := os.Link(path, alias); err != nil {
		t.Fatal(err)
	}
	old, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.replace(path, question(1, "Second")); err != nil {
		t.Fatal(err)
	}
	if kept, err := os.ReadFile(alias); err != nil || string(kept) != string(old) {
		t.Errorf("replace rewrote the file in place (err = %v)", err)
	}
	got, err := model.ReadFile(path)
	if err != nil || got.Title != "Second" {
		t.Errorf("file = %+v, %v", got, err)
	}
	if want := []string{"0001-a.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v (no leftovers)", names(t, s.Dir()), want)
	}
	// replace needs an existing record only by convention: it also fails clean
	// when the record is invalid.
	if err := s.replace(path, question(0, "Bad")); !errors.Is(err, model.ErrInvalid) {
		t.Errorf("invalid replace err = %v, want model.ErrInvalid", err)
	}
	if want := []string{"0001-a.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v (no leftovers)", names(t, s.Dir()), want)
	}
}

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Store format", "store-format"},
		{"a", "a"},
		{"--a--", "a"},
		{"A1 b2", "a1-b2"},
		{"", "untitled"},
		{"!!!", "untitled"},
		// The Kelvin sign and the dotted I lower-case to ASCII letters, but
		// only ASCII letters count.
		{"\u212a\u0130 x", "x"},
		{strings.Repeat("x", 100), strings.Repeat("x", 60)},
	}
	for _, tt := range tests {
		if got := slug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no unix permission bits: a file reports 0666, or 0444 when read-only")
	}
	t.Run("Create writes 0644", func(t *testing.T) {
		s := mustOpen(t, t.TempDir())
		if _, err := s.Create(question(0, "Mode")); err != nil {
			t.Fatal(err)
		}
		fi, err := os.Stat(filepath.Join(s.Dir(), "0001-mode.md"))
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.Mode().Perm(); got != 0o644 {
			t.Errorf("mode = %o, want 644", got)
		}
	})
	for _, perm := range []os.FileMode{0o644, 0o640, 0o600} {
		t.Run("Update keeps "+perm.String(), func(t *testing.T) {
			s := mustOpen(t, t.TempDir())
			if _, err := s.Create(question(0, "Mode")); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(s.Dir(), "0001-mode.md")
			if err := os.Chmod(path, perm); err != nil {
				t.Fatal(err)
			}
			if err := s.Update(question(1, "Mode changed")); err != nil {
				t.Fatal(err)
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := fi.Mode().Perm(); got != perm {
				t.Errorf("mode = %o, want %o", got, perm)
			}
		})
	}
}

func TestCreateRetriesWhenPathIsTaken(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	var targets []string
	s.beforeLink = func(path string) {
		targets = append(targets, path)
		if len(targets) == 1 {
			// Another process creates the same file after our scan.
			writeFile(t, path, "taken by another writer\n")
		}
	}
	got, err := s.Create(question(0, "t"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 {
		t.Errorf("ID = %d, want 2 (one past the file that appeared)", got.ID)
	}
	if want := []string{"0001-t.md", "0002-t.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v", names(t, s.Dir()), want)
	}
	if b, _ := os.ReadFile(filepath.Join(s.Dir(), "0001-t.md")); string(b) != "taken by another writer\n" {
		t.Errorf("the other writer's file was changed: %q", b)
	}
	if disk, err := model.ReadFile(filepath.Join(s.Dir(), "0002-t.md")); err != nil || disk.ID != 2 {
		t.Errorf("0002-t.md = %+v, %v", disk, err)
	}
}

func TestCreateGivesUpAfterThreeAttempts(t *testing.T) {
	s := mustOpen(t, t.TempDir())
	var targets []string
	s.beforeLink = func(path string) {
		targets = append(targets, path)
		writeFile(t, path, "taken by another writer\n") // every time
	}
	_, err := s.Create(question(0, "t"))
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("err = %v, want os.ErrExist", err)
	}
	if len(targets) != 3 {
		t.Fatalf("attempts = %d, want 3", len(targets))
	}
	last := targets[2]
	if filepath.Base(last) != "0003-t.md" || !strings.Contains(err.Error(), last) || strings.Contains(err.Error(), ".adj-") {
		t.Errorf("err = %v, want it to name %s and not the temporary file", err, last)
	}
	if want := []string{"0001-t.md", "0002-t.md", "0003-t.md"}; !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v (no leftovers)", names(t, s.Dir()), want)
	}
}

// Files lists the record files by name (C59), in ID then file-name order: a
// file that does not parse keeps its ID, two files may share one, and what is
// not named like a record is left out.
func TestFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0002-b.md", question(2, "B"))
	seed(t, dir, "0001-a.md", question(1, "A"))
	seed(t, dir, "0003-y.md", question(3, "Y"))
	seed(t, dir, "0003-x.md", question(3, "X"))
	writeFile(t, filepath.Join(dir, "0004-broken.md"), "not a record\n")
	for _, ignored := range []string{"notes.md", "0005.md", "005-x.md", "0000-zero.md", "0006-x.txt", ".0007-x.md"} {
		writeFile(t, filepath.Join(dir, ignored), "x\n")
	}
	if err := os.Mkdir(filepath.Join(dir, "0008-dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, root)

	got, err := s.Files()
	if err != nil {
		t.Fatal(err)
	}
	want := []File{
		{1, "0001-a.md", filepath.Join(dir, "0001-a.md")},
		{2, "0002-b.md", filepath.Join(dir, "0002-b.md")},
		{3, "0003-x.md", filepath.Join(dir, "0003-x.md")},
		{3, "0003-y.md", filepath.Join(dir, "0003-y.md")},
		{4, "0004-broken.md", filepath.Join(dir, "0004-broken.md")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Files = %+v\nwant %+v", got, want)
	}

	// A missing store directory is an empty store, as for List.
	if got, err := mustOpen(t, t.TempDir()).Files(); err != nil || len(got) != 0 {
		t.Errorf("Files of a missing store dir = %v, %v; want none and no error", got, err)
	}
}
