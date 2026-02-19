package timeline

import "time"

// SnapshotType distinguishes between kinds of timeline entries.
type SnapshotType int

const (
	FileChange SnapshotType = iota
	Commit
)

// AgentEvent is a placeholder for future coding agent integration.
// In v1, Snapshot.Source is always nil.
type AgentEvent struct {
	Action string
	Detail string
}

// Snapshot represents a recorded diff state at a point in time.
type Snapshot struct {
	Timestamp time.Time
	FilePath  string
	Diff      string
	Type      SnapshotType
	Source    *AgentEvent // nil in v1
}
