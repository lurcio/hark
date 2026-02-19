# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Hark is a real-time, read-only TUI for watching changes to a git repository as they happen. It monitors a git working directory for file changes and displays diffs with syntax highlighting, a navigable timeline, and a terminal interface. It is strictly read-only — no git write operations.

## Status

Active development. SPEC.md contains the full specification.

## Build & Test Commands

```bash
go build -o hark ./cmd/hark    # Build binary
go test ./...                          # Run all tests
go test ./internal/diff/...            # Run tests for a specific package
go test -race ./...                    # Run with race detector
go test -run TestName ./internal/diff/ # Run a single test
golangci-lint run ./...                # Lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
```

## Tech Stack

- **Language**: Go
- **TUI**: Bubble Tea + Lipgloss (charmbracelet)
- **Syntax highlighting**: Chroma
- **Git**: go-git (pure Go, no system git dependency)
- **File watching**: fsnotify

## Architecture

Package structure:

- `cmd/hark/main.go` — entry point, CLI flag parsing
- `internal/app/` — Bubble Tea main model and update loop
- `internal/watcher/` — file change detection (fsnotify primary, git polling fallback), debouncing
- `internal/diff/` — diff computation (unified, side-by-side, full-file) and Chroma syntax highlighting
- `internal/timeline/` — snapshot storage, navigation, eviction (in-memory, max 1000 default)
- `internal/ui/` — TUI components (diff view, file list overlay, status bar, timeline bar, help, themes)
- `internal/config/` — TOML config (`~/.config/hark/config.toml`) merged with CLI flags
- `internal/git/` — git operations via go-git (diff, status, HEAD tracking)

## CI

GitHub Actions runs on pushes and PRs to `main` (Ubuntu + macOS). See `.github/workflows/ci.yml`.

**golangci-lint caveat:** The official `golangci/golangci-lint-action` bundles a pre-built binary compiled with an older Go version. Since this project uses Go 1.25, the action fails. CI instead installs golangci-lint from source via `go install` so it's compiled with the project's Go toolchain.

## Key Design Decisions

- **Snapshot model** should include an optional `Source *AgentEvent` field from day one (nil for v1) to support future coding agent integration without data model changes.
- **Testing philosophy**: test logic, not UI. Focus on diff engine, gitignore matching, timeline, config merging, and debouncing. Do not test Bubble Tea rendering, Chroma output, or terminal escape sequences.
- Watches unstaged working directory changes only (working tree vs HEAD/index).
- Respects `.gitignore` at all levels including global `~/.config/git/ignore`.
- Config file is TOML; CLI flags override config values.
