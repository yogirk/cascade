# Cascade CLI

## What This Is

Cascade is an AI-native terminal agent for GCP data engineering — a conversational CLI that understands your BigQuery warehouse, Airflow pipelines, dbt models, and entire GCP data platform as deeply as Claude Code understands a codebase. Built in Go with the Charm TUI stack, it unifies 8+ daily-use GCP tools into a single platform-aware interface. Command: `cascade` (alias: `csc`).

## Core Value

A data engineer can diagnose pipeline failures, investigate costs, write queries, and manage their GCP data stack through one conversational interface that understands their warehouse schema, pipeline dependencies, and cost profile — eliminating context-switching between disparate tools.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Single-threaded agent loop (observe → reason → select tool → execute) with streaming LLM output
- [ ] Bubble Tea TUI with Lip Gloss styling, Glamour markdown rendering, and interactive components
- [ ] LLM integration via Google ADK Go with Gemini 2.5 Pro as default
- [ ] Model-agnostic provider abstraction (Gemini, Claude, OpenAI, Ollama, Bifrost)
- [ ] Core file tools: Read, Write, Edit, Glob, Grep, Bash
- [ ] GCP auth via Application Default Credentials + service account impersonation
- [ ] BigQuery query execution with dry-run cost estimation before every query
- [ ] BigQuery schema exploration (list datasets, describe tables, search columns)
- [ ] Schema cache built from INFORMATION_SCHEMA, stored in SQLite with FTS5
- [ ] Schema-aware context injection for SQL generation
- [ ] Cloud Composer/Airflow integration (list DAGs, dag status, task logs, list failures)
- [ ] Cloud Logging tool for querying logs and errors
- [ ] GCS tool (ls, head, profile — read-only)
- [ ] Cross-service pipeline debugging (Composer → Logging → GCS → BigQuery)
- [ ] Platform summary injection (alerts, failures, cost in system prompt)
- [ ] dbt integration (run, test, build, manifest parsing, model generation, lineage)
- [ ] Data profiling (nulls, cardinality, distribution, outliers)
- [ ] SQL analysis (cost, performance, correctness suggestions)
- [ ] Cost monitoring and session cost tracking with configurable budgets
- [ ] Permission model with risk classification (READ_ONLY → DDL → DML → DESTRUCTIVE → ADMIN)
- [ ] PII detection via Dataplex tags, column name heuristics, and data pattern matching
- [ ] Interactive mode (primary) and one-shot mode (-p flag) for scripting/CI
- [ ] Session management with context compaction for long sessions
- [ ] CASCADE.md project config file (like CLAUDE.md)
- [ ] Config via ~/.cascade/config.toml (project, model, cost, composer, dbt, cache, security, display)
- [ ] Skills system (markdown files with domain-specific knowledge, auto-activated)
- [ ] Hooks system (lifecycle scripts: PreToolUse, PostToolUse, PreSQLExecution, etc.)
- [ ] Subagents (fire-and-forget goroutines with isolated context for log analysis, cost analysis, etc.)
- [ ] MCP server integration for external tools
- [ ] Slash commands (/help, /compact, /plan, /sync, /cost, /failures, /lineage, /profile, /dbt, etc.)
- [ ] Keyboard shortcuts (Ctrl+C cancel, Ctrl+B background, Shift+Tab permission modes, etc.)
- [ ] Permission modes: CONFIRM (default), PLAN (read-only), BYPASS (auto-approve)
- [ ] Output formats: terminal (default), JSON, CSV, Markdown, quiet
- [ ] Setup wizard for first-run (detect GCP project, datasets, Composer env, dbt project, build cache)
- [ ] Distribution as single static binary via Homebrew, GoReleaser, go install
- [ ] Degraded/offline mode using cached schema when APIs are unavailable
- [ ] Error retry with model fallback for bad SQL generation

### Out of Scope

- OS-level sandboxing (sandbox-exec, bubblewrap) — maintenance nightmare, Claude Code has dedicated team for this; V1 relies on IAM + Permission Engine + Cost Gates
- Multi-panel TUI for concurrent subagent rendering — excessive complexity for V1
- IDE extensions (VS Code, JetBrains) — terminal-first
- Cascade Cloud / managed team version — premature
- Multi-cloud support (AWS, Azure) — GCP-first focus
- Slack/Teams bot — different product
- Mobile app — terminal tool
- Plugin marketplace — support local skills/hooks only
- Telemetry reporting to remote server — local-only stats
- Schema-aware autocomplete — requires stable schema cache, Phase 2+ effort
- Terraform apply integration — read-only terraform plan is fine for V1
- DataflowTool, PubSubTool — streaming pipelines complex, Phase 2+

## Context

Cascade fills the gap between general AI coding assistants (Claude Code, Cursor) that lack platform awareness and single-service CLI tools (bq, gcloud, gsutil) that require constant context-switching. The closest comparable is Snowflake's Cortex CLI, but nothing equivalent exists for GCP.

The project draws heavily from Claude Code's proven patterns (agent loop, tool-first architecture, permission model, context compaction, extensibility) while adding data engineering domain knowledge (schema awareness, cost estimation, pipeline debugging, dbt integration, governance).

Key technical decisions from spec review:
- **Read-first strategy**: Investigation and debugging before code generation. The "what failed last night?" scenario is high-frequency, low-risk, and trust-building.
- **INFORMATION_SCHEMA for schema cache**: Bulk SQL queries instead of per-table API calls. Handles 10K+ table warehouses.
- **Deferred OS sandboxing**: Layers 1 (IAM), 2 (Permission Engine), and 4 (Cost Gates) provide sufficient V1 security.
- **Simplified subagents**: Fire-and-forget goroutines returning summaries, no concurrent TUI rendering.

Competitive advantages: cross-service pipeline debugging (no existing tool does this), cost-awareness by default, governance-native, model-agnostic, open source.

## Constraints

- **Tech Stack**: Go 1.23+, Google ADK Go, Bubble Tea + Lip Gloss + Glamour + Bubbles + Huh, SQLite (pure Go via modernc.org/sqlite), TOML config
- **Auth**: GCP Application Default Credentials only — no custom auth system
- **LLM Default**: Gemini 2.5 Pro via ADK Go, with provider abstraction for alternatives
- **Distribution**: Single static binary, cross-compiled for linux/darwin/windows × amd64/arm64
- **Security Model**: 3-layer for V1 (IAM + Permission Engine + Cost Gates), OS sandbox deferred
- **Schema Cache**: INFORMATION_SCHEMA-based, dataset-scoped, SQLite with FTS5
- **Subagents**: Fire-and-forget goroutines only, no orchestration complexity

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go + Charm stack | ~5ms startup, single binary, native concurrency, proven for AI CLIs (OpenCode 41K stars) | — Pending |
| Google ADK Go as primary LLM framework | Native agent loop, GCP auth, Vertex AI path, Gemini 2M context | — Pending |
| Read-first build strategy | Investigation/debugging is high-frequency, low-risk, trust-building; earns right to write later | — Pending |
| INFORMATION_SCHEMA for schema cache | Bulk queries vs 10K+ individual API calls; avoids quota issues | — Pending |
| Defer OS-level sandboxing | Maintenance nightmare, breaks legitimate tools; IAM + Permission Engine + Cost Gates sufficient for V1 | — Pending |
| Simplified subagents (fire-and-forget goroutines) | Context isolation without orchestration complexity; fancy UI later | — Pending |
| Model-agnostic via Provider interface | Claude stronger at reasoning, Gemini at structured data; user choice | — Pending |
| Dataset-scoped schema cache | Most users care about 2-3 datasets, not all 50; prevents blocking startup | — Pending |

---
*Last updated: 2026-03-16 after initialization*
