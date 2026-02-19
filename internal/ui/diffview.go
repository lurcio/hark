package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lurcio/hark/internal/diff"
)

// DiffView renders diff content with syntax highlighting, line numbers,
// and scrolling support.
type DiffView struct {
	Result   diff.DiffResult
	FileName string // relative path shown as header

	ScrollOffset int
	Width        int
	Height       int

	ShowLineNumbers bool
	Theme           ChromaPalette

	// hunkPositions caches line indices (in the rendered content) where
	// hunks begin, for n/N navigation.
	hunkPositions []int
	renderedLines []string
	dirty         bool
}

// NewDiffView creates a DiffView with default values.
func NewDiffView(theme ChromaPalette) DiffView {
	return DiffView{
		ShowLineNumbers: true,
		Theme:           theme,
		dirty:           true,
	}
}

// SetContent updates the diff result and filename, resetting scroll.
func (d *DiffView) SetContent(result diff.DiffResult, filename string) {
	d.Result = result
	d.FileName = filename
	d.ScrollOffset = 0
	d.dirty = true
}

// SetSize updates the viewport dimensions.
func (d *DiffView) SetSize(width, height int) {
	d.Width = width
	d.Height = height
}

// ScrollDown scrolls down by n lines.
func (d *DiffView) ScrollDown(n int) {
	d.ensureRendered()
	d.ScrollOffset += n
	maxScroll := len(d.renderedLines) - d.Height
	if maxScroll < 0 {
		maxScroll = 0
	}
	if d.ScrollOffset > maxScroll {
		d.ScrollOffset = maxScroll
	}
}

// ScrollUp scrolls up by n lines.
func (d *DiffView) ScrollUp(n int) {
	d.ScrollOffset -= n
	if d.ScrollOffset < 0 {
		d.ScrollOffset = 0
	}
}

// ScrollToTop jumps to the top.
func (d *DiffView) ScrollToTop() {
	d.ScrollOffset = 0
}

// ScrollToBottom jumps to the bottom.
func (d *DiffView) ScrollToBottom() {
	d.ensureRendered()
	maxScroll := len(d.renderedLines) - d.Height
	if maxScroll < 0 {
		maxScroll = 0
	}
	d.ScrollOffset = maxScroll
}

// JumpToNextHunk moves the scroll position to the next hunk header.
func (d *DiffView) JumpToNextHunk() {
	d.ensureRendered()
	for _, pos := range d.hunkPositions {
		if pos > d.ScrollOffset {
			d.ScrollOffset = pos
			return
		}
	}
}

// JumpToPrevHunk moves the scroll position to the previous hunk header.
func (d *DiffView) JumpToPrevHunk() {
	d.ensureRendered()
	for i := len(d.hunkPositions) - 1; i >= 0; i-- {
		if d.hunkPositions[i] < d.ScrollOffset {
			d.ScrollOffset = d.hunkPositions[i]
			return
		}
	}
}

// CurrentLineNumber returns the source file line number at the current scroll
// position. It maps the scroll offset to the new-file line number from the diff.
// Returns 1 if no line mapping is available.
func (d *DiffView) CurrentLineNumber() int {
	d.ensureRendered()

	// Account for the file header lines (filename + blank line)
	headerLines := 0
	if d.FileName != "" {
		headerLines = 2
	}

	// Map scroll offset to diff line index
	targetIdx := d.ScrollOffset - headerLines
	if targetIdx < 0 {
		targetIdx = 0
	}

	if d.Result.Style == diff.SideBySide {
		if targetIdx < len(d.Result.SideBySidePairs) {
			pair := d.Result.SideBySidePairs[targetIdx]
			if pair.Right != nil && pair.Right.NewLineNum > 0 {
				return pair.Right.NewLineNum
			}
			if pair.Left != nil && pair.Left.OldLineNum > 0 {
				return pair.Left.OldLineNum
			}
		}
	} else {
		if targetIdx < len(d.Result.Lines) {
			line := d.Result.Lines[targetIdx]
			if line.NewLineNum > 0 {
				return line.NewLineNum
			}
			if line.OldLineNum > 0 {
				return line.OldLineNum
			}
		}
	}

	return 1
}

// View renders the diff view as a string.
func (d *DiffView) View() string {
	d.ensureRendered()

	if len(d.renderedLines) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(d.Theme.ContextFg).
			Width(d.Width)
		return empty.Render("  No changes to display")
	}

	end := d.ScrollOffset + d.Height
	if end > len(d.renderedLines) {
		end = len(d.renderedLines)
	}
	start := d.ScrollOffset
	if start < 0 {
		start = 0
	}
	if start > end {
		start = end
	}

	visible := d.renderedLines[start:end]

	// Pad to full height
	for len(visible) < d.Height {
		visible = append(visible, "")
	}

	return strings.Join(visible, "\n")
}

// ensureRendered rebuilds the renderedLines cache if dirty.
func (d *DiffView) ensureRendered() {
	if !d.dirty {
		return
	}
	d.dirty = false
	d.renderedLines = nil
	d.hunkPositions = nil

	switch d.Result.Style {
	case diff.SideBySide:
		d.renderSideBySide()
	default:
		d.renderLines()
	}
}

// renderLines builds renderedLines for Unified and FullFile styles.
func (d *DiffView) renderLines() {
	addedBg := d.Theme.AddedBg
	removedBg := d.Theme.RemovedBg
	contextFg := d.Theme.ContextFg
	headerFg := d.Theme.HeaderFg

	// File header
	if d.FileName != "" {
		headerStyle := lipgloss.NewStyle().
			Foreground(headerFg).
			Bold(true)
		d.renderedLines = append(d.renderedLines, headerStyle.Render("  "+d.FileName))
		d.renderedLines = append(d.renderedLines, "")
	}

	for _, line := range d.Result.Lines {
		var rendered string
		switch line.Type {
		case diff.Header:
			d.hunkPositions = append(d.hunkPositions, len(d.renderedLines))
			style := lipgloss.NewStyle().Foreground(headerFg)
			rendered = style.Render("  " + line.Content)
		case diff.Added:
			lineNum := d.formatLineNum(0, line.NewLineNum)
			style := lipgloss.NewStyle().Background(addedBg)
			rendered = style.Render(lineNum + "+" + line.Content)
		case diff.Removed:
			lineNum := d.formatLineNum(line.OldLineNum, 0)
			style := lipgloss.NewStyle().Background(removedBg)
			rendered = style.Render(lineNum + "-" + line.Content)
		case diff.Context:
			lineNum := d.formatLineNum(line.OldLineNum, line.NewLineNum)
			style := lipgloss.NewStyle().Foreground(contextFg)
			rendered = style.Render(lineNum + " " + line.Content)
		}
		d.renderedLines = append(d.renderedLines, rendered)
	}
}

// renderSideBySide builds renderedLines for the SideBySide style.
func (d *DiffView) renderSideBySide() {
	addedBg := d.Theme.AddedBg
	removedBg := d.Theme.RemovedBg
	contextFg := d.Theme.ContextFg
	headerFg := d.Theme.HeaderFg

	halfWidth := d.Width / 2
	if halfWidth < 10 {
		halfWidth = 10
	}

	// File header
	if d.FileName != "" {
		headerStyle := lipgloss.NewStyle().
			Foreground(headerFg).
			Bold(true)
		d.renderedLines = append(d.renderedLines, headerStyle.Render("  "+d.FileName))
		d.renderedLines = append(d.renderedLines, "")
	}

	for _, pair := range d.Result.SideBySidePairs {
		leftStr := ""
		rightStr := ""

		if pair.Left != nil {
			num := ""
			if d.ShowLineNumbers {
				num = fmt.Sprintf("%4d ", pair.Left.OldLineNum)
			}
			switch pair.Left.Type {
			case diff.Removed:
				style := lipgloss.NewStyle().Background(removedBg).Width(halfWidth)
				leftStr = style.Render(num + "-" + pair.Left.Content)
			default:
				style := lipgloss.NewStyle().Foreground(contextFg).Width(halfWidth)
				leftStr = style.Render(num + " " + pair.Left.Content)
			}
		} else {
			leftStr = lipgloss.NewStyle().Width(halfWidth).Render("")
		}

		if pair.Right != nil {
			num := ""
			if d.ShowLineNumbers {
				num = fmt.Sprintf("%4d ", pair.Right.NewLineNum)
			}
			switch pair.Right.Type {
			case diff.Added:
				style := lipgloss.NewStyle().Background(addedBg).Width(halfWidth)
				rightStr = style.Render(num + "+" + pair.Right.Content)
			default:
				style := lipgloss.NewStyle().Foreground(contextFg).Width(halfWidth)
				rightStr = style.Render(num + " " + pair.Right.Content)
			}
		} else {
			rightStr = lipgloss.NewStyle().Width(halfWidth).Render("")
		}

		d.renderedLines = append(d.renderedLines, leftStr+rightStr)
	}
}

// formatLineNum formats old/new line numbers for display.
func (d *DiffView) formatLineNum(old, new int) string {
	if !d.ShowLineNumbers {
		return " "
	}
	oldStr := "    "
	newStr := "    "
	if old > 0 {
		oldStr = fmt.Sprintf("%4d", old)
	}
	if new > 0 {
		newStr = fmt.Sprintf("%4d", new)
	}
	return fmt.Sprintf(" %s %s ", oldStr, newStr)
}
