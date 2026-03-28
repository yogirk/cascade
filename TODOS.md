# TODOS

## Infrastructure

### Add .golangci.yml Linter Config

**What:** Create a .golangci.yml with project-appropriate linter rules.

**Why:** Currently falls back to `go vet` only. A proper linter config catches more issues and standardizes CI checks.

**Context:** Identified during eng review 2026-03-24 (Wave 3). The Makefile already has a `lint` target that checks for golangci-lint.

**Effort:** S
**Priority:** P2
**Depends on:** None

### Tests for Logging/GCS Tools and OpenAI/Anthropic Providers

**What:** Add test coverage for cloud_logging, gcs tools, and openai/anthropic provider packages.

**Why:** These packages shipped without dedicated tests. Current test strength is in agent, auth, config, oneshot, bigquery. The newer packages need parity.

**Context:** Eng review 2026-03-24 (Wave 4). Use MockProvider pattern from testutil/ and mock client interfaces. Follow patterns in tools/bigquery/*_test.go.

**Effort:** M
**Priority:** P2
**Depends on:** None

### PermissionPlanner Migration

**What:** Migrate tools that need input-aware risk classification to implement the PermissionPlanner interface.

**Why:** Some tools currently have static risk levels that should vary based on input (e.g., a bash command that reads vs writes). PermissionPlanner enables dynamic risk gating.

**Context:** Eng review 2026-03-24 (Wave 5). The interface exists in tools/ — need to implement it for bash and potentially GCS tools.

**Effort:** S
**Priority:** P2
**Depends on:** None

### GCS Binary Detection Magic Bytes

**What:** Use file magic bytes (not just extension) to detect binary files before attempting to read GCS objects.

**Why:** Current detection relies on extension/heuristics. Magic byte detection prevents attempting to display binary content that would corrupt the terminal output.

**Context:** Eng review 2026-03-24 (Wave 5). Scanner buffer already increased to 1MB. Add magic byte check before passing content through.

**Effort:** S
**Priority:** P3
**Depends on:** None

## Architecture

### app/ Package Refactor (Pattern B)

**What:** When app/ exceeds 7 files or 1800 LOC, extract init logic into service packages (e.g., `bigquery.NewClient(cfg, resource)`, `platform.NewClients(cfg, resource)`).

**Why:** Both Claude and Codex independently validated Pattern B (push init into service packages) over Pattern A (sub-packages). Init knowledge belongs near the code it configures. Keep `app.New()` as thin orchestrator.

**Context:** Not needed yet — trigger when next integration lands (Composer or dbt). Smell test: "How does BigQuery get built?" belongs in bigquery/. "When and why do we build BigQuery?" belongs in app/. Source: eng review 2026-03-24.

**Effort:** M
**Priority:** P2
**Depends on:** Next integration landing (Composer or dbt)

### MCP Server Support

**What:** Support Model Context Protocol with stdio, http, and streamable HTTP transports. Let users connect external tools without Cascade building every integration.

**Why:** Instead of building a Composer tool, a dbt tool, a Dataflow tool — let users bring their own via MCP. This is the extensibility moat. Also unlocks community-contributed tools. Dramatically changes the roadmap — many planned tools could become MCP servers.

**Context:** Crush supports MCP with 3 transports. Source: Crush comparison 2026-03-24. Scoping via /office-hours before implementation.

**Effort:** L
**Priority:** P1
**Depends on:** None

### Multi-Model Slots (Large + Small)

**What:** Two model slots (large for reasoning, small for simple tasks), switchable mid-session. Gemini Pro for complex SQL generation, Flash for schema lookups and formatting.

**Why:** Cost optimization + speed. Simple tasks don't need the most expensive model. Provider interface already supports model switching.

**Context:** Crush has this pattern. Source: Crush comparison 2026-03-24.

**Effort:** S
**Priority:** P2
**Depends on:** None

## TUI

### TUI Alt Screen Trade-off

**What:** Root cause of recurring scroll + text selection conflicts: alt screen disables terminal-native scroll/selection. No wheel-only mouse mode exists in xterm protocol.

**Why:** Current decision: keep alt screen + mouse mode (1 line/tick for smoother scroll). Options explored: (A) drop alt screen — Claude Code model, (B) keep alt screen + keyboard-only — lazygit model, (C) drop Bubble Tea entirely — custom render loop.

**Context:** Revisit when/if UX model shifts toward conversational (non-dashboard) paradigm. Full analysis in personal notes. Source: TUI investigation 2026-03-24, confirmed by Codex.

**Effort:** L
**Priority:** P3
**Depends on:** None

## Charm Libraries

### huh — Terminal Forms

**What:** Replace hand-rolled interactive components (model picker, confirmation dialog) with huh's form primitives. Unlocks richer flows: config setup wizard, project init, multi-step prompts.

**Why:** Embeds directly as Bubble Tea component. 6.7k stars, MIT, actively maintained, fits our Charm v2 stack.

**Context:** Migration candidates: `models.go` (model picker -> huh select), `confirm.go` (permission prompt -> huh confirm). New opportunities: `/config` interactive editor, first-run setup wizard. Repo: github.com/charmbracelet/huh

**Effort:** M
**Priority:** P2
**Depends on:** None

### harmonica — Physics-Based Animations

**What:** Replace manual sine-wave interpolation in spinner pulse effects with proper spring/damping curves. Enables smooth scroll easing and cursor animations.

**Why:** Small library, no dependencies. Proper physics instead of hand-rolled math. 1.5k stars.

**Context:** Candidates: `spinner.go` pulse animation, viewport scroll transitions. Repo: github.com/charmbracelet/harmonica

**Effort:** S
**Priority:** P3
**Depends on:** None

### log — Structured Colorful Logging

**What:** Lip Gloss-styled structured logging for `--verbose`/`--debug` output. Drop-in replacement for stdlib log with leveled output and key-value fields.

**Why:** Matches Cascade's visual style. 3.2k stars.

**Context:** Candidates: debug/verbose mode output, agent loop tracing, tool execution logs. Repo: github.com/charmbracelet/log

**Effort:** S
**Priority:** P3
**Depends on:** None

## Completed

### Config Surface Audit

**What:** Audit all config fields to ensure every declared field is consumed by the code.

**Context:** 30 fields audited, 28 were already used, 2 wired up (`agent.tool_timeout`, `cost.auth`). `api_key_env` was a false positive.

**Completed:** v0.3.1.0 (2026-03-28)

### Color-Blind Tool Bullet Differentiation

**What:** Add glyph shape differentiation to tool bullets alongside color coding: `○` read, `◇` write, `●` exec, `△` query, `□` data.

**Context:** Risk level now flows from agent loop through ToolStartEvent to TUI rendering. Name-based fallback preserved.

**Completed:** v0.3.1.0 (2026-03-28)

### Session Persistence (SQLite)

**What:** Store conversations in SQLite so users can resume after exit. CLI flags `--resume` / `--session`, slash commands `/sessions` / `/save`, subcommand `cascade sessions`.

**Context:** New `internal/persist/` package following schema cache SQLite patterns. Auto-saves after every turn and compaction.

**Completed:** v0.3.1.0 (2026-03-28)
