// Package tui presents a read-only, single-pane change browser.
package tui

import (
	"fmt"
	"strings"

	"diffo/internal/git"
	"diffo/internal/terminaltext"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selected = lipgloss.NewStyle().Reverse(true)
)

type filesLoaded struct {
	repository *git.Repository
	files      []git.File
	err        error
}

type diffLoaded struct {
	request int
	diff    Content
	err     error
}

type fileView struct {
	rows   []string
	offset int
}

type Model struct {
	content     *ContentService
	discover    func() (*git.Repository, error)
	repository  *git.Repository
	files       []git.File
	selection   int
	first       int
	width       int
	height      int
	loading     bool
	err         error
	inDiff      bool
	diff        Content
	fullContent bool
	diffView    fileView
	contentView fileView
	hunkRows    []int
	hunkTarget  int
	request     int
}

func New(discover func() (*git.Repository, error), content *ContentService) *Model {
	return &Model{discover: discover, content: content, loading: true, hunkTarget: -1}
}

func (m *Model) Init() tea.Cmd { return m.loadFiles() }

func (m *Model) loadFiles() tea.Cmd {
	repository := m.repository
	discover := m.discover
	return func() tea.Msg {
		var err error
		if repository == nil {
			repository, err = discover()
			if err != nil {
				return filesLoaded{err: err}
			}
		}
		files, err := repository.Files()
		return filesLoaded{repository: repository, files: files, err: err}
	}
}

func (m *Model) open() tea.Cmd {
	if len(m.files) == 0 || (!m.inDiff && (m.loading || m.err != nil)) {
		return nil
	}
	if !m.inDiff {
		m.fullContent = false
	}
	m.inDiff, m.loading = true, true
	m.diff, m.hunkRows = Content{}, nil
	m.diffView, m.contentView = fileView{}, fileView{}
	m.err, m.hunkTarget = nil, -1
	m.clamp()
	m.request++
	request, repository, file, content := m.request, m.repository, m.files[m.selection], m.content
	return func() tea.Msg {
		diff, err := repository.Diff(file)
		if err != nil {
			return diffLoaded{request: request, err: err}
		}
		return diffLoaded{request: request, diff: content.Prepare(diff, file)}
	}
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.hunkTarget = -1
		m.wrapViews()
	case filesLoaded:
		m.repository, m.files, m.err, m.loading = msg.repository, msg.files, msg.err, false
		m.selection = min(m.selection, max(0, len(m.files)-1))
	case diffLoaded:
		if msg.request != m.request || !m.inDiff {
			return m, nil
		}
		m.diff, m.err, m.loading = msg.diff, msg.err, false
		m.wrapViews()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "h", "esc", "left":
			if m.inDiff {
				m.inDiff, m.loading, m.err = false, false, nil
				m.request++
			}
		case "r":
			if !m.loading || m.inDiff {
				m.inDiff, m.loading, m.err = false, true, nil
				m.request++
				return m, m.loadFiles()
			}
		case "v":
			if m.inDiff {
				m.fullContent = !m.fullContent
				m.hunkTarget = -1
			}
		case "n":
			m.jumpHunk(1)
		case "p":
			m.jumpHunk(-1)
		case "N", "tab":
			return m, m.switchFile(1)
		case "P", "shift+tab":
			return m, m.switchFile(-1)
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "pgdown", "ctrl+f", " ":
			m.move(m.bodyHeight())
		case "pgup", "ctrl+b":
			m.move(-m.bodyHeight())
		case "g", "home":
			m.move(-max(len(m.activeView().rows), len(m.files)))
		case "G", "end":
			m.move(max(len(m.activeView().rows), len(m.files)))
		case "enter", "l", "right":
			if !m.inDiff {
				return m, m.open()
			}
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.move(-3)
		case tea.MouseButtonWheelDown:
			m.move(3)
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionPress && !m.inDiff && !m.loading && m.err == nil && msg.Y >= 1 && msg.Y < m.height-1 {
				index := m.first + msg.Y - 1
				if index < len(m.files) {
					m.selection = index
					return m, m.open()
				}
			}
		}
	}
	m.clamp()
	return m, nil
}

func (m *Model) bodyHeight() int { return max(0, m.height-2) }

func (m *Model) move(delta int) {
	if m.inDiff {
		m.hunkTarget = -1
		m.activeView().offset += delta
	} else {
		m.selection += delta
	}
	m.clamp()
}

func (m *Model) switchFile(delta int) tea.Cmd {
	if !m.inDiff || len(m.files) == 0 {
		return nil
	}
	selection := m.selection + delta
	if selection < 0 || selection >= len(m.files) {
		return nil
	}
	m.selection = selection
	return m.open()
}

func (m *Model) jumpHunk(direction int) {
	if !m.inDiff || m.fullContent || m.loading || m.err != nil || len(m.hunkRows) == 0 {
		return
	}
	top := m.diffView.offset
	// At EOF several hunks can share a clamped scroll offset. Remember the
	// requested header so repeated jumps still advance, and reversing goes back.
	if m.hunkTarget >= 0 {
		top = max(top, m.hunkRows[m.hunkTarget])
	}
	if direction > 0 {
		for i, row := range m.hunkRows {
			if row > top {
				m.hunkTarget, m.diffView.offset = i, row
				m.clamp()
				return
			}
		}
	} else {
		for i := len(m.hunkRows) - 1; i >= 0; i-- {
			if m.hunkRows[i] < top {
				m.hunkTarget, m.diffView.offset = i, m.hunkRows[i]
				m.clamp()
				return
			}
		}
	}
}

func (m *Model) clamp() {
	m.selection = max(0, min(m.selection, len(m.files)-1))
	for _, view := range []*fileView{&m.diffView, &m.contentView} {
		view.offset = max(0, min(view.offset, len(view.rows)-m.bodyHeight()))
	}
	if m.selection < m.first {
		m.first = m.selection
	}
	if m.selection >= m.first+m.bodyHeight() {
		m.first = max(0, m.selection-m.bodyHeight()+1)
	}
	m.first = max(0, min(m.first, len(m.files)-m.bodyHeight()))
}

func (m *Model) activeView() *fileView {
	if m.fullContent {
		return &m.contentView
	}
	return &m.diffView
}

func (m *Model) wrapViews() {
	m.diffView.rows, m.hunkRows = wrapLines(m.diff.Lines, m.diff.Hunks, m.width)
	m.contentView.rows, _ = wrapLines(m.diff.Full, nil, m.width)
	m.clamp()
}

func wrapLines(lines []string, hunks []int, width int) (rows []string, hunkRows []int) {
	if width < 1 {
		return nil, nil
	}
	hunk := 0
	for i, line := range lines {
		if hunk < len(hunks) && hunks[hunk] == i {
			hunkRows = append(hunkRows, len(rows))
			hunk++
		}
		// Wrap plain graphemes first, then cut the trusted ANSI line at those
		// cell boundaries. Cut retains SGR state, so each viewport row stands
		// alone even when scrolling starts in the middle of a styled token.
		column := 0
		for _, plain := range strings.Split(ansi.Hardwrap(ansi.Strip(line), width, true), "\n") {
			cells := ansi.StringWidth(plain)
			row := ansi.Cut(line, column, column+cells)
			// A single wide grapheme may not fit a one-cell terminal.
			rows = append(rows, ansi.Truncate(row, width, ""))
			column += cells
		}
	}
	return rows, hunkRows
}

func (m *Model) View() string {
	if m.width < 1 || m.height < 1 {
		return ""
	}
	header := fmt.Sprintf("diffo · HEAD → worktree · %d files", len(m.files))
	help := "j/k move · enter open · r refresh · q quit"
	if m.inDiff {
		mode := "DIFF · HEAD → current"
		if m.fullContent {
			mode = "FILE · current"
			if len(m.files) > 0 && m.files[m.selection].Status == "D" {
				mode = "FILE · HEAD"
			}
		}
		header = mode
		if len(m.files) > 0 {
			header += " · " + m.files[m.selection].Status + " " + terminaltext.SingleLine(m.files[m.selection].Path)
		}
		help = "v diff/file · n/p hunk · N/P file · j/k scroll · h/esc back · r refresh · q quit"
	}
	lines := []string{ansi.Truncate(header, m.width, "…")}
	if m.height == 1 {
		return lines[0]
	}
	var body []string
	switch {
	case m.loading:
		body = []string{"Loading…"}
	case m.err != nil:
		body = strings.Split(ansi.Hardwrap("Error: "+terminaltext.SingleLine(m.err.Error())+" (r to retry)", m.width, true), "\n")
	case m.inDiff:
		view := m.activeView()
		if len(view.rows) == 0 {
			body = []string{"No diff; r refreshes the file list."}
		} else {
			body = view.rows[view.offset:min(len(view.rows), view.offset+m.bodyHeight())]
		}
	case len(m.files) == 0:
		body = []string{"No changes. r refreshes."}
	default:
		for i := m.first; i < min(len(m.files), m.first+m.bodyHeight()); i++ {
			file := m.files[i]
			path := terminaltext.SingleLine(file.Path)
			if file.OldPath != "" {
				path += " ← " + terminaltext.SingleLine(file.OldPath)
			}
			marker := "  "
			if i == m.selection {
				marker = "> "
			}
			row := ansi.Truncate(marker+file.Status+" "+path, m.width, "…")
			if i == m.selection {
				row = selected.Render(row)
			}
			body = append(body, row)
		}
	}
	for i := 0; i < m.bodyHeight(); i++ {
		row := ""
		if i < len(body) {
			row = ansi.Truncate(body[i], m.width, "")
		}
		lines = append(lines, row)
	}
	lines = append(lines, muted.Render(ansi.Truncate(help, m.width, "…")))
	return strings.Join(lines, "\n")
}
