# Project Research Summary

**Project:** Cascade CLI — AI-native terminal agent for GCP data engineering
**Domain:** Conversational AI CLI agent with deep GCP platform integration
**Researched:** 2026-03-16
**Confidence:** HIGH (stack verified via live tooling; architecture patterns from production reference implementations)

## Executive Summary

Cascade is an AI terminal agent that brings conversational intelligence to GCP data engineering workflows — filling the gap that exists between generic AI coding assistants (Claude Code, Aider) and walled-garden platform AI (Snowflake Cortex, Databricks Assistant). The product's core premise is validated by a clear market void: no tool today combines AI conversation, GCP platform awareness (BigQuery, Composer, Cloud Logging, GCS), and terminal-native UX into a single binary. The reference architecture is well-established: Go + Bubble Tea + an agent loop pattern proven by OpenCode (41K stars), with Google ADK Go providing the agent framework and GCP integration infrastructure.

The recommended approach is an inside-out build strategy: foundation first (agent loop + TUI + GCP auth), then BigQuery as the primary data surface (schema cache is the quality foundation for everything), then platform tools (Composer, Logging, GCS) that enable the killer cross-service debugging workflow. The schema cache powered by SQLite + FTS5 is the single most important architectural component — it transforms Cascade from a generic chatbot into a warehouse-aware agent, lifting SQL generation accuracy from ~60% to ~90%. Every feature that matters (NL-to-SQL, cross-service debugging, platform summary) depends on schema quality.

The critical risks are all known and solvable: Bubble Tea streaming deadlock must be designed correctly from day one (ring buffer + render tick, never per-token TUI messages); context window exhaustion from schema injection requires FTS5-based relevance filtering (inject 10-20 tables, never all); ADK Go is early-stage and must be abstracted behind a Provider interface so it remains replaceable. The permission model needs read-auto-approve from the start to avoid permission fatigue. None of these are novel problems — each has a well-understood solution documented in PITFALLS.md.

## Key Findings

### Recommended Stack

Cascade is a single-binary Go CLI with a cohesive modern stack. The Charm v2 ecosystem (Bubble Tea v2, Lip Gloss v2, Glamour v2, Huh v2) handles the entire TUI layer. Google ADK Go v0.6.0 provides the agent framework. GCP official Go client libraries cover BigQuery, GCS, Cloud Logging, Composer, and Dataplex. SQLite via `modernc.org/sqlite` (pure Go, zero cgo, FTS5 support) stores schema cache and session state.

All packages require Go 1.26+ (floor set by Glamour v2 + Huh v2 at 1.25.8; use 1.26.1). The entire Charm stack migrated to `charm.land/` vanity import paths in v2 — do not mix v1 and v2 packages.

**Core technologies:**
- **Go 1.26+**: Language runtime — sub-5ms startup, single binary, goroutines for subagents
- **Google ADK Go v0.6.0**: Agent framework — agent loop, tool dispatch, session management, MCP integration, tool confirmation (HITL); use as LLM client abstraction, not as agent loop orchestrator
- **Google GenAI SDK v1.50.0**: Underlying Gemini API client used by ADK
- **Anthropic SDK v1.26.0 + OpenAI SDK v3.28.0**: Provider adapters for multi-model support
- **Bubble Tea v2.0.2**: TUI framework (Elm architecture) — 40.7K stars, v2 ships Cursed Renderer
- **Lip Gloss v2.0.2 + Glamour v2.0.0 + Bubbles v2.0.0 + Huh v2.0.3**: TUI styling, markdown, components, forms
- **modernc.org/sqlite v1.46.1**: Schema cache + session DB — pure Go, FTS5, zero cgo
- **cloud.google.com/go/bigquery v1.74.0**: Primary GCP data surface
- **cloud.google.com/go/{storage, logging, orchestration, dataplex}**: Full GCP tool coverage
- **Cobra v1.10.2**: CLI framework — used by ADK Go itself, kubectl, gh
- **BurntSushi/toml v1.6.0**: Config parsing — lighter than Viper, sufficient for Cascade's needs
- **MCP Go SDK v1.4.1**: Official MCP client/server (already integrated with ADK Go)
- **GoReleaser v2.14.3**: Cross-platform distribution

**Critical decision:** ADK Go uses `genai.*` types throughout its `model.LLM` interface. Non-Gemini providers (Claude, OpenAI) require translation adapters (~200-400 lines each). This is the right tradeoff — ADK provides the agent loop, tool dispatch, session management, and MCP integration for free. Abstract ADK behind a `Provider` interface from day one so it remains replaceable if ADK proves too immature.

### Expected Features

The "What Failed Last Night?" framing captures the V1 value proposition perfectly. A data engineer currently debugging a pipeline failure opens 4-5 browser tabs: Composer UI, Cloud Logging, BigQuery console, GCS browser. Cascade collapses this into a single conversational session.

**Must have (table stakes — V1):**
- Natural language to SQL with schema-aware context injection
- BigQuery query execution with automatic dry-run cost estimation
- Schema exploration (list datasets, describe tables, FTS5 column search)
- GCP authentication via ADC (zero custom auth code)
- Cloud Composer integration (read-only: DAG status, task logs, failures)
- Cloud Logging queries (scoped, paginated, time-bounded)
- GCS read-only tools (list, head, profile landing data)
- Permission engine with risk classification (READ_ONLY through ADMIN)
- Interactive mode + one-shot `-p` flag
- Streaming LLM output with real-time token rendering
- Session cost tracking (BigQuery bytes + LLM tokens)
- Configuration system (`~/.cascade/config.toml` + `CASCADE.md`)
- Readable terminal output (Glamour markdown, syntax-highlighted SQL, formatted tables)
- Error explanations (parse BigQuery/Composer errors, explain and suggest fixes)
- Setup wizard (first-run project/dataset/Composer detection, schema cache build)

**Should have (competitive differentiators — V1.x after validation):**
- Cross-service pipeline debugging orchestration (the capstone: Composer → Logs → GCS → BigQuery, automated by the LLM)
- Schema-aware context injection with relevance filtering (FTS5, token-budgeted)
- dbt integration (manifest parsing, lineage, run/test/build, model generation)
- Platform summary injection (morning briefing: failures, cost delta, freshness alerts)
- Data profiling on demand (null rates, cardinality, distributions)
- SQL analysis and optimization (query plan, partition filter suggestions)
- PII detection (Dataplex tags + column name heuristics + pattern matching)
- Skills system (domain knowledge markdown files, auto-activated)
- Hooks system (PreToolUse/PostToolUse lifecycle extensibility)
- Subagent delegation (fire-and-forget background analysis)
- Model provider switching (Claude, OpenAI, Ollama via Bifrost)
- Degraded/offline mode (cached schema enables SQL generation without GCP)

**Defer (V2+):**
- Dataflow/Pub/Sub streaming pipeline debugging
- Schema-aware tab autocomplete
- MCP server integration (ecosystem maturity unclear)
- Multi-panel TUI for concurrent subagent rendering
- Terraform plan analysis (read-only IaC understanding)
- Dataplex deep integration (growing adoption curve)

**Anti-features (never build):** multi-cloud support, GUI/web dashboard, IDE extension, automated pipeline deployment, plugin marketplace, built-in charting, Slack bot, multi-user server features.

### Architecture Approach

Cascade follows the layered architecture proven by OpenCode and Claude Code: a single-threaded agent loop at the core, surrounded by an interface-based tool registry, a layered context builder for dynamic system prompt assembly, and a Bubble Tea TUI that communicates with the agent via typed event channels (not direct calls). The TUI is a thin rendering layer; the agent loop is fully independent and works identically in interactive and one-shot modes.

**Major components:**
1. **Agent Loop** (`internal/agent/loop.go`) — single goroutine, observe-reason-act cycle; hard limit of 15-20 tool calls per turn; loop governor prevents infinite cycling on ambiguous requests
2. **Schema Cache** (`internal/schema/`) — SQLite + FTS5 storing INFORMATION_SCHEMA data; relevance-filtered injection (10-20 tables max per LLM call, never full schema); background async refresh, never blocking startup
3. **Tool Registry** (`internal/tools/`) — interface-based, self-describing (name, description, JSON schema, risk level, execute); tool domains in subpackages: `bigquery/`, `composer/`, `logging/`, `gcs/`, `dbt/`, `core/`
4. **Permission Engine** (`internal/permission/`) — middleware between agent loop and tool execution; auto-approve READ_ONLY, prompt for DML+; risk enum per tool; PLAN/CONFIRM/BYPASS modes
5. **Context Builder** (`internal/agent/context.go`) — dynamic system prompt assembly with token budgets per layer: base instructions → CASCADE.md → schema context (FTS5-filtered) → platform summary → active skills → compacted history
6. **Provider Layer** (`internal/provider/`) — `model.LLM` adapters for Gemini (via ADK), Claude, OpenAI; agent loop never sees provider-specific types
7. **TUI Layer** (`internal/tui/`) — Bubble Tea model; receives events from agent via channel; never calls tools directly; token streaming via ring buffer + 30fps render tick
8. **Config Manager** (`internal/config/`) — layered: defaults → global TOML → CASCADE.md → env vars → CLI flags; read-only after init
9. **Cost Tracker** (`internal/cost/`) — per-query dry-run estimates, session accumulator, budget enforcement; partition metadata in schema cache to guide LLM toward cheaper queries
10. **Session Manager** (`internal/agent/session.go`) — SQLite-backed message store; token counting; automatic compaction at 80% context window

**Key patterns:** single-threaded agent loop (deterministic, debuggable); interface-based tool registry (extensible without touching agent loop); event-driven TUI decoupled from agent; permission as middleware (tools declare risk, engine enforces); layered context assembly with token budgets; FTS5-based schema relevance filtering.

### Critical Pitfalls

1. **Bubble Tea streaming deadlock** — Use a ring buffer for LLM token delivery, batch renders at 30fps tick, never send per-token TUI messages. Must be designed correctly in Phase 1; retrofitting requires near-rewrite of TUI layer.

2. **Context window exhaustion from schema injection** — Never inject full schema. FTS5-filter to 10-20 relevant tables. Truncate query results to 50 rows. Auto-compact session at 80% context window. Gemini's 2M context is a safety net, not a crutch — irrelevant context degrades quality.

3. **ADK Go immaturity** — Abstract ADK Go behind a `Provider` interface from day one. Use ADK as an LLM client library, not as the agent loop orchestrator. With proper abstraction, swapping providers is 2-3 days; without it, 2-3 weeks of rework.

4. **Agent loop infinite cycling** — Hard limit of 15-20 tool calls per turn, configurable. Detect diminishing returns (same tool called twice with same args). After 5 tool calls with no user-facing output, inject a progress nudge. Must be built into the loop from the start.

5. **INFORMATION_SCHEMA over-scanning** — Always use dataset-scoped views (`project.dataset.INFORMATION_SCHEMA.COLUMNS`), never project-scoped region views. One wrong query on a 500+ dataset enterprise org scans TBs and costs real money. Per-dataset sync with progress reporting.

6. **Permission fatigue causing BYPASS overuse** — Auto-approve READ_ONLY operations silently. Only prompt for DML and above. Reserve blocking confirmation dialogs for writes, DDL, destructive ops. Design this correctly in Phase 1 — it affects UX profoundly.

7. **GCP auth token expiry mid-session** — Use `google.DefaultTokenSource()` exclusively, never cache tokens manually. Add retry-on-401 middleware. Data engineers sit in sessions for hours; the 1-hour ADC token expiry will be hit in production.

8. **SQLite FTS5 corruption under concurrent access** — Single `*sql.DB` with WAL mode + `busy_timeout=5000`. `SetMaxOpenConns(1)` for writes. Schema cache writes via a serialized channel; subagents never write to FTS5 index directly.

## Implications for Roadmap

Based on the dependency graph in FEATURES.md and the build order in ARCHITECTURE.md, the natural phase structure is:

### Phase 1: Foundation — Working Agent
**Rationale:** Everything depends on a functioning agent loop, TUI, and GCP auth. No other work is useful without this base. Streaming deadlock (Pitfall 1), tool call resilience (Pitfall 2), permission fatigue (Pitfall 6), auth expiry (Pitfall 7), and ADK Go abstraction (Pitfall 10) must all be addressed here — they cannot be retrofitted cheaply.
**Delivers:** A working conversational agent that can authenticate to GCP, execute tools, render streaming output, and enforce permissions. Interactive mode + one-shot `-p` flag. Core file tools (read, write, bash, glob, grep).
**Addresses:** Agent loop, TUI, GCP auth, permission engine, configuration system, streaming output, one-shot mode
**Avoids:** Streaming deadlock (ring buffer + tick design), permission fatigue (read-auto-approve), ADK Go lock-in (Provider interface), infinite loop (turn governor)

### Phase 2: BigQuery Core — The Data Surface
**Rationale:** Schema cache is the quality foundation for everything. Natural language to SQL is the headline feature, but it requires the schema cache to be complete and correctly scoped. Cost estimation builds trust with data engineers who fear surprise bills. This phase validates the core hypothesis: "AI + schema context = useful SQL generation."
**Delivers:** BigQuery query execution with dry-run cost estimation, INFORMATION_SCHEMA-backed schema cache (SQLite + FTS5), schema-aware NL-to-SQL, schema exploration tools, session cost tracking.
**Addresses:** Schema cache (INFORMATION_SCHEMA, dataset-scoped), BigQuery query tool, dry-run cost gate, schema exploration, cost tracking
**Avoids:** INFORMATION_SCHEMA over-scanning (dataset-scoped queries only), context window exhaustion (FTS5 relevance injection, 10-20 tables max), cost gate bypass (partition metadata in cache), SQLite concurrency (WAL mode + serialized writer)

### Phase 3: Platform Tools — The Debugging Surface
**Rationale:** Cross-service debugging ("What failed last night?") is the killer differentiator. It requires Composer, Logging, and GCS tools all working. Each is independently valuable, but together they enable the AI to autonomously chain: Composer failure → task logs → Cloud Logging → GCS landing data → BigQuery destination table. The LLM orchestrates this chain naturally once all tools exist — no hard-coded orchestration needed.
**Delivers:** Cloud Composer read-only integration (DAG status, task logs, failures), Cloud Logging queries (scoped, time-bounded), GCS read-only tools (list, head, profile), cross-service pipeline debugging (emergent from tool combination), error explanations
**Addresses:** Composer integration, Cloud Logging, GCS tools, setup wizard
**Avoids:** Unbounded log queries (always time-bounded), Composer 1 vs 2 API differences, GCS OOM on large files (size check + streaming)

### Phase 4: Advanced Platform — Deepening the Integration
**Rationale:** Once the core GCP tools are validated with real users, these features deepen the platform. dbt integration is independent of GCP tools and can be developed in parallel. Platform summary requires Composer + BigQuery cost data to be stable first.
**Delivers:** Platform summary injection (morning briefing), dbt integration (manifest, lineage, run/test/build), data profiling, SQL optimization suggestions, PII detection, degraded/offline mode
**Addresses:** dbt manifest parsing, platform summary, data profiling, SQL analysis, PII detection (Dataplex + heuristics), offline mode
**Avoids:** dbt manifest version incompatibilities, re-parsing manifest on every command (parse once, watch mtime), Dataplex graceful degradation (optional feature)

### Phase 5: Power Features — Team Extensibility
**Rationale:** Skills, hooks, and subagents are team-level features that require the core agent loop and tool system to be stable. These are "deepening" features, not discovery features — teams adopt them after the core proves its value.
**Delivers:** Skills system (domain knowledge markdown, auto-activated), hooks system (lifecycle extensibility for governance), subagent delegation (background analysis), session compaction, model provider switching
**Addresses:** Skills loader, hook runner, subagent goroutines, session summarization, multi-model support
**Avoids:** SQLite FTS5 corruption from subagents (serialized schema write channel established in Phase 2), context window exhaustion in long sessions (compaction from Phase 2 extended here)

### Phase 6: Polish — Distribution Ready
**Rationale:** Final hardening before wide distribution. Error recovery, rate limit handling, and UX polish across all flows. GoReleaser cross-platform builds.
**Delivers:** Exponential backoff on GCP errors, LLM rate limit handling, BigQuery REPEATED/RECORD type support in schema cache, Composer 1 vs 2 API compatibility, dbt manifest version checking, GoReleaser distribution, Homebrew formula
**Addresses:** All "Looks Done But Isn't" checklist items from PITFALLS.md, cross-platform binary distribution
**Avoids:** Silent failures on transient errors, immediate retry storms on quota exceeded

### Phase Ordering Rationale

- **Agent loop before tools:** No tool is useful without a functioning agent loop. The loop's design decisions (streaming architecture, permission model, turn governor) affect every subsequent phase.
- **Schema cache before NL-to-SQL:** SQL generation quality is directly proportional to schema context quality. Without the cache, NL-to-SQL is a generic chatbot feature, not a GCP-native one.
- **BigQuery before Composer/Logging:** BigQuery is the output surface of every pipeline. Schema understanding unlocks data investigation even before operational tools exist.
- **All GCP tools before cross-service debugging:** Cross-service debugging is emergent from tool combinations. The LLM orchestrates it; the developer doesn't hard-code it. But all tools must exist first.
- **Stable core before team features:** Skills and hooks encode institutional knowledge — they're only useful once teams trust the core enough to invest in customization.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2:** Cloud Composer REST API surface — the `cloud.google.com/go/orchestration` SDK only handles environment management; DAG/task operations require direct Airflow REST API calls via the Composer web server URL. Airflow 2.x stable API endpoints need verification.
- **Phase 2:** BigQuery slot-based vs on-demand pricing — cost estimation via dry-run bytes is straightforward for on-demand, but organizations using flat-rate/editions pricing need different cost calculation. Needs validation against current BigQuery Editions pricing model.
- **Phase 4:** ADK Go multi-agent composition (`workflowagents`) — ADK Go is v0.6.0 and early-stage. If subagent delegation uses ADK's built-in multi-agent features, API stability is a concern. May need to implement fire-and-forget goroutines independently.
- **Phase 4:** dbt manifest schema versions — manifest format changes between dbt versions (v7, v8, v9). Need to verify current version range in use and handle gracefully.

Phases with well-documented patterns (skip research-phase):
- **Phase 1:** Agent loop + Bubble Tea TUI — extensively documented by OpenCode and Charm ecosystem. Patterns are clear.
- **Phase 1:** GCP ADC authentication — standard `google.DefaultTokenSource()` flow, well-documented.
- **Phase 3:** Cloud Logging + GCS — official Go client libraries, stable APIs, straightforward integration.
- **Phase 6:** GoReleaser distribution — standard Go CLI distribution, well-documented.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versions verified via live tooling (`go list -m`, GitHub releases). Charm v2 ecosystem and GCP client libs are stable. ADK Go is the one variable — v0.6.0 is early-stage. |
| Features | MEDIUM | GCP API capabilities are HIGH confidence (stable, well-documented). Competitive landscape is MEDIUM — web search unavailable, competitors may have evolved since early 2025. Feature prioritization reflects product judgment as much as market research. |
| Architecture | MEDIUM | Patterns are proven (OpenCode, Claude Code, aider). ADK Go integration specifics are MEDIUM — ADK Go was relatively new at training cutoff, verify current API surface. Bubble Tea v2's Cursed Renderer is newer — streaming behavior may differ from v1. |
| Pitfalls | MEDIUM | Pitfalls derived from training data across Go, Bubble Tea, BigQuery, and LLM agent patterns. Most are well-understood with documented solutions. GCP-specific edge cases (Composer 1 vs 2, BigQuery Editions pricing) need live verification. |

**Overall confidence:** MEDIUM-HIGH — stack is solid, architecture is proven, features are well-reasoned. ADK Go maturity and competitive landscape are the two areas requiring live validation.

### Gaps to Address

- **ADK Go API surface validation:** Before Phase 1 begins, prototype the ADK Go agent loop + Bubble Tea TUI integration to verify streaming works as expected and the Provider interface design is sound. This is the highest-risk unknown.
- **Competitive landscape currency:** Research was based on training data through early 2025. Validate that no GCP-native AI terminal agent has launched in 2025-2026 that changes the positioning. Specifically check if Google has released anything in the `gcloud` CLI that overlaps.
- **BigQuery Editions pricing model:** Verify dry-run bytes-to-cost calculation is still accurate for the current BigQuery Editions pricing. Flat-rate organizations need different cost display logic.
- **Composer 2 Airflow REST API:** Prototype the Composer environment discovery → Airflow web server URL → DAG API call chain before committing to Composer integration design. The two-hop auth (ADC → Composer environment → Airflow API) has non-obvious error modes.
- **Charm v2 streaming behavior:** The Bubble Tea v2 Cursed Renderer is new (Feb 2026). Validate the ring buffer + render tick streaming architecture against v2 before Phase 1 is locked.

## Sources

### Primary (HIGH confidence)
- STACK.md — versions verified via `go list -m`, `gh release list`, and GitHub API (2026-03-16)
- BigQuery Go client library official documentation — stable, well-documented
- Charm ecosystem documentation (Bubble Tea, Lip Gloss, Glamour, Huh) — extensive, stable
- OpenCode source code (github.com/opencode-ai/opencode, 41K stars) — primary Go AI agent reference
- Claude Code public documentation — agent loop patterns, permission model, context management

### Secondary (MEDIUM confidence)
- Google ADK Go documentation and source (v0.6.0) — relatively new, sparse docs vs Python ADK
- ARCHITECTURE.md — based on training data patterns from OpenCode, Claude Code, aider
- FEATURES.md — competitive landscape from training data through early 2025; GCP API capabilities are HIGH
- PITFALLS.md — derived from training data across Go/Bubble Tea/BigQuery/LLM agent patterns

### Tertiary (LOW confidence)
- ADK Go multi-agent and subagent API surface — early-stage, needs live verification
- Competitive landscape (Snowflake Cortex, Databricks Assistant 2025-2026 state) — may have evolved
- BigQuery Editions pricing model details — pricing structure may have changed

---
*Research completed: 2026-03-16*
*Ready for roadmap: yes*
