// Package tui presents a read-only, single-pane change browser.
package tui

import (
	"fmt"
	"strings"

	"diffo/internal/git"
	"diffo/internal/highlight"

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
	diff    highlight.Result
	err     error
}

type Model struct {
	highlighter *highlight.Service
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
	diff        highlight.Result
	rows        []string
	hunkRows    []int
	hunkTarget  int
	offset      int
	request     int
}

func New(discover func() (*git.Repository, error), highlighter *highlight.Service) *Model {
	return &Model{discover: discover, highlighter: highlighter, loading: true, hunkTarget: -1}
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
	m.inDiff, m.loading = true, true
	m.diff, m.rows, m.hunkRows = highlight.Result{}, nil, nil
	m.err, m.offset, m.hunkTarget = nil, 0, -1
	m.clamp()
	m.request++
	request, repository, file, highlighter := m.request, m.repository, m.files[m.selection], m.highlighter
	return func() tea.Msg {
		diff, err := repository.Diff(file)
		if err != nil {
			return diffLoaded{request: request, err: err}
		}
		return diffLoaded{request: request, diff: highlighter.Lines(diff)}
	}
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.hunkTarget = -1
		m.wrapDiff()
	case filesLoaded:
		m.repository, m.files, m.err, m.loading = msg.repository, msg.files, msg.err, false
		m.selection = min(m.selection, max(0, len(m.files)-1))
	case diffLoaded:
		if msg.request != m.request || !m.inDiff {
			return m, nil
		}
		m.diff, m.err, m.loading = msg.diff, msg.err, false
		m.wrapDiff()
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
			m.move(-max(len(m.rows), len(m.files)))
		case "G", "end":
			m.move(max(len(m.rows), len(m.files)))
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
		m.offset += delta
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
	if !m.inDiff || m.loading || m.err != nil || len(m.hunkRows) == 0 {
		return
	}
	top := m.offset
	// At EOF several hunks can share a clamped scroll offset. Remember the
	// requested header so repeated jumps still advance, and reversing goes back.
	if m.hunkTarget >= 0 {
		top = max(top, m.hunkRows[m.hunkTarget])
	}
	if direction > 0 {
		for i, row := range m.hunkRows {
			if row > top {
				m.hunkTarget, m.offset = i, row
				m.clamp()
				return
			}
		}
	} else {
		for i := len(m.hunkRows) - 1; i >= 0; i-- {
			if m.hunkRows[i] < top {
				m.hunkTarget, m.offset = i, m.hunkRows[i]
				m.clamp()
				return
			}
		}
	}
}

func (m *Model) clamp() {
	m.selection = max(0, min(m.selection, len(m.files)-1))
	m.offset = max(0, min(m.offset, len(m.rows)-m.bodyHeight()))
	if m.selection < m.first {
		m.first = m.selection
	}
	if m.selection >= m.first+m.bodyHeight() {
		m.first = max(0, m.selection-m.bodyHeight()+1)
	}
	m.first = max(0, min(m.first, len(m.files)-m.bodyHeight()))
}

func singleLine(text string) string {
	return strings.NewReplacer("\n", "\\n", "\t", "\\t").Replace(highlight.VisibleText(text))
}

func (m *Model) wrapDiff() {
	m.rows, m.hunkRows = nil, nil
	if m.width < 1 || len(m.diff.Lines) == 0 {
		m.clamp()
		return
	}
	hunk := 0
	for i, line := range m.diff.Lines {
		if hunk < len(m.diff.Hunks) && m.diff.Hunks[hunk] == i {
			m.hunkRows = append(m.hunkRows, len(m.rows))
			hunk++
		}
		// Wrap plain graphemes first, then cut the trusted ANSI line at those
		// cell boundaries. Cut retains SGR state, so each viewport row stands
		// alone even when scrolling starts in the middle of a styled token.
		column := 0
		for _, plain := range strings.Split(ansi.Hardwrap(ansi.Strip(line), m.width, true), "\n") {
			width := ansi.StringWidth(plain)
			row := ansi.Cut(line, column, column+width)
			// A single wide grapheme may not fit a one-cell terminal.
			m.rows = append(m.rows, ansi.Truncate(row, m.width, ""))
			column += width
		}
	}
	m.clamp()
}

func (m *Model) View() string {
	if m.width < 1 || m.height < 1 {
		return ""
	}
	header := fmt.Sprintf("diffo · HEAD → worktree · %d files", len(m.files))
	help := "j/k move · enter open · r refresh · q quit"
	if m.inDiff {
		if len(m.files) > 0 {
			header = m.files[m.selection].Status + " " + singleLine(m.files[m.selection].Path)
		}
		help = "n/p hunk · N/P file · j/k scroll · h/esc back · r refresh · q quit"
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
		body = strings.Split(ansi.Hardwrap("Error: "+singleLine(m.err.Error())+" (r to retry)", m.width, true), "\n")
	case m.inDiff:
		if len(m.rows) == 0 {
			body = []string{"No diff; r refreshes the file list."}
		} else {
			body = m.rows[m.offset:min(len(m.rows), m.offset+m.bodyHeight())]
		}
	case len(m.files) == 0:
		body = []string{"No changes. r refreshes."}
	default:
		for i := m.first; i < min(len(m.files), m.first+m.bodyHeight()); i++ {
			file := m.files[i]
			path := singleLine(file.Path)
			if file.OldPath != "" {
				path += " ← " + singleLine(file.OldPath)
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
