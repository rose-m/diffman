package diffview

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

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

	if !strings.HasPrefix(stripANSI(oldLines[1]), ">C ") {
		t.Fatalf("expected cursor+comment marker on old row 1, got %q", oldLines[1])
	}
	if !strings.HasPrefix(stripANSI(newLines[1]), ">C ") {
		t.Fatalf("expected cursor+comment marker on new row 1, got %q", newLines[1])
	}

	for i, line := range oldLines {
		if lipgloss.Width(line) > 30 {
			t.Fatalf("old line %d exceeds width: %q", i, line)
		}
	}
	for i, line := range newLines {
		if lipgloss.Width(line) > 30 {
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
	old0 := stripANSI(oldLines[0])
	new0 := stripANSI(newLines[0])
	old1 := stripANSI(oldLines[1])
	new1 := stripANSI(newLines[1])

	if !strings.Contains(old0, "-   5 gone") {
		t.Fatalf("expected removed marker in old pane, got %q", old0)
	}
	if strings.TrimSpace(new0) != ">" {
		t.Fatalf("expected blank new-side delete row except cursor prefix, got %q", new0)
	}

	if strings.TrimSpace(old1) != "" {
		t.Fatalf("expected blank old-side add row, got %q", old1)
	}
	if !strings.Contains(new1, "+   8 new") {
		t.Fatalf("expected added marker in new pane, got %q", new1)
	}
}

func TestRenderSplitWithLayoutKeepsRowsAlignedWhenWrapping(t *testing.T) {
	rows := []DiffRow{
		{
			Kind:    RowChange,
			Path:    "a.txt",
			OldLine: intPtr(10),
			NewLine: intPtr(10),
			OldText: "old side has a much longer line than new side",
			NewText: "short",
		},
		{
			Kind:    RowContext,
			Path:    "a.txt",
			OldLine: intPtr(11),
			NewLine: intPtr(11),
			OldText: "next",
			NewText: "next",
		},
	}

	out := RenderSplitWithLayout(rows, 20, 20, 0, nil)
	if len(out.RowStarts) != len(rows) || len(out.RowHeights) != len(rows) {
		t.Fatalf("unexpected row map sizes starts=%d heights=%d", len(out.RowStarts), len(out.RowHeights))
	}
	if len(out.OldLines) != len(out.NewLines) {
		t.Fatalf("old/new visual line counts differ old=%d new=%d", len(out.OldLines), len(out.NewLines))
	}
	if out.RowHeights[0] <= 1 {
		t.Fatalf("expected wrapped first row height > 1, got %d", out.RowHeights[0])
	}
	if out.RowStarts[1] != out.RowStarts[0]+out.RowHeights[0] {
		t.Fatalf("second row start misaligned: got %d want %d", out.RowStarts[1], out.RowStarts[0]+out.RowHeights[0])
	}
}

func TestRenderSplitWithLayoutExpandsTabsBeforeWrapping(t *testing.T) {
	rows := []DiffRow{
		{
			Kind:    RowAdd,
			Path:    "a.txt",
			NewLine: intPtr(5),
			NewText: "\tif len(items) > 0 {\treturn items[0]\t}",
		},
	}

	out := RenderSplitWithLayout(rows, 24, 24, 0, nil)
	if len(out.OldLines) != len(out.NewLines) {
		t.Fatalf("old/new visual line counts differ old=%d new=%d", len(out.OldLines), len(out.NewLines))
	}
	for i, line := range out.NewLines {
		plain := stripANSI(line)
		if strings.ContainsRune(plain, '\t') {
			t.Fatalf("new line %d still contains tab: %q", i, plain)
		}
		if lipgloss.Width(line) > 24 {
			t.Fatalf("new line %d exceeds width: %q", i, line)
		}
	}
}

func TestRenderSplitWithLayoutContinuationKeepsLineNumberIndent(t *testing.T) {
	rows := []DiffRow{
		{
			Kind:    RowAdd,
			Path:    "a.txt",
			NewLine: intPtr(12),
			NewText: "abcdefghijklmnopqrstuvwxyz",
		},
	}

	out := RenderSplitWithLayout(rows, 22, 22, 0, nil)
	if len(out.NewLines) < 2 {
		t.Fatalf("expected wrapped output, got %d visual lines", len(out.NewLines))
	}

	plain := stripANSI(out.NewLines[1])
	// Prefix (3) + meta for '+ %3d ' (6) => continuation text begins at column 10.
	wantPrefix := strings.Repeat(" ", 9)
	if !strings.HasPrefix(plain, wantPrefix) {
		t.Fatalf("continuation line does not keep line-number indent: %q", plain)
	}
	if len([]rune(plain)) <= len([]rune(wantPrefix)) || []rune(plain)[len([]rune(wantPrefix))] == ' ' {
		t.Fatalf("continuation line does not keep line-number indent: %q", plain)
	}
}

func TestRenderSplitWithLayoutHighlightsChangedWords(t *testing.T) {
	rows := []DiffRow{
		{
			Kind:    RowChange,
			Path:    "a.txt",
			OldLine: intPtr(3),
			NewLine: intPtr(3),
			OldText: "alpha beta gamma",
			NewText: "alpha zeta gamma",
		},
	}

	out := RenderSplitWithLayout(rows, 40, 40, 0, nil)
	if !strings.Contains(stripANSI(out.OldLines[0]), "beta") || !strings.Contains(stripANSI(out.NewLines[0]), "zeta") {
		t.Fatalf("expected changed words in output old=%q new=%q", stripANSI(out.OldLines[0]), stripANSI(out.NewLines[0]))
	}

	oldRanges, newRanges := changedWordRanges(normalizeDisplayText(rows[0].OldText), normalizeDisplayText(rows[0].NewText))
	if len(oldRanges) == 0 || len(newRanges) == 0 {
		t.Fatalf("expected non-empty changed-word ranges old=%v new=%v", oldRanges, newRanges)
	}
}

func TestRenderSplitShowsCommentMarkerOnBothPanes(t *testing.T) {
	rows := []DiffRow{
		{Kind: RowChange, Path: "a.txt", OldLine: intPtr(4), NewLine: intPtr(4), OldText: "old", NewText: "new"},
	}

	out := RenderSplitWithLayout(rows, 30, 30, 0, func(path string, line int, side Side) bool {
		return path == "a.txt" && line == 4 && side == SideNew
	})

	if !strings.HasPrefix(stripANSI(out.OldLines[0]), ">C ") {
		t.Fatalf("expected comment marker on old pane row, got %q", stripANSI(out.OldLines[0]))
	}
	if !strings.HasPrefix(stripANSI(out.NewLines[0]), ">C ") {
		t.Fatalf("expected comment marker on new pane row, got %q", stripANSI(out.NewLines[0]))
	}
}

func TestRenderSplitWithLayoutCursorStylingDoesNotChangeWrapWidth(t *testing.T) {
	rows := []DiffRow{
		{
			Kind:    RowAdd,
			Path:    "a.txt",
			NewLine: intPtr(7),
			NewText: "this is a fairly long wrapped line for cursor width checks",
		},
	}

	unstyled := RenderSplitWithLayout(rows, 24, 24, -1, nil)
	styled := RenderSplitWithLayout(rows, 24, 24, 0, func(path string, line int, side Side) bool {
		return path == "a.txt" && line == 7 && side == SideNew
	})

	if styled.RowHeights[0] != unstyled.RowHeights[0] {
		t.Fatalf("cursor/comment styling changed wrap height: styled=%d unstyled=%d", styled.RowHeights[0], unstyled.RowHeights[0])
	}
	for i, line := range styled.NewLines {
		if lipgloss.Width(line) != 24 {
			t.Fatalf("styled line %d visual width = %d, want 24: %q", i, lipgloss.Width(line), line)
		}
	}
}

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

func intPtr(n int) *int {
	v := n
	return &v
}
