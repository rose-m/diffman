package diffview

import (
	"fmt"
	"strings"
)

func RenderSplit(
	rows []DiffRow,
	oldWidth int,
	newWidth int,
	cursor int,
	hasComment func(path string, line int, side Side) bool,
) ([]string, []string) {
	if oldWidth <= 0 {
		oldWidth = 1
	}
	if newWidth <= 0 {
		newWidth = 1
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

	oldLines := make([]string, 0, len(rows))
	newLines := make([]string, 0, len(rows))
	for i, row := range rows {
		oldLines = append(oldLines, renderRowForSide(row, SideOld, oldWidth, oldNumW, i == cursor, hasComment))
		newLines = append(newLines, renderRowForSide(row, SideNew, newWidth, newNumW, i == cursor, hasComment))
	}
	return oldLines, newLines
}

func renderRowForSide(row DiffRow, side Side, width, numW int, isCursor bool, hasComment func(path string, line int, side Side) bool) string {
	cursorMark := " "
	if isCursor {
		cursorMark = ">"
	}

	commentMark := " "
	if hasCommentOnSide(row, side, hasComment) {
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
		return prefix + padRight(truncateRunes(header, lineWidth), lineWidth)
	}

	lineNo, text, marker, ok := sideContent(row, side)
	if !ok {
		return prefix + strings.Repeat(" ", lineWidth)
	}

	num := ""
	if lineNo != nil {
		num = fmt.Sprintf("%d", *lineNo)
	}
	base := fmt.Sprintf("%c %*s %s", marker, numW, num, text)
	return prefix + padRight(truncateRunes(base, lineWidth), lineWidth)
}

func sideContent(row DiffRow, side Side) (*int, string, rune, bool) {
	switch side {
	case SideOld:
		if row.OldLine == nil {
			return nil, "", ' ', false
		}
		marker := ' '
		if row.Kind == RowDelete || row.Kind == RowChange {
			marker = '-'
		}
		return row.OldLine, row.OldText, marker, true

	case SideNew:
		if row.NewLine == nil {
			return nil, "", ' ', false
		}
		marker := ' '
		if row.Kind == RowAdd || row.Kind == RowChange {
			marker = '+'
		}
		return row.NewLine, row.NewText, marker, true
	}

	return nil, "", ' ', false
}

func hasCommentOnSide(row DiffRow, side Side, hasComment func(path string, line int, side Side) bool) bool {
	if hasComment == nil {
		return false
	}

	if side == SideOld && row.OldLine != nil {
		return hasComment(row.Path, *row.OldLine, SideOld)
	}
	if side == SideNew && row.NewLine != nil {
		return hasComment(row.Path, *row.NewLine, SideNew)
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
