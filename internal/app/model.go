package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
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
	focusComments
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

type commentStaleLoadedMsg struct {
	stale map[string]bool
	err   error
}

type alertTickMsg struct{}

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

	fileItems      []gitint.FileItem
	selected       int
	selectedF      string
	filePaneW      int
	fileHidden     bool
	fileCursor     int
	fileScroll     int
	treeCollapsed  map[string]bool
	commentsCursor int
	commentsScroll int
	commentsReturn focusPane
	commentStale   map[string]bool

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
	commentInputModel  textinput.Model
	commentInputErr    string
	commentEditAnchor  *commentAnchor
	commentEditKey     string

	alertMsg           string
	alertUntil         time.Time
	clearConfirmModal  bool
	pendingCommentJump *commentAnchor

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
	commentInput := textinput.New()
	commentInput.Prompt = ""
	commentInput.Placeholder = "Type comment"
	commentInput.CharLimit = 4096
	commentInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("51"))
	commentInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	m := Model{
		keys:              defaultKeyMap(),
		focus:             focusFiles,
		cwd:               cwd,
		diffMode:          gitint.DiffModeAll,
		statusSvc:         gitint.NewStatusService(),
		diffSvc:           gitint.NewDiffService(),
		helpOpen:          false,
		filePaneW:         filePaneWidthDefault,
		treeCollapsed:     make(map[string]bool),
		commentsReturn:    focusDiff,
		commentStale:      make(map[string]bool),
		commentStore:      store,
		comments:          commentMap,
		commentInputModel: commentInput,
		diffDirty:         true,
		oldWidth:          -1,
		newWidth:          -1,
	}
	if loadErr != nil {
		m.setAlert(fmt.Sprintf("failed to load comments: %v", loadErr))
	}

	m.oldView = viewport.New(1, 1)
	m.newView = viewport.New(1, 1)
	m.oldView.SetContent("Select a file to load its diff.")
	m.newView.SetContent("Select a file to load its diff.")
	return m, nil
}

func (m Model) Init() tea.Cmd {
	m.loadingFiles = true
	return tea.Batch(m.loadFilesCmd(), alertTickCmd())
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
			m.fileCursor = 0
			m.fileScroll = 0
			m.diffRows = nil
			m.diffCursor = 0
			m.rowStarts = nil
			m.rowHeights = nil
			m.diffDirty = false
			m.oldView.GotoTop()
			m.newView.GotoTop()
			m.oldView.SetContent("No changed files found in this repository.")
			m.newView.SetContent("No changed files found in this repository.")
			m.commentStale = m.staleAllComments()
			return m, nil
		}

		if idx := indexOfFilePath(m.fileItems, m.selectedF); idx >= 0 {
			m.selected = idx
		}
		if m.selected >= len(m.fileItems) {
			m.selected = len(m.fileItems) - 1
		}
		m.selectedF = m.fileItems[m.selected].Path
		m.syncFileCursorToSelectedPath()
		m.ensureFileCursorVisible(m.fileTreeEntries())
		return m, tea.Batch(
			m.loadDiffCmd(m.selectedF),
			m.loadCommentStaleCmd(m.fileItems, m.comments, m.diffMode),
		)

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
		if m.pendingCommentJump != nil && m.pendingCommentJump.Path == msg.path {
			m.jumpToCommentAnchor(*m.pendingCommentJump)
			m.pendingCommentJump = nil
		}
		return m, nil

	case clipboardResultMsg:
		if msg.err != nil {
			m.setAlert(fmt.Sprintf("export failed: %v", msg.err))
			return m, nil
		}
		m.setAlert("Copied comments export to clipboard.")
		return m, nil

	case commentStaleLoadedMsg:
		if msg.stale == nil {
			m.commentStale = make(map[string]bool)
		} else {
			m.commentStale = msg.stale
		}
		return m, nil

	case alertTickMsg:
		if m.alertMsg != "" && !m.alertUntil.IsZero() && time.Now().After(m.alertUntil) {
			m.alertMsg = ""
			m.alertUntil = time.Time{}
		}
		return m, alertTickCmd()

	case tea.KeyMsg:
		if m.commentInputActive {
			return m.handleCommentInput(msg)
		}
		if m.clearConfirmModal {
			return m.handleClearConfirm(msg)
		}
		if m.focus == focusComments && isRuneKey(msg, "q") {
			m.focus = m.commentsReturn
			if m.focus == focusFiles {
				m.ensureFilePaneVisible()
			}
			return m, nil
		}

		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.ToggleFocus) {
			if m.focus == focusFiles {
				m.focus = focusDiff
			} else if m.focus == focusDiff {
				m.commentsReturn = focusDiff
				m.focus = focusComments
			} else {
				m.focus = focusFiles
				m.ensureFilePaneVisible()
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.CommentsView) {
			if m.focus == focusComments {
				m.focus = m.commentsReturn
				if m.focus == focusFiles {
					m.ensureFilePaneVisible()
				}
			} else {
				m.commentsReturn = m.focus
				m.focus = focusComments
			}
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
				return m, tea.Batch(
					m.loadDiffCmd(m.selectedF),
					m.loadCommentStaleCmd(m.fileItems, m.comments, m.diffMode),
				)
			}
			return m, m.loadCommentStaleCmd(m.fileItems, m.comments, m.diffMode)
		}
		if key.Matches(msg, m.keys.ClearAll) {
			if len(m.comments) == 0 {
				m.setAlert("No comments to clear.")
				return m, nil
			}
			m.clearConfirmModal = true
			return m, nil
		}

		if m.focus == focusFiles {
			return m.updateFilesPane(msg)
		}
		if m.focus == focusComments {
			return m.updateCommentsPane(msg)
		}
		return m.updateDiffPane(msg)
	}

	return m, nil
}

func (m Model) updateFilesPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ToggleFiles) {
		m.toggleFilePaneWidth()
		return m, nil
	}

	entries := m.fileTreeEntries()
	if len(entries) == 0 {
		return m, nil
	}
	m.clampFileCursor(entries)

	switch {
	case key.Matches(msg, m.keys.Up):
		if m.fileCursor > 0 {
			m.fileCursor--
		}
		m.ensureFileCursorVisible(entries)
		return m.updateSelectedFileFromCursor(entries)

	case key.Matches(msg, m.keys.Down):
		if m.fileCursor < len(entries)-1 {
			m.fileCursor++
		}
		m.ensureFileCursorVisible(entries)
		return m.updateSelectedFileFromCursor(entries)

	case key.Matches(msg, m.keys.ScrollDown):
		return m.scrollFilesWindow(1, entries)

	case key.Matches(msg, m.keys.ScrollUp):
		return m.scrollFilesWindow(-1, entries)

	case isRuneKey(msg, "h"):
		return m.handleFilesLeft(entries)

	case isRuneKey(msg, "l"):
		return m.handleFilesRight(entries)

	case key.Matches(msg, m.keys.Open):
		entry := entries[m.fileCursor]
		if entry.IsDir {
			m.toggleDirCollapsed(entry.Path)
			m.ensureFileCursorVisible(m.fileTreeEntries())
			return m, nil
		}
		if entry.FileIndex >= 0 && entry.FileIndex < len(m.fileItems) {
			m.selected = entry.FileIndex
			m.selectedF = m.fileItems[m.selected].Path
			m.loadingDiff = true
			m.focus = focusDiff
			return m, m.loadDiffCmd(m.selectedF)
		}
		return m, nil

	}

	return m, nil
}

func (m *Model) updateSelectedFileFromCursor(entries []fileTreeEntry) (tea.Model, tea.Cmd) {
	m.clampFileCursor(entries)
	entry := entries[m.fileCursor]
	if entry.IsDir || entry.FileIndex < 0 || entry.FileIndex >= len(m.fileItems) {
		return *m, nil
	}
	if m.selected == entry.FileIndex && m.selectedF == entry.Path {
		return *m, nil
	}
	m.selected = entry.FileIndex
	m.selectedF = entry.Path
	m.loadingDiff = true
	return *m, m.loadDiffCmd(m.selectedF)
}

func (m Model) updateCommentsPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.sortedComments()
	if len(items) == 0 {
		switch {
		case key.Matches(msg, m.keys.ScrollDown), key.Matches(msg, m.keys.ScrollUp):
			return m, nil
		case key.Matches(msg, m.keys.PageDown), key.Matches(msg, m.keys.PageUp):
			return m, nil
		case key.Matches(msg, m.keys.Top), key.Matches(msg, m.keys.Bottom):
			return m, nil
		case key.Matches(msg, m.keys.Edit), key.Matches(msg, m.keys.Delete), key.Matches(msg, m.keys.Open):
			m.setAlert("No comments.")
			return m, nil
		}
		return m, nil
	}

	m.clampCommentsCursor(items)
	switch {
	case key.Matches(msg, m.keys.Up):
		if m.commentsCursor > 0 {
			m.commentsCursor--
		}
		m.ensureCommentsCursorVisible(items)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.commentsCursor < len(items)-1 {
			m.commentsCursor++
		}
		m.ensureCommentsCursorVisible(items)
		return m, nil

	case key.Matches(msg, m.keys.ScrollDown):
		m.scrollCommentsWindow(1, items)
		return m, nil

	case key.Matches(msg, m.keys.ScrollUp):
		m.scrollCommentsWindow(-1, items)
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.pageComments(1, items)
		return m, nil

	case key.Matches(msg, m.keys.PageUp):
		m.pageComments(-1, items)
		return m, nil

	case key.Matches(msg, m.keys.Top):
		m.commentsCursor = 0
		m.ensureCommentsCursorVisible(items)
		return m, nil

	case key.Matches(msg, m.keys.Bottom):
		m.commentsCursor = len(items) - 1
		m.ensureCommentsCursorVisible(items)
		return m, nil

	case key.Matches(msg, m.keys.Edit):
		return m, m.startCommentEditByComment(items[m.commentsCursor])

	case key.Matches(msg, m.keys.Delete):
		m.deleteCommentByKey(commentKey(items[m.commentsCursor]))
		next := m.sortedComments()
		m.clampCommentsCursor(next)
		m.ensureCommentsCursorVisible(next)
		return m, nil

	case key.Matches(msg, m.keys.Open):
		if m.isCommentStale(items[m.commentsCursor]) {
			m.setAlert("Selected comment is stale and cannot be jumped to.")
			return m, nil
		}
		return m, m.jumpToCommentInDiff(items[m.commentsCursor])
	}

	return m, nil
}

func (m *Model) handleFilesLeft(entries []fileTreeEntry) (tea.Model, tea.Cmd) {
	m.clampFileCursor(entries)
	entry := entries[m.fileCursor]
	if !entry.IsDir {
		parent := parentDirPath(entry.Path)
		if parent != "" && m.setFileCursorByDir(entries, parent) {
			m.ensureFileCursorVisible(entries)
			return *m, nil
		}
		return *m, nil
	}

	if !m.isDirCollapsed(entry.Path) {
		m.toggleDirCollapsed(entry.Path)
		m.ensureFileCursorVisible(m.fileTreeEntries())
		return *m, nil
	}

	parent := parentDirPath(entry.Path)
	if parent != "" {
		m.setFileCursorByDir(entries, parent)
	}
	m.ensureFileCursorVisible(entries)
	return *m, nil
}

func (m *Model) handleFilesRight(entries []fileTreeEntry) (tea.Model, tea.Cmd) {
	m.clampFileCursor(entries)
	entry := entries[m.fileCursor]
	if !entry.IsDir {
		if entry.FileIndex >= 0 && entry.FileIndex < len(m.fileItems) {
			if m.selected != entry.FileIndex || m.selectedF != entry.Path {
				m.selected = entry.FileIndex
				m.selectedF = entry.Path
				m.loadingDiff = true
				m.focus = focusDiff
				return *m, m.loadDiffCmd(m.selectedF)
			}
			m.focus = focusDiff
		}
		return *m, nil
	}

	if m.isDirCollapsed(entry.Path) {
		delete(m.treeCollapsed, entry.Path)
	}

	updated := m.fileTreeEntries()
	dirIdx := -1
	for i, e := range updated {
		if e.IsDir && e.Path == entry.Path {
			dirIdx = i
			break
		}
	}
	if dirIdx == -1 {
		m.ensureFileCursorVisible(updated)
		return *m, nil
	}

	dirDepth := updated[dirIdx].Depth
	for i := dirIdx + 1; i < len(updated); i++ {
		if updated[i].Depth <= dirDepth {
			break
		}
		if updated[i].Depth == dirDepth+1 {
			m.fileCursor = i
			m.ensureFileCursorVisible(updated)
			return m.updateSelectedFileFromCursor(updated)
		}
	}

	m.ensureFileCursorVisible(updated)
	return *m, nil
}

func (m *Model) clampFileCursor(entries []fileTreeEntry) {
	if len(entries) == 0 {
		m.fileCursor = 0
		return
	}
	if m.fileCursor < 0 {
		m.fileCursor = 0
	}
	if m.fileCursor >= len(entries) {
		m.fileCursor = len(entries) - 1
	}
}

func (m *Model) ensureFileCursorVisible(entries []fileTreeEntry) {
	m.clampFileCursor(entries)
	page := m.fileListPageSize()
	if page < 1 {
		page = 1
	}
	maxScroll := len(entries) - page
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.fileScroll < 0 {
		m.fileScroll = 0
	}
	if m.fileScroll > maxScroll {
		m.fileScroll = maxScroll
	}
	if m.fileCursor < m.fileScroll {
		m.fileScroll = m.fileCursor
	}
	if m.fileCursor >= m.fileScroll+page {
		m.fileScroll = m.fileCursor - page + 1
	}
	if m.fileScroll < 0 {
		m.fileScroll = 0
	}
	if m.fileScroll > maxScroll {
		m.fileScroll = maxScroll
	}
}

func (m *Model) scrollFilesWindow(delta int, entries []fileTreeEntry) (tea.Model, tea.Cmd) {
	if len(entries) == 0 || delta == 0 {
		return *m, nil
	}
	page := m.fileListPageSize()
	if page < 1 {
		page = 1
	}
	maxScroll := len(entries) - page
	if maxScroll < 0 {
		maxScroll = 0
	}
	oldTop := m.fileScroll
	newTop := oldTop + delta
	if newTop < 0 {
		newTop = 0
	}
	if newTop > maxScroll {
		newTop = maxScroll
	}
	if newTop == oldTop {
		return *m, nil
	}

	rel := m.fileCursor - oldTop
	if rel < 0 {
		rel = 0
	}
	if rel >= page {
		rel = page - 1
	}
	m.fileScroll = newTop
	target := newTop + rel
	if target < 0 {
		target = 0
	}
	if target >= len(entries) {
		target = len(entries) - 1
	}
	m.fileCursor = target
	return m.updateSelectedFileFromCursor(entries)
}

func (m *Model) setFileCursorByDir(entries []fileTreeEntry, dirPath string) bool {
	for i, e := range entries {
		if e.IsDir && e.Path == dirPath {
			m.fileCursor = i
			return true
		}
	}
	return false
}

func (m *Model) setFileCursorByPath(entries []fileTreeEntry, path string) bool {
	for i, e := range entries {
		if !e.IsDir && e.Path == path {
			m.fileCursor = i
			return true
		}
	}
	return false
}

func (m *Model) isDirCollapsed(path string) bool {
	return m.treeCollapsed[path]
}

func (m *Model) toggleDirCollapsed(path string) {
	if m.treeCollapsed == nil {
		m.treeCollapsed = make(map[string]bool)
	}
	if m.treeCollapsed[path] {
		delete(m.treeCollapsed, path)
		return
	}
	m.treeCollapsed[path] = true
}

func (m *Model) firstFilePathUnderDir(dirPath string) (string, bool) {
	prefix := dirPath + "/"
	for _, item := range m.fileItems {
		if strings.HasPrefix(item.Path, prefix) {
			return item.Path, true
		}
	}
	return "", false
}

func (m *Model) expandDirsForFilePath(filePath string) {
	if m.treeCollapsed == nil {
		return
	}
	parts := strings.Split(strings.TrimSuffix(filePath, "/"), "/")
	if len(parts) <= 1 {
		return
	}
	cur := ""
	for i := 0; i < len(parts)-1; i++ {
		if cur == "" {
			cur = parts[i]
		} else {
			cur = cur + "/" + parts[i]
		}
		delete(m.treeCollapsed, cur)
	}
}

func parentDirPath(p string) string {
	i := strings.LastIndex(p, "/")
	if i <= 0 {
		return ""
	}
	return p[:i]
}

func isRuneKey(msg tea.KeyMsg, key string) bool {
	return msg.Type == tea.KeyRunes && msg.String() == key
}

func indexOfFilePath(items []gitint.FileItem, path string) int {
	if path == "" {
		return -1
	}
	for i, item := range items {
		if item.Path == path {
			return i
		}
	}
	return -1
}

func (m *Model) syncFileCursorToSelectedPath() {
	entries := m.fileTreeEntries()
	if len(entries) == 0 {
		m.fileCursor = 0
		return
	}
	for i, e := range entries {
		if !e.IsDir && e.Path == m.selectedF {
			m.fileCursor = i
			return
		}
	}
	m.fileCursor = 0
}

func commentKey(c comments.Comment) string {
	return comments.AnchorKey(c.Path, c.Side, c.Line)
}

func (m *Model) clampCommentsCursor(items []comments.Comment) {
	if len(items) == 0 {
		m.commentsCursor = 0
		return
	}
	if m.commentsCursor < 0 {
		m.commentsCursor = 0
	}
	if m.commentsCursor >= len(items) {
		m.commentsCursor = len(items) - 1
	}
}

func (m *Model) commentsPageSize() int {
	if m.height <= 0 {
		return 1
	}
	footerPlain := truncateLinesToWidth(m.helpText(), m.width)
	footerHeight := lineCount(footerPlain)
	dockHeight := 0
	if m.commentInputActive {
		dockHeight = lipgloss.Height(m.renderCommentDock())
	} else if m.alertMsg != "" {
		dockHeight = lipgloss.Height(m.renderAlertDock())
	}
	paneContentHeight := max(1, m.height-footerHeight-dockHeight-2)
	listHeight := paneContentHeight - 2
	if listHeight < 1 {
		listHeight = 1
	}
	return listHeight
}

func (m *Model) ensureCommentsCursorVisible(items []comments.Comment) {
	m.clampCommentsCursor(items)
	page := m.commentsPageSize()
	if page < 1 {
		page = 1
	}
	maxScroll := len(items) - page
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.commentsScroll < 0 {
		m.commentsScroll = 0
	}
	if m.commentsScroll > maxScroll {
		m.commentsScroll = maxScroll
	}
	if m.commentsCursor < m.commentsScroll {
		m.commentsScroll = m.commentsCursor
	}
	if m.commentsCursor >= m.commentsScroll+page {
		m.commentsScroll = m.commentsCursor - page + 1
	}
	if m.commentsScroll < 0 {
		m.commentsScroll = 0
	}
	if m.commentsScroll > maxScroll {
		m.commentsScroll = maxScroll
	}
}

func (m *Model) scrollCommentsWindow(delta int, items []comments.Comment) {
	if len(items) == 0 || delta == 0 {
		return
	}
	page := m.commentsPageSize()
	if page < 1 {
		page = 1
	}
	maxScroll := len(items) - page
	if maxScroll < 0 {
		maxScroll = 0
	}
	oldTop := m.commentsScroll
	newTop := oldTop + delta
	if newTop < 0 {
		newTop = 0
	}
	if newTop > maxScroll {
		newTop = maxScroll
	}
	if newTop == oldTop {
		return
	}
	rel := m.commentsCursor - oldTop
	if rel < 0 {
		rel = 0
	}
	if rel >= page {
		rel = page - 1
	}
	m.commentsScroll = newTop
	m.commentsCursor = newTop + rel
	if m.commentsCursor >= len(items) {
		m.commentsCursor = len(items) - 1
	}
}

func (m *Model) pageComments(direction int, items []comments.Comment) {
	if len(items) == 0 || direction == 0 {
		return
	}
	page := m.commentsPageSize()
	if page < 1 {
		page = 1
	}
	step := page
	if step > 1 {
		step--
	}
	m.scrollCommentsWindow(direction*step, items)
}

func (m *Model) jumpToCommentInDiff(c comments.Comment) tea.Cmd {
	anchor := commentAnchor{
		Path: c.Path,
		Side: c.Side,
		Line: c.Line,
	}
	m.focus = focusDiff
	m.pendingCommentJump = &anchor
	if idx := indexOfFilePath(m.fileItems, c.Path); idx >= 0 {
		m.selected = idx
		m.selectedF = c.Path
		m.syncFileCursorToSelectedPath()
		m.ensureFileCursorVisible(m.fileTreeEntries())
	}

	if m.selectedF == c.Path && len(m.diffRows) > 0 && m.jumpToCommentAnchor(anchor) {
		m.pendingCommentJump = nil
		return nil
	}
	m.loadingDiff = true
	return m.loadDiffCmd(c.Path)
}

func (m *Model) jumpToCommentAnchor(anchor commentAnchor) bool {
	for i, row := range m.diffRows {
		if row.Path != anchor.Path {
			continue
		}
		switch anchor.Side {
		case comments.SideOld:
			if row.OldLine != nil && *row.OldLine == anchor.Line {
				m.diffCursor = i
				m.diffDirty = true
				m.refreshDiffContent()
				m.scrollCursorWithPadding(10)
				return true
			}
		case comments.SideNew:
			if row.NewLine != nil && *row.NewLine == anchor.Line {
				m.diffCursor = i
				m.diffDirty = true
				m.refreshDiffContent()
				m.scrollCursorWithPadding(10)
				return true
			}
		}
	}
	return false
}

func (m Model) updateDiffPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if key.Matches(msg, m.keys.ToggleFiles) || isRuneKey(msg, "l") {
		m.toggleFilePaneHidden()
		return m, nil
	}
	if isRuneKey(msg, "h") {
		m.focus = focusFiles
		m.ensureFilePaneVisible()
		return m, nil
	}

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

	case key.Matches(msg, m.keys.ScrollDown):
		m.scrollDiffWindow(1)
		return m, nil

	case key.Matches(msg, m.keys.ScrollUp):
		m.scrollDiffWindow(-1)
		return m, nil

	case key.Matches(msg, m.keys.PageDown):
		m.pageDiff(1)
		return m, nil

	case key.Matches(msg, m.keys.PageUp):
		m.pageDiff(-1)
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
		return m, m.startCommentEdit(false)

	case key.Matches(msg, m.keys.Edit):
		return m, m.startCommentEdit(true)

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
		if len(m.exportableComments()) == 0 {
			m.setAlert("No non-stale comments to export.")
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
		m.commentInputModel.SetValue("")
		m.commentInputModel.Blur()
		m.commentInputErr = ""
		m.commentEditAnchor = nil
		m.commentEditKey = ""
		return m, nil

	case tea.KeyEnter:
		return m, m.saveCommentInput()
	}

	var cmd tea.Cmd
	m.commentInputModel, cmd = m.commentInputModel.Update(msg)
	m.commentInputErr = ""
	return m, cmd
}

func (m Model) handleClearConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.clearConfirmModal = false
		return m, nil
	case tea.KeyEnter:
		m.clearConfirmModal = false
		m.clearAllComments()
		return m, nil
	case tea.KeyRunes:
		switch msg.String() {
		case "y", "Y":
			m.clearConfirmModal = false
			m.clearAllComments()
			return m, nil
		case "n", "N":
			m.clearConfirmModal = false
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) saveCommentInput() tea.Cmd {
	if m.commentEditAnchor == nil && m.commentEditKey == "" {
		m.commentInputActive = false
		m.commentInputModel.SetValue("")
		m.commentInputModel.Blur()
		m.commentInputErr = ""
		return nil
	}

	body := strings.TrimSpace(m.commentInputModel.Value())
	if body == "" {
		m.commentInputErr = "Comment text is empty."
		return nil
	}

	if m.commentEditAnchor == nil && m.commentEditKey != "" {
		existing, ok := m.comments[m.commentEditKey]
		if !ok {
			m.commentInputErr = "Comment no longer exists."
			return nil
		}
		existing.Body = body
		m.comments[m.commentEditKey] = existing
		if err := m.persistComments(); err != nil {
			m.commentInputErr = fmt.Sprintf("failed to save comment: %v", err)
			return nil
		}
		m.commentInputActive = false
		m.commentInputModel.SetValue("")
		m.commentInputModel.Blur()
		m.commentInputErr = ""
		m.commentEditAnchor = nil
		m.commentEditKey = ""
		m.diffDirty = true
		m.refreshDiffContent()
		return nil
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
	if m.commentStale == nil {
		m.commentStale = make(map[string]bool)
	}
	m.commentStale[key] = false

	if err := m.persistComments(); err != nil {
		m.commentInputErr = fmt.Sprintf("failed to save comment: %v", err)
		return nil
	}

	m.commentInputActive = false
	m.commentInputModel.SetValue("")
	m.commentInputModel.Blur()
	m.commentInputErr = ""
	m.commentEditAnchor = nil
	m.commentEditKey = ""
	m.diffDirty = true
	m.refreshDiffContent()
	return nil
}

func (m *Model) startCommentEdit(requireExisting bool) tea.Cmd {
	anchor, ok := m.currentAnchor()
	if !ok {
		m.setAlert("No commentable line selected.")
		return nil
	}

	key := comments.AnchorKey(anchor.Path, anchor.Side, anchor.Line)
	existing, exists := m.comments[key]
	if requireExisting && !exists {
		m.setAlert("No comment exists on selected line.")
		return nil
	}

	m.commentInputActive = true
	m.commentInputModel.SetValue("")
	m.commentInputErr = ""
	if exists {
		m.commentInputModel.SetValue(existing.Body)
	}
	cmd := m.commentInputModel.Focus()
	m.commentInputModel.CursorEnd()
	a := anchor
	m.commentEditAnchor = &a
	m.commentEditKey = key
	return cmd
}

func (m *Model) startCommentEditByComment(c comments.Comment) tea.Cmd {
	key := commentKey(c)
	if _, ok := m.comments[key]; !ok {
		m.setAlert("Comment no longer exists.")
		return nil
	}
	m.commentInputActive = true
	m.commentInputModel.SetValue(c.Body)
	m.commentInputErr = ""
	m.commentEditAnchor = nil
	m.commentEditKey = key
	cmd := m.commentInputModel.Focus()
	m.commentInputModel.CursorEnd()
	return cmd
}

func (m *Model) deleteCommentAtCursor() {
	anchor, ok := m.currentAnchor()
	if !ok {
		m.setAlert("No commentable line selected.")
		return
	}

	key := comments.AnchorKey(anchor.Path, anchor.Side, anchor.Line)
	if _, exists := m.comments[key]; !exists {
		m.setAlert("No comment exists on selected line.")
		return
	}
	m.deleteCommentByKey(key)
}

func (m *Model) deleteCommentByKey(key string) {
	if _, exists := m.comments[key]; !exists {
		return
	}
	delete(m.comments, key)
	delete(m.commentStale, key)
	if err := m.persistComments(); err != nil {
		m.setAlert(fmt.Sprintf("failed to save comments: %v", err))
		return
	}
	m.diffDirty = true
	m.refreshDiffContent()
}

func (m *Model) clearAllComments() {
	if len(m.comments) == 0 {
		return
	}
	prev := m.comments
	prevStale := m.commentStale
	m.comments = make(map[string]comments.Comment)
	m.commentStale = make(map[string]bool)
	if err := m.persistComments(); err != nil {
		m.comments = prev
		m.commentStale = prevStale
		m.setAlert(fmt.Sprintf("failed to clear comments: %v", err))
		return
	}
	m.diffDirty = true
	m.refreshDiffContent()
}

func (m *Model) jumpToComment(direction int) {
	rows := m.commentRowIndices()
	if len(rows) == 0 {
		m.setAlert("No comments in current diff.")
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
	m.scrollCursorWithPadding(10)
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	help := m.helpText()

	footerHelpPlain := truncateLinesToWidth(help, m.width)
	footerLines := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(footerHelpPlain),
	}
	if staleCount := m.staleCommentCount(); staleCount > 0 {
		warn := truncateLinesToWidth(
			fmt.Sprintf("Warning: %d stale comment(s). They are marked with ! and excluded from export.", staleCount),
			m.width,
		)
		footerLines = append(footerLines, lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render(warn))
	}
	footer := strings.Join(footerLines, "\n")
	footerHeight := lipgloss.Height(footer)

	dock := ""
	dockHeight := 0
	if m.commentInputActive {
		dock = m.renderCommentDock()
		dockHeight = lipgloss.Height(dock)
	} else if m.alertMsg != "" {
		dock = m.renderAlertDock()
		dockHeight = lipgloss.Height(dock)
	}

	leftW, rightW := paneWidths(m.width, m.filePaneW, m.fileHidden)
	oldPaneW, newPaneW := splitRightPanes(rightW)
	// lipgloss Height applies to content height; borders add 2 more rows.
	paneContentHeight := max(1, m.height-footerHeight-dockHeight-2)
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

	content := ""
	if m.focus == focusComments {
		content = m.renderCommentsPane(m.width, paneContentHeight)
	} else {
		rightPane := m.renderDiffPanes(oldPaneW, newPaneW, paneContentHeight)
		content = rightPane
		if !m.fileHidden {
			leftPane := m.renderFilesPane(leftW, paneContentHeight)
			content = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
		}
	}

	body := content
	if dock != "" {
		body = lipgloss.JoinVertical(lipgloss.Left, body, dock)
	}
	if m.clearConfirmModal {
		body = overlayCentered(body, m.renderClearAllConfirmModal(), m.width, lipgloss.Height(body))
	}
	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (m Model) helpText() string {
	if !m.helpOpen {
		return "tab focus | m comments view | j/k move | ctrl-f/b page | ctrl-e/y scroll | enter open diff | z zoom/hide files | t mode | c/e/d comment | n/p comment nav | y export | C clear all | r refresh | ? help | q quit"
	}
	return strings.Join([]string{
		"Global: q quit, tab switch focus, m comments view, t toggle diff mode, C clear all comments, ? toggle help",
		"Files pane: j/k move, ctrl-e/ctrl-y scroll, h/l tree nav, enter open diff, z toggle file pane width, r refresh",
		"Diff pane: j/k move cursor, ctrl-e/ctrl-y scroll, ctrl-f/ctrl-b page, g/G top/bottom, h focus files, z/l hide/show file list",
		"Comments view: j/k move, ctrl-e/ctrl-y scroll, ctrl-f/ctrl-b page, g/G top/bottom, e edit, d delete, enter jump to diff",
		"Comments: c create, e edit, d delete, n/p next/prev, y export to clipboard",
	}, "\n")
}

func (m *Model) fileListPageSize() int {
	if m.height <= 0 {
		return 1
	}
	footerPlain := truncateLinesToWidth(m.helpText(), m.width)
	footerHeight := lineCount(footerPlain)
	dockHeight := 0
	if m.commentInputActive {
		dockHeight = lipgloss.Height(m.renderCommentDock())
	} else if m.alertMsg != "" {
		dockHeight = lipgloss.Height(m.renderAlertDock())
	}
	paneContentHeight := max(1, m.height-footerHeight-dockHeight-2)
	listHeight := paneContentHeight - 2
	if listHeight < 1 {
		listHeight = 1
	}
	return listHeight
}

func (m Model) renderCommentDock() string {
	title := "Add Comment"
	if m.commentEditAnchor != nil {
		key := comments.AnchorKey(m.commentEditAnchor.Path, m.commentEditAnchor.Side, m.commentEditAnchor.Line)
		if _, exists := m.comments[key]; exists {
			title = "Edit Comment"
		}
	}

	contentW := max(10, m.width-2)
	inputWidth := max(1, contentW-9)
	input := m.commentInputModel
	input.Width = inputWidth
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		Render(input.View())
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Enter save | Esc cancel | Backspace delete")

	bodyLines := []string{inputBox, "", hint}
	if m.commentInputErr != "" {
		bodyLines = append(bodyLines, "")
		bodyLines = append(bodyLines, lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Render("Error: "+m.commentInputErr))
	}

	body := strings.Join(bodyLines, "\n")
	return m.renderDockPanel(title, lipgloss.Color("39"), lipgloss.Color("39"), body)
}

func (m Model) renderAlertDock() string {
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Auto-hides after 3s")
	body := strings.Join([]string{
		m.alertMsg,
		"",
		hint,
	}, "\n")
	return m.renderDockPanel("Notice", lipgloss.Color("220"), lipgloss.Color("220"), body)
}

func (m Model) renderClearAllConfirmModal() string {
	body := strings.Join([]string{
		"Clear all comments across all files?",
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render("Y/Enter confirm | N/Esc cancel"),
	}, "\n")

	width := 54
	if m.width > 0 && m.width-6 < width {
		width = max(24, m.width-6)
	}

	title := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Padding(0, 1).
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(lipgloss.Color("196")).
		Render("Clear All Comments")

	bodyBlock := lipgloss.NewStyle().
		Width(max(1, width-2)).
		Padding(1, 2).
		Render(body)

	return lipgloss.NewStyle().
		Width(width).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Render(title + "\n" + bodyBlock)
}

func (m Model) renderDockPanel(title string, titleColor, borderColor lipgloss.Color, body string) string {
	contentW := max(10, m.width-2)
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
		Render(titleBar + "\n" + bodyBlock)
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

	entries := m.fileTreeEntries()
	cursor := m.fileCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(entries) {
		cursor = len(entries) - 1
	}

	if len(entries) == 0 {
		bodyLines = append(bodyLines, "No changed files")
	} else {
		pageSize := m.fileListPageSize()
		if pageSize < 1 {
			pageSize = 1
		}
		maxScroll := len(entries) - pageSize
		if maxScroll < 0 {
			maxScroll = 0
		}
		start := m.fileScroll
		if start < 0 {
			start = 0
		}
		if start > maxScroll {
			start = maxScroll
		}
		end := start + pageSize
		if end > len(entries) {
			end = len(entries)
		}

		for i := start; i < end; i++ {
			entry := entries[i]
			prefix := "  "
			if i == cursor {
				prefix = "> "
			}
			indent := strings.Repeat("  ", entry.Depth)
			line := ""
			if entry.IsDir {
				icon := "[-]"
				if m.isDirCollapsed(entry.Path) {
					icon = "[+]"
				}
				line = fmt.Sprintf("%s%s%s %s/", prefix, indent, icon, entry.Name)
			} else {
				commentMark := " "
				if entry.HasComment {
					commentMark = "C"
				}
				line = fmt.Sprintf("%s%s%s [%s] %s", prefix, indent, commentMark, entry.Status, entry.Name)
			}
			lineStyle := lipgloss.NewStyle().Width(innerW).MaxWidth(innerW)
			if entry.IsDir {
				lineStyle = lineStyle.Foreground(lipgloss.Color("244"))
			}
			if i == cursor {
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

func (m Model) renderCommentsPane(width, height int) string {
	border := lipgloss.NormalBorder()
	borderColor := lipgloss.Color("245")
	if m.focus == focusComments {
		borderColor = lipgloss.Color("39")
	}
	contentW := max(1, width-2)
	paneStyle := lipgloss.NewStyle().
		Width(contentW).
		Height(max(1, height)).
		Border(border).
		BorderForeground(borderColor)

	items := m.sortedComments()
	cursor := m.commentsCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(items) {
		cursor = len(items) - 1
	}

	bodyLines := make([]string, 0, len(items)+2)
	title := fmt.Sprintf("Comments (%d)", len(items))
	bodyLines = append(bodyLines, title)
	bodyLines = append(bodyLines, "")
	if len(items) == 0 {
		bodyLines = append(bodyLines, "No comments")
		return paneStyle.Render(strings.Join(bodyLines, "\n"))
	}

	pageSize := m.commentsPageSize()
	if pageSize < 1 {
		pageSize = 1
	}
	maxScroll := len(items) - pageSize
	if maxScroll < 0 {
		maxScroll = 0
	}
	start := m.commentsScroll
	if start < 0 {
		start = 0
	}
	if start > maxScroll {
		start = maxScroll
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}

	innerW := max(1, contentW)
	for i := start; i < end; i++ {
		c := items[i]
		prefix := "  "
		if i == cursor {
			prefix = "> "
		}
		side := c.Side.String()
		summary := strings.ReplaceAll(strings.TrimSpace(c.Body), "\n", " / ")
		staleMark := " "
		if m.isCommentStale(c) {
			staleMark = "!"
		}
		line := fmt.Sprintf("%s%s %s:%s:%d | %s", prefix, staleMark, c.Path, side, c.Line, summary)
		style := lipgloss.NewStyle().Width(innerW).MaxWidth(innerW)
		if i == cursor {
			style = style.Foreground(lipgloss.Color("39")).Bold(true)
		} else if staleMark == "!" {
			style = style.Foreground(lipgloss.Color("214"))
		}
		bodyLines = append(bodyLines, style.Render(line))
	}
	return paneStyle.Render(strings.Join(bodyLines, "\n"))
}

type fileTreeEntry struct {
	Path       string
	Name       string
	Depth      int
	IsDir      bool
	FileIndex  int
	Status     string
	HasComment bool
}

type fileTreeDir struct {
	Name  string
	Path  string
	Dirs  map[string]*fileTreeDir
	Files []fileTreeFile
}

type fileTreeFile struct {
	Name       string
	Path       string
	FileIndex  int
	Status     string
	HasComment bool
}

func (m Model) commentedPaths() map[string]bool {
	out := make(map[string]bool, len(m.comments))
	for _, c := range m.comments {
		out[c.Path] = true
	}
	return out
}

func (m Model) fileTreeEntries() []fileTreeEntry {
	root := &fileTreeDir{
		Name: "",
		Path: "",
		Dirs: make(map[string]*fileTreeDir),
	}
	commented := m.commentedPaths()
	for i, item := range m.fileItems {
		parts := strings.Split(strings.TrimSuffix(item.Path, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			continue
		}
		node := root
		for d := 0; d < len(parts)-1; d++ {
			name := parts[d]
			child, ok := node.Dirs[name]
			if !ok {
				path := name
				if node.Path != "" {
					path = node.Path + "/" + name
				}
				child = &fileTreeDir{
					Name: name,
					Path: path,
					Dirs: make(map[string]*fileTreeDir),
				}
				node.Dirs[name] = child
			}
			node = child
		}
		name := parts[len(parts)-1]
		node.Files = append(node.Files, fileTreeFile{
			Name:       name,
			Path:       item.Path,
			FileIndex:  i,
			Status:     item.Status,
			HasComment: commented[item.Path],
		})
	}

	out := make([]fileTreeEntry, 0, len(m.fileItems)*2)
	flattenTreeEntries(root, 0, m.treeCollapsed, &out)
	return out
}

func flattenTreeEntries(node *fileTreeDir, depth int, collapsed map[string]bool, out *[]fileTreeEntry) {
	dirNames := make([]string, 0, len(node.Dirs))
	for name := range node.Dirs {
		dirNames = append(dirNames, name)
	}
	sort.Strings(dirNames)
	for _, name := range dirNames {
		child := node.Dirs[name]
		*out = append(*out, fileTreeEntry{
			Path:      child.Path,
			Name:      child.Name,
			Depth:     depth,
			IsDir:     true,
			FileIndex: -1,
		})
		if collapsed != nil && collapsed[child.Path] {
			continue
		}
		flattenTreeEntries(child, depth+1, collapsed, out)
	}

	sort.Slice(node.Files, func(i, j int) bool {
		return node.Files[i].Name < node.Files[j].Name
	})
	for _, f := range node.Files {
		*out = append(*out, fileTreeEntry{
			Path:       f.Path,
			Name:       f.Name,
			Depth:      depth,
			IsDir:      false,
			FileIndex:  f.FileIndex,
			Status:     f.Status,
			HasComment: f.HasComment,
		})
	}
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
	_, rightW := paneWidths(m.width, m.filePaneW, m.fileHidden)
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

func (m Model) loadCommentStaleCmd(items []gitint.FileItem, commentMap map[string]comments.Comment, mode gitint.DiffMode) tea.Cmd {
	cwd := m.cwd
	service := m.diffSvc
	itemSnapshot := append([]gitint.FileItem(nil), items...)
	commentSnapshot := make([]comments.Comment, 0, len(commentMap))
	for _, c := range commentMap {
		commentSnapshot = append(commentSnapshot, c)
	}
	return func() tea.Msg {
		stale, err := buildCommentStaleMap(context.Background(), cwd, service, itemSnapshot, commentSnapshot, mode)
		return commentStaleLoadedMsg{stale: stale, err: err}
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
	snapshot := m.exportableComments()
	return func() tea.Msg {
		text := comments.ExportPlain(snapshot, "Review comments:")
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

func (m *Model) pageDiff(direction int) {
	if len(m.diffRows) == 0 {
		return
	}
	if direction == 0 {
		return
	}

	visible := m.oldView.VisibleLineCount()
	if nv := m.newView.VisibleLineCount(); nv < visible || visible <= 0 {
		visible = nv
	}
	if visible <= 0 {
		visible = m.oldView.Height
		if m.newView.Height < visible || visible <= 0 {
			visible = m.newView.Height
		}
	}
	if visible <= 0 {
		return
	}

	total := m.oldView.TotalLineCount()
	if n := m.newView.TotalLineCount(); n > total {
		total = n
	}
	if total <= 0 {
		return
	}

	maxTop := total - visible
	if maxTop < 0 {
		maxTop = 0
	}
	step := visible
	if step > 1 {
		// Viewport's visible count includes one extra slot for paging here;
		// subtract one so we don't skip the first line of the next page.
		step--
	}
	targetTop := m.oldView.YOffset + direction*step
	if targetTop < 0 {
		targetTop = 0
	}
	if targetTop > maxTop {
		targetTop = maxTop
	}

	m.diffCursor = m.rowIndexForVisualLine(targetTop)
	m.diffDirty = true
	m.refreshDiffContent()
	m.oldView.SetYOffset(targetTop)
	m.newView.SetYOffset(targetTop)
}

func (m *Model) scrollDiffWindow(delta int) {
	if delta == 0 || len(m.diffRows) == 0 {
		return
	}
	visible := m.oldView.VisibleLineCount()
	if nv := m.newView.VisibleLineCount(); nv < visible || visible <= 0 {
		visible = nv
	}
	if visible <= 0 {
		visible = 1
	}

	total := m.oldView.TotalLineCount()
	if n := m.newView.TotalLineCount(); n > total {
		total = n
	}
	if total <= 0 {
		return
	}

	oldTop := m.oldView.YOffset
	maxTop := total - visible
	if maxTop < 0 {
		maxTop = 0
	}
	newTop := oldTop + delta
	if newTop < 0 {
		newTop = 0
	}
	if newTop > maxTop {
		newTop = maxTop
	}
	if newTop == oldTop {
		return
	}

	start, _ := m.cursorVisualRange()
	rel := start - oldTop
	if rel < 0 {
		rel = 0
	}
	if rel >= visible {
		rel = visible - 1
	}
	targetLine := newTop + rel
	m.diffCursor = m.rowIndexForVisualLine(targetLine)
	m.diffDirty = true
	m.refreshDiffContent()
	m.oldView.SetYOffset(newTop)
	m.newView.SetYOffset(newTop)
}

func (m *Model) toggleFilePaneWidth() {
	if m.filePaneW == filePaneWidthWide {
		m.filePaneW = filePaneWidthDefault
	} else {
		m.filePaneW = filePaneWidthWide
	}
	m.diffDirty = true
	m.resizePanes()
}

func (m *Model) toggleFilePaneHidden() {
	m.fileHidden = !m.fileHidden
	m.diffDirty = true
	m.resizePanes()
}

func (m *Model) ensureFilePaneVisible() {
	if !m.fileHidden {
		return
	}
	m.fileHidden = false
	m.diffDirty = true
	m.resizePanes()
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

	rendered := diffview.RenderSplitWithLayoutComments(
		m.diffRows,
		m.oldView.Width,
		m.newView.Width,
		m.diffCursor,
		func(path string, line int, side diffview.Side) bool {
			return m.hasComment(path, line, side)
		},
		func(path string, line int, side diffview.Side) (string, bool) {
			return m.commentText(path, line, side)
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

func (m *Model) scrollCursorWithPadding(padding int) {
	if padding < 0 {
		padding = 0
	}
	visibleHeight := m.oldView.VisibleLineCount()
	if nv := m.newView.VisibleLineCount(); nv < visibleHeight {
		visibleHeight = nv
	}
	if visibleHeight <= 0 {
		return
	}

	start, end := m.cursorVisualRange()
	totalLines := m.oldView.TotalLineCount()
	if n := m.newView.TotalLineCount(); n > totalLines {
		totalLines = n
	}
	if totalLines <= 0 {
		return
	}
	maxTop := totalLines - visibleHeight
	if maxTop < 0 {
		maxTop = 0
	}

	lowerBound := end + padding - visibleHeight + 1
	upperBound := start - padding
	newTop := m.oldView.YOffset
	if lowerBound <= upperBound {
		if newTop < lowerBound {
			newTop = lowerBound
		}
		if newTop > upperBound {
			newTop = upperBound
		}
	} else {
		newTop = start - padding
	}

	if newTop < 0 {
		newTop = 0
	}
	if newTop > maxTop {
		newTop = maxTop
	}
	m.oldView.SetYOffset(newTop)
	m.newView.SetYOffset(newTop)
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

func (m *Model) rowIndexForVisualLine(line int) int {
	if len(m.rowStarts) != len(m.diffRows) || len(m.rowHeights) != len(m.diffRows) {
		return m.diffCursor
	}
	if line <= 0 {
		return 0
	}
	for i := 0; i < len(m.rowStarts); i++ {
		start := m.rowStarts[i]
		height := m.rowHeights[i]
		if height <= 0 {
			height = 1
		}
		if line >= start && line < start+height {
			return i
		}
	}
	return len(m.diffRows) - 1
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

func (m *Model) commentText(path string, line int, side diffview.Side) (string, bool) {
	commentSide := comments.SideNew
	if side == diffview.SideOld {
		commentSide = comments.SideOld
	}
	c, ok := m.comments[comments.AnchorKey(path, commentSide, line)]
	if !ok {
		return "", false
	}
	return c.Body, true
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

func (m Model) exportableComments() []comments.Comment {
	all := m.sortedComments()
	out := make([]comments.Comment, 0, len(all))
	for _, c := range all {
		if m.isCommentStale(c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (m Model) isCommentStale(c comments.Comment) bool {
	return m.commentStale[commentKey(c)]
}

func (m Model) staleCommentCount() int {
	n := 0
	for _, stale := range m.commentStale {
		if stale {
			n++
		}
	}
	return n
}

func (m Model) staleAllComments() map[string]bool {
	out := make(map[string]bool, len(m.comments))
	for _, c := range m.comments {
		out[commentKey(c)] = true
	}
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

func alertTickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return alertTickMsg{}
	})
}

func (m *Model) setAlert(msg string) {
	m.alertMsg = msg
	m.alertUntil = time.Now().Add(3 * time.Second)
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

func buildCommentStaleMap(
	ctx context.Context,
	cwd string,
	diffSvc gitint.DiffService,
	items []gitint.FileItem,
	allComments []comments.Comment,
	mode gitint.DiffMode,
) (map[string]bool, error) {
	stale := make(map[string]bool, len(allComments))
	if len(allComments) == 0 {
		return stale, nil
	}

	fileSet := make(map[string]bool, len(items))
	for _, it := range items {
		fileSet[it.Path] = true
	}

	byPath := make(map[string][]comments.Comment)
	for _, c := range allComments {
		k := commentKey(c)
		if !fileSet[c.Path] {
			stale[k] = true
			continue
		}
		byPath[c.Path] = append(byPath[c.Path], c)
	}

	var firstErr error
	for path, group := range byPath {
		d, err := diffSvc.Diff(ctx, cwd, path, mode)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			for _, c := range group {
				stale[commentKey(c)] = true
			}
			continue
		}
		if strings.TrimSpace(d) == "" {
			for _, c := range group {
				stale[commentKey(c)] = true
			}
			continue
		}

		rows, err := diffview.ParseUnifiedDiff([]byte(d))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			for _, c := range group {
				stale[commentKey(c)] = true
			}
			continue
		}

		oldLines := make(map[int]bool)
		newLines := make(map[int]bool)
		for _, r := range rows {
			if r.OldLine != nil {
				oldLines[*r.OldLine] = true
			}
			if r.NewLine != nil {
				newLines[*r.NewLine] = true
			}
		}
		for _, c := range group {
			k := commentKey(c)
			switch c.Side {
			case comments.SideOld:
				stale[k] = !oldLines[c.Line]
			default:
				stale[k] = !newLines[c.Line]
			}
		}
	}

	return stale, firstErr
}
