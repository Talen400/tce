package tools

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type ignorePattern struct {
	negate   bool
	dirOnly  bool
	anchored bool
	segments []string
}

// IgnoreMatcher matches file paths against gitignore-style patterns.
// Load with LoadIgnoreFile or NewIgnoreMatcher.
type IgnoreMatcher struct {
	patterns []ignorePattern
}

// LoadIgnoreFile reads a gitignore-style file and returns a matcher for its patterns.
func LoadIgnoreFile(path string) (*IgnoreMatcher, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return NewIgnoreMatcher(lines), nil
}

// NewIgnoreMatcher creates a matcher from a list of gitignore-style patterns.
func NewIgnoreMatcher(patterns []string) *IgnoreMatcher {
	m := &IgnoreMatcher{}
	for _, line := range patterns {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		p := ignorePattern{}

		if strings.HasPrefix(line, "!") {
			p.negate = true
			line = line[1:]
		}

		if strings.HasSuffix(line, "/") {
			p.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}

		if strings.Contains(line, "/") {
			p.anchored = true
		}

		p.segments = strings.Split(filepath.ToSlash(line), "/")

		if len(p.segments) > 0 && p.segments[0] == "" {
			p.segments = p.segments[1:]
			p.anchored = true
		}

		m.patterns = append(m.patterns, p)
	}
	return m
}

// Match returns true if the relative path should be ignored.
// relPath is a relative path from the ignore file root with forward slashes.
// isDir indicates whether the path refers to a directory.
func (m *IgnoreMatcher) Match(relPath string, isDir bool) bool {
	if m == nil {
		return false
	}
	if relPath == "" {
		return false
	}
	relPath = filepath.ToSlash(relPath)

	matched := false
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		if matchPattern(p, relPath) {
			matched = !p.negate
		}
	}
	return matched
}

func matchPattern(p ignorePattern, relPath string) bool {
	pathSegs := strings.Split(relPath, "/")

	if p.anchored {
		return matchSegments(p.segments, pathSegs)
	}

	for i := 0; i <= len(pathSegs); i++ {
		if matchSegments(p.segments, pathSegs[i:]) {
			return true
		}
	}
	return false
}

func matchSegments(pattern, path []string) bool {
	if len(pattern) == 0 {
		return false
	}

	pi, pp := 0, 0
	for pi < len(pattern) && pp < len(path) {
		if pattern[pi] == "**" {
			if pi == len(pattern)-1 {
				return true
			}
			for skip := 0; pp+skip <= len(path); skip++ {
				if matchSegments(pattern[pi+1:], path[pp+skip:]) {
					return true
				}
			}
			return false
		}

		if !matchSegment(pattern[pi], path[pp]) {
			return false
		}

		pi++
		pp++
	}

	return pi == len(pattern) && pp == len(path)
}

func matchSegment(pattern, text string) bool {
	if pattern == text {
		return true
	}
	matched, err := filepath.Match(pattern, text)
	return err == nil && matched
}
