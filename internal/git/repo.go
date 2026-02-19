package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// FileStatus represents the type of change to a file.
type FileStatus int

const (
	StatusAdded    FileStatus = iota
	StatusModified
	StatusDeleted
)

func (s FileStatus) String() string {
	switch s {
	case StatusAdded:
		return "added"
	case StatusModified:
		return "modified"
	case StatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// ChangedFile represents a file changed in the working tree vs HEAD.
type ChangedFile struct {
	Path   string
	Status FileStatus
	Diff   string // unified diff format
	Binary bool   // true if binary file detected
}

// Repo provides read-only access to a git repository.
type Repo struct {
	repo     *gogit.Repository
	path     string
	mu       sync.Mutex
	lastHEAD plumbing.Hash
}

// Open opens a git repository at the given path.
func Open(path string) (*Repo, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("git: resolving path: %w", err)
	}

	repo, err := gogit.PlainOpenWithOptions(absPath, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("git: opening repository: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("git: getting worktree: %w", err)
	}

	r := &Repo{
		repo: repo,
		path: wt.Filesystem.Root(),
	}

	// Initialize lastHEAD — may fail for empty repos with no commits.
	if head, err := repo.Head(); err == nil {
		r.lastHEAD = head.Hash()
	}

	return r, nil
}

// Path returns the repository root path.
func (r *Repo) Path() string {
	return r.path
}

// HeadChanged checks if HEAD has changed since the last call.
// Returns whether it changed and the new commit message.
func (r *Repo) HeadChanged() (bool, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	head, err := r.repo.Head()
	if err != nil {
		return false, "", fmt.Errorf("git: getting HEAD: %w", err)
	}

	current := head.Hash()
	if current == r.lastHEAD {
		return false, "", nil
	}

	r.lastHEAD = current

	commit, err := r.repo.CommitObject(current)
	if err != nil {
		return true, "", fmt.Errorf("git: getting commit: %w", err)
	}

	return true, strings.TrimSpace(commit.Message), nil
}

// ChangedFiles returns files changed in the working tree compared to HEAD.
// It is safe for concurrent use. If go-git's status fails (e.g. due to
// permission errors on directories), it falls back to system git.
func (r *Repo) ChangedFiles() ([]ChangedFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	changedPaths, err := r.statusGoGit()
	if err != nil {
		// Fallback: use system git which handles permission errors gracefully.
		changedPaths, err = r.statusSystemGit()
		if err != nil {
			return nil, err
		}
	}

	headTree, _ := r.headTree()

	var files []ChangedFile
	for filePath := range changedPaths {
		inHEAD := r.fileExistsInTree(headTree, filePath)
		onDisk := r.fileExistsOnDisk(filePath)

		if !inHEAD && !onDisk {
			continue
		}

		var change FileStatus
		switch {
		case !inHEAD:
			change = StatusAdded
		case !onDisk:
			change = StatusDeleted
		default:
			change = StatusModified
		}

		cf := ChangedFile{
			Path:   filePath,
			Status: change,
		}

		var oldBytes, newBytes []byte
		if inHEAD {
			oldBytes, _ = r.readFromTree(headTree, filePath)
		}
		if onDisk {
			newBytes, _ = r.readFromWorkdir(filePath)
		}

		if isBinary(oldBytes) || isBinary(newBytes) {
			cf.Binary = true
			cf.Diff = "Binary file changed"
			files = append(files, cf)
			continue
		}

		cf.Diff = computeUnifiedDiff(filePath, string(oldBytes), string(newBytes))
		files = append(files, cf)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return files, nil
}

// statusGoGit uses go-git to get the set of changed file paths.
func (r *Repo) statusGoGit() (map[string]struct{}, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return nil, err
	}
	status, err := wt.Status()
	if err != nil {
		return nil, err
	}
	paths := make(map[string]struct{}, len(status))
	for p := range status {
		paths[p] = struct{}{}
	}
	return paths, nil
}

// statusSystemGit shells out to git status as a fallback when go-git fails
// (e.g. due to permission-denied directories). System git handles this
// gracefully because it respects .gitignore before accessing directories.
func (r *Repo) statusSystemGit() (map[string]struct{}, error) {
	cmd := exec.Command("git", "status", "--porcelain", "-z")
	cmd.Dir = r.path
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git: system git status: %w", err)
	}

	paths := make(map[string]struct{})
	// Porcelain -z format: "XY filename\0" or "XY old\0new\0" for renames.
	entries := bytes.Split(out, []byte{0})
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		// XY prefix is 2 chars + space, then filename.
		xy := string(entry[:2])
		filePath := string(entry[3:])

		// Skip renames' second entry (the old name).
		if xy[0] == 'R' || xy[1] == 'R' {
			i++ // skip next entry (old filename)
		}

		if filePath != "" {
			paths[filePath] = struct{}{}
		}
	}

	return paths, nil
}

func (r *Repo) headTree() (*object.Tree, error) {
	head, err := r.repo.Head()
	if err != nil {
		return nil, err
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}
	return commit.Tree()
}

func (r *Repo) fileExistsInTree(tree *object.Tree, path string) bool {
	if tree == nil {
		return false
	}
	_, err := tree.File(path)
	return err == nil
}

func (r *Repo) fileExistsOnDisk(path string) bool {
	_, err := os.Stat(filepath.Join(r.path, path))
	return err == nil
}

func (r *Repo) readFromTree(tree *object.Tree, path string) ([]byte, error) {
	if tree == nil {
		return nil, fmt.Errorf("no tree")
	}
	f, err := tree.File(path)
	if err != nil {
		return nil, err
	}
	contents, err := f.Contents()
	if err != nil {
		return nil, err
	}
	return []byte(contents), nil
}

func (r *Repo) readFromWorkdir(path string) ([]byte, error) {
	return os.ReadFile(filepath.Join(r.path, path))
}

// isBinary checks if data contains null bytes in the first 8000 bytes.
func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	n := len(data)
	if n > 8000 {
		n = 8000
	}
	return bytes.IndexByte(data[:n], 0) != -1
}

// computeUnifiedDiff generates a unified diff between old and new content.
func computeUnifiedDiff(path, oldContent, newContent string) string {
	if oldContent == newContent {
		return ""
	}

	dmp := diffmatchpatch.New()

	// Line-level diff using DiffLinesToChars trick.
	a, b, lineArray := dmp.DiffLinesToChars(oldContent, newContent)
	diffs := dmp.DiffMain(a, b, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	// Expand into individual line operations.
	type lineOp struct {
		op   diffmatchpatch.Operation
		text string
	}
	var ops []lineOp
	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for _, l := range lines {
			ops = append(ops, lineOp{op: d.Type, text: l})
		}
	}
	if len(ops) == 0 {
		return ""
	}

	// Find indices of changed lines.
	var changeIdx []int
	for i, op := range ops {
		if op.op != diffmatchpatch.DiffEqual {
			changeIdx = append(changeIdx, i)
		}
	}
	if len(changeIdx) == 0 {
		return ""
	}

	// Group into hunks with context lines.
	const ctx = 3
	type hunkBounds struct{ start, end int }

	start := max(0, changeIdx[0]-ctx)
	end := min(len(ops), changeIdx[0]+1+ctx)

	var hunks []hunkBounds
	for _, ci := range changeIdx[1:] {
		cs := max(0, ci-ctx)
		ce := min(len(ops), ci+1+ctx)
		if cs <= end {
			end = ce
		} else {
			hunks = append(hunks, hunkBounds{start, end})
			start = cs
			end = ce
		}
	}
	hunks = append(hunks, hunkBounds{start, end})

	// Format unified diff.
	var buf strings.Builder
	fmt.Fprintf(&buf, "--- a/%s\n+++ b/%s\n", path, path)

	for _, h := range hunks {
		oldLine, newLine := 1, 1
		for i := 0; i < h.start; i++ {
			if ops[i].op != diffmatchpatch.DiffInsert {
				oldLine++
			}
			if ops[i].op != diffmatchpatch.DiffDelete {
				newLine++
			}
		}

		oldCount, newCount := 0, 0
		for i := h.start; i < h.end; i++ {
			if ops[i].op != diffmatchpatch.DiffInsert {
				oldCount++
			}
			if ops[i].op != diffmatchpatch.DiffDelete {
				newCount++
			}
		}

		fmt.Fprintf(&buf, "@@ -%d,%d +%d,%d @@\n", oldLine, oldCount, newLine, newCount)
		for i := h.start; i < h.end; i++ {
			switch ops[i].op {
			case diffmatchpatch.DiffEqual:
				fmt.Fprintf(&buf, " %s\n", ops[i].text)
			case diffmatchpatch.DiffDelete:
				fmt.Fprintf(&buf, "-%s\n", ops[i].text)
			case diffmatchpatch.DiffInsert:
				fmt.Fprintf(&buf, "+%s\n", ops[i].text)
			}
		}
	}

	return buf.String()
}
