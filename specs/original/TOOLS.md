# Cascade CLI — Tool System

## Tool Categories

Cascade's tools are organized into five categories, inspired by Claude Code's clean separation of concerns but extended with platform-aware data engineering primitives.

```
Tools
├── Core (file/code operations — same as Claude Code)
├── GCP Platform (BigQuery, Composer, Dataflow, GCS, Pub/Sub, Dataplex)
├── Data Engineering (dbt, SQL analysis, data profiling, lineage)
├── Observability (logging, monitoring, cost tracking)
└── External (MCP servers, custom tools)
```

---

## 1. Core Tools (inherited from Claude Code model)

These provide the foundation for general-purpose coding and file manipulation. Cascade is a coding agent first — it can write Python, edit YAML, manage git, and do everything Claude Code does.

### Read
Read files from the local filesystem. Supports code, configs, logs, images, PDFs, notebooks.
```
Read(file_path="/path/to/dags/orders_daily.py")
Read(file_path="/path/to/models/staging/stg_orders.sql")
```

### Write
Create or overwrite files. Used for generating dbt models, DAGs, Terraform configs, etc.
```
Write(file_path="models/marts/customer_ltv.sql", content="...")
```

### Edit
Exact string replacement in files. Surgical edits to existing code.
```
Edit(file_path="dags/orders.py", old_string="schedule='0 2 * * *'", new_string="schedule='0 3 * * *'")
```

### Glob
Fast file pattern matching. Find files by name patterns.
```
Glob(pattern="models/**/*.sql")
Glob(pattern="dags/*.py")
Glob(pattern="**/*.tf")
```

### Grep
Content search powered by ripgrep. Find code patterns across files.
```
Grep(pattern="ref\\('stg_orders'\\)", type="sql")
Grep(pattern="BigQueryInsertJobOperator", glob="dags/*.py")
```

### Bash
Execute shell commands. For git operations, dbt CLI, gcloud, terraform, and other tools.
```
Bash(command="git diff HEAD~1 -- models/")
Bash(command="dbt compile --select customer_ltv")
Bash(command="terraform plan -target=module.bigquery")
```

---

## 2. GCP Platform Tools

These are Cascade's superpower — deep, native integration with GCP data services.

### BigQueryQuery
Execute SQL against BigQuery with built-in cost estimation and safety.

```go
BigQueryQuery{
    sql="SELECT customer_id, SUM(amount) FROM `warehouse.orders` GROUP BY 1",
    dry_run=False,           # If True, only estimate cost
    max_cost_usd=5.00,       # Abort if estimated cost exceeds threshold
    destination=None,         # Optional destination table
    labels={"source": "cascade", "user": "rk"},
    timeout_seconds=300,
)
```

**Behavior:**
1. Always runs a dry-run first to estimate bytes scanned / cost
2. Displays cost estimate to user before execution
3. If `max_cost_usd` exceeded, requires explicit approval
4. Adds labels for cost attribution
5. Returns results as formatted table (small results) or summary (large results)
6. Logs query to session history for lineage tracking

**Risk Classification:**
| SQL Type | Risk Level | Default Behavior |
|----------|-----------|-----------------|
| SELECT, SHOW, DESCRIBE | READ_ONLY | Auto-approve |
| CREATE TABLE, CREATE VIEW | DDL | Prompt |
| INSERT, UPDATE, DELETE, MERGE | DML | Prompt |
| DROP, TRUNCATE | DESTRUCTIVE | Always prompt + confirmation |
| ALTER, GRANT, REVOKE | ADMIN | Always prompt + confirmation |

### BigQuerySchema
Introspect warehouse schema without writing SQL.

```go
BigQuerySchema{
    action="list_datasets"     # list_datasets | list_tables | describe_table |
                               # describe_column | search | refresh_cache
    dataset="warehouse",
    table="raw_orders",
    search_query="customer email",  # Natural language schema search
)
```

**Returns:** Structured schema info including columns, types, partitioning, clustering, row counts, descriptions, tags, and policies.

### BigQueryCost
Analyze and optimize BigQuery costs.

```go
BigQueryCost{
    action="top_queries"        # top_queries | slot_usage | storage_cost |
                                # estimate | budget_status | recommendations
    days=7,
    min_cost_usd=1.0,
    group_by="user",            # user | label | dataset | query_pattern
)
```

**Uses:** `INFORMATION_SCHEMA.JOBS`, `INFORMATION_SCHEMA.TABLE_STORAGE`, reservation APIs, and billing export.

### ComposerTool
Interact with Cloud Composer / Airflow.

```go
ComposerTool{
    action="list_dags"          # list_dags | dag_status | task_logs |
                                # trigger_dag | clear_task | list_failures |
                                # dag_dependencies | pause_dag | unpause_dag
    environment="prod-composer",
    dag_id="orders_daily",
    task_id="load_to_bq",
    execution_date="2026-02-05",
)
```

**Behavior:**
- `list_failures`: Shows failed DAG runs in the last N hours with error summaries
- `task_logs`: Fetches logs from Cloud Logging (not the Airflow UI)
- `dag_dependencies`: Visualizes upstream/downstream DAG relationships
- `trigger_dag`: Requires approval, shows downstream impact before triggering
- `clear_task`: Shows what will re-run (downstream tasks) before clearing

### DataflowTool
Manage Dataflow / Apache Beam jobs.

```go
DataflowTool{
    action="list_jobs"          # list_jobs | job_status | job_metrics |
                                # drain_job | cancel_job | launch_template |
                                # list_templates | worker_logs
    job_id="2026-02-05_...",
    region="us-central1",
)
```

**Behavior:**
- `job_metrics`: Shows throughput, backlog, watermark lag, error rate
- `drain_job`: Explains the difference between drain and cancel, requires approval
- `launch_template`: Supports both Flex and Classic templates with parameter prompting

### GCSTool
Browse, profile, and manage Cloud Storage.

```go
GCSTool{
    action="ls"                 # ls | cat | head | profile | cp | mv |
                                # lifecycle | size | diff
    bucket="raw-data",
    prefix="orders/2026-02-05/",
    profile_rows=1000,          # For profile action: sample N rows
)
```

**Behavior:**
- `profile`: Reads a sample of files, infers schema, shows column stats (nulls, cardinality, min/max/mean)
- `diff`: Compares file schema between two dates/partitions
- `size`: Shows storage breakdown by prefix, with cost estimates per storage class

### PubSubTool
Manage Pub/Sub topics and subscriptions.

```go
PubSubTool{
    action="list_topics"        # list_topics | list_subscriptions |
                                # peek_messages | publish | subscription_lag |
                                # dead_letter_status
    topic="orders-events",
    subscription="orders-to-bq",
    peek_count=5,
)
```

### DataplexTool
Interact with Dataplex Universal Catalog for governance and lineage.

```go
DataplexTool{
    action="search"             # search | lineage | tags | quality_results |
                                # glossary | data_product
    query="customer PII tables",
    table="warehouse.raw_orders",
    lineage_depth=3,            # How many hops upstream/downstream
)
```

**Behavior:**
- `search`: Natural language search across all data assets
- `lineage`: Visual lineage graph (rendered as ASCII tree in terminal)
- `tags`: Show business and technical tags for a data asset
- `quality_results`: Show recent data quality scan results

### LoggingTool
Query Cloud Logging for pipeline debugging.

```go
LoggingTool{
    action="query"              # query | tail | errors | correlate
    filter='resource.type="cloud_composer_environment" severity>=ERROR',
    time_range="1h",
    service="composer",         # Shorthand: composer | dataflow | bigquery | functions
    correlate_with="dag_run",   # Cross-reference with DAG run metadata
)
```

---

## 3. Data Engineering Tools

Higher-level tools that combine platform primitives with data engineering logic.

### DbtTool
Full dbt lifecycle management.

```go
DbtTool{
    action="run"                # run | test | build | compile | ls | docs |
                                # show_lineage | source_freshness | debug |
                                # generate_model | show_compiled_sql
    select="customer_ltv",
    full_refresh=False,
    target="dev",
)
```

**Behavior:**
- `generate_model`: Given a description and source tables, generates a dbt model with proper refs, schema tests, and documentation
- `show_lineage`: Parses manifest.json and shows upstream/downstream model dependencies
- `show_compiled_sql`: Compiles a model and shows the raw SQL that would run in BigQuery (with cost estimate)
- `source_freshness`: Checks source freshness against configured thresholds

### DataProfiler
Profile data in tables or files without writing SQL.

```go
DataProfiler{
    source="warehouse.bronze.raw_orders",  # BQ table or GCS path
    sample_size=10000,
    columns=["customer_email", "order_amount", "region"],
    checks=["nulls", "cardinality", "distribution", "outliers", "patterns"],
)
```

**Returns:**
```
Column: customer_email
  Type: STRING | Nulls: 0.2% | Cardinality: 8.4M | Pattern: email (99.8%)
  Top values: [redacted - PII detected]

Column: order_amount
  Type: FLOAT64 | Nulls: 0% | Min: 0.01 | Max: 99,847.50 | Mean: 127.43
  Distribution: right-skewed | Outliers: 342 rows > 3 std dev

Column: region
  Type: STRING | Nulls: 0% | Cardinality: 12
  Top values: US (45.2%), EU (28.1%), APAC (15.3%), ...
```

### PipelineDebugger
Automated pipeline failure diagnosis.

```go
PipelineDebugger{
    pipeline="orders_daily",    # DAG name, Dataflow job, or dbt model
    execution_date="2026-02-05",
    depth="deep",               # quick | deep
)
```

**Deep debug flow:**
1. Identify the failed task/step
2. Fetch logs from Cloud Logging
3. Classify the error (schema mismatch, timeout, OOM, permission, data quality, etc.)
4. Check upstream dependencies (did source data arrive? Schema change?)
5. Check recent changes (git log for DAG/model changes in last 48h)
6. Propose fix with step-by-step remediation plan

### SQLAnalyzer
Analyze SQL for cost, performance, and correctness.

```go
SQLAnalyzer{
    sql="SELECT * FROM warehouse.orders WHERE date > '2020-01-01'",
    checks=["cost", "partition_pruning", "clustering_benefit",
            "join_explosion", "select_star", "best_practices"],
)
```

**Returns:**
```
Issues found:
  [COST] SELECT * scans all 48 columns — specify only needed columns
  [PERF] No partition filter on `order_date` — full table scan (1.2TB, ~$7.50)
  [PERF] Consider adding WHERE order_date > '2020-01-01' instead of date > ...
  [BEST] Use APPROX_COUNT_DISTINCT() if exact count not needed
  Estimated cost: $7.50 → $0.12 with suggested optimizations
```

---

## 4. Observability Tools

### CostMonitor
Proactive cost intelligence.

```go
CostMonitor{
    action="dashboard"          # dashboard | anomalies | forecast |
                                # budget_alert | slot_utilization
    scope="project",            # project | dataset | user | label
    period="7d",
)
```

### AlertsTool
Check active alerts and incidents.

```go
AlertsTool{
    action="active"             # active | history | acknowledge | create
    service="all",              # all | bigquery | composer | dataflow | pubsub
)
```

---

## 5. External Tools (MCP + Custom)

### MCP Server Support

Cascade supports the Model Context Protocol for extensibility, configured in `~/.cascade/mcp.json` or `.cascade/mcp.json` (project-level).

```json
{
    "mcpServers": {
        "github": {
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-github"],
            "env": {"GITHUB_TOKEN": "..."}
        },
        "slack": {
            "command": "npx",
            "args": ["-y", "@anthropic/mcp-slack"],
            "env": {"SLACK_TOKEN": "..."}
        },
        "fivetran": {
            "command": "python",
            "args": ["-m", "cascade_mcp_fivetran"],
            "env": {"FIVETRAN_API_KEY": "..."}
        }
    }
}
```

### Custom Tools

Users can define custom tools as Go plugins or as external scripts invoked via MCP:

**Option 1: Go plugin (compiled into Cascade or loaded at startup)**

```go
// .cascade/tools/check_sla.go
package tools

import "google.golang.org/adk/tools/functiontool"

var CheckSLA = functiontool.New(
    "check_sla",
    "Check if a pipeline meets its SLA based on team config",
    func(ctx context.Context, args struct {
        PipelineName string `json:"pipeline_name"`
        Date         string `json:"date,omitempty"` // defaults to "today"
    }) (map[string]any, error) {
        slaConfig := loadSLAConfig()
        actual := getPipelineCompletionTime(args.PipelineName, args.Date)
        expected := slaConfig[args.PipelineName].Deadline
        return map[string]any{
            "pipeline":          args.PipelineName,
            "sla_deadline":      expected,
            "actual_completion": actual,
            "met_sla":           actual.Before(expected),
        }, nil
    },
)
```

**Option 2: Script-based tool (any language, invoked as subprocess)**

```bash
# .cascade/tools/check_sla.sh — receives JSON args via stdin, returns JSON via stdout
#!/bin/bash
PIPELINE=$(echo "$1" | jq -r '.pipeline_name')
# ... check SLA logic ...
echo '{"pipeline": "'$PIPELINE'", "met_sla": true}'
```

---

## Tool Execution Model

### Parallel Execution
Like Claude Code, independent tool calls execute in parallel:

```
# These run in parallel (no dependencies)
BigQuerySchema(action="describe_table", table="orders")
BigQuerySchema(action="describe_table", table="customers")
ComposerTool(action="list_failures", time_range="24h")

# These run sequentially (output of first feeds into second)
result = BigQueryQuery(sql="SELECT ...", dry_run=True)
# User approves cost
BigQueryQuery(sql="SELECT ...", dry_run=False)
```

### Tool Result Rendering

Tool results are rendered contextually:
- **Tables**: Formatted with Lip Gloss tables (small results) or paginated via Bubble Tea viewport (large results)
- **SQL**: Syntax-highlighted with cost annotations
- **Lineage**: ASCII tree diagrams
- **Logs**: Color-coded by severity
- **Diffs**: Git-style colorized diffs
- **Costs**: Color-coded (green < $1, yellow < $10, red > $10)

### Tool Timeout Policy

| Tool Category | Default Timeout | Max Timeout |
|--------------|----------------|-------------|
| Schema/metadata | 30s | 60s |
| SQL queries | 120s | 600s (10 min) |
| Log queries | 60s | 300s |
| dbt commands | 300s | 600s |
| File operations | 10s | 30s |
| Bash commands | 120s | 600s |
