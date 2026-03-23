# Cascade

AI-native terminal agent for GCP data engineering. Think Claude Code, but for BigQuery, Airflow, dbt, and your entire GCP data platform.

## What It Does

Cascade is a conversational CLI that understands your data warehouse schema, pipeline dependencies, and cost profile. Ask questions in natural language, run queries with cost awareness, explore schemas, and diagnose issues — all from the terminal.

## Status

**Pre-alpha — actively developed.** Cascade is usable today for BigQuery workflows. Platform tools and multi-provider support are in progress.

### What's working

| Component | Status | Notes |
|-----------|--------|-------|
| Conversational TUI | Done | Streaming, markdown, trackpad scroll, sweep glow spinner |
| Core tools | Done | read, write, edit, glob, grep, bash |
| Permission engine | Done | Risk classification, approval modal, 3 modes |
| BigQuery query | Done | Execute SQL, dry-run cost estimation, cost guards |
| BigQuery schema | Done | Schema cache (SQLite + FTS5), explore, search, context injection |
| Cost tracking | Done | Per-query cost, session totals, budget warnings, `/cost` |
| Cost intelligence | Done | `/insights` dashboard, INFORMATION_SCHEMA analysis, inline sparklines + bar charts |
| Billing export | Done | Cross-project billing queries, auto-discovers export table |
| Multi-project cache | Done | Unified SQLite cache across projects, no dataset collisions |
| SQL optimization hints | Done | Partition filters, clustering keys, expensive JOINs |
| Context compaction | Done | Auto at 80%, `/compact` manual trigger |
| One-shot mode | Done | `cascade -p "..."` for scripting |
| Gemini provider | Done | API key and Vertex AI |
| OpenAI provider | Done | GPT-4o, GPT-4 Turbo, o3-mini |
| Anthropic provider | Done | Claude Sonnet, Opus, Haiku |
| Interactive model picker | Done | `/model` with arrow key selection |
| Turn summaries | Done | Elapsed time + token counts persist after each turn |
| Cloud Logging | Done | Query/tail logs, severity coloring, `/logs` command |
| Cloud Storage | Done | Browse buckets, list objects, read files (capped), metadata |

### Roadmap

| Phase | Description | Status |
|-------|-------------|--------|
| Cloud Composer | Airflow DAG inspection, task logs, trigger runs | Next |
| Cloud Composer | Airflow DAG inspection, task logs, trigger runs | Planned |
| Cost recommendations | "This table hasn't been queried in 90 days" — automated insights | Planned |
| dbt integration | Model lineage, run/test commands, source freshness | Planned |
| Schema autocomplete | Tab completion for table/column names | Planned |

## Getting Started

### Prerequisites

- **Go 1.26+**
- **GCP credentials** (for BigQuery and other GCP tools):
  ```bash
  gcloud auth application-default login
  ```
- **LLM provider** — one of:
  - Gemini API key: `export GOOGLE_API_KEY="your-key"` (cheapest, recommended)
  - Vertex AI: uses your GCP credentials automatically
  - OpenAI: `export OPENAI_API_KEY="sk-..."` (requires API credits from platform.openai.com)
  - Anthropic: `export ANTHROPIC_API_KEY="sk-ant-..."` (requires API credits from console.anthropic.com)

### Install

```bash
# From source
git clone https://github.com/yogirk/cascade.git
cd cascade
make build

# Or install directly
go install github.com/yogirk/cascade/cmd/cascade@latest
```

### Run

```bash
# Interactive mode
./bin/cascade

# One-shot mode
./bin/cascade -p "show me the largest tables in my project"
```

### Configure

Create `~/.cascade/config.toml`:

```toml
# LLM provider
[model]
provider = "gemini_api"  # gemini_api | vertex | openai | anthropic
model = "gemini-3-flash-preview"

# GCP platform access
[gcp]
project = "my-gcp-project"

[gcp.auth]
mode = "adc"  # adc | impersonation | service_account_key

# BigQuery
[bigquery]
datasets = ["my_dataset", "analytics"]  # Datasets to cache for schema-aware queries

# Cost controls
[cost]
warn_threshold = 1.0     # Dollar amount to prompt confirmation
max_query_cost = 10.0    # Dollar amount to block query
daily_budget_usd = 100.0 # Session budget warning at 80%
billing_project = ""     # Project with billing export (optional, cross-project OK)
billing_dataset = ""     # Billing export dataset name (optional)

# Permission mode
[security]
default_mode = "ask"  # ask | read-only | full-access
```

Without a config file, Cascade auto-detects: `GOOGLE_API_KEY` for the LLM, ADC for GCP tools.

## Features

### Core
- Streaming conversational TUI (Bubble Tea v2 + Lip Gloss v2 + Glamour v2)
- Tool system: `read_file`, `write_file`, `edit_file`, `glob`, `grep`, `bash`
- Policy-first permission engine with risk classification
- Approval modal: allow once, allow tool for session, deny
- Session context compaction at 80% window usage (`/compact`)
- One-shot mode for scripting (`cascade -p "..."`)

### BigQuery
- `bigquery_query` — Execute SQL with automatic dry-run cost estimation; `dry_run=true` for cost-only
- `bigquery_schema` — Explore schemas: list datasets, tables, describe columns, FTS5 search
- Multi-project schema cache (unified SQLite + FTS5) — datasets from multiple GCP projects in one index
- Schema-aware context injection with fully-qualified `project.dataset.table` references
- SQL optimization hints: missing partition filters, unused clustering keys, expensive JOINs
- Cost intelligence: INFORMATION_SCHEMA analysis for query costs, storage, slot utilization
- Billing export support: cross-project queries with auto-discovery of export table
- Inline terminal charts: sparklines (▁▂▃▄▅▆▇█) and horizontal bar charts in ocean blue
- `/insights` — One-command cost health dashboard (query trend, top queries, storage, slots)
- `/cost` — Styled session cost breakdown
- `/sync [dataset]` — Refresh schema cache (syncs all configured projects)

### Cloud Logging
- `cloud_logging` — Query and tail GCP log entries with filter syntax
- Severity coloring: DEBUG (dim) → INFO (blue) → WARNING (amber) → ERROR (red) → CRITICAL (bright red)
- Smart message extraction from proto/JSON payloads
- `/logs [severity] [duration]` — Quick access to recent logs (default: WARNING, 1h)

### Cloud Storage
- `gcs` — Browse buckets, list objects, read files, inspect metadata
- Directory-style browsing with prefix + delimiter
- File reading capped at 100 lines (text files only, binary detection)
- Styled output with line numbers for file content

### Auth
- Two independent auth planes: GCP resources + LLM provider
- GCP: ADC, service account impersonation, or key file
- LLM: Vertex AI (reuses GCP auth), Gemini API key, OpenAI API key, Anthropic API key
- Auto-detection: checks env vars in order (`GOOGLE_API_KEY` → `ANTHROPIC_API_KEY` → `OPENAI_API_KEY` → Vertex AI)
- Note: consumer subscriptions (ChatGPT Pro, Claude Max) cannot be used — separate API keys required
- Startup report shows what's available

### UX
- Ocean blue cascade branding with animated tilde spinner
- Sweep glow text effect and per-turn elapsed timer with token counts
- Welcome screen with connection dashboard (project, datasets, mode)
- Human-friendly model names in status bar (e.g., "Gemini 3 (Flash)")
- Interactive model picker (`/model`) with arrow key navigation
- Custom markdown theme with borderless tables and alternating row dimming
- Trackpad scroll support
- Slash commands: `/help`, `/model`, `/compact`, `/sync`, `/cost`, `/insights`, `/logs`

## Architecture

```mermaid
graph TD
    User([User]) --> TUI

    subgraph Cascade
        TUI[Bubble Tea TUI<br/><i>chat, input, status, confirm</i>]
        TUI --> Agent[Agent Loop<br/><i>observe - reason - tool - execute</i>]
        Agent --> Permissions[Permission Engine<br/><i>risk classification, approval modal</i>]

        Agent --> Tools

        subgraph Tools
            Core[Core Tools<br/><i>read, write, edit, glob, grep, bash</i>]
            BQTools[BigQuery Tools<br/><i>query, schema, insights</i>]
            PlatformTools[Platform Tools<br/><i>cloud_logging, gcs</i>]
        end

        subgraph Auth[Auth Resolvers]
            Resource[Resource Plane<br/><i>ADC / impersonation / SA key</i>]
            Model[Model Plane<br/><i>vertex / gemini_api / openai / anthropic</i>]
        end

        Agent --> Model
        BQTools --> Resource
        PlatformTools --> Resource
        BQTools --> SchemaCache[Schema Cache<br/><i>SQLite + FTS5, multi-project</i>]
        BQTools --> BillingExport[Billing Export<br/><i>cross-project cost data</i>]
    end

    Model --> LLM[LLM Provider<br/><i>Gemini, Claude, GPT, ...</i>]
    Resource --> GCP[GCP APIs<br/><i>BigQuery, GCS, Logging, Composer</i>]
    SchemaCache --> GCP
    BillingExport --> GCP

    style TUI fill:#1e3a5f,stroke:#6B9FFF,color:#F3F4F6
    style Agent fill:#1e3a5f,stroke:#6B9FFF,color:#F3F4F6
    style Permissions fill:#2d2235,stroke:#818CF8,color:#F3F4F6
    style Core fill:#1a2e1a,stroke:#34D399,color:#F3F4F6
    style BQTools fill:#1a2e1a,stroke:#34D399,color:#F3F4F6
    style PlatformTools fill:#1a2e1a,stroke:#34D399,color:#F3F4F6
    style Resource fill:#2a2510,stroke:#FBBF24,color:#F3F4F6
    style Model fill:#2a2510,stroke:#FBBF24,color:#F3F4F6
    style SchemaCache fill:#1a2e1a,stroke:#34D399,color:#F3F4F6
    style BillingExport fill:#2a2510,stroke:#FBBF24,color:#F3F4F6
    style LLM fill:#0d1117,stroke:#4B5563,color:#9CA3AF
    style GCP fill:#0d1117,stroke:#4B5563,color:#9CA3AF
    style User fill:#0d1117,stroke:#6B9FFF,color:#F3F4F6
```

## Development

```bash
make build      # Build binary to bin/cascade
make test       # Run all tests with race detector
make test-short # Run unit tests only
make lint       # Run go vet
make clean      # Remove build artifacts
```

## License

[MIT](LICENSE)
