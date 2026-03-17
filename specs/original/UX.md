# Cascade CLI — User Experience

## Installation & First Run

```bash
# Install (single binary — no runtime dependencies)
brew install cascade-cli

# or download the binary directly
curl -LsS https://cascade.dev/install.sh | sh

# or via Go
go install github.com/cascade-cli/cascade@latest

# First run — interactive setup wizard
$ cascade

  Welcome to Cascade CLI v0.1.0

  Let's connect to your GCP project.

  GCP Project ID: my-analytics-prod
  Authentication: Using Application Default Credentials (ADC) ✓
  Default dataset: warehouse
  Cloud Composer environment: prod-composer (us-central1) ✓
  dbt project detected: ./dbt_project/ ✓

  Building schema cache... ━━━━━━━━━━━━━━━━ 100% (147 tables cached)

  Ready. Type a question or task, or /help for commands.

cascade>
```

## Interactive Mode

The primary interface. Launch with `cascade` (no args).

```
cascade> what tables have PII?

Searching Dataplex tags and column descriptions...

Found 12 tables with PII-tagged columns:

  Dataset: warehouse
  ┌─────────────────────┬──────────────────────────┬───────────┐
  │ Table               │ PII Columns              │ Policy    │
  ├─────────────────────┼──────────────────────────┼───────────┤
  │ raw_orders          │ customer_email, address   │ masked    │
  │ raw_customers       │ email, phone, ssn         │ masked    │
  │ stg_customers       │ email_hash               │ hashed    │
  │ dim_customers       │ (none - all anonymized)  │ clean     │
  └─────────────────────┴──────────────────────────┴───────────┘

  Dataset: raw_import
  ┌─────────────────────┬──────────────────────────┬───────────┐
  │ Table               │ PII Columns              │ Policy    │
  ...

cascade> write a query to find customers who ordered >$1000 last month

  Estimated cost: $0.43 (scanning 2.1 GB from warehouse.fct_orders)

  SELECT
      c.customer_id,
      c.customer_name,
      SUM(o.order_total) AS total_spent
  FROM `my-project.warehouse.fct_orders` o
  JOIN `my-project.warehouse.dim_customers` c
      ON o.customer_id = c.customer_id
  WHERE o.order_date BETWEEN '2026-01-01' AND '2026-01-31'
  GROUP BY 1, 2
  HAVING total_spent > 1000
  ORDER BY total_spent DESC

  Run this query? [Y/n/edit]
```

## One-Shot Mode

For scripts, CI/CD, and quick questions:

```bash
# Quick question
cascade -p "what failed last night?"

# Pipe-friendly
cascade -p "top 5 most expensive queries this week" --format json | jq '.queries'

# In CI/CD
cascade -p "run dbt test --select tag:critical" --auto-approve --max-cost 10

# From stdin
cat failing_query.sql | cascade -p "optimize this query for cost"

# Output to file
cascade -p "generate a dbt model for customer cohort analysis" -o models/marts/customer_cohorts.sql
```

## Terminal UI Elements

### Streaming Output
All LLM responses stream in real-time, character by character. Tool execution shows a spinner with the active tool name:

```
cascade> debug the orders pipeline failure from last night

  ⠋ Checking Cloud Composer DAG runs...
  ✓ Found failed run: orders_daily @ 2026-02-05 02:47 UTC

  ⠋ Fetching task logs from Cloud Logging...
  ✓ Error in task: load_raw_orders_to_bq

  ⠋ Analyzing error...

  Root cause identified: Schema mismatch

  The source file at gs://raw-data/orders/2026-02-05/orders_00001.json
  contains a new field `discount_type` (STRING) not present in the
  destination table `warehouse.bronze.raw_orders`.

  This appears to be a schema change from the upstream Shopify webhook
  (no prior occurrence in the last 30 days of data).

  Recommended fix:
  1. Add the column to BigQuery:
     ALTER TABLE `warehouse.bronze.raw_orders`
     ADD COLUMN discount_type STRING;

  2. Update the dbt staging model to include the new column:
     File: models/staging/stg_orders.sql (line 14)

  3. Backfill today's partition by clearing the Airflow task

  Apply fix? [y/N/plan]
```

### Cost Warnings
SQL execution always shows cost before running:

```
cascade> SELECT * FROM warehouse.raw_events

  ⚠️  Cost warning: This query will scan 847 GB (~$5.29)
  The raw_events table has 847 GB across all partitions.

  Suggestions:
  • Add a date filter: WHERE event_date >= '2026-02-01' (est. $0.18)
  • Select specific columns instead of * (est. savings: 60-80%)

  Proceed anyway? [y/N/optimize]
```

### Colorized Diffs
File modifications show git-style diffs:

```
cascade> add the discount_type column to the staging model

  models/staging/stg_orders.sql
  ────────────────────────────────
  @@ -12,6 +12,7 @@
       order_total,
       shipping_cost,
       payment_method,
  +    discount_type,
       _loaded_at
   FROM {{ source('bronze', 'raw_orders') }}
```

### Interactive Tables
Query results render as formatted tables with smart truncation:

```
  ┌────────────┬─────────────────┬──────────────┬──────────┐
  │ customer_id│ customer_name   │ total_spent  │ orders   │
  ├────────────┼─────────────────┼──────────────┼──────────┤
  │ C-89421    │ Acme Corp       │ $284,521.00  │ 1,247    │
  │ C-12083    │ GlobalTech Inc  │ $198,334.50  │ 892      │
  │ C-45672    │ DataDriven LLC  │ $156,221.75  │ 634      │
  │ ...        │ (47 more rows)  │              │          │
  └────────────┴─────────────────┴──────────────┴──────────┘
  Query cost: $0.43 | Duration: 2.1s | Bytes scanned: 2.1 GB
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `Ctrl+C` | Cancel current generation/execution |
| `Ctrl+B` | Background a running task |
| `Ctrl+O` | Toggle verbose mode (show LLM thinking) |
| `Shift+Tab` | Cycle permission modes: Confirm → Plan → Bypass |
| `Ctrl+R` | Refresh schema cache |
| `Ctrl+L` | Clear screen |
| `Ctrl+D` | Exit |
| `Tab` | Autocomplete (tables, columns, DAGs, commands) |
| `Up/Down` | History navigation |

## Permission Modes

Cycle with `Shift+Tab`:

```
┌─────────────────────────────────────────────────────────┐
│  CONFIRM (blue)  │ Default. Prompts before mutations.   │
│  PLAN (orange)   │ Read-only. Shows plan, no execution. │
│  BYPASS (red)    │ Auto-approve everything. Use with     │
│                  │ caution in trusted environments.      │
└─────────────────────────────────────────────────────────┘
```

## Slash Commands

```
/help                   Show help and available commands
/compact [focus]        Compress context, optionally preserving specific topics
/plan                   Enter plan mode (read-only exploration)
/sync                   Refresh schema cache and platform state
/cost [period]          Show cost dashboard for current project
/failures [hours]       Show pipeline failures in last N hours
/lineage <table>        Show data lineage for a table
/profile <table>        Profile a table's data
/dbt <command>          Run dbt commands with enhanced output
/dag <dag_id>           Show DAG details and recent runs
/explain <query>        Explain a SQL query's execution plan and cost
/history                Show session history
/resume                 Resume a previous session
/config                 Edit configuration
/clear                  Clear the screen
```

## Autocomplete

Cascade provides context-aware autocomplete powered by the schema cache:

```
cascade> SELECT customer_id, ord[TAB]
  order_id
  order_date
  order_total
  order_status

cascade> /dag ord[TAB]
  orders_daily
  orders_hourly
  orders_backfill

cascade> /lineage warehouse.[TAB]
  warehouse.raw_orders
  warehouse.stg_orders
  warehouse.fct_orders
  warehouse.dim_customers
  ...
```

## Output Formats

```bash
# Default: Styled terminal output (Lip Gloss + Glamour)
cascade -p "top 5 expensive queries"

# JSON (for piping)
cascade -p "top 5 expensive queries" --format json

# CSV
cascade -p "top 5 expensive queries" --format csv

# Markdown (for docs/reports)
cascade -p "top 5 expensive queries" --format markdown

# Quiet (minimal output, for scripts)
cascade -p "run dbt test" --quiet
```

## Configuration File

```toml
# ~/.cascade/config.toml

[project]
gcp_project = "my-analytics-prod"
default_dataset = "warehouse"
default_region = "us-central1"

[model]
provider = "google"
model = "gemini-2.5-pro"
thinking = "adaptive"
effort = "high"

[cost]
max_query_cost_usd = 10.0        # Prompt if query exceeds this
warn_query_cost_usd = 1.0        # Show warning if query exceeds this
daily_budget_usd = 100.0          # Alert if daily spend approaches this
track_all_queries = true           # Log all query costs to local DB

[composer]
environment = "prod-composer"
region = "us-central1"

[dbt]
project_dir = "./dbt_project"
target = "dev"
profiles_dir = "~/.dbt"

[cache]
schema_refresh_minutes = 60        # Auto-refresh schema cache interval
cache_dir = "~/.cascade/cache"

[security]
sandbox_mode = "auto"              # auto | container | off
permission_mode = "confirm"        # confirm | plan | bypass
allow_destructive_sql = false      # Block DROP/TRUNCATE by default
mask_pii_in_output = true          # Redact PII-tagged columns in display

[display]
theme = "dark"                     # dark | light | auto
max_table_rows = 50                # Truncate result tables after N rows
show_cost_always = true            # Always show cost estimate before SQL
```

## Project Configuration (CASCADE.md)

Like Claude Code's CLAUDE.md — a Markdown file checked into your repo that provides project-specific context.

```markdown
# CASCADE.md

## Project: Analytics Warehouse

### Architecture
- Bronze layer: `raw_import` dataset (raw JSON/CSV from sources)
- Silver layer: `warehouse` dataset (cleaned, typed, deduplicated)
- Gold layer: `marts` dataset (business-ready aggregations)
- Sources: Shopify (orders, customers), Stripe (payments), Segment (events)

### Conventions
- dbt models use `stg_` prefix for staging, `fct_` for facts, `dim_` for dimensions
- All timestamps are UTC
- Customer IDs are prefixed with `C-`
- Use `APPROX_COUNT_DISTINCT` unless exact counts are required
- Partition all large tables by date; cluster by the most common filter columns

### Cost Rules
- Never run SELECT * on raw_events (847 GB) without a date filter
- Use `_TABLE_SUFFIX` for sharded tables in raw_import
- Prefer materialized views over scheduled queries for repeated aggregations

### DAG Naming
- `{domain}_{frequency}` e.g., `orders_daily`, `events_hourly`
- Backfill DAGs: `{domain}_backfill`

### Team
- Data platform team owns `raw_import` and infrastructure
- Analytics engineering owns `warehouse` and `marts`
- Slack channel for alerts: #data-alerts
```
