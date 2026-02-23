package diffview

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

type SplitRender struct {
	OldLines   []string
	NewLines   []string
	RowStarts  []int
	RowHeights []int
}

type textRange struct {
	start int
	end   int
}

type wrappedChunk struct {
	text  string
	start int
}

type token struct {
	text    string
	isSpace bool
	start   int
	end     int
}

var (
	addBaseStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	deleteBaseStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	changeOldBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210"))
	changeNewBaseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("121"))
	contextBaseStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	hunkBaseStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true)

	addWordStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("121")).Background(lipgloss.Color("22")).Bold(true)
	deleteWordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("210")).Background(lipgloss.Color("52")).Bold(true)
	cursorRowBg     = lipgloss.Color("236")

	cursorGutterStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("45")).Bold(true)
	commentGutterStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("220")).Bold(true)
	cursorCommentGutterStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("201")).Bold(true)

	commentInlineTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("236"))
)

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
	return RenderSplitWithLayoutComments(rows, oldWidth, newWidth, cursor, hasComment, nil)
}

func RenderSplitWithLayoutComments(
	rows []DiffRow,
	oldWidth int,
	newWidth int,
	cursor int,
	hasComment func(path string, line int, side Side) bool,
	commentText func(path string, line int, side Side) (string, bool),
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
		oldMain := renderRowSegments(row, SideOld, oldWidth, oldNumW, i == cursor, hasComment)
		newMain := renderRowSegments(row, SideNew, newWidth, newNumW, i == cursor, hasComment)
		mainHeight := maxInt(len(oldMain), len(newMain))
		if mainHeight <= 0 {
			mainHeight = 1
		}
		oldSegs := padSegments(oldMain, oldWidth, mainHeight)
		newSegs := padSegments(newMain, newWidth, mainHeight)

		if commentText != nil {
			oldCommentBody, oldHasComment := commentTextForSide(row, SideOld, commentText)
			newCommentBody, newHasComment := commentTextForSide(row, SideNew, commentText)
			if oldHasComment || newHasComment {
				oldIndent := commentTextIndent(row, SideOld, oldNumW)
				newIndent := commentTextIndent(row, SideNew, newNumW)
				oldCommentSegs := []string{}
				if oldHasComment {
					oldCommentSegs = renderInlineCommentSegments(oldCommentBody, oldWidth, oldIndent)
				}
				newCommentSegs := []string{}
				if newHasComment {
					newCommentSegs = renderInlineCommentSegments(newCommentBody, newWidth, newIndent)
				}
				commentHeight := maxInt(len(oldCommentSegs), len(newCommentSegs))
				if commentHeight <= 0 {
					commentHeight = 1
				}
				oldCommentSegs = padCommentSegments(oldCommentSegs, oldWidth, commentHeight)
				newCommentSegs = padCommentSegments(newCommentSegs, newWidth, commentHeight)
				oldSegs = append(oldSegs, oldCommentSegs...)
				newSegs = append(newSegs, newCommentSegs...)
			}
		}

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

func renderRowSegments(
	row DiffRow,
	side Side,
	width, numW int,
	isCursor bool,
	hasComment func(path string, line int, side Side) bool,
) []string {
	hasAnyComment := hasCommentOnAnySide(row, hasComment)
	prefix := renderGutterPrefix(isCursor, hasAnyComment)
	contPrefix := "   "
	lineWidth := maxInt(1, width-lipgloss.Width(prefix))

	switch row.Kind {
	case RowHunkHeader:
		text := row.OldText
		if text == "" {
			text = row.NewText
		}
		text = normalizeDisplayText(text)
		chunks := wrapRunesWithOffsets(text, lineWidth)
		out := make([]string, 0, len(chunks))
		for i, chunk := range chunks {
			p := contPrefix
			if i == 0 {
				p = prefix
			}
			hstyle := hunkBaseStyle
			if isCursor {
				hstyle = hstyle.Background(cursorRowBg)
			}
			styled := hstyle.Render(chunk.text)
			out = append(out, p+styled+strings.Repeat(" ", lineWidth-len([]rune(chunk.text))))
		}
		if len(out) == 0 {
			out = append(out, prefix+strings.Repeat(" ", lineWidth))
		}
		return out

	case RowFileHeader:
		// File headers are no longer emitted by parser; keep safe behavior.
		return []string{prefix + strings.Repeat(" ", lineWidth)}
	}

	lineNo, sideText, marker, ok := sideContent(row, side)
	if !ok {
		return []string{prefix + strings.Repeat(" ", lineWidth)}
	}

	num := ""
	if lineNo != nil {
		num = fmt.Sprintf("%d", *lineNo)
	}
	meta := fmt.Sprintf("%c %*s ", marker, numW, num)
	metaWidth := len([]rune(meta))
	textWidth := maxInt(1, lineWidth-metaWidth)

	plainText := normalizeDisplayText(sideText)
	chunks := wrapRunesWithOffsets(plainText, textWidth)
	if len(chunks) == 0 {
		chunks = []wrappedChunk{{text: "", start: 0}}
	}

	baseStyle, highlightStyle := stylesForContent(row.Kind, side)
	if isCursor {
		baseStyle = baseStyle.Background(cursorRowBg)
	}
	changed := highlightRanges(row, side)

	out := make([]string, 0, len(chunks))
	firstStyled := styleChunk(chunks[0].text, chunks[0].start, changed, baseStyle, highlightStyle)
	metaStyled := styleMeta(meta, row.Kind, side, isCursor)
	out = append(out, prefix+metaStyled+firstStyled+strings.Repeat(" ", textWidth-len([]rune(chunks[0].text))))

	contMeta := strings.Repeat(" ", metaWidth)
	for _, chunk := range chunks[1:] {
		styled := styleChunk(chunk.text, chunk.start, changed, baseStyle, highlightStyle)
		out = append(out, contPrefix+contMeta+styled+strings.Repeat(" ", textWidth-len([]rune(chunk.text))))
	}

	return out
}

func renderGutterPrefix(isCursor, hasComment bool) string {
	cursorMark := " "
	if isCursor {
		cursorMark = ">"
	}
	commentMark := " "
	if hasComment {
		commentMark = "C"
	}
	marks := cursorMark + commentMark

	switch {
	case isCursor && hasComment:
		return cursorCommentGutterStyle.Render(marks) + " "
	case isCursor:
		return cursorGutterStyle.Render(marks) + " "
	case hasComment:
		return commentGutterStyle.Render(marks) + " "
	default:
		return marks + " "
	}
}

func styleMeta(meta string, kind RowKind, side Side, isCursor bool) string {
	base, _ := stylesForContent(kind, side)
	metaStyle := base.Bold(isCursor)
	if isCursor {
		metaStyle = metaStyle.Foreground(lipgloss.Color("230")).Background(cursorRowBg)
	}
	return metaStyle.Render(meta)
}

func stylesForContent(kind RowKind, side Side) (lipgloss.Style, lipgloss.Style) {
	switch kind {
	case RowAdd:
		return addBaseStyle, addWordStyle
	case RowDelete:
		return deleteBaseStyle, deleteWordStyle
	case RowChange:
		if side == SideOld {
			return changeOldBaseStyle, deleteWordStyle
		}
		return changeNewBaseStyle, addWordStyle
	default:
		return contextBaseStyle, contextBaseStyle
	}
}

func highlightRanges(row DiffRow, side Side) []textRange {
	if row.Kind != RowChange {
		return nil
	}
	oldText := normalizeDisplayText(row.OldText)
	newText := normalizeDisplayText(row.NewText)
	oldRanges, newRanges := changedWordRanges(oldText, newText)
	if side == SideOld {
		return oldRanges
	}
	return newRanges
}

func styleChunk(text string, chunkStart int, ranges []textRange, baseStyle, highlightStyle lipgloss.Style) string {
	if text == "" {
		return ""
	}
	if len(ranges) == 0 {
		return baseStyle.Render(text)
	}

	runes := []rune(text)
	var b strings.Builder
	start := 0
	inHighlight := inRanges(chunkStart, ranges)
	for i := 1; i <= len(runes); i++ {
		if i == len(runes) || inRanges(chunkStart+i, ranges) != inHighlight {
			seg := string(runes[start:i])
			if inHighlight {
				b.WriteString(highlightStyle.Render(seg))
			} else {
				b.WriteString(baseStyle.Render(seg))
			}
			start = i
			if i < len(runes) {
				inHighlight = !inHighlight
			}
		}
	}
	return b.String()
}

func inRanges(pos int, ranges []textRange) bool {
	for _, r := range ranges {
		if pos >= r.start && pos < r.end {
			return true
		}
	}
	return false
}

func changedWordRanges(oldText, newText string) ([]textRange, []textRange) {
	oldTokens := tokenizeWords(oldText)
	newTokens := tokenizeWords(newText)

	oldWords, oldWordTokenIdx := extractWords(oldTokens)
	newWords, newWordTokenIdx := extractWords(newTokens)

	matchedOld, matchedNew := lcsMatches(oldWords, newWords)

	oldRanges := make([]textRange, 0)
	for wordIdx, tokIdx := range oldWordTokenIdx {
		if matchedOld[wordIdx] {
			continue
		}
		tok := oldTokens[tokIdx]
		oldRanges = append(oldRanges, textRange{start: tok.start, end: tok.end})
	}

	newRanges := make([]textRange, 0)
	for wordIdx, tokIdx := range newWordTokenIdx {
		if matchedNew[wordIdx] {
			continue
		}
		tok := newTokens[tokIdx]
		newRanges = append(newRanges, textRange{start: tok.start, end: tok.end})
	}

	return mergeRanges(oldRanges), mergeRanges(newRanges)
}

func tokenizeWords(s string) []token {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}

	tokens := make([]token, 0, len(runes)/2)
	start := 0
	currentSpace := unicode.IsSpace(runes[0])
	for i := 1; i < len(runes); i++ {
		isSpace := unicode.IsSpace(runes[i])
		if isSpace == currentSpace {
			continue
		}
		tokens = append(tokens, token{text: string(runes[start:i]), isSpace: currentSpace, start: start, end: i})
		start = i
		currentSpace = isSpace
	}
	tokens = append(tokens, token{text: string(runes[start:]), isSpace: currentSpace, start: start, end: len(runes)})
	return tokens
}

func extractWords(tokens []token) ([]string, []int) {
	words := make([]string, 0, len(tokens))
	indices := make([]int, 0, len(tokens))
	for i, tok := range tokens {
		if tok.isSpace {
			continue
		}
		words = append(words, tok.text)
		indices = append(indices, i)
	}
	return words, indices
}

func lcsMatches(a, b []string) (map[int]bool, map[int]bool) {
	n := len(a)
	m := len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	matchedA := make(map[int]bool)
	matchedB := make(map[int]bool)
	i, j := 0, 0
	for i < n && j < m {
		if a[i] == b[j] {
			matchedA[i] = true
			matchedB[j] = true
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			i++
		} else {
			j++
		}
	}

	return matchedA, matchedB
}

func mergeRanges(in []textRange) []textRange {
	if len(in) == 0 {
		return nil
	}
	out := make([]textRange, 0, len(in))
	cur := in[0]
	for _, r := range in[1:] {
		if r.start <= cur.end {
			if r.end > cur.end {
				cur.end = r.end
			}
			continue
		}
		out = append(out, cur)
		cur = r
	}
	out = append(out, cur)
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

func hasCommentOnAnySide(row DiffRow, hasComment func(path string, line int, side Side) bool) bool {
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

func commentTextForSide(row DiffRow, side Side, commentText func(path string, line int, side Side) (string, bool)) (string, bool) {
	if commentText == nil {
		return "", false
	}
	switch side {
	case SideOld:
		if row.OldLine == nil {
			return "", false
		}
		return commentText(row.Path, *row.OldLine, SideOld)
	case SideNew:
		if row.NewLine == nil {
			return "", false
		}
		return commentText(row.Path, *row.NewLine, SideNew)
	default:
		return "", false
	}
}

func commentTextIndent(row DiffRow, side Side, numW int) int {
	lineNo, _, marker, ok := sideContent(row, side)
	if !ok {
		meta := fmt.Sprintf("%c %*s ", ' ', numW, "")
		return 3 + len([]rune(meta))
	}
	num := ""
	if lineNo != nil {
		num = fmt.Sprintf("%d", *lineNo)
	}
	meta := fmt.Sprintf("%c %*s ", marker, numW, num)
	return 3 + len([]rune(meta))
}

func renderInlineCommentSegments(commentBody string, width, indent int) []string {
	if width <= 0 {
		width = 1
	}
	if indent < 0 {
		indent = 0
	}
	if indent >= width {
		indent = width - 1
	}
	textWidth := maxInt(1, width-indent)
	lines := strings.Split(commentBody, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		plain := normalizeDisplayText(line)
		chunks := wrapRunesWithOffsets(plain, textWidth)
		if len(chunks) == 0 {
			chunks = []wrappedChunk{{text: "", start: 0}}
		}
		for _, chunk := range chunks {
			raw := strings.Repeat(" ", indent) + chunk.text
			pad := width - len([]rune(raw))
			if pad < 0 {
				pad = 0
			}
			out = append(out, commentInlineTextStyle.Render(raw+strings.Repeat(" ", pad)))
		}
	}
	if len(out) == 0 {
		out = append(out, commentInlineTextStyle.Render(strings.Repeat(" ", width)))
	}
	return out
}

func padCommentSegments(segs []string, width, height int) []string {
	if len(segs) >= height {
		return segs
	}
	line := commentInlineTextStyle.Render(strings.Repeat(" ", maxInt(1, width)))
	for len(segs) < height {
		segs = append(segs, line)
	}
	return segs
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

func wrapRunesWithOffsets(s string, width int) []wrappedChunk {
	if width <= 0 {
		return []wrappedChunk{{text: "", start: 0}}
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return []wrappedChunk{{text: "", start: 0}}
	}

	out := make([]wrappedChunk, 0, len(runes)/width+1)
	for start := 0; start < len(runes); start += width {
		end := start + width
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, wrappedChunk{text: string(runes[start:end]), start: start})
	}
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
