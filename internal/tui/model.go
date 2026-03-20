package tui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/yogirk/cascade/internal/app"
	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/pkg/types"
)

// validModelName matches model identifiers like "gemini-2.5-pro" or "models/gemini-2.5-flash".
var validModelName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,127}$`)

// UIState represents the current state of the TUI.
type UIState int

const (
	StateIdle          UIState = iota // Waiting for user input
	StateStreaming                    // Receiving streaming LLM tokens
	StateToolExecuting               // A tool is currently executing
	StateConfirming                  // Waiting for permission confirmation
)

// tickMsg triggers a render cycle to drain the token buffer.
type tickMsg time.Time

// agentEventMsg wraps an agent event received from the event channel.
type agentEventMsg struct {
	event types.Event
}

// agentDoneMsg signals that the agent RunTurn goroutine finished.
type agentDoneMsg struct {
	err error
}

// Model is the root Bubble Tea model for the Cascade TUI.
type Model struct {
	app           *app.App
	chat          ChatModel
	input         InputModel
	status        StatusModel
	spinner       SpinnerModel
	confirm       ConfirmModel
	welcome       WelcomeModel
	renderer      *StreamRenderer
	keys          KeyMap
	width         int
	height        int
	state         UIState
	cancel          context.CancelFunc
	showWelcome     bool
	lastToolStart   *types.ToolStartEvent
	preConfirmState UIState // State to restore after confirmation
}

// NewModel creates a new TUI model wired to the given application.
func NewModel(application *app.App) Model {
	gitBranch := DetectGitBranch()
	cwd, _ := os.Getwd()
	shortCwd := ShortenPath(cwd)

	mode := application.Permissions.Mode()
	modelName := application.Config.Model.Model

	status := NewStatusModel(modelName, mode)
	status.SetGitBranch(gitBranch)
	status.SetCwd(shortCwd)

	welcome := NewWelcomeModel(modelName, mode, shortCwd, gitBranch)

	return Model{
		app:         application,
		chat:        NewChatModel(80, 20),
		input:       NewInputModel(),
		status:      status,
		spinner:     NewSpinnerModel(),
		confirm:     NewConfirmModel(),
		welcome:     welcome,
		renderer:    NewStreamRenderer(),
		keys:        DefaultKeyMap(),
		state:       StateIdle,
		showWelcome: true,
	}
}

// Init initializes the TUI: start event polling and the render tick.
func (m Model) Init() tea.Cmd {
	return m.pollEvents() // Tick starts on-demand when streaming begins
}

// Update processes messages and returns the updated model and any commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tickMsg:
		if m.state == StateStreaming {
			drained := m.renderer.DrainAll()
			if drained > 0 {
				m.chat.SetStreamingContent(m.renderer.Content())
			}
			return m, m.tickCmd() // Re-arm only while streaming
		}
		// Don't re-arm when idle — saves CPU/battery
		return m, nil

	case agentEventMsg:
		return m.handleAgentEvent(msg.event)

	case agentDoneMsg:
		// Agent goroutine finished
		if msg.err != nil {
			m.chat.AddMessage(ChatMessage{Role: "error", Content: msg.err.Error()})
		}
		return m, m.pollEvents()
	}

	// Forward to sub-models
	if m.spinner.Active() {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	var chatCmd tea.Cmd
	m.chat, chatCmd = m.chat.Update(msg)
	cmds = append(cmds, chatCmd)

	if m.state == StateIdle && m.input.Focused() {
		prevH := m.input.Height()
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		cmds = append(cmds, inputCmd)
		// Re-layout only when input height actually changed
		if m.input.Height() != prevH {
			m.layout()
		}
	}

	return m, tea.Batch(cmds...)
}

// contentMargin is the left indent applied to all content except the status bar.
const contentMargin = "  "

// View renders the entire TUI layout.
func (m Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var view string

	// Welcome screen or chat viewport
	if m.showWelcome && m.chat.MessageCount() == 0 {
		view += indentBlock(m.welcome.View(), " ") + "\n"
	} else {
		view += indentBlock(m.chat.View(), contentMargin) + "\n"
	}

	// Spinner (conditional)
	if m.spinner.Active() {
		view += contentMargin + m.spinner.View() + "\n"
	}

	// Confirm prompt (conditional)
	if m.confirm.Active() {
		view += indentBlock(m.confirm.View(), contentMargin) + "\n"
	}

	// Input area (1-space indent to align with welcome box)
	view += indentBlock(m.input.View(), " ") + "\n"

	// Status bar (edge-to-edge, no indent)
	view += m.status.View()

	v := tea.NewView(view)
	v.AltScreen = true
	// MouseModeNone: let the terminal handle text selection natively.
	// Scrolling is handled via keyboard (arrows, pgup/pgdown).
	return v
}

// handleKey processes keyboard input.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Confirmation mode: only allow confirm keys
	if m.state == StateConfirming {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		if !m.confirm.Active() {
			// User responded, restore pre-confirm state
			m.state = m.preConfirmState
			m.status.SetPendingApproval(false)
			cmds := []tea.Cmd{cmd, m.input.Focus()}
			return m, tea.Batch(cmds...)
		}
		return m, cmd
	}

	switch {
	case m.keys.Cancel.Matches(key):
		if m.state != StateIdle && m.cancel != nil {
			// Cancel current operation
			m.cancel()
			m.cancel = nil
			m.state = StateIdle
			m.renderer.Reset()
			m.spinner.Stop()
			m.status.SetToolName("")
			m.chat.AddMessage(ChatMessage{Role: "error", Content: "Operation cancelled."})
			return m, m.input.Focus()
		}
		// Idle: quit
		return m, tea.Quit

	case m.keys.Exit.Matches(key):
		return m, tea.Quit

	case m.keys.CycleMode.Matches(key):
		m.app.Permissions.CycleMode()
		m.status.SetMode(m.app.Permissions.Mode())
		return m, nil

	case m.keys.ClearScreen.Matches(key):
		return m, nil

	case m.keys.Background.Matches(key):
		m.status.SetMessage("Background mode not implemented yet")
		return m, nil

	case m.keys.Refresh.Matches(key):
		m.status.SetMessage("Cache refresh not implemented yet")
		return m, nil

	case m.keys.CopyLast.Matches(key):
		if content := m.chat.LastAssistantContent(); content != "" {
			m.status.SetMessage("Copied to clipboard")
			return m, tea.SetClipboard(content)
		}
		m.status.SetMessage("Nothing to copy")
		return m, nil

	// Up/Down: history when idle+focused, scroll otherwise
	case m.keys.ScrollUp.Matches(key):
		if m.state == StateIdle && m.input.Focused() {
			if m.input.HistoryUp() {
				m.layout()
				return m, nil
			}
		}
		m.chat.ScrollUp(1)
		return m, nil
	case m.keys.ScrollDown.Matches(key):
		if m.state == StateIdle && m.input.Focused() {
			if m.input.HistoryDown() {
				m.layout()
				return m, nil
			}
		}
		m.chat.ScrollDown(1)
		return m, nil
	case m.keys.PageUp.Matches(key):
		m.chat.HalfPageUp()
		return m, nil
	case m.keys.PageDown.Matches(key):
		m.chat.HalfPageDown()
		return m, nil

	case m.keys.Submit.Matches(key):
		if m.state == StateIdle && m.input.Focused() {
			text := m.input.Value()
			if text == "" {
				return m, nil
			}
			m.input.PushHistory(text)
			m.input.Reset()
			m.input.Blur()

			// Persist welcome screen into chat history on first message
			if m.showWelcome {
				m.chat.AddMessage(ChatMessage{Role: "welcome", Content: m.welcome.View()})
				m.showWelcome = false
			}

			// Intercept slash commands
			if strings.HasPrefix(text, "/") {
				cmd := m.handleSlashCommand(text)
				return m, tea.Batch(m.input.Focus(), cmd)
			}

			// Add user message to chat
			m.chat.AddMessage(ChatMessage{Role: "user", Content: text})

			// Start agent turn
			m.state = StateStreaming
			m.renderer.Reset()

			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel

			return m, tea.Batch(
				m.runAgent(ctx, text),
				m.pollEvents(),
			)
		}
		// When not idle, forward enter to input (if focused)
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	default:
		// Forward to input when idle
		if m.state == StateIdle && m.input.Focused() {
			prevH := m.input.Height()
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			// Re-layout only when input height actually changed (shift+enter newline)
			if m.input.Height() != prevH {
				m.layout()
			}
			return m, cmd
		}
		return m, nil
	}
}

// handleAgentEvent processes events from the agent event channel.
func (m Model) handleAgentEvent(event types.Event) (tea.Model, tea.Cmd) {
	switch e := event.(type) {
	case *types.TurnStartEvent:
		_ = e
		m.chat.ResumeFollow() // New turn: auto-scroll to follow output
		return m, m.pollEvents()

	case *types.TokenEvent:
		m.renderer.Push(e.Token)
		return m, m.pollEvents()

	case *types.StreamStartEvent:
		m.state = StateStreaming
		m.renderer.Reset()
		m.chat.SetStreamingContent("")
		m.spinner.StartThinking()
		return m, tea.Batch(m.pollEvents(), m.spinner.Tick(), m.tickCmd())

	case *types.StreamCompleteEvent:
		m.spinner.Stop()
		m.renderer.DrainAll()
		if e.Content != "" {
			m.chat.AddMessage(ChatMessage{Role: "assistant", Content: e.Content})
		}
		if e.Usage != nil {
			m.status.AddTokens(e.Usage.PromptTokens, e.Usage.CompletionTokens, e.Usage.TotalTokens)
		}
		m.renderer.Reset()
		m.chat.SetStreamingContent("")
		return m, m.pollEvents()

	case *types.ToolStartEvent:
		m.state = StateToolExecuting
		m.lastToolStart = e
		m.spinner.StartTool(e.Name)
		m.status.SetToolName(e.Name)
		return m, tea.Batch(m.pollEvents(), m.spinner.Tick())

	case *types.ToolEndEvent:
		m.spinner.Stop()
		m.status.SetToolName("")
		msg := ChatMessage{
			Role:    "tool",
			Content: e.Content,
			Display: e.Display,
			IsError: e.IsError,
		}
		if m.lastToolStart != nil {
			msg.ToolName = m.lastToolStart.Name
			msg.ToolArgs = m.lastToolStart.Input
		}
		m.chat.AddMessage(msg)
		m.lastToolStart = nil
		m.state = StateStreaming
		return m, m.pollEvents()

	case *types.PermissionRequestEvent:
		m.preConfirmState = m.state // Save state to restore after confirm
		m.state = StateConfirming
		m.spinner.Stop()
		m.status.SetPendingApproval(true)
		m.status.SetToolName("")
		m.input.Blur()
		m.confirm.Show(e.ToolName, e.Input, e.RiskLevel, e.Response)
		m.layout() // recalc since spinner stopped and confirm appeared
		return m, m.pollEvents()

	case *types.ErrorEvent:
		m.chat.AddMessage(ChatMessage{Role: "error", Content: e.Err.Error()})
		return m, m.pollEvents()

	case *types.DoneEvent:
		m.renderer.Reset()
		m.chat.SetStreamingContent("")
		m.state = StateIdle
		m.status.SetToolName("")
		m.spinner.Stop()
		m.lastToolStart = nil
		return m, m.input.Focus()
	}

	return m, m.pollEvents()
}

// pollEvents returns a command that reads the next event from the app's
// event channel and wraps it as an agentEventMsg.
func (m Model) pollEvents() tea.Cmd {
	return func() tea.Msg {
		event, ok := <-m.app.Events
		if !ok {
			return nil
		}
		return agentEventMsg{event: event}
	}
}

// runAgent starts the agent's RunTurn in a goroutine and returns a command
// that resolves when the turn completes. Recovers from panics to prevent
// the TUI from silently hanging.
func (m Model) runAgent(ctx context.Context, input string) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				msg = agentDoneMsg{err: fmt.Errorf("agent panic: %v", r)}
			}
		}()
		err := m.app.Agent.RunTurn(ctx, input)
		return agentDoneMsg{err: err}
	}
}

// tickCmd returns a command that fires a tickMsg every 33ms (~30fps).
func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(33*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// layout recalculates component dimensions based on the terminal size.
func (m *Model) layout() {
	marginWidth := len(contentMargin)
	contentWidth := m.width - marginWidth

	inputHeight := m.input.Height()
	statusHeight := 1
	spinnerHeight := 0
	if m.spinner.Active() {
		spinnerHeight = 1
	}
	confirmHeight := m.confirm.Height()

	chatHeight := m.height - inputHeight - statusHeight - spinnerHeight - confirmHeight - 1
	if chatHeight < 3 {
		chatHeight = 3
	}

	m.chat.SetSize(contentWidth, chatHeight)
	m.welcome.SetSize(m.width-1, chatHeight) // -1 for the 1-space left indent
	m.input.SetWidth(m.width - 1)            // Same indent as welcome box
	m.status.SetWidth(m.width) // Status bar stays full-width
}

// indentBlock prepends a margin to each line of a multi-line string.
func indentBlock(s string, margin string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = margin + line
	}
	return strings.Join(lines, "\n")
}

// handleSlashCommand processes slash commands entered by the user.
// Returns a tea.Cmd for commands that need async work (e.g., clipboard).
func (m *Model) handleSlashCommand(text string) tea.Cmd {
	cmd := strings.TrimSpace(text)

	switch {
	case cmd == "/help":
		help := strings.Join([]string{
			"Commands:",
			"  /help           Show this help",
			"  /clear          Clear conversation",
			"  /copy           Copy last response",
			"  /copy-code      Copy last code block",
			"  /model <name>   Switch LLM model",
			"",
			"Shortcuts:",
			"  Enter           Send message",
			"  Shift+Enter     New line",
			"  ↑ / ↓           Input history",
			"  PgUp / PgDown   Scroll chat",
			"  Ctrl+Y          Copy last response",
			"  Shift+Tab       Cycle permission mode",
			"  Ctrl+C          Cancel / quit",
			"  Ctrl+D          Exit",
			"",
			"Text selection:",
			"  Option+drag (macOS) or Shift+drag (Linux) to select text",
		}, "\n")
		m.chat.AddMessage(ChatMessage{Role: "system", Content: help})

	case cmd == "/clear":
		m.chat.Clear()

	case cmd == "/copy":
		if content := m.chat.LastAssistantContent(); content != "" {
			m.status.SetMessage("Copied to clipboard")
			return tea.SetClipboard(content)
		}
		m.chat.AddMessage(ChatMessage{Role: "system", Content: "No assistant response to copy."})

	case cmd == "/copy-code":
		if code := m.chat.LastCodeBlock(); code != "" {
			m.status.SetMessage("Code block copied")
			return tea.SetClipboard(code)
		}
		m.chat.AddMessage(ChatMessage{Role: "system", Content: "No code block found in last response."})

	case strings.HasPrefix(cmd, "/model "):
		newModel := strings.TrimSpace(strings.TrimPrefix(cmd, "/model "))
		if newModel == "" {
			m.chat.AddMessage(ChatMessage{Role: "system", Content: "Current model: " + m.app.Config.Model.Model})
			return nil
		}
		if !validModelName.MatchString(newModel) {
			m.chat.AddMessage(ChatMessage{
				Role:    "error",
				Content: "Invalid model name. Use alphanumeric characters, hyphens, dots, and slashes.",
			})
			return nil
		}
		m.app.Config.Model.Model = newModel
		if switcher, ok := m.app.Provider.(provider.ModelSwitcher); ok {
			switcher.SetModel(newModel)
		}
		m.status.SetModel(newModel)
		m.chat.AddMessage(ChatMessage{
			Role:    "system",
			Content: "Switched model to " + newModel + ".",
		})

	case cmd == "/model":
		m.chat.AddMessage(ChatMessage{Role: "system", Content: "Current model: " + m.app.Config.Model.Model})

	default:
		m.chat.AddMessage(ChatMessage{
			Role:    "error",
			Content: "Unknown command: " + cmd + ". Type /help for available commands.",
		})
	}
	return nil
}
