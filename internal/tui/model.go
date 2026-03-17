package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/cascade-cli/cascade/internal/app"
	"github.com/cascade-cli/cascade/pkg/types"
)

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
	app      *app.App
	chat     ChatModel
	input    InputModel
	status   StatusModel
	spinner  SpinnerModel
	confirm  ConfirmModel
	renderer *StreamRenderer
	keys     KeyMap
	width    int
	height   int
	state    UIState
	cancel   context.CancelFunc
}

// NewModel creates a new TUI model wired to the given application.
func NewModel(application *app.App) Model {
	return Model{
		app:      application,
		chat:     NewChatModel(80, 20),
		input:    NewInputModel(),
		status:   NewStatusModel(application.Config.Model.Model, "v0.1.0", application.Permissions.Mode()),
		spinner:  NewSpinnerModel(),
		confirm:  NewConfirmModel(),
		renderer: NewStreamRenderer(),
		keys:     DefaultKeyMap(),
		state:    StateIdle,
	}
}

// Init initializes the TUI: start event polling and the render tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.pollEvents(),
		m.tickCmd(),
	)
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
		}
		return m, m.tickCmd()

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
		var inputCmd tea.Cmd
		m.input, inputCmd = m.input.Update(msg)
		cmds = append(cmds, inputCmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the entire TUI layout.
func (m Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var view string

	// Chat viewport (flex -- takes remaining space)
	view += m.chat.View() + "\n"

	// Spinner (conditional)
	if m.spinner.Active() {
		view += m.spinner.View() + "\n"
	}

	// Confirm prompt (conditional)
	if m.confirm.Active() {
		view += m.confirm.View() + "\n"
	}

	// Input area
	view += m.input.View() + "\n"

	// Status bar
	view += m.status.View()

	v := tea.NewView(view)
	v.AltScreen = true
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
			// User responded, return to previous state
			m.state = StateStreaming
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
		// Clear is handled by re-rendering
		return m, nil

	case m.keys.Background.Matches(key):
		m.status.SetMessage("Background mode not implemented yet")
		return m, nil

	case m.keys.Refresh.Matches(key):
		m.status.SetMessage("Cache refresh not implemented yet")
		return m, nil

	case m.keys.Submit.Matches(key):
		if m.state == StateIdle && m.input.Focused() {
			text := m.input.Value()
			if text == "" {
				return m, nil
			}
			m.input.Reset()
			m.input.Blur()

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
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
		return m, nil
	}
}

// handleAgentEvent processes events from the agent event channel.
func (m Model) handleAgentEvent(event types.Event) (tea.Model, tea.Cmd) {
	switch e := event.(type) {
	case *types.TokenEvent:
		m.renderer.Push(e.Token)
		return m, m.pollEvents()

	case *types.ToolStartEvent:
		m.state = StateToolExecuting
		m.spinner.Start(e.Name)
		m.status.SetToolName(e.Name)
		return m, tea.Batch(m.pollEvents(), m.spinner.Tick())

	case *types.ToolEndEvent:
		m.spinner.Stop()
		m.status.SetToolName("")
		m.chat.AddMessage(ChatMessage{
			Role:    "tool",
			Content: FormatToolResult(e.Name, e.Content, e.IsError),
		})
		m.state = StateStreaming
		return m, m.pollEvents()

	case *types.PermissionRequestEvent:
		m.state = StateConfirming
		m.input.Blur()
		m.confirm.Show(e.ToolName, e.Input, e.RiskLevel, e.Response)
		return m, m.pollEvents()

	case *types.ErrorEvent:
		m.chat.AddMessage(ChatMessage{Role: "error", Content: e.Err.Error()})
		return m, m.pollEvents()

	case *types.DoneEvent:
		// Final render: drain any remaining tokens and render with Glamour
		m.renderer.DrainAll()
		content := m.renderer.Content()
		if content != "" {
			m.chat.AddMessage(ChatMessage{Role: "assistant", Content: content})
		}
		m.renderer.Reset()
		m.state = StateIdle
		m.status.SetToolName("")
		m.spinner.Stop()
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
// that resolves when the turn completes.
func (m Model) runAgent(ctx context.Context, input string) tea.Cmd {
	return func() tea.Msg {
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
	// Reserve space: status bar (1 line), input (3 lines + 2 border), spinner (1 conditional)
	inputHeight := 5
	statusHeight := 1
	spinnerHeight := 0
	if m.spinner.Active() {
		spinnerHeight = 1
	}
	confirmHeight := 0
	if m.confirm.Active() {
		confirmHeight = 3
	}

	chatHeight := m.height - inputHeight - statusHeight - spinnerHeight - confirmHeight - 1
	if chatHeight < 3 {
		chatHeight = 3
	}

	m.chat.SetSize(m.width, chatHeight)
	m.input.SetWidth(m.width)
	m.status.SetWidth(m.width)
}
