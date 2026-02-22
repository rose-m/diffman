package diffview

import (
	"fmt"
	"strings"
)

func RenderSideBySide(rows []DiffRow, width int, cursor int, hasComment func(path string, line int, side Side) bool) []string {
	if width <= 0 {
		return []string{""}
	}

	maxOld := 0
	maxNew := 0
	for _, row := range rows {
		if row.OldLine != nil && *row.OldLine > maxOld {
			maxOld = *row.OldLine
		}
		if row.NewLine != nil && *row.NewLine > maxNew {
			maxNew = *row.NewLine
		}
	}
	oldNumW := maxInt(3, digits(maxOld))
	newNumW := maxInt(3, digits(maxNew))

	out := make([]string, 0, len(rows))
	for i, row := range rows {
		cursorMark := " "
		if i == cursor {
			cursorMark = ">"
		}

		commentMark := " "
		if hasRowComment(row, hasComment) {
			commentMark = "*"
		}

		prefix := cursorMark + commentMark + " "
		lineWidth := maxInt(1, width-len(prefix))
		switch row.Kind {
		case RowFileHeader, RowHunkHeader:
			header := row.OldText
			if header == "" {
				header = row.NewText
			}
			out = append(out, prefix+padRight(truncateRunes(header, lineWidth), lineWidth))

		default:
			sep := " | "
			bodyWidth := maxInt(1, lineWidth-len(sep))
			leftW := bodyWidth / 2
			rightW := bodyWidth - leftW
			left := renderSideCell(row.OldLine, oldNumW, row.OldText, leftW)
			right := renderSideCell(row.NewLine, newNumW, row.NewText, rightW)
			out = append(out, prefix+left+sep+right)
		}
	}
	return out
}

func renderSideCell(line *int, numW int, text string, width int) string {
	if width <= 0 {
		return ""
	}
	num := ""
	if line != nil {
		num = fmt.Sprintf("%d", *line)
	}
	base := fmt.Sprintf("%*s %s", numW, num, text)
	return padRight(truncateRunes(base, width), width)
}

func hasRowComment(row DiffRow, hasComment func(path string, line int, side Side) bool) bool {
	if hasComment == nil {
		return false
	}
	if row.OldLine != nil && hasComment(row.Path, *row.OldLine, SideOld) {
		return true
	}
	if row.NewLine != nil && hasComment(row.Path, *row.NewLine, SideNew) {
		return true
	}
	return false
}

func truncateRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}

func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}

func digits(n int) int {
	if n <= 0 {
		return 1
	}
	d := 0
	for n > 0 {
		d++
		n /= 10
	}
	return d
}
