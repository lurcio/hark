package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// WatchState represents the current watching state shown in the status bar.
type WatchState int

const (
	StateWatching  WatchState = iota // ▶ watching
	StatePaused                      // ⏸ paused
	StateRewinding                   // ⏪ rewinding
)

// StatusBar renders the top status bar.
type StatusBar struct {
	Width        int
	WatchDir     string
	State        WatchState
	ChangedFiles int
	Theme        ChromaPalette
}

// NewStatusBar creates a StatusBar with default values.
func NewStatusBar(theme ChromaPalette) StatusBar {
	return StatusBar{Theme: theme}
}

// View renders the status bar as a string.
func (s StatusBar) View() string {
	bg := s.Theme.StatusBarBg
	fg := s.Theme.StatusBarFg
	accent := s.Theme.AccentColor

	base := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg)

	brandStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(accent).
		Bold(true)

	stateStr := stateLabel(s.State)
	filesStr := fmt.Sprintf("%d files", s.ChangedFiles)
	if s.ChangedFiles == 1 {
		filesStr = "1 file"
	}

	left := brandStyle.Render("hark") + base.Render("  "+s.WatchDir)
	right := base.Render(stateStr+"   "+filesStr) + base.Render(" ")

	gap := s.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	middle := base.Render(repeat(" ", gap))

	return left + middle + right
}

func stateLabel(s WatchState) string {
	switch s {
	case StatePaused:
		return "⏸ paused"
	case StateRewinding:
		return "⏪ rewinding"
	default:
		return "▶ watching"
	}
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, len(s)*n)
	for range n {
		b = append(b, s...)
	}
	return string(b)
}
