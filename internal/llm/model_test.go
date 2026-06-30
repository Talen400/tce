package llm

import (
	"testing"
)

func TestMatchProfileExact(t *testing.T) {
	p := MatchProfile("qwen3.5:0.8b")
	if p.MaxContext != 10000 {
		t.Errorf("expected 10000, got %d", p.MaxContext)
	}
	if !p.MinimalMode {
		t.Error("expected minimal mode for 0.8b")
	}
	if p.Temperature != 0.2 {
		t.Errorf("expected 0.2, got %f", p.Temperature)
	}
}

func TestMatchProfileCaseInsensitive(t *testing.T) {
	p := MatchProfile("QWEN3.5:0.8B")
	if p.MaxContext != 10000 {
		t.Errorf("expected 10000, got %d", p.MaxContext)
	}
	if !p.MinimalMode {
		t.Error("expected minimal mode")
	}
}

func TestMatchProfilePrefix(t *testing.T) {
	p := MatchProfile("qwen3.5:4b")
	if p.MinimalMode {
		t.Error("expected non-minimal mode for 4b family fallback")
	}
	if p.MaxContext != 20000 {
		t.Errorf("expected 20000 for qwen3.5 prefix, got %d", p.MaxContext)
	}
}

func TestMatchProfileDefault(t *testing.T) {
	p := MatchProfile("gpt-4")
	if p.MaxContext != 24000 {
		t.Errorf("expected 24000 default, got %d", p.MaxContext)
	}
	if p.Temperature != 0.2 {
		t.Errorf("expected 0.2 default, got %f", p.Temperature)
	}
}

func TestMatchProfileEmptyString(t *testing.T) {
	p := MatchProfile("")
	if p.MaxContext != 24000 {
		t.Errorf("expected default for empty, got %d", p.MaxContext)
	}
}

func TestMatchProfileTwoB(t *testing.T) {
	p := MatchProfile("qwen3.5:2b")
	if p.MaxContext != 16000 {
		t.Errorf("expected 16000 for 2b, got %d", p.MaxContext)
	}
	if !p.MinimalMode {
		t.Error("expected minimal mode for 2b")
	}
}
