# Requirements: Cascade CLI

**Defined:** 2026-03-16
**Core Value:** A data engineer can diagnose pipeline failures, investigate costs, write queries, and manage their GCP data stack through one conversational interface that understands their warehouse schema, pipeline dependencies, and cost profile.

## v1 Requirements

Requirements for initial release. Each maps to roadmap phases.

### Agent Core

- [x] **AGNT-01**: User can have multi-turn conversational sessions with streaming LLM output rendered in real-time
- [x] **AGNT-02**: Agent executes a single-threaded observe-reason-select tool-execute loop with hard limit on tool calls per turn (15-20) to prevent infinite cycling
- [x] **AGNT-03**: User sees a Bubble Tea TUI with Lip Gloss styling, Glamour markdown rendering, syntax-highlighted SQL, and formatted tables
- [x] **AGNT-04**: Agent uses Google ADK Go with Gemini 2.5 Pro as default LLM, abstracted behind a Provider interface for future model switching
- [x] **AGNT-05**: User can run Cascade in interactive mode (default) or one-shot mode via `-p` flag for scripting and CI/CD
- [ ] **AGNT-06**: Agent manages session context with automatic compaction at 80% context window to support long sessions
- [x] **AGNT-07**: Core file tools available: Read, Write, Edit, Glob, Grep, Bash — same capabilities as Claude Code for code operations
- [ ] **AGNT-08**: Agent explains errors from BigQuery, Composer, and other GCP services in plain language with suggested fixes

### Authentication & Security

- [x] **AUTH-01**: User authenticates via GCP Application Default Credentials with zero custom auth — `gcloud auth application-default login` just works
- [x] **AUTH-02**: User can impersonate a service account for elevated access via config
- [x] **AUTH-03**: Permission engine classifies every tool call by risk level (READ_ONLY, DDL, DML, DESTRUCTIVE, ADMIN) and enforces appropriate confirmation
- [x] **AUTH-04**: READ_ONLY operations auto-approve silently; DML and above require explicit user confirmation
- [x] **AUTH-05**: User can cycle between permission modes: CONFIRM (default), PLAN (read-only), BYPASS (auto-approve all)
- [ ] **AUTH-06**: PII detection warns user before queries touch sensitive columns, using Dataplex tags, column name heuristics, and data pattern matching
- [x] **AUTH-07**: Auth token expiry handled transparently with retry-on-401 middleware — sessions survive the 1-hour ADC token refresh

### BigQuery

- [ ] **BQ-01**: User can execute BigQuery SQL queries and see results formatted as readable tables with smart truncation
- [ ] **BQ-02**: Every query shows estimated cost (bytes scanned + dollar estimate) via dry-run before execution, with user confirmation for expensive queries
- [ ] **BQ-03**: Schema cache built from INFORMATION_SCHEMA bulk queries, stored in SQLite with FTS5 full-text search, scoped to configured datasets
- [ ] **BQ-04**: User can explore schema: list datasets, list tables, describe table columns/partitioning/clustering, search columns by name or type via natural language
- [ ] **BQ-05**: Schema-aware context injection provides relevant table schemas to LLM for SQL generation, filtered by FTS5 relevance (10-20 tables max per call)
- [ ] **BQ-06**: User can generate SQL from natural language descriptions with ~90% first-attempt accuracy due to schema context
- [ ] **BQ-07**: Session cost tracking accumulates bytes scanned and LLM token usage with running total display and configurable budget alerts
- [ ] **BQ-08**: Schema cache refreshes incrementally in background, never blocks startup, with manual refresh via `/sync` command
- [ ] **BQ-09**: SQL analysis suggests query optimizations: partition filter recommendations, clustering key usage, expensive JOIN identification

### Platform Tools

- [ ] **PLAT-01**: User can list Cloud Composer DAGs, check run status, read task logs, and identify failures via natural language
- [ ] **PLAT-02**: User can query Cloud Logging with scoped, time-bounded queries to find errors and debug issues
- [ ] **PLAT-03**: User can browse GCS buckets and objects: list, head (preview first N lines), and profile landing data — read-only
- [ ] **PLAT-04**: Cross-service pipeline debugging chains Composer failure → task logs → Cloud Logging → GCS landing files → BigQuery destination tables, orchestrated by the LLM
- [ ] **PLAT-05**: Platform summary injection provides morning briefing on startup: failed DAGs, cost anomalies, freshness alerts — injected into system prompt
- [ ] **PLAT-06**: Degraded/offline mode uses cached schema for SQL generation when GCP APIs are unavailable, with staleness warnings

### Data Engineering

- [ ] **DATA-01**: dbt integration parses manifest.json for model dependencies, lineage, and metadata
- [ ] **DATA-02**: User can execute dbt commands (run, test, build) through Cascade with formatted output
- [ ] **DATA-03**: User can generate new dbt models from natural language descriptions using schema context
- [ ] **DATA-04**: User can view dbt lineage for any model in terminal
- [ ] **DATA-05**: Data profiling generates and executes profiling SQL (null rates, cardinality, distributions, outliers) for any table on demand
- [ ] **DATA-06**: SQL analysis evaluates query cost, performance, and correctness with concrete optimization suggestions

### Configuration & UX

- [x] **UX-01**: Global config via `~/.cascade/config.toml` with sections for project, model, cost, composer, dbt, cache, security, display
- [ ] **UX-02**: Project-specific config via `CASCADE.md` file in repo root (like CLAUDE.md) for team context and conventions
- [ ] **UX-03**: Setup wizard on first run detects GCP project, available datasets, Composer environments, dbt project, and builds initial schema cache
- [x] **UX-04**: Keyboard shortcuts: Ctrl+C (cancel), Ctrl+B (background), Shift+Tab (cycle permission modes), Ctrl+R (refresh cache), Ctrl+L (clear), Ctrl+D (exit)
- [ ] **UX-05**: Slash commands: /help, /compact, /plan, /sync, /cost, /failures, /lineage, /profile, /dbt, /config, /clear
- [ ] **UX-06**: Output formats: terminal (default with markdown/syntax highlighting), JSON, CSV, Markdown, quiet — for piping and scripting
- [ ] **UX-07**: Single static binary distribution via Homebrew, GoReleaser, and `go install` — cross-compiled for linux/darwin/windows x amd64/arm64

### Extensibility

- [ ] **EXT-01**: Skills system loads domain-specific markdown files from `.cascade/skills/` that auto-activate based on context matching
- [ ] **EXT-02**: Hooks system executes scripts at lifecycle points (PreToolUse, PostToolUse, PreSQLExecution, PostSQLExecution) for team governance
- [ ] **EXT-03**: Subagent delegation runs background analysis as fire-and-forget goroutines with isolated context, returning summaries to main session
- [ ] **EXT-04**: MCP server integration for external tool connectivity (GitHub, Slack, Fivetran, etc.)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced LLM

- **ADV-01**: Model routing — different models for different tasks (Gemini for SQL, Claude for reasoning)
- **ADV-02**: Multi-provider switching at runtime via slash command or config

### Streaming Pipelines

- **STRM-01**: Dataflow job management (list, status, metrics, drain)
- **STRM-02**: Pub/Sub topic management (list, publish test messages, read subscriptions)
- **STRM-03**: Streaming pipeline debugging (backlog, watermarks, windowing)

### Advanced UX

- **AUX-01**: Schema-aware tab autocomplete for table and column names
- **AUX-02**: Terraform plan analysis (read-only IaC understanding)
- **AUX-03**: Dataplex deep integration (data quality rules, lineage graph)
- **AUX-04**: Multi-panel TUI for concurrent subagent rendering

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Multi-cloud support (AWS/Azure) | Dilutes deep GCP integration; each cloud has fundamentally different APIs and auth |
| GUI/web dashboard | Terminal-first tool; web UI doubles frontend surface and competes with BigQuery Console |
| IDE extensions (VS Code/JetBrains) | Data engineers live in terminals; IDE AI space already crowded (Claude Code, Cursor, Cody) |
| Automated pipeline deployment | Too much blast radius — an AI auto-deploying DAGs to production is dangerous; write code locally, deploy via existing CI/CD |
| Plugin marketplace | Needs review, security scanning, versioning, hosting — massive investment for uncertain return; local skills suffice |
| Built-in charting/visualization | Terminal charting is limited; export to CSV for proper visualization tools |
| Slack/Teams bot | Different product entirely requiring webhooks, message formatting, always-on hosting |
| Multi-user/team features | Requires server, user management, shared state — Cascade Cloud territory |
| OS-level sandboxing | Maintenance nightmare that breaks legitimate tools; IAM + Permission Engine + Cost Gates sufficient for V1 |
| Telemetry to remote server | Local-only stats; remote telemetry erodes trust with security-conscious data engineers |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| AGNT-01 | Phase 1 | Complete |
| AGNT-02 | Phase 1 | Complete |
| AGNT-03 | Phase 1 | Complete |
| AGNT-04 | Phase 1 | Complete |
| AGNT-05 | Phase 1 | Complete |
| AGNT-06 | Phase 2 | Pending |
| AGNT-07 | Phase 1 | Complete |
| AGNT-08 | Phase 3 | Pending |
| AUTH-01 | Phase 1 | Complete |
| AUTH-02 | Phase 1 | Complete |
| AUTH-03 | Phase 1 | Complete |
| AUTH-04 | Phase 1 | Complete |
| AUTH-05 | Phase 1 | Complete |
| AUTH-06 | Phase 4 | Pending |
| AUTH-07 | Phase 1 | Complete |
| BQ-01 | Phase 2 | Pending |
| BQ-02 | Phase 2 | Pending |
| BQ-03 | Phase 2 | Pending |
| BQ-04 | Phase 2 | Pending |
| BQ-05 | Phase 2 | Pending |
| BQ-06 | Phase 2 | Pending |
| BQ-07 | Phase 2 | Pending |
| BQ-08 | Phase 2 | Pending |
| BQ-09 | Phase 2 | Pending |
| PLAT-01 | Phase 3 | Pending |
| PLAT-02 | Phase 3 | Pending |
| PLAT-03 | Phase 3 | Pending |
| PLAT-04 | Phase 3 | Pending |
| PLAT-05 | Phase 4 | Pending |
| PLAT-06 | Phase 4 | Pending |
| DATA-01 | Phase 4 | Pending |
| DATA-02 | Phase 4 | Pending |
| DATA-03 | Phase 4 | Pending |
| DATA-04 | Phase 4 | Pending |
| DATA-05 | Phase 4 | Pending |
| DATA-06 | Phase 4 | Pending |
| UX-01 | Phase 1 | Complete |
| UX-02 | Phase 6 | Pending |
| UX-03 | Phase 6 | Pending |
| UX-04 | Phase 1 | Complete |
| UX-05 | Phase 6 | Pending |
| UX-06 | Phase 6 | Pending |
| UX-07 | Phase 6 | Pending |
| EXT-01 | Phase 5 | Pending |
| EXT-02 | Phase 5 | Pending |
| EXT-03 | Phase 5 | Pending |
| EXT-04 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 47 total
- Mapped to phases: 47
- Unmapped: 0

---
*Requirements defined: 2026-03-16*
*Last updated: 2026-03-16 after roadmap creation*
