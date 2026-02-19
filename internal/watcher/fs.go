package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-git/go-billy/v5/osfs"
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// fsWatcher uses fsnotify for near-instant filesystem change detection.
type fsWatcher struct {
	watcher    *fsnotify.Watcher
	events     chan Event
	done       chan struct{}
	root       string
	ignores    []string
	gitMatcher gitignore.Matcher
	once       sync.Once
	logFn      LogFunc
}

// newFSWatcher creates a filesystem watcher that recursively watches root,
// debounces events within the given window, and filters paths matching
// extraIgnore glob patterns, .git/, and .gitignore rules.
func newFSWatcher(root string, debounce time.Duration, extraIgnore []string, logFn LogFunc) (*fsWatcher, error) {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("watcher: creating fsnotify watcher: %w", err)
	}

	matcher := loadGitignorePatterns(root, logFn)

	fw := &fsWatcher{
		watcher:    w,
		events:     make(chan Event, 100),
		done:       make(chan struct{}),
		root:       root,
		ignores:    extraIgnore,
		gitMatcher: matcher,
		logFn:      logFn,
	}

	if err := fw.addRecursive(root); err != nil {
		w.Close()
		return nil, err
	}

	watchedDirs := len(w.WatchList())
	logFn("fsnotify: watching %d directories under %s", watchedDirs, root)

	raw := make(chan Event, 100)
	go fw.readEvents(raw)
	go func() {
		runDebouncer(raw, debounce, fw.events, fw.done)
		close(fw.events)
	}()

	return fw, nil
}

// addRecursive walks from root and adds fsnotify watches for all directories,
// skipping ignored paths. Directories that can't be watched are silently skipped.
func (fw *fsWatcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(fw.root, path)
		if err != nil {
			return nil
		}
		if rel != "." && fw.isIgnored(rel) {
			return filepath.SkipDir
		}
		if err := fw.watcher.Add(path); err != nil {
			fw.logFn("fsnotify: skip dir %s: %v", rel, err)
			return nil
		}
		return nil
	})
}

// isIgnored returns true if the relative path should be excluded from watching.
func (fw *fsWatcher) isIgnored(rel string) bool {
	// Always ignore .git directory.
	if rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
		return true
	}

	// Check gitignore patterns.
	if fw.gitMatcher != nil {
		pathParts := strings.Split(filepath.ToSlash(rel), "/")
		// Check if path is a directory on disk to correctly match directory-only patterns.
		isDir := false
		if info, err := os.Stat(filepath.Join(fw.root, rel)); err == nil {
			isDir = info.IsDir()
		}
		if fw.gitMatcher.Match(pathParts, isDir) {
			return true
		}
	}

	// Check extra ignore patterns.
	base := filepath.Base(rel)
	for _, pattern := range fw.ignores {
		if matched, _ := filepath.Match(pattern, rel); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// readEvents reads raw fsnotify events, filters them, and sends watcher Events
// to the raw channel. Closes raw when done.
func (fw *fsWatcher) readEvents(raw chan<- Event) {
	defer close(raw)
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			rel, err := filepath.Rel(fw.root, event.Name)
			if err != nil {
				continue
			}

			if fw.isIgnored(rel) {
				continue
			}

			// Auto-watch new directories recursively.
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					fw.logFn("fsnotify: new dir, adding watch: %s", rel)
					_ = fw.addRecursive(event.Name)
					continue
				}
			}

			var kind EventKind
			switch {
			case event.Has(fsnotify.Create):
				kind = Created
			case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
				kind = Deleted
			case event.Has(fsnotify.Write):
				kind = Modified
			default:
				continue
			}

			fw.logFn("fsnotify: raw event: path=%q kind=%v (op=%v)", rel, kind, event.Op)
			select {
			case raw <- Event{Path: rel, Kind: kind}:
			case <-fw.done:
				return
			}

		case fsErr, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logFn("fsnotify: error: %v", fsErr)

		case <-fw.done:
			return
		}
	}
}

func (fw *fsWatcher) Events() <-chan Event {
	return fw.events
}

func (fw *fsWatcher) Close() error {
	var err error
	fw.once.Do(func() {
		close(fw.done)
		err = fw.watcher.Close()
	})
	return err
}

// loadGitignorePatterns reads gitignore rules from the repo, global config,
// and system config, returning a combined matcher.
func loadGitignorePatterns(root string, logFn LogFunc) gitignore.Matcher {
	var patterns []gitignore.Pattern

	// Load system-level patterns (e.g. /etc/gitconfig core.excludesFile).
	rootFS := osfs.New("/")
	if ps, err := gitignore.LoadSystemPatterns(rootFS); err == nil {
		patterns = append(patterns, ps...)
	} else {
		logFn("gitignore: loading system patterns: %v", err)
	}

	// Load global patterns (~/.config/git/ignore or core.excludesFile from ~/.gitconfig).
	if ps, err := gitignore.LoadGlobalPatterns(rootFS); err == nil {
		patterns = append(patterns, ps...)
	} else {
		logFn("gitignore: loading global patterns: %v", err)
	}

	// Load repo-level patterns (.git/info/exclude + all .gitignore files).
	repoFS := osfs.New(root)
	if ps, err := gitignore.ReadPatterns(repoFS, nil); err == nil {
		patterns = append(patterns, ps...)
	} else {
		logFn("gitignore: reading repo patterns: %v", err)
	}

	if len(patterns) == 0 {
		return nil
	}

	logFn("gitignore: loaded %d patterns for %s", len(patterns), root)
	return gitignore.NewMatcher(patterns)
}
