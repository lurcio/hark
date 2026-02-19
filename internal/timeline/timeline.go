package timeline

import "sync"

// DefaultMaxSnapshots is the default maximum number of snapshots retained.
const DefaultMaxSnapshots = 1000

// Timeline provides in-memory, append-only storage of snapshots with
// navigation, pause/resume, and configurable eviction.
// It is safe for concurrent use.
type Timeline struct {
	mu        sync.RWMutex
	snapshots []Snapshot
	pos       int  // current viewing position (0-based index into snapshots)
	paused    bool // when paused, new snapshots are recorded but pos doesn't advance
	maxCap    int  // maximum number of snapshots before eviction
}

// New creates a Timeline with the given maximum capacity.
// If maxCap <= 0, DefaultMaxSnapshots is used.
func New(maxCap int) *Timeline {
	if maxCap <= 0 {
		maxCap = DefaultMaxSnapshots
	}
	return &Timeline{
		snapshots: make([]Snapshot, 0, min(maxCap, 1024)),
		pos:       -1, // no snapshots yet
		maxCap:    maxCap,
	}
}

// Add appends a snapshot to the timeline, evicting the oldest entry if at
// capacity. When not paused, the viewing position advances to the new snapshot.
func (t *Timeline) Add(s Snapshot) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.snapshots) >= t.maxCap {
		// Evict the oldest snapshot.
		copy(t.snapshots, t.snapshots[1:])
		t.snapshots = t.snapshots[:len(t.snapshots)-1]
		// Adjust position for the removed element.
		if t.pos > 0 {
			t.pos--
		} else {
			t.pos = 0
		}
	}

	t.snapshots = append(t.snapshots, s)

	if !t.paused {
		t.pos = len(t.snapshots) - 1
	}
}

// Current returns the snapshot at the current viewing position, or nil if the
// timeline is empty.
func (t *Timeline) Current() *Snapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.snapshots) == 0 || t.pos < 0 || t.pos >= len(t.snapshots) {
		return nil
	}
	s := t.snapshots[t.pos]
	return &s
}

// StepBack moves the viewing position one snapshot backward. It returns false
// if already at the beginning.
func (t *Timeline) StepBack() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.pos <= 0 {
		return false
	}
	t.pos--
	return true
}

// StepForward moves the viewing position one snapshot forward. It returns false
// if already at the latest snapshot.
func (t *Timeline) StepForward() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.pos >= len(t.snapshots)-1 {
		return false
	}
	t.pos++
	return true
}

// JumpToNextCommit advances the viewing position to the next Commit snapshot
// after the current position. Returns false if no such snapshot exists.
func (t *Timeline) JumpToNextCommit() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i := t.pos + 1; i < len(t.snapshots); i++ {
		if t.snapshots[i].Type == Commit {
			t.pos = i
			return true
		}
	}
	return false
}

// JumpToPrevCommit moves the viewing position to the previous Commit snapshot
// before the current position. Returns false if no such snapshot exists.
func (t *Timeline) JumpToPrevCommit() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i := t.pos - 1; i >= 0; i-- {
		if t.snapshots[i].Type == Commit {
			t.pos = i
			return true
		}
	}
	return false
}

// Pause stops the viewing position from auto-advancing when new snapshots are
// added. Snapshots continue to be recorded.
func (t *Timeline) Pause() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = true
}

// Resume unpauses and advances the viewing position to the latest snapshot.
func (t *Timeline) Resume() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.paused = false
	if len(t.snapshots) > 0 {
		t.pos = len(t.snapshots) - 1
	}
}

// IsPaused reports whether the timeline is paused.
func (t *Timeline) IsPaused() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.paused
}

// IsAtLatest reports whether the viewing position is at the most recent snapshot.
func (t *Timeline) IsAtLatest() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.snapshots) == 0 {
		return true
	}
	return t.pos == len(t.snapshots)-1
}

// Position returns the current 0-based index and total number of snapshots.
func (t *Timeline) Position() (current, total int) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.pos, len(t.snapshots)
}

// Len returns the number of snapshots in the timeline.
func (t *Timeline) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.snapshots)
}
