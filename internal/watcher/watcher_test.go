package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/lurcio/hark/internal/git"
)

// --- Debouncer tests ---

func TestDebouncerCoalesces(t *testing.T) {
	raw := make(chan Event, 100)
	out := make(chan Event, 100)
	done := make(chan struct{})

	go func() {
		runDebouncer(raw, 50*time.Millisecond, out, done)
	}()

	// Send 10 rapid events for the same file.
	for i := 0; i < 10; i++ {
		raw <- Event{Path: "test.go", Kind: Modified}
	}

	// Wait for the debounced event.
	select {
	case evt := <-out:
		if evt.Path != "test.go" || evt.Kind != Modified {
			t.Errorf("unexpected event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for debounced event")
	}

	// No additional events should arrive.
	select {
	case evt := <-out:
		t.Fatalf("unexpected extra event: %+v", evt)
	case <-time.After(200 * time.Millisecond):
		// expected
	}

	close(done)
}

func TestDebouncerMultipleFiles(t *testing.T) {
	raw := make(chan Event, 100)
	out := make(chan Event, 100)
	done := make(chan struct{})

	go func() {
		runDebouncer(raw, 50*time.Millisecond, out, done)
	}()

	// Send rapid events for two different files.
	for i := 0; i < 5; i++ {
		raw <- Event{Path: "a.go", Kind: Modified}
		raw <- Event{Path: "b.go", Kind: Created}
	}

	// Collect debounced events.
	received := make(map[string]Event)
	timeout := time.After(2 * time.Second)
	for len(received) < 2 {
		select {
		case evt := <-out:
			received[evt.Path] = evt
		case <-timeout:
			t.Fatalf("timeout; received only %d events", len(received))
		}
	}

	if received["a.go"].Kind != Modified {
		t.Errorf("expected Modified for a.go, got %v", received["a.go"].Kind)
	}
	if received["b.go"].Kind != Created {
		t.Errorf("expected Created for b.go, got %v", received["b.go"].Kind)
	}

	close(done)
}

func TestDebouncerPassthrough(t *testing.T) {
	raw := make(chan Event, 100)
	out := make(chan Event, 100)
	done := make(chan struct{})

	// Window of 0 means passthrough — no debouncing.
	go func() {
		runDebouncer(raw, 0, out, done)
	}()

	raw <- Event{Path: "a.go", Kind: Modified}
	raw <- Event{Path: "a.go", Kind: Modified}

	// Both events should pass through.
	for i := 0; i < 2; i++ {
		select {
		case <-out:
		case <-time.After(time.Second):
			t.Fatalf("timeout on event %d", i)
		}
	}

	close(done)
}

func TestDebouncerLastEventWins(t *testing.T) {
	raw := make(chan Event, 100)
	out := make(chan Event, 100)
	done := make(chan struct{})

	go func() {
		runDebouncer(raw, 50*time.Millisecond, out, done)
	}()

	// Send Created then Modified for the same file — Modified should win.
	raw <- Event{Path: "x.go", Kind: Created}
	raw <- Event{Path: "x.go", Kind: Modified}

	select {
	case evt := <-out:
		if evt.Kind != Modified {
			t.Errorf("expected last event (Modified) to win, got %v", evt.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	close(done)
}

// --- Combined watcher dedup tests ---

func TestEmitDedupFirstEventPasses(t *testing.T) {
	cw := &combinedWatcher{
		events: make(chan Event, 10),
		done:   make(chan struct{}),
		logFn:  func(string, ...any) {},
	}

	recent := make(map[string]time.Time)
	evt := Event{Path: "file.go", Kind: Modified}

	if !cw.emitDedup(evt, recent) {
		t.Error("expected first event to be emitted")
	}

	select {
	case got := <-cw.events:
		if got.Path != "file.go" {
			t.Errorf("expected path file.go, got %s", got.Path)
		}
	default:
		t.Error("expected event in channel")
	}
}

func TestEmitDedupSuppressesDuplicate(t *testing.T) {
	cw := &combinedWatcher{
		events: make(chan Event, 10),
		done:   make(chan struct{}),
		logFn:  func(string, ...any) {},
	}

	recent := make(map[string]time.Time)
	evt := Event{Path: "file.go", Kind: Modified}

	// First event passes.
	if !cw.emitDedup(evt, recent) {
		t.Fatal("expected first event to be emitted")
	}
	<-cw.events // drain

	// Second event for the same path within the dedup window is suppressed.
	if cw.emitDedup(evt, recent) {
		t.Error("expected duplicate event to be suppressed")
	}

	select {
	case got := <-cw.events:
		t.Errorf("unexpected event in channel: %+v", got)
	default:
		// expected — no event
	}
}

func TestEmitDedupAllowsDifferentPaths(t *testing.T) {
	cw := &combinedWatcher{
		events: make(chan Event, 10),
		done:   make(chan struct{}),
		logFn:  func(string, ...any) {},
	}

	recent := make(map[string]time.Time)

	if !cw.emitDedup(Event{Path: "a.go", Kind: Modified}, recent) {
		t.Error("expected event for a.go to be emitted")
	}
	if !cw.emitDedup(Event{Path: "b.go", Kind: Modified}, recent) {
		t.Error("expected event for b.go to be emitted")
	}

	// Both should be in the channel.
	for _, expected := range []string{"a.go", "b.go"} {
		select {
		case got := <-cw.events:
			if got.Path != expected {
				t.Errorf("expected path %s, got %s", expected, got.Path)
			}
		default:
			t.Errorf("expected event for %s in channel", expected)
		}
	}
}

func TestEmitDedupAllowsAfterWindowExpires(t *testing.T) {
	cw := &combinedWatcher{
		events: make(chan Event, 10),
		done:   make(chan struct{}),
		logFn:  func(string, ...any) {},
	}

	recent := make(map[string]time.Time)
	evt := Event{Path: "file.go", Kind: Modified}

	// Simulate an event that was emitted beyond the dedup window.
	recent["file.go"] = time.Now().Add(-3 * time.Second)

	if !cw.emitDedup(evt, recent) {
		t.Error("expected event to be emitted after dedup window expired")
	}
}

func TestEmitDedupDoneChannelStopsEmit(t *testing.T) {
	cw := &combinedWatcher{
		events: make(chan Event), // unbuffered — will block
		done:   make(chan struct{}),
		logFn:  func(string, ...any) {},
	}

	close(cw.done)
	recent := make(map[string]time.Time)
	evt := Event{Path: "file.go", Kind: Modified}

	if cw.emitDedup(evt, recent) {
		t.Error("expected emitDedup to return false when done is closed")
	}
}

// --- Poll watcher tests ---

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}

	// Create an initial file and commit so HEAD exists.
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial\n"), 0644); err != nil {
		t.Fatal(err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add("initial.txt"); err != nil {
		t.Fatal(err)
	}
	_, err = wt.Commit("initial commit", &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@test.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestPollWatcherDetectsNewFile(t *testing.T) {
	dir := initTestRepo(t)

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	pw, err := newPollWatcher(repo, 50*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("newPollWatcher: %v", err)
	}
	defer pw.Close()

	// Create a new file in the repo.
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-pw.Events():
		if evt.Path != "new.txt" {
			t.Errorf("expected path new.txt, got %s", evt.Path)
		}
		if evt.Kind != Created {
			t.Errorf("expected Created, got %v", evt.Kind)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPollWatcherDetectsModification(t *testing.T) {
	dir := initTestRepo(t)

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatalf("git.Open: %v", err)
	}

	pw, err := newPollWatcher(repo, 50*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pw.Close()

	// Modify an existing tracked file.
	if err := os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-pw.Events():
		if evt.Path != "initial.txt" {
			t.Errorf("expected path initial.txt, got %s", evt.Path)
		}
		if evt.Kind != Modified {
			t.Errorf("expected Modified, got %v", evt.Kind)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPollWatcherNoSpuriousEvents(t *testing.T) {
	dir := initTestRepo(t)

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	pw, err := newPollWatcher(repo, 50*time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pw.Close()

	// No changes — should not receive events.
	select {
	case evt := <-pw.Events():
		t.Fatalf("unexpected event: %+v", evt)
	case <-time.After(300 * time.Millisecond):
		// expected
	}
}
