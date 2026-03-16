package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yogirk/cascade/internal/app"
	plat "github.com/yogirk/cascade/internal/platform"
	"github.com/yogirk/cascade/internal/provider"
	logtool "github.com/yogirk/cascade/internal/tools/logging"
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

// approvalRequestMsg wraps a dedicated permission request received from the
// approval channel.
type approvalRequestMsg struct {
	request types.ApprovalRequest
}

// agentDoneMsg signals that the agent RunTurn goroutine finished.
type agentDoneMsg struct {
	err error
}

// CostTrackerView is the interface for reading cost data in the TUI.
// Implemented by bigquery.CostTracker; wired in Plan 05.
type CostTrackerView interface {
	Entries() []CostEntry
	SessionTotal() float64
	BudgetPercent() float64
	IsOverBudgetWarning() bool
}

// CostEntry mirrors bigquery.QueryCostEntry for TUI display.
// Avoids importing internal/bigquery from the TUI package.
type CostEntry struct {
	SQL          string
	BytesScanned int64
	Cost         float64
	DurationMs   int64
	IsDML        bool
}

// Model is the root Bubble Tea model for the Cascade TUI.
type Model struct {
	app           *app.App
	chat          ChatModel
	input         InputModel
	status        StatusModel
	spinner       SpinnerModel
	confirm       ConfirmModel
	modelPicker   ModelPickerModel
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
	costTracker     CostTrackerView // BigQuery cost tracker (nil until wired)
	dailyBudget     float64         // Daily budget from config (for /cost display)
}

// NewModel creates a new TUI model wired to the given application.
func NewModel(application *app.App) Model {
	// Apply theme override from config before any rendering.
	SetTheme(application.Config.Display.Theme)

	gitBranch := DetectGitBranch()
	cwd, _ := os.Getwd()
	shortCwd := ShortenPath(cwd)

	mode := application.Permissions.Mode()
	modelName := application.Config.Model.Model

	status := NewStatusModel(modelName, mode)
	status.SetGitBranch(gitBranch)
	status.SetCwd(shortCwd)

	project := application.Config.GCP.Project
	datasets := application.Config.BigQuery.Datasets
	welcome := NewWelcomeModel(mode, project, datasets)

	m := Model{
		app:         application,
		chat:        NewChatModel(80, 20),
		input:       NewInputModel(),
		status:      status,
		spinner:     NewSpinnerModel(),
		confirm:     NewConfirmModel(),
		modelPicker: NewModelPicker(),
		welcome:     welcome,
		renderer:    NewStreamRenderer(),
		keys:        DefaultKeyMap(),
		state:       StateIdle,
		showWelcome: true,
	}

	// Wire BigQuery cost tracker if BQ is configured
	if application.BQ != nil && application.BQ.CostTracker != nil {
		m.SetCostTracker(
			newCostTrackerAdapter(application.BQ.CostTracker),
			application.Config.Cost.DailyBudget,
		)
	}

	// Hydrate chat with resumed session messages (if any)
	if msgs := application.Agent.Session().Messages(); len(msgs) > 0 {
		m.showWelcome = false
		for _, msg := range msgs {
			switch msg.Role {
			case types.RoleUser:
				m.chat.AddMessage(ChatMessage{Role: "user", Content: msg.Content})
			case types.RoleAssistant:
				if msg.Content != "" {
					m.chat.AddMessage(ChatMessage{Role: "assistant", Content: msg.Content})
				}
			case types.RoleSystem:
				// Skip system messages (prompt injection, not user-visible)
			case types.RoleTool:
				if msg.ToolResult != nil {
					m.chat.AddMessage(ChatMessage{
						Role:    "tool",
						Content: msg.ToolResult.Content,
						IsError: msg.ToolResult.IsError,
					})
				}
			}
		}
	}

	return m
}

// Init initializes the TUI: start event polling and the render tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.pollEvents(), m.pollApprovals()) // Tick starts on-demand when streaming begins
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

	case approvalRequestMsg:
		return m.handleApprovalRequest(msg.request)

	case agentDoneMsg:
		// Agent goroutine finished
		if msg.err != nil {
			m.chat.AddMessage(ChatMessage{Role: "error", Content: msg.err.Error()})
		}
		return m, m.pollEvents()

	case ChatMessage:
		m.chat.AddMessage(msg)
		return m, nil
	}

	// Forward to sub-models
	if m.spinner.Active() {
		if _, ok := msg.(cascadeTickMsg); ok {
			m.spinner, _ = m.spinner.Update(msg)
			cmds = append(cmds, m.spinner.Tick())
		}
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
		view += indentBlock(m.welcome.View(), " ") + "\n\n"
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

	// Model picker (conditional)
	if m.modelPicker.Active() {
		view += indentBlock(m.modelPicker.View(), contentMargin) + "\n"
	}

	// Input area uses a slightly wider gutter than the transcript.
	view += indentBlock(m.input.View(), "  ") + "\n"

	// Status bar (edge-to-edge, no indent)
	view += m.status.View()

	v := tea.NewView(view)
	v.AltScreen = true
	// CellMotion captures wheel events for trackpad scrolling.
	// Trade-off: native text selection requires Option+drag (macOS).
	// TODO: fix at Bubble Tea level to support wheel-only mouse mode.
	v.MouseMode = tea.MouseModeCellMotion
	v.OnMouse = func(msg tea.MouseMsg) tea.Cmd {
		if wheel, ok := msg.(tea.MouseWheelMsg); ok {
			switch wheel.Button {
			case tea.MouseWheelUp:
				m.chat.ScrollUp(1)
			case tea.MouseWheelDown:
				m.chat.ScrollDown(1)
			}
		}
		return nil
	}
	return v
}

// handleKey processes keyboard input.
func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// When the approval modal is visible, it owns keyboard input regardless of
	// any transient state drift from queued agent events. Takes precedence over
	// the model picker since approvals are time-sensitive.
	if m.confirm.Active() {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		if !m.confirm.Active() {
			// User responded, restore pre-confirm state
			m.state = m.preConfirmState
			m.status.SetPendingApproval(false)
			cmds := []tea.Cmd{cmd}
			switch m.state {
			case StateToolExecuting:
				if m.lastToolStart != nil {
					m.spinner.StartTool(m.lastToolStart.Name)
					m.status.SetToolName(m.lastToolStart.Name)
					cmds = append(cmds, m.spinner.Tick())
				}
				m.input.Blur()
			case StateStreaming:
				m.spinner.StartThinking()
				cmds = append(cmds, m.spinner.Tick(), m.tickCmd())
				m.input.Blur()
			default:
				cmds = append(cmds, m.input.Focus())
			}
			m.layout()
			cmds = append(cmds, m.pollApprovals())
			return m, tea.Batch(cmds...)
		}
		return m, cmd
	}

	// When the model picker is visible, it owns keyboard input (after confirm).
	if m.modelPicker.Active() {
		switch key {
		case "up", "k":
			m.modelPicker.MoveUp()
		case "down", "j":
			m.modelPicker.MoveDown()
		case "enter":
			m.modelPicker.Confirm()
			if chosen := m.modelPicker.Chosen(); chosen != "" {
				m.app.Config.Model.Model = chosen
				if switcher, ok := m.app.Provider.(provider.ModelSwitcher); ok {
					switcher.SetModel(chosen)
				}
				m.status.SetModel(chosen)
				m.chat.AddMessage(ChatMessage{
					Role:    "system",
					Content: "Switched to " + friendlyModelName(chosen) + " (" + chosen + ")",
				})
			}
			m.layout()
			return m, m.input.Focus()
		case "esc", "q":
			m.modelPicker.Dismiss()
			m.layout()
			return m, m.input.Focus()
		}
		m.layout()
		return m, nil
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
			m.chat.AddMessage(ChatMessage{Role: "error", Content: "Operation canceled."})
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

	case key == " " && !m.input.Focused():
		// Space in viewport mode: toggle expand on most recent truncated tool output
		m.chat.ToggleExpand()
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

			// Start agent turn — begin elapsed timer immediately
			m.state = StateStreaming
			m.renderer.Reset()
			m.spinner.StartTurn()
			m.spinner.StartThinking()

			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel

			m.layout()
			return m, tea.Batch(
				m.runAgent(ctx, text),
				m.pollEvents(),
				m.spinner.Tick(),
				m.tickCmd(),
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
		m.layout()
		return m, tea.Batch(m.pollEvents(), m.spinner.Tick(), m.tickCmd())

	case *types.StreamCompleteEvent:
		m.spinner.Stop()
		m.renderer.DrainAll()
		if e.Content != "" {
			m.chat.AddMessage(ChatMessage{Role: "assistant", Content: e.Content})
		}
		if e.Usage != nil {
			m.status.AddTokens(e.Usage.PromptTokens, e.Usage.CompletionTokens, e.Usage.TotalTokens)
			m.spinner.AddTurnTokens(e.Usage.PromptTokens, e.Usage.CompletionTokens)
		}
		m.renderer.Reset()
		m.chat.SetStreamingContent("")
		m.layout()
		return m, m.pollEvents()

	case *types.ToolStartEvent:
		m.state = StateToolExecuting
		m.lastToolStart = e
		m.spinner.StartTool(e.Name)
		m.status.SetToolName(e.Name)
		m.layout()
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
			msg.ToolRiskLevel = m.lastToolStart.RiskLevel
		}
		m.chat.AddMessage(msg)
		m.lastToolStart = nil
		m.state = StateStreaming
		m.layout()
		return m, m.pollEvents()

	case *types.CostUpdateEvent:
		m.status.SetCost(e.SessionTotal)
		// Check budget warning
		if m.costTracker != nil && m.costTracker.IsOverBudgetWarning() {
			pct := int(m.costTracker.BudgetPercent())
			m.chat.AddMessage(ChatMessage{
				Role: "system",
				Content: fmt.Sprintf("Budget alert: session cost $%.2f has reached %d%% of daily budget",
					e.SessionTotal, pct),
			})
		}
		return m, m.pollEvents()

	case *types.CompactEvent:
		m.status.SetMessage("Context compacted")
		msg := fmt.Sprintf("Context compacted (%s -> compacted)", formatTokens(e.BeforeTokens))
		m.chat.AddMessage(ChatMessage{Role: "system", Content: msg})
		return m, m.pollEvents()

	case *types.StatusEvent:
		m.status.SetMessage(e.Message)
		return m, m.pollEvents()

	case *types.ErrorEvent:
		m.chat.AddMessage(ChatMessage{Role: "error", Content: e.Err.Error()})
		return m, m.pollEvents()

	case *types.DoneEvent:
		m.renderer.Reset()
		m.chat.SetStreamingContent("")
		m.state = StateIdle
		m.status.SetToolName("")

		// Persist turn summary (elapsed + tokens) as a dim footer in the chat
		if summary := m.spinner.TurnSummary(); summary != "" {
			m.chat.AddMessage(ChatMessage{
				Role:    "system",
				Display: "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563")).Render(summary),
				Content: summary,
			})
		}

		m.spinner.EndTurn()
		m.lastToolStart = nil
		m.layout()
		return m, m.input.Focus()
	}

	return m, m.pollEvents()
}

func (m Model) handleApprovalRequest(req types.ApprovalRequest) (tea.Model, tea.Cmd) {
	m.preConfirmState = m.state
	m.state = StateConfirming
	m.spinner.Stop()
	m.status.SetPendingApproval(true)
	m.status.SetToolName("")
	m.input.Blur()
	m.confirm.Show(req.ToolName, req.Input, req.RiskLevel, req.Response)
	m.layout()
	return m, nil
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

// pollApprovals reads the next permission request from the dedicated approval
// channel and wraps it as an approvalRequestMsg.
func (m Model) pollApprovals() tea.Cmd {
	return func() tea.Msg {
		request, ok := <-m.app.Approvals
		if !ok {
			return nil
		}
		return approvalRequestMsg{request: request}
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
	pickerHeight := m.modelPicker.Height()

	chatHeight := m.height - inputHeight - statusHeight - spinnerHeight - confirmHeight - pickerHeight - 1
	if chatHeight < 3 {
		chatHeight = 3
	}

	m.chat.SetSize(contentWidth, chatHeight)
	m.welcome.SetSize(m.width-1, chatHeight) // -1 for the 1-space left indent
	m.input.SetTerminalHeight(m.height)
	m.input.SetWidth(m.width - 4)            // 2-space gutter on both sides
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

// SetCostTracker sets the BigQuery cost tracker for /cost display.
// Called during app assembly (Plan 05).
func (m *Model) SetCostTracker(ct CostTrackerView, dailyBudget float64) {
	m.costTracker = ct
	m.dailyBudget = dailyBudget
	m.status.SetDailyBudget(dailyBudget)
}

// formatBytesSimple formats a byte count as a human-readable string.
func formatBytesSimple(bytes int64) string {
	switch {
	case bytes >= 1<<40:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(1<<40))
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// formatDurationSimple formats a millisecond duration as a human-readable string.
func formatDurationSimple(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	secs := float64(ms) / 1000
	if secs < 60 {
		return fmt.Sprintf("%.1fs", secs)
	}
	mins := int(secs) / 60
	remSecs := int(secs) % 60
	return fmt.Sprintf("%dm%ds", mins, remSecs)
}

// renderCostBreakdown renders a styled session cost report.
func renderCostBreakdown(entries []CostEntry, total, dailyBudget float64) (display string, content string) {
	costHeaderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B9FFF")).Bold(true)
	costTitleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
	costDimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	costLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	costValueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6")).Bold(true)
	costAccentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#38BDF8"))
	costSepStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#374151"))
	costWarnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#D97706"))

	var db, cb strings.Builder

	// Title
	db.WriteString("\n")
	db.WriteString("  " + costAccentStyle.Render("≋") + " " +
		costTitleStyle.Render("Session Cost Breakdown") + "  " +
		costDimStyle.Render(fmt.Sprintf("%d queries", len(entries))) + "\n")
	db.WriteString("  " + costSepStyle.Render(strings.Repeat("─", 60)) + "\n\n")
	cb.WriteString(fmt.Sprintf("Session Cost Breakdown (%d queries)\n\n", len(entries)))

	// Query list
	for i, entry := range entries {
		sqlTrunc := strings.ReplaceAll(entry.SQL, "\n", " ")
		if len(sqlTrunc) > 55 {
			sqlTrunc = sqlTrunc[:52] + "..."
		}

		if entry.IsDML {
			db.WriteString(fmt.Sprintf("  %s  %s\n",
				costLabelStyle.Render(fmt.Sprintf("#%-2d", i+1)),
				costDimStyle.Render("DML")))
		} else {
			costStr := fmt.Sprintf("$%.2f", entry.Cost)
			if entry.Cost < 0.01 && entry.Cost > 0 {
				costStr = "<$0.01"
			}
			db.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n",
				costLabelStyle.Render(fmt.Sprintf("#%-2d", i+1)),
				costValueStyle.Render(fmt.Sprintf("%-7s", costStr)),
				costDimStyle.Render(formatBytesSimple(entry.BytesScanned)),
				costDimStyle.Render(formatDurationSimple(entry.DurationMs))))
		}
		db.WriteString("      " + costDimStyle.Render(sqlTrunc) + "\n")

		if i < len(entries)-1 {
			db.WriteString("\n")
		}

		// Plain text
		if entry.IsDML {
			cb.WriteString(fmt.Sprintf("  #%d  DML  %s\n", i+1, sqlTrunc))
		} else {
			cb.WriteString(fmt.Sprintf("  #%d  $%.2f  %s  %s  %s\n",
				i+1, entry.Cost, formatBytesSimple(entry.BytesScanned),
				formatDurationSimple(entry.DurationMs), sqlTrunc))
		}
	}

	// Total
	db.WriteString("\n  " + costSepStyle.Render(strings.Repeat("─", 60)) + "\n")
	db.WriteString("  " + costLabelStyle.Render("Session total  ") +
		costHeaderStyle.Render(fmt.Sprintf("$%.2f", total)) + "\n")
	cb.WriteString(fmt.Sprintf("\n  Session total: $%.2f\n", total))

	// Budget
	if dailyBudget > 0 {
		pct := total / dailyBudget * 100
		budgetLine := fmt.Sprintf("  Budget: $%.2f / $%.2f daily (%.1f%%)", total, dailyBudget, pct)
		if pct >= 80 {
			db.WriteString("  " + costWarnStyle.Render(budgetLine) + "\n")
		} else {
			db.WriteString("  " + costDimStyle.Render(budgetLine) + "\n")
		}
		cb.WriteString(budgetLine + "\n")
	}

	db.WriteString("\n")
	return db.String(), cb.String()
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
			"  /model          Pick model (interactive)",
			"  /compact        Compact conversation context",
			"  /cost           Show session cost breakdown",
			"  /morning        Platform health briefing",
			"  /insights       BigQuery cost health dashboard",
			"  /logs           Recent warnings and errors",
			"  /sync           Refresh schema cache",
			"  /sessions       List saved sessions",
			"  /save           Force-save current session",
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
		m.showWelcome = true // restore welcome banner on empty conversation

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

	case cmd == "/model":
		m.modelPicker.Show(m.app.Config.Model.Provider, m.app.Config.Model.Model)
		if m.modelPicker.Active() {
			m.input.Blur()
			m.layout()
		} else {
			m.chat.AddMessage(ChatMessage{Role: "system",
				Content: "Current model: " + m.app.Config.Model.Model + ". No model list available for provider " + m.app.Config.Model.Provider})
		}

	case strings.HasPrefix(cmd, "/model "):
		input := strings.TrimSpace(strings.TrimPrefix(cmd, "/model "))
		if input == "" {
			m.modelPicker.Show(m.app.Config.Model.Provider, m.app.Config.Model.Model)
			if m.modelPicker.Active() {
				m.input.Blur()
				m.layout()
			}
			return nil
		}

		// Resolve by number first, then by name
		newModel := resolveModelByNumber(m.app.Config.Model.Provider, input)
		if newModel == "" {
			newModel = input
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
			Content: "Switched to " + friendlyModelName(newModel) + " (" + newModel + ")",
		})

	case cmd == "/compact":
		m.chat.AddMessage(ChatMessage{Role: "system", Content: "Compacting context..."})
		return func() tea.Msg {
			ctx := context.Background()
			if err := m.app.Agent.Compact(ctx); err != nil {
				return ChatMessage{Role: "error", Content: fmt.Sprintf("Compaction failed: %v", err)}
			}
			return nil
		}

	case cmd == "/logs" || strings.HasPrefix(cmd, "/logs "):
		if m.app.Platform == nil || m.app.Platform.GetLogClient() == nil {
			m.chat.AddMessage(ChatMessage{Role: "error",
				Content: "Cloud Logging not available. Check GCP credentials (roles/logging.viewer)."})
			return nil
		}
		// Parse args: /logs [severity] [duration]
		severity := m.app.Config.Logging.DefaultSeverity
		if severity == "" {
			severity = "WARNING"
		}
		duration := "1h"
		if args := strings.TrimSpace(strings.TrimPrefix(cmd, "/logs")); args != "" {
			parts := strings.Fields(args)
			if len(parts) >= 1 {
				severity = strings.ToUpper(parts[0])
			}
			if len(parts) >= 2 {
				duration = parts[1]
			}
		}
		filter := fmt.Sprintf("severity >= %s", severity)
		maxEntries := m.app.Config.Logging.MaxEntries
		if maxEntries <= 0 {
			maxEntries = 50
		}

		m.status.SetMessage("Fetching logs...")
		platform := m.app.Platform
		return func() tea.Msg {
			lt := logtool.NewLogTool(platform.GetLogClient, platform.ProjectID, maxEntries)
			input, _ := json.Marshal(map[string]interface{}{
				"action":   "query",
				"filter":   filter,
				"duration": duration,
				"limit":    maxEntries,
			})
			result, _ := lt.Execute(context.Background(), input)
			if result == nil {
				return ChatMessage{Role: "error", Content: "Log query returned no result"}
			}
			return ChatMessage{Role: "system", Content: result.Content, Display: result.Display}
		}

	case cmd == "/morning" || strings.HasPrefix(cmd, "/morning "):
		if m.app.Morning == nil {
			m.chat.AddMessage(ChatMessage{Role: "error",
				Content: "No platform sources available. Configure GCP credentials and BigQuery to use /morning."})
			return nil
		}
		m.chat.AddMessage(ChatMessage{Role: "system", Content: "Gathering platform intelligence..."})
		m.status.SetMessage("Running morning briefing...")
		since := 12 * time.Hour
		if args := strings.TrimSpace(strings.TrimPrefix(cmd, "/morning")); args != "" {
			if d, err := time.ParseDuration(args); err == nil {
				since = d
			}
		}
		return func() tea.Msg {
			report := m.app.Morning.Collect(context.Background(), since)
			display, content := plat.RenderMorningReport(report)
			return ChatMessage{Role: "system", Content: content, Display: display}
		}

	case cmd == "/insights":
		if m.app.BQ == nil {
			m.chat.AddMessage(ChatMessage{Role: "error",
				Content: "BigQuery not configured. Add [gcp] and [bigquery] sections to ~/.cascade/config.toml"})
			return nil
		}
		m.chat.AddMessage(ChatMessage{Role: "system", Content: "Gathering insights..."})
		m.status.SetMessage("Running cost analysis...")
		location := m.app.Config.BigQuery.Location
		if location == "" {
			location = "US"
		}
		return func() tea.Msg {
			report := app.RunInsights(context.Background(), m.app.BQ, location)
			display, content := app.RenderInsightsReport(report)
			return ChatMessage{Role: "system", Content: content, Display: display}
		}

	case cmd == "/cost":
		if m.costTracker == nil {
			m.chat.AddMessage(ChatMessage{Role: "system", Content: "No queries executed this session."})
			return nil
		}
		entries := m.costTracker.Entries()
		if len(entries) == 0 {
			m.chat.AddMessage(ChatMessage{Role: "system", Content: "No queries executed this session."})
			return nil
		}

		display, content := renderCostBreakdown(entries, m.costTracker.SessionTotal(), m.dailyBudget)
		m.chat.AddMessage(ChatMessage{Role: "system", Content: content, Display: display})

	case cmd == "/sync" || strings.HasPrefix(cmd, "/sync "):
		if m.app.BQ == nil {
			m.chat.AddMessage(ChatMessage{Role: "system",
				Content: "No BigQuery project configured. Set [gcp] project or [bigquery] project in ~/.cascade/config.toml"})
			return nil
		}

		// Determine datasets: explicit argument > configured datasets
		datasets := m.app.Config.BigQuery.Datasets
		if arg := strings.TrimSpace(strings.TrimPrefix(cmd, "/sync")); arg != "" {
			datasets = strings.Fields(arg)
		}
		if len(datasets) == 0 {
			m.chat.AddMessage(ChatMessage{Role: "system",
				Content: "Usage: /sync [dataset ...]\nOr add datasets to [bigquery] section in ~/.cascade/config.toml"})
			return nil
		}

		m.chat.AddMessage(ChatMessage{Role: "system",
			Content: fmt.Sprintf("Syncing %d dataset(s): %s ...", len(datasets), strings.Join(datasets, ", "))})
		m.status.SetMessage("Syncing schema...")
		return func() tea.Msg {
			ctx := context.Background()
			var totalTables int

			if arg := strings.TrimSpace(strings.TrimPrefix(cmd, "/sync")); arg != "" {
				// Explicit datasets: use main project populator
				for _, entry := range m.app.BQ.Populators {
					err := entry.Populator.PopulateAll(ctx, datasets, func(completed, total int) {
						totalTables = total
					})
					if err != nil {
						return ChatMessage{Role: "error",
							Content: fmt.Sprintf("Schema refresh failed: %v", err)}
					}
					break // use first populator for explicit datasets
				}
			} else {
				// No args: sync all configured projects/datasets
				for projectID, entry := range m.app.BQ.Populators {
					err := entry.Populator.PopulateAll(ctx, entry.Datasets, func(completed, total int) {
						totalTables += total
					})
					if err != nil {
						return ChatMessage{Role: "error",
							Content: fmt.Sprintf("Schema refresh failed for %s: %v", projectID, err)}
					}
				}
			}

			// Update system prompt with new schema context
			newPrompt := app.BuildSystemPrompt(m.app.BQ, m.app.Config)
			m.app.Agent.Session().SetSystemPrompt(newPrompt)
			return ChatMessage{Role: "system",
				Content: fmt.Sprintf("Schema cache refreshed — %d tables across %s", totalTables, strings.Join(datasets, ", "))}
		}

	case cmd == "/sessions":
		if m.app.Sessions == nil {
			m.chat.AddMessage(ChatMessage{Role: "error", Content: "Session persistence not available."})
			return nil
		}
		sessions, err := m.app.Sessions.ListSessions()
		if err != nil {
			m.chat.AddMessage(ChatMessage{Role: "error", Content: fmt.Sprintf("List sessions: %v", err)})
			return nil
		}
		if len(sessions) == 0 {
			m.chat.AddMessage(ChatMessage{Role: "system", Content: "No saved sessions."})
			return nil
		}
		var sb strings.Builder
		sb.WriteString("Saved sessions:\n")
		for _, s := range sessions {
			summary := s.Summary
			if len([]rune(summary)) > 60 {
				summary = string([]rune(summary)[:60]) + "..."
			}
			if summary == "" {
				summary = "(empty)"
			}
			sb.WriteString(fmt.Sprintf("  %s  %-20s  %s  %s\n", s.ID, s.Model, s.UpdatedAt.Format("2006-01-02 15:04"), summary))
		}
		sb.WriteString("\nResume with: cascade --session <id>")
		m.chat.AddMessage(ChatMessage{Role: "system", Content: sb.String()})

	case cmd == "/save":
		if m.app.Sessions == nil {
			m.chat.AddMessage(ChatMessage{Role: "error", Content: "Session persistence not available."})
			return nil
		}
		m.app.Agent.Session().NotifySave()
		m.chat.AddMessage(ChatMessage{Role: "system", Content: fmt.Sprintf("Session saved: %s", m.app.SessionID)})

	default:
		m.chat.AddMessage(ChatMessage{
			Role:    "error",
			Content: "Unknown command: " + cmd + ". Type /help for available commands.",
		})
	}
	return nil
}
