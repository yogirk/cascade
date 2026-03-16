# Changelog

All notable changes to Cascade are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [0.3.1.0] — 2026-03-28

### Added

- **Session persistence**: Conversations auto-saved to SQLite (`~/.cascade/sessions.db`). Resume with `--resume` (latest) or `--session <id>` (specific). New `cascade sessions` subcommand for listing and deleting. Slash commands `/sessions` and `/save`
- **Color-blind tool bullets**: Shape-differentiated glyphs alongside color — `○` read, `◇` write, `●` exec, `△` query, `□` data. Risk level flows from agent loop through ToolStartEvent to TUI
- **Tool execution timeout**: `agent.tool_timeout` config field now enforced via `context.WithTimeout` on every tool execution (default 120s)
- **Cross-project billing auth**: `cost.auth` config section wired up — use a separate service account for billing export queries
- **Design system**: DESIGN.md with industrial aesthetic, typography, color palette, component patterns, accessibility notes
- **Progressive tool output**: Space to expand/collapse long tool output in viewport mode
- **Welcome banner degraded states**: Graceful degradation when GCP or provider unavailable
- **Adaptive theme colors**: WCAG AA-compliant contrast ratios for light and dark terminals
- **Turn separators**: Visual dividers between conversation turns
- **Responsive status bar**: Width-breakpoint layout (collapses segments at narrow terminals)
- **Split cost display**: Separate BQ and LLM token cost segments in status bar
- **ROADMAP.md**: Consolidated roadmap replacing GSD planning artifacts
- **CHANGELOG.md**: Versioned changelog for gstack `/ship` workflow
- **VERSION**: 4-digit version file for gstack `/ship` workflow

### Changed

- Input box height capped at 40% terminal height to prevent viewport takeover
- Tool bullet colors updated: cyan for query tools, indigo for data platform tools
- Dim text color raised for WCAG readability (~3.7:1 contrast ratio)

### Fixed

- Config audit: `agent.tool_timeout` and `cost.auth` fields were declared but never consumed — both now wired to actual behavior
- `/clear` restores welcome banner on empty conversation

## [0.3.0.0] — 2026-03-28

Baseline release capturing all work shipped to date. Cascade is pre-alpha with full BigQuery workflows, Cloud Logging, GCS, platform intelligence, and multi-provider support.

### Added

- **Agent loop**: Single-threaded observe-reason-act cycle with streaming LLM output
- **TUI**: Bubble Tea v2 terminal interface — streaming markdown, trackpad scroll, sweep glow spinner, ocean blue branding
- **Core tools**: read_file, write_file, edit_file, glob, grep, bash with risk classification
- **Permission engine**: ASK / READ_ONLY / FULL_ACCESS modes, approval modal (allow once / allow tool for session / deny), session-scoped tool allowlisting
- **BigQuery query**: SQL execution with automatic dry-run cost estimation, cost guards
- **BigQuery schema**: INFORMATION_SCHEMA-based cache in SQLite + FTS5, multi-project support, schema-aware context injection
- **Cost tracking**: Per-query costs, session totals, budget warnings, `/cost` command
- **Cost intelligence**: `/insights` dashboard, INFORMATION_SCHEMA analysis for query costs/storage/slots, billing export (cross-project), inline sparklines and bar charts
- **Cloud Logging**: Query/tail logs with severity coloring, smart message extraction, `/logs` command
- **Cloud Storage**: Browse buckets, list objects, read files (capped), metadata inspection
- **Platform intelligence**: `/morning` briefing, cross-service signal correlation (union-find), CASCADE.md per-project config
- **Multi-provider support**: Gemini (default), OpenAI (GPT-4o, o3-mini), Anthropic (Claude Sonnet/Opus/Haiku)
- **Interactive model picker**: `/model` with arrow key navigation
- **Session management**: Context compaction at 80% window usage, `/compact` command
- **One-shot mode**: `cascade -p "..."` for scripting and CI
- **Config**: `~/.cascade/config.toml` with layered defaults (TOML < env vars < CLI flags)
- **Auth**: Two-plane auth — GCP resources (ADC/impersonation/SA key) + LLM provider (Vertex/Gemini API/OpenAI/Anthropic)
- **Design system**: DESIGN.md with industrial aesthetic, typography, color palette, component patterns
- **TUI polish**: Adaptive light/dark themes, turn separators, progressive tool output (Space to expand/collapse), welcome banner with degraded states, user message restyling

### Fixed

- CTE-prefixed DML bypass in SQL classifier (security fix — WITH...INSERT classified correctly)
- Multi-project schema lookups filter by project_id
- FTS5 duplicate accumulation on /sync (DELETE before re-insert)
- Context injection computed once per turn (moved outside agent loop)
- GCS pagination capped at 100, scanner buffer increased to 1MB
- Cloud Logging payload extraction and message truncation
- Scroll step reduced to 1 line per wheel tick
- Input box height capped to prevent viewport takeover
- /clear restores welcome banner on empty conversation
