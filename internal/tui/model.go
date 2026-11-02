// Package tui presents a read-only, single-pane change browser.
package tui

import (
	"diffo/internal/git"

	tea "github.com/charmbracelet/bubbletea"
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

// Screens return navigation requests synchronously; only the root starts I/O.
type openFileRequest struct{ index int }
type switchFileRequest struct{ delta int }
type showBrowserRequest struct{}

type screen int

const (
	browserScreen screen = iota
	viewerScreen
)

// Model coordinates screens, navigation, and asynchronous service calls.
type Model struct {
	content    *ContentService
	discover   func() (*git.Repository, error)
	repository *git.Repository
	browser    browserModel
	viewer     viewerModel
	active     screen
	request    int
}

func New(discover func() (*git.Repository, error), content *ContentService) *Model {
	return &Model{
		discover: discover,
		content:  content,
		browser:  browserModel{loading: true},
		viewer:   viewerModel{hunkTarget: -1},
	}
}

func (m *Model) Init() tea.Cmd { return m.loadFiles() }

func (m *Model) loadFiles() tea.Cmd {
	repository, discover := m.repository, m.discover
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

func (m *Model) openFile(index int) tea.Cmd {
	file := m.browser.files[index]
	m.active = viewerScreen
	m.viewer.open(index, file)
	m.request++
	request, repository, content := m.request, m.repository, m.content
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
		m.browser.resize(msg.Width, msg.Height)
		m.viewer.resize(msg.Width, msg.Height)
		return m, nil
	case filesLoaded:
		m.repository = msg.repository
		m.browser.loaded(msg.files, msg.err)
		return m, nil
	case diffLoaded:
		if msg.request == m.request && m.active == viewerScreen {
			m.viewer.loaded(msg.diff, msg.err)
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			if m.active == viewerScreen || !m.browser.loading {
				m.active = browserScreen
				m.browser.loading, m.browser.err = true, nil
				m.request++
				return m, m.loadFiles()
			}
			return m, nil
		}
	}

	var navigation tea.Msg
	switch m.active {
	case browserScreen:
		navigation = m.browser.update(message)
	case viewerScreen:
		navigation = m.viewer.update(message)
	}
	switch request := navigation.(type) {
	case openFileRequest:
		return m, m.openFile(request.index)
	case switchFileRequest:
		index := m.viewer.index + request.delta
		if index >= 0 && index < len(m.browser.files) {
			return m, m.openFile(index)
		}
	case showBrowserRequest:
		m.active = browserScreen
		m.request++
		m.browser.selectFile(m.viewer.index)
	}
	return m, nil
}

func (m *Model) View() string {
	if m.active == viewerScreen {
		return m.viewer.view()
	}
	return m.browser.view()
}
