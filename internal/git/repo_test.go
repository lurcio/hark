package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var testAuthor = &object.Signature{
	Name:  "Test",
	Email: "test@test.com",
	When:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
}

// initTestRepo creates a temporary git repo with one committed file (README.md).
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	if _, err := wt.Add("README.md"); err != nil {
		t.Fatalf("add: %v", err)
	}

	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: testAuthor,
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}

	return dir
}

func TestOpen(t *testing.T) {
	dir := initTestRepo(t)

	r, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if r.Path() != dir {
		t.Errorf("Path() = %q, want %q", r.Path(), dir)
	}
}

func TestOpen_Subdirectory(t *testing.T) {
	dir := initTestRepo(t)
	sub := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	r, err := Open(sub)
	if err != nil {
		t.Fatalf("Open from subdirectory: %v", err)
	}
	if r.Path() != dir {
		t.Errorf("Path() = %q, want repo root %q", r.Path(), dir)
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open(t.TempDir()) // empty dir, not a repo
	if err == nil {
		t.Error("expected error for non-repo directory")
	}
}

func TestChangedFiles_NoChanges(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	files, err := r.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 changed files, got %d", len(files))
	}
}

func TestChangedFiles_NewFile(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\nworld\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := r.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}

	found := findFile(files, "hello.txt")
	if found == nil {
		t.Fatal("hello.txt not found in changed files")
	}
	if found.Status != StatusAdded {
		t.Errorf("status = %v, want added", found.Status)
	}
	if !strings.Contains(found.Diff, "+hello") {
		t.Errorf("diff should contain +hello, got:\n%s", found.Diff)
	}
	if !strings.Contains(found.Diff, "+world") {
		t.Errorf("diff should contain +world, got:\n%s", found.Diff)
	}
	if found.Binary {
		t.Error("should not be detected as binary")
	}
}

func TestChangedFiles_ModifiedFile(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := r.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}

	found := findFile(files, "README.md")
	if found == nil {
		t.Fatal("README.md not found in changed files")
	}
	if found.Status != StatusModified {
		t.Errorf("status = %v, want modified", found.Status)
	}
	if !strings.Contains(found.Diff, "-# Test") {
		t.Errorf("diff should contain removed old line, got:\n%s", found.Diff)
	}
	if !strings.Contains(found.Diff, "+# Modified") {
		t.Errorf("diff should contain added new line, got:\n%s", found.Diff)
	}
}

func TestChangedFiles_DeletedFile(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(filepath.Join(dir, "README.md")); err != nil {
		t.Fatal(err)
	}

	files, err := r.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}

	found := findFile(files, "README.md")
	if found == nil {
		t.Fatal("README.md not found in changed files")
	}
	if found.Status != StatusDeleted {
		t.Errorf("status = %v, want deleted", found.Status)
	}
	if !strings.Contains(found.Diff, "-# Test") {
		t.Errorf("diff should contain removed line, got:\n%s", found.Diff)
	}
}

func TestChangedFiles_BinaryFile(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Write a file with null bytes (binary content).
	binaryContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00}
	if err := os.WriteFile(filepath.Join(dir, "image.png"), binaryContent, 0644); err != nil {
		t.Fatal(err)
	}

	files, err := r.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}

	found := findFile(files, "image.png")
	if found == nil {
		t.Fatal("image.png not found in changed files")
	}
	if !found.Binary {
		t.Error("expected binary detection")
	}
	if found.Diff != "Binary file changed" {
		t.Errorf("diff = %q, want %q", found.Diff, "Binary file changed")
	}
}

func TestChangedFiles_MultipleFiles(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("aaa\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("bbb\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := r.ChangedFiles()
	if err != nil {
		t.Fatal(err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 changed files, got %d", len(files))
	}

	// Files should be sorted by path.
	if files[0].Path != "README.md" || files[1].Path != "a.txt" || files[2].Path != "b.txt" {
		t.Errorf("files not sorted: %v, %v, %v", files[0].Path, files[1].Path, files[2].Path)
	}
}

func TestHeadChanged(t *testing.T) {
	dir := initTestRepo(t)
	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// No change since Open.
	changed, _, err := r.HeadChanged()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("HEAD should not have changed yet")
	}

	// Make a new commit.
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("new.txt"); err != nil {
		t.Fatal(err)
	}
	_, err = wt.Commit("second commit", &gogit.CommitOptions{
		Author: testAuthor,
	})
	if err != nil {
		t.Fatal(err)
	}

	// HEAD should have changed.
	changed, msg, err := r.HeadChanged()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("HEAD should have changed after commit")
	}
	if msg != "second commit" {
		t.Errorf("commit message = %q, want %q", msg, "second commit")
	}

	// Calling again should report no change.
	changed, _, err = r.HeadChanged()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("HEAD should not have changed on second call")
	}
}

func TestUnifiedDiff_Format(t *testing.T) {
	diff := computeUnifiedDiff("test.go",
		"line1\nline2\nline3\n",
		"line1\nmodified\nline3\n",
	)

	if !strings.HasPrefix(diff, "--- a/test.go\n+++ b/test.go\n") {
		t.Errorf("diff should start with file headers, got:\n%s", diff)
	}
	if !strings.Contains(diff, "@@") {
		t.Error("diff should contain hunk header")
	}
	if !strings.Contains(diff, "-line2") {
		t.Errorf("diff should show removed line, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+modified") {
		t.Errorf("diff should show added line, got:\n%s", diff)
	}
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name   string
		data   []byte
		binary bool
	}{
		{"empty", nil, false},
		{"text", []byte("hello world"), false},
		{"null byte", []byte("hello\x00world"), true},
		{"png header", []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBinary(tt.data); got != tt.binary {
				t.Errorf("isBinary(%q) = %v, want %v", tt.data, got, tt.binary)
			}
		})
	}
}

func findFile(files []ChangedFile, path string) *ChangedFile {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}
