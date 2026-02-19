package ui

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
)

// FileEntry represents a single file in the file list.
type FileEntry struct {
	Path   string // relative file path
	Unseen bool   // true if the file has unseen changes
}

// FileList renders the toggleable file list overlay panel.
type FileList struct {
	Files    []FileEntry
	Selected int // index of the currently selected file
	Visible  bool
	Width    int // width of the file list panel
	Height   int // available height (excluding status/timeline bars)
	Theme    ChromaPalette
}

// NewFileList creates a FileList with default values.
func NewFileList(theme ChromaPalette) FileList {
	return FileList{
		Width: 25,
		Theme: theme,
	}
}

// Toggle flips the file list visibility.
func (f *FileList) Toggle() {
	f.Visible = !f.Visible
}

// SetFiles replaces the file list entries. Preserves selected index if valid.
func (f *FileList) SetFiles(files []FileEntry) {
	f.Files = files
	if f.Selected >= len(files) {
		if len(files) > 0 {
			f.Selected = len(files) - 1
		} else {
			f.Selected = 0
		}
	}
}

// SelectNext moves selection down by one.
func (f *FileList) SelectNext() {
	if len(f.Files) > 0 && f.Selected < len(f.Files)-1 {
		f.Selected++
	}
}

// SelectPrev moves selection up by one.
func (f *FileList) SelectPrev() {
	if f.Selected > 0 {
		f.Selected--
	}
}

// SelectIndex jumps to a 0-based file index. Returns false if out of range.
func (f *FileList) SelectIndex(idx int) bool {
	if idx < 0 || idx >= len(f.Files) {
		return false
	}
	f.Selected = idx
	return true
}

// SelectedPath returns the path of the currently selected file, or "".
func (f *FileList) SelectedPath() string {
	if f.Selected < 0 || f.Selected >= len(f.Files) {
		return ""
	}
	return f.Files[f.Selected].Path
}

// View renders the file list panel.
func (f FileList) View() string {
	if !f.Visible {
		return ""
	}

	bg := f.Theme.FileListBg
	fg := f.Theme.FileListFg
	selBg := f.Theme.SelectedBg
	accent := f.Theme.AccentColor
	badgeColor := f.Theme.UnseenBadge

	headerStyle := lipgloss.NewStyle().
		Width(f.Width).
		Background(bg).
		Foreground(accent).
		Bold(true).
		Padding(0, 1)

	itemStyle := lipgloss.NewStyle().
		Width(f.Width).
		Background(bg).
		Foreground(fg).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Width(f.Width).
		Background(selBg).
		Foreground(fg).
		Bold(true).
		Padding(0, 1)

	badgeStyle := lipgloss.NewStyle().
		Foreground(badgeColor)

	var lines []string
	lines = append(lines, headerStyle.Render("Changed Files"))

	maxItems := f.Height - 1 // minus header
	if maxItems < 0 {
		maxItems = 0
	}

	for i, entry := range f.Files {
		if i >= maxItems {
			break
		}

		name := filepath.Base(entry.Path)
		badge := " "
		if entry.Unseen {
			badge = badgeStyle.Render("●")
		}

		label := fmt.Sprintf("%s %s", badge, name)

		if i == f.Selected {
			lines = append(lines, selectedStyle.Render(label))
		} else {
			lines = append(lines, itemStyle.Render(label))
		}
	}

	// Fill remaining height
	emptyStyle := lipgloss.NewStyle().
		Width(f.Width).
		Background(bg)
	for len(lines) < f.Height {
		lines = append(lines, emptyStyle.Render(""))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
