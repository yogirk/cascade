# Cascade CLI — Extensibility

## Extension Points

Cascade provides four extensibility mechanisms, mirroring Claude Code and Cortex CLI's proven patterns but tailored for data engineering workflows.

```
Extensibility
├── Skills (domain knowledge injection)
├── Hooks (lifecycle automation)
├── Subagents (specialized AI workers)
└── MCP Servers (external tool integration)
```

---

## 1. Skills

Skills are Markdown files that inject domain-specific knowledge into Cascade's context. They activate automatically when their description matches the user's task, or manually via `$skill-name` syntax.

### Skill Locations

```
.cascade/skills/          # Project-level (checked into repo)
~/.cascade/skills/        # User-level (personal)
```

### Example: Cost Optimization Skill

```markdown
<!-- .cascade/skills/cost-optimization.md -->
---
name: cost-optimization
description: BigQuery cost optimization patterns and rules for this project
triggers:
  - "cost"
  - "expensive"
  - "optimize"
  - "budget"
---

# Cost Optimization Rules

## Project-Specific Rules
- `raw_events` table: ALWAYS include partition filter on `event_date`
- `raw_clickstream`: Use `_TABLE_SUFFIX` for date-sharded tables
- Prefer `APPROX_COUNT_DISTINCT` over `COUNT(DISTINCT ...)` for events
- Use materialized views for any aggregation queried > 3x/day

## General BigQuery Cost Patterns
- SELECT only needed columns (never SELECT *)
- Filter on partition columns in the outermost query
- Use `--dry_run` flag before running exploratory queries
- Check `INFORMATION_SCHEMA.JOBS` for query cost patterns
- Consider BI Engine for sub-second dashboard queries

## Cost Thresholds (this project)
- Single query warning: > $1
- Single query block: > $25
- Daily budget: $200
- Alert channel: #data-costs in Slack
```

### Example: dbt Conventions Skill

```markdown
<!-- .cascade/skills/dbt-conventions.md -->
---
name: dbt-conventions
description: dbt modeling conventions and patterns for this warehouse
triggers:
  - "dbt"
  - "model"
  - "staging"
  - "mart"
---

# dbt Conventions

## Naming
- Staging: `stg_{source}_{entity}` (e.g., `stg_shopify_orders`)
- Intermediate: `int_{entity}_{verb}` (e.g., `int_orders_pivoted`)
- Facts: `fct_{entity}` (e.g., `fct_orders`)
- Dimensions: `dim_{entity}` (e.g., `dim_customers`)

## Model Structure
- All staging models SELECT from `{{ source() }}`, never raw table refs
- All marts SELECT from `{{ ref() }}` to other models
- Incremental models use `merge` strategy with unique_key
- Every model has a .yml file with description and tests

## Required Tests
- All primary keys: `unique` + `not_null`
- All foreign keys: `relationships`
- All enums: `accepted_values`
- Financial amounts: `dbt_utils.expression_is_true` for non-negative

## Materialization Defaults
- Staging: `view`
- Intermediate: `ephemeral`
- Facts/Dimensions: `incremental` (partitioned by date)
- Aggregations: `table` with clustering
```

### Sharing Skills

Skills can be installed from Git repos:

```bash
# Install a community skill pack
cascade skill add https://github.com/cascade-community/gcp-skills

# Install a specific skill
cascade skill add https://github.com/team/skills/cost-rules.md

# List installed skills
cascade skill list

# Remove a skill
cascade skill remove cost-rules
```

---

## 2. Hooks

Hooks are scripts that execute at specific lifecycle points, enabling automation and policy enforcement.

### Hook Types

| Event | When | Use Case |
|-------|------|----------|
| `PreToolUse` | Before any tool executes | Block dangerous operations, add context |
| `PostToolUse` | After any tool executes | Log operations, trigger alerts |
| `PreSQLExecution` | Before SQL runs in BigQuery | Cost gate, SQL linting, audit logging |
| `PostSQLExecution` | After SQL completes | Cost logging, result caching |
| `PreDbtRun` | Before dbt run/build/test | Validate target, check git status |
| `PostDbtRun` | After dbt completes | Notify Slack, update docs |
| `SessionStart` | When a session begins | Load context, check for active alerts |
| `SessionEnd` | When a session ends | Summarize, log costs, clean up |
| `PreCompact` | Before context compaction | Preserve critical context |
| `CostThreshold` | When cost exceeds threshold | Alert, block, or escalate |
| `PipelineFailure` | When a failure is detected | Auto-create incident, notify |

### Configuration

```json
// .cascade/settings.json
{
    "hooks": {
        "PreSQLExecution": [
            {
                "type": "command",
                "command": "./scripts/sql-lint.sh",
                "description": "Lint SQL before execution"
            },
            {
                "type": "prompt",
                "prompt": "Check if this SQL follows our cost optimization rules from the cost-optimization skill. Block if it will scan more than 100GB without a partition filter.",
                "description": "AI cost review"
            }
        ],
        "PostSQLExecution": [
            {
                "type": "command",
                "command": "./scripts/log-query-cost.sh",
                "description": "Log query cost to tracking DB"
            }
        ],
        "PostDbtRun": [
            {
                "type": "command",
                "command": "./scripts/notify-slack.sh",
                "description": "Post dbt run results to #data-builds"
            }
        ],
        "SessionStart": [
            {
                "type": "command",
                "command": "./scripts/check-active-alerts.sh",
                "description": "Check for active pipeline alerts"
            }
        ],
        "CostThreshold": [
            {
                "matcher": "daily_budget",
                "type": "command",
                "command": "./scripts/budget-alert.sh",
                "description": "Alert when daily budget is approaching"
            }
        ]
    }
}
```

### Hook Script Interface

Hook scripts receive context via environment variables and stdin:

```bash
#!/bin/bash
# scripts/sql-lint.sh

# Available env vars:
# CASCADE_TOOL: tool name (e.g., "BigQueryQuery")
# CASCADE_ACTION: action (e.g., "SELECT")
# CASCADE_PROJECT: GCP project ID
# CASCADE_SESSION: session ID
# CASCADE_USER: authenticated user

# SQL is passed via stdin
SQL=$(cat)

# Run sqlfluff or custom linter
echo "$SQL" | sqlfluff lint --dialect bigquery -

# Exit code:
# 0 = allow
# 1 = block (with stderr as reason)
# 2 = warn (show warning but allow)
```

---

## 3. Subagents

Specialized AI workers that run in isolated context windows for focused tasks.

### Built-in Subagents

| Agent | Model | Tools | Purpose |
|-------|-------|-------|---------|
| `explorer` | Fast (Haiku/Flash) | Schema, Read, Glob, Grep | Quick codebase/schema exploration |
| `planner` | Primary | Read-only tools | Design implementation plans |
| `log-analyst` | Primary | Logging, Read | Analyze logs and debug failures |
| `cost-analyst` | Primary | BigQueryCost, Read | Cost analysis and optimization |
| `sql-writer` | Primary | BigQuerySchema, Read, Write | Generate and optimize SQL |
| `dbt-builder` | Primary | DbtTool, Read, Write, Edit | Build and modify dbt models |
| `reviewer` | Primary | Read, Grep, BigQuerySchema | Review SQL/dbt changes for quality |

### Custom Subagents

Define custom subagents as Markdown files:

```markdown
<!-- .cascade/agents/pipeline-oncall.md -->
---
name: pipeline-oncall
description: On-call triage agent for pipeline failures
model: primary
tools:
  - ComposerTool
  - LoggingTool
  - BigQueryQuery
  - DataflowTool
  - Read
  - Grep
memory: project
---

You are an on-call data engineer triaging pipeline failures.

## Triage Process
1. Identify the failing pipeline and specific task
2. Fetch relevant logs (last 1 hour, ERROR and WARNING severity)
3. Classify the failure type:
   - Schema mismatch
   - Data quality issue
   - Infrastructure (OOM, timeout, quota)
   - Permission/auth
   - Upstream dependency failure
4. Check if this is a known issue (check memory)
5. Propose a fix with:
   - Immediate remediation (get data flowing)
   - Root cause fix (prevent recurrence)
   - Impact assessment (what downstream is affected)

## Escalation Rules
- If the fix requires production DDL changes → flag for human review
- If the failure is in a Tier 1 pipeline → always notify #data-alerts
- If root cause is unclear after 3 investigation rounds → escalate
```

### Subagent Invocation

```
# Automatic (agent decides when to delegate)
cascade> debug the orders pipeline failure
  # Main agent may spawn log-analyst and cost-analyst subagents

# Manual
cascade> @pipeline-oncall what's failing right now?

# Background
cascade> run @cost-analyst in the background: analyze last 30 days of costs
  # Returns immediately, check results with /tasks
```

---

## 4. MCP Servers

Model Context Protocol servers extend Cascade with external tool integrations.

### Configuration

```json
// .cascade/mcp.json (project-level) or ~/.cascade/mcp.json (user-level)
{
    "mcpServers": {
        "github": {
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-github"],
            "env": {"GITHUB_TOKEN": "${GITHUB_TOKEN}"}
        },
        "slack": {
            "command": "npx",
            "args": ["-y", "@anthropic/mcp-slack"],
            "env": {"SLACK_TOKEN": "${SLACK_TOKEN}"}
        },
        "fivetran": {
            "command": "python",
            "args": ["-m", "cascade_mcp_fivetran"],
            "env": {"FIVETRAN_API_KEY": "${FIVETRAN_API_KEY}"}
        },
        "monte-carlo": {
            "command": "python",
            "args": ["-m", "cascade_mcp_montecarlo"],
            "env": {"MC_API_KEY": "${MC_API_KEY}"}
        },
        "jira": {
            "command": "npx",
            "args": ["-y", "@anthropic/mcp-jira"],
            "env": {"JIRA_URL": "${JIRA_URL}", "JIRA_TOKEN": "${JIRA_TOKEN}"}
        },
        "looker": {
            "transport": "http",
            "url": "https://looker.company.com/mcp",
            "auth": {"type": "oauth2"}
        }
    }
}
```

### Recommended MCP Integrations for Data Engineering

| MCP Server | Purpose |
|-----------|---------|
| **GitHub** | PR creation, issue management, code review |
| **Slack** | Pipeline alerts, team notifications |
| **Fivetran/Airbyte** | Ingestion job management and monitoring |
| **Monte Carlo/Soda** | Data observability and quality alerts |
| **dbt Cloud** | Cloud-based dbt job management |
| **Jira/Linear** | Incident tracking and task management |
| **Looker** | Dashboard metadata, report lineage |
| **Confluence** | Documentation search and updates |
| **PagerDuty** | On-call escalation and incident management |

### MCP Management

```bash
# Add a server
cascade mcp add github --command "npx -y @modelcontextprotocol/server-github"

# List configured servers
cascade mcp list

# Test connectivity
cascade mcp test github

# Remove a server
cascade mcp remove github
```

---

## Plugin Packs

Plugins bundle skills, hooks, subagents, and MCP configs into distributable packages:

```
cascade-plugin-dbt-best-practices/
├── plugin.json
├── skills/
│   ├── dbt-conventions.md
│   ├── dbt-testing.md
│   └── dbt-performance.md
├── agents/
│   ├── dbt-reviewer.md
│   └── dbt-migration.md
├── hooks/
│   └── pre-dbt-run.sh
└── README.md
```

```bash
# Install a plugin
cascade plugin add cascade-plugin-dbt-best-practices

# From git
cascade plugin add https://github.com/org/cascade-plugin-dbt

# List plugins
cascade plugin list

# Remove
cascade plugin remove dbt-best-practices
```
