package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSearchToolName(t *testing.T) {
	tool := &SearchTool{}
	if tool.Name() != "search" {
		t.Errorf("expected search, got %s", tool.Name())
	}
}

func TestSearchToolShortDescription(t *testing.T) {
	tool := &SearchTool{}
	if tool.ShortDescription() == "" {
		t.Error("short description should not be empty")
	}
}

func TestSearchToolSchema(t *testing.T) {
	tool := &SearchTool{}
	schema := tool.Schema()
	m, ok := schema.(map[string]any)
	if !ok {
		t.Fatal("schema should be a map")
	}
	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}
	if _, ok := props["query"]; !ok {
		t.Error("schema should have query property")
	}
}

func TestSearchToolExecuteEmptyQuery(t *testing.T) {
	tool := &SearchTool{}
	input, _ := json.Marshal(map[string]string{"query": ""})
	_, err := tool.Execute(ExecContext{}, input)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestSearchToolExecuteMissingQuery(t *testing.T) {
	tool := &SearchTool{}
	input, _ := json.Marshal(map[string]string{"wrong": "field"})
	_, err := tool.Execute(ExecContext{}, input)
	if err == nil {
		t.Error("expected error for missing query")
	}
}

func TestSearchToolExecuteInvalidJSON(t *testing.T) {
	tool := &SearchTool{}
	_, err := tool.Execute(ExecContext{}, json.RawMessage(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseLiteHTMLEmpty(t *testing.T) {
	results := parseLiteHTML("")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseLiteHTMLNoResults(t *testing.T) {
	html := `<html><body>No results here</body></html>`
	results := parseLiteHTML(html)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestParseLiteHTMLWithResults(t *testing.T) {
	html := `<html><body>
<!-- Web results are present -->
<table border="0">
<tr><td><a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgithub.com%2Ftest" class='result-link'>Test Project</a></td></tr>
<tr><td class='result-snippet'>A test <b>C</b> project</td></tr>
</table>
</body></html>`
	results := parseLiteHTML(html)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Test Project" {
		t.Errorf("expected 'Test Project', got %s", results[0].Title)
	}
	if results[0].URL != "https://github.com/test" {
		t.Errorf("expected https://github.com/test, got %s", results[0].URL)
	}
	if !strings.Contains(results[0].Snippet, "A test C project") {
		t.Errorf("expected snippet with 'A test C project', got %s", results[0].Snippet)
	}
}

func TestParseLiteHTMLMultipleResults(t *testing.T) {
	html := `<html><body>
<!-- Web results are present -->
<table border="0">
<tr><td><a rel="nofollow" href="//ddg.com/l/?uddg=https%3A%2F%2Fa.com" class='result-link'>Result A</a></td></tr>
<tr><td class='result-snippet'>Snippet A</td></tr>
</table>
<table border="0">
<tr><td><a rel="nofollow" href="//ddg.com/l/?uddg=https%3A%2F%2Fb.com" class='result-link'>Result B</a></td></tr>
<tr><td class='result-snippet'>Snippet B</td></tr>
</table>
</body></html>`
	results := parseLiteHTML(html)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].URL != "https://a.com" {
		t.Errorf("expected https://a.com, got %s", results[0].URL)
	}
	if results[1].URL != "https://b.com" {
		t.Errorf("expected https://b.com, got %s", results[1].URL)
	}
}

func TestStripTags(t *testing.T) {
	tests := []struct{ in, want string }{
		{"<b>C</b> programming", "C programming"},
		{"<span class='x'>text</span>", "text"},
		{"no tags", "no tags"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripTags(tt.in)
		if got != tt.want {
			t.Errorf("stripTags(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDecodeEntities(t *testing.T) {
	tests := []struct{ in, want string }{
		{"C &amp; printf", "C & printf"},
		{"&lt;stdio.h&gt;", "<stdio.h>"},
		{"&quot;hello&quot;", `"hello"`},
		{"&nbsp;text", " text"},
	}
	for _, tt := range tests {
		got := decodeEntities(tt.in)
		if got != tt.want {
			t.Errorf("decodeEntities(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDecodeURLParam(t *testing.T) {
	result := decodeURLParam("//ddg.com/l/?uddg=https%3A%2F%2Fgithub.com%2Ftest&rut=abc", "uddg")
	if result != "https://github.com/test" {
		t.Errorf("expected https://github.com/test, got %s", result)
	}
}

func TestDecodeURLParamNoParam(t *testing.T) {
	result := decodeURLParam("https://example.com", "uddg")
	if result != "https://example.com" {
		t.Errorf("expected unchanged URL, got %s", result)
	}
}

func BenchmarkParseLiteHTML(b *testing.B) {
	html := `<html><body>
<!-- Web results are present -->
<table border="0">
<tr><td><a rel="nofollow" href="//ddg.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fa" class='result-link'>Result A</a></td></tr>
<tr><td class='result-snippet'>Snippet A with <b>bold</b> text</td></tr>
</table>
<table border="0">
<tr><td><a rel="nofollow" href="//ddg.com/l/?uddg=https%3A%2F%2Fgithub.com%2Fb" class='result-link'>Result B</a></td></tr>
<tr><td class='result-snippet'>Snippet B with <em>emphasis</em></td></tr>
</table>
</body></html>`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseLiteHTML(html)
	}
}

func BenchmarkStripTags(b *testing.B) {
	text := strings.Repeat("This is <b>bold</b> and <em>emphasized</em> text with <a href='x'>links</a>. ", 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stripTags(text)
	}
}
