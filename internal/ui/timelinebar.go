package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TimelineBar renders the bottom timeline bar with transport controls,
// scrubber, position, and timestamp.
type TimelineBar struct {
	Width     int
	Position  int       // 0-based current snapshot index
	Total     int       // total number of snapshots
	Paused    bool      // whether timeline is paused
	Timestamp time.Time // timestamp of current snapshot
	Theme     ChromaPalette
}

// NewTimelineBar creates a TimelineBar with default values.
func NewTimelineBar(theme ChromaPalette) TimelineBar {
	return TimelineBar{Theme: theme}
}

// View renders the timeline bar as a string.
func (t TimelineBar) View() string {
	bg := t.Theme.TimelineBg
	fg := t.Theme.TimelineFg
	accent := t.Theme.AccentColor

	base := lipgloss.NewStyle().
		Background(bg).
		Foreground(fg)

	accentStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(accent)

	// Transport controls
	transport := " ◀ "
	if t.Paused {
		transport += "▶"
	} else {
		transport += "■"
	}
	transport += " ▶ "

	// Position display (1-based for humans)
	posStr := ""
	if t.Total > 0 {
		posStr = fmt.Sprintf(" %d/%d ", t.Position+1, t.Total)
	} else {
		posStr = " 0/0 "
	}

	// Timestamp
	tsStr := ""
	if !t.Timestamp.IsZero() {
		tsStr = t.Timestamp.Format("15:04:05")
	}

	left := accentStyle.Render(transport)
	right := base.Render(posStr+"  "+tsStr) + base.Render(" ")

	// Scrubber fills the remaining space
	scrubberWidth := t.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if scrubberWidth < 3 {
		scrubberWidth = 3
	}

	scrubber := t.renderScrubber(scrubberWidth, bg, fg, accent)

	return left + scrubber + right
}

func (t TimelineBar) renderScrubber(width int, bg, fg, accent lipgloss.Color) string {
	base := lipgloss.NewStyle().Background(bg).Foreground(fg)
	accentStyle := lipgloss.NewStyle().Background(bg).Foreground(accent)

	if t.Total <= 0 || width < 3 {
		return base.Render(repeat("─", width))
	}

	// Calculate indicator position
	innerWidth := width - 2 // space for padding
	if innerWidth < 1 {
		innerWidth = 1
	}
	pos := 0
	if t.Total > 1 {
		pos = t.Position * (innerWidth - 1) / (t.Total - 1)
	}
	if pos < 0 {
		pos = 0
	}
	if pos >= innerWidth {
		pos = innerWidth - 1
	}

	before := repeat("─", pos)
	after := repeat("─", innerWidth-pos-1)

	return base.Render(" "+before) + accentStyle.Render("●") + base.Render(after+" ")
}
