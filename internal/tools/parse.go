package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

// firstOf tries each key in order and returns the first non-empty string value.
// Multiple aliases per parameter give resilience against LLM naming variations (ADR-002).
func firstOf(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := raw[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// tryFixJSON recovers malformed JSON from LLM output — common with models
// that nest tool calls inside markdown or use single quotes (ADR-003).
func tryFixJSON(raw json.RawMessage) json.RawMessage {
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return nil
	}

	if s[0] != '{' {
		idx := strings.Index(s, "{")
		if idx >= 0 {
			s = s[idx:]
		} else {
			return nil
		}
	}

	closeIdx := strings.LastIndex(s, "}")
	if closeIdx < 0 {
		return nil
	}
	s = s[:closeIdx+1]

	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}

	s = fixJSONLines(s)
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}

	return nil
}

func fixJSONLines(s string) string {
	var b strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		b.WriteByte(ch)
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if !inString && ch == '\n' {
			b.WriteByte('\\')
			b.WriteByte('n')
		}
	}
	return b.String()
}

func fmtErr(reason, example string, received json.RawMessage) string {
	var receivedStr string
	if received != nil {
		receivedStr = string(received)
	}
	if receivedStr == "" || receivedStr == "null" {
		return fmt.Sprintf("Error: %s\nExpected: %s", reason, example)
	}
	return fmt.Sprintf("Error: %s\nExpected: %s\nYou sent: %s", reason, example, receivedStr)
}

func fuzzyMatch(name string, available []string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}

	for _, a := range available {
		if strings.ToLower(a) == name {
			return a
		}
	}

	var prefixMatch string
	for _, a := range available {
		lower := strings.ToLower(a)
		if strings.HasPrefix(lower, name) {
			if prefixMatch != "" {
				return ""
			}
			prefixMatch = a
		}
	}
	if prefixMatch != "" {
		return prefixMatch
	}

	for _, a := range available {
		lower := strings.ToLower(a)
		if strings.HasPrefix(name, lower) {
			return a
		}
	}

	return ""
}
