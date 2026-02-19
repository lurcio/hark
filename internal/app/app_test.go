package app

import (
	"testing"

	"github.com/lurcio/hark/internal/diff"
)

func TestParseUnifiedDiffSingleHunk(t *testing.T) {
	raw := `--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 line1
-old line
+new line
 line3
`
	result := parseUnifiedDiff(raw)

	if !result.HasChanges {
		t.Error("expected HasChanges=true")
	}
	if result.Style != diff.Unified {
		t.Errorf("Style = %v, want Unified", result.Style)
	}

	// Expect: Header, Context(line1), Removed(old line), Added(new line), Context(line3)
	if len(result.Lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(result.Lines))
	}

	expect := []struct {
		typ     diff.LineType
		content string
	}{
		{diff.Header, "@@ -1,3 +1,3 @@"},
		{diff.Context, "line1"},
		{diff.Removed, "old line"},
		{diff.Added, "new line"},
		{diff.Context, "line3"},
	}

	for i, e := range expect {
		if result.Lines[i].Type != e.typ {
			t.Errorf("line[%d].Type = %v, want %v", i, result.Lines[i].Type, e.typ)
		}
		if result.Lines[i].Content != e.content {
			t.Errorf("line[%d].Content = %q, want %q", i, result.Lines[i].Content, e.content)
		}
	}
}

func TestParseUnifiedDiffMultipleHunks(t *testing.T) {
	raw := `--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 context1
-removed1
+added1
 context2
@@ -10,3 +10,3 @@
 context3
-removed2
+added2
 context4
`
	result := parseUnifiedDiff(raw)

	if !result.HasChanges {
		t.Error("expected HasChanges=true")
	}

	// Count headers
	headers := 0
	for _, l := range result.Lines {
		if l.Type == diff.Header {
			headers++
		}
	}
	if headers != 2 {
		t.Errorf("got %d headers, want 2", headers)
	}

	// Total lines: 2 headers + 2*3 content lines = 8
	if len(result.Lines) != 10 {
		t.Errorf("got %d lines, want 10", len(result.Lines))
	}
}

func TestParseUnifiedDiffAddedOnly(t *testing.T) {
	raw := `--- /dev/null
+++ b/new.txt
@@ -0,0 +1,3 @@
+line1
+line2
+line3
`
	result := parseUnifiedDiff(raw)

	if !result.HasChanges {
		t.Error("expected HasChanges=true")
	}

	added := 0
	for _, l := range result.Lines {
		if l.Type == diff.Added {
			added++
		}
	}
	if added != 3 {
		t.Errorf("got %d added lines, want 3", added)
	}
}

func TestParseUnifiedDiffRemovedOnly(t *testing.T) {
	raw := `--- a/old.txt
+++ /dev/null
@@ -1,3 +0,0 @@
-line1
-line2
-line3
`
	result := parseUnifiedDiff(raw)

	if !result.HasChanges {
		t.Error("expected HasChanges=true")
	}

	removed := 0
	for _, l := range result.Lines {
		if l.Type == diff.Removed {
			removed++
		}
	}
	if removed != 3 {
		t.Errorf("got %d removed lines, want 3", removed)
	}
}

func TestParseUnifiedDiffEmpty(t *testing.T) {
	result := parseUnifiedDiff("")

	if result.HasChanges {
		t.Error("expected HasChanges=false for empty diff")
	}
	if len(result.Lines) != 0 {
		t.Errorf("got %d lines, want 0", len(result.Lines))
	}
	if result.Style != diff.Unified {
		t.Errorf("Style = %v, want Unified", result.Style)
	}
}

func TestParseUnifiedDiffLineNumbers(t *testing.T) {
	raw := `@@ -5,3 +5,3 @@
 context
-old
+new
 context2
`
	result := parseUnifiedDiff(raw)

	if len(result.Lines) < 5 {
		t.Fatalf("got %d lines, want at least 5", len(result.Lines))
	}

	// OldLineNum is parsed from the hunk header; verify it increments correctly
	contextLine := result.Lines[1]
	if contextLine.OldLineNum != 5 {
		t.Errorf("context OldLineNum = %d, want 5", contextLine.OldLineNum)
	}

	removedLine := result.Lines[2]
	if removedLine.OldLineNum != 6 {
		t.Errorf("removed OldLineNum = %d, want 6", removedLine.OldLineNum)
	}
	if removedLine.Type != diff.Removed {
		t.Errorf("removed Type = %v, want Removed", removedLine.Type)
	}

	addedLine := result.Lines[3]
	if addedLine.Type != diff.Added {
		t.Errorf("added Type = %v, want Added", addedLine.Type)
	}

	// Last context line's OldLineNum should be 7 (5 + 2 increments)
	context2 := result.Lines[4]
	if context2.OldLineNum != 7 {
		t.Errorf("context2 OldLineNum = %d, want 7", context2.OldLineNum)
	}
}

func TestParseUnifiedDiffNoHeader(t *testing.T) {
	// A diff with content lines but no @@ header
	raw := `-old line
+new line
`
	result := parseUnifiedDiff(raw)

	if !result.HasChanges {
		t.Error("expected HasChanges=true")
	}
	// Lines should still parse, with line numbers starting at 0
	if len(result.Lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(result.Lines))
	}
	if result.Lines[0].Type != diff.Removed {
		t.Errorf("line[0].Type = %v, want Removed", result.Lines[0].Type)
	}
	if result.Lines[1].Type != diff.Added {
		t.Errorf("line[1].Type = %v, want Added", result.Lines[1].Type)
	}
}
