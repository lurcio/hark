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

	// Search state
	SearchQuery    string // current search query (empty = no search)
	SearchMatchLines []int  // indices into renderedLines that contain matches

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

// SetSearchQuery updates the search query and triggers a re-render.
func (d *DiffView) SetSearchQuery(query string) {
	d.SearchQuery = query
	d.dirty = true
}

// JumpToNextMatch moves the scroll position to the next search match.
// Returns true if a match was found.
func (d *DiffView) JumpToNextMatch() bool {
	d.ensureRendered()
	for _, pos := range d.SearchMatchLines {
		if pos > d.ScrollOffset {
			d.ScrollOffset = pos
			return true
		}
	}
	// Wrap around to first match
	if len(d.SearchMatchLines) > 0 {
		d.ScrollOffset = d.SearchMatchLines[0]
		return true
	}
	return false
}

// JumpToPrevMatch moves the scroll position to the previous search match.
// Returns true if a match was found.
func (d *DiffView) JumpToPrevMatch() bool {
	d.ensureRendered()
	for i := len(d.SearchMatchLines) - 1; i >= 0; i-- {
		if d.SearchMatchLines[i] < d.ScrollOffset {
			d.ScrollOffset = d.SearchMatchLines[i]
			return true
		}
	}
	// Wrap around to last match
	if len(d.SearchMatchLines) > 0 {
		d.ScrollOffset = d.SearchMatchLines[len(d.SearchMatchLines)-1]
		return true
	}
	return false
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
	d.SearchMatchLines = nil

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
	query := strings.ToLower(d.SearchQuery)

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
		hasMatch := query != "" && strings.Contains(strings.ToLower(line.Content), query)
		content := line.Content

		switch line.Type {
		case diff.Header:
			d.hunkPositions = append(d.hunkPositions, len(d.renderedLines))
			style := lipgloss.NewStyle().Foreground(headerFg)
			rendered = style.Render("  " + content)
		case diff.Added:
			lineNum := d.formatLineNum(0, line.NewLineNum)
			if hasMatch {
				rendered = lineNum + "+" + d.highlightMatches(content, addedBg)
			} else {
				style := lipgloss.NewStyle().Background(addedBg)
				rendered = style.Render(lineNum + "+" + content)
			}
		case diff.Removed:
			lineNum := d.formatLineNum(line.OldLineNum, 0)
			if hasMatch {
				rendered = lineNum + "-" + d.highlightMatches(content, removedBg)
			} else {
				style := lipgloss.NewStyle().Background(removedBg)
				rendered = style.Render(lineNum + "-" + content)
			}
		case diff.Context:
			lineNum := d.formatLineNum(line.OldLineNum, line.NewLineNum)
			if hasMatch {
				rendered = lineNum + " " + d.highlightMatches(content, "")
			} else {
				style := lipgloss.NewStyle().Foreground(contextFg)
				rendered = style.Render(lineNum + " " + content)
			}
		}

		if hasMatch {
			d.SearchMatchLines = append(d.SearchMatchLines, len(d.renderedLines))
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
	query := strings.ToLower(d.SearchQuery)

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
		hasMatch := false

		if pair.Left != nil {
			leftMatch := query != "" && strings.Contains(strings.ToLower(pair.Left.Content), query)
			if leftMatch {
				hasMatch = true
			}
			num := ""
			if d.ShowLineNumbers {
				num = fmt.Sprintf("%4d ", pair.Left.OldLineNum)
			}
			switch pair.Left.Type {
			case diff.Removed:
				if leftMatch {
					inner := num + "-" + d.highlightMatches(pair.Left.Content, removedBg)
					leftStr = lipgloss.NewStyle().Width(halfWidth).Render(inner)
				} else {
					style := lipgloss.NewStyle().Background(removedBg).Width(halfWidth)
					leftStr = style.Render(num + "-" + pair.Left.Content)
				}
			default:
				if leftMatch {
					inner := num + " " + d.highlightMatches(pair.Left.Content, "")
					leftStr = lipgloss.NewStyle().Width(halfWidth).Render(inner)
				} else {
					style := lipgloss.NewStyle().Foreground(contextFg).Width(halfWidth)
					leftStr = style.Render(num + " " + pair.Left.Content)
				}
			}
		} else {
			leftStr = lipgloss.NewStyle().Width(halfWidth).Render("")
		}

		if pair.Right != nil {
			rightMatch := query != "" && strings.Contains(strings.ToLower(pair.Right.Content), query)
			if rightMatch {
				hasMatch = true
			}
			num := ""
			if d.ShowLineNumbers {
				num = fmt.Sprintf("%4d ", pair.Right.NewLineNum)
			}
			switch pair.Right.Type {
			case diff.Added:
				if rightMatch {
					inner := num + "+" + d.highlightMatches(pair.Right.Content, addedBg)
					rightStr = lipgloss.NewStyle().Width(halfWidth).Render(inner)
				} else {
					style := lipgloss.NewStyle().Background(addedBg).Width(halfWidth)
					rightStr = style.Render(num + "+" + pair.Right.Content)
				}
			default:
				if rightMatch {
					inner := num + " " + d.highlightMatches(pair.Right.Content, "")
					rightStr = lipgloss.NewStyle().Width(halfWidth).Render(inner)
				} else {
					style := lipgloss.NewStyle().Foreground(contextFg).Width(halfWidth)
					rightStr = style.Render(num + " " + pair.Right.Content)
				}
			}
		} else {
			rightStr = lipgloss.NewStyle().Width(halfWidth).Render("")
		}

		if hasMatch {
			d.SearchMatchLines = append(d.SearchMatchLines, len(d.renderedLines))
		}
		d.renderedLines = append(d.renderedLines, leftStr+rightStr)
	}
}

// highlightMatches returns the content string with search matches highlighted.
// baseBg is the background color for non-matching portions (empty string for none).
func (d *DiffView) highlightMatches(content string, baseBg lipgloss.Color) string {
	query := strings.ToLower(d.SearchQuery)
	if query == "" {
		return content
	}

	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FFD700")).
		Foreground(lipgloss.Color("#000000"))

	var baseStyle lipgloss.Style
	if baseBg != "" {
		baseStyle = lipgloss.NewStyle().Background(baseBg)
	} else {
		baseStyle = lipgloss.NewStyle().Foreground(d.Theme.ContextFg)
	}

	lower := strings.ToLower(content)
	var b strings.Builder
	pos := 0
	for {
		idx := strings.Index(lower[pos:], query)
		if idx < 0 {
			b.WriteString(baseStyle.Render(content[pos:]))
			break
		}
		if idx > 0 {
			b.WriteString(baseStyle.Render(content[pos : pos+idx]))
		}
		b.WriteString(highlightStyle.Render(content[pos+idx : pos+idx+len(query)]))
		pos += idx + len(query)
	}
	return b.String()
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
