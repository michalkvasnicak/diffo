// Package diff parses unified diffs and matches source coordinates to plain snapshots.
package diff

import (
	"regexp"
	"strconv"
	"strings"
)

type Snapshot struct {
	Text      string
	Available bool
}

// Line keeps the original diff text. Source coordinates are one-based; zero
// means that the line does not consume that side of the source.
type Line struct {
	Text   string
	Kind   byte
	Old    int
	New    int
	Mapped bool
}

type Hunk struct {
	Line     int
	OldStart int
	OldCount int
	NewStart int
	NewCount int
}

type Result struct {
	Lines []Line
	Hunks []Hunk
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func sourceLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// Parse retains all headers and markers. A hunk maps only when every source
// line matches both snapshots as applicable and its declared counts are consumed.
func Parse(text string, oldSnapshot, newSnapshot Snapshot) Result {
	lines := sourceLines(text)
	result := Result{Lines: make([]Line, len(lines))}
	oldLines, newLines := sourceLines(oldSnapshot.Text), sourceLines(newSnapshot.Text)
	// A pathspec can select multiple patches after a rename. Never map these
	// to one pair of snapshots, even if individual lines happen to match.
	patchCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			patchCount++
		}
	}
	for i := 0; i < len(lines); {
		result.Lines[i] = Line{Text: lines[i]}
		match := hunkHeader.FindStringSubmatch(lines[i])
		if match == nil {
			i++
			continue
		}
		old, oldCount, oldOK := coordinate(match[1], match[2])
		new, newCount, newOK := coordinate(match[3], match[4])
		result.Hunks = append(result.Hunks, Hunk{Line: i, OldStart: old, OldCount: oldCount, NewStart: new, NewCount: newCount})
		start := i + 1
		i++
		valid := oldOK && newOK && patchCount <= 1
		for i < len(lines) && (oldCount > 0 || newCount > 0) {
			line := Line{Text: lines[i]}
			if strings.HasPrefix(line.Text, "\\") {
				result.Lines[i] = line
				i++
				continue
			}
			if line.Text == "" || !strings.ContainsRune(" +-", rune(line.Text[0])) {
				valid = false
				break
			}
			line.Kind = line.Text[0]
			content := line.Text[1:]
			switch line.Kind {
			case '-':
				line.Old = old
				line.Mapped = matches(oldSnapshot, oldLines, old, content)
				old++
				oldCount--
			case '+':
				line.New = new
				line.Mapped = matches(newSnapshot, newLines, new, content)
				new++
				newCount--
			case ' ':
				line.Old, line.New = old, new
				line.Mapped = matches(newSnapshot, newLines, new, content) && matches(oldSnapshot, oldLines, old, content)
				old++
				new++
				oldCount--
				newCount--
			}
			valid = valid && line.Mapped && oldCount >= 0 && newCount >= 0
			result.Lines[i] = line
			i++
		}
		if !valid || oldCount != 0 || newCount != 0 {
			for j := start; j < i; j++ {
				result.Lines[j].Mapped = false
			}
		}
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

func matches(snapshot Snapshot, lines []string, line int, text string) bool {
	return snapshot.Available && line > 0 && line <= len(lines) && lines[line-1] == text
}
