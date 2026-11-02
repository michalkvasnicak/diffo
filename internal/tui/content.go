package tui

import (
	"sort"
	"strings"

	"diffo/internal/diff"
	"diffo/internal/git"
	"diffo/internal/highlight"
	"diffo/internal/terminaltext"

	"github.com/charmbracelet/lipgloss"
)

// ContentLine combines trusted ANSI text with source coordinates and hunk identity.
// Zero coordinates and hunk IDs mean no corresponding source line or hunk.
type ContentLine struct {
	Text    string
	Old     int
	New     int
	Deleted bool
	Edited  bool
	Hunk    int
}

// Content holds trusted ANSI lines prepared once per file, before wrapping.
// Hunks contains zero-based header positions in Lines; FullHunks contains
// the first displayed edit line for each mapped hunk in Full.
type Content struct {
	Lines     []ContentLine
	Hunks     []int
	Full      []ContentLine
	FullHunks []int
}

type ContentService struct {
	highlighter *highlight.Service
	added       lipgloss.Style
	changed     lipgloss.Style
	deleted     lipgloss.Style
	metadata    lipgloss.Style
}

func NewContentService(highlighter *highlight.Service) *ContentService {
	return &ContentService{
		highlighter: highlighter,
		added:       lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#ddf4df", Dark: "#173524"}),
		changed:     lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#ffdfb5", Dark: "#4a3018"}),
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
	result := Content{Lines: make([]ContentLine, len(parsed.Lines))}
	result.Full, result.FullHunks = s.annotatedContent(parsed, change, file, oldTokens, newTokens)
	for _, hunk := range parsed.Hunks {
		result.Hunks = append(result.Hunks, hunk.Line)
	}
	hunk, nextHunk := 0, 0
	for i, line := range parsed.Lines {
		if nextHunk < len(parsed.Hunks) && parsed.Hunks[nextHunk].Line == i {
			nextHunk++
			hunk = nextHunk
		} else if line.Kind == 0 && !strings.HasPrefix(line.Text, "\\") {
			hunk = 0
		}
		prepared := ContentLine{
			Text: s.diffLine(line, oldTokens, newTokens), Hunk: hunk,
			Edited: line.Kind == '+' || line.Kind == '-',
		}
		if line.Mapped {
			prepared.Old, prepared.New = line.Old, line.New
		}
		result.Lines[i] = prepared
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

func (s *ContentService) annotatedContent(parsed diff.Result, change git.Diff, file git.File, oldTokens, newTokens []highlight.Line) ([]ContentLine, []int) {
	snapshot, tokens := change.New, newTokens
	if file.Status == "D" {
		snapshot, tokens = change.Old, oldTokens
	}
	if !snapshot.Available || strings.ContainsRune(snapshot.Text, '\x00') {
		return s.fullContent(snapshot, tokens, file.Status == "D"), nil
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
		return append([]ContentLine{{Text: "Annotations unavailable; showing snapshot without change backgrounds."}}, s.fullContent(snapshot, tokens, file.Status == "D")...), nil
	}
	if len(lines) == 0 {
		return []ContentLine{{Text: "Empty file."}}, nil
	}
	result := make([]ContentLine, len(lines))
	var hunks []int
	for i, line := range lines {
		if line.HunkStart {
			hunks = append(hunks, i)
		}
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
		spans := make([]highlight.Span, 0, len(line.Added)+len(line.Changed))
		for _, added := range line.Added {
			spans = append(spans, highlight.Span{Start: added.Start, End: added.End, Style: s.added})
		}
		for _, changed := range line.Changed {
			spans = append(spans, highlight.Span{Start: changed.Start, End: changed.End, Style: s.changed})
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].Start < spans[j].Start })
		result[i] = ContentLine{
			Text: s.highlighter.Render(tokenLine, base, spans...),
			Old:  line.Old, New: line.New, Deleted: line.Deleted, Edited: line.Edited, Hunk: line.Hunk,
		}
	}
	return result, hunks
}

func (s *ContentService) fullContent(snapshot git.Snapshot, tokens []highlight.Line, deleted bool) []ContentLine {
	switch {
	case !snapshot.Available:
		return []ContentLine{{Text: "File content unavailable for this snapshot."}}
	case strings.ContainsRune(snapshot.Text, '\x00'):
		return []ContentLine{{Text: "Binary file; content is not displayed."}}
	case snapshot.Text == "":
		return []ContentLine{{Text: "Empty file."}}
	}
	lines := strings.Split(strings.TrimSuffix(snapshot.Text, "\n"), "\n")
	result := make([]ContentLine, len(lines))
	for i, line := range lines {
		text := terminaltext.Display(line)
		if i < len(tokens) {
			text = s.highlighter.Render(tokens[i], lipgloss.NewStyle())
		}
		result[i] = ContentLine{Text: text, New: i + 1}
		if deleted {
			result[i].Old, result[i].New, result[i].Deleted = i+1, 0, true
		}
	}
	return result
}
