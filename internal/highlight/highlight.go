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

// Span applies presentation within terminal-safe source byte offsets, before
// tabs expand. Callers supply sorted, non-overlapping spans.
type Span struct {
	Start int
	End   int
	Style lipgloss.Style
}

// Plain retains safe text when a source has no supported syntax lexer.
func Plain(text string) Line {
	return Line{tokens: []chroma.Token{{Type: chroma.Text, Value: terminaltext.Visible(text)}}}
}

// Render intersects presentation spans with cached tokens, preserving syntax
// foregrounds even when a span changes in the middle of a token.
func (s *Service) Render(line Line, base lipgloss.Style, spans ...Span) string {
	var styled strings.Builder
	offset, span := 0, 0
	for _, token := range line.tokens {
		end := offset + len(token.Value)
		for position := 0; offset < end; {
			for span < len(spans) && spans[span].End <= offset {
				span++
			}
			style, next := base, end
			if span < len(spans) {
				if spans[span].Start <= offset {
					style = spans[span].Style.Inherit(base)
					next = min(next, spans[span].End)
				} else {
					next = min(next, spans[span].Start)
				}
			}
			colour := s.syntax.Get(token.Type).Colour
			if colour.IsSet() {
				style = style.Foreground(lipgloss.Color(colour.String()))
			}
			length := next - offset
			styled.WriteString(style.Render(terminaltext.ExpandTabs(token.Value[position : position+length])))
			position += length
			offset = next
		}
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
