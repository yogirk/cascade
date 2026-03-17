# Feature Research

**Domain:** AI-native GCP data engineering terminal agent
**Researched:** 2026-03-16
**Confidence:** MEDIUM (training data through early 2025; web search unavailable for 2025-2026 ecosystem validation)

## Competitive Landscape Context

Before categorizing features, it helps to understand what exists and what Cascade is competing against in users' minds:

**Direct competitors (AI + data engineering CLI):**
- **Snowflake Cortex Analyst / Arctic CLI** -- Snowflake-native AI assistant for SQL and data exploration. Snowflake-only.
- **Databricks Assistant** -- In-notebook AI for Spark/SQL. Databricks-only, not CLI.
- **Vanna.ai** -- Open-source text-to-SQL with schema training. Python, notebook-oriented.
- **SQLCoder / Defog** -- Fine-tuned models for text-to-SQL. Model, not a tool.
- **Amazon Q Developer (formerly CodeWhisperer/Fig)** -- AI terminal assistant with AWS integration. AWS-only.

**Adjacent competitors (AI coding CLI):**
- **Claude Code** -- The gold standard for AI terminal agents. Codebase-aware, tool-based, but zero data platform awareness.
- **Aider** -- Git-aware AI pair programmer. Code-focused, no data platform.
- **OpenCode** -- Go + Charm TUI AI coding agent (same tech stack as Cascade). Code-focused.
- **Cursor / Continue / Cody** -- IDE-based AI assistants. Not terminal-native.

**Traditional tools Cascade replaces/unifies:**
- `bq` CLI, `gcloud` CLI, `gsutil`, Airflow CLI, dbt CLI, Cloud Logging viewer, BigQuery Console, Composer Console

**Key insight:** No tool combines AI conversation + GCP data platform awareness + terminal-native UX. Snowflake and Databricks have walled-garden AI, but GCP has nothing comparable. Claude Code proves the agent-in-terminal pattern works, but lacks domain specialization.

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these means the product feels broken or incomplete. These are informed by what Claude Code, `bq` CLI, and dbt CLI already provide -- users will compare Cascade against their existing tools.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Natural language to SQL** | The defining feature of any AI data tool. Users type "show me revenue by region last quarter" and get working SQL. Without this, Cascade has no reason to exist. | MEDIUM | Requires schema context injection. Quality depends heavily on schema cache completeness. Error retry with model fallback is essential -- first-attempt SQL accuracy is ~70-80% even with good context. |
| **BigQuery query execution** | `bq query` is the most-used data engineering command. Cascade must execute queries and display results in a readable format. | MEDIUM | Must handle large result sets gracefully (pagination, truncation). Streaming results for long queries. Support both standard and legacy SQL (warn on legacy). |
| **Dry-run cost estimation before queries** | Data engineers are trained to fear surprise bills. BigQuery charges by bytes scanned. Every query tool worth using shows estimated cost before execution. | LOW | BigQuery API has native dry-run support. Display bytes scanned + estimated cost. This is a trust-builder -- without it, users will not let an AI run queries autonomously. |
| **Schema exploration** | `bq ls`, `bq show`, `INFORMATION_SCHEMA` queries are daily tasks. Users need to browse datasets, describe tables, find columns. An AI tool that cannot answer "what tables do we have?" is useless. | MEDIUM | List datasets, list tables in dataset, describe table schema, search columns by name/type. FTS5 on cached schema enables fuzzy matching. |
| **GCP authentication via ADC** | Every GCP tool uses Application Default Credentials. Users expect `gcloud auth application-default login` to just work. Custom auth = instant rejection. | LOW | Use standard Google Cloud Go SDK auth. Support service account impersonation for elevated access. Zero custom auth code. |
| **Conversational interface with context** | Claude Code, ChatGPT, and every AI tool have trained users to expect multi-turn conversation where the AI remembers what you said 5 messages ago. | HIGH | Session management, context window management, compaction for long sessions. The agent loop (observe-reason-select tool-execute) is the core architecture. |
| **Readable output formatting** | `bq` outputs ugly tables. Users expect markdown tables, syntax-highlighted SQL, colored status indicators. Modern CLIs (gh, docker) set the bar. | MEDIUM | Glamour for markdown rendering, Lip Gloss for styling. Support multiple output formats (terminal, JSON, CSV, markdown) for piping. |
| **Interactive and one-shot modes** | Claude Code has both. CI/CD pipelines need `-p "run this query"`. Humans want interactive conversation. Both are required. | LOW | Interactive mode as default, `-p` flag for one-shot. One-shot must handle auth, execute, output, exit cleanly. |
| **Error explanations** | When a BigQuery query fails, users expect the AI to explain the error, not just dump a stack trace. This is the minimum bar for "AI-powered." | LOW | Parse BigQuery error messages (which are actually quite good), add context about likely cause and fix. |
| **Permission/safety controls** | Data engineers work with production data. An AI that can `DROP TABLE` without confirmation is terrifying. Permission model is not a feature -- it is a requirement. | MEDIUM | Risk classification (READ_ONLY through DESTRUCTIVE), confirmation prompts, PLAN mode for read-only exploration. Claude Code's permission model is the reference. |
| **Configuration file** | Every CLI tool has dotfiles. `~/.cascade/config.toml` for global, `CASCADE.md` for project-specific. Users expect to customize behavior without flags. | LOW | TOML for structured config, markdown for project context (like CLAUDE.md). Must support project detection (nearest CASCADE.md up the tree). |
| **Streaming LLM output** | Users expect to see the AI "thinking" in real-time. Waiting 10 seconds for a complete response with no feedback feels broken in 2026. | MEDIUM | SSE/streaming from LLM API, token-by-token rendering. Bubble Tea handles this well with its message-based architecture. |
| **Session cost tracking** | Users need to know how much their Cascade session is costing in BigQuery bytes scanned + LLM API calls. Surprise bills destroy trust. | LOW | Track bytes_scanned per query, LLM token usage per turn. Display running total. Configurable budget alerts. |

### Differentiators (Competitive Advantage)

Features that make Cascade uniquely valuable versus using Claude Code + `bq` CLI separately. These are what justify Cascade's existence as a specialized tool.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Cross-service pipeline debugging** | THE killer feature. "What failed last night?" triggers: check Composer for failed DAGs, pull task logs, query Cloud Logging for errors, inspect GCS landing files, check BigQuery target tables. No existing tool does this across services. A data engineer currently does this manually across 4-5 browser tabs. | HIGH | Requires tools for Composer, Cloud Logging, GCS, BigQuery all working together. The AI orchestrates the investigation flow. This is where the agent loop pattern shines -- the AI decides which service to check next based on what it finds. |
| **Schema-aware context injection** | Unlike generic AI tools, Cascade knows your warehouse schema. When you say "join customers with orders," it knows the column names, types, and likely join keys. This is the difference between ~60% and ~90% first-attempt SQL accuracy. | HIGH | Schema cache (SQLite + FTS5) from INFORMATION_SCHEMA. Inject relevant table schemas into LLM context for SQL generation. Must handle schema staleness (TTL, manual refresh). Dataset-scoped to avoid overwhelming context. |
| **Platform summary in system prompt** | On startup, Cascade tells you: "3 DAGs failed overnight, BigQuery costs up 40% from yesterday, 2 tables have freshness alerts." You get a morning briefing without asking. | MEDIUM | Query Composer for failures, BigQuery for cost deltas, Dataplex for alerts. Inject summary into system prompt. Must be fast (<5 seconds) or done async. |
| **dbt integration (manifest-aware)** | Cascade understands your dbt project: model dependencies, test results, source freshness. "Why did the orders model fail?" knows to check upstream sources, recent test failures, and the model SQL. | HIGH | Parse dbt manifest.json and catalog.json. Execute dbt commands (run, test, build). Generate new models from natural language. Lineage visualization in terminal. |
| **Data profiling on demand** | "Profile the customers table" returns null rates, cardinality, value distributions, outlier detection. Currently requires separate tools (Great Expectations, dbt-profiler, manual queries). | MEDIUM | Generate and execute profiling SQL (COUNT, COUNT DISTINCT, MIN, MAX, percentiles, NULL counts). Format results readably. Can be a subagent task for large tables. |
| **Cost-awareness by default** | Every query shows cost before execution. Cascade can suggest query optimizations ("add a partition filter to save 90% cost"). It understands BigQuery pricing model (on-demand vs slots, storage tiers). | MEDIUM | Dry-run for cost estimation, query plan analysis for optimization suggestions, historical cost tracking per session. Distinguishes this from "dumb" query tools. |
| **PII detection and governance** | Cascade warns when query results might contain PII. Uses Dataplex tags, column name heuristics (email, ssn, phone), and data pattern matching. Data engineers need this for compliance. | MEDIUM | Three-layer detection: Dataplex policy tags (if available), column name heuristics, regex pattern matching on results. Warn, do not block (user decides). |
| **Cloud Composer/Airflow deep integration** | List DAGs, check run status, read task logs, find failures, understand dependencies. Currently requires Airflow UI or clunky `gcloud composer` commands. | HIGH | Composer REST API for DAG management, task instance queries, log retrieval. The Airflow API surface is large -- focus on read operations first (status, logs, failures). |
| **Subagent delegation** | Fire off background analysis: "analyze our cost trends while I work on this query." Subagent runs in parallel, returns summary when done. Like having a junior analyst. | HIGH | Goroutines with isolated LLM context. Results pushed back to main session. Must handle failures gracefully. Keep V1 simple: fire-and-forget, no orchestration. |
| **Skills system (domain knowledge)** | Markdown files that inject domain-specific knowledge: "Our revenue table uses fiscal quarters starting in February." Auto-activated based on context. Like Claude Code's CLAUDE.md but for data domains. | MEDIUM | Skill files in `.cascade/skills/`. Matched by keyword/topic. Injected into LLM context when relevant. Lets teams encode institutional knowledge. |
| **SQL analysis and optimization** | Beyond just running queries: analyze query plans, suggest partition/cluster key improvements, identify expensive JOINs, recommend materialized views. | MEDIUM | Parse BigQuery query plan (from EXPLAIN or job metadata). Compare estimated vs actual bytes. Suggest concrete optimizations with cost impact estimates. |
| **Degraded/offline mode** | When GCP APIs are slow or down, Cascade still works with cached schema for SQL generation, local dbt operations, and cached metadata. | LOW | SQLite schema cache enables SQL generation without API calls. Detect API failures gracefully. Display staleness warnings. |
| **Hooks system (lifecycle extensibility)** | PreToolUse, PostToolUse, PreSQLExecution hooks let teams enforce custom policies: "all queries must include a partition filter," "DDL requires ticket number in comment." | MEDIUM | Lifecycle hook points, script execution (bash/Go), pass/fail/modify semantics. Follows Claude Code's hooks pattern. Enables team governance without forking. |

### Anti-Features (Deliberately NOT Building)

Features that seem appealing but would hurt the product, dilute focus, or create maintenance nightmares.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Multi-cloud support (AWS/Azure)** | "Support Redshift/Snowflake/Synapse too." Broader market. | Dilutes the deep GCP integration that IS the product. Each cloud has fundamentally different APIs, auth models, pricing. Doing 3 clouds at 30% depth loses to doing 1 cloud at 95% depth. Snowflake and Databricks already have their own AI tools. | GCP-first, GCP-only for V1. If the pattern works, multi-cloud is a V2+ consideration with provider abstraction already in place. |
| **GUI/web dashboard** | "I want to see results in a browser." Visual data exploration. | Cascade is a terminal tool. Adding a web UI doubles the frontend surface, introduces browser auth, deployment, and maintenance. Competes with BigQuery Console, Looker, and a thousand BI tools. | Excellent terminal rendering via Glamour/Lip Gloss. Export to CSV/JSON for users who want GUI tools. Consider `--serve` for result sharing later (not V1). |
| **IDE extension (VS Code/JetBrains)** | "I want Cascade in my editor." Developers live in IDEs. | Data engineers live in terminals + SQL editors + Airflow UI. IDE extensions have their own lifecycle, API constraints, and maintenance burden. Claude Code/Continue/Cody already own this space. | Terminal-first. IDE users can use Cascade in integrated terminal. |
| **Automated pipeline creation/scheduling** | "Write me a DAG and deploy it." End-to-end automation. | Creating and deploying pipelines requires understanding CI/CD, testing, staging environments. An AI auto-deploying to production Composer is terrifying. Too much blast radius for V1. | Generate DAG code locally (write to file). User reviews, tests, and deploys through their existing CI/CD. Read-first strategy. |
| **Real-time data streaming tools (Dataflow/Pub/Sub)** | "I need to debug my streaming pipeline." Streaming is part of data engineering. | Streaming pipelines have fundamentally different debugging patterns (backlog, watermarks, windowing). Adds enormous complexity. Small percentage of data engineering work. | Acknowledge streaming exists, defer to `gcloud dataflow` for V1. Phase 2+ consideration if demand exists. |
| **Plugin marketplace** | "Let the community build extensions." Ecosystem growth. | Marketplaces need review, security scanning, versioning, hosting, discovery. Massive investment for uncertain return. | Local skills + hooks system provides extensibility without marketplace overhead. Community shares via git repos. |
| **Built-in visualization/charting** | "Show me a chart of revenue over time." Data viz is expected. | Terminal charting is limited (ASCII charts are ugly, sixel support is spotty). Real visualization needs a browser. Adding a chart library is scope creep. | Format data as clean tables. Export to CSV for visualization in proper tools. Consider ASCII sparklines for trends (very low complexity). |
| **Slack/Teams bot** | "I want to ask questions in Slack." Team collaboration. | Different product entirely: webhook handling, message formatting, auth delegation, rate limiting, always-on hosting. | Cascade is a personal terminal tool. Teams share via skills files, not chat bots. |
| **Schema-aware autocomplete/intellisense** | "Tab-complete table and column names." IDE-like experience. | Requires stable, always-current schema cache + complex TUI input handling. Bubble Tea input is not an editor component. High complexity, moderate value. | AI handles the "autocomplete" -- describe what you want in natural language, get correct SQL. Slash commands for quick schema lookup (`/tables`, `/columns`). Phase 2+. |
| **Terraform apply/infrastructure management** | "Manage my BigQuery datasets via Terraform." IaC integration. | Terraform state management is complex and dangerous. Apply operations can destroy infrastructure. | Read-only Terraform plan analysis is fine ("what would this change?"). Actual apply goes through existing IaC pipelines. |
| **Multi-user/team features** | "Shared sessions, team dashboards, access control." Collaboration. | Requires a server, user management, shared state. Cascade Cloud territory. | Single-user tool. Teams collaborate via shared CASCADE.md, skills files, and git. |

## Feature Dependencies

```
[GCP Authentication (ADC)]
    |
    +---> [BigQuery Query Execution]
    |         |
    |         +---> [Dry-Run Cost Estimation]
    |         +---> [Data Profiling]
    |         +---> [SQL Analysis & Optimization]
    |         +---> [Session Cost Tracking]
    |
    +---> [Schema Cache (INFORMATION_SCHEMA + SQLite/FTS5)]
    |         |
    |         +---> [Schema Exploration]
    |         +---> [Schema-Aware Context Injection]
    |         |         |
    |         |         +---> [Natural Language to SQL] (quality depends on schema context)
    |         |
    |         +---> [PII Detection (column heuristics)]
    |         +---> [Degraded/Offline Mode]
    |
    +---> [Cloud Composer API Integration]
    |         |
    |         +---> [DAG Status / Failure Detection]
    |         +---> [Task Log Retrieval]
    |
    +---> [Cloud Logging API Integration]
    |
    +---> [GCS Read-Only Tools]

[Agent Loop (LLM + Tool Dispatch)]
    |
    +---> [Conversational Interface]
    |         |
    |         +---> [Session Management + Compaction]
    |         +---> [Streaming LLM Output]
    |
    +---> [Tool System (registration, dispatch, result handling)]
    |         |
    |         +---> [All GCP Tools above]
    |         +---> [File Tools (Read, Write, Edit, Glob, Grep, Bash)]
    |         +---> [Hooks System (lifecycle extensibility)]
    |
    +---> [Permission Engine (risk classification)]
    |         |
    |         +---> [Confirmation Prompts]
    |         +---> [Plan Mode (read-only)]
    |
    +---> [Subagent Delegation]

[Bubble Tea TUI]
    |
    +---> [Interactive Mode]
    +---> [Readable Output Formatting]
    +---> [Keyboard Shortcuts]
    +---> [Slash Commands]

[Configuration System (TOML + CASCADE.md)]
    |
    +---> [Skills System]
    +---> [Model Provider Selection]
    +---> [Cost Budget Configuration]

[Composer + Logging + GCS + BigQuery Tools]
    |
    +---> [Cross-Service Pipeline Debugging] (the orchestration layer)

[dbt Project Detection]
    |
    +---> [Manifest Parsing]
    |         +---> [Lineage Awareness]
    |         +---> [Model Generation]
    +---> [dbt Run/Test/Build Execution]

[Schema Cache + Composer Failures + Cost Queries]
    |
    +---> [Platform Summary Injection]
```

### Dependency Notes

- **Schema-Aware Context Injection requires Schema Cache:** SQL generation quality is directly proportional to schema context quality. The schema cache must be built before text-to-SQL is useful. This is the single most important dependency.
- **Cross-Service Pipeline Debugging requires all GCP tool integrations:** This is the capstone feature. It only works when Composer, Logging, GCS, and BigQuery tools are all functional. Must be built last among GCP integrations.
- **Platform Summary requires multiple data sources:** Needs Composer failure data, BigQuery cost data, and optionally Dataplex alerts. Built after individual integrations are stable.
- **Natural Language to SQL requires both Agent Loop and Schema Cache:** The agent loop provides the conversation; the schema cache provides the context. Both must be functional for the core use case to work.
- **Subagents require a stable Agent Loop:** Cannot delegate to subagents until the main agent loop is reliable. Subagents are an enhancement, not a foundation.
- **dbt integration is independent of GCP tools:** dbt operates locally (manifest parsing, command execution). Can be developed in parallel with GCP integrations.
- **Hooks System requires Tool System:** Hooks intercept tool lifecycle events. The tool dispatch system must be stable before hooks can be layered on.

## MVP Definition

### Launch With (v1.0) -- The "What Failed Last Night?" Release

Minimum viable product that validates the core hypothesis: a conversational GCP data engineering agent is more productive than switching between `bq`, `gcloud`, Airflow UI, and Cloud Logging.

- [ ] **Agent loop with streaming output** -- The foundation. Observe-reason-select tool-execute cycle with real-time token rendering.
- [ ] **GCP authentication via ADC** -- Zero-friction auth. `gcloud auth application-default login` and go.
- [ ] **BigQuery query execution with dry-run cost estimation** -- The most-used operation. Show cost, confirm, execute, display results.
- [ ] **Schema cache from INFORMATION_SCHEMA** -- SQLite + FTS5. Dataset-scoped. The quality foundation for everything else.
- [ ] **Schema-aware natural language to SQL** -- The headline feature. "Show me the top 10 customers by revenue" generates correct SQL.
- [ ] **Schema exploration** -- List datasets, describe tables, search columns. The "what do we have?" workflow.
- [ ] **Cloud Composer integration (read-only)** -- List DAGs, check status, read task logs, find failures. The "what broke?" workflow.
- [ ] **Cloud Logging queries** -- Search logs for errors related to pipeline failures. Completes the debugging loop.
- [ ] **GCS read-only tools** -- List files, head files, profile landing data. Needed for pipeline debugging.
- [ ] **Permission engine with risk classification** -- READ_ONLY through DESTRUCTIVE. Confirmation prompts for writes. Trust by default.
- [ ] **Interactive and one-shot modes** -- Conversational TUI + `-p` flag for scripting.
- [ ] **Session cost tracking** -- Running total of bytes scanned and LLM tokens. Budget alerts.
- [ ] **Configuration system** -- `~/.cascade/config.toml` + `CASCADE.md` project config.
- [ ] **Readable terminal output** -- Markdown rendering, syntax highlighting, formatted tables.
- [ ] **Error explanations** -- Parse BigQuery/Composer errors, explain in plain language, suggest fixes.
- [ ] **Setup wizard** -- First-run detection of GCP project, datasets, Composer environment. Build initial schema cache.

### Add After Validation (v1.x) -- Deepening the Platform

Features to add once the core agent loop and GCP integrations are validated with real users.

- [ ] **Cross-service pipeline debugging orchestration** -- The AI autonomously chains: Composer failure -> task logs -> Cloud Logging -> GCS -> BigQuery. Trigger: core GCP tools are stable and users are manually doing this chain.
- [ ] **dbt integration** -- Manifest parsing, lineage, run/test/build, model generation. Trigger: users request dbt support or most early adopters use dbt.
- [ ] **Platform summary injection** -- Morning briefing in system prompt. Trigger: startup time is fast enough (<5s for summary generation).
- [ ] **Data profiling** -- Null rates, cardinality, distributions. Trigger: users request it or SQL generation needs data understanding.
- [ ] **SQL analysis and optimization** -- Query plan analysis, cost optimization suggestions. Trigger: users are running expensive queries and asking "why is this slow?"
- [ ] **PII detection** -- Column heuristics + Dataplex tags + data pattern matching. Trigger: enterprise/compliance-sensitive users adopt.
- [ ] **Skills system** -- Domain knowledge markdown files, auto-activated. Trigger: teams want to encode institutional knowledge.
- [ ] **Hooks system** -- Lifecycle extensibility for team governance. Trigger: teams need custom policies.
- [ ] **Subagent delegation** -- Background analysis tasks. Trigger: users want to parallelize investigation workflows.
- [ ] **Model provider switching** -- Claude, OpenAI, Ollama, Bifrost support. Trigger: users want model choice or Gemini is insufficient for some tasks.

### Future Consideration (v2+) -- Platform Expansion

Features to defer until product-market fit is established and the core is rock-solid.

- [ ] **Dataflow/Pub/Sub integration** -- Streaming pipeline debugging. Defer: fundamentally different patterns, small user overlap.
- [ ] **Schema-aware autocomplete** -- Tab-completion for tables/columns. Defer: requires stable schema cache + complex TUI work.
- [ ] **MCP server integration** -- External tool ecosystem. Defer: ecosystem maturity, unclear demand.
- [ ] **Multi-panel TUI** -- Concurrent subagent rendering. Defer: excessive complexity, V1 subagents are fire-and-forget.
- [ ] **Terraform plan analysis** -- Read-only IaC understanding. Defer: niche use case.
- [ ] **Dataplex deep integration** -- Data quality rules, lineage, discovery. Defer: Dataplex adoption is still growing.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Agent loop + streaming | HIGH | HIGH | P1 |
| GCP auth (ADC) | HIGH | LOW | P1 |
| BigQuery query execution | HIGH | MEDIUM | P1 |
| Dry-run cost estimation | HIGH | LOW | P1 |
| Schema cache (INFORMATION_SCHEMA) | HIGH | HIGH | P1 |
| Schema-aware NL-to-SQL | HIGH | MEDIUM | P1 |
| Schema exploration | HIGH | MEDIUM | P1 |
| Interactive + one-shot modes | HIGH | LOW | P1 |
| Permission engine | HIGH | MEDIUM | P1 |
| Readable output formatting | MEDIUM | MEDIUM | P1 |
| Session cost tracking | MEDIUM | LOW | P1 |
| Configuration system | MEDIUM | LOW | P1 |
| Error explanations | MEDIUM | LOW | P1 |
| Streaming LLM output | MEDIUM | MEDIUM | P1 |
| Setup wizard | MEDIUM | MEDIUM | P1 |
| Cloud Composer integration | HIGH | HIGH | P1 |
| Cloud Logging queries | HIGH | MEDIUM | P1 |
| GCS read-only tools | MEDIUM | LOW | P1 |
| Cross-service pipeline debugging | HIGH | HIGH | P2 |
| dbt integration | HIGH | HIGH | P2 |
| Platform summary injection | MEDIUM | MEDIUM | P2 |
| Data profiling | MEDIUM | MEDIUM | P2 |
| SQL analysis/optimization | MEDIUM | MEDIUM | P2 |
| PII detection | MEDIUM | MEDIUM | P2 |
| Skills system | MEDIUM | MEDIUM | P2 |
| Hooks system | MEDIUM | MEDIUM | P2 |
| Subagent delegation | MEDIUM | HIGH | P2 |
| Model provider switching | MEDIUM | MEDIUM | P2 |
| Degraded/offline mode | LOW | LOW | P2 |
| Dataflow/Pub/Sub | LOW | HIGH | P3 |
| Schema autocomplete | LOW | HIGH | P3 |
| MCP server integration | LOW | MEDIUM | P3 |
| Multi-panel TUI | LOW | HIGH | P3 |
| Terraform plan analysis | LOW | MEDIUM | P3 |

**Priority key:**
- P1: Must have for launch -- validates the core hypothesis
- P2: Should have, add after core is stable -- deepens the platform
- P3: Nice to have, future consideration -- expands scope

## Competitor Feature Analysis

| Feature | Claude Code | bq CLI | gcloud CLI | Snowflake Cortex | Databricks Asst. | Cascade Approach |
|---------|-------------|--------|------------|------------------|-------------------|-----------------|
| Natural language to SQL | Generic (no schema awareness) | None | None | Native (Snowflake-only) | Native (Spark-only) | Schema-aware, GCP-native |
| Query execution | Via bash tool | Native | N/A | Native | Native | Native BigQuery tool |
| Cost estimation | None | `--dry_run` flag | N/A | Credits estimate | DBU estimate | Automatic before every query |
| Schema exploration | Must query manually | `bq ls`, `bq show` | N/A | DESCRIBE | DESCRIBE | AI-powered search + FTS5 |
| Pipeline debugging | Manual across tools | N/A | Partial | Snowflake Tasks only | Databricks Jobs only | Cross-service (Composer+Logging+GCS+BQ) |
| dbt integration | Via file editing | None | None | Limited | dbt Cloud native | Manifest-aware, lineage, run/test |
| Airflow/orchestrator | None | N/A | `gcloud composer` | None | None | Deep Composer integration |
| Log analysis | None | N/A | `gcloud logging` | Query History | Spark UI | Cloud Logging tool |
| Data profiling | None | None | None | Via queries | Via notebooks | Built-in profiling tool |
| Permission model | Yes (excellent) | IAM only | IAM only | RBAC | RBAC | Risk-classified + IAM |
| Conversational | Yes | No | No | Yes | Yes | Yes, with platform context |
| Terminal-native | Yes | Yes | Yes | No (web) | No (notebook) | Yes |
| Offline/degraded | N/A | No | No | No | No | Yes (cached schema) |
| Cost tracking | Token usage | None | None | Credits | DBUs | BQ bytes + LLM tokens |
| Extensibility | CLAUDE.md + hooks | None | None | None | None | Skills + hooks + CASCADE.md |

## Sources

- Training data knowledge of GCP BigQuery, Cloud Composer, Cloud Logging, GCS, Dataplex APIs and CLI tools (HIGH confidence -- well-documented, stable APIs)
- Training data knowledge of Claude Code architecture and features (HIGH confidence -- extensively documented)
- Training data knowledge of Snowflake Cortex, Databricks Assistant capabilities (MEDIUM confidence -- features may have expanded since early 2025)
- Training data knowledge of dbt CLI, Great Expectations, SQLMesh (HIGH confidence -- mature, well-documented tools)
- Training data knowledge of OpenCode, Aider, and Go/Charm TUI ecosystem (MEDIUM confidence -- rapidly evolving space)
- Training data knowledge of Vanna.ai, SQLCoder, text-to-SQL landscape (MEDIUM confidence -- fast-moving research area)
- PROJECT.md for Cascade-specific requirements and constraints

**Confidence note:** Web search was unavailable during this research. All findings are based on training data through early 2025. The AI terminal agent space is evolving rapidly -- new competitors may have emerged in 2025-2026 that are not reflected here. Recommend validating competitive landscape with current web research before finalizing roadmap.

---
*Feature research for: AI-native GCP data engineering terminal agent*
*Researched: 2026-03-16*
