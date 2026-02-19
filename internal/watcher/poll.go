package watcher

import (
	"sync"
	"time"

	"github.com/lurcio/hark/internal/git"
)

// pollWatcher detects changes by periodically calling git.Repo.ChangedFiles.
type pollWatcher struct {
	repo     *git.Repo
	interval time.Duration
	events   chan Event
	done     chan struct{}
	once     sync.Once
	logFn    LogFunc
}

// newPollWatcher creates a polling-based watcher that checks for changes at
// the given interval. Events are emitted when files appear, disappear, or
// change their diff content in the working tree.
func newPollWatcher(repo *git.Repo, interval time.Duration, logFn LogFunc) (*pollWatcher, error) {
	if logFn == nil {
		logFn = func(string, ...any) {}
	}
	pw := &pollWatcher{
		repo:     repo,
		interval: interval,
		events:   make(chan Event, 100),
		done:     make(chan struct{}),
		logFn:    logFn,
	}
	go pw.poll()
	return pw, nil
}

// fileSnapshot captures enough about a changed file to detect further edits.
type fileSnapshot struct {
	status  git.FileStatus
	diffLen int    // length of diff content
	diffSig string // first 64 bytes of diff for quick comparison
}

func makeFileSnapshot(f git.ChangedFile) fileSnapshot {
	sig := f.Diff
	if len(sig) > 64 {
		sig = sig[:64]
	}
	return fileSnapshot{
		status:  f.Status,
		diffLen: len(f.Diff),
		diffSig: sig,
	}
}

func (s fileSnapshot) equals(other fileSnapshot) bool {
	return s.status == other.status && s.diffLen == other.diffLen && s.diffSig == other.diffSig
}

func (pw *pollWatcher) poll() {
	defer close(pw.events)

	prevFiles := make(map[string]fileSnapshot)
	pollCount := 0

	ticker := time.NewTicker(pw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pollCount++
			changed, err := pw.repo.ChangedFiles()
			if err != nil {
				pw.logFn("poll[%d]: ChangedFiles error: %v", pollCount, err)
				continue
			}
			pw.logFn("poll[%d]: got %d changed files", pollCount, len(changed))

			currentFiles := make(map[string]fileSnapshot)
			for _, f := range changed {
				snap := makeFileSnapshot(f)
				currentFiles[f.Path] = snap

				prev, existed := prevFiles[f.Path]
				if !existed || !prev.equals(snap) {
					reason := "new"
					if existed {
						reason = "changed"
					}
					pw.logFn("poll[%d]: emit %s event for %q (%s)", pollCount, statusToKind(f.Status), f.Path, reason)
					select {
					case pw.events <- Event{Path: f.Path, Kind: statusToKind(f.Status)}:
					case <-pw.done:
						return
					}
				}
			}

			// Files that disappeared from the changed set (committed or reverted).
			for path := range prevFiles {
				if _, ok := currentFiles[path]; !ok {
					pw.logFn("poll[%d]: emit deleted event for %q (disappeared)", pollCount, path)
					select {
					case pw.events <- Event{Path: path, Kind: Deleted}:
					case <-pw.done:
						return
					}
				}
			}

			prevFiles = currentFiles

		case <-pw.done:
			return
		}
	}
}

func statusToKind(s git.FileStatus) EventKind {
	switch s {
	case git.StatusAdded:
		return Created
	case git.StatusDeleted:
		return Deleted
	default:
		return Modified
	}
}

func (pw *pollWatcher) Events() <-chan Event {
	return pw.events
}

func (pw *pollWatcher) Close() error {
	pw.once.Do(func() {
		close(pw.done)
	})
	return nil
}
