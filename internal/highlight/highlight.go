// Package highlight tokenizes complete sources and applies syntax foregrounds.
package highlight

import (
	"strings"

	"diffo/internal/terminaltext"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

type Service struct {
	syntax *chroma.Style
}

func New() *Service {
	theme := "github"
	if lipgloss.HasDarkBackground() {
		theme = "github-dark"
	}
	return &Service{syntax: styles.Get(theme)}
}

// Line contains safe source tokens without exposing Chroma to consumers.
type Line struct {
	tokens []chroma.Token
}

// Render adds syntax foregrounds to a caller's presentation style.
func (s *Service) Render(line Line, base lipgloss.Style) string {
	var styled strings.Builder
	for _, token := range line.tokens {
		style := base
		colour := s.syntax.Get(token.Type).Colour
		if colour.IsSet() {
			style = style.Foreground(lipgloss.Color(colour.String()))
		}
		styled.WriteString(style.Render(terminaltext.ExpandTabs(token.Value)))
	}
	return styled.String()
}

// Source tokenizes a complete source once, retaining multiline lexical context.
// Chroma iterators may panic on lexical errors. Unsupported lexers and streams
// that change the source return no lines so callers can display plain text.
func (s *Service) Source(path, source string) (lines []Line) {
	defer func() {
		if recover() != nil {
			lines = nil
		}
	}()
	if strings.ContainsRune(source, '\x00') {
		return nil
	}
	lexer := lexers.Match(path)
	if lexer == nil {
		return nil
	}
	text := terminaltext.Visible(source)
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
	split := chroma.SplitTokensIntoLines(tokens)
	lines = make([]Line, len(split))
	for i, tokens := range split {
		for j := range tokens {
			tokens[j].Value = strings.TrimSuffix(tokens[j].Value, "\n")
		}
		lines[i] = Line{tokens: tokens}
	}
	return lines
}
