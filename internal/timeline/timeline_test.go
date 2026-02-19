package timeline

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func makeSnapshot(filePath string, typ SnapshotType) Snapshot {
	return Snapshot{
		Timestamp: time.Now(),
		FilePath:  filePath,
		Diff:      fmt.Sprintf("diff for %s", filePath),
		Type:      typ,
	}
}

func TestNewDefaults(t *testing.T) {
	tl := New(0)
	if tl.maxCap != DefaultMaxSnapshots {
		t.Errorf("expected default max cap %d, got %d", DefaultMaxSnapshots, tl.maxCap)
	}
	if tl.Len() != 0 {
		t.Errorf("expected empty timeline, got %d", tl.Len())
	}
}

func TestAddAndCurrent(t *testing.T) {
	tl := New(10)

	// Empty timeline returns nil.
	if tl.Current() != nil {
		t.Fatal("expected nil for empty timeline")
	}

	s := makeSnapshot("a.go", FileChange)
	tl.Add(s)

	cur := tl.Current()
	if cur == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if cur.FilePath != "a.go" {
		t.Errorf("expected a.go, got %s", cur.FilePath)
	}
}

func TestPositionTracking(t *testing.T) {
	tl := New(10)
	tl.Add(makeSnapshot("a.go", FileChange))
	tl.Add(makeSnapshot("b.go", FileChange))
	tl.Add(makeSnapshot("c.go", FileChange))

	pos, total := tl.Position()
	if pos != 2 || total != 3 {
		t.Errorf("expected pos=2, total=3, got pos=%d, total=%d", pos, total)
	}
	if !tl.IsAtLatest() {
		t.Error("expected IsAtLatest to be true")
	}
}

func TestStepBackAndForward(t *testing.T) {
	tl := New(10)
	tl.Add(makeSnapshot("a.go", FileChange))
	tl.Add(makeSnapshot("b.go", FileChange))
	tl.Add(makeSnapshot("c.go", FileChange))

	// Step back from c -> b.
	if !tl.StepBack() {
		t.Fatal("expected StepBack to succeed")
	}
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected b.go, got %s", cur.FilePath)
	}

	// Step back from b -> a.
	if !tl.StepBack() {
		t.Fatal("expected StepBack to succeed")
	}
	if cur := tl.Current(); cur.FilePath != "a.go" {
		t.Errorf("expected a.go, got %s", cur.FilePath)
	}

	// Step back at beginning should fail.
	if tl.StepBack() {
		t.Error("expected StepBack to fail at beginning")
	}

	// Step forward a -> b.
	if !tl.StepForward() {
		t.Fatal("expected StepForward to succeed")
	}
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected b.go, got %s", cur.FilePath)
	}

	// Step forward b -> c.
	if !tl.StepForward() {
		t.Fatal("expected StepForward to succeed")
	}
	if cur := tl.Current(); cur.FilePath != "c.go" {
		t.Errorf("expected c.go, got %s", cur.FilePath)
	}

	// Step forward at end should fail.
	if tl.StepForward() {
		t.Error("expected StepForward to fail at end")
	}
}

func TestIsAtLatest(t *testing.T) {
	tl := New(10)
	tl.Add(makeSnapshot("a.go", FileChange))
	tl.Add(makeSnapshot("b.go", FileChange))

	if !tl.IsAtLatest() {
		t.Error("expected IsAtLatest true")
	}

	tl.StepBack()
	if tl.IsAtLatest() {
		t.Error("expected IsAtLatest false after StepBack")
	}

	tl.StepForward()
	if !tl.IsAtLatest() {
		t.Error("expected IsAtLatest true after StepForward to end")
	}
}

func TestEvictionAtCapacity(t *testing.T) {
	const cap = 1000
	tl := New(cap)

	// Fill to capacity.
	for i := 0; i < cap; i++ {
		tl.Add(makeSnapshot(fmt.Sprintf("file_%04d.go", i), FileChange))
	}
	if tl.Len() != cap {
		t.Fatalf("expected %d snapshots, got %d", cap, tl.Len())
	}

	// Verify first snapshot.
	tl.StepBack() // go to second to last, then walk to beginning
	for tl.StepBack() {
	}
	if cur := tl.Current(); cur.FilePath != "file_0000.go" {
		t.Errorf("expected file_0000.go at start, got %s", cur.FilePath)
	}

	// Jump back to latest.
	tl.Resume()

	// Add one more, triggering eviction.
	tl.Add(makeSnapshot("file_1000.go", FileChange))
	if tl.Len() != cap {
		t.Fatalf("expected %d snapshots after eviction, got %d", cap, tl.Len())
	}

	// The oldest (file_0000.go) should be gone; first should now be file_0001.go.
	for tl.StepBack() {
	}
	if cur := tl.Current(); cur.FilePath != "file_0001.go" {
		t.Errorf("expected file_0001.go after eviction, got %s", cur.FilePath)
	}
}

func TestEvictionSmallCapacity(t *testing.T) {
	tl := New(3)
	tl.Add(makeSnapshot("a.go", FileChange))
	tl.Add(makeSnapshot("b.go", FileChange))
	tl.Add(makeSnapshot("c.go", FileChange))
	tl.Add(makeSnapshot("d.go", FileChange)) // evicts a.go

	if tl.Len() != 3 {
		t.Fatalf("expected 3 snapshots, got %d", tl.Len())
	}

	// Walk to beginning.
	for tl.StepBack() {
	}
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected b.go at start after eviction, got %s", cur.FilePath)
	}

	// Walk to end.
	for tl.StepForward() {
	}
	if cur := tl.Current(); cur.FilePath != "d.go" {
		t.Errorf("expected d.go at end, got %s", cur.FilePath)
	}
}

func TestJumpToCommit(t *testing.T) {
	tl := New(100)

	// Add: file, file, COMMIT, file, file, COMMIT, file
	tl.Add(makeSnapshot("a.go", FileChange))   // 0
	tl.Add(makeSnapshot("b.go", FileChange))   // 1
	tl.Add(makeSnapshot("c1", Commit))          // 2
	tl.Add(makeSnapshot("c.go", FileChange))    // 3
	tl.Add(makeSnapshot("d.go", FileChange))    // 4
	tl.Add(makeSnapshot("c2", Commit))          // 5
	tl.Add(makeSnapshot("e.go", FileChange))    // 6

	// Position is at 6 (latest). Jump to prev commit -> 5.
	if !tl.JumpToPrevCommit() {
		t.Fatal("expected JumpToPrevCommit to succeed")
	}
	if cur := tl.Current(); cur.FilePath != "c2" {
		t.Errorf("expected c2, got %s", cur.FilePath)
	}

	// Jump to prev commit again -> 2.
	if !tl.JumpToPrevCommit() {
		t.Fatal("expected JumpToPrevCommit to succeed")
	}
	if cur := tl.Current(); cur.FilePath != "c1" {
		t.Errorf("expected c1, got %s", cur.FilePath)
	}

	// No more previous commits.
	if tl.JumpToPrevCommit() {
		t.Error("expected JumpToPrevCommit to fail with no more commits")
	}

	// Jump forward: from 2 -> 5.
	if !tl.JumpToNextCommit() {
		t.Fatal("expected JumpToNextCommit to succeed")
	}
	if cur := tl.Current(); cur.FilePath != "c2" {
		t.Errorf("expected c2, got %s", cur.FilePath)
	}

	// No more next commits (6 is a FileChange).
	if tl.JumpToNextCommit() {
		t.Error("expected JumpToNextCommit to fail with no more commits")
	}
}

func TestPauseAndResume(t *testing.T) {
	tl := New(100)
	tl.Add(makeSnapshot("a.go", FileChange))
	tl.Add(makeSnapshot("b.go", FileChange))

	// Pause.
	tl.Pause()
	if !tl.IsPaused() {
		t.Error("expected IsPaused true")
	}

	// Record the position before adding.
	posBefore, _ := tl.Position()

	// Add while paused — position should NOT advance.
	tl.Add(makeSnapshot("c.go", FileChange))
	tl.Add(makeSnapshot("d.go", FileChange))

	posAfter, total := tl.Position()
	if posAfter != posBefore {
		t.Errorf("expected position unchanged at %d, got %d", posBefore, posAfter)
	}
	if total != 4 {
		t.Errorf("expected 4 total snapshots, got %d", total)
	}
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected b.go while paused, got %s", cur.FilePath)
	}
	if tl.IsAtLatest() {
		t.Error("expected IsAtLatest false while paused with new snapshots")
	}

	// Resume — should jump to latest.
	tl.Resume()
	if tl.IsPaused() {
		t.Error("expected IsPaused false after Resume")
	}
	if cur := tl.Current(); cur.FilePath != "d.go" {
		t.Errorf("expected d.go after Resume, got %s", cur.FilePath)
	}
	if !tl.IsAtLatest() {
		t.Error("expected IsAtLatest true after Resume")
	}
}

func TestPauseNavigationWhilePaused(t *testing.T) {
	tl := New(100)
	tl.Add(makeSnapshot("a.go", FileChange))
	tl.Add(makeSnapshot("b.go", FileChange))
	tl.Add(makeSnapshot("c.go", FileChange))

	tl.Pause()

	// Can navigate while paused.
	tl.StepBack()
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected b.go, got %s", cur.FilePath)
	}

	// New snapshots still don't move position.
	tl.Add(makeSnapshot("d.go", FileChange))
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected still b.go, got %s", cur.FilePath)
	}
}

func TestEvictionWhilePaused(t *testing.T) {
	tl := New(3)
	tl.Add(makeSnapshot("a.go", FileChange)) // 0
	tl.Add(makeSnapshot("b.go", FileChange)) // 1
	tl.Add(makeSnapshot("c.go", FileChange)) // 2

	// Pause at position 2 (c.go).
	tl.Pause()

	// Step back to position 1 (b.go).
	tl.StepBack()
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Fatalf("expected b.go, got %s", cur.FilePath)
	}

	// Add a new snapshot, evicting a.go. Position should shift.
	tl.Add(makeSnapshot("d.go", FileChange))

	// b.go should still be current (it shifted from index 1 to 0).
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected b.go after eviction while paused, got %s", cur.FilePath)
	}
	if tl.Len() != 3 {
		t.Errorf("expected 3 snapshots, got %d", tl.Len())
	}
}

func TestConcurrentAccess(t *testing.T) {
	tl := New(100)
	var wg sync.WaitGroup

	// Concurrent writers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tl.Add(makeSnapshot(fmt.Sprintf("file_%d_%d.go", n, j), FileChange))
			}
		}(i)
	}

	// Concurrent readers.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tl.Current()
				tl.Position()
				tl.IsAtLatest()
				tl.Len()
			}
		}()
	}

	wg.Wait()

	// Just verify it didn't panic and has a valid state.
	if tl.Len() > 100 {
		t.Errorf("expected at most 100 snapshots, got %d", tl.Len())
	}
}

func TestEmptyTimeline(t *testing.T) {
	tl := New(10)

	if tl.Current() != nil {
		t.Error("expected nil Current on empty timeline")
	}
	if tl.StepBack() {
		t.Error("expected StepBack to fail on empty timeline")
	}
	if tl.StepForward() {
		t.Error("expected StepForward to fail on empty timeline")
	}
	if tl.JumpToNextCommit() {
		t.Error("expected JumpToNextCommit to fail on empty timeline")
	}
	if tl.JumpToPrevCommit() {
		t.Error("expected JumpToPrevCommit to fail on empty timeline")
	}
	if !tl.IsAtLatest() {
		t.Error("expected IsAtLatest true on empty timeline")
	}

	pos, total := tl.Position()
	if pos != -1 || total != 0 {
		t.Errorf("expected pos=-1, total=0, got pos=%d, total=%d", pos, total)
	}
}

func TestSingleSnapshot(t *testing.T) {
	tl := New(10)
	tl.Add(makeSnapshot("only.go", FileChange))

	if tl.StepBack() {
		t.Error("expected StepBack to fail with single snapshot")
	}
	if tl.StepForward() {
		t.Error("expected StepForward to fail with single snapshot")
	}

	pos, total := tl.Position()
	if pos != 0 || total != 1 {
		t.Errorf("expected pos=0, total=1, got pos=%d, total=%d", pos, total)
	}
}

func TestSnapshotTypes(t *testing.T) {
	s := Snapshot{
		Timestamp: time.Now(),
		FilePath:  "test.go",
		Diff:      "some diff",
		Type:      FileChange,
		Source:    nil,
	}
	if s.Type != FileChange {
		t.Error("expected FileChange type")
	}

	s.Source = &AgentEvent{Action: "edit", Detail: "test"}
	if s.Source.Action != "edit" {
		t.Error("expected AgentEvent action 'edit'")
	}
}

func TestCapacityOne(t *testing.T) {
	tl := New(1)
	tl.Add(makeSnapshot("a.go", FileChange))
	tl.Add(makeSnapshot("b.go", FileChange))

	if tl.Len() != 1 {
		t.Fatalf("expected 1 snapshot, got %d", tl.Len())
	}
	if cur := tl.Current(); cur.FilePath != "b.go" {
		t.Errorf("expected b.go, got %s", cur.FilePath)
	}
}
