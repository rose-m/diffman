package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	gitint "lediff/internal/git"
)

type focusPane int

const (
	focusFiles focusPane = iota
	focusDiff
)

type filesLoadedMsg struct {
	items []gitint.FileItem
	err   error
}

type diffLoadedMsg struct {
	path string
	diff string
	err  error
}

// Model is the Bubble Tea state container for the app.
type Model struct {
	keys      KeyMap
	focus     focusPane
	cwd       string
	statusSvc gitint.StatusService
	diffSvc   gitint.DiffService

	width  int
	height int
	ready  bool

	fileItems []gitint.FileItem
	selected  int
	selectedF string

	diffView viewport.Model
	helpOpen bool

	loadingFiles bool
	loadingDiff  bool
	err          error
}

func NewModel() (Model, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Model{}, err
	}

	m := Model{
		keys:      defaultKeyMap(),
		focus:     focusFiles,
		cwd:       cwd,
		statusSvc: gitint.NewStatusService(),
		diffSvc:   gitint.NewDiffService(),
		helpOpen:  false,
	}
	m.diffView = viewport.New(1, 1)
	m.diffView.SetContent("Select a file to load its diff.")
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
		return m, nil

	case filesLoadedMsg:
		m.loadingFiles = false
		m.err = msg.err
		m.fileItems = msg.items
		if len(m.fileItems) == 0 {
			m.selected = 0
			m.selectedF = ""
			m.diffView.GotoTop()
			m.diffView.SetContent("No changed files found in this repository.")
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
			m.diffView.SetContent(fmt.Sprintf("Failed to load diff for %s:\n%v", msg.path, msg.err))
			return m, nil
		}
		if strings.TrimSpace(msg.diff) == "" {
			m.diffView.SetContent(fmt.Sprintf("No diff for %s.", msg.path))
			return m, nil
		}
		m.diffView.GotoTop()
		m.diffView.SetContent(msg.diff)
		return m, nil

	case tea.KeyMsg:
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
		if key.Matches(msg, m.keys.Help) {
			m.helpOpen = !m.helpOpen
			return m, nil
		}
		if key.Matches(msg, m.keys.Refresh) {
			m.loadingFiles = true
			return m, m.loadFilesCmd()
		}

		if m.focus == focusFiles {
			return m.updateFilesPane(msg)
		}
		return m.updateDiffPane(msg)
	}

	if m.focus == focusDiff {
		var cmd tea.Cmd
		m.diffView, cmd = m.diffView.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateFilesPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.fileItems) == 0 {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
		}
		m.selectedF = m.fileItems[m.selected].Path
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.selected < len(m.fileItems)-1 {
			m.selected++
		}
		m.selectedF = m.fileItems[m.selected].Path
		return m, nil

	case key.Matches(msg, m.keys.Open):
		m.selectedF = m.fileItems[m.selected].Path
		m.loadingDiff = true
		return m, m.loadDiffCmd(m.selectedF)
	}

	return m, nil
}

func (m Model) updateDiffPane(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.diffView.LineUp(1)
		return m, nil
	case key.Matches(msg, m.keys.Down):
		m.diffView.LineDown(1)
		return m, nil
	case key.Matches(msg, m.keys.Top):
		m.diffView.GotoTop()
		return m, nil
	case key.Matches(msg, m.keys.Bottom):
		m.diffView.GotoBottom()
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	help := "tab focus | j/k move | enter open diff | g/G top/bottom | r refresh | ? help | q quit"
	if m.helpOpen {
		help = strings.Join([]string{
			"Global: q quit, tab switch focus, ? toggle help",
			"Files pane: j/k move, enter open diff, r refresh",
			"Diff pane: j/k scroll, g/G top/bottom",
		}, "\n")
	}
	help = truncateLinesToWidth(help, m.width)
	helpLines := lineCount(help)

	leftW, rightW := paneWidths(m.width)
	// lipgloss Height applies to content height; borders add 2 more rows.
	paneContentHeight := max(1, m.height-helpLines-2)
	m.diffView.Width = max(1, rightW-4)
	m.diffView.Height = max(1, paneContentHeight-4)

	leftPane := m.renderFilesPane(leftW, paneContentHeight)
	rightPane := m.renderDiffPane(rightW, paneContentHeight)
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	return lipgloss.JoinVertical(lipgloss.Left, content, help)
}

func (m Model) renderFilesPane(width, height int) string {
	border := lipgloss.NormalBorder()
	borderColor := lipgloss.Color("241")
	if m.focus == focusFiles {
		borderColor = lipgloss.Color("39")
	}

	paneStyle := lipgloss.NewStyle().
		Width(max(1, width)).
		Height(max(1, height)).
		Border(border).
		BorderForeground(borderColor)

	title := "Files"
	if m.loadingFiles {
		title += " (loading...)"
	}

	innerW := max(1, width-4)
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
			bodyLines = append(bodyLines, lipgloss.NewStyle().Width(innerW).MaxWidth(innerW).Render(line))
		}
	}

	if m.err != nil {
		bodyLines = append(bodyLines, "")
		bodyLines = append(bodyLines, fmt.Sprintf("error: %v", m.err))
	}

	return paneStyle.Render(strings.Join(bodyLines, "\n"))
}

func (m Model) renderDiffPane(width, height int) string {
	border := lipgloss.NormalBorder()
	borderColor := lipgloss.Color("241")
	if m.focus == focusDiff {
		borderColor = lipgloss.Color("39")
	}

	paneStyle := lipgloss.NewStyle().
		Width(max(1, width)).
		Height(max(1, height)).
		Border(border).
		BorderForeground(borderColor)

	title := "Diff"
	if m.selectedF != "" {
		title = "Diff: " + m.selectedF
	}
	if m.loadingDiff {
		title += " (loading...)"
	}

	innerW := max(1, width-4)
	header := lipgloss.NewStyle().Bold(true).Width(innerW).MaxWidth(innerW).Render(title)
	body := m.diffView.View()

	return paneStyle.Render(header + "\n\n" + body)
}

func (m *Model) resizePanes() {
	_, rightW := paneWidths(m.width)
	availableHeight := max(3, m.height-2)
	m.diffView.Width = max(1, rightW-4)
	m.diffView.Height = max(1, availableHeight-4)
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
	return func() tea.Msg {
		d, err := service.AllChangesDiff(context.Background(), cwd, path)
		return diffLoadedMsg{path: path, diff: d, err: err}
	}
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
