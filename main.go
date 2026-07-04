package main

import (
	"bufio"
	"errors"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/talen/tce/internal/agent"
	"github.com/talen/tce/internal/config"
	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/project"
	"github.com/talen/tce/internal/session"
	"github.com/talen/tce/internal/tools"
	"github.com/talen/tce/internal/tui"
)

var version = "0.1.0"

func main() {
	model := flag.String("model", "", "LLM model name")
	baseURL := flag.String("base-url", "", "LLM API base URL")
	apiKey := flag.String("api-key", "", "LLM API key")
	agentType := flag.String("agent", "build", "Agent type: build, plan, explore")
	projectRoot := flag.String("dir", ".", "Project root directory")
	minimal := flag.Bool("minimal", false, "Use minimal prompt (better for small models <4B)")
	cliMode := flag.Bool("cli", false, "Use CLI mode instead of TUI")
	contextSize := flag.Int("context-size", 0, "Max context tokens before compaction (0=auto)")
	showVersion := flag.Bool("version", false, "Show version")
	verbose := flag.Bool("verbose", false, "Show detailed tool call payloads")
	resume := flag.String("resume", "", "Resume a previous session from .tce/sessions/ file")
	flag.Parse()

	if *showVersion {
		fmt.Printf("tce v%s\n", version)
		return
	}

	root := *projectRoot
	if root == "." {
		var err error
		root, err = os.Getwd()
		if err != nil {
			root = "."
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}

	profile := project.Detect(absRoot)
	if profile.Root == "" {
		profile.Root = absRoot
	}

	projectCfg := config.Load(absRoot)
	if *model == "" && projectCfg.Model != "" {
		*model = projectCfg.Model
	}
	if *agentType == "build" && projectCfg.Agent != "" {
		*agentType = projectCfg.Agent
	}

	llmCfg := llm.DefaultConfig
	if *model != "" {
		llmCfg.Model = *model
	}
	if *baseURL != "" {
		llmCfg.BaseURL = *baseURL
	}
	if *apiKey != "" {
		llmCfg.APIKey = *apiKey
	}
	if envURL := os.Getenv("TCE_API_URL"); envURL != "" {
		llmCfg.BaseURL = envURL
	}
	if envKey := os.Getenv("TCE_API_KEY"); envKey != "" {
		llmCfg.APIKey = envKey
	}
	if envModel := os.Getenv("TCE_MODEL"); envModel != "" {
		llmCfg.Model = envModel
	}

	modelProfile := llm.MatchProfile(llmCfg.Model)
	llmCfg.ResponseFormat = modelProfile.ResponseFormat

	agent.TCEVersion = version

	llmClient := llm.NewClient(llmCfg)
	toolReg := tools.NewRegistry()

	toolReg.Register(&tools.ReadTool{})
	toolReg.Register(&tools.WriteTool{})
	toolReg.Register(&tools.EditTool{})
	toolReg.Register(&tools.GrepTool{})
	toolReg.Register(&tools.GlobTool{})
	toolReg.Register(&tools.BashTool{})
	toolReg.Register(&tools.AskTool{})
	toolReg.Register(&tools.TaskTool{})
	toolReg.Register(&tools.SearchTool{})
	toolReg.Register(&tools.UndoTool{})

	at := agent.AgentType(*agentType)
	if at != agent.AgentBuild && at != agent.AgentPlan && at != agent.AgentExplore {
		fmt.Fprintf(os.Stderr, "Invalid agent type: %s (use: build, plan, explore)\n", *agentType)
		os.Exit(1)
	}

	maxCtx := *contextSize
	if maxCtx <= 0 {
		maxCtx = modelProfile.MaxContext
	}

	showMinimal := *minimal || modelProfile.MinimalMode
	disableStream := strings.ToLower(os.Getenv("TCE_STREAM")) == "false"

	agentCfg := agent.Config{
		Type:            at,
		LLM:             llmClient,
		Tools:           toolReg,
		Project:         profile,
		MaxTurns:        modelProfile.MaxTurns,
		MinimalMode:     showMinimal,
		MaxContext:      maxCtx,
		ForceSingleCall: modelProfile.ForceSingleCall,
		KeepTurns:       modelProfile.KeepTurns,
		MaxToolContent:  modelProfile.MaxToolContent,
		TokenRatio:      modelProfile.TokenRatio,
		DisableStream:   disableStream,
		Verbose:         *verbose || projectCfg.Verbose(),
	}

	ag := agent.New(agentCfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()

	if *resume != "" {
		modelName, prevTurns, prevTokIn, prevTokOut, msgs, err := session.Load(*resume)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading session %s: %v\n", *resume, err)
			os.Exit(1)
		}
		if len(msgs) > 0 {
			ag.SetMessages(msgs)
		}
		fmt.Printf("📂 Resumed session from %s (model: %s, %d turns, ~%d tokens)\n", *resume, modelName, prevTurns, prevTokIn+prevTokOut)
	}

	if *cliMode {
		runCLI(ctx, ag, profile, at, llmClient.ModelName())
		turns, tokIn, tokOut := ag.Stats()
		if turns > 1 {
			fmt.Printf("\n📊 Session: %d turns, ~%d tokens in, ~%d tokens out\n", turns, tokIn, tokOut)
			session.Save(profile.Root, llmClient.ModelName(), turns, tokIn, tokOut, ag.GetMessages())
		}
		return
	}

	app := tui.NewModel(profile, agentCfg)
	if _, err := tea.NewProgram(&app).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCLI(ctx context.Context, ag *agent.Agent, profile *project.Profile, agentType agent.AgentType, modelName string) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("tce v%s\n", version)
	fmt.Printf("Project: %s  Agent: %s  Model: %s\n\n", profile.Summary(), agentType, modelName)
	fmt.Println("Commands: /help, /exit, /clear, /project | Type your prompt:")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch {
		case input == "/exit":
			return
		case input == "/clear":
			ag.Reset()
			fmt.Print("\033[H\033[2J")
			continue
		case input == "/help":
			fmt.Println("Commands: /exit, /clear, /project")
			continue
		case input == "/project":
			fmt.Println(profile.String())
			continue
		}

		start := time.Now()

		turnCtx, turnCancel := context.WithCancel(ctx)

		var lastTool string
		result, err := ag.Run(turnCtx, input, nil,
			func(name, args string) {
				lastTool = name
				fmt.Printf("\n🔧 %s(%s)\n", name, truncate(args, 60))
			},
			func(name, result string) {
				prefix := "✅"
				if strings.HasPrefix(result, "Error") {
					prefix = "❌"
				}
				fmt.Printf("%s %s → %s\n", prefix, name, truncate(result, 80))
			},
		)

		turnCancel()

		if errors.Is(err, context.Canceled) {
			fmt.Println()
			return
		}

		if err != nil {
			if result != "" {
				fmt.Println(result)
			}
			fmt.Printf("❌ Error: %v\n", err)
		} else if result != "" {
			if lastTool == "" {
				fmt.Println()
				fmt.Println(result)
			} else {
				fmt.Printf("\n%s\n", result)
			}
		}

		fmt.Printf("(completed in %s)\n", time.Since(start).Round(time.Millisecond))
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
