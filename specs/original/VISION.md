# Cascade CLI — Vision & Philosophy

> What if Claude Code and Snowflake Cortex CLI had a GCP data engineering baby?

## The Name

**Cascade** — because data cascades through pipeline stages, `CASCADE` is a SQL keyword every data engineer knows, and it evokes powerful, natural flow. CLI command: `cascade` (alias: `csc`).

## The One-Liner

An AI-native terminal agent for GCP data engineering that understands your warehouse, your pipelines, and your platform — as deeply as Claude Code understands your codebase.

## The Problem

GCP data engineers context-switch between 8+ tools daily:

```
bq query ...          # BigQuery
gcloud composer ...   # Airflow/Composer
gsutil cp ...         # Cloud Storage
dbt run ...           # Transformations
terraform plan ...    # Infrastructure
gcloud dataflow ...   # Streaming jobs
gcloud logging ...    # Debugging
# + the BigQuery console, Airflow UI, Cloud Logging, Dataplex...
```

Each tool has its own syntax, flags, auth model, and mental overhead. When a pipeline fails at 3am, you're juggling five terminal tabs, cross-referencing logs, and remembering which `gcloud` flag does what.

General-purpose AI coding tools (Claude Code, Cursor, Copilot) are brilliant at code — but they have zero awareness of your warehouse schema, your DAG dependencies, your cost profile, or your governance policies. They can write SQL, but they can't tell you it'll scan 4TB and cost $25.

Snowflake solved this for their ecosystem with Cortex CLI — a Claude-powered agent that deeply understands Snowflake's catalog, governance, and platform semantics. But nothing equivalent exists for GCP.

## The Solution

Cascade is the missing layer: a single conversational interface that unifies the entire GCP data engineering stack with platform-aware AI reasoning.

```
$ cascade "why did last night's orders pipeline fail?"

Checking Cloud Composer DAGs...
Found failed task: orders_daily.load_to_bq (failed at 02:47 UTC)
Reading task logs from Cloud Logging...

Root cause: Schema mismatch. Source file in gs://raw-data/orders/2026-02-05/
has a new column `discount_type` (STRING) not present in
`warehouse.bronze.raw_orders`.

Suggested fix:
1. ALTER TABLE warehouse.bronze.raw_orders ADD COLUMN discount_type STRING
2. Update the dbt staging model to handle the new column
3. Backfill today's partition

Want me to apply these changes? [y/N]
```

## Design Principles

### 1. Platform-Aware, Not Platform-Locked
Cascade deeply understands GCP services (BigQuery, Composer, Dataflow, GCS, Pub/Sub, Dataplex) but is not a thin wrapper around `gcloud`. It reasons about your platform holistically — connecting a Composer DAG failure to a schema change in GCS to a cost spike in BigQuery.

### 2. Claude Code's Soul, Data Engineer's Brain
The agent loop, tool system, permission model, context management, and terminal UX are inspired by Claude Code — the gold standard for AI CLI agents. But every tool, prompt, and heuristic is tuned for data engineering workflows.

### 3. Cost-Aware by Default
Every SQL query shows estimated cost before execution. Every suggestion considers slot usage, storage costs, and compute trade-offs. The agent can proactively surface cost anomalies and optimization opportunities.

### 4. Governance-Native
Respects IAM, column-level security, data masking, and Dataplex policies. The agent can only see and do what your service account permits. Sensitive columns are flagged before they appear in query results.

### 5. Code-First, Not Chat-Only
Like Claude Code, Cascade can write and modify files: dbt models, Airflow DAGs, Terraform configs, SQL scripts, Python Beam pipelines. It's not just a query runner — it's a coding agent that happens to understand your data platform.

### 6. Progressive Disclosure
Simple questions get simple answers. Complex tasks get step-by-step plans with approval gates. You control the depth of autonomy.

## The Golden Standard Test

Cascade should pass the "Claude Code test" — it should be equally capable when you:

1. **Ask it a question**: "What's the largest table in the warehouse by storage cost?"
2. **Ask it to investigate**: "Why is this query slow? Show me the execution plan."
3. **Ask it to build**: "Write a dbt model for customer lifetime value using the orders and returns tables."
4. **Ask it to fix**: "The streaming pipeline is dropping messages — debug it."
5. **Ask it anything else**: "Explain the CAP theorem" or "Write a Python script to parse this CSV."

It's a data engineering power tool first, but a general-purpose AI assistant always.
