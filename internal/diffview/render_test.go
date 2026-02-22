package diffview

import (
	"strings"
	"testing"
)

func TestRenderSideBySideIncludesCursorAndCommentMarkers(t *testing.T) {
	rows := []DiffRow{
		{Kind: RowContext, Path: "a.txt", OldLine: intPtr(1), NewLine: intPtr(1), OldText: "before", NewText: "before"},
		{Kind: RowChange, Path: "a.txt", OldLine: intPtr(2), NewLine: intPtr(2), OldText: "old", NewText: "new"},
		{Kind: RowAdd, Path: "a.txt", NewLine: intPtr(3), NewText: "added"},
	}

	lines := RenderSideBySide(rows, 60, 1, func(path string, line int, side Side) bool {
		return path == "a.txt" && line == 2 && side == SideNew
	})
	if len(lines) != len(rows) {
		t.Fatalf("line count = %d, want %d", len(lines), len(rows))
	}
	if !strings.HasPrefix(lines[1], ">* ") {
		t.Fatalf("expected cursor/comment prefix on row 1, got %q", lines[1])
	}
	for i, line := range lines {
		if len([]rune(line)) > 60 {
			t.Fatalf("line %d exceeds width: %q", i, line)
		}
	}
}

func intPtr(n int) *int {
	v := n
	return &v
}
