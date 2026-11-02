// Package terminaltext makes untrusted text safe for terminal display.
package terminaltext

import (
	"strings"
	"unicode"
)

// Visible replaces terminal controls while retaining source line and tab boundaries.
func Visible(text string) string {
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

func ExpandTabs(text string) string {
	return strings.ReplaceAll(text, "\t", "    ")
}

func Display(text string) string {
	return ExpandTabs(Visible(text))
}

func SingleLine(text string) string {
	return strings.NewReplacer("\n", "\\n", "\t", "\\t").Replace(Visible(text))
}
