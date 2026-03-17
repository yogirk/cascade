# Cascade CLI

> An AI-native terminal agent for GCP data engineering.

**What if Claude Code and Snowflake Cortex CLI had a GCP data engineering baby?**

Cascade is a conversational CLI that understands your BigQuery warehouse, your Airflow pipelines, your dbt models, and your entire GCP data platform — as deeply as Claude Code understands your codebase.

```
$ cascade "why did last night's orders pipeline fail?"

Root cause: Schema mismatch. Source file has a new column `discount_type`
not present in `warehouse.bronze.raw_orders`.

Recommended fix:
1. ALTER TABLE to add the column
2. Update the dbt staging model
3. Clear the failed Airflow task to retry

Apply fix? [y/N]
```

## Specs

| Document | Description |
|----------|-------------|
| [VISION.md](specs/VISION.md) | Problem statement, philosophy, and design principles |
| [ARCHITECTURE.md](specs/ARCHITECTURE.md) | System architecture, agent loop, platform context engine, tech stack |
| [TOOLS.md](specs/TOOLS.md) | Complete tool system — core, GCP platform, data engineering, observability, external |
| [UX.md](specs/UX.md) | Terminal UX, commands, keyboard shortcuts, autocomplete, configuration |
| [SECURITY.md](specs/SECURITY.md) | Four-layer security model — IAM, permissions, sandboxing, cost gates |
| [CONTEXT.md](specs/CONTEXT.md) | Context management, schema cache, compaction, session and memory systems |
| [EXTENSIBILITY.md](specs/EXTENSIBILITY.md) | Skills, hooks, subagents, MCP servers, and plugin packs |
| [SCENARIOS.md](specs/SCENARIOS.md) | Six real-world usage scenarios with full interaction transcripts |
| [COMPARISON.md](specs/COMPARISON.md) | Competitive analysis vs Claude Code, Cortex CLI, and GCP native tools |
| [ROADMAP.md](specs/ROADMAP.md) | Six-phase development roadmap with exit criteria and success metrics |

## Key Ideas

**From Claude Code (the golden standard):**
- Single-threaded agent loop — simple, debuggable, reliable
- Tool-first architecture (Read, Write, Edit, Glob, Grep, Bash)
- Permission model with deny/ask/allow tiers + OS sandboxing
- Context compaction for long sessions
- Skills, hooks, subagents, MCP extensibility
- Project config (CASCADE.md)

**From Snowflake Cortex CLI:**
- Deep platform awareness (schema, governance, lineage, cost)
- SQL risk classification (READ → DDL → DML → DESTRUCTIVE)
- Built-in cost estimation before every query
- Enterprise security (RBAC enforcement, PII masking, audit logging)
- Data catalog integration for natural language schema search

**Original to Cascade (the GCP baby):**
- Cross-service pipeline debugging (Composer → Logging → GCS → BigQuery)
- Native dbt integration with manifest parsing and model generation
- Streaming pipeline management (Dataflow + Pub/Sub)
- Terraform integration for data infrastructure
- Model-agnostic (Gemini, Claude, GPT, or local models)
- Open source

## Tech Stack

- **Language:** Go 1.23+
- **Agent Framework:** Google ADK Go (`google.golang.org/adk`)
- **LLM:** Gemini 2.5 Pro (default), model-agnostic via `anthropic-sdk-go`, `openai-go`, Bifrost
- **Terminal UI:** Bubble Tea + Lip Gloss + Glamour + Bubbles + Huh (Charm stack)
- **GCP Auth:** Application Default Credentials (`golang.org/x/oauth2/google`)
- **GCP Clients:** `cloud.google.com/go/bigquery`, `storage`, `logging`, `dataplex`, etc.
- **Local Cache:** SQLite (pure Go, no CGO via `modernc.org/sqlite`)
- **Distribution:** Single static binary — Homebrew, GoReleaser, `go install`
