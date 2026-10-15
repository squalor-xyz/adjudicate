// SPDX-License-Identifier: GPL-3.0-or-later

package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/squalor-xyz/adjudicate/internal/model"
)

// renumberDate is the date Renumber is given in these tests.
const renumberDate = "2026-11-02"

// fixture copies the store in testdata/<name> to a temp dir and returns its
// root. Renumber writes, so no test runs on the fixture itself.
func fixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// tree maps every file under dir (slash-separated, relative) to its content
// and every directory to "dir", so a test can tell that nothing changed. The
// lock file is left out.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.Name() == lockName {
			return nil // a refusal may leave the lock file it took, which holds no data (C114)
		}
		if d.IsDir() {
			out[filepath.ToSlash(rel)] = "<dir>"
			return nil
		}
		data, err := os.ReadFile(path)
		out[filepath.ToSlash(rel)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func mustRead(t *testing.T, path string) model.Record {
	t.Helper()
	r, err := model.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The fixture "collision" has ID 2 in two files and ID 3 in three. The record
// that keeps an ID is the one created first, and the file name breaks a tie
// (C146); every other file gets the next free ID, colliding IDs in ascending
// order, so the new IDs are 4, 5 and 6.
func TestRenumberGivesTheLosersTheNextFreeIDs(t *testing.T) {
	root := fixture(t, "collision")
	s := mustOpen(t, root)
	dir := s.Dir()
	before := tree(t, root)
	// A permission bit a user chose survives, as for Update (C64).
	if err := os.Chmod(filepath.Join(dir, "0002-beta.md"), 0o640); err != nil {
		t.Fatal(err)
	}

	got, err := s.Renumber(renumberDate, false)
	if err != nil {
		t.Fatal(err)
	}
	wantMoves := []Move{
		{OldID: 2, NewID: 4, OldName: "0002-beta.md", NewName: "0004-beta.md", KeptName: "0002-alpha.md"},
		{OldID: 3, NewID: 5, OldName: "0003-mmm-mid.md", NewName: "0005-mmm-mid.md", KeptName: "0003-zzz-early.md"},
		{OldID: 3, NewID: 6, OldName: "0003-aaa-late.md", NewName: "0006-aaa-late.md", KeptName: "0003-zzz-early.md"},
	}
	if !reflect.DeepEqual(got.Moves, wantMoves) {
		t.Errorf("moves =\n%+v\nwant\n%+v", got.Moves, wantMoves)
	}
	if len(got.Links) != 0 {
		t.Errorf("links = %+v, want none", got.Links)
	}
	wantNames := []string{"0001-intro.md", "0002-alpha.md", "0003-zzz-early.md", "0004-beta.md", "0005-mmm-mid.md", "0006-aaa-late.md"}
	if !reflect.DeepEqual(names(t, dir), wantNames) {
		t.Errorf("files = %v, want %v", names(t, dir), wantNames)
	}

	// The files that keep their IDs are untouched, byte for byte.
	after := tree(t, root)
	for _, kept := range []string{"0001-intro.md", "0002-alpha.md", "0003-zzz-early.md"} {
		key := "decisions/" + kept
		if after[key] != before[key] {
			t.Errorf("%s changed:\n%s\nwas\n%s", kept, after[key], before[key])
		}
	}

	// A moved record has the new ID, one more history entry, and its body.
	moved := mustRead(t, filepath.Join(dir, "0004-beta.md"))
	if moved.ID != 4 || moved.Title != "Beta" || moved.Body != "Beta body.\n" {
		t.Errorf("0004-beta.md = %+v", moved)
	}
	wantHistory := []model.HistoryEntry{
		{Date: "2026-10-02", Event: model.EventAsk},
		{Date: renumberDate, Event: model.EventRenumber, Note: "from 2"},
	}
	if !reflect.DeepEqual(moved.History, wantHistory) {
		t.Errorf("history = %+v, want %+v", moved.History, wantHistory)
	}
	if fi, err := os.Stat(filepath.Join(dir, "0004-beta.md")); err != nil || fi.Mode().Perm() != 0o640 {
		t.Errorf("0004-beta.md mode = %v, %v; want 0640 kept", fi.Mode().Perm(), err)
	}
	if r := mustRead(t, filepath.Join(dir, "0006-aaa-late.md")); r.ID != 6 || r.Title != "Late" || len(r.History) != 1 || r.History[0].Note != "from 3" {
		t.Errorf("0006-aaa-late.md = %+v", r)
	}

	// The store is whole again: every ID is in one file, and no temporary
	// file is left.
	if _, err := s.List(); err != nil {
		t.Errorf("List after Renumber: %v", err)
	}
	if r, err := s.Get(2); err != nil || r.Title != "Alpha" {
		t.Errorf("Get(2) = %+v, %v; want Alpha", r, err)
	}
	if r, err := s.Get(3); err != nil || r.Title != "Early" {
		t.Errorf("Get(3) = %+v, %v; want Early", r, err)
	}

	// Run again: nothing is left to do and nothing changes.
	settled := tree(t, root)
	again, err := s.Renumber(renumberDate, false)
	if err != nil || len(again.Moves) != 0 || len(again.Links) != 0 {
		t.Errorf("second Renumber = %+v, %v; want nothing to do", again, err)
	}
	if !reflect.DeepEqual(tree(t, root), settled) {
		t.Error("a second Renumber changed the store")
	}
}

// Without a collision Renumber reads nothing, so even a broken file elsewhere
// is no obstacle, and it changes nothing.
func TestRenumberWithoutACollisionChangesNothing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	seed(t, dir, "0002-b.md", question(2, "B"))
	writeFile(t, filepath.Join(dir, "0003-broken.md"), "not a record\n")
	s := mustOpen(t, root)
	before := tree(t, root)

	got, err := s.Renumber(renumberDate, false)
	if err != nil || len(got.Moves) != 0 || len(got.Links) != 0 {
		t.Errorf("Renumber = %+v, %v; want nothing to do and no error", got, err)
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("Renumber changed a store with no collision")
	}
	// A store directory that does not exist yet is an empty store.
	if got, err := mustOpen(t, t.TempDir()).Renumber(renumberDate, false); err != nil || len(got.Moves) != 0 {
		t.Errorf("Renumber of an empty store = %+v, %v", got, err)
	}
}

// A link to a colliding ID is ambiguous: it was written for one of the two
// records and nothing says which (C147). Renumber refuses, names each link
// and changes nothing, unless the caller accepts that the link stays on the
// record that keeps the ID.
func TestRenumberRefusesLinksToACollidingID(t *testing.T) {
	root := fixture(t, "collision-links")
	s := mustOpen(t, root)
	before := tree(t, root)

	_, err := s.Renumber(renumberDate, false)
	if !errors.Is(err, ErrAmbiguousLinks) {
		t.Fatalf("err = %v, want ErrAmbiguousLinks", err)
	}
	for _, want := range []string{"0002-replacement.md", "supersedes 1", "0003-gone.md", "superseded_by 1", "0001-store-a.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("a refused Renumber changed the store")
	}
}

func TestRenumberKeepLinksLeavesThemOnTheRecordThatKeepsTheID(t *testing.T) {
	root := fixture(t, "collision-links")
	s := mustOpen(t, root)
	before := tree(t, root)

	got, err := s.Renumber(renumberDate, true)
	if err != nil {
		t.Fatal(err)
	}
	wantMoves := []Move{{OldID: 1, NewID: 4, OldName: "0001-store-b.md", NewName: "0004-store-b.md", KeptName: "0001-store-a.md"}}
	if !reflect.DeepEqual(got.Moves, wantMoves) {
		t.Errorf("moves = %+v, want %+v", got.Moves, wantMoves)
	}
	wantLinks := []Link{
		{File: "0002-replacement.md", ID: 2, Field: "supersedes", To: 1, KeptName: "0001-store-a.md"},
		{File: "0003-gone.md", ID: 3, Field: "superseded_by", To: 1, KeptName: "0001-store-a.md"},
	}
	if !reflect.DeepEqual(got.Links, wantLinks) {
		t.Errorf("links = %+v, want %+v", got.Links, wantLinks)
	}
	after := tree(t, root)
	for _, f := range []string{"0001-store-a.md", "0002-replacement.md", "0003-gone.md"} {
		if after["decisions/"+f] != before["decisions/"+f] {
			t.Errorf("%s changed", f)
		}
	}
	if r := mustRead(t, filepath.Join(s.Dir(), "0004-store-b.md")); r.ID != 4 {
		t.Errorf("moved record = %+v", r)
	}
}

// Renumber reads every record file to find links, so it cannot go on past a
// file it cannot read: that file might be a link it would have to refuse.
func TestRenumberRefusesAnUnreadableFile(t *testing.T) {
	root := fixture(t, "collision")
	writeFile(t, filepath.Join(root, "decisions", "0009-broken.md"), "not a record\n")
	s := mustOpen(t, root)
	before := tree(t, root)

	_, err := s.Renumber(renumberDate, false)
	if err == nil || !strings.Contains(err.Error(), "0009-broken.md") {
		t.Fatalf("err = %v, want one that names 0009-broken.md", err)
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("a refused Renumber changed the store")
	}
}

// Every move is checked in memory before the first file is written, so a
// record that cannot be written under a new ID stops the whole run.
func TestRenumberChecksEveryMoveBeforeWriting(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	late := question(1, "B")
	late.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", late)
	seed(t, dir, "0002-c.md", question(2, "C"))
	// Decided with no rationale: it parses, but it cannot be written.
	writeFile(t, filepath.Join(dir, "0002-d.md"),
		"---\n{\"id\":2,\"title\":\"D\",\"status\":\"decided\",\"created\":\"2026-10-03\",\"decision\":\"x\"}\n---\n")
	s := mustOpen(t, root)
	before := tree(t, root)

	_, err := s.Renumber(renumberDate, false)
	if !errors.Is(err, model.ErrInvalid) || !strings.Contains(err.Error(), "0002-d.md") {
		t.Fatalf("err = %v, want model.ErrInvalid naming 0002-d.md", err)
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("the first collision was renumbered although the second could not be")
	}
}

// The new file is complete before the old one goes (C148): a crash between the
// two leaves both files, never none.
func TestRenumberWritesTheNewFileBeforeRemovingTheOldOne(t *testing.T) {
	root := fixture(t, "collision-links")
	s := mustOpen(t, root)
	calls := 0
	s.beforeRemove = func(oldPath, newPath string) error {
		calls++
		if r, err := model.ReadFile(newPath); err != nil || r.ID != 4 {
			t.Errorf("new file when the old one is about to go: %+v, %v", r, err)
		}
		if r, err := model.ReadFile(oldPath); err != nil || r.ID != 1 || r.Title != "Store B" {
			t.Errorf("old file when it is about to go: %+v, %v", r, err)
		}
		return nil
	}
	if _, err := s.Renumber(renumberDate, true); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("beforeRemove called %d times, want 1", calls)
	}
}

// If the old file cannot be removed, the new one is taken away again, so the
// store is as it was.
func TestRenumberPutsBackWhenTheOldFileCannotGo(t *testing.T) {
	root := fixture(t, "collision")
	s := mustOpen(t, root)
	before := tree(t, root)
	s.beforeRemove = func(oldPath, newPath string) error { return errors.New("disk on fire") }

	got, err := s.Renumber(renumberDate, false)
	if len(got.Moves) != 0 {
		t.Errorf("moves = %+v, want none", got.Moves)
	}
	if err == nil || !strings.Contains(err.Error(), "0002-beta.md") || !strings.Contains(err.Error(), "disk on fire") || !strings.Contains(err.Error(), "left as it was") {
		t.Fatalf("err = %v, want one that names 0002-beta.md and the cause and says the record is left as it was", err)
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("the store changed although the move failed")
	}
}

// If the new file cannot be taken away either, the error says that both files
// are there and which to delete.
func TestRenumberSaysWhatIsLeftWhenPuttingBackFails(t *testing.T) {
	root := fixture(t, "collision")
	s := mustOpen(t, root)
	s.beforeRemove = func(oldPath, newPath string) error {
		// Make the new path something os.Remove cannot delete.
		if err := os.Remove(newPath); err != nil {
			t.Error(err)
		}
		if err := os.MkdirAll(filepath.Join(newPath, "sub"), 0o755); err != nil {
			t.Error(err)
		}
		return errors.New("disk on fire")
	}
	_, err := s.Renumber(renumberDate, false)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"0002-beta.md", "0004-beta.md", "disk on fire", "delete"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(s.Dir(), "0002-beta.md")); err != nil {
		t.Errorf("the old file is gone: %v", err)
	}
}

// A non-regular entry or another writer's file at the next ID's name is
// skipped: the record takes the next ID after it.
func TestRenumberSkipsANameThatIsTaken(t *testing.T) {
	root := fixture(t, "collision-links")
	dir := filepath.Join(root, "decisions")
	// The scan ignores a directory, so ID 4 looks free; its name is not.
	if err := os.Mkdir(filepath.Join(dir, "0004-store-b.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, root)

	got, err := s.Renumber(renumberDate, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Moves) != 1 || got.Moves[0].NewID != 5 || got.Moves[0].NewName != "0005-store-b.md" {
		t.Errorf("moves = %+v, want one move to 0005-store-b.md", got.Moves)
	}
	if fi, err := os.Stat(filepath.Join(dir, "0004-store-b.md")); err != nil || !fi.IsDir() {
		t.Errorf("the directory at 0004-store-b.md was touched: %v, %v", fi, err)
	}
	if r := mustRead(t, filepath.Join(dir, "0005-store-b.md")); r.ID != 5 || r.History[len(r.History)-1].Note != "from 1" {
		t.Errorf("moved record = %+v", r)
	}
}

// Renumber leaves no temporary file behind, on success or on failure.
func TestRenumberLeavesNoTemporaryFiles(t *testing.T) {
	root := fixture(t, "collision")
	s := mustOpen(t, root)
	if _, err := s.Renumber(renumberDate, false); err != nil {
		t.Fatal(err)
	}
	// names leaves out the lock file, the one dot-file that stays (C114).
	for _, name := range names(t, s.Dir()) {
		if strings.HasPrefix(name, ".") {
			t.Errorf("left behind: %s", name)
		}
	}
}

// A move that fails after others succeeded leaves those done, says so in the
// result, and undoes only itself.
func TestRenumberReportsTheMovesDoneBeforeAFailure(t *testing.T) {
	root := fixture(t, "collision")
	s := mustOpen(t, root)
	calls := 0
	s.beforeRemove = func(oldPath, newPath string) error {
		calls++
		if calls == 2 {
			return errors.New("disk on fire")
		}
		return nil
	}
	got, err := s.Renumber(renumberDate, false)
	if err == nil || !strings.Contains(err.Error(), "0003-mmm-mid.md") {
		t.Fatalf("err = %v, want one that names 0003-mmm-mid.md", err)
	}
	if len(got.Moves) != 1 || got.Moves[0].NewName != "0004-beta.md" {
		t.Errorf("moves = %+v, want only the move to 0004-beta.md", got.Moves)
	}
	want := []string{"0001-intro.md", "0002-alpha.md", "0003-aaa-late.md", "0003-mmm-mid.md", "0003-zzz-early.md", "0004-beta.md"}
	if !reflect.DeepEqual(names(t, s.Dir()), want) {
		t.Errorf("files = %v, want %v", names(t, s.Dir()), want)
	}
}

// The ID in the file name is the record's identity (C59): a record whose id
// field says something else is renumbered from the file's ID, and its new file
// is whole again.
func TestRenumberUsesTheIDInTheFileName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	late := question(1, "B")
	late.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", late)
	// The loser's id field is 9, not 1.
	data, err := os.ReadFile(filepath.Join(dir, "0001-b.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "0001-b.md"), strings.Replace(string(data), `"id": 1`, `"id": 9`, 1))
	s := mustOpen(t, root)

	if _, err := s.Renumber(renumberDate, false); err != nil {
		t.Fatal(err)
	}
	r := mustRead(t, filepath.Join(dir, "0002-b.md"))
	if r.ID != 2 || len(r.History) != 1 || r.History[0].Note != "from 1" {
		t.Errorf("0002-b.md = %+v, want id 2 and the note \"from 1\"", r)
	}
}

// After ten taken names Renumber gives up on that record with an error that
// names the last path, and the moves done before it are in the result.
func TestRenumberGivesUpWhenEveryNameIsTaken(t *testing.T) {
	root := fixture(t, "collision")
	dir := filepath.Join(root, "decisions")
	for id := 5; id < 5+renumberAttempts; id++ {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprintf("%04d-mmm-mid.md", id)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s := mustOpen(t, root)

	got, err := s.Renumber(renumberDate, false)
	last := filepath.Join(dir, "0014-mmm-mid.md")
	if !errors.Is(err, os.ErrExist) || !strings.Contains(err.Error(), last) || !strings.Contains(err.Error(), "0003-mmm-mid.md") {
		t.Fatalf("err = %v, want os.ErrExist naming 0003-mmm-mid.md and %s", err, last)
	}
	if len(got.Moves) != 1 || got.Moves[0].NewName != "0004-beta.md" {
		t.Errorf("moves = %+v, want only the move to 0004-beta.md", got.Moves)
	}
	if _, err := os.Stat(filepath.Join(dir, "0003-mmm-mid.md")); err != nil {
		t.Errorf("the record that could not move is gone: %v", err)
	}
}

// A link counts wherever it is, a file that shares an ID included: 0002-x.md
// is itself a collision, and its link to ID 1 is just as ambiguous.
func TestRenumberFindsLinksInCollidingFilesToo(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	seed(t, dir, "0001-b.md", question(1, "B"))
	x := question(2, "X")
	x.Supersedes = []int{1}
	seed(t, dir, "0002-x.md", x)
	y := question(2, "Y")
	y.Created = "2026-10-02"
	seed(t, dir, "0002-y.md", y)
	s := mustOpen(t, root)
	before := tree(t, root)

	_, err := s.Renumber(renumberDate, false)
	if !errors.Is(err, ErrAmbiguousLinks) || !strings.Contains(err.Error(), "0002-x.md supersedes 1") {
		t.Fatalf("err = %v, want ErrAmbiguousLinks naming 0002-x.md supersedes 1", err)
	}
	if !reflect.DeepEqual(tree(t, root), before) {
		t.Error("a refused Renumber changed the store")
	}
}

// crash runs f and reports whether it panicked with crashed, the way a process
// that dies between two steps does.
type crashed struct{}

func crashDuring(f func()) (died bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(crashed); !ok {
				panic(r)
			}
			died = true
		}
	}()
	f()
	return false
}

// A crash after the new file is linked and before the old one is removed
// (C148) leaves the record twice. Running Renumber again must not move the
// stale copy to yet another ID: it refuses, names both files and changes
// nothing; once the stale copy is deleted, it carries on with what is left.
func TestRenumberRefusesAfterAnInterruptedMove(t *testing.T) {
	root := fixture(t, "collision")
	s := mustOpen(t, root)
	s.beforeRemove = func(oldPath, newPath string) error { panic(crashed{}) }
	if !crashDuring(func() { s.Renumber(renumberDate, false) }) {
		t.Fatal("the simulated crash did not happen")
	}
	s.beforeRemove = nil
	for _, name := range []string{"0002-beta.md", "0004-beta.md"} {
		if _, err := os.Stat(filepath.Join(s.Dir(), name)); err != nil {
			t.Fatalf("after the crash: %v", err)
		}
	}
	crashedTree := tree(t, root)

	got, err := s.Renumber(renumberDate, false)
	if !errors.Is(err, ErrStaleCopy) {
		t.Fatalf("err = %v, want ErrStaleCopy", err)
	}
	for _, want := range []string{"0002-beta.md", "0004-beta.md", "delete"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q:\n%v", want, err)
		}
	}
	if len(got.Moves) != 0 || !reflect.DeepEqual(tree(t, root), crashedTree) {
		t.Errorf("a refused Renumber moved %+v or changed the store", got.Moves)
	}

	// The owner deletes the stale copy; the rest of the collisions are fixed.
	if err := os.Remove(filepath.Join(s.Dir(), "0002-beta.md")); err != nil {
		t.Fatal(err)
	}
	got, err = s.Renumber(renumberDate, false)
	if err != nil || len(got.Moves) != 2 || got.Moves[0].NewID != 5 || got.Moves[1].NewID != 6 {
		t.Errorf("Renumber after the delete = %+v, %v; want the two ID 3 moves to 5 and 6", got.Moves, err)
	}
}

// The copy's last history entry need not be the renumber one: the record is
// edited after the crash (here it is decided) and the old file is still a
// stale copy. Renumber refuses, and IsStaleCopy says so for any renumber
// entry "from N" in the history, not only for the last entry.
func TestRenumberRefusesAStaleCopyWhoseRecordWasEditedSince(t *testing.T) {
	root := fixture(t, "collision-interrupted")
	path := filepath.Join(root, "decisions", "0004-beta.md")
	writeFile(t, path, `---
{"id":4,"title":"Beta","status":"decided","created":"2026-10-02","decided_at":"2026-11-03","decision":"b","rationale":"because","history":[{"date":"2026-10-02","event":"ask"},{"date":"2026-11-02","event":"renumber","note":"from 2"},{"date":"2026-11-03","event":"decide"}]}
---
Beta body.
`)
	before := tree(t, root)
	got, err := mustOpen(t, root).Renumber(renumberDate, false)
	if !errors.Is(err, ErrStaleCopy) {
		t.Fatalf("Renumber = %+v, %v; want ErrStaleCopy", got.Moves, err)
	}
	if len(got.Moves) != 0 || !reflect.DeepEqual(tree(t, root), before) {
		t.Errorf("a refused Renumber moved %+v or changed the store", got.Moves)
	}

	rec := model.Record{History: []model.HistoryEntry{
		{Event: model.EventAsk},
		{Event: model.EventRenumber, Note: "from 2"},
		{Event: model.EventDecide},
	}}
	stale, moved := File{ID: 2, Name: "0002-beta.md"}, File{ID: 4, Name: "0004-beta.md"}
	if !IsStaleCopy(stale, moved, rec) {
		t.Error("IsStaleCopy = false for a record renumbered from 2 and edited since")
	}
	// A renumber entry for another ID anywhere in the history is no match.
	rec.History[1].Note = "from 9"
	if IsStaleCopy(stale, moved, rec) {
		t.Error("IsStaleCopy = true for a record renumbered from 9")
	}
}

// A file that looks like a copy only by name text is not one: the copy must
// have been renumbered from this file's ID.
func TestRenumberOnlyRefusesACopyRenumberedFromThatID(t *testing.T) {
	root := fixture(t, "collision-interrupted")
	// The renumbered record says it came from ID 9, not from 2.
	path := filepath.Join(root, "decisions", "0004-beta.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(data), `"from 2"`, `"from 9"`, 1))
	if got, err := mustOpen(t, root).Renumber(renumberDate, false); err != nil || len(got.Moves) != 1 {
		t.Errorf("from another ID: Renumber = %+v, %v; want the one move of 0002-beta.md", got.Moves, err)
	}

	// Nor does a renumbered record with other name text count.
	root = fixture(t, "collision-interrupted")
	if err := os.Rename(filepath.Join(root, "decisions", "0004-beta.md"), filepath.Join(root, "decisions", "0004-gamma.md")); err != nil {
		t.Fatal(err)
	}
	if got, err := mustOpen(t, root).Renumber(renumberDate, false); err != nil || len(got.Moves) != 1 {
		t.Errorf("other name text: Renumber = %+v, %v; want the one move of 0002-beta.md", got.Moves, err)
	}

	// Nor does a record whose last entry is another event.
	root = fixture(t, "collision-interrupted")
	path = filepath.Join(root, "decisions", "0004-beta.md")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(data), `"event":"renumber"`, `"event":"decide"`, 1))
	if got, err := mustOpen(t, root).Renumber(renumberDate, false); err != nil || len(got.Moves) != 1 {
		t.Errorf("other event: Renumber = %+v, %v; want the one move of 0002-beta.md", got.Moves, err)
	}

	// A file is never the stale copy of itself.
	root = fixture(t, "collision-interrupted")
	if err := os.Remove(filepath.Join(root, "decisions", "0004-beta.md")); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(root, "decisions", "0002-beta.md")
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(data), `"event":"ask"`, `"event":"renumber","note":"from 2"`, 1))
	if got, err := mustOpen(t, root).Renumber(renumberDate, false); err != nil || len(got.Moves) != 1 {
		t.Errorf("itself: Renumber = %+v, %v; want the one move of 0002-beta.md", got.Moves, err)
	}
}

// Renumber checks the moves against the IDs they will really get: one past
// the highest, and never an ID that an existing link names without a file
// (a dangling link would silently resolve to the moved record). Here record
// 4 is named by 0002-d.md's own link, so its move skips 4 and takes 5.
func TestRenumberSkipsIDsNamedByDanglingLinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	b := question(1, "B")
	b.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", b)
	seed(t, dir, "0002-c.md", question(2, "C"))
	d := question(2, "D")
	d.Created = "2026-10-03"
	d.Supersedes = []int{4}
	seed(t, dir, "0002-d.md", d)
	s := mustOpen(t, root)

	got, err := s.Renumber(renumberDate, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Moves) != 2 || got.Moves[0].NewName != "0003-b.md" || got.Moves[1].NewName != "0005-d.md" {
		t.Errorf("moves = %+v, want 0001-b.md to 0003-b.md and 0002-d.md to 0005-d.md", got.Moves)
	}
	if r := mustRead(t, filepath.Join(dir, "0005-d.md")); r.ID != 5 || !reflect.DeepEqual(r.Supersedes, []int{4}) {
		t.Errorf("0005-d.md = %+v, want id 5 and its link to 4 untouched", r)
	}
}

// The same holds for superseded_by, and for the ID a name would take when the
// link is the only thing that names it.
func TestRenumberSkipsAnIDNamedBySupersededBy(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "decisions")
	seed(t, dir, "0001-a.md", question(1, "A"))
	b := question(1, "B")
	b.Created = "2026-10-02"
	seed(t, dir, "0001-b.md", b)
	y := question(2, "Y")
	y.SupersededBy = 3
	seed(t, dir, "0002-y.md", y)
	s := mustOpen(t, root)

	got, err := s.Renumber(renumberDate, false)
	if err != nil || len(got.Moves) != 1 || got.Moves[0].NewID != 4 {
		t.Errorf("Renumber = %+v, %v; want the move to ID 4 (3 is named by a link)", got.Moves, err)
	}
}

// The keeper is chosen by the created date, so a colliding record whose date
// is missing or not a date stops the run, naming the file.
func TestRenumberRefusesACollidingRecordWithoutAValidCreatedDate(t *testing.T) {
	for _, created := range []string{"", "yesterday", "2026-13-01"} {
		t.Run(created, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "decisions")
			seed(t, dir, "0001-a.md", question(1, "A"))
			writeFile(t, filepath.Join(dir, "0001-b.md"),
				"---\n{\"id\":1,\"title\":\"B\",\"status\":\"open\",\"created\":\""+created+"\"}\n---\n")
			s := mustOpen(t, root)
			before := tree(t, root)

			_, err := s.Renumber(renumberDate, false)
			if err == nil || !strings.Contains(err.Error(), "0001-b.md") || !strings.Contains(err.Error(), "created") {
				t.Fatalf("err = %v, want one that names 0001-b.md and created", err)
			}
			if !reflect.DeepEqual(tree(t, root), before) {
				t.Error("a refused Renumber changed the store")
			}
		})
	}
}
