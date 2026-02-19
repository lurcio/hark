// Package watcher provides file change detection for a git repository.
// It supports fsnotify-based filesystem watching (primary) and git polling (fallback).
package watcher

import (
	"sync"
	"time"

	"github.com/lurcio/hark/internal/config"
	"github.com/lurcio/hark/internal/git"
)

// EventKind represents the type of file change.
type EventKind int

const (
	Created EventKind = iota
	Modified
	Deleted
)

func (k EventKind) String() string {
	switch k {
	case Created:
		return "created"
	case Modified:
		return "modified"
	case Deleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// Event represents a file change event.
type Event struct {
	Path string
	Kind EventKind
}

// Watcher watches a git repository for file changes and emits events.
type Watcher interface {
	// Events returns a channel that receives file change events.
	Events() <-chan Event
	// Close stops the watcher and releases resources.
	Close() error
}

// LogFunc is an optional debug logging function.
type LogFunc func(format string, args ...any)

// New creates a Watcher based on the given configuration.
// If PollOnly is set, only git polling is used. Otherwise, fsnotify is the
// primary watcher with git polling as a fallback.
// The optional logFn is used for debug logging; pass nil to disable.
func New(cfg config.Config, repo *git.Repo, logFn LogFunc) (Watcher, error) {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}
	if cfg.Watch.PollOnly {
		return newPollWatcher(repo, cfg.Watch.PollInterval.Duration, logFn)
	}
	return newCombinedWatcher(cfg, repo, logFn)
}

// combinedWatcher merges events from the filesystem watcher and poll watcher.
type combinedWatcher struct {
	fs     *fsWatcher
	poll   *pollWatcher
	events chan Event
	done   chan struct{}
	once   sync.Once
	logFn  LogFunc
}

func newCombinedWatcher(cfg config.Config, repo *git.Repo, logFn LogFunc) (*combinedWatcher, error) {
	fsw, err := newFSWatcher(repo.Path(), cfg.Watch.Debounce.Duration, cfg.Filter.ExtraIgnore, logFn)
	if err != nil {
		return nil, err
	}

	pw, err := newPollWatcher(repo, cfg.Watch.PollInterval.Duration, logFn)
	if err != nil {
		fsw.Close()
		return nil, err
	}

	cw := &combinedWatcher{
		fs:     fsw,
		poll:   pw,
		events: make(chan Event, 100),
		done:   make(chan struct{}),
		logFn:  logFn,
	}

	go cw.merge()
	return cw, nil
}

func (cw *combinedWatcher) merge() {
	defer close(cw.events)
	for {
		select {
		case evt, ok := <-cw.fs.Events():
			if !ok {
				cw.logFn("combined: fs channel closed")
				return
			}
			cw.logFn("combined: fs event: path=%q kind=%v", evt.Path, evt.Kind)
			select {
			case cw.events <- evt:
			case <-cw.done:
				return
			}
		case evt, ok := <-cw.poll.Events():
			if !ok {
				cw.logFn("combined: poll channel closed")
				return
			}
			cw.logFn("combined: poll event: path=%q kind=%v", evt.Path, evt.Kind)
			select {
			case cw.events <- evt:
			case <-cw.done:
				return
			}
		case <-cw.done:
			return
		}
	}
}

func (cw *combinedWatcher) Events() <-chan Event {
	return cw.events
}

func (cw *combinedWatcher) Close() error {
	var err1, err2 error
	cw.once.Do(func() {
		close(cw.done)
		err1 = cw.fs.Close()
		err2 = cw.poll.Close()
	})
	if err1 != nil {
		return err1
	}
	return err2
}

// runDebouncer reads from raw and writes debounced events to out.
// Events for the same Path within window are coalesced into a single event.
// Returns when raw is closed or done is signalled. Does not close out.
func runDebouncer(raw <-chan Event, window time.Duration, out chan<- Event, done <-chan struct{}) {
	if window <= 0 {
		// Passthrough — no debouncing.
		for {
			select {
			case evt, ok := <-raw:
				if !ok {
					return
				}
				select {
				case out <- evt:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	}

	expired := make(chan string, 100)
	timers := make(map[string]*time.Timer)
	pending := make(map[string]Event)

	for {
		select {
		case evt, ok := <-raw:
			if !ok {
				for _, t := range timers {
					t.Stop()
				}
				for _, e := range pending {
					select {
					case out <- e:
					case <-done:
						return
					}
				}
				return
			}
			pending[evt.Path] = evt
			if t, exists := timers[evt.Path]; exists {
				t.Reset(window)
			} else {
				path := evt.Path
				timers[path] = time.AfterFunc(window, func() {
					select {
					case expired <- path:
					case <-done:
					}
				})
			}

		case path := <-expired:
			if e, ok := pending[path]; ok {
				select {
				case out <- e:
				case <-done:
					return
				}
				delete(pending, path)
				delete(timers, path)
			}

		case <-done:
			for _, t := range timers {
				t.Stop()
			}
			return
		}
	}
}
