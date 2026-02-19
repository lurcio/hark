package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// fsWatcher uses fsnotify for near-instant filesystem change detection.
type fsWatcher struct {
	watcher *fsnotify.Watcher
	events  chan Event
	done    chan struct{}
	root    string
	ignores []string
	once    sync.Once
	logFn   LogFunc
}

// newFSWatcher creates a filesystem watcher that recursively watches root,
// debounces events within the given window, and filters paths matching
// extraIgnore glob patterns and .git/.
func newFSWatcher(root string, debounce time.Duration, extraIgnore []string, logFn LogFunc) (*fsWatcher, error) {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("watcher: creating fsnotify watcher: %w", err)
	}

	fw := &fsWatcher{
		watcher: w,
		events:  make(chan Event, 100),
		done:    make(chan struct{}),
		root:    root,
		ignores: extraIgnore,
		logFn:   logFn,
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
