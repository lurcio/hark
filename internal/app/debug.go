package app

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// DebugLogger writes timestamped debug lines to a file.
// It is a no-op when disabled.
type DebugLogger struct {
	mu   sync.Mutex
	file *os.File
}

// NewDebugLogger creates a debug logger. If path is empty, logging is disabled.
func NewDebugLogger(path string) *DebugLogger {
	if path == "" {
		return &DebugLogger{}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not open debug log %s: %v\n", path, err)
		return &DebugLogger{}
	}
	return &DebugLogger{file: f}
}

// Log writes a formatted message with a timestamp.
func (d *DebugLogger) Log(format string, args ...any) {
	if d == nil || d.file == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(d.file, "[%s] %s\n", ts, fmt.Sprintf(format, args...))
}

// Close closes the log file.
func (d *DebugLogger) Close() {
	if d == nil || d.file == nil {
		return
	}
	d.file.Close()
}

// Enabled reports whether the logger is active.
func (d *DebugLogger) Enabled() bool {
	return d != nil && d.file != nil
}
