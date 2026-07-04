package tools

import (
	"strings"
	"testing"
)

func TestUnifiedDiffIdentical(t *testing.T) {
	text := "line1\nline2\nline3"
	diff := unifiedDiff(text, text)
	if diff != "" {
		t.Errorf("expected empty diff for identical text, got %q", diff)
	}
}

func TestUnifiedDiffInsertion(t *testing.T) {
	old := "line1\nline3"
	new := "line1\nline2\nline3"
	diff := unifiedDiff(old, new)
	if !strings.Contains(diff, "+line2") {
		t.Errorf("expected +line2 in diff, got:\n%s", diff)
	}
}

func TestUnifiedDiffDeletion(t *testing.T) {
	old := "line1\nline2\nline3"
	new := "line1\nline3"
	diff := unifiedDiff(old, new)
	if !strings.Contains(diff, "-line2") {
		t.Errorf("expected -line2 in diff, got:\n%s", diff)
	}
}

func TestUnifiedDiffReplacement(t *testing.T) {
	old := "hello world\nfoo bar"
	new := "hi there\nfoo bar"
	diff := unifiedDiff(old, new)
	if !strings.Contains(diff, "-hello world") {
		t.Errorf("expected -hello world in diff, got:\n%s", diff)
	}
	if !strings.Contains(diff, "+hi there") {
		t.Errorf("expected +hi there in diff, got:\n%s", diff)
	}
}

func TestUnifiedDiffEmptyOld(t *testing.T) {
	diff := unifiedDiff("", "new content")
	if !strings.Contains(diff, "+new content") {
		t.Errorf("expected +new content in diff, got:\n%s", diff)
	}
}

func TestUnifiedDiffEmptyNew(t *testing.T) {
	diff := unifiedDiff("old content", "")
	if !strings.Contains(diff, "-old content") {
		t.Errorf("expected -old content in diff, got:\n%s", diff)
	}
}

func TestUnifiedDiffMultipleHunks(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng"
	new := "a\nX\nc\nd\nY\nf\ng"
	diff := unifiedDiff(old, new)
	if !strings.Contains(diff, "-b") || !strings.Contains(diff, "+X") {
		t.Errorf("expected first hunk to show -b/+X, got:\n%s", diff)
	}
	if !strings.Contains(diff, "-e") || !strings.Contains(diff, "+Y") {
		t.Errorf("expected second hunk to show -e/+Y, got:\n%s", diff)
	}
}
