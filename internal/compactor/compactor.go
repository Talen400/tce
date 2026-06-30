package compactor

import (
	"github.com/talen/tce/internal/llm"
)

type Config struct {
	MaxContextTokens  int
	KeepTurns         int
	MaxToolContentLen int
	TokenRatio        float64
}

type Compactor struct {
	MaxContextTokens  int
	KeepTurns         int
	MaxToolContentLen int
	tokenRatio        float64
}

func New(maxTokens int) *Compactor {
	return NewWithConfig(Config{
		MaxContextTokens:  maxTokens,
		KeepTurns:         2,
		MaxToolContentLen: 1000,
		TokenRatio:        3.5,
	})
}

func NewWithConfig(cfg Config) *Compactor {
	if cfg.MaxContextTokens <= 0 {
		cfg.MaxContextTokens = 24000
	}
	if cfg.KeepTurns <= 0 {
		cfg.KeepTurns = 2
	}
	if cfg.MaxToolContentLen <= 0 {
		cfg.MaxToolContentLen = 1000
	}
	if cfg.TokenRatio <= 0 {
		cfg.TokenRatio = 3.5
	}
	return &Compactor{
		MaxContextTokens:  cfg.MaxContextTokens,
		KeepTurns:         cfg.KeepTurns,
		MaxToolContentLen: cfg.MaxToolContentLen,
		tokenRatio:        cfg.TokenRatio,
	}
}

func (c *Compactor) Compact(msgs []llm.Message) []llm.Message {
	if len(msgs) == 0 {
		return msgs
	}

	tokens := c.estimateMessages(msgs)
	if tokens <= c.MaxContextTokens {
		return msgs
	}

	return c.prune(msgs)
}

func (c *Compactor) prune(msgs []llm.Message) []llm.Message {
	systemIdx := 0
	hasSystem := len(msgs) > 0 && msgs[0].Role == "system"
	if hasSystem {
		systemIdx = 1
	}

	if len(msgs) <= systemIdx+c.KeepTurns*3 {
		return c.truncateToolResults(msgs)
	}

	work := make([]llm.Message, len(msgs))
	copy(work, msgs)

	removed := 0
	i := systemIdx
	maxRemove := len(work) - systemIdx - c.KeepTurns*3

	for i < len(work) && removed < maxRemove {
		role := work[i].Role

		if role == "user" {
			if removed+3 <= maxRemove && i+2 < len(work) {
				copy(work[i:], work[i+3:])
				work = work[:len(work)-3]
				removed += 3
				continue
			}
			if i+1 < len(work) && work[i+1].Role == "assistant" {
				hasTool := i+2 < len(work) && work[i+2].Role == "tool"
				if hasTool && removed+3 <= maxRemove {
					copy(work[i:], work[i+3:])
					work = work[:len(work)-3]
					removed += 3
					continue
				}
				if removed+2 <= maxRemove {
					copy(work[i:], work[i+2:])
					work = work[:len(work)-2]
					removed += 2
					continue
				}
			}
		}
		i++
	}

	cnt := c.estimateMessages(work)
	if cnt > c.MaxContextTokens {
		work = c.truncateToolResults(work)
	}

	return work
}

func (c *Compactor) truncateToolResults(msgs []llm.Message) []llm.Message {
	var total int
	for _, m := range msgs {
		total += c.estimateTokens(m.Content)
	}
	if total <= c.MaxContextTokens {
		return msgs
	}

	result := make([]llm.Message, len(msgs))
	copy(result, msgs)

	for i := len(result) - 1; i >= 0; i-- {
		if total <= c.MaxContextTokens {
			break
		}
		if result[i].Role == "tool" && len(result[i].Content) > c.MaxToolContentLen {
			oldLen := len(result[i].Content)
			result[i].Content = result[i].Content[:c.MaxToolContentLen] + "\n... (truncated)"
			total -= (oldLen - len(result[i].Content))
		}
	}

	return result
}

func (c *Compactor) estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	ascii := 0
	nonASCII := 0
	for _, r := range text {
		if r < 128 {
			ascii++
		} else {
			nonASCII++
		}
	}

	const maxTokens = 128 * 1024

	asciiTokens := int(float64(ascii) / c.tokenRatio)
	if asciiTokens > maxTokens {
		return maxTokens
	}
	nonASCIITokens := nonASCII / 2
	if nonASCIITokens > maxTokens {
		return maxTokens
	}
	total := asciiTokens + nonASCIITokens
	if total < 1 {
		total = 1
	}
	if total > maxTokens {
		return maxTokens
	}
	return total
}

func (c *Compactor) estimateMessages(msgs []llm.Message) int {
	total := 0
	for _, m := range msgs {
		total += c.estimateTokens(m.Content)
		total += 12
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				total += c.estimateTokens(tc.Name)
				total += c.estimateTokens(tc.Arguments)
				total += 4
			}
		}
	}
	return total
}


