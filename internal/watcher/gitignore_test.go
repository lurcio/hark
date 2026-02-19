package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupGitignoreRepo creates a temp directory initialized as a git repo with
// the specified .gitignore content and directory/file structure. Returns the
// repo root path.
func setupGitignoreRepo(t *testing.T, rootIgnore string, structure map[string]string) string {
	t.Helper()
	dir := initTestRepo(t)

	if rootIgnore != "" {
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(rootIgnore), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for path, content := range structure {
		abs := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

// newTestFSWatcher creates an fsWatcher for testing with a short debounce.
func newTestFSWatcher(t *testing.T, root string) *fsWatcher {
	t.Helper()
	fw, err := newFSWatcher(root, 50*time.Millisecond, nil, nil)
	if err != nil {
		t.Fatalf("newFSWatcher: %v", err)
	}
	t.Cleanup(func() { fw.Close() })
	return fw
}

func TestGitignoreRootPatterns(t *testing.T) {
	dir := setupGitignoreRepo(t, "node_modules/\n*.log\nbuild/\n", map[string]string{
		"node_modules/pkg/index.js": "module.exports = {}",
		"build/output.js":           "compiled",
		"app.log":                   "log entry",
		"src/main.go":               "package main",
	})

	fw := newTestFSWatcher(t, dir)

	tests := []struct {
		rel     string
		ignored bool
	}{
		{"node_modules", true},
		{"node_modules/pkg/index.js", true},
		{"build", true},
		{"build/output.js", true},
		{"app.log", true},
		{"src/main.go", false},
		{"src", false},
	}

	for _, tc := range tests {
		t.Run(tc.rel, func(t *testing.T) {
			got := fw.isIgnored(tc.rel)
			if got != tc.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", tc.rel, got, tc.ignored)
			}
		})
	}
}

func TestGitignoreNestedPatterns(t *testing.T) {
	dir := setupGitignoreRepo(t, "", map[string]string{
		"src/.gitignore":        "*.generated.go\n",
		"src/handler.go":        "package src",
		"src/handler.generated.go": "// generated",
		"docs/.gitignore":       "draft/\n",
		"docs/readme.md":        "# docs",
		"docs/draft/wip.md":     "work in progress",
	})

	fw := newTestFSWatcher(t, dir)

	tests := []struct {
		rel     string
		ignored bool
	}{
		{"src/handler.go", false},
		{"src/handler.generated.go", true},
		{"docs/readme.md", false},
		{"docs/draft", true},
		{"docs/draft/wip.md", true},
	}

	for _, tc := range tests {
		t.Run(tc.rel, func(t *testing.T) {
			got := fw.isIgnored(tc.rel)
			if got != tc.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", tc.rel, got, tc.ignored)
			}
		})
	}
}

func TestGitignoreNegationPatterns(t *testing.T) {
	dir := setupGitignoreRepo(t, "*.log\n!important.log\n", map[string]string{
		"debug.log":     "debug info",
		"important.log": "important info",
		"app.log":       "app info",
	})

	fw := newTestFSWatcher(t, dir)

	tests := []struct {
		rel     string
		ignored bool
	}{
		{"debug.log", true},
		{"app.log", true},
		{"important.log", false},
	}

	for _, tc := range tests {
		t.Run(tc.rel, func(t *testing.T) {
			got := fw.isIgnored(tc.rel)
			if got != tc.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", tc.rel, got, tc.ignored)
			}
		})
	}
}

func TestGitignoreDirectoryPatterns(t *testing.T) {
	dir := setupGitignoreRepo(t, "build/\ntmp/\n", map[string]string{
		"build/output.js":   "compiled",
		"tmp/cache.dat":     "cached",
		"src/build_utils.go": "package src", // "build" in filename, NOT a directory match
	})

	fw := newTestFSWatcher(t, dir)

	tests := []struct {
		rel     string
		ignored bool
	}{
		{"build", true},
		{"build/output.js", true},
		{"tmp", true},
		{"tmp/cache.dat", true},
		{"src/build_utils.go", false},
	}

	for _, tc := range tests {
		t.Run(tc.rel, func(t *testing.T) {
			got := fw.isIgnored(tc.rel)
			if got != tc.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", tc.rel, got, tc.ignored)
			}
		})
	}
}

func TestGitignoreWildcardPatterns(t *testing.T) {
	dir := setupGitignoreRepo(t, "*.log\n**/*.tmp\n*.o\n", map[string]string{
		"app.log":            "log",
		"src/debug.log":      "log",
		"cache.tmp":          "temp",
		"deep/nested/file.tmp": "temp",
		"main.o":             "object",
		"src/main.go":        "package main",
	})

	fw := newTestFSWatcher(t, dir)

	tests := []struct {
		rel     string
		ignored bool
	}{
		{"app.log", true},
		{"src/debug.log", true},
		{"cache.tmp", true},
		{"deep/nested/file.tmp", true},
		{"main.o", true},
		{"src/main.go", false},
	}

	for _, tc := range tests {
		t.Run(tc.rel, func(t *testing.T) {
			got := fw.isIgnored(tc.rel)
			if got != tc.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", tc.rel, got, tc.ignored)
			}
		})
	}
}

func TestGitignoreAlwaysIgnoresGitDir(t *testing.T) {
	dir := setupGitignoreRepo(t, "", nil)
	fw := newTestFSWatcher(t, dir)

	tests := []struct {
		rel     string
		ignored bool
	}{
		{".git", true},
		{".git/config", true},
		{".git/objects/pack", true},
		{".github", false},
		{".gitignore", false},
	}

	for _, tc := range tests {
		t.Run(tc.rel, func(t *testing.T) {
			got := fw.isIgnored(tc.rel)
			if got != tc.ignored {
				t.Errorf("isIgnored(%q) = %v, want %v", tc.rel, got, tc.ignored)
			}
		})
	}
}

func TestGitignoreExtraIgnorePatternsStillWork(t *testing.T) {
	dir := setupGitignoreRepo(t, "", map[string]string{
		"vendor/lib.go": "package vendor",
		"src/main.go":   "package main",
	})

	// Create watcher with extra ignore patterns.
	fw, err := newFSWatcher(dir, 50*time.Millisecond, []string{"vendor"}, nil)
	if err != nil {
		t.Fatalf("newFSWatcher: %v", err)
	}
	defer fw.Close()

	// The "vendor" pattern matches the directory name directly. In practice,
	// addRecursive skips the entire directory so no events for files inside
	// vendor/ would ever be generated.
	if !fw.isIgnored("vendor") {
		t.Error("expected vendor to be ignored via extraIgnore")
	}
	if fw.isIgnored("src/main.go") {
		t.Error("expected src/main.go to NOT be ignored")
	}
}

func TestGitignoreCommentAndBlankLines(t *testing.T) {
	// Comments and blank lines should not affect matching.
	dir := setupGitignoreRepo(t, "# This is a comment\n\n*.log\n\n# Another comment\ntmp/\n", map[string]string{
		"app.log":     "log",
		"src/main.go": "package main",
		"tmp/data":    "temp data",
	})

	fw := newTestFSWatcher(t, dir)

	if !fw.isIgnored("app.log") {
		t.Error("expected app.log to be ignored")
	}
	if !fw.isIgnored("tmp") {
		t.Error("expected tmp to be ignored")
	}
	if fw.isIgnored("src/main.go") {
		t.Error("expected src/main.go to NOT be ignored")
	}
}

func TestGitignoreNoGitignoreFile(t *testing.T) {
	// When there's no .gitignore, only .git should be ignored.
	dir := setupGitignoreRepo(t, "", map[string]string{
		"src/main.go": "package main",
		"readme.txt":  "hello",
	})

	fw := newTestFSWatcher(t, dir)

	if fw.isIgnored("src/main.go") {
		t.Error("expected src/main.go to NOT be ignored")
	}
	if fw.isIgnored("readme.txt") {
		t.Error("expected readme.txt to NOT be ignored")
	}
	if !fw.isIgnored(".git") {
		t.Error("expected .git to be ignored")
	}
}
