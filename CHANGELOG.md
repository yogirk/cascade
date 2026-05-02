# Changelog

All notable changes to Cascade are documented here. Format follows [Keep a Changelog](https://keepachangelog.com/).

## [0.4.0.0] — 2026-05-02

### Added

- **Theme system** with three named themes — Classic (sky blue, the legacy palette), Verse in Code (warm chestnut), and Midnight Hydrology (deep ocean blue). `/theme` slash command lists themes and switches live; the active palette flows through every renderer including markdown, BigQuery tables, charts, and the spinner.
- **Bordered adaptive-width tables** for tool output — rounded corners, accent header, alternating-row dim, column separators. Tables shrink-wrap to content rather than stretching to fill the terminal.
- **Branch-glyph (⎿) information hierarchy** — tool body, errors, and diffs render with `⎿` on the first line and aligned continuation indent on the rest, mirroring Claude Code's parent/child reading.
- **Click to expand/collapse tool blocks**. Mouse click on any tool body toggles it. `Ctrl+E` now acts on the bottom-most expandable tool currently visible in the viewport (replaces the older reverse-cycle behaviour).
- **Inline SQL syntax highlighting** in tool-args headers and the destructive-action confirm dialog, via chroma. Data-driven through a `languageByArgKey` map — adding Python, dbt, YAML, etc. is a one-line entry, not a new highlighter.
- **Responsive markdown tables in assistant output**. Glamour v2 hardcodes table width with no exposed knob; no project in the charmbracelet ecosystem (Crush, Mods, Glow) has solved this. Cascade now extracts pipe tables in a pre-pass and routes them through a shrink-wrapping lipgloss builder, with prose still going through Glamour.
- **Welcome logo redesign** — 4×3 rubik-cube grid of square tiles in the slokam/anushtup-inspired dialect, with three brightness tiers (`cascadeBg1/2/3`) painting a directional cascade gradient. Logo is vertically centered against right-panel content so it stays anchored regardless of how many status rows render.

### Changed

- **Module path** renamed from `github.com/yogirk/cascade` to `github.com/slokam-ai/cascade` following the repo move to the slokam-ai GitHub org.
- **Confirm dialog "Allow tool for session" description** now reads "Allow ALL future invocations of this tool until you exit" — the original "Skip future prompts" copy underplayed the blast radius for tools like `bash` or `bigquery_query`.
- **Welcome screen layout** — left panel sized to match right panel height with the logo centered inside. `JoinHorizontal` switched from `Center` to `Top` so vertical centering happens inside the left box, not at the join.

### Fixed

- **(HIGH security)** `gcloud auth print-access-token` removed from the bash gcloud read-only allowlist — was auto-approved in ModeAsk, which allowed the LLM to print a live OAuth bearer token without user confirmation. Pinned with a regression test asserting it classifies as `RiskDestructive`.
- **`SELECT` queries surfaced as `[DESTRUCTIVE]`** in the confirm dialog when no cost config was supplied. `QueryTool.PlanPermission` now sets `RiskOverride: RiskReadOnly` on every read-only return path; previously four paths returned `nil, nil` and let the conservative base risk bleed through. Three regression tests pin the previously-broken paths.
- **Theme switch** now correctly refreshes the welcome banner snapshot (was baked with the previous palette and read as a stale strip after switching) and the input textarea inner styles (placeholder, cursor line, end-of-buffer region).

### Security

- **BigQuery scripting constructs** (`CALL <proc>`, `BEGIN ... END`, `EXECUTE IMMEDIATE`) explicitly verified to fall through to `RiskDestructive` in `classifyKeyword`. Test cases pin the behaviour and the default branch carries a comment warning future maintainers not to add them to the read-only branch without a parser that can analyse the procedure body — that would open a permission-bypass path.

### Removed

- Dead `cascadeBg1/2/3` lipgloss.Style var wrappers from `internal/tui/styles.go`. The current welcome logo applies the palette colours as foregrounds directly via `cascadeBg{1,2,3}Color`; the Style wrappers were leftover from an earlier rendering approach with no readers.

## [0.3.2.0] — 2026-03-29

### Added

- **Shell escape**: `! <command>` runs a shell command inline and shows output in chat
- **Tool reload**: `/reload` re-registers all tools without restarting
- **Compact tool rendering**: Tool headers use dim name (not bold), show value only (e.g., `src/main.go` not `file_path=src/main.go`), truncated to 40 characters
- **3-line default collapse**: Tool output shows 3 lines by default with `... [N more lines] Ctrl+E` indicator. Errors and diffs still show in full
- **Consecutive tool grouping**: Tool calls within the same turn render with zero spacing between them
- **Ctrl+E cycling**: Each press expands the next collapsed tool output (most recent first). When all expanded, collapses them all

### Fixed

- Welcome screen restored on fresh start (session hydration was hiding it due to system prompt counting as a message)
- Viewport no longer yanks to bottom when scrolled up during streaming/tool execution (save/restore YOffset around SetContent, disable viewport internal mouse wheel bypass)
- Mouse wheel scroll restored after followTail tracking fix (re-enabled viewport internal handling, track followTail in chat.Update)

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
- **Shell escape**: `! <command>` runs a shell command inline and shows output in chat
- **Tool reload**: `/reload` re-registers all tools (core + BigQuery + platform) without restarting
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
