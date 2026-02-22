package diffview

import (
	"strings"
	"testing"
)

func TestRenderSplitIncludesCursorAndSideCommentMarkers(t *testing.T) {
	rows := []DiffRow{
		{Kind: RowContext, Path: "a.txt", OldLine: intPtr(1), NewLine: intPtr(1), OldText: "before", NewText: "before"},
		{Kind: RowChange, Path: "a.txt", OldLine: intPtr(2), NewLine: intPtr(2), OldText: "old", NewText: "new"},
		{Kind: RowAdd, Path: "a.txt", NewLine: intPtr(3), NewText: "added"},
	}

	oldLines, newLines := RenderSplit(rows, 30, 30, 1, func(path string, line int, side Side) bool {
		return path == "a.txt" && line == 2 && side == SideNew
	})

	if len(oldLines) != len(rows) || len(newLines) != len(rows) {
		t.Fatalf("line counts mismatch old=%d new=%d rows=%d", len(oldLines), len(newLines), len(rows))
	}

	if !strings.HasPrefix(oldLines[1], ">  ") {
		t.Fatalf("expected cursor-only marker on old row 1, got %q", oldLines[1])
	}
	if !strings.HasPrefix(newLines[1], ">* ") {
		t.Fatalf("expected cursor+comment marker on new row 1, got %q", newLines[1])
	}

	for i, line := range oldLines {
		if len([]rune(line)) > 30 {
			t.Fatalf("old line %d exceeds width: %q", i, line)
		}
	}
	for i, line := range newLines {
		if len([]rune(line)) > 30 {
			t.Fatalf("new line %d exceeds width: %q", i, line)
		}
	}
}

func TestRenderSplitUsesAddRemoveMarkersForSingleSidedRows(t *testing.T) {
	rows := []DiffRow{
		{Kind: RowDelete, Path: "a.txt", OldLine: intPtr(5), OldText: "gone"},
		{Kind: RowAdd, Path: "a.txt", NewLine: intPtr(8), NewText: "new"},
	}

	oldLines, newLines := RenderSplit(rows, 40, 40, 0, nil)

	if !strings.Contains(oldLines[0], "-   5 gone") {
		t.Fatalf("expected removed marker in old pane, got %q", oldLines[0])
	}
	if strings.TrimSpace(newLines[0]) != ">" {
		t.Fatalf("expected blank new-side delete row except cursor prefix, got %q", newLines[0])
	}

	if strings.TrimSpace(oldLines[1]) != "" {
		t.Fatalf("expected blank old-side add row, got %q", oldLines[1])
	}
	if !strings.Contains(newLines[1], "+   8 new") {
		t.Fatalf("expected added marker in new pane, got %q", newLines[1])
	}
}

func intPtr(n int) *int {
	v := n
	return &v
}
