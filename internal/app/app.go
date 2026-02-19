// Package app implements the main Bubble Tea model that composes all UI
// components, wires up the watcher, timeline, and diff engine, and handles
// all keybindings.
package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gogit "github.com/go-git/go-git/v5"

	"github.com/lurcio/hark/internal/config"
	"github.com/lurcio/hark/internal/diff"
	"github.com/lurcio/hark/internal/git"
	"github.com/lurcio/hark/internal/timeline"
	"github.com/lurcio/hark/internal/ui"
	"github.com/lurcio/hark/internal/watcher"
)

// Custom messages for the Bubble Tea update loop.
type (
	watcherEventMsg  watcher.Event
	headCheckTickMsg time.Time
	refreshTickMsg   time.Time
	initialScanMsg   struct{}
	editorFinishedMsg struct{ err error }
)

// Model is the main Bubble Tea model that composes all UI components.
type Model struct {
	cfg      config.Config
	repo     *git.Repo
	gogit    *gogit.Repository
	watcher  watcher.Watcher
	timeline *timeline.Timeline

	// UI components
	diffView    ui.DiffView
	fileList    ui.FileList
	statusBar   ui.StatusBar
	timelineBar ui.TimelineBar
	help        ui.HelpOverlay

	// Display settings
	themeName    string
	palette      ui.ChromaPalette
	diffStyle    diff.DiffStyle
	contextLines int
	wordDiff     bool

	// File state
	changedFiles []git.ChangedFile
	currentFile  int
	unseenFiles  map[string]bool
	needsRefresh bool
	pendingEvent *watcher.Event

	// Window dimensions
	width  int
	height int

	// Search state
	searchMode  bool
	searchQuery string

	// Debug
	debug       *DebugLogger
	watcherEvts int // count of watcher events received

	quitting bool
}

// New creates a new Model wired up with all dependencies.
func New(cfg config.Config, repo *git.Repo, w watcher.Watcher, tl *timeline.Timeline, dbg *DebugLogger) (*Model, error) {
	gogitRepo, err := gogit.PlainOpenWithOptions(repo.Path(), &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
	if err != nil {
		return nil, fmt.Errorf("app: opening git repo: %w", err)
	}

	palette := ui.ThemeByName(cfg.Display.Theme)

	var ds diff.DiffStyle
	switch cfg.Display.DiffStyle {
	case "side-by-side":
		ds = diff.SideBySide
	case "full-file":
		ds = diff.FullFile
	default:
		ds = diff.Unified
	}

	m := &Model{
		cfg:          cfg,
		repo:         repo,
		gogit:        gogitRepo,
		watcher:      w,
		timeline:     tl,
		diffView:     ui.NewDiffView(palette),
		fileList:     ui.NewFileList(palette),
		statusBar:    ui.NewStatusBar(palette),
		timelineBar:  ui.NewTimelineBar(palette),
		help:         ui.NewHelpOverlay(palette),
		themeName:    cfg.Display.Theme,
		palette:      palette,
		diffStyle:    ds,
		contextLines: cfg.Display.ContextLines,
		wordDiff:     cfg.Display.WordDiff,
		unseenFiles:  make(map[string]bool),
		currentFile:  -1,
		debug:        dbg,
	}

	m.diffView.ShowLineNumbers = cfg.Display.ShowLineNumbers

	dbg.Log("app initialized: repo=%s, poll_only=%v, poll_interval=%v, debounce=%v",
		repo.Path(), cfg.Watch.PollOnly, cfg.Watch.PollInterval.Duration, cfg.Watch.Debounce.Duration)

	m.statusBar.WatchDir = repo.Path()
	m.statusBar.State = ui.StateWatching

	return m, nil
}

// Init starts listening for watcher events and HEAD changes.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		m.waitForWatcherEvent(),
		m.headCheckTick(),
		m.refreshTick(),
		func() tea.Msg { return initialScanMsg{} },
	)
}

// Update handles all incoming messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case watcherEventMsg:
		m.handleFileChange(watcher.Event(msg))
		return m, m.waitForWatcherEvent()

	case headCheckTickMsg:
		m.checkHEAD()
		return m, m.headCheckTick()

	case refreshTickMsg:
		m.refreshFiles()
		return m, m.refreshTick()

	case initialScanMsg:
		m.doInitialScan()
		return m, nil

	case editorFinishedMsg:
		return m, nil
	}

	return m, nil
}

// View renders the full UI.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if m.width == 0 || m.height == 0 {
		return "Starting hark..."
	}

	// Help overlay covers everything
	if m.help.Visible {
		return m.help.View()
	}

	statusBar := m.statusBar.View()

	var bottomBar string
	if m.searchMode {
		bottomBar = m.renderSearchBar()
	} else {
		bottomBar = m.timelineBar.View()
	}

	contentHeight := m.height - 2 // status bar + bottom bar
	if contentHeight < 0 {
		contentHeight = 0
	}

	// Middle section: diff view, optionally with file list overlay
	var middle string
	if m.fileList.Visible {
		fileListView := m.fileList.View()
		middle = lipgloss.JoinHorizontal(lipgloss.Top, fileListView, m.diffView.View())
	} else {
		middle = m.diffView.View()
	}

	// Force middle to exact height so the timeline bar stays at the bottom.
	middle = lipgloss.NewStyle().Height(contentHeight).Render(middle)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, middle, bottomBar)
}

// --- Commands ---

func (m *Model) waitForWatcherEvent() tea.Cmd {
	ch := m.watcher.Events()
	return func() tea.Msg {
		evt, ok := <-ch
		if !ok {
			return nil
		}
		return watcherEventMsg(evt)
	}
}

func (m *Model) headCheckTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return headCheckTickMsg(t)
	})
}

func (m *Model) refreshTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

// --- Key handling ---

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When help is visible, only allow closing it
	if m.help.Visible {
		switch msg.String() {
		case "?", "q", "esc":
			m.help.Toggle()
		}
		return m, nil
	}

	// Search mode: route keys to search input
	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	switch msg.String() {
	// Quit
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	// Help
	case "?":
		m.help.Toggle()

	// Search
	case "/":
		m.searchMode = true
		m.searchQuery = ""
		m.diffView.SetSearchQuery("")

	// Navigation
	case "j", "down":
		m.diffView.ScrollDown(1)
	case "k", "up":
		m.diffView.ScrollUp(1)
	case "d", "pgdown":
		m.diffView.ScrollDown(m.diffView.Height / 2)
	case "u", "pgup":
		m.diffView.ScrollUp(m.diffView.Height / 2)
	case "g":
		m.diffView.ScrollToTop()
	case "G":
		m.diffView.ScrollToBottom()
	case "n":
		if m.diffView.SearchQuery != "" {
			m.diffView.JumpToNextMatch()
		} else {
			m.diffView.JumpToNextHunk()
		}
	case "N":
		if m.diffView.SearchQuery != "" {
			m.diffView.JumpToPrevMatch()
		} else {
			m.diffView.JumpToPrevHunk()
		}

	// File management
	case "tab":
		m.fileList.Toggle()
		m.layout()
	case "]":
		m.nextFile()
	case "[":
		m.prevFile()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0] - '1')
		m.selectFile(idx)

	// Timeline
	case " ":
		m.togglePause()
	case "h", "left":
		m.stepBack()
	case "l", "right":
		m.stepForward()
	case "H":
		if m.timeline.JumpToPrevCommit() {
			m.syncFromTimeline()
		}
	case "L":
		if m.timeline.JumpToNextCommit() {
			m.syncFromTimeline()
		}

	// Display
	case "v":
		m.cycleDiffStyle()
	case "t":
		m.cycleTheme()
	case "w":
		m.wordDiff = !m.wordDiff
		m.recomputeCurrentDiff()
	case "+", "=":
		m.contextLines++
		m.recomputeCurrentDiff()
	case "-":
		if m.contextLines > 0 {
			m.contextLines--
			m.recomputeCurrentDiff()
		}

	// Editor
	case "e":
		return m, m.openInEditor()
	}

	return m, nil
}

// handleSearchKey processes key events while in search mode.
func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Exit search mode, keep highlights
		m.searchMode = false
	case "esc":
		// Exit search mode and clear search
		m.searchMode = false
		m.searchQuery = ""
		m.diffView.SetSearchQuery("")
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.diffView.SetSearchQuery(m.searchQuery)
		}
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	default:
		// Only append printable characters (single runes)
		key := msg.String()
		if len(key) == 1 && key[0] >= ' ' && key[0] <= '~' {
			m.searchQuery += key
			m.diffView.SetSearchQuery(m.searchQuery)
		}
	}
	return m, nil
}

// --- File change handling ---

func (m *Model) handleFileChange(evt watcher.Event) {
	m.watcherEvts++
	m.debug.Log("watcher event #%d: path=%q kind=%v", m.watcherEvts, evt.Path, evt.Kind)
	m.pendingEvent = &evt
	m.needsRefresh = true
}

// refreshFiles periodically re-scans changed files from git.
// This avoids calling the expensive ChangedFiles() on every single watcher event.
func (m *Model) refreshFiles() {
	if !m.needsRefresh {
		return
	}
	m.needsRefresh = false

	lastEvt := m.pendingEvent
	m.pendingEvent = nil

	start := time.Now()
	files, err := m.repo.ChangedFiles()
	elapsed := time.Since(start)
	if err != nil {
		m.debug.Log("refreshFiles: ChangedFiles error after %v: %v", elapsed, err)
		return
	}
	m.debug.Log("refreshFiles: got %d files in %v (evt=%v)", len(files), elapsed, lastEvt != nil)

	m.changedFiles = files

	// If there was a watcher event, add a snapshot and auto-focus
	if lastEvt != nil {
		changedIdx := -1
		for i, f := range files {
			if f.Path == lastEvt.Path {
				changedIdx = i
				m.timeline.Add(timeline.Snapshot{
					Timestamp: time.Now(),
					FilePath:  lastEvt.Path,
					Diff:      f.Diff,
					Type:      timeline.FileChange,
				})
				break
			}
		}

		// Auto-focus if watching live
		if !m.timeline.IsPaused() && changedIdx >= 0 {
			for _, f := range files {
				if f.Path != lastEvt.Path {
					m.unseenFiles[f.Path] = true
				}
			}
			m.unseenFiles[lastEvt.Path] = false
			m.currentFile = changedIdx
		} else if changedIdx >= 0 {
			m.unseenFiles[lastEvt.Path] = true
		}
	}

	m.rebuildFileList()

	if !m.timeline.IsPaused() {
		m.computeCurrentFileDiff()
	}

	m.updateStatusBar()
	m.updateTimelineBar()
}

func (m *Model) checkHEAD() {
	changed, message, err := m.repo.HeadChanged()
	if err != nil {
		m.debug.Log("checkHEAD: error: %v", err)
		return
	}
	if !changed {
		return
	}
	m.debug.Log("checkHEAD: HEAD changed, commit=%q", message)

	// Insert commit milestone into timeline
	m.timeline.Add(timeline.Snapshot{
		Timestamp: time.Now(),
		FilePath:  "",
		Diff:      message,
		Type:      timeline.Commit,
	})

	// A commit means a fresh slate — resume live watching and reset rewind.
	if m.timeline.IsPaused() {
		m.timeline.Resume()
	}
	m.statusBar.State = ui.StateWatching

	// After a commit, working tree changes reset — refresh
	files, err := m.repo.ChangedFiles()
	if err != nil {
		files = nil
	}
	m.changedFiles = files
	m.unseenFiles = make(map[string]bool)

	if len(files) > 0 {
		m.currentFile = 0
	} else {
		m.currentFile = -1
	}

	m.rebuildFileList()
	m.computeCurrentFileDiff()
	m.updateStatusBar()
	m.updateTimelineBar()
}

func (m *Model) doInitialScan() {
	m.debug.Log("doInitialScan: starting")
	start := time.Now()
	files, err := m.repo.ChangedFiles()
	elapsed := time.Since(start)
	if err != nil {
		m.debug.Log("doInitialScan: error after %v: %v", elapsed, err)
		return
	}
	m.debug.Log("doInitialScan: got %d files in %v", len(files), elapsed)
	if len(files) == 0 {
		return
	}
	for i, f := range files {
		m.debug.Log("  file[%d]: %s status=%v binary=%v diff_len=%d", i, f.Path, f.Status, f.Binary, len(f.Diff))
	}
	m.changedFiles = files
	m.currentFile = 0
	m.rebuildFileList()
	m.computeCurrentFileDiff()
	m.updateStatusBar()
	m.updateTimelineBar()
}

// --- File navigation ---

func (m *Model) nextFile() {
	if len(m.changedFiles) == 0 {
		return
	}
	m.currentFile++
	if m.currentFile >= len(m.changedFiles) {
		m.currentFile = len(m.changedFiles) - 1
	}
	m.selectFile(m.currentFile)
}

func (m *Model) prevFile() {
	if len(m.changedFiles) == 0 {
		return
	}
	m.currentFile--
	if m.currentFile < 0 {
		m.currentFile = 0
	}
	m.selectFile(m.currentFile)
}

func (m *Model) selectFile(idx int) {
	if idx < 0 || idx >= len(m.changedFiles) {
		return
	}
	m.currentFile = idx
	m.fileList.Selected = idx

	// Clear unseen badge for the selected file
	m.unseenFiles[m.changedFiles[idx].Path] = false
	m.rebuildFileList()
	m.computeCurrentFileDiff()
}

// --- Diff computation ---

func (m *Model) computeCurrentFileDiff() {
	if m.currentFile < 0 || m.currentFile >= len(m.changedFiles) {
		m.diffView.SetContent(diff.DiffResult{}, "")
		return
	}

	file := m.changedFiles[m.currentFile]
	if file.Binary {
		m.diffView.SetContent(diff.DiffResult{
			Lines:      []diff.DiffLine{{Type: diff.Context, Content: "Binary file changed"}},
			Style:      diff.Unified,
			HasChanges: true,
		}, file.Path)
		return
	}

	oldContent := m.readFromHEAD(file.Path)
	newContent := m.readFromWorkdir(file.Path)

	style := m.diffStyle
	// Fall back to unified when terminal is too narrow for side-by-side
	if style == diff.SideBySide && m.width < 120 {
		style = diff.Unified
	}

	opts := diff.Options{
		ContextLines: m.contextLines,
		Style:        style,
		FileName:     file.Path,
		WordDiff:     m.wordDiff,
	}

	result := diff.Compute(oldContent, newContent, opts)
	m.debug.Log("computeDiff: file=%s old_len=%d new_len=%d lines=%d has_changes=%v",
		file.Path, len(oldContent), len(newContent), len(result.Lines), result.HasChanges)
	m.diffView.SetContent(result, file.Path)
}

func (m *Model) recomputeCurrentDiff() {
	if m.timeline.IsPaused() && !m.timeline.IsAtLatest() {
		// When viewing historical snapshot, re-parse it
		m.syncFromTimeline()
		return
	}
	m.computeCurrentFileDiff()
}

func (m *Model) readFromHEAD(filePath string) string {
	head, err := m.gogit.Head()
	if err != nil {
		return ""
	}
	commit, err := m.gogit.CommitObject(head.Hash())
	if err != nil {
		return ""
	}
	tree, err := commit.Tree()
	if err != nil {
		return ""
	}
	f, err := tree.File(filePath)
	if err != nil {
		return ""
	}
	content, err := f.Contents()
	if err != nil {
		return ""
	}
	return content
}

func (m *Model) readFromWorkdir(filePath string) string {
	data, err := os.ReadFile(filepath.Join(m.repo.Path(), filePath))
	if err != nil {
		return ""
	}
	return string(data)
}

// --- Timeline controls ---

func (m *Model) togglePause() {
	if m.timeline.IsPaused() {
		m.timeline.Resume()
		m.statusBar.State = ui.StateWatching
		// Catch up to latest state
		m.computeCurrentFileDiff()
	} else {
		m.timeline.Pause()
		m.statusBar.State = ui.StatePaused
	}
	m.updateTimelineBar()
}

func (m *Model) stepBack() {
	if !m.timeline.IsPaused() {
		m.timeline.Pause()
		m.statusBar.State = ui.StatePaused
	}
	if m.timeline.StepBack() {
		m.syncFromTimeline()
	}
	if !m.timeline.IsAtLatest() {
		m.statusBar.State = ui.StateRewinding
	}
	m.updateTimelineBar()
}

func (m *Model) stepForward() {
	if m.timeline.StepForward() {
		if m.timeline.IsAtLatest() {
			m.timeline.Resume()
			m.statusBar.State = ui.StateWatching
			m.computeCurrentFileDiff()
		} else {
			m.syncFromTimeline()
		}
	}
	m.updateTimelineBar()
}

func (m *Model) syncFromTimeline() {
	snap := m.timeline.Current()
	if snap == nil {
		return
	}

	if snap.Type == timeline.Commit {
		// Show commit message
		m.diffView.SetContent(diff.DiffResult{
			Lines: []diff.DiffLine{
				{Type: diff.Header, Content: "Commit"},
				{Type: diff.Context, Content: ""},
				{Type: diff.Context, Content: snap.Diff},
			},
			Style:      diff.Unified,
			HasChanges: true,
		}, "commit milestone")
	} else {
		// Show the stored diff from the snapshot
		result := parseUnifiedDiff(snap.Diff)
		m.diffView.SetContent(result, snap.FilePath)
	}

	m.updateTimelineBar()
}

// --- Display controls ---

func (m *Model) cycleDiffStyle() {
	switch m.diffStyle {
	case diff.Unified:
		m.diffStyle = diff.SideBySide
	case diff.SideBySide:
		m.diffStyle = diff.FullFile
	default:
		m.diffStyle = diff.Unified
	}
	m.recomputeCurrentDiff()
	m.persistDisplaySettings()
}

func (m *Model) cycleTheme() {
	m.themeName = ui.CycleTheme(m.themeName)
	m.palette = ui.ThemeByName(m.themeName)
	m.applyTheme()
	m.persistDisplaySettings()
}

func (m *Model) persistDisplaySettings() {
	// Update the in-memory config with current display state
	switch m.diffStyle {
	case diff.SideBySide:
		m.cfg.Display.DiffStyle = "side-by-side"
	case diff.FullFile:
		m.cfg.Display.DiffStyle = "full-file"
	default:
		m.cfg.Display.DiffStyle = "unified"
	}
	m.cfg.Display.Theme = m.themeName
	m.cfg.SaveDisplaySettingsAsync()
}

func (m *Model) applyTheme() {
	m.diffView.Theme = m.palette
	m.fileList.Theme = m.palette
	m.statusBar.Theme = m.palette
	m.timelineBar.Theme = m.palette
	m.help.Theme = m.palette
	m.recomputeCurrentDiff()
}

// --- Editor integration ---

func (m *Model) openInEditor() tea.Cmd {
	if m.currentFile < 0 || m.currentFile >= len(m.changedFiles) {
		return nil
	}

	filePath := filepath.Join(m.repo.Path(), m.changedFiles[m.currentFile].Path)
	lineNum := m.diffView.CurrentLineNumber()

	editor := m.cfg.Editor.Command
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}

	parts := strings.Fields(editor)
	base := filepath.Base(parts[0])

	// Build args with line number based on editor convention
	var args []string
	switch base {
	case "code", "code-insiders":
		args = append(parts[1:], "--goto", fmt.Sprintf("%s:%d", filePath, lineNum))
	default:
		// vi, vim, nvim, nano, emacs, and most editors support +line
		args = append(parts[1:], fmt.Sprintf("+%d", lineNum), filePath)
	}

	c := exec.Command(parts[0], args...)

	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

// --- Search bar rendering ---

func (m *Model) renderSearchBar() string {
	bg := m.palette.TimelineBg
	fg := m.palette.TimelineFg
	accent := m.palette.AccentColor

	promptStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(accent).
		Bold(true)

	inputStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg)

	matchInfo := ""
	if m.searchQuery != "" {
		count := len(m.diffView.SearchMatchLines)
		if count == 0 {
			matchInfo = " (no matches)"
		} else {
			matchInfo = fmt.Sprintf(" (%d matches)", count)
		}
	}

	content := promptStyle.Render("/") + inputStyle.Render(m.searchQuery+"_"+matchInfo)

	// Pad to full width
	gap := m.width - lipgloss.Width(content)
	if gap > 0 {
		content += lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
	}

	return content
}

// --- Layout ---

func (m *Model) layout() {
	m.statusBar.Width = m.width
	m.timelineBar.Width = m.width
	m.help.Width = m.width
	m.help.Height = m.height

	contentHeight := m.height - 2 // status bar + timeline bar
	if contentHeight < 0 {
		contentHeight = 0
	}

	diffWidth := m.width
	if m.fileList.Visible {
		diffWidth = m.width - m.fileList.Width
		if diffWidth < 10 {
			diffWidth = 10
		}
	}

	m.diffView.SetSize(diffWidth, contentHeight)
	m.fileList.Height = contentHeight
}

// --- Status and timeline bar updates ---

func (m *Model) updateStatusBar() {
	m.statusBar.ChangedFiles = len(m.changedFiles)
}

func (m *Model) updateTimelineBar() {
	pos, total := m.timeline.Position()
	m.timelineBar.Position = pos
	m.timelineBar.Total = total
	m.timelineBar.Paused = m.timeline.IsPaused()

	snap := m.timeline.Current()
	if snap != nil {
		m.timelineBar.Timestamp = snap.Timestamp
	}
}

func (m *Model) rebuildFileList() {
	entries := make([]ui.FileEntry, len(m.changedFiles))
	for i, f := range m.changedFiles {
		entries[i] = ui.FileEntry{
			Path:   f.Path,
			Unseen: m.unseenFiles[f.Path],
		}
	}
	m.fileList.SetFiles(entries)
	if m.currentFile >= 0 && m.currentFile < len(m.changedFiles) {
		m.fileList.Selected = m.currentFile
	}
}

// parseUnifiedDiff converts a raw unified diff string into a DiffResult.
// Used when viewing historical snapshots from the timeline.
func parseUnifiedDiff(rawDiff string) diff.DiffResult {
	lines := strings.Split(rawDiff, "\n")
	var result []diff.DiffLine
	hasChanges := false
	oldLine, newLine := 0, 0

	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			_, _ = fmt.Sscanf(line, "@@ -%d,%*d +%d,%*d @@", &oldLine, &newLine)
			result = append(result, diff.DiffLine{
				Type:    diff.Header,
				Content: line,
			})
			continue
		}
		if len(line) == 0 {
			continue
		}
		switch line[0] {
		case '-':
			hasChanges = true
			result = append(result, diff.DiffLine{
				Type:       diff.Removed,
				Content:    line[1:],
				OldLineNum: oldLine,
			})
			oldLine++
		case '+':
			hasChanges = true
			result = append(result, diff.DiffLine{
				Type:       diff.Added,
				Content:    line[1:],
				NewLineNum: newLine,
			})
			newLine++
		case ' ':
			result = append(result, diff.DiffLine{
				Type:       diff.Context,
				Content:    line[1:],
				OldLineNum: oldLine,
				NewLineNum: newLine,
			})
			oldLine++
			newLine++
		}
	}

	return diff.DiffResult{
		Lines:      result,
		Style:      diff.Unified,
		HasChanges: hasChanges,
	}
}
