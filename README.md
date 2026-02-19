# hark

A real-time, read-only TUI for watching changes to a git repository as they happen.

Built for developers who want to follow along as coding agents (or teammates) make changes. Like spectator mode for your codebase. Hark watches everything, understands the changes deeply, and never modifies a thing.

![Go](https://img.shields.io/github/go-mod/go-version/lurcio/hark)
![CI](https://github.com/lurcio/hark/actions/workflows/ci.yml/badge.svg)
![License](https://img.shields.io/github/license/lurcio/hark)

## Install

With Go installed:

```bash
go install github.com/lurcio/hark/cmd/hark@latest
```

Or grab a binary from [Releases](https://github.com/lurcio/hark/releases).

## Quick start

```bash
# Watch the current directory
hark

# Watch a specific repo
hark ~/projects/myapp
```

That's it. Hark picks up file changes in real time and shows you syntax-highlighted diffs as they happen. It respects your `.gitignore` so you won't be flooded with noise from build artifacts or dependencies.

## What you see

```
┌─────────────────────────────────────────────────────┐
│ hark  ~/projects/myapp         ▶ watching   3 files │
├─────────────────────────────────────────────────────┤
│                                                     │
│  src/api/handler.go                                 │
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
│ ◀ ■ ▶  ────●──────────────── 14/27    12:04:32      │
└─────────────────────────────────────────────────────┘
```

The main view is a full-screen diff viewer. Press `Tab` to toggle a file list overlay on the left. Files with unseen changes get a `●` badge.

## Features

- **Real-time diffs** with syntax highlighting via [Chroma](https://github.com/alecthomas/chroma) (200+ languages)
- **Three diff styles**: unified, side-by-side, and full-file (toggle with `v`)
- **Timeline**: every file change is recorded as a snapshot. Pause, rewind, step through history, jump between commits
- **10 colour themes** including Dracula, Catppuccin, Nord, Solarized, and more (cycle with `t`)
- **Gitignore-aware**: respects `.gitignore` at all levels plus your global ignore file
- **Editor escape hatch**: press `e` to open the current file in `$EDITOR` at the right line
- **Search**: press `/` to search within the current diff
- **No git dependency**: uses [go-git](https://github.com/go-git/go-git) (pure Go), so there's nothing to install beyond the binary

## Keybindings

### Navigation

| Key | Action |
|---|---|
| `j` / `↓` | Scroll down |
| `k` / `↑` | Scroll up |
| `d` / `Page Down` | Half page down |
| `u` / `Page Up` | Half page up |
| `g` / `G` | Jump to top / bottom |
| `n` / `N` | Next / previous hunk |

### Files

| Key | Action |
|---|---|
| `Tab` | Toggle file list |
| `]` / `[` | Next / previous file |
| `1`-`9` | Jump to file by index |

### Timeline

| Key | Action |
|---|---|
| `Space` | Pause / resume |
| `h` / `←` | Step back one snapshot |
| `l` / `→` | Step forward one snapshot |
| `H` / `L` | Jump to previous / next commit |

### Display

| Key | Action |
|---|---|
| `v` | Cycle diff style |
| `t` | Cycle theme |
| `w` | Toggle word-level diffs |
| `+` / `-` | More / fewer context lines |

### Other

| Key | Action |
|---|---|
| `?` | Help |
| `e` | Open in editor |
| `/` | Search |
| `q` | Quit |

## Configuration

Hark reads from `~/.config/hark/config.toml`. See [`config.example.toml`](config.example.toml) for all options with defaults.

CLI flags override config values:

```
hark [path] [flags]

Flags:
    --poll-only            Use git polling only (no filesystem watcher)
    --poll-interval dur    Polling interval (default: 1s)
    --theme string         Colour theme (default: dracula)
    --diff-style string    unified, side-by-side, or full-file
    --context int          Context lines around changes (default: 3)
    --ignore strings       Extra ignore patterns (can be repeated)
    --max-snapshots int    Max timeline snapshots (default: 1000)
-v, --version              Show version
-h, --help                 Show help
```

## How it works

Hark uses [fsnotify](https://github.com/fsnotify/fsnotify) to get OS-level file change events (inotify on Linux, FSEvents on macOS). A git polling fallback runs alongside it to catch anything the filesystem watcher misses, which can happen on network filesystems or in containers.

When a change is detected, hark diffs the working tree against HEAD using go-git. Each change becomes a snapshot in the timeline. Snapshots are kept in memory (default 1000, configurable) and discarded when hark exits.

Hark is strictly read-only. It never stages, commits, or writes to your repo.

## Building from source

```bash
git clone https://github.com/lurcio/hark.git
cd hark
go build -o hark ./cmd/hark
```

Requires Go 1.25+.

## License

[MIT](LICENSE)
