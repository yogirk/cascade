# Cascade CLI — Development Roadmap

## Phase 0: Foundation (Weeks 1-4)

**Goal:** A working CLI agent that can chat, execute BigQuery queries, and read/write files.

### Deliverables
- [ ] Project scaffolding (Go module, Google ADK Go, Bubble Tea TUI)
- [ ] Agent loop with streaming output (Bubble Tea viewport + Glamour markdown)
- [ ] Core tools: Read, Write, Edit, Glob, Grep, Bash
- [ ] BigQueryQuery tool with dry-run cost estimation
- [ ] BigQuerySchema tool with basic schema introspection
- [ ] GCP authentication (ADC + service account impersonation)
- [ ] Basic permission model (READ_ONLY auto-approve, DML prompt)
- [ ] Interactive mode + one-shot mode (`-p` flag)
- [ ] Configuration file (`~/.cascade/config.toml`)
- [ ] CASCADE.md loading

### Tech Stack Decisions
- Go 1.23+ with goroutines for concurrency
- Google ADK Go for agent loop and tool framework
- Bubble Tea + Lip Gloss + Glamour + Bubbles + Huh for terminal UI
- anthropic-sdk-go + openai-go for multi-model support
- SQLite (modernc.org/sqlite, pure Go) for local caches
- TOML for configuration (BurntSushi/toml)
- GoReleaser for cross-platform binary distribution

### Exit Criteria
Can have a conversation, write SQL against BigQuery with cost awareness, and generate/edit local files (dbt models, SQL scripts).

---

## Phase 1: Platform Awareness (Weeks 5-8)

**Goal:** Deep GCP integration — schema cache, Composer, GCS, logging.

### Deliverables
- [ ] Schema cache (SQLite-backed, full/incremental/on-demand refresh)
- [ ] Schema-aware SQL generation (inject relevant schema into context)
- [ ] ComposerTool (list_dags, dag_status, task_logs, clear_task, trigger_dag)
- [ ] GCSTool (ls, head, profile, size)
- [ ] LoggingTool (query, tail, errors)
- [ ] Natural language schema search (FTS5 on schema cache)
- [ ] Autocomplete for table names, column names, DAG names
- [ ] `/sync` command for cache refresh
- [ ] `/failures` command for quick pipeline health check
- [ ] Platform summary injection (alerts, failures, cost status)

### Exit Criteria
Can debug pipeline failures end-to-end: identify failed DAG → read logs → find root cause → suggest fix. Schema-aware SQL generation works correctly.

---

## Phase 2: Data Engineering Workflows (Weeks 9-12)

**Goal:** First-class dbt, data profiling, cost analysis, and lineage.

### Deliverables
- [ ] DbtTool (run, test, build, compile, generate_model, show_lineage)
- [ ] dbt manifest parsing for dependency awareness
- [ ] DataProfiler tool (sample data, column stats, patterns)
- [ ] SQLAnalyzer tool (cost optimization, partition pruning, best practices)
- [ ] BigQueryCost tool (top queries, slot usage, storage costs, recommendations)
- [ ] DataplexTool (search, lineage, tags, quality)
- [ ] PipelineDebugger (automated failure diagnosis)
- [ ] `/cost` command for cost dashboard
- [ ] `/profile` command for data profiling
- [ ] `/lineage` command for lineage exploration
- [ ] `/explain` command for SQL execution plan analysis
- [ ] Cost gates (warn/block thresholds, daily budget)
- [ ] Session cost tracking

### Exit Criteria
Full dbt lifecycle works from Cascade. Can profile data, analyze costs, trace lineage, and provide optimization recommendations.

---

## Phase 3: Security & Production Hardening (Weeks 13-16)

**Goal:** Enterprise-ready security, sandboxing, audit logging.

### Deliverables
- [ ] OS-level sandboxing (macOS sandbox-exec, Linux bubblewrap)
- [ ] Network allowlisting (only *.googleapis.com by default)
- [ ] SQL risk classification engine (READ/DDL/DML/DESTRUCTIVE/ADMIN)
- [ ] Permission caching (per-project, per-tool, per-pattern)
- [ ] Blocklist for dangerous operations
- [ ] PII detection and masking (Dataplex tags + heuristics)
- [ ] Audit logging (append-only local log)
- [ ] Service account impersonation support
- [ ] Recommended IAM role documentation
- [ ] Security configuration guide

### Exit Criteria
Passes security review. PII columns are masked. Destructive operations require explicit confirmation. Audit trail captures all operations.

---

## Phase 4: Extensibility (Weeks 17-20)

**Goal:** Skills, hooks, subagents, MCP servers, and plugins.

### Deliverables
- [ ] Skills system (Markdown files with YAML frontmatter, auto-activation)
- [ ] Hooks system (PreToolUse, PostToolUse, PreSQLExecution, etc.)
- [ ] Built-in subagents (explorer, planner, log-analyst, cost-analyst, sql-writer)
- [ ] Custom subagent support (Markdown definition files)
- [ ] MCP server support (stdio + http transports)
- [ ] `cascade skill add/list/remove` commands
- [ ] `cascade mcp add/list/remove/test` commands
- [ ] Plugin pack format and installation
- [ ] Community skill repository scaffold
- [ ] Default skill packs (cost-optimization, dbt-conventions, pipeline-oncall)

### Exit Criteria
Users can extend Cascade with custom skills, hooks, subagents, and MCP servers. Default skill packs cover common data engineering patterns.

---

## Phase 5: Advanced Features (Weeks 21-24)

**Goal:** Streaming pipelines, Terraform, advanced multi-agent, and polish.

### Deliverables
- [ ] DataflowTool (list_jobs, metrics, drain, launch_template)
- [ ] PubSubTool (list_topics, peek_messages, subscription_lag, dead_letter)
- [ ] Terraform integration (plan, apply with approval, state inspection)
- [ ] Dataform integration (compile, run, assertions)
- [ ] Multi-agent orchestration (parallel subagent execution)
- [ ] Background tasks (long-running operations with notifications)
- [ ] Context compaction with data-engineering-specific preservation
- [ ] Session management (continue, resume, list, search)
- [ ] Memory system (persistent notes across sessions)
- [ ] Output formats (JSON, CSV, Markdown for piping)
- [ ] CI/CD mode (non-interactive, exit codes, structured output)

### Exit Criteria
Full coverage of GCP data engineering stack. Multi-agent workflows work. CI/CD integration is functional.

---

## Phase 6: Polish & Launch (Weeks 25-28)

**Goal:** Documentation, distribution, community, and launch.

### Deliverables
- [ ] Comprehensive documentation site
- [ ] Installation via Homebrew, standalone binary (GoReleaser), Docker, `go install`
- [ ] Getting Started guide with video walkthrough
- [ ] Cookbook: 20+ common data engineering recipes
- [ ] Performance optimization (startup time < 50ms, schema cache < 30s)
- [ ] Error messages and edge case handling
- [ ] Telemetry (opt-in, anonymized usage stats)
- [ ] Community Discord / GitHub Discussions
- [ ] Blog post: "Building Cascade: A Claude Code for Data Engineers"
- [ ] Launch on Product Hunt, Hacker News, r/dataengineering

### Exit Criteria
Public beta launch with documentation, distribution, and community channels.

---

## Future / Post-Launch

- **Cascade Cloud**: Managed version with team collaboration, shared sessions, and centralized governance
- **IDE Extensions**: VS Code and JetBrains plugins with inline SQL completion
- **Slack/Teams Bot**: `@cascade what failed last night?` in team channels
- **Mobile App**: Check pipeline health from your phone
- **Multi-Cloud**: AWS (Athena, Glue, Redshift) and Azure (Synapse, ADF) support
- **Data Contracts**: Generate and validate data contracts from schema + tests
- **ML Pipeline Support**: Vertex AI model training and serving integration
- **Observability Dashboards**: Built-in web UI for cost/health/quality metrics

---

## Key Metrics

### Adoption
- GitHub stars
- Homebrew installs / binary downloads
- Weekly active users
- Session count and duration

### Quality
- Time-to-resolution for pipeline failures (target: 5min vs 30min manual)
- Cost savings identified per user per month
- SQL quality score (partition pruning, cost optimization)
- User satisfaction (NPS)

### Engagement
- Sessions per user per week
- Tools used per session
- Skills and plugins installed
- Community contributions (skills, plugins, MCP servers)
