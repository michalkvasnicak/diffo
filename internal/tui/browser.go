package tui

import (
	"fmt"

	"diffo/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

// browserModel owns the file tree screen and its loading/error state.
type browserModel struct {
	layout  screenLayout
	files   []git.File
	tree    fileTree
	loading bool
	err     error
}

func (m *browserModel) resize(width, height int) {
	m.layout = screenLayout{width: width, height: height}
	m.tree.clamp(m.layout.bodyHeight())
}

func (m *browserModel) loaded(files []git.File, err error) {
	m.files, m.err, m.loading = files, err, false
	m.tree.rebuild(files)
	m.tree.clamp(m.layout.bodyHeight())
}

func (m *browserModel) selectFile(index int) {
	m.tree.selectFile(index)
	m.tree.clamp(m.layout.bodyHeight())
}

func (m *browserModel) update(message tea.Msg) tea.Msg {
	var navigation tea.Msg
	switch msg := message.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "h", "left":
			m.tree.collapse()
		case "j", "down":
			m.tree.selection++
		case "k", "up":
			m.tree.selection--
		case "pgdown", "ctrl+f", " ":
			m.tree.selection += m.layout.bodyHeight()
		case "pgup", "ctrl+b":
			m.tree.selection -= m.layout.bodyHeight()
		case "g", "home":
			m.tree.selection = 0
		case "G", "end":
			m.tree.selection = len(m.tree.rows) - 1
		case "enter", "l", "right":
			navigation = m.activateTree(msg.String() == "enter")
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.tree.selection -= 3
		case tea.MouseButtonWheelDown:
			m.tree.selection += 3
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionPress && !m.loading && m.err == nil && msg.Y >= 1 && msg.Y < m.layout.height-1 {
				index := m.tree.first + msg.Y - 1
				if index < len(m.tree.rows) {
					m.tree.selection = index
					navigation = m.activateTree(true)
				}
			}
		}
	}
	m.tree.clamp(m.layout.bodyHeight())
	return navigation
}

func (m *browserModel) activateTree(toggle bool) tea.Msg {
	if m.loading || m.err != nil {
		return nil
	}
	node := m.tree.selectedNode()
	if node == nil {
		return nil
	}
	if node.file >= 0 {
		return openFileRequest{index: node.file}
	}
	if toggle || node.collapsed {
		node.collapsed = !node.collapsed
		m.tree.flatten()
	} else {
		m.tree.selection++
	}
	return nil
}

func (m *browserModel) view() string {
	if m.layout.width < 1 || m.layout.height < 1 {
		return ""
	}
	header := fmt.Sprintf("HEAD → worktree · %d files", len(m.files))
	help := "j/k move · h/l collapse/expand · enter toggle/open · r refresh · q quit"
	var body []string
	switch {
	case m.loading:
		body = []string{"Loading…"}
	case m.err != nil:
		body = m.layout.errorLines(m.err)
	case len(m.files) == 0:
		body = []string{"No changes. r refreshes."}
	default:
		body = m.tree.render(m.files, m.layout.width, m.layout.bodyHeight())
	}
	return m.layout.render(header, body, help)
}
