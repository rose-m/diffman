package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lediff/internal/clipboard"
	"lediff/internal/comments"
	"lediff/internal/diffview"
	gitint "lediff/internal/git"
)

type focusPane int

const (
	focusFiles focusPane = iota
	focusDiff
)

const (
	filePaneWidthDefault = 40
	filePaneWidthWide    = 120
)

type filesLoadedMsg struct {
	items []gitint.FileItem
	err   error
}

type diffLoadedMsg struct {
	path  string
	rows  []diffview.DiffRow
	empty bool
	err   error
}

type clipboardResultMsg struct {
	err error
}

type commentAnchor struct {
	Path   string
	Side   comments.Side
	Line   int
	RowIdx int
}

// Model is the Bubble Tea state container for the app.
type Model struct {
	keys      KeyMap
	focus     focusPane
	cwd       string
	diffMode  gitint.DiffMode
	statusSvc gitint.StatusService
	diffSvc   gitint.DiffService

	width  int
	height int
	ready  bool

	fileItems []gitint.FileItem
	selected  int
	selectedF string
	filePaneW int

	diffRows   []diffview.DiffRow
	diffCursor int
	rowStarts  []int
	rowHeights []int
	oldView    viewport.Model
	newView    viewport.Model
	helpOpen   bool
	diffDirty  bool
	oldWidth   int
	newWidth   int

	commentStore       comments.Store
	comments           map[string]comments.Comment
	commentInputActive bool
	commentInput       string
	commentInputErr    string
	commentEditAnchor  *commentAnchor

	alertMsg string

	loadingFiles bool
	loadingDiff  bool
	err          error
}

func NewModel() (Model, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Model{}, err
	}

	gitDir, err := gitint.DiscoverGitDir(context.Background(), cwd)
	if err != nil {
		return Model{}, err
	}

	store := comments.NewStore(gitDir)
	loadedComments, loadErr := store.Load()
	commentMap := make(map[string]comments.Comment, len(loadedComments))
	for _, c := range loadedComments {
		commentMap[comments.AnchorKey(c.Path, c.Side, c.Line)] = c
	}

	m := Model{
		keys:         defaultKeyMap(),
		focus:        focusFiles,
		cwd:          cwd,
		diffMode:     gitint.DiffModeAll,
		statusSvc:    gitint.NewStatusService(),
		diffSvc:      gitint.NewDiffService(),
		helpOpen:     false,
		filePaneW:    filePaneWidthDefault,
		commentStore: store,
		comments:     commentMap,
		diffDirty:    true,
		oldWidth:     -1,
		newWidth:     -1,
	}
	if loadErr != nil {
		m.alertMsg = fmt.Sprintf("failed to load comments: %v", loadErr)
	}

	m.oldView = viewport.New(1, 1)
	m.newView = viewport.New(1, 1)
	m.oldView.SetContent("Select a file to load its diff.")
	m.newView.SetContent("Select a file to load its diff.")
	return m, nil
}

func (m Model) Init() tea.Cmd {
	m.loadingFiles = true
	return m.loadFilesCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.resizePanes()
		m.diffDirty = true
		m.refreshDiffContent()
		return m, nil

	case filesLoadedMsg:
		m.loadingFiles = false
		m.err = msg.err
		m.fileItems = msg.items
		if len(m.fileItems) == 0 {
			m.selected = 0
			m.selectedF = ""
			m.diffRows = nil
			m.diffCursor = 0
			m.rowStarts = nil
			m.rowHeights = nil
			m.diffDirty = false
			m.oldView.GotoTop()
			m.newView.GotoTop()
			m.oldView.SetContent("No changed files found in this repository.")
			m.newView.SetContent("No changed files found in this repository.")
			return m, nil
		}

		if m.selected >= len(m.fileItems) {
			m.selected = len(m.fileItems) - 1
		}
		m.selectedF = m.fileItems[m.selected].Path
		return m, m.loadDiffCmd(m.selectedF)

	case diffLoadedMsg:
		m.loadingDiff = false
		m.err = msg.err
		if msg.err != nil {
			m.diffRows = nil
			m.rowStarts = nil
			m.rowHeights = nil
			m.diffDirty = false
			errMsg := fmt.Sprintf("Failed to load diff for %s:\n%v", msg.path, msg.err)
			m.oldView.SetContent(errMsg)
			m.newView.SetContent(errMsg)
			return m, nil
		}
		if msg.empty || len(msg.rows) == 0 {
			m.diffRows = nil
			m.diffCursor = 0
			m.rowStarts = nil
			m.rowHeights = nil
			m.diffDirty = false
			noDiff := fmt.Sprintf("No diff for %s.", msg.path)
			m.oldView.SetContent(noDiff)
			m.newView.SetContent(noDiff)
			return m, nil
		}
		m.diffRows = msg.rows
		m.diffCursor = firstRenderableRow(m.diffRows)
		m.diffDirty = true
		m.refreshDiffContent()
		return m, nil

	case clipboardResultMsg:
		if msg.err != nil {
			m.alertMsg = fmt.Sprintf("export failed: %v", msg.err)
			return m, nil
		}
		m.alertMsg = "Copied comments export to clipboard."
		return m, nil

	case tea.KeyMsg:
		if m.commentInputActive {
			return m.handleCommentInput(msg)
		}
		if m.alertMsg != "" {
			m.alertMsg = ""
			return m, nil
		}

		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.ToggleFocus) {
			if m.focus == focusFiles {
				m.focus = focusDiff
			} else {
				m.focus = focusFiles
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.FocusFiles) {
			m.focus = focusFiles
			return m, nil
		}
		if key.Matches(msg, m.keys.FocusDiff) {
			m.focus = focusDiff
			return m, nil
		}
		if key.Matches(msg, m.keys.Help) {
			m.helpOpen = !m.helpOpen
			return m, nil
		}
		if key.Matches(msg, m.keys.Refresh) {
			m.loadingFiles = true
			return m, m.loadFilesCmd()
		}
		if key.Matches(msg, m.keys.ToggleMode) {
			m.advanceDiffMode()
			if m.selectedF != "" {
				m.loadingDiff = true
				return m, m.loadDiffCmd(m.selectedF)
			}
			return m, nil
		}

		if m.focus == focusFiles {
			return m.updateFilesPane(msg)
		}
		return m.updateDiffPane(msg)
	}

	return m, nil
}

func (m Model) updateFilesPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ToggleFiles) {
		if m.filePaneW == filePaneWidthWide {
			m.filePaneW = filePaneWidthDefault
		} else {
			m.filePaneW = filePaneWidthWide
		}
		m.diffDirty = true
		m.resizePanes()
		return m, nil
	}

	if len(m.fileItems) == 0 {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		prev := m.selected
		if m.selected > 0 {
			m.selected--
		}
		m.selectedF = m.fileItems[m.selected].Path
		if m.selected != prev {
			m.loadingDiff = true
			return m, m.loadDiffCmd(m.selectedF)
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		prev := m.selected
		if m.selected < len(m.fileItems)-1 {
			m.selected++
		}
		m.selectedF = m.fileItems[m.selected].Path
		if m.selected != prev {
			m.loadingDiff = true
			return m, m.loadDiffCmd(m.selectedF)
		}
		return m, nil

	case key.Matches(msg, m.keys.Open):
		m.selectedF = m.fileItems[m.selected].Path
		m.loadingDiff = true
		m.focus = focusDiff
		return m, m.loadDiffCmd(m.selectedF)

	}

	return m, nil
}

func (m Model) updateDiffPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.diffRows) == 0 {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveDiffCursor(-1)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.moveDiffCursor(1)
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.diffCursor = 0
		m.diffDirty = true
		m.refreshDiffContent()
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.diffCursor = len(m.diffRows) - 1
		m.diffDirty = true
		m.refreshDiffContent()
		return m, nil

	case key.Matches(msg, m.keys.Create):
		m.startCommentEdit(false)
		return m, nil

	case key.Matches(msg, m.keys.Edit):
		m.startCommentEdit(true)
		return m, nil

	case key.Matches(msg, m.keys.Delete):
		m.deleteCommentAtCursor()
		return m, nil

	case key.Matches(msg, m.keys.NextComment):
		m.jumpToComment(1)
		return m, nil

	case key.Matches(msg, m.keys.PrevComment):
		m.jumpToComment(-1)
		return m, nil

	case key.Matches(msg, m.keys.Export):
		if len(m.comments) == 0 {
			m.alertMsg = "No comments to export."
			return m, nil
		}
		return m, m.exportCommentsCmd()
	}
	return m, nil
}

func (m Model) handleCommentInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.commentInputActive = false
		m.commentInput = ""
		m.commentInputErr = ""
		m.commentEditAnchor = nil
		return m, nil

	case tea.KeyEnter:
		m.saveCommentInput()
		return m, nil

	case tea.KeyBackspace, tea.KeyCtrlH:
		m.commentInput = removeLastRune(m.commentInput)
		m.commentInputErr = ""
		return m, nil

	case tea.KeySpace:
		m.commentInput += " "
		m.commentInputErr = ""
		return m, nil

	case tea.KeyRunes:
		m.commentInput += msg.String()
		m.commentInputErr = ""
		return m, nil
	}

	return m, nil
}

func (m *Model) saveCommentInput() {
	if m.commentEditAnchor == nil {
		m.commentInputActive = false
		m.commentInput = ""
		m.commentInputErr = ""
		return
	}

	body := strings.TrimSpace(m.commentInput)
	if body == "" {
		m.commentInputErr = "Comment text is empty."
		return
	}

	anchor := *m.commentEditAnchor
	key := comments.AnchorKey(anchor.Path, anchor.Side, anchor.Line)
	existing, exists := m.comments[key]
	createdAt := time.Now()
	if exists {
		createdAt = existing.CreatedAt
	}

	contextBefore, contextAfter := m.contextAround(anchor)
	m.comments[key] = comments.Comment{
		Path:          anchor.Path,
		Side:          anchor.Side,
		Line:          anchor.Line,
		Body:          body,
		CreatedAt:     createdAt,
		HunkHeader:    m.hunkHeaderForRow(anchor.RowIdx, anchor.Path),
		ContextBefore: contextBefore,
		ContextAfter:  contextAfter,
	}

	if err := m.persistComments(); err != nil {
		m.alertMsg = fmt.Sprintf("failed to save comments: %v", err)
		return
	}

	m.commentInputActive = false
	m.commentInput = ""
	m.commentInputErr = ""
	m.commentEditAnchor = nil
	m.diffDirty = true
	m.refreshDiffContent()
}

func (m *Model) startCommentEdit(requireExisting bool) {
	anchor, ok := m.currentAnchor()
	if !ok {
		m.alertMsg = "No commentable line selected."
		return
	}

	key := comments.AnchorKey(anchor.Path, anchor.Side, anchor.Line)
	existing, exists := m.comments[key]
	if requireExisting && !exists {
		m.alertMsg = "No comment exists on selected line."
		return
	}

	m.commentInputActive = true
	m.commentInput = ""
	m.commentInputErr = ""
	if exists {
		m.commentInput = existing.Body
	}
	a := anchor
	m.commentEditAnchor = &a
}

func (m *Model) deleteCommentAtCursor() {
	anchor, ok := m.currentAnchor()
	if !ok {
		m.alertMsg = "No commentable line selected."
		return
	}

	key := comments.AnchorKey(anchor.Path, anchor.Side, anchor.Line)
	if _, exists := m.comments[key]; !exists {
		m.alertMsg = "No comment exists on selected line."
		return
	}
	delete(m.comments, key)

	if err := m.persistComments(); err != nil {
		m.alertMsg = fmt.Sprintf("failed to save comments: %v", err)
		return
	}

	m.diffDirty = true
	m.refreshDiffContent()
}

func (m *Model) jumpToComment(direction int) {
	rows := m.commentRowIndices()
	if len(rows) == 0 {
		m.alertMsg = "No comments in current diff."
		return
	}

	next := rows[0]
	if direction < 0 {
		next = rows[len(rows)-1]
	}
	for _, idx := range rows {
		if direction > 0 && idx > m.diffCursor {
			next = idx
			break
		}
	}
	if direction < 0 {
		for i := len(rows) - 1; i >= 0; i-- {
			if rows[i] < m.diffCursor {
				next = rows[i]
				break
			}
		}
	}

	m.diffCursor = next
	m.diffDirty = true
	m.refreshDiffContent()
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	help := "tab focus | h/l focus panes | j/k move | enter open diff | z file width | t mode | c/e/d comment | n/p comment nav | y export | r refresh | ? help | q quit"
	if m.helpOpen {
		help = strings.Join([]string{
			"Global: q quit, tab switch focus, h files focus, l diff focus, t toggle diff mode, ? toggle help",
			"Files pane: j/k move, enter open diff, z toggle file pane width, r refresh",
			"Diff pane: j/k move cursor, g/G top/bottom",
			"Comments: c create, e edit, d delete, n/p next/prev, y export to clipboard",
		}, "\n")
	}

	footer := truncateLinesToWidth(help, m.width)
	footerHeight := lineCount(footer)

	leftW, rightW := paneWidths(m.width, m.filePaneW)
	oldPaneW, newPaneW := splitRightPanes(rightW)
	// lipgloss Height applies to content height; borders add 2 more rows.
	paneContentHeight := max(1, m.height-footerHeight-2)
	newOldWidth := max(1, oldPaneW)
	newNewWidth := max(1, newPaneW)
	if m.oldView.Width != newOldWidth || m.newView.Width != newNewWidth {
		m.diffDirty = true
	}
	m.oldView.Width = newOldWidth
	m.newView.Width = newNewWidth
	m.oldView.Height = max(1, paneContentHeight-4)
	m.newView.Height = max(1, paneContentHeight-4)
	m.refreshDiffContent()

	leftPane := m.renderFilesPane(leftW, paneContentHeight)
	rightPane := m.renderDiffPanes(oldPaneW, newPaneW, paneContentHeight)
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	contentAreaHeight := paneContentHeight + 2
	if m.commentInputActive {
		content = overlayCentered(content, m.renderCommentModal(), m.width, contentAreaHeight)
	} else if m.alertMsg != "" {
		content = overlayCentered(content, m.renderAlertModal(), m.width, contentAreaHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m Model) renderCommentModal() string {
	title := "Add Comment"
	if m.commentEditAnchor != nil {
		key := comments.AnchorKey(m.commentEditAnchor.Path, m.commentEditAnchor.Side, m.commentEditAnchor.Line)
		if _, exists := m.comments[key]; exists {
			title = "Edit Comment"
		}
	}

	innerW := max(10, m.modalWidth()-8)
	inputBox := lipgloss.NewStyle().
		Width(innerW).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Render(m.renderCommentInputLine(innerW))
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Enter save | Esc cancel | Backspace delete")

	bodyLines := []string{inputBox, "", hint}
	if m.commentInputErr != "" {
		bodyLines = append(bodyLines, "")
		bodyLines = append(bodyLines, lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("Error: "+m.commentInputErr))
	}

	body := strings.Join(bodyLines, "\n")
	return m.renderModalCard(title, lipgloss.Color("39"), lipgloss.Color("39"), body)
}

func (m Model) renderAlertModal() string {
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Press any key to dismiss")
	body := strings.Join([]string{
		m.alertMsg,
		"",
		hint,
	}, "\n")
	return m.renderModalCard("Notice", lipgloss.Color("220"), lipgloss.Color("220"), body)
}

func (m Model) renderCommentInputLine(width int) string {
	if width < 2 {
		width = 2
	}
	cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("51")).Render(" ")
	if m.commentInput == "" {
		placeholderRaw := ansi.Truncate("(type comment)", width-1, "")
		placeholder := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(placeholderRaw)
		return placeholder + cursor
	}
	raw := ansi.Truncate(m.commentInput, width-1, "")
	return raw + cursor
}

func (m Model) modalWidth() int {
	w := m.width - 8
	if w > 100 {
		w = 100
	}
	if w < 30 {
		w = m.width - 2
	}
	if w < 10 {
		w = 10
	}
	return w
}

func (m Model) renderModalCard(title string, titleColor, borderColor lipgloss.Color, body string) string {
	outerW := m.modalWidth()
	contentW := max(10, outerW-2)
	titleText := ansi.Truncate(title, max(1, contentW-2), "")
	titleBar := lipgloss.NewStyle().
		Width(contentW).
		Padding(0, 1).
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(titleColor).
		Render(titleText)

	bodyBlock := lipgloss.NewStyle().
		Width(contentW).
		Padding(1, 2).
		Render(body)

	return lipgloss.NewStyle().
		Width(contentW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Background(lipgloss.Color("235")).
		Render(titleBar + "\n" + bodyBlock)
}

func overlayCentered(base, overlay string, width, height int) string {
	baseLines := normalizeCanvas(base, width, height)
	overlayLines := strings.Split(overlay, "\n")
	overlayW := lipgloss.Width(overlay)
	overlayH := len(overlayLines)
	if overlayW <= 0 || overlayH <= 0 {
		return strings.Join(baseLines, "\n")
	}

	x := max(0, (width-overlayW)/2)
	y := max(0, (height-overlayH)/2)
	for i, ol := range overlayLines {
		row := y + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		baseLines[row] = overlayLine(baseLines[row], ol, x, overlayW, width)
	}
	return strings.Join(baseLines, "\n")
}

func normalizeCanvas(s string, width, height int) []string {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	raw := strings.Split(s, "\n")
	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		line := ""
		if i < len(raw) {
			line = raw[i]
		}
		w := lipgloss.Width(line)
		switch {
		case w > width:
			lines = append(lines, ansi.Truncate(line, width, ""))
		case w < width:
			lines = append(lines, line+strings.Repeat(" ", width-w))
		default:
			lines = append(lines, line)
		}
	}
	return lines
}

func overlayLine(baseLine, overlayLine string, x, overlayW, totalW int) string {
	if overlayW <= 0 {
		return baseLine
	}
	if x < 0 {
		x = 0
	}
	if x >= totalW {
		return baseLine
	}
	if x+overlayW > totalW {
		overlayLine = ansi.Truncate(overlayLine, totalW-x, "")
		overlayW = lipgloss.Width(overlayLine)
		if overlayW <= 0 {
			return baseLine
		}
	}

	plain := []rune(ansi.Strip(baseLine))
	if len(plain) < totalW {
		plain = append(plain, []rune(strings.Repeat(" ", totalW-len(plain)))...)
	}
	left := string(plain[:x])
	rightStart := x + overlayW
	if rightStart > len(plain) {
		rightStart = len(plain)
	}
	right := string(plain[rightStart:])
	return left + overlayLine + right
}

func (m Model) renderFilesPane(width, height int) string {
	border := lipgloss.NormalBorder()
	borderColor := lipgloss.Color("245")
	if m.focus == focusFiles {
		borderColor = lipgloss.Color("39")
	}

	paneStyle := lipgloss.NewStyle().
		Width(max(1, width)).
		Height(max(1, height)).
		Border(border).
		BorderForeground(borderColor)

	title := fmt.Sprintf("Files (%d)", len(m.fileItems))
	if m.loadingFiles {
		title += " (loading...)"
	}

	innerW := max(1, width)
	bodyLines := make([]string, 0, len(m.fileItems)+2)
	bodyLines = append(bodyLines, title)
	bodyLines = append(bodyLines, "")

	if len(m.fileItems) == 0 {
		bodyLines = append(bodyLines, "No changed files")
	} else {
		for i, item := range m.fileItems {
			prefix := "  "
			if i == m.selected {
				prefix = "> "
			}
			line := fmt.Sprintf("%s[%s] %s", prefix, item.Status, item.Path)
			lineStyle := lipgloss.NewStyle().Width(innerW).MaxWidth(innerW)
			if i == m.selected {
				lineStyle = lineStyle.Foreground(lipgloss.Color("39")).Bold(true)
			}
			bodyLines = append(bodyLines, lineStyle.Render(line))
		}
	}

	if m.err != nil {
		bodyLines = append(bodyLines, "")
		bodyLines = append(bodyLines, fmt.Sprintf("error: %v", m.err))
	}

	return paneStyle.Render(strings.Join(bodyLines, "\n"))
}

func (m Model) renderDiffPanes(oldWidth, newWidth, height int) string {
	oldPane := m.renderDiffSidePane(oldWidth, height, "Old", m.oldView.View(), false)
	newPane := m.renderDiffSidePane(newWidth, height, "New", m.newView.View(), true)
	return lipgloss.JoinHorizontal(lipgloss.Top, oldPane, newPane)
}

func (m Model) renderDiffSidePane(width, height int, sideLabel, body string, withRightBorder bool) string {
	border := lipgloss.NormalBorder()
	borderColor := lipgloss.Color("245")
	if m.focus == focusDiff {
		borderColor = lipgloss.Color("39")
	}

	paneStyle := lipgloss.NewStyle().
		Width(max(1, width)).
		Height(max(1, height)).
		Border(border, true, withRightBorder, true, true).
		BorderForeground(borderColor)

	title := sideLabel
	if m.selectedF != "" {
		title = sideLabel + ": " + m.selectedF
	}
	title += fmt.Sprintf(" [%s]", m.diffMode.String())
	if m.loadingDiff {
		title += " (loading...)"
	}

	innerW := max(1, width)
	header := lipgloss.NewStyle().Bold(true).Width(innerW).MaxWidth(innerW).Render(title)

	return paneStyle.Render(header + "\n\n" + body)
}

func (m *Model) resizePanes() {
	_, rightW := paneWidths(m.width, m.filePaneW)
	oldPaneW, newPaneW := splitRightPanes(rightW)
	m.oldView.Width = max(1, oldPaneW)
	m.newView.Width = max(1, newPaneW)
	m.oldView.Height = max(1, m.height-6)
	m.newView.Height = max(1, m.height-6)
	m.diffDirty = true
}

func (m Model) loadFilesCmd() tea.Cmd {
	cwd := m.cwd
	service := m.statusSvc
	return func() tea.Msg {
		items, err := service.ListChangedFiles(context.Background(), cwd)
		return filesLoadedMsg{items: items, err: err}
	}
}

func (m Model) loadDiffCmd(path string) tea.Cmd {
	cwd := m.cwd
	service := m.diffSvc
	mode := m.diffMode
	return func() tea.Msg {
		d, err := service.Diff(context.Background(), cwd, path, mode)
		if err != nil {
			return diffLoadedMsg{path: path, err: err}
		}
		if strings.TrimSpace(d) == "" {
			return diffLoadedMsg{path: path, empty: true}
		}

		rows, err := diffview.ParseUnifiedDiff([]byte(d))
		if err != nil {
			return diffLoadedMsg{path: path, err: err}
		}
		return diffLoadedMsg{path: path, rows: rows}
	}
}

func (m Model) exportCommentsCmd() tea.Cmd {
	snapshot := m.sortedComments()
	mode := m.diffMode
	return func() tea.Msg {
		title := fmt.Sprintf("Review comments (%s diff mode):", mode.String())
		text := comments.ExportPlain(snapshot, title)
		err := clipboard.CopyText(context.Background(), text)
		return clipboardResultMsg{err: err}
	}
}

func (m *Model) moveDiffCursor(delta int) {
	if len(m.diffRows) == 0 {
		m.diffCursor = 0
		return
	}
	m.diffCursor += delta
	if m.diffCursor < 0 {
		m.diffCursor = 0
	}
	if m.diffCursor >= len(m.diffRows) {
		m.diffCursor = len(m.diffRows) - 1
	}
	m.diffDirty = true
	m.refreshDiffContent()
}

func (m *Model) advanceDiffMode() {
	switch m.diffMode {
	case gitint.DiffModeAll:
		m.diffMode = gitint.DiffModeUnstaged
	case gitint.DiffModeUnstaged:
		m.diffMode = gitint.DiffModeStaged
	default:
		m.diffMode = gitint.DiffModeAll
	}
	m.diffDirty = true
}

func (m *Model) refreshDiffContent() {
	if len(m.diffRows) == 0 {
		return
	}
	m.clampDiffCursor()
	if !m.diffDirty && m.oldWidth == m.oldView.Width && m.newWidth == m.newView.Width {
		m.ensureCursorVisible()
		return
	}

	rendered := diffview.RenderSplitWithLayout(
		m.diffRows,
		m.oldView.Width,
		m.newView.Width,
		m.diffCursor,
		func(path string, line int, side diffview.Side) bool {
			return m.hasComment(path, line, side)
		},
	)
	m.oldView.SetContent(strings.Join(rendered.OldLines, "\n"))
	m.newView.SetContent(strings.Join(rendered.NewLines, "\n"))
	m.rowStarts = rendered.RowStarts
	m.rowHeights = rendered.RowHeights
	m.oldWidth = m.oldView.Width
	m.newWidth = m.newView.Width
	m.diffDirty = false
	m.ensureCursorVisible()
}

func (m *Model) ensureCursorVisible() {
	visibleHeight := m.oldView.Height
	if m.newView.Height < visibleHeight {
		visibleHeight = m.newView.Height
	}
	if visibleHeight <= 0 {
		return
	}
	start, end := m.cursorVisualRange()
	if start < m.oldView.YOffset {
		m.oldView.SetYOffset(start)
		m.newView.SetYOffset(start)
		return
	}
	bottom := m.oldView.YOffset + visibleHeight - 1
	if end > bottom {
		next := end - visibleHeight + 1
		m.oldView.SetYOffset(next)
		m.newView.SetYOffset(next)
	}
}

func (m *Model) cursorVisualRange() (int, int) {
	if len(m.rowStarts) != len(m.diffRows) || len(m.rowHeights) != len(m.diffRows) {
		return m.diffCursor, m.diffCursor
	}
	if m.diffCursor < 0 || m.diffCursor >= len(m.rowStarts) {
		return 0, 0
	}
	start := m.rowStarts[m.diffCursor]
	height := m.rowHeights[m.diffCursor]
	if height <= 0 {
		height = 1
	}
	return start, start + height - 1
}

func (m *Model) clampDiffCursor() {
	if len(m.diffRows) == 0 {
		m.diffCursor = 0
		return
	}
	if m.diffCursor < 0 {
		m.diffCursor = 0
	}
	if m.diffCursor >= len(m.diffRows) {
		m.diffCursor = len(m.diffRows) - 1
	}
}

func (m *Model) hasComment(path string, line int, side diffview.Side) bool {
	commentSide := comments.SideNew
	if side == diffview.SideOld {
		commentSide = comments.SideOld
	}
	_, ok := m.comments[comments.AnchorKey(path, commentSide, line)]
	return ok
}

func (m *Model) currentAnchor() (commentAnchor, bool) {
	if len(m.diffRows) == 0 || m.diffCursor < 0 || m.diffCursor >= len(m.diffRows) {
		return commentAnchor{}, false
	}
	row := m.diffRows[m.diffCursor]
	if row.Kind == diffview.RowFileHeader || row.Kind == diffview.RowHunkHeader {
		return commentAnchor{}, false
	}
	if row.Path == "" {
		return commentAnchor{}, false
	}

	side, line, ok := pickAnchor(row)
	if !ok {
		return commentAnchor{}, false
	}

	return commentAnchor{Path: row.Path, Side: side, Line: line, RowIdx: m.diffCursor}, true
}

func (m *Model) commentRowIndices() []int {
	rows := make([]int, 0)
	for i, row := range m.diffRows {
		if row.OldLine != nil && m.hasComment(row.Path, *row.OldLine, diffview.SideOld) {
			rows = append(rows, i)
			continue
		}
		if row.NewLine != nil && m.hasComment(row.Path, *row.NewLine, diffview.SideNew) {
			rows = append(rows, i)
		}
	}
	return rows
}

func (m *Model) persistComments() error {
	return m.commentStore.Save(m.sortedComments())
}

func (m Model) sortedComments() []comments.Comment {
	out := make([]comments.Comment, 0, len(m.comments))
	for _, c := range m.comments {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		if out[i].Side != out[j].Side {
			return out[i].Side < out[j].Side
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func (m *Model) contextAround(anchor commentAnchor) ([]string, []string) {
	target := m.sideText(m.diffRows[anchor.RowIdx], anchor.Side)
	before := ""
	after := ""

	for i := anchor.RowIdx - 1; i >= 0; i-- {
		if m.diffRows[i].Path != anchor.Path {
			continue
		}
		text := m.sideText(m.diffRows[i], anchor.Side)
		if text != "" {
			before = text
			break
		}
	}

	for i := anchor.RowIdx + 1; i < len(m.diffRows); i++ {
		if m.diffRows[i].Path != anchor.Path {
			continue
		}
		text := m.sideText(m.diffRows[i], anchor.Side)
		if text != "" {
			after = text
			break
		}
	}

	contextBefore := make([]string, 0, 1)
	if before != "" {
		contextBefore = append(contextBefore, before)
	}

	contextAfter := make([]string, 0, 2)
	if target != "" {
		contextAfter = append(contextAfter, target)
	}
	if after != "" {
		contextAfter = append(contextAfter, after)
	}

	return contextBefore, contextAfter
}

func (m *Model) sideText(row diffview.DiffRow, side comments.Side) string {
	if side == comments.SideOld && row.OldLine != nil {
		return row.OldText
	}
	if side == comments.SideNew && row.NewLine != nil {
		return row.NewText
	}
	return ""
}

func (m *Model) hunkHeaderForRow(rowIdx int, path string) string {
	for i := rowIdx; i >= 0; i-- {
		row := m.diffRows[i]
		if row.Path != path {
			continue
		}
		if row.Kind == diffview.RowHunkHeader {
			return row.OldText
		}
		if row.Kind == diffview.RowFileHeader {
			break
		}
	}
	return ""
}

func pickAnchor(row diffview.DiffRow) (comments.Side, int, bool) {
	switch row.Kind {
	case diffview.RowDelete:
		if row.OldLine != nil {
			return comments.SideOld, *row.OldLine, true
		}
	case diffview.RowAdd:
		if row.NewLine != nil {
			return comments.SideNew, *row.NewLine, true
		}
	default:
		if row.NewLine != nil {
			return comments.SideNew, *row.NewLine, true
		}
		if row.OldLine != nil {
			return comments.SideOld, *row.OldLine, true
		}
	}
	return comments.SideNew, 0, false
}

func firstRenderableRow(rows []diffview.DiffRow) int {
	if len(rows) == 0 {
		return 0
	}
	for i, row := range rows {
		if row.Kind != diffview.RowFileHeader && row.Kind != diffview.RowHunkHeader {
			return i
		}
	}
	return 0
}

func removeLastRune(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func truncateLinesToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) > width {
			lines[i] = string(runes[:width])
		}
	}
	return strings.Join(lines, "\n")
}

func lineCount(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}
