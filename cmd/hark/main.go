// Package main is the entry point for hark, a real-time read-only TUI
// for watching changes to a git repository.
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	flag "github.com/spf13/pflag"

	"github.com/lurcio/hark/internal/app"
	"github.com/lurcio/hark/internal/config"
	"github.com/lurcio/hark/internal/git"
	"github.com/lurcio/hark/internal/timeline"
	"github.com/lurcio/hark/internal/watcher"
)

var version = "dev"

func main() {
	// Define flags with pflag (GNU-style --double-dash)
	pollOnly := flag.Bool("poll-only", false, "Use git polling only (no filesystem watcher)")
	pollInterval := flag.Duration("poll-interval", 0, "Polling interval (default: from config or 1s)")
	theme := flag.String("theme", "", "Colour theme (default: from config or dracula)")
	diffStyle := flag.String("diff-style", "", "Diff display style: unified, side-by-side, full-file")
	contextLines := flag.Int("context", 0, "Lines of context around changes (default: from config or 3)")
	ignore := flag.StringArray("ignore", nil, "Additional ignore patterns (can be repeated)")
	maxSnapshots := flag.Int("max-snapshots", 0, "Max timeline snapshots (default: from config or 1000)")
	showVersion := flag.BoolP("version", "v", false, "Show version")
	debugLog := flag.String("debug-log", "", "Path to debug log file (for development)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  hark [path] [flags]\n\nArguments:\n  path    Path to git repository (default: current directory)\n\nFlags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *showVersion {
		fmt.Printf("hark %s\n", version)
		os.Exit(0)
	}

	// Positional argument: repo path
	repoPath := "."
	if flag.NArg() > 0 {
		repoPath = flag.Arg(0)
	}

	// Load config file
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Build CLI flags for merge — only override when explicitly set
	cliFlags := config.CLIFlags{}
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "poll-only":
			cliFlags.PollOnly = pollOnly
		case "poll-interval":
			cliFlags.PollInterval = pollInterval
		case "theme":
			cliFlags.Theme = theme
		case "diff-style":
			cliFlags.DiffStyle = diffStyle
		case "context":
			cliFlags.ContextLines = contextLines
		case "max-snapshots":
			cliFlags.MaxSnapshots = maxSnapshots
		case "ignore":
			cliFlags.Ignore = *ignore
		}
	})
	cfg = cfg.Merge(cliFlags)

	// Open the git repo
	repo, err := git.Open(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Set up debug logger
	dbg := app.NewDebugLogger(*debugLog)
	defer dbg.Close()

	// Create watcher
	var logFn watcher.LogFunc
	if dbg.Enabled() {
		logFn = dbg.Log
	}
	w, err := watcher.New(cfg, repo, logFn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating watcher: %v\n", err)
		os.Exit(1)
	}
	defer w.Close()

	// Create timeline
	tl := timeline.New(cfg.Timeline.MaxSnapshots)

	// Create app model
	model, err := app.New(cfg, repo, w, tl, dbg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Run the TUI
	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

