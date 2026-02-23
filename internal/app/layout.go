package app

func paneWidths(totalWidth int, desiredLeft int, hideLeft bool, splitRight bool) (int, int) {
	if hideLeft {
		// Hidden file list: only diff pane(s) are visible.
		// Border overhead is:
		//   split right panes => 3 (left + divider + right)
		//   single right pane => 2 (left + right)
		overhead := 2
		if splitRight {
			overhead = 3
		}
		available := totalWidth - overhead
		if available < 1 {
			return 0, 1
		}
		return 0, available
	}

	// Widths returned here are content widths, not outer widths.
	// Border overhead:
	//   files pane => 2 (left+right)
	//   right pane area =>
	//      split diff panes => 3 (outer left + shared divider + outer right)
	//      single diff pane => 2 (outer left + outer right)
	overhead := 4
	if splitRight {
		overhead = 5
	}
	available := totalWidth - overhead
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
