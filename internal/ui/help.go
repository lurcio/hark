package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HelpOverlay renders the full-screen help overlay showing all keybindings.
type HelpOverlay struct {
	Visible bool
	Width   int
	Height  int
	Theme   ChromaPalette
}

// NewHelpOverlay creates a HelpOverlay with default values.
func NewHelpOverlay(theme ChromaPalette) HelpOverlay {
	return HelpOverlay{Theme: theme}
}

// Toggle flips the help overlay visibility.
func (h *HelpOverlay) Toggle() {
	h.Visible = !h.Visible
}

type helpSection struct {
	title    string
	bindings []helpBinding
}

type helpBinding struct {
	key  string
	desc string
}

var helpSections = []helpSection{
	{
		title: "Navigation",
		bindings: []helpBinding{
			{"j / ↓", "Scroll down one line"},
			{"k / ↑", "Scroll up one line"},
			{"d / PgDn", "Scroll down half page"},
			{"u / PgUp", "Scroll up half page"},
			{"g", "Jump to top of diff"},
			{"G", "Jump to bottom of diff"},
			{"n", "Jump to next changed hunk"},
			{"N", "Jump to previous changed hunk"},
		},
	},
	{
		title: "File Management",
		bindings: []helpBinding{
			{"Tab", "Toggle file list overlay"},
			{"]", "Next changed file"},
			{"[", "Previous changed file"},
			{"1-9", "Jump to file by index"},
		},
	},
	{
		title: "Timeline",
		bindings: []helpBinding{
			{"Space", "Pause / resume"},
			{"h / ←", "Step back one snapshot"},
			{"l / →", "Step forward one snapshot"},
			{"H", "Jump to previous commit"},
			{"L", "Jump to next commit"},
		},
	},
	{
		title: "Display",
		bindings: []helpBinding{
			{"v", "Cycle diff style"},
			{"t", "Cycle colour theme"},
			{"w", "Toggle word-level diff"},
			{"+ / -", "Adjust context lines"},
		},
	},
	{
		title: "General",
		bindings: []helpBinding{
			{"?", "Toggle this help"},
			{"e", "Open file in $EDITOR"},
			{"/", "Search within diff"},
			{"q", "Quit"},
		},
	},
}

// View renders the help overlay as a string.
func (h HelpOverlay) View() string {
	if !h.Visible {
		return ""
	}

	bg := h.Theme.HelpBg
	fg := h.Theme.HelpFg
	headerFg := h.Theme.HelpHeaderFg

	titleStyle := lipgloss.NewStyle().
		Foreground(headerFg).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(headerFg).
		Width(12)

	descStyle := lipgloss.NewStyle().
		Foreground(fg)

	var sections []string

	// Title
	sections = append(sections, titleStyle.Render("  hark — Keybindings"))
	sections = append(sections, "")

	for _, sec := range helpSections {
		sections = append(sections, titleStyle.Render("  "+sec.title))
		for _, b := range sec.bindings {
			line := "    " + keyStyle.Render(b.key) + descStyle.Render(b.desc)
			sections = append(sections, line)
		}
		sections = append(sections, "")
	}

	sections = append(sections, descStyle.Render("  Press ? to close"))

	content := strings.Join(sections, "\n")

	overlay := lipgloss.NewStyle().
		Width(h.Width).
		Height(h.Height).
		Background(bg).
		Foreground(fg)

	return overlay.Render(content)
}
