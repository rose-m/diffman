package app

func paneWidths(totalWidth int) (int, int) {
	if totalWidth < 20 {
		return totalWidth / 2, totalWidth - (totalWidth / 2)
	}

	left := totalWidth / 3
	right := totalWidth - left
	return left, right
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
