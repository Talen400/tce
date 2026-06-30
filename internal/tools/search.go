package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SearchTool struct{}

func (t *SearchTool) Name() string        { return "search" }
func (t *SearchTool) Description() string { return "Search the web for documentation, code references, and APIs. Use when you don't know a function signature or library." }
func (t *SearchTool) ShortDescription() string { return "Search the web" }

func (t *SearchTool) Schema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
		"required": []string{"query"},
	}
}

type searchInput struct {
	Query string `json:"query"`
}

type searchResult struct {
	Title   string
	URL     string
	Snippet string
}

func (t *SearchTool) Execute(ctx ExecContext, input json.RawMessage) (string, error) {
	var in searchInput
	if err := json.Unmarshal(input, &in); err != nil {
		var raw map[string]any
		if err2 := json.Unmarshal(input, &raw); err2 != nil {
			return "", fmt.Errorf("%s", fmtErr("invalid input", `{"query": "golang fmt.Sprintf"}`, input))
		}
		in.Query = firstOf(raw, "query", "q", "search", "find")
	}
	if in.Query == "" {
		return "", fmt.Errorf("%s", fmtErr("missing \"query\"", `{"query": "golang fmt.Sprintf"}`, input))
	}

	results, err := searchLite(in.Query)
	if err != nil || len(results) == 0 {
		results2, err2 := searchInstant(in.Query)
		if err2 != nil {
			if err != nil {
				return "", fmt.Errorf("search failed: %v (fallback: %v)", err, err2)
			}
			return "", fmt.Errorf("search failed: %w", err2)
		}
		results = results2
	}

	if len(results) == 0 {
		return fmt.Sprintf("❯ search(%s)\n── No results found ──\n── End ──", in.Query), nil
	}

	var parts []string
	for _, r := range results {
		parts = append(parts, fmt.Sprintf("- %s\n  %s\n  %s", r.Title, r.URL, r.Snippet))
	}

	return fmt.Sprintf("❯ search(%s)\n── Results ──\n%s\n── End ──", in.Query, strings.Join(parts, "\n")), nil
}

func searchLite(query string) ([]searchResult, error) {
	apiURL := fmt.Sprintf("https://lite.duckduckgo.com/lite/?q=%s", url.QueryEscape(query))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tce/0.1)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return parseLiteHTML(string(body)), nil
}

func parseLiteHTML(html string) []searchResult {
	var results []searchResult
	inResults := false
	inSnippet := false
	var snippetBuf strings.Builder
	var current searchResult

	lines := strings.Split(html, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(trimmed, "<!-- Web results are present -->") {
			inResults = true
			continue
		}

		if !inResults {
			continue
		}

		if strings.Contains(trimmed, `class='result-link'`) || strings.Contains(trimmed, `class="result-link"`) {
			title := extractBetween(trimmed, ">", "</a>")
			if title == "" {
				title = extractBetween(trimmed, ">", "</span>")
			}
			title = decodeEntities(stripTags(title))
			href := extractHref(trimmed)
			if href != "" {
				href = decodeURLParam(href, "uddg")
			}

			if current.Title != "" {
				current.Snippet = strings.TrimSpace(snippetBuf.String())
				results = append(results, current)
			}
			snippetBuf.Reset()
			inSnippet = false
			current = searchResult{
				Title: strings.TrimSpace(title),
				URL:   href,
			}
			continue
		}

		if strings.Contains(trimmed, `class='result-snippet'`) || strings.Contains(trimmed, `class="result-snippet"`) {
			inSnippet = true
			content := extractBetween(trimmed, ">", "</td>")
			if content != "" {
				snippetBuf.WriteString(decodeEntities(stripTags(content)))
				inSnippet = strings.Contains(trimmed, "</td>")
			}
			continue
		}

		if strings.Contains(trimmed, `</table>`) {
			if current.Title != "" {
				current.Snippet = strings.TrimSpace(snippetBuf.String())
				results = append(results, current)
				current = searchResult{}
			}
			snippetBuf.Reset()
			inSnippet = false
			continue
		}

		if inSnippet {
			content := strings.ReplaceAll(trimmed, "<br>", "\n")
			if end := strings.Index(content, "</td>"); end >= 0 {
				snippetBuf.WriteString(decodeEntities(stripTags(content[:end])))
				inSnippet = false
			} else {
				snippetBuf.WriteString(decodeEntities(stripTags(content)) + " ")
			}
		}
	}

	return results
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := strings.Index(s[i:], end)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}

func extractHref(line string) string {
	idx := strings.Index(line, `href="`)
	if idx < 0 {
		idx = strings.Index(line, `href='`)
		if idx < 0 {
			return ""
		}
		start := idx + 6
		end := strings.Index(line[start:], "'")
		if end < 0 {
			return ""
		}
		return line[start : start+end]
	}
	start := idx + 6
	end := strings.Index(line[start:], `"`)
	if end < 0 {
		return ""
	}
	return line[start : start+end]
}

func decodeURLParam(rawURL, param string) string {
	qIdx := strings.Index(rawURL, "?")
	if qIdx < 0 {
		return rawURL
	}
	query := rawURL[qIdx+1:]
	for _, part := range strings.Split(query, "&") {
		if strings.HasPrefix(part, param+"=") {
			val := strings.TrimPrefix(part, param+"=")
			if decoded, err := url.QueryUnescape(val); err == nil {
				return decoded
			}
			return val
		}
	}
	return rawURL
}

func decodeEntities(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&#x27;", "'")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return s
}

func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, ch := range s {
		if ch == '<' {
			inTag = true
			continue
		}
		if ch == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

type instantAnswer struct {
	AbstractText   string            `json:"AbstractText"`
	AbstractSource string            `json:"AbstractSource"`
	AbstractURL    string            `json:"AbstractURL"`
	Answer         string            `json:"Answer"`
	RelatedTopics  []json.RawMessage `json:"RelatedTopics"`
}

func searchInstant(query string) ([]searchResult, error) {
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&skip_disambig=1",
		url.QueryEscape(query))

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tce/0.1)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result instantAnswer
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	var out []searchResult

	if result.Answer != "" {
		out = append(out, searchResult{Title: "Answer", Snippet: result.Answer})
	}

	if result.AbstractText != "" {
		out = append(out, searchResult{
			Title:   result.AbstractSource,
			URL:     result.AbstractURL,
			Snippet: result.AbstractText,
		})
	}

	for _, rawMsg := range result.RelatedTopics {
		var topic struct {
			Text     string            `json:"Text"`
			FirstURL string            `json:"FirstURL"`
			Topics   []json.RawMessage `json:"Topics"`
		}
		if err := json.Unmarshal(rawMsg, &topic); err != nil {
			continue
		}
		if topic.Text != "" {
			out = append(out, searchResult{
				Title:   truncateText(topic.Text, 80),
				URL:     topic.FirstURL,
				Snippet: topic.Text,
			})
		}
		for _, subRaw := range topic.Topics {
			var sub struct {
				Text     string `json:"Text"`
				FirstURL string `json:"FirstURL"`
			}
			if json.Unmarshal(subRaw, &sub) == nil && sub.Text != "" {
				out = append(out, searchResult{
					Title:   truncateText(sub.Text, 80),
					URL:     sub.FirstURL,
					Snippet: sub.Text,
				})
			}
		}
	}

	return out, nil
}

func truncateText(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
