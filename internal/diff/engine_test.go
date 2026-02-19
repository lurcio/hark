package diff

import (
	"testing"
)

// --- helpers ---

func lineTypes(lines []DiffLine) []LineType {
	types := make([]LineType, len(lines))
	for i, l := range lines {
		types[i] = l.Type
	}
	return types
}

func lineContents(lines []DiffLine) []string {
	contents := make([]string, len(lines))
	for i, l := range lines {
		contents[i] = l.Content
	}
	return contents
}

func assertEqualLineTypes(t *testing.T, want, got []LineType) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("line type count: want %d, got %d\nwant: %v\ngot:  %v", len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("line %d type: want %v, got %v", i, want[i], got[i])
		}
	}
}

func assertEqualStrings(t *testing.T, want, got []string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("string count: want %d, got %d\nwant: %v\ngot:  %v", len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("index %d: want %q, got %q", i, want[i], got[i])
		}
	}
}

// --- Unified diff tests ---

func TestUnifiedBasicChange(t *testing.T) {
	old := "line1\nline2\nline3\n"
	new_ := "line1\nmodified\nline3\n"

	result := Compute(old, new_, Options{ContextLines: 3, Style: Unified})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}
	if result.Style != Unified {
		t.Fatalf("expected Unified style, got %v", result.Style)
	}

	wantTypes := []LineType{Header, Context, Removed, Added, Context}
	assertEqualLineTypes(t, wantTypes, lineTypes(result.Lines))

	// Verify content
	if result.Lines[1].Content != "line1" {
		t.Errorf("context line: want %q, got %q", "line1", result.Lines[1].Content)
	}
	if result.Lines[2].Content != "line2" {
		t.Errorf("removed line: want %q, got %q", "line2", result.Lines[2].Content)
	}
	if result.Lines[3].Content != "modified" {
		t.Errorf("added line: want %q, got %q", "modified", result.Lines[3].Content)
	}
	if result.Lines[4].Content != "line3" {
		t.Errorf("context line: want %q, got %q", "line3", result.Lines[4].Content)
	}

	// Verify line numbers
	if result.Lines[2].OldLineNum != 2 {
		t.Errorf("removed line old num: want 2, got %d", result.Lines[2].OldLineNum)
	}
	if result.Lines[3].NewLineNum != 2 {
		t.Errorf("added line new num: want 2, got %d", result.Lines[3].NewLineNum)
	}
}

func TestUnifiedMultipleHunks(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	new_ := "a\nB\nc\nd\ne\nf\ng\nh\nI\nj\n"

	// Context=1: changes at line 2 and line 9 should form separate hunks
	result := Compute(old, new_, Options{ContextLines: 1, Style: Unified})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	// Count headers to verify two hunks
	headerCount := 0
	for _, l := range result.Lines {
		if l.Type == Header {
			headerCount++
		}
	}
	if headerCount != 2 {
		t.Errorf("expected 2 hunks, got %d", headerCount)
	}
}

func TestUnifiedMultipleHunksMerge(t *testing.T) {
	old := "a\nb\nc\nd\ne\n"
	new_ := "a\nB\nc\nD\ne\n"

	// Context=3: changes at line 2 and 4 — context overlaps, should merge to one hunk
	result := Compute(old, new_, Options{ContextLines: 3, Style: Unified})

	headerCount := 0
	for _, l := range result.Lines {
		if l.Type == Header {
			headerCount++
		}
	}
	if headerCount != 1 {
		t.Errorf("expected 1 merged hunk, got %d", headerCount)
	}
}

func TestUnifiedHunkHeader(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nB\nc\n"

	result := Compute(old, new_, Options{ContextLines: 1, Style: Unified})

	if len(result.Lines) == 0 || result.Lines[0].Type != Header {
		t.Fatal("expected first line to be a header")
	}
	header := result.Lines[0].Content
	// Header should indicate old and new line ranges
	if header != "@@ -1,3 +1,3 @@" {
		t.Errorf("header: want %q, got %q", "@@ -1,3 +1,3 @@", header)
	}
}

// --- Side-by-side tests ---

func TestSideBySideBasic(t *testing.T) {
	old := "line1\nline2\nline3\n"
	new_ := "line1\nmodified\nline3\n"

	result := Compute(old, new_, Options{ContextLines: 3, Style: SideBySide})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}
	if result.Style != SideBySide {
		t.Fatalf("expected SideBySide style, got %v", result.Style)
	}
	if len(result.SideBySidePairs) == 0 {
		t.Fatal("expected side-by-side pairs")
	}

	// Should have 3 pairs: context, change, context
	if len(result.SideBySidePairs) != 3 {
		t.Fatalf("expected 3 pairs, got %d", len(result.SideBySidePairs))
	}

	// First pair: both sides are context "line1"
	p := result.SideBySidePairs[0]
	if p.Left == nil || p.Right == nil {
		t.Fatal("first pair should have both sides")
	}
	if p.Left.Type != Context || p.Right.Type != Context {
		t.Error("first pair should be context on both sides")
	}
	if p.Left.Content != "line1" {
		t.Errorf("left content: want %q, got %q", "line1", p.Left.Content)
	}

	// Second pair: removed on left, added on right
	p = result.SideBySidePairs[1]
	if p.Left == nil || p.Right == nil {
		t.Fatal("second pair should have both sides")
	}
	if p.Left.Type != Removed {
		t.Errorf("left type: want Removed, got %v", p.Left.Type)
	}
	if p.Right.Type != Added {
		t.Errorf("right type: want Added, got %v", p.Right.Type)
	}
	if p.Left.Content != "line2" {
		t.Errorf("left content: want %q, got %q", "line2", p.Left.Content)
	}
	if p.Right.Content != "modified" {
		t.Errorf("right content: want %q, got %q", "modified", p.Right.Content)
	}
}

func TestSideBySideUnbalanced(t *testing.T) {
	old := "line1\nline2\n"
	new_ := "line1\nnew1\nnew2\nnew3\n"

	result := Compute(old, new_, Options{ContextLines: 3, Style: SideBySide})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	// Find the change pairs (skip context)
	var changePairs []SideBySidePair
	for _, p := range result.SideBySidePairs {
		isChange := (p.Left != nil && p.Left.Type != Context) ||
			(p.Right != nil && p.Right.Type != Context)
		if isChange {
			changePairs = append(changePairs, p)
		}
	}

	// Should have 3 change pairs (1 delete paired with first insert, then 2 insert-only)
	if len(changePairs) != 3 {
		t.Fatalf("expected 3 change pairs, got %d", len(changePairs))
	}

	// First change: delete on left, insert on right
	if changePairs[0].Left == nil {
		t.Error("first change pair should have left side")
	}
	if changePairs[0].Right == nil {
		t.Error("first change pair should have right side")
	}

	// Remaining pairs: should have nil on left (insert only)
	for i := 1; i < len(changePairs); i++ {
		if changePairs[i].Left != nil {
			t.Errorf("pair %d: expected nil left for insert-only, got %v", i, changePairs[i].Left)
		}
		if changePairs[i].Right == nil {
			t.Errorf("pair %d: expected right side for insert-only", i)
		}
	}
}

// --- Full file tests ---

func TestFullFileBasic(t *testing.T) {
	old := "line1\nline2\nline3\n"
	new_ := "line1\nmodified\nline3\n"

	result := Compute(old, new_, Options{Style: FullFile})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}
	if result.Style != FullFile {
		t.Fatalf("expected FullFile style, got %v", result.Style)
	}

	// Full file should have NO header lines
	for i, l := range result.Lines {
		if l.Type == Header {
			t.Errorf("full file should not have header lines, found at index %d", i)
		}
	}

	// Should include all lines: context, removed, added, context
	wantTypes := []LineType{Context, Removed, Added, Context}
	assertEqualLineTypes(t, wantTypes, lineTypes(result.Lines))

	wantContents := []string{"line1", "line2", "modified", "line3"}
	assertEqualStrings(t, wantContents, lineContents(result.Lines))
}

func TestFullFileShowsEntireFile(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	new_ := "a\nb\nc\nd\nE\nf\ng\nh\ni\nj\n"

	result := Compute(old, new_, Options{Style: FullFile})

	// Full file should show all 11 lines (10 context/change + 1 removed + 1 added = 11)
	// Actually: 4 context before, 1 removed, 1 added, 5 context after = 11
	if len(result.Lines) != 11 {
		t.Errorf("expected 11 lines in full file, got %d", len(result.Lines))
		for i, l := range result.Lines {
			t.Logf("  line %d: type=%v content=%q", i, l.Type, l.Content)
		}
	}
}

// --- Context lines tests ---

func TestContextLinesZero(t *testing.T) {
	old := "a\nb\nc\nd\ne\n"
	new_ := "a\nb\nC\nd\ne\n"

	result := Compute(old, new_, Options{ContextLines: 0, Style: Unified})

	// With 0 context, should only have header + the changed lines
	wantTypes := []LineType{Header, Removed, Added}
	assertEqualLineTypes(t, wantTypes, lineTypes(result.Lines))
}

func TestContextLinesOne(t *testing.T) {
	old := "a\nb\nc\nd\ne\n"
	new_ := "a\nb\nC\nd\ne\n"

	result := Compute(old, new_, Options{ContextLines: 1, Style: Unified})

	// With 1 context line: header, context(b), removed(c), added(C), context(d)
	wantTypes := []LineType{Header, Context, Removed, Added, Context}
	assertEqualLineTypes(t, wantTypes, lineTypes(result.Lines))
}

func TestContextLinesLargerThanFile(t *testing.T) {
	old := "a\nb\nc\n"
	new_ := "a\nB\nc\n"

	result := Compute(old, new_, Options{ContextLines: 100, Style: Unified})

	// All lines should be included
	wantTypes := []LineType{Header, Context, Removed, Added, Context}
	assertEqualLineTypes(t, wantTypes, lineTypes(result.Lines))
}

// --- Edge cases ---

func TestNoChanges(t *testing.T) {
	content := "same\ncontent\nhere\n"

	result := Compute(content, content, Options{ContextLines: 3, Style: Unified})

	if result.HasChanges {
		t.Error("expected HasChanges to be false")
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected no lines, got %d", len(result.Lines))
	}
}

func TestBothEmpty(t *testing.T) {
	result := Compute("", "", Options{ContextLines: 3, Style: Unified})

	if result.HasChanges {
		t.Error("expected HasChanges to be false")
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected no lines, got %d", len(result.Lines))
	}
}

func TestNewFile(t *testing.T) {
	new_ := "line1\nline2\nline3\n"

	result := Compute("", new_, Options{ContextLines: 3, Style: Unified})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	// All lines should be Added (plus a header)
	for _, l := range result.Lines {
		if l.Type != Header && l.Type != Added {
			t.Errorf("new file should only have Header and Added lines, got %v for %q", l.Type, l.Content)
		}
	}

	// Verify content
	addedContents := []string{}
	for _, l := range result.Lines {
		if l.Type == Added {
			addedContents = append(addedContents, l.Content)
		}
	}
	assertEqualStrings(t, []string{"line1", "line2", "line3"}, addedContents)
}

func TestDeletedFile(t *testing.T) {
	old := "line1\nline2\nline3\n"

	result := Compute(old, "", Options{ContextLines: 3, Style: Unified})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	// All lines should be Removed (plus a header)
	for _, l := range result.Lines {
		if l.Type != Header && l.Type != Removed {
			t.Errorf("deleted file should only have Header and Removed lines, got %v for %q", l.Type, l.Content)
		}
	}

	removedContents := []string{}
	for _, l := range result.Lines {
		if l.Type == Removed {
			removedContents = append(removedContents, l.Content)
		}
	}
	assertEqualStrings(t, []string{"line1", "line2", "line3"}, removedContents)
}

func TestNewFileSideBySide(t *testing.T) {
	new_ := "alpha\nbeta\n"

	result := Compute("", new_, Options{ContextLines: 3, Style: SideBySide})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	// All pairs should have nil Left and non-nil Right
	for i, p := range result.SideBySidePairs {
		if p.Left != nil {
			t.Errorf("pair %d: expected nil left for new file", i)
		}
		if p.Right == nil {
			t.Errorf("pair %d: expected non-nil right for new file", i)
		}
		if p.Right != nil && p.Right.Type != Added {
			t.Errorf("pair %d: expected Added type, got %v", i, p.Right.Type)
		}
	}
}

func TestDeletedFileSideBySide(t *testing.T) {
	old := "alpha\nbeta\n"

	result := Compute(old, "", Options{ContextLines: 3, Style: SideBySide})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	// All pairs should have non-nil Left and nil Right
	for i, p := range result.SideBySidePairs {
		if p.Left == nil {
			t.Errorf("pair %d: expected non-nil left for deleted file", i)
		}
		if p.Right != nil {
			t.Errorf("pair %d: expected nil right for deleted file", i)
		}
		if p.Left != nil && p.Left.Type != Removed {
			t.Errorf("pair %d: expected Removed type, got %v", i, p.Left.Type)
		}
	}
}

func TestNewFileFullFile(t *testing.T) {
	new_ := "one\ntwo\nthree\n"

	result := Compute("", new_, Options{Style: FullFile})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	for _, l := range result.Lines {
		if l.Type != Added {
			t.Errorf("new file full view: expected all Added, got %v for %q", l.Type, l.Content)
		}
	}
}

// --- Line number tests ---

func TestLineNumbers(t *testing.T) {
	old := "a\nb\nc\nd\n"
	new_ := "a\nB\nc\nd\n"

	result := Compute(old, new_, Options{ContextLines: 1, Style: Unified})

	for _, l := range result.Lines {
		switch l.Type {
		case Context:
			if l.OldLineNum == 0 || l.NewLineNum == 0 {
				t.Errorf("context line %q should have both line numbers, got old=%d new=%d",
					l.Content, l.OldLineNum, l.NewLineNum)
			}
		case Removed:
			if l.OldLineNum == 0 {
				t.Errorf("removed line %q should have old line number", l.Content)
			}
			if l.NewLineNum != 0 {
				t.Errorf("removed line %q should not have new line number, got %d", l.Content, l.NewLineNum)
			}
		case Added:
			if l.NewLineNum == 0 {
				t.Errorf("added line %q should have new line number", l.Content)
			}
			if l.OldLineNum != 0 {
				t.Errorf("added line %q should not have old line number, got %d", l.Content, l.OldLineNum)
			}
		}
	}
}

// --- Word diff tests ---

func TestWordDiffUnified(t *testing.T) {
	old := "hello world\n"
	new_ := "hello universe\n"

	result := Compute(old, new_, Options{ContextLines: 3, Style: Unified, WordDiff: true})

	if !result.HasChanges {
		t.Fatal("expected HasChanges to be true")
	}

	// Find the removed and added lines
	var removedLine, addedLine *DiffLine
	for i := range result.Lines {
		if result.Lines[i].Type == Removed {
			removedLine = &result.Lines[i]
		}
		if result.Lines[i].Type == Added {
			addedLine = &result.Lines[i]
		}
	}

	if removedLine == nil || addedLine == nil {
		t.Fatal("expected both a removed and added line")
	}

	// Both should have inline changes
	if len(removedLine.InlineChanges) == 0 {
		t.Error("removed line should have inline changes for word diff")
	}
	if len(addedLine.InlineChanges) == 0 {
		t.Error("added line should have inline changes for word diff")
	}

	// Verify the change spans are within bounds
	for _, h := range removedLine.InlineChanges {
		if h.Start < 0 || h.End > len(removedLine.Content) || h.Start >= h.End {
			t.Errorf("invalid inline highlight range [%d, %d) for content length %d",
				h.Start, h.End, len(removedLine.Content))
		}
	}
	for _, h := range addedLine.InlineChanges {
		if h.Start < 0 || h.End > len(addedLine.Content) || h.Start >= h.End {
			t.Errorf("invalid inline highlight range [%d, %d) for content length %d",
				h.Start, h.End, len(addedLine.Content))
		}
	}
}

func TestWordDiffSideBySide(t *testing.T) {
	old := "foo bar baz\n"
	new_ := "foo qux baz\n"

	result := Compute(old, new_, Options{ContextLines: 3, Style: SideBySide, WordDiff: true})

	// Find the change pair
	var changePair *SideBySidePair
	for i := range result.SideBySidePairs {
		p := &result.SideBySidePairs[i]
		if p.Left != nil && p.Left.Type == Removed {
			changePair = p
			break
		}
	}

	if changePair == nil {
		t.Fatal("expected a change pair in side-by-side result")
	}

	if len(changePair.Left.InlineChanges) == 0 {
		t.Error("left side should have inline changes")
	}
	if len(changePair.Right.InlineChanges) == 0 {
		t.Error("right side should have inline changes")
	}
}

func TestWordDiffDisabledByDefault(t *testing.T) {
	old := "hello world\n"
	new_ := "hello universe\n"

	result := Compute(old, new_, Options{ContextLines: 3, Style: Unified})

	for _, l := range result.Lines {
		if len(l.InlineChanges) > 0 {
			t.Error("inline changes should not be present when WordDiff is disabled")
		}
	}
}

// --- DefaultOptions test ---

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.ContextLines != 3 {
		t.Errorf("default context lines: want 3, got %d", opts.ContextLines)
	}
	if opts.Style != Unified {
		t.Errorf("default style: want Unified, got %v", opts.Style)
	}
	if opts.WordDiff {
		t.Error("default word diff should be false")
	}
}

// --- Highlight helper tests (language detection only, not output) ---

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		wantLang string
	}{
		{"main.go", "Go"},
		{"app.py", "Python"},
		{"index.js", "JavaScript"},
		{"style.css", "CSS"},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := DetectLanguage(tt.filename)
			if got != tt.wantLang {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.filename, got, tt.wantLang)
			}
		})
	}
}

// --- Internal function tests ---

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty", "", nil},
		{"single no newline", "hello", []string{"hello"}},
		{"single with newline", "hello\n", []string{"hello"}},
		{"multiple with trailing", "a\nb\nc\n", []string{"a", "b", "c"}},
		{"multiple no trailing", "a\nb\nc", []string{"a", "b", "c"}},
		{"empty lines", "\n\n", []string{"", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitLines(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}
