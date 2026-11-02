package tui

import (
	"strings"

	"diffo/internal/terminaltext"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	muted    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selected = lipgloss.NewStyle().Reverse(true)
)

// screenLayout keeps both screens' header, body, and footer sizing consistent.
type screenLayout struct {
	width  int
	height int
}

func (l screenLayout) bodyHeight() int { return max(0, l.height-2) }

func (l screenLayout) errorLines(err error) []string {
	return strings.Split(ansi.Hardwrap("Error: "+terminaltext.SingleLine(err.Error())+" (r to retry)", l.width, true), "\n")
}

func (l screenLayout) render(header string, body []string, help string) string {
	if l.width < 1 || l.height < 1 {
		return ""
	}
	lines := []string{ansi.Truncate(header, l.width, "…")}
	if l.height == 1 {
		return lines[0]
	}
	for i := 0; i < l.bodyHeight(); i++ {
		row := ""
		if i < len(body) {
			row = ansi.Truncate(body[i], l.width, "")
		}
		lines = append(lines, row)
	}
	lines = append(lines, muted.Render(ansi.Truncate(help, l.width, "…")))
	return strings.Join(lines, "\n")
}
