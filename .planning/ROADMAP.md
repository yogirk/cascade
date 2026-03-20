# Roadmap: Cascade CLI

## Overview

Cascade is built inside-out: a working conversational agent first (loop + TUI + auth + permissions), then BigQuery as the primary data surface (schema cache is the quality foundation for everything), then platform tools that enable cross-service debugging (the killer differentiator), then data engineering depth (dbt, profiling, SQL analysis), then team extensibility (skills, hooks, subagents), and finally distribution polish. Each phase delivers a complete, verifiable capability that builds trust and unblocks the next.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - Working conversational agent with GCP auth, permissions, TUI, and core file tools (completed 2026-03-17)
- [x] **Phase 1.1: TUI Excellence** - Claude Code/Codex-grade terminal experience for conversation flow, tool UX, approvals, and operational clarity (completed 2026-03-20)
- [ ] **Phase 2: BigQuery Core** - Schema-aware data surface with query execution, cost estimation, and NL-to-SQL (gap closure in progress)
- [ ] **Phase 3: Platform Tools** - GCP operational surface with Composer, Logging, GCS, and cross-service debugging
- [ ] **Phase 4: Data Engineering** - dbt integration, data profiling, SQL analysis, platform summary, PII detection, and offline mode
- [ ] **Phase 5: Extensibility** - Skills, hooks, subagents, and MCP integration for team customization
- [ ] **Phase 6: Distribution** - Setup wizard, binary distribution, slash commands, output formats, and project config

## Phase Details

### Phase 1: Foundation
**Goal**: User can have a streaming conversation with an AI agent in the terminal that authenticates to GCP, executes file tools, and enforces risk-based permissions
**Depends on**: Nothing (first phase)
**Requirements**: AGNT-01, AGNT-02, AGNT-03, AGNT-04, AGNT-05, AGNT-07, AUTH-01, AUTH-02, AUTH-03, AUTH-04, AUTH-05, AUTH-07, UX-01, UX-04
**Success Criteria** (what must be TRUE):
  1. User can launch `cascade`, type a question, and see streaming LLM output rendered with markdown formatting and syntax highlighting in the terminal
  2. User can execute core file operations (read, write, edit, glob, grep, bash) through conversational requests and see results inline
  3. User can authenticate via `gcloud auth application-default login` with zero additional setup, and sessions survive the 1-hour ADC token refresh transparently
  4. READ_ONLY tool calls execute without prompts; DML and above require explicit user confirmation before execution
  5. User can run `cascade -p "list files in current dir"` for one-shot scripting output, or launch interactive mode by default
**Plans**: 4 plans

Plans:
- [x] 01-01-PLAN.md -- Project scaffold, core types, config, auth, and LLM provider
- [x] 01-02-PLAN.md -- Tool system (6 core tools) and permission engine
- [x] 01-03-PLAN.md -- Agent loop, governor, session management, and app wiring
- [x] 01-04-PLAN.md -- Bubble Tea TUI, one-shot runner, and Cobra CLI entry point

### Phase 2: BigQuery Core
**Goal**: User can explore their warehouse schema, execute queries with cost awareness, and generate SQL from natural language with high accuracy due to schema context
**Depends on**: Phase 1.1
**Requirements**: BQ-01, BQ-02, BQ-03, BQ-04, BQ-05, BQ-06, BQ-07, BQ-08, BQ-09, AGNT-06
**Success Criteria** (what must be TRUE):
  1. User can ask "show me tables in the analytics dataset" and see formatted schema information including columns, partitioning, and clustering
  2. User sees estimated cost (bytes and dollars) before every query executes, with confirmation required for queries above a configurable threshold
  3. User can describe a query in natural language and receive accurate SQL that references correct table and column names from their warehouse
  4. User can see a running session cost total (BigQuery bytes + LLM tokens) and receives alerts when approaching configured budget limits
  5. Schema cache refreshes in the background without blocking the session, and user can force refresh via `/sync`
**Plans**: 7 plans

Plans:
- [x] 02-01-PLAN.md -- Config extensions, BQ client, SQL classifier, schema cache (SQLite+FTS5), cost tracker
- [x] 02-02-PLAN.md -- Session context compaction (AGNT-06) with auto-trigger and /compact command
- [x] 02-03-PLAN.md -- BigQuery tools (Query, Schema, Cost) with Lipgloss table rendering
- [x] 02-04-PLAN.md -- TUI integration: BQ styles, status bar cost, /cost and /sync commands
- [x] 02-05-PLAN.md -- App assembly wiring, dynamic SQL risk, system prompt injection, end-to-end verification
- [ ] 02-06-PLAN.md -- Gap closure: CostUpdateEvent emission wiring and lazy cache prompt update
- [ ] 02-07-PLAN.md -- Gap closure: SQL optimization analysis (BQ-09) with partition/clustering/JOIN hints

### Phase 1.1: TUI Excellence
**Goal**: User can operate Cascade through a calm, trustworthy, efficient terminal interface that matches the interaction quality of Claude Code/Codex while staying tailored to GCP data engineering workflows
**Depends on**: Phase 1
**Requirements**: UX-04, UX-05, UX-06, AGNT-03, AGNT-05
**Success Criteria** (what must be TRUE):
  1. Streaming output is lossless, visually stable, and never drops assistant text during long responses or tool-heavy turns
  2. Tool activity is rendered as high-signal operational UI with intent, progress, outcome, and errors, rather than raw log spam
  3. Permission prompts provide enough context to make safe decisions quickly, including risk, impact, and preview information
  4. Input ergonomics support fast expert usage through history, slash-command discovery, and clear mode/status awareness
  5. The TUI surfaces session state that matters for trust: model, permission mode, active work, pending approvals, project context, and eventually session cost/cache freshness hooks
  6. The interface establishes a reusable UI contract for upcoming BigQuery and platform-native views without needing a TUI rewrite in Phase 2
**Plans**: 3 plans (consolidated from 5 originally planned)

Plans:
- [x] 01.1-01-PLAN.md -- Lossless streaming, TurnStartEvent, tool output formatting, input history
- [x] 01.1-02-PLAN.md -- Approval UX redesign, slash commands, status bar refinements
- [x] 01.1-03-PLAN.md -- Visual polish, adaptive colors, terminal width resilience, interaction tests

Additional work completed beyond plans (review-driven):
- Bug fixes: idle tick loop, pre-confirm state restore, panic recovery, layout optimization
- Framework: lipgloss borders for input, pre-computed badge styles, deduplicated ToolBullet
- UX: confirm left-accent border, native text selection, steady cursor (no blink)

### Phase 3: Platform Tools
**Goal**: User can investigate pipeline failures across Composer, Cloud Logging, GCS, and BigQuery through a single conversational session without switching tools
**Depends on**: Phase 2
**Requirements**: PLAT-01, PLAT-02, PLAT-03, PLAT-04, AGNT-08
**Success Criteria** (what must be TRUE):
  1. User can ask "what DAGs failed last night?" and see Composer DAG status, run history, and task-level failure details
  2. User can ask "show me errors from the ingestion pipeline" and see relevant Cloud Logging entries scoped by time and service
  3. User can ask "what landed in gs://bucket/path today?" and see GCS file listings with the ability to preview file contents
  4. User can ask "why did the orders pipeline fail?" and the agent autonomously chains across Composer task logs, Cloud Logging entries, GCS landing files, and BigQuery destination tables to diagnose the issue
**Plans**: TBD

Plans:
- [ ] 03-01: TBD
- [ ] 03-02: TBD

### Phase 4: Data Engineering
**Goal**: User can work with dbt projects, profile data quality, get SQL optimization advice, receive a platform morning briefing, and operate safely around sensitive data
**Depends on**: Phase 3
**Requirements**: DATA-01, DATA-02, DATA-03, DATA-04, DATA-05, DATA-06, PLAT-05, PLAT-06, AUTH-06
**Success Criteria** (what must be TRUE):
  1. User can ask "show me the lineage for the orders model" and see dbt model dependencies rendered in the terminal, and can run dbt commands (run, test, build) through Cascade
  2. User can ask "profile the customers table" and see null rates, cardinality, value distributions, and outlier detection results
  3. User can ask "how can I optimize this query?" and receive concrete suggestions about partition filters, clustering key usage, and expensive JOINs
  4. On startup, user sees a platform summary with failed DAGs, cost anomalies, and data freshness alerts without asking
  5. User receives a warning before any query touches columns flagged as PII via Dataplex tags, column name heuristics, or data pattern matching
**Plans**: TBD

Plans:
- [ ] 04-01: TBD
- [ ] 04-02: TBD
- [ ] 04-03: TBD

### Phase 5: Extensibility
**Goal**: Teams can customize Cascade with domain knowledge, governance hooks, background analysis, and external tool connections
**Depends on**: Phase 4
**Requirements**: EXT-01, EXT-02, EXT-03, EXT-04
**Success Criteria** (what must be TRUE):
  1. User can place markdown files in `.cascade/skills/` and the agent automatically activates relevant domain knowledge based on conversation context
  2. User can configure lifecycle hooks (PreToolUse, PostToolUse, PreSQLExecution) that execute scripts for team governance enforcement
  3. User can trigger background analysis tasks (log analysis, cost analysis) that run as subagents and return summaries to the main session without blocking
  4. User can connect external tools via MCP servers and use them through the same conversational interface
**Plans**: TBD

Plans:
- [ ] 05-01: TBD
- [ ] 05-02: TBD

### Phase 6: Distribution
**Goal**: User can install Cascade as a single binary, run through first-time setup, and rely on polished UX across all workflows
**Depends on**: Phase 5
**Requirements**: UX-02, UX-03, UX-05, UX-06, UX-07
**Success Criteria** (what must be TRUE):
  1. User can install Cascade via `brew install cascade`, `go install`, or download a prebuilt binary for their platform (linux/darwin/windows x amd64/arm64)
  2. First-run setup wizard detects GCP project, available datasets, Composer environments, and dbt project, then builds initial schema cache with progress feedback
  3. User can use slash commands (/help, /compact, /cost, /failures, /lineage, /profile, /dbt, etc.) for quick access to common operations
  4. User can pipe output in JSON, CSV, or Markdown formats for scripting and CI integration alongside the default rich terminal output
  5. User can place a `CASCADE.md` file in their repo root for project-specific context and conventions shared across the team
**Plans**: TBD

Plans:
- [ ] 06-01: TBD
- [ ] 06-02: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 -> 1.1 -> 2 -> 3 -> 4 -> 5 -> 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 4/4 | Complete | 2026-03-17 |
| 1.1. TUI Excellence | 3/3 | Complete | 2026-03-20 |
| 2. BigQuery Core | 5/7 | Gap closure | - |
| 3. Platform Tools | 0/2 | Not started | - |
| 4. Data Engineering | 0/3 | Not started | - |
| 5. Extensibility | 0/2 | Not started | - |
| 6. Distribution | 0/2 | Not started | - |
