package diff

import (
	"strings"
	"unicode/utf8"

	"diffo/internal/terminaltext"

	"github.com/rivo/uniseg"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Range uses byte offsets in terminal-safe, unexpanded text. Boundaries always
// fall between graphemes, before syntax tokens or tab expansion are applied.
type Range struct {
	Start int
	End   int
}

type AnnotatedLine struct {
	Text      string
	Old       int
	New       int
	Added     []Range // Inserted graphemes, including within an existing line.
	Changed   []Range // Inserted graphemes replacing deleted text.
	Deleted   bool
	Edited    bool // Added, deleted, or replaced line, excluding equal anchors.
	Hunk      int  // One-based hunk ID; zero outside a hunk.
	HunkStart bool // First displayed edit line of a mapped hunk.
}

// Annotate includes the complete new snapshot and deletion-only blocks from the
// old snapshot. All hunk mappings AND the gaps between them must match; custom
// or incomplete patches must not produce plausible but incorrect annotations.
func Annotate(parsed Result, oldSnapshot, newSnapshot Snapshot) ([]AnnotatedLine, bool) {
	if !oldSnapshot.Available || !newSnapshot.Available {
		return nil, false
	}
	oldLines, newLines := sourceLines(oldSnapshot.Text), sourceLines(newSnapshot.Text)
	var result []AnnotatedLine
	old, new := 0, 0
	appendGap := func(oldEnd, newEnd int) bool {
		if oldEnd < old || newEnd < new || oldEnd > len(oldLines) || newEnd > len(newLines) || oldEnd-old != newEnd-new {
			return false
		}
		for old < oldEnd {
			if oldLines[old] != newLines[new] {
				return false
			}
			result = append(result, annotatedLine(newLines[new], 0, new+1))
			old++
			new++
		}
		return true
	}
	for hunkIndex, hunk := range parsed.Hunks {
		oldStart, newStart := hunk.OldStart, hunk.NewStart
		if hunk.OldCount > 0 {
			oldStart--
		}
		if hunk.NewCount > 0 {
			newStart--
		}
		if !appendGap(oldStart, newStart) {
			return nil, false
		}
		oldEnd, newEnd := old+hunk.OldCount, new+hunk.NewCount
		var removed, added []Line
		markedHunk := false
		flush := func() {
			block := annotateBlock(removed, added)
			for i := range block {
				block[i].Hunk = hunkIndex + 1
			}
			if len(block) > 0 && !markedHunk {
				block[0].HunkStart = true
				markedHunk = true
			}
			result = append(result, block...)
			removed, added = nil, nil
		}
		for i := hunk.Line + 1; old < oldEnd || new < newEnd; i++ {
			if i >= len(parsed.Lines) {
				return nil, false
			}
			line := parsed.Lines[i]
			if strings.HasPrefix(line.Text, "\\") {
				continue
			}
			if !line.Mapped || (line.Old != 0 && line.Old != old+1) || (line.New != 0 && line.New != new+1) {
				return nil, false
			}
			switch line.Kind {
			case '-':
				removed = append(removed, line)
				old++
			case '+':
				added = append(added, line)
				new++
			case ' ':
				flush()
				context := annotatedLine(newLines[new], 0, new+1)
				context.Hunk = hunkIndex + 1
				result = append(result, context)
				old++
				new++
			default:
				return nil, false
			}
			if old > oldEnd || new > newEnd {
				return nil, false
			}
		}
		flush()
	}
	if !appendGap(len(oldLines), len(newLines)) {
		return nil, false
	}
	return result, true
}

func annotatedLine(text string, old, new int) AnnotatedLine {
	return AnnotatedLine{Text: terminaltext.Visible(text), Old: old, New: new}
}

// A contiguous edit block containing both sides is one replacement: none of
// its old text survives. Exact equal lines anchor alignment; between anchors,
// pair lines in order. Extra new lines are additions, extra old lines disappear.
func annotateBlock(removed, added []Line) []AnnotatedLine {
	if len(added) == 0 {
		result := make([]AnnotatedLine, len(removed))
		for i, line := range removed {
			result[i] = annotatedLine(line.Text[1:], line.Old, 0)
			result[i].Deleted, result[i].Edited = true, true
		}
		return result
	}
	result := make([]AnnotatedLine, len(added))
	oldText, newText := make([]string, len(removed)), make([]string, len(added))
	for i, line := range removed {
		oldText[i] = terminaltext.Visible(line.Text[1:])
	}
	for i, line := range added {
		result[i] = annotatedLine(line.Text[1:], 0, line.New)
		result[i].Edited = true
		newText[i] = result[i].Text
		if len(removed) == 0 && len(newText[i]) > 0 {
			result[i].Added = []Range{{Start: 0, End: len(newText[i])}}
		}
	}
	if len(removed) == 0 {
		return result
	}
	old, new := 0, 0
	oldStart, newStart := 0, 0
	flush := func() {
		for i := newStart; i < new; i++ {
			paired := oldStart + i - newStart
			if paired < old {
				result[i].Added, result[i].Changed = changedGraphemes(oldText[paired], newText[i])
			} else if len(newText[i]) > 0 {
				result[i].Added = []Range{{Start: 0, End: len(newText[i])}}
			}
		}
	}
	for _, edit := range sequenceDiff(oldText, newText) {
		length := utf8.RuneCountInString(edit.Text)
		switch edit.Type {
		case diffmatchpatch.DiffDelete:
			old += length
		case diffmatchpatch.DiffInsert:
			new += length
		case diffmatchpatch.DiffEqual:
			flush()
			for i := new; i < new+length; i++ {
				result[i].Edited = false
			}
			old += length
			new += length
			oldStart, newStart = old, new
		}
	}
	flush()
	return result
}

// Myers alignment runs on line IDs for edit blocks and grapheme IDs only for
// paired lines, never on all the characters of a complete file. No semantic
// cleanup is used: it would turn small unchanged token portions into additions.
func sequenceDiff(old, new []string) []diffmatchpatch.Diff {
	join := func(items []string) string {
		if len(items) == 0 {
			return ""
		}
		return strings.Join(items, "\n") + "\n"
	}
	engine := diffmatchpatch.New()
	engine.DiffTimeout = 0
	oldIDs, newIDs, _ := engine.DiffLinesToRunes(join(old), join(new))
	return engine.DiffMainRunes(oldIDs, newIDs, false)
}

// Each uninterrupted edit group is a replacement only when it both removes
// and inserts text. Insertions separated from removals by equal text stay added.
func changedGraphemes(old, new string) (added, changed []Range) {
	split := func(text string) []string {
		var parts []string
		graphemes := uniseg.NewGraphemes(text)
		for graphemes.Next() {
			parts = append(parts, graphemes.Str())
		}
		return parts
	}
	oldParts, newParts := split(old), split(new)
	part, offset, start := 0, 0, 0
	removed := false
	flush := func() {
		if offset > start {
			span := Range{Start: start, End: offset}
			if removed {
				changed = append(changed, span)
			} else {
				added = append(added, span)
			}
		}
		start, removed = offset, false
	}
	for _, edit := range sequenceDiff(oldParts, newParts) {
		if edit.Type == diffmatchpatch.DiffDelete {
			removed = true
			continue
		}
		if edit.Type == diffmatchpatch.DiffEqual {
			flush()
		}
		for range utf8.RuneCountInString(edit.Text) {
			offset += len(newParts[part])
			part++
		}
		if edit.Type == diffmatchpatch.DiffEqual {
			start = offset
		}
	}
	flush()
	return added, changed
}
