package tui

import (
	"strings"

	"diffo/internal/diff"
	"diffo/internal/git"
	"diffo/internal/highlight"
	"diffo/internal/terminaltext"

	"github.com/charmbracelet/lipgloss"
)

// Content holds trusted ANSI lines prepared once per file, before wrapping.
// Hunks contains zero-based header positions in Lines, never in Full.
type Content struct {
	Lines []string
	Hunks []int
	Full  []string
}

type ContentService struct {
	highlighter *highlight.Service
	added       lipgloss.Style
	deleted     lipgloss.Style
	metadata    lipgloss.Style
}

func NewContentService(highlighter *highlight.Service) *ContentService {
	return &ContentService{
		highlighter: highlighter,
		added:       lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#ddf4df", Dark: "#173524"}),
		deleted:     lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#ffebe9", Dark: "#442329"}),
		metadata:    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
}

// Prepare shares snapshot tokenization across both views. Only deleted files
// select HEAD for full content; other files always select the current snapshot.
func (s *ContentService) Prepare(change git.Diff, file git.File) Content {
	parsed := diff.Parse(change.Text,
		diff.Snapshot{Text: change.Old.Text, Available: change.Old.Available},
		diff.Snapshot{Text: change.New.Text, Available: change.New.Available})
	needsOld, needsNew := file.Status == "D", file.Status != "D"
	for _, line := range parsed.Lines {
		if line.Mapped {
			if line.Kind == '-' {
				needsOld = true
			} else {
				needsNew = true
			}
		}
	}
	var oldTokens, newTokens []highlight.Line
	if needsOld && change.Old.Available {
		oldTokens = s.highlighter.Source(change.Old.Path, change.Old.Text)
	}
	if needsNew && change.New.Available {
		newTokens = s.highlighter.Source(change.New.Path, change.New.Text)
	}
	snapshot, tokens := change.New, newTokens
	if file.Status == "D" {
		snapshot, tokens = change.Old, oldTokens
	}
	result := Content{
		Lines: make([]string, len(parsed.Lines)),
		Full:  s.fullContent(snapshot, tokens),
	}
	for _, hunk := range parsed.Hunks {
		result.Hunks = append(result.Hunks, hunk.Line)
	}
	for i, line := range parsed.Lines {
		result.Lines[i] = s.diffLine(line, oldTokens, newTokens)
	}
	return result
}

func (s *ContentService) diffLine(line diff.Line, oldTokens, newTokens []highlight.Line) string {
	base := lipgloss.NewStyle()
	switch line.Kind {
	case '+':
		base = s.added
	case '-':
		base = s.deleted
	case ' ':
	default:
		base = s.metadata
	}
	tokens, source := newTokens, line.New-1
	if line.Kind == '-' {
		tokens, source = oldTokens, line.Old-1
	}
	if !line.Mapped || source < 0 || source >= len(tokens) {
		return base.Render(terminaltext.Display(line.Text))
	}
	return base.Render(string(line.Kind)) + s.highlighter.Render(tokens[source], base)
}

func (s *ContentService) fullContent(snapshot git.Snapshot, tokens []highlight.Line) []string {
	switch {
	case !snapshot.Available:
		return []string{"File content unavailable for this snapshot."}
	case strings.ContainsRune(snapshot.Text, '\x00'):
		return []string{"Binary file; content is not displayed."}
	case snapshot.Text == "":
		return []string{"Empty file."}
	}
	lines := strings.Split(strings.TrimSuffix(snapshot.Text, "\n"), "\n")
	for i, line := range lines {
		if i < len(tokens) {
			lines[i] = s.highlighter.Render(tokens[i], lipgloss.NewStyle())
		} else {
			lines[i] = terminaltext.Display(line)
		}
	}
	return lines
}
