package app

func paneWidths(totalWidth int) (int, int) {
	if totalWidth < 20 {
		return totalWidth / 2, totalWidth - (totalWidth / 2)
	}

	left := totalWidth / 3
	right := totalWidth - left
	return left, right
}

func splitRightPanes(totalWidth int) (int, int) {
	if totalWidth <= 2 {
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
