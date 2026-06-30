package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/talen/tce/internal/agent"
	"github.com/talen/tce/internal/project"
	"github.com/talen/tce/internal/util"
)

type sessionState int

const (
	stateIdle sessionState = iota
	stateRunning
)

type (
	tokenMsg     string
	toolStartMsg struct {
		name string
		args string
	}
	toolEndMsg struct {
		name   string
		result string
	}
	agentDoneMsg struct {
		err error
	}
)

type Model struct {
	program *tea.Program

	state   sessionState
	width   int
	height  int
	ready   bool

	profile  *project.Profile
	agentCfg agent.Config
	agent    *agent.Agent
	cancel   context.CancelFunc

	input    textinput.Model
	spinner  spinner.Model
	viewport viewport.Model

	content strings.Builder
	mu      sync.Mutex
}

const maxViewportLines = 5000

var (
	styleStatus = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("255")).
			Padding(0, 1)

	styleUser = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	styleToolCall = lipgloss.NewStyle().
			Foreground(lipgloss.Color("220"))

	styleToolResult = lipgloss.NewStyle().
			Foreground(lipgloss.Color("83"))

	styleError = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	styleSeparator = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	styleContent = lipgloss.NewStyle()
)

func NewModel(profile *project.Profile, agentCfg agent.Config) Model {
	ti := textinput.New()
	ti.Placeholder = "Ask something..."
	ti.Focus()
	ti.Width = 80
	ti.Prompt = "> "

	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("83"))
	s.Spinner = spinner.Dot

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	return Model{
		state:    stateIdle,
		profile:  profile,
		agentCfg: agentCfg,
		agent:    agent.New(agentCfg),
		input:    ti,
		spinner:  s,
		viewport: vp,
	}
}

func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = msg.Height - 4
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case tokenMsg:
		m.mu.Lock()
		m.content.WriteString(string(msg))
		m.mu.Unlock()
		m.syncViewport()
		return m, nil

	case toolStartMsg:
		line := fmt.Sprintf("\n\n  %s %s(%s)\n",
			styleToolCall.Render("🔧"),
			msg.name,
			util.Truncate(msg.args, 60))
		m.mu.Lock()
		m.content.WriteString(line)
		m.mu.Unlock()
		m.syncViewport()
		return m, nil

	case toolEndMsg:
		prefix := styleToolResult.Render("✅")
		if strings.HasPrefix(msg.result, "Error") {
			prefix = styleError.Render("❌")
		}
		line := fmt.Sprintf("  %s %s → %s\n", prefix, msg.name, util.Truncate(msg.result, 80))
		m.mu.Lock()
		m.content.WriteString(line)
		m.mu.Unlock()
		m.syncViewport()
		return m, nil

	case agentDoneMsg:
		m.mu.Lock()
		if msg.err != nil {
			m.content.WriteString(fmt.Sprintf("\n  %s Error: %v\n", styleError.Render("❌"), msg.err))
		} else {
			m.content.WriteString(fmt.Sprintf("\n  %s\n", styleSeparator.Render("── completed ──")))
		}
		m.mu.Unlock()
		m.syncViewport()

		m.state = stateIdle
		m.input.Reset()
		m.input.Focus()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.state == stateRunning {
			return m, cmd
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) syncViewport() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.viewport.SetContent(m.content.String())
	m.viewport.GotoBottom()
}

func (m *Model) trimContent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	lines := strings.Count(m.content.String(), "\n")
	if lines > maxViewportLines {
		s := m.content.String()
		for i := lines - maxViewportLines; i > 0; i-- {
			idx := strings.IndexByte(s, '\n')
			if idx < 0 {
				break
			}
			s = s[idx+1:]
		}
		m.content.Reset()
		m.content.WriteString(s)
	}
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		if m.state == stateRunning && m.cancel != nil {
			m.cancel()
			m.mu.Lock()
			m.content.WriteString(fmt.Sprintf("\n  %s Canceled\n", styleError.Render("✗")))
			m.mu.Unlock()
			m.syncViewport()
			m.state = stateIdle
			m.input.Reset()
			m.input.Focus()
			return m, nil
		}
		return m, tea.Quit

	case tea.KeyEnter:
		if m.state == stateRunning {
			return m, nil
		}
		prompt := strings.TrimSpace(m.input.Value())
		if prompt == "" {
			return m, nil
		}

		if strings.HasPrefix(prompt, "/") {
			parts := strings.Fields(prompt)
			if len(parts) > 0 {
				switch parts[0] {
				case "/exit":
					return m, tea.Quit
				case "/clear":
					m.mu.Lock()
					m.content.Reset()
					m.mu.Unlock()
					m.viewport.SetContent("")
					return m, nil
				default:
					help := styleUser.Render("Commands: /exit  /clear\n")
					m.mu.Lock()
					m.content.WriteString(help)
					m.mu.Unlock()
					m.syncViewport()
				}
			}
			m.input.SetValue("")
			return m, nil
		}

		m.mu.Lock()
		m.content.WriteString(fmt.Sprintf("\n%s %s\n", styleUser.Render(">"), prompt))
		m.mu.Unlock()
		m.syncViewport()

		m.input.Blur()
		m.input.SetValue("")
		m.state = stateRunning

		m.runAgent(prompt)
		return m, m.spinner.Tick

	default:
		if m.state == stateIdle {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}
}

func (m *Model) runAgent(prompt string) {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	go func() {
		defer cancel()
		_, err := m.agent.Run(ctx, prompt,
			func(token string) {
				if m.program != nil {
					m.program.Send(tokenMsg(token))
				}
			},
			func(name, args string) {
				if m.program != nil {
					m.program.Send(toolStartMsg{name, args})
				}
			},
			func(name, result string) {
				if m.program != nil {
					m.program.Send(toolEndMsg{name, result})
				}
			},
		)
		if m.program != nil {
			m.program.Send(agentDoneMsg{err})
		}
		m.trimContent()
	}()
}

func (m *Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	running := ""
	if m.state == stateRunning {
		running = fmt.Sprintf(" %s running", m.spinner.View())
	}

	statusText := fmt.Sprintf(" tce  %s  %s  %s%s",
		m.profile.Summary(),
		strings.ToUpper(string(m.agentCfg.Type)),
		m.agentCfg.LLM.ModelName(),
		running)

	statusBar := styleStatus.Width(m.width - 2).Render(statusText)

	chatPanel := m.viewport.View()
	inputLine := fmt.Sprintf("> %s", m.input.View())

	return fmt.Sprintf("%s\n%s\n%s", statusBar, chatPanel, inputLine)
}
