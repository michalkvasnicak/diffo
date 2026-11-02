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

// Prepare shares snapshot tokenization across both views. Annotated content
// uses current tokens for replacements and HEAD tokens for standalone deletions.
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
	result := Content{
		Lines: make([]string, len(parsed.Lines)),
		Full:  s.annotatedContent(parsed, change, file, oldTokens, newTokens),
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

func (s *ContentService) annotatedContent(parsed diff.Result, change git.Diff, file git.File, oldTokens, newTokens []highlight.Line) []string {
	snapshot, tokens := change.New, newTokens
	if file.Status == "D" {
		snapshot, tokens = change.Old, oldTokens
	}
	if !snapshot.Available || strings.ContainsRune(snapshot.Text, '\x00') {
		return s.fullContent(snapshot, tokens)
	}
	old := diff.Snapshot{Text: change.Old.Text, Available: change.Old.Available}
	current := diff.Snapshot{Text: change.New.Text, Available: change.New.Available}
	// Git intentionally has no snapshot on the absent side of a file addition
	// or deletion. Treat only those known absent sides as available empty text.
	if file.Status == "A" || file.Status == "?" {
		old.Available = true
	}
	if file.Status == "D" {
		current.Available = true
	}
	lines, mapped := diff.Annotate(parsed, old, current)
	if !mapped {
		return append([]string{"Annotations unavailable; showing snapshot without change backgrounds."}, s.fullContent(snapshot, tokens)...)
	}
	if len(lines) == 0 {
		return []string{"Empty file."}
	}
	result := make([]string, len(lines))
	for i, line := range lines {
		base := lipgloss.NewStyle()
		sourceTokens, source := newTokens, line.New-1
		if line.Deleted {
			base = s.deleted
			sourceTokens, source = oldTokens, line.Old-1
		}
		tokenLine := highlight.Plain(line.Text)
		if source >= 0 && source < len(sourceTokens) {
			tokenLine = sourceTokens[source]
		}
		spans := make([]highlight.Span, len(line.Added))
		for j, added := range line.Added {
			spans[j] = highlight.Span{Start: added.Start, End: added.End, Style: s.added}
		}
		result[i] = s.highlighter.Render(tokenLine, base, spans...)
	}
	return result
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
