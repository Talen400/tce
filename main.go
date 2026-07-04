package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/talen/tce/internal/agent"
	"github.com/talen/tce/internal/config"
	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/mcp"
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
	branch := flag.String("branch", "", "Create and switch to a new git branch before starting")
	output := flag.String("output", "text", "Output format: text, json, silent")
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
	toolReg.Register(&tools.CommitTool{})
	toolReg.Register(&tools.ReviewTool{})

	// Register external tools from .tce.yaml
	for name, tc := range projectCfg.Tools {
		desc := tc.Description
		if desc == "" {
			desc = fmt.Sprintf("Custom tool: %s", tc.Command)
		}
		toolReg.Register(&tools.ExternalTool{
			NameVal:      name,
			DescVal:      desc,
			ShortDescVal: desc,
			Command:      tc.Command,
		})
	}

	// Connect to MCP servers from .tce.yaml and register their tools
	for name, mc := range projectCfg.MCPServers {
		mcpCfg := mcp.MCPServerConfig{
			Name:    name,
			Command: mc.Command,
			Args:    mc.Args,
		}
		mcpClient, err := mcp.NewClient(mcpCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP server %q failed: %v\n", name, err)
			continue
		}
		toolDefs, err := mcpClient.ListTools()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: MCP server %q tools/list failed: %v\n", name, err)
			mcpClient.Close()
			continue
		}
		for _, td := range toolDefs {
			desc := td.Description
			if desc == "" {
				desc = fmt.Sprintf("MCP tool %s from server %s", td.Name, name)
			}
			toolReg.Register(&mcp.ToolAdapter{
				Def:      td,
				Client:   mcpClient,
				ToolDesc: desc,
			})
		}
	}

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
		Stdin:           os.Stdin,
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

	if *branch != "" {
		cmd := exec.Command("git", "checkout", "-b", *branch)
		cmd.Dir = absRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating branch %q: %v\n%s\n", *branch, err, string(out))
			os.Exit(1)
		}
		fmt.Printf("🌿 Switched to new branch: %s\n", *branch)
	}

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
		runCLI(ctx, ag, profile, at, llmClient.ModelName(), *output)
		turns, tokIn, tokOut := ag.Stats()
		if turns > 1 && *output != "silent" {
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

func runCLI(ctx context.Context, ag *agent.Agent, profile *project.Profile, agentType agent.AgentType, modelName string, output string) {
	scanner := bufio.NewScanner(os.Stdin)
	if output != "silent" {
		fmt.Printf("tce v%s\n", version)
		fmt.Printf("Project: %s  Agent: %s  Model: %s\n\n", profile.Summary(), agentType, modelName)
		fmt.Println("Commands: /help, /exit, /clear, /project, /git | Type your prompt:")
	}

	for {
		if output != "silent" {
			fmt.Print("> ")
		}
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
			if output != "silent" {
				fmt.Print("\033[H\033[2J")
			}
			continue
		case input == "/help":
			if output != "silent" {
				fmt.Println("Commands: /exit, /clear, /project, /git")
			}
			continue
		case input == "/project":
			if output != "silent" {
				fmt.Println(profile.String())
			}
			continue
		case input == "/git" || strings.HasPrefix(input, "/git "):
			gitInfo := gitStatus(profile.Root)
			if output == "json" {
				fmt.Println(gitJSON(profile.Root))
			} else if output != "silent" {
				fmt.Print(gitInfo)
			}
			continue
		}

		if output == "silent" {
			turnCtx, turnCancel := context.WithCancel(ctx)
			_, err := ag.Run(turnCtx, input, nil, nil, nil)
			turnCancel()
			if err != nil && !errors.Is(err, context.Canceled) {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
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

		if output == "json" {
			jsonOut := map[string]any{
				"result":  result,
				"error":   errToString(err),
				"tools":   lastTool,
				"elapsed": time.Since(start).String(),
			}
			data, _ := json.Marshal(jsonOut)
			fmt.Println(string(data))
			continue
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

func gitStatus(root string) string {
	branch, err := exec.Command("git", "-C", root, "branch", "--show-current").Output()
	if err != nil {
		return "Not a git repository.\n"
	}
	status, _ := exec.Command("git", "-C", root, "status", "--short").Output()
	branchStr := strings.TrimSpace(string(branch))
	statusStr := strings.TrimSpace(string(status))

	var b strings.Builder
	b.WriteString("── git ──\n")
	b.WriteString(fmt.Sprintf("  🌿 Branch: %s\n", branchStr))
	if statusStr == "" {
		b.WriteString("  ✓ Clean working tree\n")
	} else {
		for _, line := range strings.Split(statusStr, "\n") {
			b.WriteString(fmt.Sprintf("    %s\n", line))
		}
	}
	return b.String()
}

func gitJSON(root string) string {
	branch, _ := exec.Command("git", "-C", root, "branch", "--show-current").Output()
	status, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output()
	statusStr := strings.TrimSpace(string(status))
	var files []string
	if statusStr != "" {
		files = append(files, strings.Split(statusStr, "\n")...)
	}
	out := map[string]any{
		"branch": strings.TrimSpace(string(branch)),
		"files":  files,
		"dirty":  len(files) > 0,
	}
	data, _ := json.Marshal(out)
	return string(data)
}

func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
