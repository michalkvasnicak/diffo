package tui

import (
	"strings"

	"diffo/internal/git"
	"diffo/internal/terminaltext"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

type fileView struct {
	rows   []string
	hunks  []int
	offset int
}

// viewerModel owns diff/annotated rendering, scrolling, and hunk navigation.
type viewerModel struct {
	layout      screenLayout
	index       int
	file        git.File
	loading     bool
	err         error
	diff        Content
	fullContent bool
	diffView    fileView
	contentView fileView
	hunkTarget  int
}

func (m *viewerModel) open(index int, file git.File, resetMode bool) {
	m.index, m.file = index, file
	if resetMode {
		m.fullContent = false
	}
	m.loading, m.err = true, nil
	m.diff = Content{}
	m.diffView, m.contentView = fileView{}, fileView{}
	m.hunkTarget = -1
}

func (m *viewerModel) loaded(content Content, err error) {
	m.diff, m.err, m.loading = content, err, false
	m.wrapViews()
}

func (m *viewerModel) resize(width, height int) {
	m.layout = screenLayout{width: width, height: height}
	m.hunkTarget = -1
	m.wrapViews()
}

func (m *viewerModel) update(message tea.Msg) tea.Msg {
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "esc", "left":
			return showBrowserRequest{}
		case "v":
			m.fullContent = !m.fullContent
			m.hunkTarget = -1
		case "n":
			m.jumpHunk(1)
		case "p":
			m.jumpHunk(-1)
		case "N", "tab":
			return switchFileRequest{delta: 1}
		case "P", "shift+tab":
			return switchFileRequest{delta: -1}
		case "j", "down":
			m.move(1)
		case "k", "up":
			m.move(-1)
		case "pgdown", "ctrl+f", " ":
			m.move(m.layout.bodyHeight())
		case "pgup", "ctrl+b":
			m.move(-m.layout.bodyHeight())
		case "g", "home":
			m.move(-len(m.activeView().rows))
		case "G", "end":
			m.move(len(m.activeView().rows))
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.move(-3)
		case tea.MouseButtonWheelDown:
			m.move(3)
		}
	}
	m.clamp()
	return nil
}

func (m *viewerModel) move(delta int) {
	m.hunkTarget = -1
	m.activeView().offset += delta
	m.clamp()
}

func (m *viewerModel) jumpHunk(direction int) {
	view := m.activeView()
	if m.loading || m.err != nil || len(view.hunks) == 0 {
		return
	}
	top := view.offset
	// At EOF several hunks can share a clamped scroll offset. Remember the
	// requested target so repeated jumps still advance, and reversing goes back.
	if m.hunkTarget >= 0 {
		top = max(top, view.hunks[m.hunkTarget])
	}
	if direction > 0 {
		for i, row := range view.hunks {
			if row > top {
				m.hunkTarget, view.offset = i, row
				m.clamp()
				return
			}
		}
	} else {
		for i := len(view.hunks) - 1; i >= 0; i-- {
			if view.hunks[i] < top {
				m.hunkTarget, view.offset = i, view.hunks[i]
				m.clamp()
				return
			}
		}
	}
}

func (m *viewerModel) clamp() {
	for _, view := range []*fileView{&m.diffView, &m.contentView} {
		view.offset = max(0, min(view.offset, len(view.rows)-m.layout.bodyHeight()))
	}
}

func (m *viewerModel) activeView() *fileView {
	if m.fullContent {
		return &m.contentView
	}
	return &m.diffView
}

func (m *viewerModel) wrapViews() {
	m.diffView.rows, m.diffView.hunks = wrapLines(m.diff.Lines, m.diff.Hunks, m.layout.width)
	m.contentView.rows, m.contentView.hunks = wrapLines(m.diff.Full, m.diff.FullHunks, m.layout.width)
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

func (m *viewerModel) view() string {
	if m.layout.width < 1 || m.layout.height < 1 {
		return ""
	}
	header := "DIFF · HEAD → current"
	if m.fullContent {
		header = "ANNOTATED FILE · current + deletions"
		if m.file.Status == "D" {
			header = "ANNOTATED FILE · HEAD deletions"
		}
	}
	header += " · " + m.file.Status + " " + terminaltext.SingleLine(m.file.Path)
	help := "v diff/annotated · n/p hunk · N/P file · j/k scroll · h/esc back · r refresh · q quit"
	var body []string
	switch {
	case m.loading:
		body = []string{"Loading…"}
	case m.err != nil:
		body = m.layout.errorLines(m.err)
	default:
		view := m.activeView()
		if len(view.rows) == 0 {
			body = []string{"No diff; r refreshes the file list."}
		} else {
			body = view.rows[view.offset:min(len(view.rows), view.offset+m.layout.bodyHeight())]
		}
	}
	return m.layout.render(header, body, help)
}
