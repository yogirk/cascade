# Roadmap

What's been built, what's next. See `specs/original/ROADMAP.md` for the original phased spec.

## Completed

### Foundation (Phase 1)
- Single-threaded agent loop (observe, reason, select tool, execute) with streaming LLM output
- Bubble Tea TUI with Lip Gloss styling, Glamour markdown rendering
- LLM integration via Google ADK Go with Gemini as default
- Model-agnostic provider abstraction (Gemini, Claude, OpenAI)
- Core file tools: Read, Write, Edit, Glob, Grep, Bash
- GCP auth via ADC + service account impersonation
- Policy-first permission model with risk classification
- Interactive mode (primary) and one-shot mode (`-p` flag)
- Config via `~/.cascade/config.toml`

### TUI Excellence (Phase 1.1)
- Streaming markdown rendering with trackpad scroll
- Sweep glow spinner, ocean blue branding, turn summaries
- Welcome screen with connection dashboard
- Interactive model picker (`/model`)
- Permission approval modal (allow once / allow tool for session / deny)

### BigQuery Core (Phase 2)
- Query execution with dry-run cost estimation
- Schema cache from INFORMATION_SCHEMA (SQLite + FTS5)
- Schema-aware context injection for SQL generation
- SQL analysis (cost, performance, correctness suggestions)
- Cost monitoring and session cost tracking with configurable budgets
- Session management with context compaction

### BQ Cost Intelligence (Phase 2.1)
- `/insights` dashboard with INFORMATION_SCHEMA analysis
- Billing export: cross-project queries, auto-discovers export table
- Multi-project schema cache: unified SQLite with project_id in all PKs
- Terminal chart rendering: sparklines and horizontal bar charts

### Multi-Provider Support
- OpenAI (GPT-4o, o3-mini) and Anthropic (Claude Sonnet/Opus/Haiku)
- Interactive model picker with arrow key navigation

### Platform Tools (Phase 3)
- Cloud Logging: query/tail with severity coloring, `/logs` command
- GCS: browse buckets, list objects, read files (capped), metadata
- Async platform client init for fast startup

### Platform Intelligence
- `/morning` briefing: cross-service signal correlation
- Union-find correlation groups related signals into incidents
- CASCADE.md per-project config (critical tables, refresh schedules, alert thresholds)
- Graceful degradation: each signal source independently optional

### Session Persistence
- Conversations stored in SQLite (`~/.cascade/sessions.db`), auto-saved after every turn
- Resume with `--resume` (latest) or `--session <id>` (specific)
- CLI: `cascade sessions` (list), `cascade sessions rm <id>` (delete)
- Slash commands: `/sessions` (list inline), `/save` (force-save)

### Config & Accessibility Hardening
- Full config audit: all 30 TOML fields traced to consumption, 2 dead fields wired up
- `agent.tool_timeout`: enforced via `context.WithTimeout` on every tool execution
- `cost.auth`: cross-project billing with separate service account credentials
- Color-blind tool bullets: shape-differentiated glyphs (`○◇●△□`) alongside color

## Next Up

Roughly prioritized. Not strict phases — can be tackled adhoc.

### Cloud Composer / Airflow Integration
- List DAGs, dag status, task logs, list failures
- Cross-service pipeline debugging (Composer -> Logging -> GCS -> BigQuery)

### MCP Server Support
- Let users connect external tools via Model Context Protocol
- Replaces the need to build every integration natively
- 3 transports: stdio, http, streamable HTTP

### Cost Recommendations
- "This table hasn't been queried in 90 days" — automated insights
- "No partition filter" warnings, unused clustering keys

### dbt Integration
- Model lineage, run/test commands, source freshness
- Manifest parsing, model generation

### Multi-Model Slots
- Large model (reasoning) + small model (simple tasks), switchable mid-session
- Provider interface already supports model switching

### Other Ideas
- Data profiling (nulls, cardinality, distribution, outliers)
- PII detection via Dataplex tags, column name heuristics
- Skills system (markdown files with domain-specific knowledge)
- Hooks system (lifecycle scripts: PreToolUse, PostToolUse, etc.)
- Subagents (isolated goroutines for log analysis, cost analysis)
- Setup wizard for first-run
- Schema-aware autocomplete (tab completion for table/column names)
- Output formats: JSON, CSV, Markdown, quiet
- Distribution via Homebrew, GoReleaser, `go install`
- Degraded/offline mode using cached schema
- Error retry with model fallback

## Out of Scope (V1)

- OS-level sandboxing (maintenance nightmare; IAM + Permission Engine + Cost Gates sufficient)
- Multi-panel TUI for concurrent subagent rendering
- IDE extensions (terminal-first)
- Cascade Cloud / managed team version
- Multi-cloud support (GCP-first)
- Slack/Teams bot, mobile app
- Plugin marketplace (local skills/hooks only)
- Telemetry reporting to remote server
- Terraform apply (read-only plan is fine)
- DataflowTool, PubSubTool (streaming pipelines too complex for V1)
