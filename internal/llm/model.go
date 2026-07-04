package llm

import "strings"

type Profile struct {
	MaxContext      int
	MaxTurns        int
	Temperature     float32
	MinimalMode     bool
	MaxToolContent  int
	KeepTurns       int
	TokenRatio      float64
	ForceSingleCall bool
	ResponseFormat  string
}

var profiles = map[string]Profile{
	"qwen3.5:0.8b": {
		MaxContext: 10000, MaxTurns: 15, Temperature: 0.0,
		MinimalMode: true, MaxToolContent: 500, KeepTurns: 1,
		TokenRatio: 3.5, ForceSingleCall: true,
	},
	"qwen3.5:2b": {
		MaxContext: 16000, MaxTurns: 20, Temperature: 0.0,
		MinimalMode: true, MaxToolContent: 800, KeepTurns: 2,
		TokenRatio: 3.5, ForceSingleCall: false,
	},
}

var prefixes = []struct {
	prefix  string
	profile Profile
}{
	{"qwen3.5:0.8b", Profile{
		MaxContext: 10000, MaxTurns: 15, Temperature: 0.0,
		MinimalMode: true, MaxToolContent: 500, KeepTurns: 1,
		TokenRatio: 3.5, ForceSingleCall: true,
	}},
	{"qwen3.5:2b", Profile{
		MaxContext: 16000, MaxTurns: 20, Temperature: 0.0,
		MinimalMode: true, MaxToolContent: 800, KeepTurns: 2,
		TokenRatio: 3.5, ForceSingleCall: false,
	}},
	{"qwen3.5", Profile{
		MaxContext: 20000, MaxTurns: 25, Temperature: 0.0,
		MinimalMode: false, MaxToolContent: 1000, KeepTurns: 2,
		TokenRatio: 3.5, ForceSingleCall: false,
	}},
}

var defaultProfile = Profile{
	MaxContext: 24000, MaxTurns: 25, Temperature: 0.0,
	MinimalMode: false, MaxToolContent: 1000, KeepTurns: 2,
	TokenRatio: 4.0, ForceSingleCall: false,
}

func MatchProfile(modelName string) Profile {
	normalized := strings.ToLower(modelName)

	if p, ok := profiles[normalized]; ok {
		return p
	}

	for _, entry := range prefixes {
		if strings.HasPrefix(normalized, entry.prefix) {
			return entry.profile
		}
	}

	return defaultProfile
}
