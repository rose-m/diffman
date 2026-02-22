package diffview

import (
	"fmt"
	"strings"
)

type SplitRender struct {
	OldLines   []string
	NewLines   []string
	RowStarts  []int
	RowHeights []int
}

func RenderSplit(
	rows []DiffRow,
	oldWidth int,
	newWidth int,
	cursor int,
	hasComment func(path string, line int, side Side) bool,
) ([]string, []string) {
	out := RenderSplitWithLayout(rows, oldWidth, newWidth, cursor, hasComment)
	return out.OldLines, out.NewLines
}

func RenderSplitWithLayout(
	rows []DiffRow,
	oldWidth int,
	newWidth int,
	cursor int,
	hasComment func(path string, line int, side Side) bool,
) SplitRender {
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

	out := SplitRender{
		OldLines:   make([]string, 0, len(rows)),
		NewLines:   make([]string, 0, len(rows)),
		RowStarts:  make([]int, 0, len(rows)),
		RowHeights: make([]int, 0, len(rows)),
	}

	for i, row := range rows {
		oldSegs := renderRowSegments(row, SideOld, oldWidth, oldNumW, i == cursor, hasComment)
		newSegs := renderRowSegments(row, SideNew, newWidth, newNumW, i == cursor, hasComment)

		height := maxInt(len(oldSegs), len(newSegs))
		if height <= 0 {
			height = 1
		}

		out.RowStarts = append(out.RowStarts, len(out.OldLines))
		out.RowHeights = append(out.RowHeights, height)

		oldSegs = padSegments(oldSegs, oldWidth, height)
		newSegs = padSegments(newSegs, newWidth, height)
		out.OldLines = append(out.OldLines, oldSegs...)
		out.NewLines = append(out.NewLines, newSegs...)
	}

	return out
}

func renderRowSegments(row DiffRow, side Side, width, numW int, isCursor bool, hasComment func(path string, line int, side Side) bool) []string {
	cursorMark := " "
	if isCursor {
		cursorMark = ">"
	}

	commentMark := " "
	if hasCommentOnSide(row, side, hasComment) {
		commentMark = "*"
	}

	prefix := cursorMark + commentMark + " "
	contPrefix := "   "
	lineWidth := maxInt(1, width-len(prefix))

	var text string
	switch row.Kind {
	case RowFileHeader, RowHunkHeader:
		text = row.OldText
		if text == "" {
			text = row.NewText
		}
		text = normalizeDisplayText(text)

	default:
		lineNo, sideText, marker, ok := sideContent(row, side)
		if !ok {
			return []string{prefix + strings.Repeat(" ", lineWidth)}
		}
		num := ""
		if lineNo != nil {
			num = fmt.Sprintf("%d", *lineNo)
		}
		text = fmt.Sprintf("%c %*s %s", marker, numW, num, normalizeDisplayText(sideText))
	}

	chunks := wrapRunes(text, lineWidth)
	out := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		p := contPrefix
		if i == 0 {
			p = prefix
		}
		out = append(out, p+padRight(chunk, lineWidth))
	}
	if len(out) == 0 {
		out = append(out, prefix+strings.Repeat(" ", lineWidth))
	}
	return out
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

func normalizeDisplayText(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	if !strings.Contains(s, "\t") {
		return s
	}
	return expandTabs(s, 4)
}

func expandTabs(s string, tabSize int) string {
	if tabSize <= 0 {
		tabSize = 4
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' {
			b.WriteString(strings.Repeat(" ", tabSize))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func wrapRunes(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return []string{""}
	}
	out := make([]string, 0, len(runes)/width+1)
	for len(runes) > width {
		out = append(out, string(runes[:width]))
		runes = runes[width:]
	}
	out = append(out, string(runes))
	return out
}

func padSegments(segs []string, width, height int) []string {
	if len(segs) >= height {
		return segs
	}
	line := strings.Repeat(" ", maxInt(1, width))
	for len(segs) < height {
		segs = append(segs, line)
	}
	return segs
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
