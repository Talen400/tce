package main

import (
	"bufio"
	"errors"
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/talen/tce/internal/agent"
	"github.com/talen/tce/internal/llm"
	"github.com/talen/tce/internal/project"
	"github.com/talen/tce/internal/tools"
	"github.com/talen/tce/internal/tui"
)

var version = "0.1.0"

var pythonCmd *exec.Cmd

func main() {
	model := flag.String("model", "", "LLM model name")
	baseURL := flag.String("base-url", "", "LLM API base URL")
	apiKey := flag.String("api-key", "", "LLM API key")
	agentType := flag.String("agent", "build", "Agent type: build, plan, explore")
	projectRoot := flag.String("dir", ".", "Project root directory")
	minimal := flag.Bool("minimal", false, "Use minimal prompt (better for small models <4B)")
	cliMode := flag.Bool("cli", false, "Use CLI mode instead of TUI")
	contextSize := flag.Int("context-size", 0, "Max context tokens before compaction (0=auto)")
	serveMode := flag.Bool("serve", false, "Start local Python backend automatically")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *serveMode {
		pythonCmd = startPythonBackend()
		defer func() {
			if pythonCmd != nil && pythonCmd.Process != nil {
				pythonCmd.Process.Kill()
			}
		}()
		if *baseURL == "" {
			*baseURL = "http://127.0.0.1:8001/v1"
		}
		if *apiKey == "" {
			*apiKey = "not-needed"
		}
	}

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

	llmCfg.Temperature = 0.0

	modelProfile := llm.MatchProfile(llmCfg.Model)
	if *model != "" {
		modelProfile = llm.MatchProfile(*model)
	}
	if llmCfg.Temperature == 0 {
		llmCfg.Temperature = modelProfile.Temperature
	}

	llmCfg.ResponseFormat = modelProfile.ResponseFormat

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

	if *cliMode {
		runCLI(ctx, ag, profile, at, llmClient.ModelName())
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

func startPythonBackend() *exec.Cmd {
	script := filepath.Join(filepath.Dir(os.Args[0]), "serve.py")
	if _, err := os.Stat(script); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		script = filepath.Join(cwd, "serve.py")
	}
	if _, err := os.Stat(script); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "serve.py not found next to binary or in current directory\n")
		os.Exit(1)
	}

	port := "8001"
	healthURL := "http://127.0.0.1:" + port + "/health"
	hc := &http.Client{Timeout: 1 * time.Second}

	if resp, err := hc.Get(healthURL); err == nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			fmt.Println("Python backend already running on port " + port)
			return nil
		}
		fmt.Fprintf(os.Stderr, "Port %s is in use but not a TCE backend\n", port)
		os.Exit(1)
	}

	dir := filepath.Dir(script)
	venvPython := filepath.Join(dir, ".venv", "bin", "python3")
	python := "python3"
	if _, err := os.Stat(venvPython); err == nil {
		python = venvPython
	}

	cmd := exec.Command(python, script)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start Python backend: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Waiting for Python backend to be ready...")
	ready := false
	for i := 0; i < 60; i++ {
		resp, err := hc.Get(healthURL)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(1 * time.Second)
	}
	if !ready {
		fmt.Fprintf(os.Stderr, "Python backend did not start in time\n")
		cmd.Process.Kill()
		os.Exit(1)
	}

	go func() {
		cmd.Wait()
	}()

	return cmd
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
