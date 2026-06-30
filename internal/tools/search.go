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

type ddgResult struct {
	AbstractText   string `json:"AbstractText"`
	AbstractSource string `json:"AbstractSource"`
	AbstractURL    string `json:"AbstractURL"`
	Answer         string `json:"Answer"`
	Image          string `json:"Image"`
	RelatedTopics  []json.RawMessage `json:"RelatedTopics"`
	Results        []json.RawMessage `json:"Results"`
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

	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&skip_disambig=1",
		url.QueryEscape(in.Query))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var result ddgResult
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	var parts []string

	if result.Answer != "" {
		parts = append(parts, result.Answer)
	}

	if result.AbstractText != "" {
		parts = append(parts, fmt.Sprintf("%s\nSource: %s", result.AbstractText, result.AbstractURL))
	}

	if len(result.RelatedTopics) > 0 {
		limit := 5
		for i, rawMsg := range result.RelatedTopics {
			if i >= limit {
				break
			}
			var topic struct {
				Text     string `json:"Text"`
				FirstURL string `json:"FirstURL"`
				Topics   []json.RawMessage `json:"Topics"`
			}
			if err := json.Unmarshal(rawMsg, &topic); err != nil {
				continue
			}
			if topic.Text != "" {
				parts = append(parts, fmt.Sprintf("- %s\n  %s", topic.Text, topic.FirstURL))
			}
			if len(topic.Topics) > 0 {
				for j, subRaw := range topic.Topics {
					if j >= 3 {
						break
					}
					var sub struct {
						Text     string `json:"Text"`
						FirstURL string `json:"FirstURL"`
					}
					if json.Unmarshal(subRaw, &sub) == nil && sub.Text != "" {
						parts = append(parts, fmt.Sprintf("  - %s\n    %s", sub.Text, sub.FirstURL))
					}
				}
			}
		}
	}

	if len(parts) == 0 {
		return fmt.Sprintf("❯ search(%s)\n── No results found ──\n── End ──", in.Query), nil
	}

	return fmt.Sprintf("❯ search(%s)\n── Results ──\n%s\n── End ──", in.Query, strings.Join(parts, "\n")), nil
}
