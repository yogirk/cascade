# Cascade CLI — Architecture

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Terminal UI (Bubble Tea)                    │
│  Lip Gloss styling, Glamour markdown, Bubbles components     │
│  Permission prompts, cost warnings, plan approval gates      │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                     Agent Loop (Core)                        │
│  observe → reason → select tool → execute → observe → ...   │
│  Extended thinking · Adaptive effort · Context compaction    │
└────────────┬───────────────┬──────────────┬─────────────────┘
             │               │              │
    ┌────────▼──────┐ ┌─────▼──────┐ ┌─────▼──────────┐
    │  Tool Registry │ │  Context   │ │  Permission    │
    │               │ │  Manager   │ │  Engine        │
    │  Built-in     │ │            │ │                │
    │  GCP Platform │ │  Schema    │ │  Risk classify │
    │  File/Code    │ │  Cache     │ │  IAM-aware     │
    │  MCP External │ │  Session   │ │  Cost gates    │
    │  Custom       │ │  Memory    │ │  Sandbox       │
    └───────────────┘ └────────────┘ └────────────────┘
             │
    ┌────────▼──────────────────────────────────────────┐
    │              GCP Platform Layer                    │
    │                                                   │
    │  BigQuery  │ Composer │ Dataflow │ GCS │ Pub/Sub  │
    │  Dataplex  │ Dataform │ Logging  │ IAM │ Billing  │
    │                                                   │
    │  Auth: Application Default Credentials (ADC)      │
    │  + Service Account Impersonation                  │
    └───────────────────────────────────────────────────┘
```

## Core Agent Loop

Inspired by Claude Code's single-threaded master loop. Simple, debuggable, reliable. Go's goroutines handle concurrent streaming, tool execution, and UI updates naturally.

```go
func (a *Agent) Run(ctx context.Context) error {
    for {
        // 1. Build context
        messages := a.buildMessages(
            a.systemPrompt,
            a.platformContext,  // schema cache, DAG state, active costs
            a.sessionHistory,
            a.cascadeMD,        // project config (like CLAUDE.md)
        )

        // 2. Stream LLM response (renders to TUI via Bubble Tea messages)
        response, err := a.llm.Stream(ctx, messages, a.activeTools)
        if err != nil {
            return err
        }

        // 3. If no tool calls, final answer — return to user
        if len(response.ToolCalls) == 0 {
            break
        }

        // 4. Execute tool calls (parallel for independent calls via goroutines)
        results := a.executeToolCalls(ctx, response.ToolCalls)
        for i, call := range response.ToolCalls {
            // Check permissions
            switch a.permissions.Classify(call) {
            case Deny:
                results[i] = ToolResult{Error: "Blocked by policy"}
            case Ask:
                if !a.promptUser(call) {
                    results[i] = ToolResult{Error: "Denied by user"}
                }
            case Allow:
                // Already executed
            }
            a.sessionHistory.Append(call, results[i])
        }
    }
    return nil
}
```

### Key Differences from Claude Code's Loop

| Aspect | Claude Code | Cascade |
|--------|------------|---------|
| Pre-execution hooks | Generic PreToolUse | + CostEstimate hook for SQL |
| Context injection | File contents, git state | + Schema cache, DAG state, cost profile |
| System reminders | TODO state, tool docs | + Active alerts, recent failures, cost budget |
| Permission classification | File/bash risk levels | + SQL risk levels (READ/WRITE/DDL/ADMIN) |
| Post-execution hooks | Generic PostToolUse | + Lineage tracking, cost logging |

## LLM Backend

### Primary: Google ADK Go + Gemini

```go
import (
    "google.golang.org/adk/agents/llmagent"
    "google.golang.org/adk/tools/functiontool"
)

agent := llmagent.New(llmagent.Config{
    Name:        "cascade",
    Model:       "gemini-2.5-pro",
    Tools:       tools,          // BigQuery, Composer, GCS, etc.
    Instruction: systemPrompt,
})
```

Google ADK Go (v0.4+) provides the agent loop, tool framework, MCP integration, and multi-agent orchestration. GCP platform tools (BigQuery, Composer, etc.) are implemented as `functiontool` wrappers around the mature `cloud.google.com/go/*` client libraries.

### Model-Agnostic via Provider Abstraction

```go
// internal/llm/provider.go
type Provider interface {
    Stream(ctx context.Context, msgs []Message, tools []Tool) (*StreamResponse, error)
}

// Implementations: Gemini (via ADK), Anthropic (anthropic-sdk-go),
// OpenAI (openai-go), Ollama (local HTTP), Bifrost (gateway)
```

```toml
# ~/.cascade/config.toml

[model]
provider = "google"          # google | anthropic | openai | ollama
model = "gemini-2.5-pro"    # or claude-sonnet-4-5, gpt-4o, etc.
thinking = "adaptive"         # off | adaptive | always
effort = "high"               # low | medium | high | max

[model.fallback]
provider = "anthropic"
model = "claude-sonnet-4-5"
```

Why Google ADK Go as primary:
- Native agent loop with tool orchestration and MCP support
- Native GCP auth (ADC) — same `golang.org/x/oauth2/google` used by all GCP Go clients
- Vertex AI deployment path
- Gemini's 2M context window (largest available)
- Go ADK supports `functiontool` to wrap any Go function as an agent tool

Why model-agnostic:
- Claude is stronger at complex reasoning and code generation
- Official Go SDKs exist for Anthropic (`anthropic-sdk-go`) and OpenAI (`openai-go`)
- Bifrost (Go LLM gateway) provides 15+ providers with 50x less latency than LiteLLM
- Local models (Ollama) for air-gapped/regulated environments

## Platform Context Engine

The key differentiator. This is what makes Cascade "platform-aware" rather than just "AI + gcloud wrapper."

```
┌─────────────────────────────────────────┐
│         Platform Context Engine          │
│                                          │
│  ┌─────────────┐  ┌──────────────────┐  │
│  │ Schema Cache │  │ Pipeline State   │  │
│  │             │  │                  │  │
│  │ Datasets    │  │ DAG run history  │  │
│  │ Tables      │  │ Active failures  │  │
│  │ Columns     │  │ Task durations   │  │
│  │ Types       │  │ Dependencies     │  │
│  │ Partitions  │  │ Schedule info    │  │
│  │ Clustering  │  │ SLA status       │  │
│  │ Descriptions│  │                  │  │
│  │ Tags        │  │                  │  │
│  └─────────────┘  └──────────────────┘  │
│                                          │
│  ┌─────────────┐  ┌──────────────────┐  │
│  │ Cost Profile │  │ Governance       │  │
│  │             │  │                  │  │
│  │ Slot usage  │  │ IAM policies     │  │
│  │ Query costs │  │ Column security  │  │
│  │ Storage     │  │ Data masking     │  │
│  │ Reservations│  │ Dataplex tags    │  │
│  │ Budgets     │  │ Lineage graph    │  │
│  │ Anomalies   │  │ PII detection    │  │
│  └─────────────┘  └──────────────────┘  │
│                                          │
│  ┌─────────────┐  ┌──────────────────┐  │
│  │ dbt State   │  │ Infra State      │  │
│  │             │  │                  │  │
│  │ manifest    │  │ Terraform state  │  │
│  │ run results │  │ Composer env     │  │
│  │ test results│  │ Dataflow jobs    │  │
│  │ sources.yml │  │ Pub/Sub topics   │  │
│  │ lineage     │  │ GCS buckets      │  │
│  └─────────────┘  └──────────────────┘  │
└─────────────────────────────────────────┘
```

### Schema Cache

The schema cache is the heart of platform awareness. It's built on first run and incrementally updated.

```json
// Schema cache structure (stored as JSON in ~/.cascade/cache/)
{
    "project_id": "my-project",
    "last_refreshed": "2026-02-05T10:30:00Z",
    "datasets": {
        "warehouse": {
            "tables": {
                "raw_orders": {
                    "columns": [...],
                    "partitioning": {"field": "order_date", "type": "DAY"},
                    "clustering": ["customer_id", "region"],
                    "row_count": 1_240_000_000,
                    "size_bytes": 48_000_000_000,
                    "description": "Raw orders from Shopify webhook",
                    "tags": {"pii": ["customer_email", "shipping_address"]},
                    "last_modified": "2026-02-05T02:47:00Z"
                }
            }
        }
    }
}
```

How it's used:
- **SQL generation**: The agent knows exact column names, types, and relationships
- **Cost estimation**: Row counts and sizes enable pre-execution cost estimates
- **Governance**: PII tags are surfaced before queries touch sensitive columns
- **Autocomplete**: Schema-aware suggestions in interactive mode

### Refresh Strategy

| Trigger | Action |
|---------|--------|
| First run in project | Full schema sync |
| `cascade sync` | Manual full refresh |
| Before SQL execution | Check table's `last_modified` vs cache |
| After DDL operations | Invalidate affected tables |
| Background (configurable) | Incremental refresh every N minutes |

## Technology Stack

```
Language:           Go 1.23+
Agent Framework:    Google ADK Go (google.golang.org/adk)
LLM SDKs:          anthropic-sdk-go, openai-go (official), Bifrost (gateway)
Terminal UI:        Bubble Tea (TUI framework)
                    + Lip Gloss (styling, tables, trees, layout)
                    + Glamour (markdown rendering, syntax highlighting)
                    + Bubbles (viewport, spinner, text input, progress)
                    + Huh (interactive forms for setup wizard, confirmations)
GCP Auth:           golang.org/x/oauth2/google (Application Default Credentials)
GCP Clients:        cloud.google.com/go/bigquery
                    cloud.google.com/go/storage
                    cloud.google.com/go/logging
                    cloud.google.com/go/orchestration/airflow (Composer)
                    cloud.google.com/go/dataflow
                    cloud.google.com/go/dataplex
dbt Integration:    Subprocess (dbt CLI) + manifest.json parsing in Go
Config:             TOML (user config, via BurntSushi/toml) + Markdown (project config)
Cache:              SQLite (via modernc.org/sqlite — pure Go, no CGO)
Distribution:       Homebrew, standalone binary (goreleaser), go install
Build:              Single static binary, cross-compiled for linux/darwin/windows × amd64/arm64
```

### Why Go + Charm Stack

| Advantage | Details |
|-----------|---------|
| **~5ms startup** | vs ~500-2000ms for Python with AI library imports |
| **Single binary** | `brew install cascade` or download. No runtime, no venv, no pip. |
| **Cross-compilation** | `GOOS=linux GOARCH=arm64 go build` from macOS. Trivial CI/CD. |
| **Native concurrency** | Goroutines for streaming LLM output + tool execution + TUI updates simultaneously |
| **Low memory** | ~5-15 MB baseline vs ~30-100 MB for Python |
| **Proven for AI CLIs** | OpenCode (41K stars) and Crush (12K stars) both use Go + Bubble Tea |
| **Charm ecosystem** | Battle-tested TUI components: tables, markdown, forms, syntax highlighting |
| **GCP clients mature** | `cloud.google.com/go/bigquery` is stable, production-grade |
