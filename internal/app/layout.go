package app

func paneWidths(totalWidth int, desiredLeft int, hideLeft bool) (int, int) {
	if hideLeft {
		// Hidden file list: only split diff panes are visible.
		// Border overhead for split diff panes is 3 (left + divider + right).
		available := totalWidth - 3
		if available < 1 {
			return 0, 1
		}
		return 0, available
	}

	// Widths returned here are content widths, not outer widths.
	// Border overhead:
	//   files pane => 2 (left+right)
	//   split diff panes => 3 (outer left + shared divider + outer right)
	//   total border overhead = 5
	available := totalWidth - 5
	if available < 2 {
		return 1, 1
	}

	left := desiredLeft
	if left < 1 {
		left = 1
	}
	if left > available-1 {
		left = available - 1
	}
	right := available - left
	if right < 1 {
		right = 1
		left = available - right
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
