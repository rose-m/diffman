package app

func paneWidths(totalWidth int) (int, int) {
	// Widths returned here are content widths, not outer widths.
	// Border overhead:
	//   files pane => 2 (left+right)
	//   split diff panes => 3 (outer left + shared divider + outer right)
	//   total border overhead = 5
	available := totalWidth - 5
	if available < 2 {
		return 1, 1
	}
	left := available / 3
	if left < 1 {
		left = 1
	}
	right := available - left
	if right < 1 {
		right = 1
	}
	return left, right
}

func splitRightPanes(totalWidth int) (int, int) {
	if totalWidth <= 1 {
		return 1, 1
	}
	left := totalWidth / 2
	right := totalWidth - left
	if left < 1 {
		left = 1
	}
	if right < 1 {
		right = 1
	}
	return left, right
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
