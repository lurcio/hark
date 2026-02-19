# hark

> A real-time, read-only TUI for watching changes to a git repository as they happen.

Built for developers who want to follow along as coding agents (or teammates) make changes — like spectator mode for your codebase. "Hark!" — to listen and pay close attention — because this tool watches everything, understands the changes deeply, and never directly modifies a thing.

---

## Overview

`hark` monitors a git working directory for file changes and displays diffs in real time with syntax highlighting, a navigable timeline, and a beautiful terminal interface. It is strictly read-only — no staging, committing, or editing — but provides a single-keypress escape hatch to open any file in your editor.

### Design Principles

- **Watch, don't touch.** No git write operations. No editing. Pure observation.
- **Stay out of the way.** Auto-focuses the most recently changed file. Minimal interaction required.
- **Look good.** Syntax-highlighted diffs, configurable colour themes, clean layout.
- **Work everywhere.** macOS, Linux, and Windows from a single binary.

---

## Technical Stack

| Component | Choice | Rationale |
|---|---|---|
| Language | Go | Single binary distribution, excellent cross-platform support, fast compilation |
| TUI framework | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss) | Best-in-class Go TUI libraries, active community, composable components |
| Syntax highlighting | [Chroma](https://github.com/alecthomas/chroma) | Go-native, 200+ languages, same engine as GitHub/Hugo |
| Git operations | [go-git](https://github.com/go-git/go-git) | Pure Go git implementation, no system git dependency |
| File watching | [fsnotify](https://github.com/fsnotify/fsnotify) | Cross-platform (inotify, FSEvents, ReadDirectoryChangesW) |

---

## Change Detection

### Primary: Filesystem Watcher

Uses `fsnotify` to receive OS-level file change events (inotify on Linux, FSEvents on macOS, ReadDirectoryChangesW on Windows). This provides near-instant detection of file writes.

**Behaviour:**

- Watches all files in the repository recursively.
- Respects `.gitignore` at all levels (root, nested subdirectories, and global `~/.config/git/ignore`) — ignored files are never watched or displayed.
- Supports additional user-defined ignore patterns via config (glob syntax).
- Debounces rapid successive writes to the same file (e.g. 100ms window) to avoid flooding the UI during bulk saves.

### Fallback: Git Polling

A configurable polling interval (default: 1 second) runs `git diff` against the working tree. This handles edge cases where filesystem events are missed (network filesystems, containerised environments, OS event queue overflow).

**Behaviour:**

- Runs on a background goroutine at the configured interval.
- Only active as a fallback — does not duplicate events already captured by the watcher.
- Can be forced as the sole detection method via `--poll-only` flag.

---

## Scope

`hark` tracks **unstaged working directory changes only**.

- Compares the working tree against the current HEAD (or the index if files are staged).
- Does not track staged changes, commits, stash operations, or branch switches.
- If the user makes a commit (or the agent does), the diff view resets — all committed changes disappear, and only new modifications are shown.

---

## User Interface

### Layout: Single Pane with Togglable Overlay

The primary view is a full-screen diff viewer. A file list panel can be toggled on/off as a left-side overlay.

```
┌─────────────────────────────────────────────────────┐
│ hark  ~/projects/myapp         ▶ watching   3 files │  ← status bar
├─────────────────────────────────────────────────────┤
│                                                     │
│  src/api/handler.go                                 │  ← filename header
│                                                     │
│  @@ -42,7 +42,9 @@ func HandleRequest(...)          │
│   42 │   ctx := r.Context()                         │
│   43 │   logger := log.FromContext(ctx)              │
│   44 │-  resp, err := svc.Process(ctx, req)          │
│   44 │+  resp, err := svc.Process(ctx, req, opts)    │
│   45 │+  if err != nil {                             │
│   46 │+      return fmt.Errorf("process: %w", err)   │
│   47 │   }                                           │
│                                                     │
├─────────────────────────────────────────────────────┤
│ ◀ ■ ▶  ────●──────────────── 14/27    12:04:32      │  ← timeline bar
└─────────────────────────────────────────────────────┘
```

**With file list overlay open (`Tab`):**

```
┌─────────────────────────────────────────────────────┐
│ hark  ~/projects/myapp         ▶ watching   3 files │
├──────────────┬──────────────────────────────────────┤
│ Changed Files│                                      │
│              │  src/api/handler.go                   │
│ ● handler.go│                                      │
│   router.go  │  @@ -42,7 +42,9 @@                   │
│ ● config.yml│   42 │ ctx := r.Context()             │
│              │   ...                                 │
│              │                                      │
├──────────────┴──────────────────────────────────────┤
│ ◀ ■ ▶  ────●──────────────── 14/27    12:04:32      │
└─────────────────────────────────────────────────────┘
```

- `●` badge indicates unseen changes on a file since it was last viewed.

### Status Bar (top)

- Tool name and watched directory path.
- Current state: `▶ watching`, `⏸ paused`, `⏪ rewinding`.
- Count of currently changed files.

### Diff View (centre)

- Filename header showing the relative path of the currently viewed file.
- Syntax-highlighted diff content.
- Line numbers shown for context.
- Scrollable with arrow keys and vi bindings.

### Timeline Bar (bottom)

- Transport controls: previous (`◀`), play/pause (`■`), next (`▶`).
- Scrubber showing position within the change timeline.
- Snapshot index (e.g. `14/27` = viewing snapshot 14 of 27).
- Timestamp of the currently viewed snapshot.

---

## Diff Display Styles

Users can toggle between three styles with a keybinding:

| Style | Description |
|---|---|
| **Unified** | Standard `git diff` format. Added lines in green, removed in red. Default. |
| **Side-by-side** | Old file on left, new file on right. Requires sufficient terminal width; falls back to unified if too narrow. |
| **Full file** | Entire file displayed with changed lines highlighted. Useful for understanding changes in full context. |

The current style is persisted in the config file when changed.

---

## Timeline & History

### Snapshots

Every detected file change creates a **snapshot** — a recorded diff state at a point in time. Snapshots are stored in memory (not persisted to disk).

A snapshot contains:
- Timestamp
- File path
- Diff content (unified diff format)
- Type: `file_change` or `commit`

### Commit Milestones

When a git commit is detected (HEAD changes), a **commit milestone** is inserted into the timeline. Milestones show the commit message and act as navigation landmarks.

### Playback Controls

| Action | Description |
|---|---|
| **Pause** | Freezes the view. New changes are still recorded but the display doesn't advance. |
| **Resume** | Catches up to the latest state (optionally fast-forwarding through intermediate snapshots). |
| **Step back** | Moves to the previous snapshot in the timeline. Automatically pauses. |
| **Step forward** | Moves to the next snapshot. If at the latest, resumes live watching. |
| **Jump to commit** | Skip backward/forward to the nearest commit milestone. |

### Memory Management

- Default: retain the last 1,000 snapshots (configurable).
- Oldest snapshots are evicted when the limit is reached.
- No disk persistence — timeline resets when `hark` exits.

---

## Syntax Highlighting

Powered by Chroma with user-selectable themes.

### Bundled Themes

- Dracula (default dark)
- Catppuccin Mocha
- Catppuccin Latte (default light)
- Monokai
- Nord
- Solarized Dark
- Solarized Light
- One Dark
- GitHub Dark
- GitHub Light

Theme is selectable via config file or `--theme` flag. A keybinding cycles through available themes at runtime.

### Language Detection

Automatic based on file extension and, where ambiguous, file content heuristics (via Chroma's built-in detection).

---

## Keybindings

### Navigation

| Key | Action |
|---|---|
| `j` / `↓` | Scroll down one line |
| `k` / `↑` | Scroll up one line |
| `d` / `Page Down` | Scroll down half page |
| `u` / `Page Up` | Scroll up half page |
| `g` | Jump to top of diff |
| `G` | Jump to bottom of diff |
| `n` | Jump to next changed hunk |
| `N` | Jump to previous changed hunk |

### File Management

| Key | Action |
|---|---|
| `Tab` | Toggle file list overlay |
| `]` | Next changed file |
| `[` | Previous changed file |
| `1-9` | Jump to file by index (when file list is open) |

### Timeline

| Key | Action |
|---|---|
| `Space` | Pause / resume |
| `h` / `←` | Step back one snapshot |
| `l` / `→` | Step forward one snapshot |
| `H` | Jump to previous commit milestone |
| `L` | Jump to next commit milestone |

### Display

| Key | Action |
|---|---|
| `v` | Cycle diff style (unified → side-by-side → full file) |
| `t` | Cycle colour theme |
| `w` | Toggle word-level diff highlighting |
| `+` / `-` | Adjust context lines shown around changes |

### General

| Key | Action |
|---|---|
| `?` | Toggle help overlay |
| `e` | Open current file at current line in `$EDITOR` |
| `/` | Search within current diff |
| `q` | Quit |

---

## Configuration

### Config File

Location: `~/.config/hark/config.toml`

```toml
# Change detection
[watch]
poll_interval = "1s"         # Git polling fallback interval
poll_only = false             # Disable filesystem watcher, use polling only
debounce = "100ms"            # Debounce window for rapid file writes

# Display
[display]
diff_style = "unified"        # "unified", "side-by-side", "full-file"
theme = "dracula"             # Chroma theme name
word_diff = false             # Word-level diff highlighting
context_lines = 3             # Lines of context around changes
show_line_numbers = true      # Show line numbers in diff view

# Timeline
[timeline]
max_snapshots = 1000          # Maximum snapshots retained in memory

# Filtering
[filter]
extra_ignore = [              # Additional glob patterns to ignore
    "*.log",
    ".env*",
    "dist/**",
]

# Editor integration
[editor]
command = ""                  # Override $EDITOR (e.g. "code --goto")
```

### CLI Flags

All config options can be overridden via CLI flags. Flags take precedence over the config file.

```
Usage:
  hark [path] [flags]

Arguments:
  path    Path to git repository (default: current directory)

Flags:
      --poll-only            Use git polling only (no filesystem watcher)
      --poll-interval dur    Polling interval (default: 1s)
      --theme string         Colour theme (default: dracula)
      --diff-style string    Diff display style: unified, side-by-side, full-file
      --context int          Lines of context around changes (default: 3)
      --ignore strings       Additional ignore patterns (can be repeated)
      --max-snapshots int    Max timeline snapshots (default: 1000)
  -h, --help                 Show help
  -v, --version              Show version
```

---

## File Filtering

### Default Behaviour

- All paths matched by `.gitignore` are excluded, including nested `.gitignore` files in subdirectories (matching git's own precedence rules).
- The global gitignore file is also respected (`~/.config/git/ignore`, or as configured via `core.excludesFile` in git config).
- `.git/` directory is always excluded.
- Binary files are detected and shown as `Binary file changed` rather than attempting a diff.

### Custom Patterns

Additional ignore patterns are specified via `filter.extra_ignore` in config or `--ignore` flag. Patterns use standard glob syntax.

---

## Stretch Goal: Coding Agent Integration

> This feature is deferred to a future version but included here to inform architectural decisions.

### Concept

When watching a coding agent work, `hark` could optionally parse agent output or event streams to annotate the timeline with agent context — e.g. which tool call triggered a file change, what prompt the agent was responding to.

### Possible Approaches

- **Log file tailing**: Watch a structured log file (JSON lines) that the agent writes, correlating timestamps with file changes.
- **Named pipe / Unix socket**: Agent pushes events to `hark` directly.
- **Generic stdin**: Pipe agent output into `hark` and parse structured events.

### UI Impact

- Timeline entries annotated with agent action labels (e.g. `"Edit file: handler.go"`, `"Run tests"`).
- Optional side panel showing the agent's current action/thought.

### Architectural Consideration

The snapshot model should include an optional `source` field from the start, even if it's unused in v1. This avoids a data model change later.

```go
type Snapshot struct {
    Timestamp time.Time
    FilePath  string
    Diff      string
    Type      SnapshotType  // FileChange, Commit
    Source    *AgentEvent   // nil in v1, populated in future
}
```

---

## Testing

### Philosophy

Test the logic, not the UI. Bubble Tea rendering and terminal output are expensive to test and fragile to maintain. Focus testing effort on the parts that can silently go wrong.

### What to Test

| Area | Approach |
|---|---|
| **Diff engine** | Unit tests. Given a before/after file pair, assert correct unified, side-by-side, and full-file output. |
| **Gitignore matching** | Unit tests. Cover nested `.gitignore`, global ignores, negation patterns, and edge cases like `.gitignore` in subdirectories. |
| **Timeline** | Unit tests. Snapshot insertion, eviction at capacity, navigation (step forward/back, jump to commit milestone). |
| **Config merging** | Unit tests. Verify CLI flags override TOML values, defaults apply when neither is set. |
| **Watcher debouncing** | Unit tests with a fake clock. Verify rapid writes to the same file produce a single event. |
| **Git operations** | Integration tests against a temporary git repo created in `t.TempDir()`. Test diff output, HEAD change detection, dirty working tree states. |

### What Not to Test

- Bubble Tea component rendering — trust the framework.
- Chroma syntax highlighting output — trust the library.
- Exact terminal escape sequences or colours.
- Filesystem watcher event delivery (OS-dependent, tested by `fsnotify` itself).

### Running

```bash
go test ./...                  # All tests
go test ./internal/diff/...    # Specific package
go test -race ./...            # Race detector
```

---



### Targets

| OS | Architecture |
|---|---|
| macOS | arm64, amd64 |
| Linux | arm64, amd64 |
| Windows | amd64 |

### Distribution

- **GitHub Releases**: Pre-built binaries for all targets.
- **Homebrew**: `brew install hark` (macOS/Linux).
- **Go install**: `go install github.com/<org>/hark@latest`.
- **AUR**: Community package for Arch Linux (stretch).
- **Scoop/Winget**: Windows package managers (stretch).

### Build

```bash
go build -o hark ./cmd/hark
```

Cross-compilation via `GOOS` / `GOARCH` or GoReleaser for automated releases.

---

## Project Structure

```
hark/
├── cmd/
│   └── hark/
│       └── main.go              # Entry point, CLI flag parsing
├── internal/
│   ├── app/
│   │   └── app.go               # Bubble Tea main model, update loop
│   ├── watcher/
│   │   ├── fs.go                 # Filesystem watcher (fsnotify)
│   │   ├── poll.go               # Git polling fallback
│   │   └── watcher.go            # Unified watcher interface
│   ├── diff/
│   │   ├── engine.go             # Diff computation (unified, side-by-side, full)
│   │   └── highlight.go          # Syntax highlighting via Chroma
│   ├── timeline/
│   │   ├── timeline.go           # Snapshot storage, navigation, eviction
│   │   └── snapshot.go           # Snapshot data model
│   ├── ui/
│   │   ├── diffview.go           # Diff display component
│   │   ├── filelist.go           # File list overlay component
│   │   ├── statusbar.go          # Top status bar
│   │   ├── timelinebar.go        # Bottom timeline bar
│   │   ├── help.go               # Help overlay
│   │   └── theme.go              # Theme management
│   ├── config/
│   │   └── config.go             # TOML config + CLI flag merging
│   └── git/
│       └── repo.go               # Git operations (diff, status, HEAD tracking)
├── config.example.toml
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## v1 Milestone Summary

The initial release includes:

1. Real-time filesystem watching with git polling fallback.
2. Three toggleable diff display styles with syntax highlighting.
3. Single-pane layout with togglable file list overlay.
4. Auto-focus on the most recently changed file with unseen-change badges.
5. Timeline with pause, step-back/forward, and commit milestones.
6. Vi-style keybindings and help overlay.
7. TOML config file with CLI flag overrides.
8. `.gitignore` respect and custom ignore patterns.
9. Open-in-editor escape hatch.
10. Cross-platform binaries (macOS, Linux, Windows).

