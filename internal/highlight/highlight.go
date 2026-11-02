// Package highlight maps full-file syntax tokens onto unified diff coordinates.
package highlight

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"diffo/internal/git"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

type Service struct {
	syntax   *chroma.Style
	added    lipgloss.Style
	deleted  lipgloss.Style
	metadata lipgloss.Style
}

func New() *Service {
	theme := "github"
	if lipgloss.HasDarkBackground() {
		theme = "github-dark"
	}
	return &Service{
		syntax:   styles.Get(theme),
		added:    lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#ddf4df", Dark: "#173524"}),
		deleted:  lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#ffebe9", Dark: "#442329"}),
		metadata: lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
	}
}

// VisibleText prevents repository content and filenames from emitting terminal controls.
func VisibleText(text string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t':
			return r
		default:
			if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
				return '\uFFFD'
			}
			return r
		}
	}, strings.ToValidUTF8(text, "\uFFFD"))
}

func displayText(text string) string {
	return strings.ReplaceAll(VisibleText(text), "\t", "    ")
}

type diffLine struct {
	text   string
	kind   byte
	source int
	mapped bool
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func sourceLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// Lines performs all tokenization and styling before the TUI wraps or renders.
// Every returned line is independently styled and contains only trusted ANSI.
func (s *Service) Lines(diff git.Diff) []string {
	if diff.Text == "" {
		return nil
	}
	lines := sourceLines(diff.Text)
	parsed := make([]diffLine, len(lines))
	oldLines, newLines := sourceLines(diff.Old.Text), sourceLines(diff.New.Text)
	// A pathspec can select more than one patch after a rename. Do not map those
	// patches to a single pair of snapshots, even if some lines happen to match.
	patchCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			patchCount++
		}
	}
	for i := 0; i < len(lines); {
		parsed[i] = diffLine{text: lines[i]}
		match := hunkHeader.FindStringSubmatch(lines[i])
		if match == nil {
			i++
			continue
		}
		old, oldCount, oldOK := coordinate(match[1], match[2])
		new, newCount, newOK := coordinate(match[3], match[4])
		start := i + 1
		i++
		valid := oldOK && newOK && patchCount <= 1
		for i < len(lines) && (oldCount > 0 || newCount > 0) {
			line := diffLine{text: lines[i]}
			if strings.HasPrefix(line.text, "\\") {
				parsed[i] = line
				i++
				continue
			}
			if line.text == "" || !strings.ContainsRune(" +-", rune(line.text[0])) {
				valid = false
				break
			}
			line.kind = line.text[0]
			content := line.text[1:]
			switch line.kind {
			case '-':
				line.source = old - 1
				line.mapped = matches(diff.Old, oldLines, old, content)
				old++
				oldCount--
			case '+':
				line.source = new - 1
				line.mapped = matches(diff.New, newLines, new, content)
				new++
				newCount--
			case ' ':
				line.source = new - 1
				line.mapped = matches(diff.New, newLines, new, content) && matches(diff.Old, oldLines, old, content)
				old++
				new++
				oldCount--
				newCount--
			}
			valid = valid && line.mapped && oldCount >= 0 && newCount >= 0
			parsed[i] = line
			i++
		}
		if !valid || oldCount != 0 || newCount != 0 {
			for j := start; j < i; j++ {
				parsed[j].mapped = false
			}
		}
	}
	needsOld, needsNew := false, false
	for _, line := range parsed {
		if line.mapped {
			if line.kind == '-' {
				needsOld = true
			} else {
				needsNew = true
			}
		}
	}
	var oldTokens, newTokens [][]chroma.Token
	if needsOld {
		oldTokens = tokenize(diff.Old)
	}
	if needsNew {
		newTokens = tokenize(diff.New)
	}
	result := make([]string, len(parsed))
	for i, line := range parsed {
		base := lipgloss.NewStyle()
		switch line.kind {
		case '+':
			base = s.added
		case '-':
			base = s.deleted
		case ' ':
		default:
			base = s.metadata
		}
		tokens := newTokens
		if line.kind == '-' {
			tokens = oldTokens
		}
		if !line.mapped || line.source < 0 || line.source >= len(tokens) {
			result[i] = base.Render(displayText(line.text))
			continue
		}
		var styled strings.Builder
		styled.WriteString(base.Render(string(line.kind)))
		for _, token := range tokens[line.source] {
			style := base
			colour := s.syntax.Get(token.Type).Colour
			if colour.IsSet() {
				style = style.Foreground(lipgloss.Color(colour.String()))
			}
			styled.WriteString(style.Render(strings.ReplaceAll(token.Value, "\t", "    ")))
		}
		result[i] = styled.String()
	}
	return result
}

func coordinate(start, count string) (int, int, bool) {
	line, err := strconv.Atoi(start)
	if err != nil {
		return 0, 0, false
	}
	length := 1
	if count != "" {
		length, err = strconv.Atoi(count)
	}
	return line, length, err == nil
}

func matches(snapshot git.Snapshot, lines []string, line int, text string) bool {
	return snapshot.Available && line > 0 && line <= len(lines) && lines[line-1] == text
}

// Chroma iterators may panic on lexical errors. Highlighting is optional: an
// unsupported lexer or a token stream that changes the source falls back to text.
func tokenize(snapshot git.Snapshot) (lines [][]chroma.Token) {
	defer func() {
		if recover() != nil {
			lines = nil
		}
	}()
	if !snapshot.Available || strings.ContainsRune(snapshot.Text, '\x00') {
		return nil
	}
	lexer := lexers.Match(snapshot.Path)
	if lexer == nil {
		return nil
	}
	text := VisibleText(snapshot.Text)
	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root", EnsureLF: false}, text)
	if err != nil {
		return nil
	}
	tokens := iterator.Tokens()
	var reconstructed strings.Builder
	for _, token := range tokens {
		reconstructed.WriteString(token.Value)
	}
	if reconstructed.String() != text && reconstructed.String() != text+"\n" {
		return nil
	}
	lines = chroma.SplitTokensIntoLines(tokens)
	for i := range lines {
		for j := range lines[i] {
			lines[i][j].Value = strings.TrimSuffix(lines[i][j].Value, "\n")
		}
	}
	return lines
}
