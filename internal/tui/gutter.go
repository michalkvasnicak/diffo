package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var activeHunk = lipgloss.NewStyle().
	Foreground(lipgloss.AdaptiveColor{Light: "#07546b", Dark: "#7dd3e8"}).
	Background(lipgloss.AdaptiveColor{Light: "#d8f2f7", Dark: "#163e49"}).
	Bold(true)

type contentRow struct {
	text   string
	gutter string
	hunk   int
	edited bool
}

type fileView struct {
	rows   []contentRow
	hunks  []int
	offset int
}

// wrapView reserves gutter space before wrapping content. Only the first
// terminal row of each source line receives a line number.
func wrapView(lines []ContentLine, targets []int, width int, full bool) fileView {
	view := fileView{}
	if width < 1 {
		return view
	}
	maxOld, maxNew := 0, 0
	for _, line := range lines {
		maxOld, maxNew = max(maxOld, line.Old), max(maxNew, line.New)
	}
	oldWidth, newWidth := len(strconv.Itoa(maxOld)), len(strconv.Itoa(maxNew))
	numberWidth := oldWidth + 1 + newWidth
	if full {
		numberWidth = max(oldWidth, newWidth) + 1 // Reserve '-' for old/deleted lines.
	}
	numbers := maxOld > 0 || maxNew > 0
	gutterWidth := numberWidth + 3 // Separator: " │ ".
	if !numbers || width-gutterWidth < 1 {
		numbers, gutterWidth = false, 0
	}
	contentWidth := width - gutterWidth
	target := 0
	for i, line := range lines {
		if target < len(targets) && targets[target] == i {
			view.hunks = append(view.hunks, len(view.rows))
			target++
		}
		gutter := ""
		if numbers {
			if full {
				number := lineNumber(line.New)
				if line.Deleted && line.Old > 0 {
					number = "-" + lineNumber(line.Old)
				}
				gutter = fmt.Sprintf("%*s │ ", numberWidth, number)
			} else {
				gutter = fmt.Sprintf("%*s %*s │ ", oldWidth, lineNumber(line.Old), newWidth, lineNumber(line.New))
			}
		}
		// Cut retains SGR state, so each wrapped row stands alone even when
		// scrolling starts inside a syntax token or a change background.
		column := 0
		for _, plain := range strings.Split(ansi.Hardwrap(ansi.Strip(line.Text), contentWidth, true), "\n") {
			cells := ansi.StringWidth(plain)
			text := ansi.Truncate(ansi.Cut(line.Text, column, column+cells), contentWidth, "")
			view.rows = append(view.rows, contentRow{text: text, gutter: gutter, hunk: line.Hunk, edited: line.Edited})
			column += cells
			if numbers {
				gutter = strings.Repeat(" ", numberWidth) + " │ "
			}
		}
	}
	return view
}

func lineNumber(number int) string {
	if number == 0 {
		return ""
	}
	return strconv.Itoa(number)
}

func (v *fileView) visibleRows(height, selectedHunk int) []string {
	var rows []string
	for i := v.offset; i < min(len(v.rows), v.offset+height); i++ {
		row := v.rows[i]
		style := muted
		if selectedHunk > 0 && row.hunk == selectedHunk && row.edited {
			style = activeHunk
		}
		rows = append(rows, style.Render(row.gutter)+row.text)
	}
	return rows
}
