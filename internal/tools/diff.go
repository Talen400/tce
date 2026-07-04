package tools

import (
	"fmt"
	"strings"
)

type diffEdit struct {
	line int
	del  string
	add  string
}

func unifiedDiff(oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	var edits []diffEdit

	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	for i := 0; i < maxLen; i++ {
		oldLine := ""
		newLine := ""
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			edits = append(edits, diffEdit{line: i, del: oldLine, add: newLine})
		}
	}

	if len(edits) == 0 {
		return ""
	}

	var b strings.Builder
	context := 2

	groupStart := 0
	for i := 1; i <= len(edits); i++ {
		if i < len(edits) && edits[i].line-edits[i-1].line <= context*2 {
			continue
		}

		first := edits[groupStart]
		last := edits[i-1]

		hunkOldStart := first.line - context + 1
		if hunkOldStart < 1 {
			hunkOldStart = 1
		}

		contextStart := first.line - context
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := last.line + context + 1
		if contextEnd > maxLen {
			contextEnd = maxLen
		}

		if b.Len() > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "@@ -%d +%d @@\n", hunkOldStart, hunkOldStart)

		for j := contextStart; j < contextEnd; j++ {
			oldLine := ""
			newLine := ""
			if j < len(oldLines) {
				oldLine = oldLines[j]
			}
			if j < len(newLines) {
				newLine = newLines[j]
			}

			if isEdit(edits, j) {
				if oldLine != "" {
					b.WriteString("-" + oldLine + "\n")
				}
				if newLine != "" {
					b.WriteString("+" + newLine + "\n")
				}
			} else {
				line := oldLine
				if line == "" {
					line = newLine
				}
				b.WriteString(" " + line + "\n")
			}
		}

		groupStart = i
	}

	return strings.TrimRight(b.String(), "\n")
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func isEdit(edits []diffEdit, line int) bool {
	for _, e := range edits {
		if e.line == line {
			return true
		}
	}
	return false
}
