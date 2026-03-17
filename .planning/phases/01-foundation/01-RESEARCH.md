# Phase 1: Foundation - Research

**Researched:** 2026-03-17
**Domain:** Go CLI agent -- ADK Go LLM client, Bubble Tea v2 TUI, GCP ADC auth, tool system, permission engine
**Confidence:** HIGH (stack versions verified in STACK.md; architecture validated against OpenCode/Claude Code patterns; pitfalls catalogued from domain research)

## Summary

Phase 1 delivers a working conversational agent in the terminal: streaming LLM output, core file tools (read/write/edit/glob/grep/bash), GCP authentication via ADC, risk-based permission enforcement, and both interactive and one-shot CLI modes. This is a greenfield Go project with no existing code.

The critical technical challenges are: (1) Bubble Tea v2 streaming integration without deadlocking the event loop, solved via a ring buffer + 30fps render tick architecture; (2) ADK Go used as an LLM client behind a Provider interface rather than as the agent loop orchestrator, because the custom loop needs permission gating and TUI event emission that ADK's runner doesn't support; (3) GCP ADC token refresh handled transparently via `google.DefaultTokenSource()` with retry-on-401 middleware; (4) permission engine that auto-approves READ operations to avoid permission fatigue while still gating writes.

**Primary recommendation:** Build inside-out -- types and interfaces first, then config, then provider, then agent loop, then tools, then permission engine, then TUI. The agent loop must work identically in interactive and one-shot modes before TUI integration begins.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- Character-by-character token rendering with markdown post-processing
- Ring buffer for LLM token delivery, batch renders at 30fps tick
- Inline prompt with risk level badge: [READ] auto-approves silently, [DML] shows warning + y/N prompt, [DESTRUCTIVE] shows red warning requiring explicit 'yes'
- Shift+Tab cycles permission modes (CONFIRM / PLAN / BYPASS) with persistent mode badge in TUI header
- Primary command: `cascade`, alias: `csc`
- Default interactive mode (Bubble Tea TUI), `-p` flag for one-shot mode
- Support stdin piping as context
- Config loading order: defaults -> `~/.cascade/config.toml` -> `CASCADE.md` -> env vars -> CLI flags
- Single-threaded observe-reason-act cycle with hard limit of 15-20 tool calls per turn
- Loop governor: detect same tool called twice with same args, inject progress nudge after 5 tool calls with no user-facing output
- ADK Go used as LLM client (via `model.LLM` interface), NOT as agent loop orchestrator -- custom loop wraps ADK
- Provider interface from day one: ADK Go abstracted behind `Provider`
- Gemini 2.5 Pro as default, model configurable via config.toml
- Go module: `cmd/cascade/`, `internal/` packages
- TUI fully decoupled from agent logic via typed event channels
- Tools are interface-based, self-describing (name, description, JSON schema, risk level, execute method)
- LLM API failure: retry once with exponential backoff, then show error with retry prompt
- GCP auth failure: clear error with fix instructions
- Tool execution failure: feed error back to LLM for self-correction, max 2 retries per tool call
- Auth token expiry: use `google.DefaultTokenSource()` exclusively, retry-on-401 middleware

### Claude's Discretion
- Exact Bubble Tea component structure and message types
- Specific Lip Gloss styling choices (colors, borders, spacing)
- Config TOML section naming and key structure
- Internal error types and error wrapping patterns
- Test structure and test helper patterns

### Deferred Ideas (OUT OF SCOPE)
- BigQuery query tool and schema cache -- Phase 2
- Platform tools (Composer, Logging, GCS) -- Phase 3
- dbt integration -- Phase 4
- Skills/hooks/subagents -- Phase 5
- Setup wizard and Homebrew distribution -- Phase 6
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AGNT-01 | Multi-turn conversational sessions with streaming LLM output rendered in real-time | Ring buffer + render tick pattern; Bubble Tea v2 event-driven TUI; agent loop emits TokenEvent via channel |
| AGNT-02 | Single-threaded observe-reason-act loop with 15-20 tool call hard limit per turn | Agent loop pattern from ARCHITECTURE.md; loop governor with duplicate detection and progress nudge |
| AGNT-03 | Bubble Tea TUI with Lip Gloss styling, Glamour markdown rendering, syntax highlighting | Charm v2 stack (BT v2.0.2, LG v2.0.2, Glamour v2.0.0, Bubbles v2.0.0); chroma/v2 for syntax highlighting |
| AGNT-04 | ADK Go with Gemini 2.5 Pro as default, abstracted behind Provider interface | ADK Go v0.6.0 `model.LLM` interface; GenAI SDK v1.50.0 for types; Provider wraps ADK |
| AGNT-05 | Interactive mode (default) and one-shot mode via `-p` flag | Cobra for CLI; agent loop produces output via event interface; one-shot mode uses stdout writer instead of TUI |
| AGNT-07 | Core file tools: Read, Write, Edit, Glob, Grep, Bash | Tool interface with registry pattern; each tool self-describes schema and risk level; uses doublestar for glob, go-udiff for diffs |
| AUTH-01 | GCP ADC authentication with zero custom auth | `golang.org/x/oauth2/google.DefaultTokenSource()`; standard GCP client init pattern |
| AUTH-02 | Service account impersonation via config | `google.golang.org/api/impersonate` package; config.toml `[auth]` section |
| AUTH-03 | Permission engine classifies every tool call by risk level | Risk enum (READ_ONLY, DML, DDL, DESTRUCTIVE, ADMIN); per-tool annotation; middleware pattern |
| AUTH-04 | READ_ONLY auto-approve; DML+ require explicit confirmation | Permission engine checks tool.RiskLevel() against current mode; emits PermissionRequestEvent for TUI |
| AUTH-05 | Cycle permission modes: CONFIRM, PLAN, BYPASS via Shift+Tab | TUI key binding; permission mode stored in app state; mode badge in status bar |
| AUTH-07 | Auth token expiry handled transparently with retry-on-401 | `DefaultTokenSource` auto-refreshes; wrap GCP calls with retry middleware; test with long sessions |
| UX-01 | Global config via `~/.cascade/config.toml` | BurntSushi/toml v1.6.0; layered config merge: defaults -> file -> env -> flags |
| UX-04 | Keyboard shortcuts: Ctrl+C, Ctrl+B, Shift+Tab, Ctrl+R, Ctrl+L, Ctrl+D | Bubble Tea v2 key bindings; progressive keyboard enhancements; keys.go defines all bindings |
</phase_requirements>

## Standard Stack

### Core (Phase 1 Only)

| Library | Version | Import Path | Purpose | Why Standard |
|---------|---------|-------------|---------|--------------|
| Go | 1.26.1 | - | Runtime | Latest stable. Floor 1.25.8 driven by Charm v2 |
| ADK Go | v0.6.0 | `google.golang.org/adk` | LLM client (model.LLM interface) | Google's official agent SDK; provides typed function tools, streaming, Gemini-native |
| GenAI SDK | v1.50.0 | `google.golang.org/genai` | Underlying Gemini API types | ADK dependency; provides `genai.Content`, `genai.GenerateContentConfig` |
| Bubble Tea | v2.0.2 | `charm.land/bubbletea/v2` | TUI framework | 40.7K stars; Elm architecture; Cursed Renderer |
| Lip Gloss | v2.0.2 | `charm.land/lipgloss/v2` | Terminal styling | Pure v2 (no I/O fighting); must pair with BT v2 |
| Glamour | v2.0.0 | `charm.land/glamour/v2` | Markdown rendering | Renders LLM markdown in terminal |
| Bubbles | v2.0.0 | `charm.land/bubbles/v2` | TUI components (viewport, text input, spinner) | Standard components for BT v2 |
| Cobra | v1.10.2 | `github.com/spf13/cobra` | CLI framework | Industry standard; handles flags, completions |
| BurntSushi/toml | v1.6.0 | `github.com/BurntSushi/toml` | TOML config parsing | Zero deps, spec-compliant |

### Supporting (Phase 1 Only)

| Library | Version | Import Path | Purpose | When to Use |
|---------|---------|-------------|---------|-------------|
| `golang.org/x/oauth2` | v0.34.0 | `golang.org/x/oauth2/google` | GCP ADC auth | Token source for all GCP calls |
| `google.golang.org/api` | v0.271.0 | `google.golang.org/api/option` | GCP client options | `option.WithTokenSource()` for client init |
| `google.golang.org/api/impersonate` | v0.271.0 | `google.golang.org/api/impersonate` | SA impersonation | When `auth.impersonate_service_account` set |
| `github.com/google/uuid` | v1.6.0 | `github.com/google/uuid` | UUID generation | Session IDs, tool call IDs |
| `github.com/aymanbagabas/go-udiff` | v0.2.0 | `github.com/aymanbagabas/go-udiff` | Unified diff generation | Edit tool diff display |
| `github.com/bmatcuk/doublestar/v4` | v4.8.1 | `github.com/bmatcuk/doublestar/v4` | Glob pattern matching | Glob tool |
| `github.com/alecthomas/chroma/v2` | v2.15.0 | `github.com/alecthomas/chroma/v2` | Syntax highlighting | Code blocks in tool results |
| `github.com/google/go-cmp` | v0.7.0 | `github.com/google/go-cmp/cmp` | Test equality comparison | Used by ADK Go itself |

### Not Needed in Phase 1

| Library | Why Deferred |
|---------|-------------|
| `modernc.org/sqlite` | No schema cache or session persistence until Phase 2 |
| `cloud.google.com/go/bigquery` | BigQuery tools are Phase 2 |
| `cloud.google.com/go/storage` | GCS tools are Phase 3 |
| `cloud.google.com/go/logging` | Logging tools are Phase 3 |
| Anthropic SDK | Multi-provider is Phase 2+; Gemini only for Phase 1 |
| OpenAI SDK | Multi-provider is Phase 2+; Gemini only for Phase 1 |
| Huh | Forms/wizard not needed until Phase 6 setup wizard |
| `lithammer/fuzzysearch` | Schema search not until Phase 2 |
| MCP Go SDK | MCP integration is Phase 5 |

**Installation (Phase 1):**
```bash
# Initialize module
go mod init github.com/cascade-cli/cascade

# Core: ADK Go + GenAI
go get google.golang.org/adk@v0.6.0
go get google.golang.org/genai@v1.50.0

# TUI: Charm v2 stack (no Huh in Phase 1)
go get charm.land/bubbletea/v2@v2.0.2
go get charm.land/lipgloss/v2@v2.0.2
go get charm.land/glamour/v2@v2.0.0
go get charm.land/bubbles/v2@v2.0.0

# CLI & Config
go get github.com/spf13/cobra@v1.10.2
go get github.com/BurntSushi/toml@v1.6.0

# Auth
go get golang.org/x/oauth2@v0.34.0
go get google.golang.org/api@v0.271.0

# Supporting
go get github.com/google/uuid@v1.6.0
go get github.com/aymanbagabas/go-udiff@v0.2.0
go get github.com/bmatcuk/doublestar/v4@v4.8.1
go get github.com/alecthomas/chroma/v2@v2.15.0
go get github.com/google/go-cmp@v0.7.0
```

## Architecture Patterns

### Phase 1 Project Structure

```
cascade/
├── cmd/
│   └── cascade/
│       └── main.go              # Entry point, Cobra root cmd, wire-up
├── internal/
│   ├── app/
│   │   └── app.go               # Application struct, initialization, Run()
│   ├── agent/
│   │   ├── loop.go              # Core observe-reason-act loop
│   │   ├── governor.go          # Loop governor (limits, duplicate detection, nudges)
│   │   └── events.go            # AgentEvent types (Token, ToolStart, ToolEnd, PermissionRequest, Error, Done)
│   ├── provider/
│   │   ├── provider.go          # Provider interface definition
│   │   └── gemini/
│   │       └── gemini.go        # Gemini via ADK Go model.LLM
│   ├── tools/
│   │   ├── tool.go              # Tool interface definition
│   │   ├── registry.go          # Tool registry, lookup, schema generation
│   │   ├── result.go            # Tool result types
│   │   └── core/
│   │       ├── read.go          # Read file tool
│   │       ├── write.go         # Write file tool
│   │       ├── edit.go          # Edit file tool (string replacement)
│   │       ├── glob.go          # Glob pattern matching tool
│   │       ├── grep.go          # Content search tool
│   │       └── bash.go          # Shell command execution tool
│   ├── permission/
│   │   ├── engine.go            # Permission check middleware
│   │   ├── risk.go              # RiskLevel enum and classification
│   │   └── mode.go              # PermissionMode enum (CONFIRM, PLAN, BYPASS)
│   ├── config/
│   │   ├── config.go            # Config struct with defaults
│   │   └── loader.go            # TOML + env + flag loading, layer merge
│   ├── auth/
│   │   ├── gcp.go               # GCP ADC token source setup
│   │   └── retry.go             # Retry-on-401 middleware for token refresh
│   ├── tui/
│   │   ├── model.go             # Root Bubble Tea model
│   │   ├── chat.go              # Chat message list + viewport
│   │   ├── input.go             # Multi-line text input
│   │   ├── status.go            # Status bar (model, mode, session info)
│   │   ├── confirm.go           # Permission confirmation inline prompt
│   │   ├── spinner.go           # Tool execution spinner
│   │   ├── styles.go            # Lip Gloss style definitions
│   │   ├── keys.go              # Key bindings (Ctrl+C, Shift+Tab, etc.)
│   │   └── renderer.go          # Ring buffer + render tick for streaming tokens
│   └── oneshot/
│       └── runner.go            # One-shot mode: stdout output, no TUI
├── pkg/
│   └── types/
│       ├── message.go           # Chat message types (User, Assistant, ToolCall, ToolResult)
│       └── event.go             # Provider-agnostic event types
├── go.mod
├── go.sum
└── Makefile
```

### Pattern 1: ADK Go as LLM Client, Not Agent Orchestrator

**What:** Use ADK Go's `model.LLM` interface for Gemini API calls and `genai.*` types for message formatting, but implement the agent loop independently. ADK's `runner.Runner` is NOT used because it doesn't support: (a) custom permission gating between tool selection and execution, (b) TUI event emission during streaming, (c) the specific loop governor behavior needed.

**When to use:** Always. This is a locked decision.

**Implementation approach:**
```go
// internal/provider/provider.go
type Provider interface {
    // GenerateStream sends messages to the LLM and returns a streaming response.
    // The caller reads tokens from the stream and collects tool calls at the end.
    GenerateStream(ctx context.Context, messages []types.Message, tools []tools.Declaration) (*Stream, error)

    // Model returns the model identifier string.
    Model() string
}

// Stream wraps the LLM's streaming response.
type Stream struct {
    // Tokens returns a channel that emits text tokens as they arrive.
    Tokens() <-chan string
    // Result blocks until streaming is complete, returns the full response
    // including any tool calls.
    Result() (*types.Response, error)
    // Cancel aborts the stream.
    Cancel()
}
```

```go
// internal/provider/gemini/gemini.go
type GeminiProvider struct {
    llm model.LLM  // ADK Go's model.LLM interface
}

func New(ctx context.Context, apiKey string, modelName string) (*GeminiProvider, error) {
    // Use ADK's model package to create a Gemini LLM
    llm, err := gemini.New(ctx, modelName, nil)  // ADK model/gemini package
    if err != nil {
        return nil, err
    }
    return &GeminiProvider{llm: llm}, nil
}
```

**Key insight:** ADK Go's `model.LLM` interface has just two methods: `Name() string` and `GenerateContent(ctx, req, config)`. This thin interface is ideal for wrapping. The heavy lifting is converting between `genai.Content` types and Cascade's internal `types.Message`. This is ~200-300 LOC for the Gemini adapter.

### Pattern 2: Ring Buffer + Render Tick for Streaming

**What:** LLM tokens flow through a ring buffer (or buffered channel) from the streaming goroutine. The TUI reads from this buffer on a 33ms tick (30fps), batching all accumulated tokens into a single render cycle.

**When to use:** Always for streaming LLM output to Bubble Tea. This prevents the deadlock pitfall.

**Implementation approach:**
```go
// internal/tui/renderer.go
type StreamRenderer struct {
    buffer    chan string     // Buffered channel (ring buffer, cap 256)
    content   strings.Builder // Accumulated content
    dirty     bool           // Whether content changed since last render
}

// In the TUI model's Update():
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tickMsg:
        // Drain all available tokens from buffer, non-blocking
        drained := m.renderer.DrainAll()
        if drained > 0 {
            // Re-render markdown only when content changed
            m.chatView.SetContent(m.renderer.Rendered())
        }
        return m, m.tickCmd() // Schedule next tick
    case agentDoneMsg:
        // Final render with full markdown processing
        m.chatView.SetContent(m.renderer.FinalRender())
        return m, nil
    }
}

func (m Model) tickCmd() tea.Cmd {
    return tea.Tick(33*time.Millisecond, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}
```

**Critical:** The streaming goroutine must NEVER block on send. Use a select with default to drop tokens if the buffer is full (extremely unlikely at 256 capacity with 30fps drain, but prevents deadlock under pathological conditions).

### Pattern 3: Event-Driven Agent-TUI Decoupling

**What:** Agent loop and TUI communicate exclusively through typed event channels. Agent emits events; TUI renders them. TUI emits user actions back.

**Event types for Phase 1:**
```go
// internal/agent/events.go
type Event interface{ agentEvent() }

type TokenEvent struct{ Token string }            // Streaming text token
type ToolStartEvent struct {                       // Tool execution beginning
    Name  string
    Input json.RawMessage
}
type ToolEndEvent struct {                         // Tool execution complete
    Name   string
    Result *tools.Result
    Err    error
}
type PermissionRequestEvent struct {                // Need user permission
    Tool     tools.Tool
    Input    json.RawMessage
    Response chan<- bool                            // TUI sends approval back
}
type ErrorEvent struct{ Err error }                // Non-fatal error
type DoneEvent struct{}                            // Turn complete
type AssistantTextEvent struct{ Text string }      // Final assembled text (for one-shot)
```

**Key insight:** One-shot mode (`-p` flag) consumes the same event stream but writes to stdout instead of Bubble Tea. The agent loop is identical in both modes. `internal/oneshot/runner.go` is a simple event consumer that prints text and auto-denies DML+ permissions.

### Pattern 4: Permission Gate Middleware

**What:** Permission checking sits between tool selection and tool execution. Tool declares its risk level; permission engine decides allow/confirm/deny based on current mode.

```go
// internal/permission/engine.go
type Engine struct {
    mode    Mode                    // CONFIRM (default), PLAN, BYPASS
    cache   map[string]Decision     // Session cache: tool+args hash -> decision
}

func (e *Engine) Check(tool tools.Tool, input json.RawMessage) Decision {
    risk := tool.RiskLevel()
    switch e.mode {
    case ModePlan:
        if risk > RiskReadOnly { return Deny }
        return Allow
    case ModeBypass:
        return Allow
    case ModeConfirm:
        if risk <= RiskReadOnly { return Allow }  // Auto-approve reads
        // Check session cache
        key := cacheKey(tool.Name(), input)
        if d, ok := e.cache[key]; ok { return d }
        return Confirm  // Triggers PermissionRequestEvent
    }
    return Deny
}
```

**Risk levels for Phase 1 core tools:**

| Tool | Risk Level | Rationale |
|------|-----------|-----------|
| Read | READ_ONLY | File read, no side effects |
| Glob | READ_ONLY | File pattern match, no side effects |
| Grep | READ_ONLY | Content search, no side effects |
| Write | DML | Creates/overwrites files |
| Edit | DML | Modifies existing files |
| Bash | DESTRUCTIVE (default) | Arbitrary shell commands; risk varies by command |

**Bash tool special handling:** Parse the command string to classify risk dynamically. Read-only commands (`ls`, `cat`, `git status`, `git log`, `git diff`, `pwd`, `whoami`, `echo`, `head`, `tail`, `wc`, `find`, `which`) can be auto-approved. Commands containing `rm`, `mv`, `chmod`, `chown`, or writing redirections (`>`, `>>`) escalate to DESTRUCTIVE.

### Pattern 5: Layered Config Loading

**What:** Config loads from multiple sources in priority order, with later sources overriding earlier ones.

```go
// internal/config/config.go
type Config struct {
    Model    ModelConfig    `toml:"model"`
    Auth     AuthConfig     `toml:"auth"`
    Agent    AgentConfig    `toml:"agent"`
    Display  DisplayConfig  `toml:"display"`
    Security SecurityConfig `toml:"security"`
}

type ModelConfig struct {
    Provider string `toml:"provider"`   // "google" (default)
    Model    string `toml:"model"`      // "gemini-2.5-pro" (default)
}

type AuthConfig struct {
    ImpersonateServiceAccount string `toml:"impersonate_service_account"`
}

type AgentConfig struct {
    MaxToolCalls int `toml:"max_tool_calls"` // 15 default
    ToolTimeout  int `toml:"tool_timeout"`   // 120 seconds default
}
```

**Loading order:** Hardcoded defaults -> `~/.cascade/config.toml` -> `CASCADE.md` (in current dir) -> environment variables (`CASCADE_MODEL`, `CASCADE_PROJECT`, etc.) -> CLI flags (`--model`, `--project`).

### Anti-Patterns to Avoid

- **Coupling TUI to agent logic:** Never put tool execution or LLM calls inside Bubble Tea `Update()`. Agent runs on its own goroutine.
- **Blocking Bubble Tea event loop:** Never make synchronous HTTP calls in `Update()`. All I/O happens in `tea.Cmd` functions or separate goroutines.
- **Provider-specific types leaking:** `genai.Content` types must not appear outside `internal/provider/gemini/`. All other code uses `pkg/types/message.go`.
- **Hardcoding tool names in the agent loop:** Agent loop uses the tool registry interface. Adding a new tool never requires modifying the loop.
- **Monolithic agent loop:** Separate the loop (`loop.go`), governor (`governor.go`), and event emission (`events.go`). Each is independently testable.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CLI flag parsing | Custom arg parser | Cobra v1.10.2 | Shell completions, help generation, subcommand support for free |
| TOML config parsing | Custom parser | BurntSushi/toml v1.6.0 | Spec-compliant, handles all edge cases |
| Glob pattern matching | `filepath.Glob` | doublestar/v4 | `filepath.Glob` doesn't support `**` recursive matching |
| Diff generation | Custom diff algorithm | go-udiff v0.2.0 | Unified diff format is complex; edge cases with whitespace, encoding |
| Syntax highlighting | ANSI escape code construction | chroma/v2 | Language detection, 200+ lexers, terminal formatters |
| Markdown rendering in terminal | Custom ANSI markdown | Glamour v2.0.0 | Handles tables, code blocks, links, lists correctly with terminal width |
| UUID generation | Custom random IDs | google/uuid v1.6.0 | RFC 4122 compliant, collision-free |
| GCP token management | Custom token refresh | oauth2/google DefaultTokenSource | Handles all credential types, auto-refresh, impersonation chains |
| Terminal input handling | Raw terminal I/O | Bubbles text input | Handles cursor, selection, multi-line, Unicode correctly |

**Key insight:** Phase 1 has zero novel infrastructure problems. Every component has a well-tested library. The value is in assembling them correctly, not building from scratch.

## Common Pitfalls

### Pitfall 1: Bubble Tea Streaming Deadlock
**What goes wrong:** LLM streaming goroutine sends tokens faster than Bubble Tea's `Update()` can process them. Channel fills up, goroutine blocks, HTTP connection stalls, TUI freezes.
**Why it happens:** Bubble Tea's Elm architecture processes messages sequentially. Per-token messages overwhelm the runtime.
**How to avoid:** Buffered channel (cap 256) + non-blocking send (select with default) + 30fps tick drain in Update(). Never send individual token messages via `tea.Cmd`.
**Warning signs:** TUI freezes during long responses; Ctrl+C unresponsive; bursty text rendering.

### Pitfall 2: LLM Tool Call Parse Failures
**What goes wrong:** LLM generates malformed JSON, wrong tool names, invalid argument types. Hard errors crash the agent.
**Why it happens:** LLMs produce imperfect structured output 5-15% of the time, especially with complex schemas.
**How to avoid:** Lenient JSON parsing (strip code fences, handle trailing commas). Fuzzy tool name matching. Type coercion where safe. On parse failure, send error back to LLM with clear message, budget 2 retries before surfacing to user.
**Warning signs:** "invalid tool call" errors in normal usage.

### Pitfall 3: GCP Auth Token Expiry Mid-Session
**What goes wrong:** ADC tokens expire after 1 hour. Without retry-on-401, all GCP calls fail mid-session.
**Why it happens:** Development sessions are short; the 1-hour boundary is never hit during testing.
**How to avoid:** Use `google.DefaultTokenSource()` which auto-refreshes. Wrap GCP API calls with retry middleware: on 401, force token refresh, retry once. Show "Re-authenticating..." indicator.
**Warning signs:** GCP calls failing after 45+ minutes of session time.

### Pitfall 4: Permission Fatigue Leading to BYPASS Mode
**What goes wrong:** Users confirm too many READ operations, get frustrated, switch to BYPASS, then run unguarded writes.
**Why it happens:** Permission design prompts for all operations instead of auto-approving reads.
**How to avoid:** READ_ONLY auto-approves in CONFIRM mode (the locked decision already handles this). Only DML and above prompt. PLAN mode is truly non-blocking (read-only, zero confirmations).
**Warning signs:** Users immediately switching to BYPASS during first use.

### Pitfall 5: Agent Loop Infinite Cycling
**What goes wrong:** Ambiguous requests ("fix it") cause the agent to call tools endlessly without producing an answer.
**Why it happens:** No loop governor; LLM keeps requesting "more information."
**How to avoid:** Hard limit of 15-20 tool calls per turn. Duplicate detection (same tool + same args = stop). Progress nudge after 5 tool calls with no user-facing output. System prompt instructs LLM to ask for clarification when request is ambiguous.
**Warning signs:** Average tool calls per turn exceeding 8-10; same tool called repeatedly.

### Pitfall 6: ADK Go API Instability
**What goes wrong:** ADK Go is pre-1.0 (v0.6.0) with monthly breaking changes. Building directly on ADK types leaks instability throughout the codebase.
**Why it happens:** Temptation to use ADK types directly for convenience.
**How to avoid:** Pin to v0.6.0. Wrap behind Provider interface. `genai.*` types confined to `internal/provider/gemini/`. All other code uses `pkg/types/`. If ADK breaks, only `gemini.go` changes.
**Warning signs:** ADK Go import paths appearing outside the gemini package.

### Pitfall 7: Charm v2 Import Path Confusion
**What goes wrong:** Using v1 import paths (`github.com/charmbracelet/bubbletea`) instead of v2 (`charm.land/bubbletea/v2`). Mixing v1 and v2 causes compilation errors and runtime panics.
**Why it happens:** Most tutorials and examples online still reference v1 paths. Charm v2 released Feb 2026.
**How to avoid:** All Charm imports use `charm.land/*/v2`. Never mix. Verify in `go.mod` that no `github.com/charmbracelet/*` v1 packages appear.
**Warning signs:** Import resolution errors; Lip Gloss style functions not accepting expected types.

## Code Examples

### Agent Loop Core (internal/agent/loop.go)

```go
// Source: ARCHITECTURE.md pattern + CONTEXT.md decisions
func (a *Agent) RunTurn(ctx context.Context, userInput string) error {
    a.session.Append(types.UserMessage(userInput))
    toolCallCount := 0

    for {
        // Check loop governor
        if toolCallCount >= a.config.MaxToolCalls {
            a.emit(ErrorEvent{Err: fmt.Errorf("reached tool call limit (%d)", a.config.MaxToolCalls)})
            break
        }

        // Build messages for LLM
        messages := a.buildMessages()
        toolDecls := a.registry.Declarations()

        // Stream LLM response
        stream, err := a.provider.GenerateStream(ctx, messages, toolDecls)
        if err != nil {
            return a.handleLLMError(ctx, err) // Retry once with backoff
        }

        // Forward tokens to TUI via event channel
        go func() {
            for token := range stream.Tokens() {
                a.emit(TokenEvent{Token: token})
            }
        }()

        // Wait for complete response
        response, err := stream.Result()
        if err != nil {
            return a.handleLLMError(ctx, err)
        }

        a.session.Append(types.AssistantMessage(response))

        // No tool calls = turn complete
        if len(response.ToolCalls) == 0 {
            a.emit(DoneEvent{})
            return nil
        }

        // Execute each tool call
        for _, call := range response.ToolCalls {
            toolCallCount++

            // Governor: duplicate detection
            if a.governor.IsDuplicate(call) {
                a.session.Append(types.ToolResult(call.ID, "Duplicate tool call detected. Please try a different approach or ask the user for clarification."))
                continue
            }

            // Governor: progress nudge
            if a.governor.ShouldNudge(toolCallCount) {
                a.session.AppendSystem("You have made several tool calls. Provide a progress update to the user or ask for clarification.")
            }

            // Permission gate
            result := a.executeWithPermission(ctx, call)
            a.session.Append(types.ToolResult(call.ID, result))
        }
    }
    return nil
}
```

### Tool Interface (internal/tools/tool.go)

```go
// Source: ARCHITECTURE.md + TOOLS.md
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any        // JSON Schema
    RiskLevel() permission.RiskLevel
    Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

type Result struct {
    Content  string      // Text result for LLM context
    Display  string      // Formatted output for TUI (may include ANSI)
    IsError  bool        // Whether this is an error result
}

type Declaration struct {
    Name        string
    Description string
    Schema      map[string]any
}
```

### Bash Tool Risk Classification (internal/tools/core/bash.go)

```go
// Source: SECURITY.md bash command risk table
var readOnlyCommands = map[string]bool{
    "ls": true, "cat": true, "head": true, "tail": true,
    "wc": true, "find": true, "which": true, "pwd": true,
    "whoami": true, "echo": true, "env": true, "date": true,
    "git": true, // git subcommands further classified below
}

var readOnlyGitSubcommands = map[string]bool{
    "status": true, "log": true, "diff": true, "show": true,
    "branch": true, "remote": true, "tag": true, "rev-parse": true,
}

func classifyBashRisk(command string) permission.RiskLevel {
    parts := strings.Fields(command)
    if len(parts) == 0 {
        return permission.RiskDestructive
    }
    base := filepath.Base(parts[0])

    // Check for dangerous patterns anywhere in command
    if containsDangerousPattern(command) {
        return permission.RiskDestructive
    }

    if readOnlyCommands[base] {
        if base == "git" && len(parts) > 1 {
            if readOnlyGitSubcommands[parts[1]] {
                return permission.RiskReadOnly
            }
            return permission.RiskDML // git write operations (commit, push, etc.)
        }
        return permission.RiskReadOnly
    }

    return permission.RiskDestructive // Unknown commands default to destructive
}
```

### GCP Auth Setup (internal/auth/gcp.go)

```go
// Source: SECURITY.md + PITFALLS.md auth token expiry
func NewTokenSource(ctx context.Context, cfg *config.AuthConfig) (oauth2.TokenSource, error) {
    var ts oauth2.TokenSource

    if cfg.ImpersonateServiceAccount != "" {
        // Service account impersonation
        impTS, err := impersonate.CredentialsTokenSource(ctx,
            impersonate.CredentialsConfig{
                TargetPrincipal: cfg.ImpersonateServiceAccount,
                Scopes:          defaultScopes,
            })
        if err != nil {
            return nil, fmt.Errorf("failed to create impersonated credentials: %w\n"+
                "Ensure you have roles/iam.serviceAccountTokenCreator on %s",
                err, cfg.ImpersonateServiceAccount)
        }
        ts = impTS
    } else {
        // Standard ADC
        creds, err := google.FindDefaultCredentials(ctx, defaultScopes...)
        if err != nil {
            return nil, fmt.Errorf("GCP auth failed. Run: gcloud auth application-default login\n%w", err)
        }
        ts = creds.TokenSource
    }

    return oauth2.ReuseTokenSource(nil, ts), nil
}

var defaultScopes = []string{
    "https://www.googleapis.com/auth/cloud-platform",
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Charm v1 (`github.com/charmbracelet/*`) | Charm v2 (`charm.land/*/v2`) | Feb 2026 | New import paths, Cursed Renderer, declarative views. Must use v2 for new projects. |
| ADK Go v0.2 | ADK Go v0.6.0 | Mar 2026 | Stabilized `model.LLM` interface. Added tool confirmation, MCP integration. |
| `model/gemini` package | `model/gemini` stable | v0.2+ | Interface unchanged since v0.2, safe to depend on |
| Custom agent loops (OpenCode pattern) | ADK Go provides runner.Runner | v0.4+ | ADK runner exists but doesn't fit Cascade's permission/TUI needs -- use as LLM client only |
| GenAI SDK v0.x | GenAI SDK v1.50.0 | 2025-2026 | Stable v1 release; `genai.Content` types are the standard |

**Deprecated/outdated:**
- Charm v1 import paths (`github.com/charmbracelet/*`): Use `charm.land/*/v2` exclusively
- `mark3labs/mcp-go`: Superseded by official `modelcontextprotocol/go-sdk`
- `sashabaranov/go-openai`: Superseded by official `openai/openai-go` SDK

## Open Questions

1. **ADK Go v0.6.0 streaming API specifics**
   - What we know: `model.LLM` has `GenerateContent()` that returns streaming responses. GenAI SDK provides the underlying streaming mechanism.
   - What's unclear: Exact method signatures for streaming token-by-token in ADK Go v0.6.0. The adapter layer between ADK streaming and Cascade's token channel needs to be prototyped.
   - Recommendation: First implementation task should prototype the ADK Go -> Provider -> token channel pipeline. If ADK streaming is awkward, fall back to using GenAI SDK directly for the Gemini provider.

2. **Bubble Tea v2 Cursed Renderer behavior under rapid updates**
   - What we know: v2 uses ncurses-based rendering (Cursed Renderer) which is fundamentally different from v1's ANSI escape sequence approach.
   - What's unclear: Whether the Cursed Renderer handles frequent partial re-renders differently than v1. The ring buffer + tick architecture is designed for v1; v2 may have improved or changed the rendering model.
   - Recommendation: Build a minimal streaming test (BT v2 + goroutine feeding tokens at 100/second) before committing to the full TUI. Validate that the tick pattern works with the Cursed Renderer.

3. **Glamour v2 incremental markdown rendering**
   - What we know: Glamour renders complete markdown strings. During streaming, we have partial markdown that may have unclosed code blocks, incomplete tables, etc.
   - What's unclear: Whether Glamour v2 handles partial/malformed markdown gracefully or if it produces garbled output.
   - Recommendation: During streaming, render plain text with basic formatting. Apply full Glamour rendering only on turn completion (the `FinalRender()` call). This is simpler and avoids partial-markdown artifacts.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` stdlib + `go-cmp` v0.7.0 |
| Config file | None (Go convention: tests alongside source) |
| Quick run command | `go test ./internal/... -short -count=1` |
| Full suite command | `go test ./... -count=1 -race` |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AGNT-01 | Streaming tokens delivered to event channel | unit | `go test ./internal/agent/ -run TestStreamingTokenDelivery -count=1` | Wave 0 |
| AGNT-02 | Loop terminates at tool call limit; governor detects duplicates | unit | `go test ./internal/agent/ -run TestLoopGovernor -count=1` | Wave 0 |
| AGNT-03 | TUI renders markdown via Glamour; styles applied via Lip Gloss | integration | `go test ./internal/tui/ -run TestMarkdownRendering -count=1` | Wave 0 |
| AGNT-04 | Gemini provider implements Provider interface; streaming works | unit | `go test ./internal/provider/gemini/ -run TestGeminiProvider -count=1` | Wave 0 |
| AGNT-05 | One-shot mode produces stdout output and exits | integration | `go test ./internal/oneshot/ -run TestOneShotMode -count=1` | Wave 0 |
| AGNT-07 | Each core tool executes correctly (read/write/edit/glob/grep/bash) | unit | `go test ./internal/tools/core/ -run TestCoreTool -count=1` | Wave 0 |
| AUTH-01 | DefaultTokenSource succeeds with valid ADC | unit (mock) | `go test ./internal/auth/ -run TestADCTokenSource -count=1` | Wave 0 |
| AUTH-02 | Impersonation config creates impersonated token source | unit (mock) | `go test ./internal/auth/ -run TestImpersonation -count=1` | Wave 0 |
| AUTH-03 | Permission engine classifies tools by risk level | unit | `go test ./internal/permission/ -run TestRiskClassification -count=1` | Wave 0 |
| AUTH-04 | READ_ONLY auto-approved; DML requires confirmation | unit | `go test ./internal/permission/ -run TestPermissionDecision -count=1` | Wave 0 |
| AUTH-05 | Mode cycling changes permission behavior | unit | `go test ./internal/permission/ -run TestModeCycling -count=1` | Wave 0 |
| AUTH-07 | Retry middleware refreshes token on 401 | unit (mock) | `go test ./internal/auth/ -run TestRetryOn401 -count=1` | Wave 0 |
| UX-01 | Config loads from TOML with layer merging | unit | `go test ./internal/config/ -run TestConfigLoading -count=1` | Wave 0 |
| UX-04 | Key bindings registered and functional | unit | `go test ./internal/tui/ -run TestKeyBindings -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./internal/... -short -count=1`
- **Per wave merge:** `go test ./... -count=1 -race`
- **Phase gate:** Full suite green + manual smoke test (launch cascade, type question, see streaming response, execute a file read tool)

### Wave 0 Gaps
- [ ] All test files listed above -- greenfield project, nothing exists yet
- [ ] `internal/provider/gemini/gemini_test.go` -- mock LLM for unit tests
- [ ] `internal/agent/loop_test.go` -- mock provider + mock tool registry
- [ ] `internal/tools/core/*_test.go` -- test each tool against temp filesystem
- [ ] `internal/permission/engine_test.go` -- test all risk/mode combinations
- [ ] `internal/config/loader_test.go` -- test layered config merge
- [ ] `internal/auth/retry_test.go` -- mock HTTP transport returning 401 then 200
- [ ] `testutil/` package -- shared test helpers (temp dirs, mock provider, mock tool)

## Sources

### Primary (HIGH confidence)
- `.planning/research/STACK.md` -- All library versions verified via `go list -m`, `gh release list`, and GitHub API on 2026-03-16
- `.planning/research/ARCHITECTURE.md` -- Component boundaries, data flow, project structure patterns derived from OpenCode and Claude Code analysis
- `.planning/research/PITFALLS.md` -- 10 critical pitfalls catalogued with prevention strategies and recovery costs
- `specs/original/ARCHITECTURE.md` -- Agent loop design, LLM backend, platform context engine
- `specs/original/TOOLS.md` -- Tool system, core tools, risk classification
- `specs/original/SECURITY.md` -- Four-layer security model, permission engine, risk levels per operation type
- `specs/original/UX.md` -- Terminal UI elements, keyboard shortcuts, permission modes
- `specs/original/VISION.md` -- Design principles, command naming
- `specs/original/SPEC-REVIEW.md` -- Pre-build decisions: read-first strategy, deferred sandboxing, ADK Go commitment

### Secondary (MEDIUM confidence)
- [golang.org/x/oauth2/google package docs](https://pkg.go.dev/golang.org/x/oauth2/google) -- ADC token source, auto-refresh behavior
- [golang/oauth2 GitHub issues](https://github.com/golang/oauth2/issues/623) -- 401 retry patterns, expiry delta behavior

### Tertiary (LOW confidence)
- ADK Go v0.6.0 streaming API exact method signatures -- based on training data knowledge of v0.4, may have changed. Needs prototyping validation.
- Bubble Tea v2 Cursed Renderer behavior under rapid updates -- v2 released Feb 2026, limited community examples. Needs prototype validation.
- Glamour v2 partial markdown rendering -- not verified, conservative approach recommended (plain text during streaming, Glamour on completion).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all versions verified against npm registry equivalents and GitHub releases on 2026-03-16
- Architecture: HIGH -- patterns validated against multiple production Go AI agents (OpenCode, Claude Code concepts)
- Pitfalls: HIGH -- 10 pitfalls documented with specific prevention strategies; Phase 1-relevant pitfalls (streaming, auth, permissions, loop governor) well understood
- ADK Go streaming specifics: MEDIUM -- interface is known but exact v0.6.0 streaming patterns need prototyping
- Bubble Tea v2 specifics: MEDIUM -- architecture is Elm-based (well understood) but v2 Cursed Renderer is new

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (30 days -- stack is stable; ADK Go could release v0.7.0 in that window)
