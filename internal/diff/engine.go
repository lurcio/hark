// Package diff computes structured diffs between file contents.
//
// It supports three display styles: Unified (standard git diff format),
// SideBySide (old on left, new on right), and FullFile (entire file with
// changed lines highlighted). Optional word-level diff highlighting shows
// exactly which characters changed within modified lines.
package diff

import (
	"fmt"
	"strings"

	diffmatchpatch "github.com/sergi/go-diff/diffmatchpatch"
)

// DiffStyle represents the display style for diffs.
type DiffStyle int

const (
	Unified    DiffStyle = iota // Standard git diff format
	SideBySide                  // Old on left, new on right
	FullFile                    // Entire file with changed lines highlighted
)

// LineType represents the type of a diff line.
type LineType int

const (
	Context LineType = iota // Unchanged line
	Added                   // Line added in new content
	Removed                 // Line removed from old content
	Header                  // Hunk header (@@ ... @@)
)

// InlineHighlight marks a byte range within a line that changed (for word diff).
type InlineHighlight struct {
	Start int // byte offset (inclusive)
	End   int // byte offset (exclusive)
}

// DiffLine represents a single line in a diff result.
type DiffLine struct {
	Type          LineType
	Content       string
	OldLineNum    int               // 0 if not applicable (e.g., added lines)
	NewLineNum    int               // 0 if not applicable (e.g., removed lines)
	InlineChanges []InlineHighlight // non-nil when word diff is enabled
}

// SideBySidePair represents a pair of lines for side-by-side display.
type SideBySidePair struct {
	Left  *DiffLine // nil if line was added (only on right)
	Right *DiffLine // nil if line was removed (only on left)
}

// DiffResult contains the result of a diff computation.
type DiffResult struct {
	Lines           []DiffLine       // populated for Unified and FullFile styles
	SideBySidePairs []SideBySidePair // populated for SideBySide style
	Style           DiffStyle
	HasChanges      bool
}

// Options configures the diff computation.
type Options struct {
	ContextLines int       // lines of context around changes (default 3)
	Style        DiffStyle // display style
	FileName     string    // used for syntax highlighting language detection
	WordDiff     bool      // enable word-level diff highlighting
}

// DefaultOptions returns Options with sensible defaults.
func DefaultOptions() Options {
	return Options{
		ContextLines: 3,
		Style:        Unified,
	}
}

// Compute computes a diff between oldContent and newContent.
func Compute(oldContent, newContent string, opts Options) DiffResult {
	if oldContent == newContent {
		return DiffResult{Style: opts.Style}
	}

	edits := computeEditScript(oldContent, newContent)

	var result DiffResult
	switch opts.Style {
	case SideBySide:
		result = buildSideBySide(edits, opts.ContextLines)
	case FullFile:
		result = buildFullFile(edits)
	default:
		result = buildUnified(edits, opts.ContextLines)
	}

	if opts.WordDiff {
		if result.Style == SideBySide {
			applyWordDiffSideBySide(result.SideBySidePairs)
		} else {
			applyWordDiffLines(result.Lines)
		}
	}

	return result
}

// --- internal types ---

type editOp int

const (
	opEqual  editOp = iota
	opInsert
	opDelete
)

type editLine struct {
	op         editOp
	text       string
	oldLineNum int
	newLineNum int
}

// --- edit script computation ---

func computeEditScript(oldContent, newContent string) []editLine {
	dmp := diffmatchpatch.New()

	// Line-level diff: encode lines as runes, diff, then decode
	chars1, chars2, lineArray := dmp.DiffLinesToChars(oldContent, newContent)
	diffs := dmp.DiffMain(chars1, chars2, false)
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	var edits []editLine
	for _, d := range diffs {
		lines := splitLines(d.Text)
		var op editOp
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			op = opEqual
		case diffmatchpatch.DiffInsert:
			op = opInsert
		case diffmatchpatch.DiffDelete:
			op = opDelete
		}
		for _, line := range lines {
			edits = append(edits, editLine{op: op, text: line})
		}
	}

	assignLineNumbers(edits)
	return edits
}

// splitLines splits text by newlines, dropping the trailing empty string
// that results from a trailing newline.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func assignLineNumbers(edits []editLine) {
	oldLine := 1
	newLine := 1
	for i := range edits {
		switch edits[i].op {
		case opEqual:
			edits[i].oldLineNum = oldLine
			edits[i].newLineNum = newLine
			oldLine++
			newLine++
		case opDelete:
			edits[i].oldLineNum = oldLine
			oldLine++
		case opInsert:
			edits[i].newLineNum = newLine
			newLine++
		}
	}
}

// --- hunk detection ---

type hunkRange struct {
	start int // inclusive index into edits
	end   int // exclusive index into edits
}

// findHunks identifies contiguous ranges of edits that include changes and
// their surrounding context lines. Adjacent hunks whose context overlaps
// are merged automatically.
func findHunks(edits []editLine, contextLines int) []hunkRange {
	n := len(edits)
	if n == 0 {
		return nil
	}

	interesting := make([]bool, n)
	for i, e := range edits {
		if e.op != opEqual {
			lo := max(0, i-contextLines)
			hi := min(n, i+contextLines+1)
			for j := lo; j < hi; j++ {
				interesting[j] = true
			}
		}
	}

	var hunks []hunkRange
	i := 0
	for i < n {
		if interesting[i] {
			start := i
			for i < n && interesting[i] {
				i++
			}
			hunks = append(hunks, hunkRange{start: start, end: i})
		} else {
			i++
		}
	}

	return hunks
}

// hunkHeader computes the unified diff hunk header for a given range.
func hunkHeader(edits []editLine, h hunkRange) string {
	oldStart := 0
	oldCount := 0
	newStart := 0
	newCount := 0
	foundOld := false
	foundNew := false

	for i := h.start; i < h.end; i++ {
		e := edits[i]
		switch e.op {
		case opEqual:
			if !foundOld {
				oldStart = e.oldLineNum
				foundOld = true
			}
			if !foundNew {
				newStart = e.newLineNum
				foundNew = true
			}
			oldCount++
			newCount++
		case opDelete:
			if !foundOld {
				oldStart = e.oldLineNum
				foundOld = true
			}
			oldCount++
		case opInsert:
			if !foundNew {
				newStart = e.newLineNum
				foundNew = true
			}
			newCount++
		}
	}

	// For pure-delete hunks (no new lines), find the new file position
	// by looking at edits before the hunk.
	if !foundNew && newCount == 0 {
		for i := h.start - 1; i >= 0; i-- {
			if edits[i].newLineNum > 0 {
				newStart = edits[i].newLineNum
				break
			}
		}
	}

	// For pure-insert hunks (no old lines), find the old file position.
	if !foundOld && oldCount == 0 {
		for i := h.start - 1; i >= 0; i-- {
			if edits[i].oldLineNum > 0 {
				oldStart = edits[i].oldLineNum
				break
			}
		}
	}

	return fmt.Sprintf("@@ -%d,%d +%d,%d @@", oldStart, oldCount, newStart, newCount)
}

// --- output builders ---

func buildUnified(edits []editLine, contextLines int) DiffResult {
	hunks := findHunks(edits, contextLines)
	if len(hunks) == 0 {
		return DiffResult{Style: Unified}
	}

	var lines []DiffLine
	for _, hunk := range hunks {
		lines = append(lines, DiffLine{
			Type:    Header,
			Content: hunkHeader(edits, hunk),
		})
		for i := hunk.start; i < hunk.end; i++ {
			e := edits[i]
			switch e.op {
			case opEqual:
				lines = append(lines, DiffLine{
					Type:       Context,
					Content:    e.text,
					OldLineNum: e.oldLineNum,
					NewLineNum: e.newLineNum,
				})
			case opDelete:
				lines = append(lines, DiffLine{
					Type:       Removed,
					Content:    e.text,
					OldLineNum: e.oldLineNum,
				})
			case opInsert:
				lines = append(lines, DiffLine{
					Type:       Added,
					Content:    e.text,
					NewLineNum: e.newLineNum,
				})
			}
		}
	}

	return DiffResult{
		Lines:      lines,
		Style:      Unified,
		HasChanges: true,
	}
}

func buildSideBySide(edits []editLine, contextLines int) DiffResult {
	hunks := findHunks(edits, contextLines)
	if len(hunks) == 0 {
		return DiffResult{Style: SideBySide}
	}

	var pairs []SideBySidePair
	for _, hunk := range hunks {
		i := hunk.start
		for i < hunk.end {
			e := edits[i]
			if e.op == opEqual {
				left := DiffLine{Type: Context, Content: e.text, OldLineNum: e.oldLineNum}
				right := DiffLine{Type: Context, Content: e.text, NewLineNum: e.newLineNum}
				pairs = append(pairs, SideBySidePair{Left: &left, Right: &right})
				i++
			} else {
				// Collect consecutive deletes and inserts in this change block
				var deletes, inserts []editLine
				for i < hunk.end && edits[i].op != opEqual {
					if edits[i].op == opDelete {
						deletes = append(deletes, edits[i])
					} else {
						inserts = append(inserts, edits[i])
					}
					i++
				}

				// Pair them: shorter side gets nil entries
				pairLen := max(len(deletes), len(inserts))
				for j := 0; j < pairLen; j++ {
					pair := SideBySidePair{}
					if j < len(deletes) {
						line := DiffLine{
							Type:       Removed,
							Content:    deletes[j].text,
							OldLineNum: deletes[j].oldLineNum,
						}
						pair.Left = &line
					}
					if j < len(inserts) {
						line := DiffLine{
							Type:       Added,
							Content:    inserts[j].text,
							NewLineNum: inserts[j].newLineNum,
						}
						pair.Right = &line
					}
					pairs = append(pairs, pair)
				}
			}
		}
	}

	return DiffResult{
		SideBySidePairs: pairs,
		Style:           SideBySide,
		HasChanges:      true,
	}
}

func buildFullFile(edits []editLine) DiffResult {
	if len(edits) == 0 {
		return DiffResult{Style: FullFile}
	}

	var lines []DiffLine
	hasChanges := false

	for _, e := range edits {
		switch e.op {
		case opEqual:
			lines = append(lines, DiffLine{
				Type:       Context,
				Content:    e.text,
				OldLineNum: e.oldLineNum,
				NewLineNum: e.newLineNum,
			})
		case opDelete:
			hasChanges = true
			lines = append(lines, DiffLine{
				Type:       Removed,
				Content:    e.text,
				OldLineNum: e.oldLineNum,
			})
		case opInsert:
			hasChanges = true
			lines = append(lines, DiffLine{
				Type:       Added,
				Content:    e.text,
				NewLineNum: e.newLineNum,
			})
		}
	}

	return DiffResult{
		Lines:      lines,
		Style:      FullFile,
		HasChanges: hasChanges,
	}
}

// --- word diff ---

func computeWordDiff(oldLine, newLine string) ([]InlineHighlight, []InlineHighlight) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldLine, newLine, false)
	diffs = dmp.DiffCleanupSemantic(diffs)

	var oldHL, newHL []InlineHighlight
	oldOff := 0
	newOff := 0

	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			oldOff += len(d.Text)
			newOff += len(d.Text)
		case diffmatchpatch.DiffDelete:
			oldHL = append(oldHL, InlineHighlight{Start: oldOff, End: oldOff + len(d.Text)})
			oldOff += len(d.Text)
		case diffmatchpatch.DiffInsert:
			newHL = append(newHL, InlineHighlight{Start: newOff, End: newOff + len(d.Text)})
			newOff += len(d.Text)
		}
	}

	return oldHL, newHL
}

func applyWordDiffLines(lines []DiffLine) {
	i := 0
	for i < len(lines) {
		if lines[i].Type == Removed {
			removeStart := i
			for i < len(lines) && lines[i].Type == Removed {
				i++
			}
			addStart := i
			for i < len(lines) && lines[i].Type == Added {
				i++
			}
			addEnd := i

			removes := lines[removeStart:addStart]
			adds := lines[addStart:addEnd]
			pairLen := min(len(removes), len(adds))

			for j := 0; j < pairLen; j++ {
				oldHL, newHL := computeWordDiff(removes[j].Content, adds[j].Content)
				removes[j].InlineChanges = oldHL
				adds[j].InlineChanges = newHL
			}
		} else {
			i++
		}
	}
}

func applyWordDiffSideBySide(pairs []SideBySidePair) {
	for i := range pairs {
		if pairs[i].Left != nil && pairs[i].Right != nil &&
			pairs[i].Left.Type == Removed && pairs[i].Right.Type == Added {
			oldHL, newHL := computeWordDiff(pairs[i].Left.Content, pairs[i].Right.Content)
			pairs[i].Left.InlineChanges = oldHL
			pairs[i].Right.InlineChanges = newHL
		}
	}
}
