# Architecture Research

**Domain:** AI-native CLI agent for GCP data engineering
**Researched:** 2026-03-16
**Confidence:** MEDIUM (based on training data knowledge of OpenCode, Claude Code, aider, and Google ADK Go -- web verification was unavailable)

## Standard Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                          TUI Layer (Bubble Tea)                     │
│  ┌────────────┐  ┌────────────┐  ┌───────────┐  ┌──────────────┐  │
│  │ Chat View  │  │ Status Bar │  │ Overlays  │  │ Input Editor │  │
│  └─────┬──────┘  └─────┬──────┘  └─────┬─────┘  └──────┬───────┘  │
│        │               │               │               │          │
├────────┴───────────────┴───────────────┴───────────────┴──────────┤
│                        Application Core                            │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                     Agent Loop (single-threaded)              │  │
│  │   observe → build prompt → LLM call → parse → execute tool   │  │
│  │        ↑                                          │          │  │
│  │        └──────────────────────────────────────────┘          │  │
│  └──────────────────────────────────────────────────────────────┘  │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐    │
│  │  Session Mgr  │  │ Permission   │  │  Context Builder      │    │
│  │  (history,    │  │ Engine       │  │  (system prompt,      │    │
│  │   compaction) │  │ (risk class, │  │   schema injection,   │    │
│  │              │  │  approval)   │  │   platform summary)   │    │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬────────────┘    │
├─────────┴────────────────┴──────────────────────┴─────────────────┤
│                        Tool System                                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐             │
│  │ Core     │ │ BigQuery │ │ Composer │ │ GCS      │             │
│  │ Tools    │ │ Tools    │ │ Tools    │ │ Tools    │             │
│  │(file,bash│ │(query,   │ │(dags,    │ │(ls,head, │             │
│  │ grep...) │ │ schema)  │ │ logs)    │ │ profile) │             │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘             │
│       │            │            │            │                    │
│  ┌────┴─────┐ ┌────┴─────┐ ┌────┴─────┐ ┌────┴─────┐             │
│  │ Logging  │ │ dbt      │ │ Cost     │ │ MCP      │             │
│  │ Tools    │ │ Tools    │ │ Tools    │ │ Client   │             │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬─────┘             │
├───────┴────────────┴────────────┴────────────┴────────────────────┤
│                     Provider / Infrastructure                      │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐    │
│  │ LLM Provider │  │ GCP Auth     │  │  Schema Cache         │    │
│  │ (Gemini,     │  │ (ADC, SA     │  │  (SQLite + FTS5)      │    │
│  │  Claude,     │  │  impersonat.)│  │                       │    │
│  │  OpenAI...)  │  │              │  │                       │    │
│  └──────────────┘  └──────────────┘  └───────────────────────┘    │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐    │
│  │ Config Mgr   │  │ Session DB   │  │  Skills / Hooks       │    │
│  │ (TOML +      │  │ (SQLite)     │  │  (markdown + scripts) │    │
│  │  CASCADE.md) │  │              │  │                       │    │
│  └──────────────┘  └──────────────┘  └───────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Typical Implementation |
|-----------|----------------|------------------------|
| **TUI Layer** | Render chat, stream tokens, handle input, display tool results | Bubble Tea model with messages; Lip Gloss for styling; Glamour for markdown |
| **Agent Loop** | Core observe-reason-act cycle; orchestrates LLM calls and tool execution | Single goroutine loop: build messages -> call LLM -> parse tool_use -> execute -> append result -> repeat until no more tool calls |
| **Session Manager** | Conversation history, context window tracking, compaction | SQLite-backed message store; token counting; summarization trigger |
| **Permission Engine** | Classify tool risk, gate execution, manage approval modes | Risk enum per tool (READ_ONLY through ADMIN); check against current mode; prompt user for confirmation |
| **Context Builder** | Assemble system prompt with schema, platform state, skills | Template system injecting schema summaries, recent failures, cost data, active skills |
| **Tool System** | Registry of callable tools; schema definition; execution | Interface-based registry; each tool defines name, description, input schema, risk level, execute function |
| **LLM Provider** | Abstract LLM communication; streaming; function calling format | Provider interface with Gemini (via ADK), Claude, OpenAI implementations; handles tool_use format differences |
| **Schema Cache** | Warehouse metadata for context injection and validation | SQLite with INFORMATION_SCHEMA data; FTS5 for search; dataset-scoped refresh |
| **Config Manager** | Layered config: defaults -> global -> project -> env -> flags | TOML parser; CASCADE.md reader; merge logic |
| **Skills / Hooks** | Domain-specific knowledge injection; lifecycle event scripts | Markdown file loader (skills); script runner with lifecycle events (hooks) |

## Recommended Project Structure

```
cascade/
├── cmd/
│   └── cascade/
│       └── main.go              # Entry point, CLI flag parsing, wire-up
├── internal/
│   ├── app/
│   │   ├── app.go               # Application struct, initialization, Run()
│   │   └── setup.go             # Setup wizard, first-run detection
│   ├── agent/
│   │   ├── loop.go              # Core agent loop (observe → reason → act)
│   │   ├── context.go           # Context builder (system prompt assembly)
│   │   ├── session.go           # Session management, compaction
│   │   └── subagent.go          # Fire-and-forget subagent spawner
│   ├── provider/
│   │   ├── provider.go          # Provider interface definition
│   │   ├── gemini/
│   │   │   └── gemini.go        # Gemini via ADK Go
│   │   ├── claude/
│   │   │   └── claude.go        # Anthropic API
│   │   ├── openai/
│   │   │   └── openai.go        # OpenAI-compatible API
│   │   └── ollama/
│   │       └── ollama.go        # Local Ollama
│   ├── tools/
│   │   ├── registry.go          # Tool registry, lookup, schema generation
│   │   ├── tool.go              # Tool interface definition
│   │   ├── core/                # File tools: read, write, edit, glob, grep, bash
│   │   │   ├── read.go
│   │   │   ├── write.go
│   │   │   ├── edit.go
│   │   │   ├── glob.go
│   │   │   ├── grep.go
│   │   │   └── bash.go
│   │   ├── bigquery/            # BQ tools: query, describe, list, profile, cost
│   │   │   ├── query.go
│   │   │   ├── schema.go
│   │   │   ├── cost.go
│   │   │   └── profile.go
│   │   ├── composer/            # Airflow tools: dags, status, logs, failures
│   │   │   ├── dags.go
│   │   │   ├── logs.go
│   │   │   └── failures.go
│   │   ├── logging/             # Cloud Logging query tool
│   │   │   └── logs.go
│   │   ├── gcs/                 # GCS tools: ls, head, profile
│   │   │   ├── list.go
│   │   │   ├── head.go
│   │   │   └── profile.go
│   │   ├── dbt/                 # dbt tools: run, test, manifest, lineage
│   │   │   ├── run.go
│   │   │   ├── manifest.go
│   │   │   └── lineage.go
│   │   └── mcp/                 # MCP client for external tool servers
│   │       └── client.go
│   ├── permission/
│   │   ├── engine.go            # Permission check, risk classification
│   │   ├── risk.go              # Risk level enum and per-tool mapping
│   │   └── pii.go               # PII detection logic
│   ├── schema/
│   │   ├── cache.go             # SQLite schema cache manager
│   │   ├── sync.go              # INFORMATION_SCHEMA sync logic
│   │   ├── search.go            # FTS5 search over schema
│   │   └── inject.go            # Schema context injection (format for LLM)
│   ├── config/
│   │   ├── config.go            # Config struct, defaults
│   │   ├── loader.go            # TOML + CASCADE.md loading, layer merge
│   │   └── validate.go          # Config validation
│   ├── skills/
│   │   ├── loader.go            # Markdown skill file discovery and parsing
│   │   └── matcher.go           # Auto-activation based on context
│   ├── hooks/
│   │   ├── runner.go            # Lifecycle hook execution
│   │   └── events.go            # Event type definitions
│   ├── cost/
│   │   ├── tracker.go           # Session cost tracking
│   │   ├── budget.go            # Budget enforcement
│   │   └── estimator.go         # Dry-run cost estimation
│   └── tui/
│       ├── model.go             # Root Bubble Tea model
│       ├── chat.go              # Chat message list component
│       ├── input.go             # Multi-line input editor
│       ├── status.go            # Status bar (model, cost, mode)
│       ├── confirm.go           # Permission confirmation overlay
│       ├── spinner.go           # Tool execution spinner
│       ├── table.go             # Query result table renderer
│       ├── styles.go            # Lip Gloss style definitions
│       ├── keys.go              # Key bindings
│       └── commands.go          # Slash command router
├── pkg/
│   └── types/                   # Shared types used across packages
│       ├── message.go           # Chat message types
│       ├── tool.go              # Tool call/result types
│       └── provider.go          # Provider-agnostic LLM types
├── skills/                      # Built-in skill markdown files
│   ├── bigquery-best-practices.md
│   ├── cost-optimization.md
│   └── pipeline-debugging.md
├── CASCADE.md                   # Example project config
├── go.mod
├── go.sum
└── Makefile
```

### Structure Rationale

- **cmd/cascade/**: Standard Go project layout. Single binary entry point. CLI flag parsing with `cobra` or raw `flag` -- for a single-command agent, raw `flag` + `pflag` is sufficient; cobra adds unnecessary complexity.
- **internal/**: All business logic is internal (unexported). No `pkg/` for business logic -- only shared types in `pkg/types/` that might be useful for MCP server implementations later.
- **internal/agent/**: The brain. Separated from TUI because the agent loop must work in both interactive and one-shot (`-p`) modes. The loop knows nothing about rendering.
- **internal/tools/**: Each tool domain gets its own subpackage. Tools are self-registering via `init()` or explicit registration in `app.go`. The registry generates JSON schemas for LLM function calling.
- **internal/tui/**: Pure Bubble Tea. Receives messages from the agent (via channels or Bubble Tea commands), renders them. Never calls tools directly.
- **internal/provider/**: Each LLM gets its own subpackage because tool_use format, streaming protocol, and error handling differ significantly between providers.
- **internal/schema/**: Isolated because schema operations are complex (sync, cache, search, inject) and used by multiple consumers (context builder, tools, PII detection).

## Architectural Patterns

### Pattern 1: Single-Threaded Agent Loop

**What:** The core agent loop runs on a single goroutine, executing one observe-reason-act cycle at a time. The LLM returns zero or more tool calls; each is executed sequentially; results are appended to context; the loop continues until the LLM returns a text-only response (no tool calls).

**When to use:** Always. This is how Claude Code, OpenCode, and aider all work. Multi-threaded agent loops create non-deterministic behavior and make debugging nearly impossible.

**Trade-offs:**
- Pro: Deterministic, debuggable, simple state management
- Pro: Natural backpressure -- one tool at a time means permission checks are sequential
- Con: Slower for independent operations (mitigated by LLM often batching related calls)
- Con: A long tool execution blocks the entire loop (mitigated by timeout + cancellation)

**Example:**
```go
// internal/agent/loop.go
func (a *Agent) Run(ctx context.Context, input string) error {
    a.session.Append(message.User(input))

    for {
        // 1. Build prompt with context injection
        messages := a.contextBuilder.Build(a.session)

        // 2. Call LLM (streaming)
        response, err := a.provider.Complete(ctx, messages)
        if err != nil {
            return a.handleError(err)
        }

        // 3. Append assistant response
        a.session.Append(response)

        // 4. If no tool calls, we're done
        if len(response.ToolCalls) == 0 {
            return nil
        }

        // 5. Execute each tool call
        for _, call := range response.ToolCalls {
            result := a.executeTool(ctx, call)
            a.session.Append(message.ToolResult(call.ID, result))
        }

        // 6. Loop back to step 1 (LLM sees tool results)
    }
}
```

### Pattern 2: Interface-Based Tool Registry

**What:** Tools implement a common interface and register themselves with a central registry. The registry generates LLM-compatible function declarations and dispatches calls by name. Each tool declares its own risk level, input schema, and execution logic.

**When to use:** Always for agent systems with extensible tool sets. This is the universal pattern across Claude Code, OpenCode, and aider.

**Trade-offs:**
- Pro: Adding new tools requires only implementing the interface -- no changes to agent loop or LLM code
- Pro: Tool schemas are co-located with tool logic (single source of truth)
- Pro: Risk levels attached to tools enable automatic permission gating
- Con: Interface indirection adds a thin layer of complexity
- Con: Schema generation from Go structs requires either code generation or reflection

**Example:**
```go
// internal/tools/tool.go
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any       // JSON Schema for LLM function calling
    RiskLevel() permission.RiskLevel   // READ_ONLY, DML, DDL, DESTRUCTIVE, ADMIN
    Execute(ctx context.Context, input json.RawMessage) (*Result, error)
}

type Result struct {
    Content  string   // Text result for LLM context
    Data     any      // Structured data for TUI rendering (tables, etc.)
    Metadata Metadata // Execution time, bytes scanned, cost, etc.
}

// internal/tools/registry.go
type Registry struct {
    tools map[string]Tool
}

func (r *Registry) Register(t Tool) { r.tools[t.Name()] = t }

func (r *Registry) FunctionDeclarations() []FunctionDecl {
    // Generate LLM-compatible tool declarations from all registered tools
}

func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (*Result, error) {
    tool, ok := r.tools[name]
    if !ok {
        return nil, fmt.Errorf("unknown tool: %s", name)
    }
    return tool.Execute(ctx, input)
}
```

### Pattern 3: Layered Context Assembly

**What:** The system prompt is not static -- it is assembled dynamically from layers: base instructions, schema context (injected from cache), platform summary (recent failures, cost), active skills (auto-matched markdown), session-specific context (CASCADE.md), and compacted history. Each layer has a token budget.

**When to use:** Any agent that needs domain awareness beyond what fits in a static system prompt. This is how Claude Code handles CLAUDE.md, and how Cascade should handle schema + platform state.

**Trade-offs:**
- Pro: LLM always has relevant context without manual user effort
- Pro: Token budget per layer prevents any single context from dominating
- Con: Token counting adds latency (use tiktoken-go or approximate)
- Con: Over-injection makes the system prompt noisy -- need good relevance filtering

**Example:**
```go
// internal/agent/context.go
type ContextBuilder struct {
    schemaCache   *schema.Cache
    skillLoader   *skills.Loader
    configLoader  *config.Loader
    costTracker   *cost.Tracker
}

func (cb *ContextBuilder) Build(session *Session) []Message {
    system := cb.basePrompt()

    // Layer 1: Project config (CASCADE.md)
    system += cb.projectConfig()

    // Layer 2: Schema context (relevant tables for current conversation)
    system += cb.schemaContext(session.RecentTopics())

    // Layer 3: Platform summary (failures, alerts, cost)
    system += cb.platformSummary()

    // Layer 4: Active skills (auto-matched)
    system += cb.activeSkills(session.RecentTopics())

    // Layer 5: Session history (possibly compacted)
    messages := session.Messages()
    if session.TokenCount() > cb.compactionThreshold {
        messages = cb.compact(messages)
    }

    return append([]Message{message.System(system)}, messages...)
}
```

### Pattern 4: Event-Driven TUI Decoupled from Agent

**What:** The TUI (Bubble Tea) and the agent loop communicate through channels/messages, not direct function calls. The agent emits events (token streamed, tool started, tool completed, permission needed, error) and the TUI subscribes to render them. The TUI emits events (user input, cancel, permission granted) back to the agent.

**When to use:** Always for interactive agents. This is the Bubble Tea way (Elm architecture) and the pattern OpenCode uses.

**Trade-offs:**
- Pro: Agent works identically in interactive mode and one-shot mode (just different event consumer)
- Pro: TUI can be tested independently with mock event streams
- Pro: Clean separation enables future GUI or web frontends
- Con: Adds message type definitions and channel plumbing
- Con: Streaming token rendering requires careful batching to avoid TUI flicker

**Example:**
```go
// Events from agent to TUI
type AgentEvent interface{ agentEvent() }
type TokenEvent struct { Token string }
type ToolStartEvent struct { Name string; Input json.RawMessage }
type ToolEndEvent struct { Name string; Result *tools.Result }
type PermissionRequestEvent struct { Tool tools.Tool; Input json.RawMessage; Respond chan bool }
type ErrorEvent struct { Err error }
type DoneEvent struct{}

// TUI listens on a channel
func (m Model) listenForAgentEvents() tea.Cmd {
    return func() tea.Msg {
        event := <-m.agentEvents
        return event // Bubble Tea dispatches to Update()
    }
}
```

### Pattern 5: Permission Gate as Middleware

**What:** Permission checking sits between the agent loop and tool execution as middleware. When a tool call arrives, the permission engine checks the tool's risk level against the current mode (CONFIRM, PLAN, BYPASS). If confirmation is needed, the agent loop blocks and emits a permission request event; the TUI shows a confirmation dialog; the user's response unblocks execution.

**When to use:** Any agent with tools that have side effects. This is how Claude Code gates dangerous operations.

**Trade-offs:**
- Pro: Tools don't need to know about permissions -- clean separation
- Pro: Permission logic is centralized and auditable
- Pro: Modes (PLAN/CONFIRM/BYPASS) are trivially switchable at runtime
- Con: Blocking on user input complicates the agent loop flow
- Con: Risk classification requires manual per-tool annotation (but this is desirable)

**Example:**
```go
// internal/agent/loop.go (inside executeTool)
func (a *Agent) executeTool(ctx context.Context, call ToolCall) *tools.Result {
    tool := a.registry.Get(call.Name)

    // Permission gate
    decision := a.permission.Check(tool, call.Input)
    switch decision {
    case permission.Allow:
        // proceed
    case permission.Confirm:
        approved := a.requestPermission(tool, call.Input) // blocks, emits event
        if !approved {
            return tools.Denied(call.Name)
        }
    case permission.Deny:
        return tools.Denied(call.Name)
    }

    // Cost gate (for BigQuery)
    if estimator, ok := tool.(tools.CostEstimatable); ok {
        estimate := estimator.EstimateCost(call.Input)
        if !a.costTracker.WithinBudget(estimate) {
            return tools.BudgetExceeded(call.Name, estimate)
        }
    }

    // Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, a.toolTimeout)
    defer cancel()
    result, err := tool.Execute(ctx, call.Input)
    if err != nil {
        return tools.Error(call.Name, err)
    }

    // Track cost
    a.costTracker.Record(result.Metadata)

    // Run post-execution hooks
    a.hooks.Run(hooks.PostToolUse, tool.Name(), call.Input, result)

    return result
}
```

### Pattern 6: Schema Cache with Relevance Filtering

**What:** The full warehouse schema (potentially 10K+ tables) is stored in SQLite with FTS5, but only a relevant subset is injected into the LLM context. Relevance is determined by: (1) tables explicitly mentioned in conversation, (2) FTS5 search against user's recent messages, (3) tables in the same dataset as recently queried tables, (4) tables with foreign key or naming relationships. A token budget caps injection size.

**When to use:** Any data-aware agent where the full schema exceeds context limits.

**Trade-offs:**
- Pro: LLM gets relevant schema without manual user selection
- Pro: FTS5 search is sub-millisecond even for large schemas
- Pro: SQLite is embedded, no external dependency
- Con: Relevance heuristics may miss important tables (mitigated by explicit /sync command)
- Con: Schema staleness if underlying warehouse changes (mitigated by TTL-based refresh)

## Data Flow

### Core Agent Flow (Interactive Mode)

```
[User types message in TUI]
    ↓ tea.Msg
[TUI Input Component] → [Agent Loop (via channel)]
    ↓
[Context Builder]
    ├── reads: Session history
    ├── reads: Schema cache (SQLite FTS5 query)
    ├── reads: Platform summary (cached GCP state)
    ├── reads: Skills (matched markdown files)
    └── reads: CASCADE.md config
    ↓ assembled []Message
[LLM Provider] ← streaming connection → [Gemini / Claude / OpenAI]
    ↓ streamed tokens + tool_calls
[Agent Loop]
    ├── streams tokens → TUI (via event channel)
    ├── if tool_call:
    │   ├── [Permission Engine] → check risk level → maybe ask user
    │   ├── [Cost Estimator] → dry-run check → maybe block
    │   ├── [Tool.Execute()] → GCP API / local filesystem / subprocess
    │   ├── [Hooks Runner] → PostToolUse lifecycle scripts
    │   ├── [Cost Tracker] → record bytes scanned, API cost
    │   └── append ToolResult → session
    └── if text only: done, wait for next user input
    ↓ loop back to Context Builder if more tool calls
[TUI] ← renders streamed output, tool results, tables, status
```

### Schema Cache Flow

```
[Setup Wizard / /sync command / TTL expiry]
    ↓
[Schema Sync]
    ├── BigQuery: SELECT * FROM `project.dataset.INFORMATION_SCHEMA.COLUMNS`
    ├── BigQuery: SELECT * FROM `project.dataset.INFORMATION_SCHEMA.TABLE_OPTIONS`
    ├── BigQuery: SELECT * FROM `project.dataset.INFORMATION_SCHEMA.TABLES` (row count, size)
    └── Optional: Dataplex tags for PII annotations
    ↓ bulk results
[Schema Cache Manager]
    ├── UPSERT into SQLite tables (columns, tables, datasets)
    ├── Rebuild FTS5 index
    └── Update sync metadata (last_synced, dataset, row_count)
    ↓
[Schema Search / Inject]
    ├── FTS5 query: match user's topic against table/column names and descriptions
    ├── Token budget: limit to N tables that fit in context budget
    └── Format: structured text block for system prompt injection
```

### Permission Flow

```
[Tool Call from LLM]
    ↓
[Permission Engine]
    ├── Lookup: tool.RiskLevel()
    │   ├── READ_ONLY: BigQuery dry-run, schema list, GCS ls, log read
    │   ├── DML: BigQuery query (non-SELECT detected), dbt run
    │   ├── DDL: CREATE/ALTER/DROP table
    │   ├── DESTRUCTIVE: DELETE, TRUNCATE, DROP dataset
    │   └── ADMIN: IAM changes, service account operations
    ├── Check: current mode
    │   ├── PLAN mode: allow READ_ONLY only, deny all else
    │   ├── CONFIRM mode: allow READ_ONLY silently, prompt for DML+
    │   └── BYPASS mode: allow all (with logging)
    └── If needs confirmation:
        ├── Emit PermissionRequestEvent to TUI
        ├── TUI shows: tool name, input summary, risk level, estimated cost
        ├── User: approve / deny / approve-session (allow this tool for rest of session)
        └── Return decision to agent loop
```

### One-Shot Mode Flow

```
[cascade -p "What tables have PII columns?"]
    ↓
[CLI Parser] → skip TUI initialization
    ↓
[Agent Loop] (same as interactive, but:)
    ├── Output: stdout (no TUI rendering)
    ├── Permission: auto-deny anything above READ_ONLY (or use --yes flag)
    ├── Format: --format json/csv/markdown/quiet
    └── Exit: after first complete response (no follow-up loop)
```

### Key Data Flows

1. **User message to LLM response:** User input -> session append -> context assembly (with schema/skills injection) -> LLM streaming call -> token-by-token TUI rendering -> tool call extraction -> permission check -> tool execution -> result append -> loop or done.

2. **Schema to context:** INFORMATION_SCHEMA bulk query -> SQLite upsert -> FTS5 index rebuild -> (on each turn) FTS5 relevance query -> token-budgeted formatting -> system prompt injection.

3. **Cross-service debugging:** User asks "why did pipeline X fail?" -> Agent calls ComposerFailures tool -> identifies failed DAG run -> calls ComposerTaskLogs for error details -> calls CloudLogging for surrounding errors -> calls BigQuery to check destination table staleness -> synthesizes timeline.

4. **Cost tracking:** BigQuery dry-run before every query -> bytes_scanned estimate -> cost calculation -> budget check -> if approved, execute -> actual bytes from job metadata -> session cost accumulator -> status bar update.

## Scaling Considerations

| Scale | Architecture Adjustments |
|-------|--------------------------|
| Single user, small warehouse (<100 tables) | Default architecture works perfectly. Full schema fits in context. No caching complexity needed beyond basic SQLite. |
| Single user, large warehouse (1K-10K tables) | Schema relevance filtering becomes critical. Dataset-scoped sync (not full warehouse). FTS5 search quality matters. Token budgets per context layer. |
| Single user, massive warehouse (10K+ tables) | Schema cache sync needs incremental updates (diff against last sync). Consider column-level relevance (don't inject all columns of matched tables). May need schema summarization (LLM-generated table descriptions cached). |
| Team usage (multiple users, shared config) | CASCADE.md per-project. Skills as shared repo. No server component needed -- each user runs their own binary with shared config files via git. |

### Scaling Priorities

1. **First bottleneck: Context window exhaustion.** Long debugging sessions fill the context. Solution: aggressive compaction (summarize older turns), token counting per message, automatic compaction trigger at 70% of context limit.

2. **Second bottleneck: Schema cache relevance.** Large warehouses mean FTS5 returns too many matches. Solution: dataset pinning in config (user specifies which 2-3 datasets they work with), recency weighting (recently queried tables rank higher), column description quality.

3. **Third bottleneck: LLM latency.** Complex queries with many tool calls take time. Solution: subagents for parallel investigation (fire-and-forget goroutines for log analysis while main agent continues), result caching for repeated schema lookups within a session.

## Anti-Patterns

### Anti-Pattern 1: Coupling TUI to Agent Logic

**What people do:** Put tool execution, LLM calls, or permission logic inside Bubble Tea Update() or View() functions.
**Why it's wrong:** Makes agent untestable without TUI. Breaks one-shot mode. Bubble Tea's Update() should only handle UI state transitions -- it runs on the main thread and blocking it freezes the entire terminal.
**Do this instead:** Agent loop runs on its own goroutine, communicates with TUI via typed event channels. TUI is a thin rendering layer.

### Anti-Pattern 2: Monolithic System Prompt

**What people do:** Write one giant system prompt string with all instructions, schema, examples, and context baked in.
**Why it's wrong:** Wastes tokens on irrelevant context. Cannot adapt to user's current task. Schema becomes stale. No way to manage token budget.
**Do this instead:** Layered context assembly with token budgets per layer. Relevance-filtered schema injection. Skills auto-activated by topic matching.

### Anti-Pattern 3: Embedding Risk Levels in the Agent Loop

**What people do:** Scatter if/else permission checks throughout the agent loop based on tool names.
**Why it's wrong:** Adding a new tool requires modifying the agent loop. Risk classification is not auditable. Permission modes become impossible to implement cleanly.
**Do this instead:** Each tool declares its own risk level. Permission engine is middleware that intercepts all tool calls. Agent loop is risk-unaware.

### Anti-Pattern 4: Synchronous Schema Sync on Startup

**What people do:** Block application startup until the full schema cache is synced from BigQuery.
**Why it's wrong:** INFORMATION_SCHEMA queries on large warehouses take 10-30 seconds. User stares at blank screen. Startup should be instant.
**Do this instead:** Start with stale cache (if available). Sync in background goroutine. Show sync progress in status bar. Schema cache has TTL -- refresh only when stale or user requests /sync.

### Anti-Pattern 5: Provider-Specific Code in Tools

**What people do:** Write tool execution code that assumes a specific LLM's tool_use format (e.g., Gemini's FunctionCall structure).
**Why it's wrong:** Locks tools to one provider. The whole point of model-agnostic design is that tools work identically regardless of which LLM selected them.
**Do this instead:** Tools receive provider-agnostic `json.RawMessage` input and return provider-agnostic `Result`. The provider layer handles format translation.

### Anti-Pattern 6: Direct GCP Client Construction in Tools

**What people do:** Each tool creates its own BigQuery/Composer/GCS/Logging client with `bigquery.NewClient(ctx, projectID)`.
**Why it's wrong:** Duplicated auth setup. Cannot share connection pools. Makes testing impossible (no way to inject mocks). Service account impersonation logic duplicated everywhere.
**Do this instead:** Dependency injection. A `GCPClients` struct is created once during app initialization (with proper auth, project ID, impersonation) and passed to tools that need it. Tools accept interfaces, not concrete clients.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| BigQuery | Go client library (`cloud.google.com/go/bigquery`) | Use Jobs API for query execution (not legacy). Dry-run via `QueryConfig.DryRun = true`. INFORMATION_SCHEMA via standard query. |
| Cloud Composer | REST API via `google.golang.org/api/composer/v1`) | Airflow REST API for DAG operations (Composer 2 exposes it). Auth via Composer environment's web server URL + IAM. |
| Cloud Logging | Go client library (`cloud.google.com/go/logging/logadmin`) | Use `Entries()` with filters. Beware: large log reads are slow -- always scope with time range + resource filter. |
| GCS | Go client library (`cloud.google.com/go/storage`) | Read-only for V1. `head` = read first N bytes. Profile = sample rows from CSV/JSON/Parquet. |
| Dataplex | REST API (`google.golang.org/api/dataplex/v1`) | For PII tag retrieval. Optional -- gracefully degrade if Dataplex not configured. |
| dbt | CLI subprocess (`exec.Command("dbt", args...)`) | Parse manifest.json for lineage. Run/test via subprocess. No Go SDK for dbt. |
| Gemini (via ADK Go) | Google ADK Go SDK | Primary LLM. Handles function calling natively. Streaming via ADK's stream API. |
| Claude | Anthropic Go SDK or HTTP API | Tool use via messages API. Different tool_use format from Gemini -- provider layer translates. |
| MCP Servers | JSON-RPC over stdio | Spawn MCP server as subprocess, communicate via stdin/stdout JSON-RPC. Discover tools, call them as if native. |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| TUI <-> Agent | Typed Go channels (event stream) | Agent emits events, TUI renders. TUI emits user input + permission responses. Never direct function calls. |
| Agent <-> Tools | Interface dispatch via Registry | Agent calls `registry.Execute(name, input)`. Tools return `Result`. No back-channel from tool to agent. |
| Agent <-> Provider | Provider interface (streaming) | Agent sends `[]Message`, receives streaming response. Provider handles format translation. Agent never sees Gemini/Claude-specific types. |
| Tools <-> GCP | Go client libraries (injected) | Tools receive pre-configured GCP clients. Never construct clients internally. |
| Schema Cache <-> Tools | Read-only access | Tools (especially BigQuery) can query schema cache for context. Schema sync is a separate concern (triggered by agent, not tools). |
| Config <-> Everything | Read-only after init | Config loaded once at startup, passed to components that need it. No runtime config mutation except permission mode toggle. |
| Hooks <-> Agent | Event-driven, fire-and-forget | Agent emits lifecycle events. Hook runner executes matching scripts. Hook results are logged but don't block agent loop (except PreToolUse which can deny). |

## Build Order (Dependency Graph)

The components have clear dependency relationships that dictate build order:

```
Phase 1 (Foundation):
  Config → Provider Interface → Agent Loop (minimal) → TUI (basic chat)

Phase 2 (Core Tools):
  Tool Interface + Registry → Core File Tools (read, write, bash)
  Permission Engine (basic CONFIRM/PLAN/BYPASS)

Phase 3 (GCP Read Path):
  GCP Auth (ADC) → BigQuery Client → Schema Cache (SQLite + FTS5)
  Schema Sync → Schema Search → Context Injection
  BigQuery Query Tool (with dry-run) → BigQuery Schema Tools

Phase 4 (Platform Awareness):
  Composer Tools → Cloud Logging Tools → GCS Tools
  Cross-service debugging (orchestrated by LLM, not hard-coded)
  Platform Summary injection

Phase 5 (Advanced):
  dbt Integration → Cost Tracking + Budgets → PII Detection
  Skills System → Hooks System → Session Compaction
  Subagents → MCP Client

Phase 6 (Polish):
  Setup Wizard → One-shot mode → Output formats
  Degraded/offline mode → Error retry + model fallback
```

**Rationale:** Each phase produces a usable product. Phase 1 gives you a working chat agent. Phase 2 makes it useful for file operations. Phase 3 makes it a BigQuery agent. Phase 4 makes it a platform agent. Phase 5 adds power features. Phase 6 polishes for distribution.

**Critical dependency:** Schema Cache (Phase 3) must exist before Context Injection works. Without it, the agent has no warehouse awareness and is just a generic chatbot.

**Critical dependency:** Permission Engine (Phase 2) must exist before any write/mutate tools are added. Never ship a tool that can modify data without permission gating.

## Sources

- OpenCode (github.com/opencode-ai/opencode): Go + Bubble Tea AI agent, ~41K stars. Studied architecture patterns for Go-based agent loop, TUI integration, provider abstraction. **Confidence: MEDIUM** (from training data, not live verification).
- Claude Code (Anthropic): Agent loop design, tool-first architecture, permission model, CLAUDE.md pattern, context compaction. **Confidence: MEDIUM** (from training data and published documentation patterns).
- aider (github.com/Aider-AI/aider): Python AI coding assistant. Edit format patterns, repo-map context injection (analogous to schema injection), model switching. **Confidence: MEDIUM** (from training data).
- Google ADK Go: Agent Development Kit for Go. Agent loop primitives, Gemini function calling integration. **Confidence: LOW** (ADK Go was relatively new at training cutoff; verify current API surface).
- Bubble Tea / Charm ecosystem: Elm architecture for terminal UIs. Event-driven model, component composition, styling. **Confidence: HIGH** (well-established, stable API).
- BigQuery Go client library: Standard GCP Go client. **Confidence: HIGH** (stable, well-documented).

---
*Architecture research for: AI-native GCP data engineering CLI agent*
*Researched: 2026-03-16*
