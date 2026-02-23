package app

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"

	"lediff/internal/diffview"
)

func TestDiffPaneModeNewOnly(t *testing.T) {
	m := Model{
		diffRows: []diffview.DiffRow{
			{Kind: diffview.RowHunkHeader, OldText: "@@ -0,0 +1,2 @@"},
			{Kind: diffview.RowAdd, NewLine: intPtr(1), NewText: "a"},
			{Kind: diffview.RowAdd, NewLine: intPtr(2), NewText: "b"},
		},
	}

	if got := m.diffPaneMode(); got != diffPaneModeNewOnly {
		t.Fatalf("diffPaneMode()=%v want %v", got, diffPaneModeNewOnly)
	}

	oldW, newW := m.diffSidePaneWidths(80)
	if oldW != 0 || newW != 80 {
		t.Fatalf("diffSidePaneWidths()=(%d,%d) want (0,80)", oldW, newW)
	}
}

func TestDiffPaneModeSplitWhenBothSidesPresent(t *testing.T) {
	m := Model{
		diffRows: []diffview.DiffRow{
			{Kind: diffview.RowChange, OldLine: intPtr(10), NewLine: intPtr(10), OldText: "x", NewText: "y"},
		},
	}

	if got := m.diffPaneMode(); got != diffPaneModeSplit {
		t.Fatalf("diffPaneMode()=%v want %v", got, diffPaneModeSplit)
	}
}

func TestRefreshDiffContentNewOnlyDoesNotInflateRowHeights(t *testing.T) {
	m := Model{
		diffRows: []diffview.DiffRow{
			{Kind: diffview.RowHunkHeader, OldText: "@@ -0,0 +1,2 @@"},
			{Kind: diffview.RowAdd, NewLine: intPtr(1), NewText: "package config"},
			{Kind: diffview.RowAdd, NewLine: intPtr(2), NewText: ""},
		},
		diffDirty: true,
	}
	m.oldView = viewport.New(1, 40)   // hidden old side width
	m.newView = viewport.New(100, 40) // visible new side width

	m.refreshDiffContent()

	if got := m.newView.TotalLineCount(); got > 8 {
		t.Fatalf("newView.TotalLineCount()=%d, expected compact rendering for new-only diff", got)
	}
}

func intPtr(v int) *int {
	n := v
	return &n
}
