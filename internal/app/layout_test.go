package app

import "testing"

func TestPaneWidthsWithSplitRightAndFilesPane(t *testing.T) {
	left, right := paneWidths(120, 40, false, true)
	if left != 40 || right != 75 {
		t.Fatalf("paneWidths(split) = (%d,%d), want (40,75)", left, right)
	}
}

func TestPaneWidthsWithSingleRightAndFilesPane(t *testing.T) {
	left, right := paneWidths(120, 40, false, false)
	if left != 40 || right != 76 {
		t.Fatalf("paneWidths(single) = (%d,%d), want (40,76)", left, right)
	}
}

func TestPaneWidthsWithSplitRightHiddenFiles(t *testing.T) {
	left, right := paneWidths(120, 40, true, true)
	if left != 0 || right != 117 {
		t.Fatalf("paneWidths(hidden split) = (%d,%d), want (0,117)", left, right)
	}
}

func TestPaneWidthsWithSingleRightHiddenFiles(t *testing.T) {
	left, right := paneWidths(120, 40, true, false)
	if left != 0 || right != 118 {
		t.Fatalf("paneWidths(hidden single) = (%d,%d), want (0,118)", left, right)
	}
}
