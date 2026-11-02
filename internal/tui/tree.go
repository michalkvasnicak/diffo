package tui

import (
	"fmt"
	"sort"
	"strings"

	"diffo/internal/git"
	"diffo/internal/terminaltext"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type treeNode struct {
	name      string
	path      string
	file      int // -1 for directories; indexes the unchanged Git file list.
	parent    *treeNode
	children  []*treeNode
	statuses  map[string]int
	count     int
	collapsed bool
}

type treeRow struct {
	node  *treeNode
	depth int
}

type fileTree struct {
	roots     []*treeNode
	rows      []treeRow
	selection int
	first     int
}

func (t *fileTree) selectedNode() *treeNode {
	if len(t.rows) == 0 {
		return nil
	}
	return t.rows[t.selection].node
}

func nodeKey(n *treeNode) string {
	if n.file < 0 {
		return "directory:" + n.path
	}
	status := ""
	for value := range n.statuses {
		status = value
	}
	return status + ":" + n.path
}

func (t *fileTree) rebuild(files []git.File) {
	key := ""
	if node := t.selectedNode(); node != nil {
		key = nodeKey(node)
	}
	collapsed := make(map[string]bool)
	var remember func([]*treeNode)
	remember = func(nodes []*treeNode) {
		for _, node := range nodes {
			if node.file < 0 {
				collapsed[node.path] = node.collapsed
				remember(node.children)
			}
		}
	}
	remember(t.roots)
	t.roots = nil
	directories := make(map[string]*treeNode)
	for index, file := range files {
		parts := strings.Split(file.Path, "/")
		var parent *treeNode
		for depth, name := range parts {
			path := strings.Join(parts[:depth+1], "/")
			var node *treeNode
			if depth < len(parts)-1 {
				node = directories[path]
			}
			if node == nil {
				node = &treeNode{name: name, path: path, file: -1, parent: parent, statuses: make(map[string]int)}
				if depth == len(parts)-1 {
					node.file = index
				} else {
					directories[path] = node
					node.collapsed = collapsed[path]
				}
				if parent == nil {
					t.roots = append(t.roots, node)
				} else {
					parent.children = append(parent.children, node)
				}
			}
			node.count++
			node.statuses[file.Status]++
			parent = node
		}
	}
	var sortNodes func([]*treeNode)
	sortNodes = func(nodes []*treeNode) {
		sort.SliceStable(nodes, func(i, j int) bool {
			if (nodes[i].file < 0) != (nodes[j].file < 0) {
				return nodes[i].file < 0
			}
			return nodes[i].name < nodes[j].name
		})
		for _, node := range nodes {
			sortNodes(node.children)
		}
	}
	sortNodes(t.roots)
	t.flatten()
	for i, row := range t.rows {
		if nodeKey(row.node) == key {
			t.selection = i
			break
		}
	}
}

func (t *fileTree) flatten() {
	t.rows = nil
	var visit func([]*treeNode, int)
	visit = func(nodes []*treeNode, depth int) {
		for _, node := range nodes {
			t.rows = append(t.rows, treeRow{node: node, depth: depth})
			if !node.collapsed {
				visit(node.children, depth+1)
			}
		}
	}
	visit(t.roots, 0)
	t.selection = max(0, min(t.selection, len(t.rows)-1))
}

func (t *fileTree) clamp(height int) {
	t.selection = max(0, min(t.selection, len(t.rows)-1))
	if t.selection < t.first {
		t.first = t.selection
	}
	if t.selection >= t.first+height {
		t.first = max(0, t.selection-height+1)
	}
	t.first = max(0, min(t.first, len(t.rows)-height))
}

func (t *fileTree) collapse() {
	node := t.selectedNode()
	if node == nil {
		return
	}
	if node.file < 0 && !node.collapsed {
		node.collapsed = true
		t.flatten()
	} else if node.parent != nil {
		for i, row := range t.rows {
			if row.node == node.parent {
				t.selection = i
				break
			}
		}
	}
}

func (t *fileTree) selectFile(index int) {
	var target *treeNode
	var visit func([]*treeNode)
	visit = func(nodes []*treeNode) {
		for _, node := range nodes {
			if node.file == index {
				target = node
			}
			visit(node.children)
		}
	}
	visit(t.roots)
	if target == nil {
		return
	}
	for parent := target.parent; parent != nil; parent = parent.parent {
		parent.collapsed = false
	}
	t.flatten()
	for i, row := range t.rows {
		if row.node == target {
			t.selection = i
			break
		}
	}
}

func statusBadge(status string) string {
	color := "8"
	switch status {
	case "A":
		color = "2"
	case "M", "T":
		color = "3"
	case "D":
		color = "1"
	case "R", "C":
		color = "6"
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(status)
}

func (t *fileTree) render(files []git.File, width, height int) []string {
	var lines []string
	for i := t.first; i < min(len(t.rows), t.first+height); i++ {
		row := t.rows[i]
		node := row.node
		marker := "  "
		if i == t.selection {
			marker = "> "
		}
		icon, name := "  ", terminaltext.SingleLine(node.name)
		var badges []string
		if node.file < 0 {
			icon = "▾ "
			if node.collapsed {
				icon = "▸ "
			}
			name += "/"
			badges = append(badges, muted.Render(fmt.Sprintf("%d", node.count)))
		} else if old := files[node.file].OldPath; old != "" {
			name += " ← " + terminaltext.SingleLine(old)
		}
		for _, status := range []string{"M", "A", "D", "R", "C", "T", "?"} {
			if node.statuses[status] > 0 {
				badges = append(badges, statusBadge(status))
			}
		}
		suffix := " " + strings.Join(badges, " ")
		labelWidth := max(0, width-ansi.StringWidth(suffix))
		label := ansi.Truncate(marker+strings.Repeat("  ", row.depth)+icon+name, labelWidth, "…")
		line := label + strings.Repeat(" ", max(0, labelWidth-ansi.StringWidth(label))) + suffix
		line = ansi.Truncate(line, width, "")
		if i == t.selection {
			line = selected.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}
