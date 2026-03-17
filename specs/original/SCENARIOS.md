# Cascade CLI — Usage Scenarios

Real-world scenarios showing how Cascade handles common data engineering tasks. These demonstrate the "Claude Code test" — equally capable for investigation, building, debugging, and general questions.

---

## Scenario 1: Morning Pipeline Triage

**Context:** It's 8am. You open your terminal to check overnight pipeline health.

```
$ cascade

cascade> what failed last night?

  ⠋ Checking Cloud Composer DAGs (last 12 hours)...
  ⠋ Checking Dataflow streaming jobs...
  ⠋ Checking BigQuery scheduled queries...

  Pipeline Status (last 12h):

  FAILURES (2):
  ┌────────────────────┬──────────────┬───────────┬──────────────────────────┐
  │ Pipeline           │ Failed At    │ Type      │ Error Summary            │
  ├────────────────────┼──────────────┼───────────┼──────────────────────────┤
  │ orders_daily       │ 02:47 UTC    │ Composer  │ Schema mismatch in load  │
  │ events_hourly      │ 05:15 UTC    │ Composer  │ BQ quota exceeded        │
  └────────────────────┴──────────────┴───────────┴──────────────────────────┘

  WARNINGS (1):
  ┌────────────────────┬──────────────┬───────────┬──────────────────────────┐
  │ Pipeline           │ Time         │ Type      │ Warning                  │
  ├────────────────────┼──────────────┼───────────┼──────────────────────────┤
  │ clickstream_ingest │ ongoing      │ Dataflow  │ Backlog: 47K messages    │
  └────────────────────┴──────────────┴───────────┴──────────────────────────┘

  HEALTHY (14): payments_daily, customers_sync, inventory_hourly, ...

  Want me to investigate the failures? [orders_daily / events_hourly / both]

cascade> orders_daily

  ⠋ Fetching task-level details...
  ⠋ Reading logs for failed task: load_raw_orders_to_bq...
  ⠋ Checking source data in GCS...

  Root Cause Analysis: orders_daily

  Failed task: load_raw_orders_to_bq
  Error: Schema mismatch when loading gs://raw-data/orders/2026-02-05/

  Details:
  The source file contains a new column `discount_type` (STRING) that
  doesn't exist in the destination table `warehouse.bronze.raw_orders`.

  Investigation:
  ✓ Column first appeared in today's files (not in yesterday's)
  ✓ 847 files in today's partition, all contain the new field
  ✓ Likely a Shopify webhook schema change (upstream)
  ✓ No recent changes to the DAG or dbt models (git log clean)

  Downstream Impact:
  • stg_orders (dbt staging model) — will need update
  • fct_orders (dbt fact table) — depends on stg_orders
  • orders_daily_report (Looker dashboard) — delayed until fix

  Recommended Fix (3 steps):
  1. ALTER TABLE to add the column to BigQuery
  2. Update stg_orders.sql dbt model to include discount_type
  3. Clear the failed Airflow task to retry

  Apply fix? [y/N/plan]

cascade> y

  Step 1/3: Adding column to BigQuery
  SQL: ALTER TABLE `my-project.warehouse.bronze.raw_orders`
       ADD COLUMN discount_type STRING;
  ✓ Column added successfully

  Step 2/3: Updating dbt staging model
  models/staging/stg_orders.sql
  @@ -12,6 +12,7 @@
       order_total,
       shipping_cost,
       payment_method,
  +    discount_type,
       _loaded_at
   FROM {{ source('bronze', 'raw_orders') }}
  ✓ File updated

  Step 3/3: Clearing Airflow task to retry
  Clearing: orders_daily.load_raw_orders_to_bq (2026-02-05)
  This will also re-run downstream tasks: [transform_orders, test_orders]
  Confirm? [Y/n]

cascade> y

  ✓ Task cleared. DAG re-running.
  ⠋ Monitoring... load_raw_orders_to_bq: running (47s elapsed)
  ✓ load_raw_orders_to_bq: success (2m 14s)
  ⠋ transform_orders: running...
  ✓ transform_orders: success (45s)
  ✓ test_orders: success (12s)

  Pipeline recovered. Total time: 3m 11s.

  Suggestion: Consider adding a schema evolution handler to the
  orders DAG to auto-detect new columns. Want me to implement that?
```

---

## Scenario 2: Cost Investigation

```
cascade> our BigQuery bill spiked 3x this week. what happened?

  ⠋ Analyzing INFORMATION_SCHEMA.JOBS for the last 7 days...
  ⠋ Comparing to previous 7-day baseline...

  Cost Analysis: Feb 1-5 vs Jan 25-31

  Total: $847.23 (this week) vs $271.45 (last week) — 3.1x increase

  Top Contributors to Increase:
  ┌───┬──────────────────────────┬───────────┬───────────┬──────────┐
  │ # │ Query Pattern            │ Last Week │ This Week │ Delta    │
  ├───┼──────────────────────────┼───────────┼───────────┼──────────┤
  │ 1 │ SELECT * FROM raw_events │ $12.40    │ $398.50   │ +$386.10 │
  │   │ (user: jake@company.com) │ (2 runs)  │ (64 runs) │          │
  ├───┼──────────────────────────┼───────────┼───────────┼──────────┤
  │ 2 │ fct_events rebuild       │ $45.20    │ $142.80   │ +$97.60  │
  │   │ (dbt: events_daily DAG)  │ (7 runs)  │ (21 runs) │          │
  ├───┼──────────────────────────┼───────────┼───────────┼──────────┤
  │ 3 │ Looker dashboard refresh │ $38.90    │ $89.40    │ +$50.50  │
  │   │ (sa: looker-sa@...)      │ (168 runs)│ (504 runs)│          │
  └───┴──────────────────────────┴───────────┴───────────┴──────────┘

  Analysis:

  1. Jake's raw_events queries: He's running SELECT * without partition
     filters. 64 runs in 5 days = likely iterative exploration.
     Fix: Talk to Jake, or add a required partition filter policy.

  2. fct_events rebuild: The events_daily DAG is running 3x/day instead
     of 1x. Checked git history — a PR merged Feb 1 changed the schedule
     from `0 4 * * *` to `0 */8 * * *` (every 8 hours).
     Fix: Verify if 3x/day is intentional. If not, revert the schedule.

  3. Looker dashboard: Dashboard refresh frequency tripled. This usually
     means someone added auto-refresh or new tiles. Check Looker admin.
     Fix: Add a materialized view for the underlying aggregation.

  Estimated savings if all fixes applied: ~$530/week ($27.5K/year)

  Want me to create a materialized view for the Looker dashboard query?
```

---

## Scenario 3: Building a New dbt Model

```
cascade> I need a dbt model for customer lifetime value. Use orders and
         returns tables. Include cohort analysis by signup month.

  ⠋ Loading schema for warehouse.fct_orders and warehouse.fct_returns...
  ⠋ Loading schema for warehouse.dim_customers...
  ⠋ Checking existing dbt models for dependencies...

  I'll create two models:

  1. `int_customer_orders_summary` — intermediate model aggregating
     per-customer order and return metrics
  2. `fct_customer_ltv` — final fact table with LTV and cohort analysis

  Here's the plan:

  models/intermediate/int_customer_orders_summary.sql
  ─────────────────────────────────────────────────────
  - Aggregate orders by customer: count, total, avg, first/last date
  - Aggregate returns by customer: count, total
  - Calculate net revenue (orders - returns)
  - Materialized as: ephemeral (used only by downstream model)

  models/marts/fct_customer_ltv.sql
  ─────────────────────────────────────
  - Join customer summary with dim_customers for signup date
  - Calculate: months_active, avg_monthly_revenue, projected_ltv
  - Cohort by: DATE_TRUNC(signup_date, MONTH)
  - Include cohort-level aggregates as window functions
  - Materialized as: incremental (partitioned by cohort_month)
  - Clustered by: customer_segment, region

  models/marts/fct_customer_ltv.yml
  ─────────────────────────────────────
  - Column descriptions
  - Tests: unique(customer_id), not_null(customer_id, ltv),
    expression_is_true(ltv >= 0)

  Estimated query cost for full build: $1.23 (scanning 4.8 GB)

  Create these files? [Y/n/edit plan]

cascade> y

  ✓ Created models/intermediate/int_customer_orders_summary.sql
  ✓ Created models/marts/fct_customer_ltv.sql
  ✓ Created models/marts/fct_customer_ltv.yml

  Running dbt compile to validate...
  ✓ Compiled successfully. No errors.

  Running dbt build --select fct_customer_ltv+ --target dev...
  ⠋ int_customer_orders_summary (ephemeral)
  ✓ fct_customer_ltv (incremental, 2.4s, $1.18)
  ✓ Tests: 3 passed, 0 failed

  Model is live in dev. Preview:

  ┌─────────────┬──────────────┬──────────┬──────────┬──────────────┐
  │ cohort_month │ customers   │ avg_ltv  │ median   │ total_revenue│
  ├─────────────┼──────────────┼──────────┼──────────┼──────────────┤
  │ 2025-01     │ 4,521       │ $1,247   │ $843     │ $5,637,387   │
  │ 2025-02     │ 5,103       │ $1,089   │ $712     │ $5,557,167   │
  │ ...         │             │          │          │              │
  └─────────────┴──────────────┴──────────┴──────────┴──────────────┘

  Ready to deploy to prod, or want to iterate?
```

---

## Scenario 4: Streaming Pipeline Debugging

```
cascade> the clickstream pipeline has a growing backlog. what's going on?

  ⠋ Checking Dataflow job: clickstream-ingest-prod...
  ⠋ Pulling job metrics...
  ⠋ Checking Pub/Sub subscription lag...

  Streaming Pipeline Health: clickstream-ingest-prod

  Status: RUNNING (degraded)
  ┌────────────────────┬───────────────┬──────────────────┐
  │ Metric             │ Current       │ Normal Range     │
  ├────────────────────┼───────────────┼──────────────────┤
  │ Throughput          │ 1.2K msg/s   │ 8-12K msg/s      │
  │ Backlog            │ 847K messages │ < 1K messages    │
  │ Backlog age        │ 47 minutes    │ < 30 seconds     │
  │ Workers            │ 4             │ 4-8 (autoscale)  │
  │ CPU utilization    │ 94%           │ 40-60%           │
  │ Memory utilization │ 87%           │ 50-70%           │
  │ Error rate         │ 2.3%          │ < 0.1%           │
  └────────────────────┴───────────────┴──────────────────┘

  ⠋ Checking worker logs for errors...
  ⠋ Checking for recent input schema changes...

  Root Cause: Worker OOM + autoscaling blocked

  1. Workers are at 94% CPU, 87% memory — they can't keep up
  2. Autoscaling is configured for max 8 workers, but...
  3. Autoscaling is BLOCKED: quota `CPUS` in us-central1 is at limit (96/96)
  4. Error logs show 2.3% of messages failing with OOM on a new
     `enrichment_payload` field that contains large nested JSON (avg 12KB
     vs previous 800B)

  Timeline:
  - 05:00 UTC: New field started appearing in clickstream events
  - 05:12 UTC: Worker memory started climbing
  - 05:28 UTC: Backlog started growing
  - 05:45 UTC: Autoscaler tried to add workers, quota blocked

  Recommended Actions:
  1. Immediate: Request CPU quota increase (or switch to n2-highmem workers)
  2. Short-term: Add max message size filter in Beam pipeline to handle
     oversized payloads gracefully
  3. Long-term: Partition the enrichment payload processing into a
     separate, dedicated pipeline

  Which action do you want to take? [1/2/3/all]
```

---

## Scenario 5: General Knowledge Question

```
cascade> explain the difference between MERGE and INSERT OVERWRITE in
         BigQuery. When should I use each for incremental loads?

  ## MERGE vs INSERT OVERWRITE in BigQuery

  ### MERGE (DML)
  Performs upsert logic: matches rows between source and target on a key,
  then inserts new rows, updates existing ones, and optionally deletes.

  MERGE warehouse.fct_orders AS target
  USING staging.new_orders AS source
  ON target.order_id = source.order_id
  WHEN MATCHED THEN UPDATE SET ...
  WHEN NOT MATCHED THEN INSERT ...

  **Cost:** Scans the entire target table partition + source data.
  **Use when:** Source data may contain updates to existing rows (CDC,
  late-arriving facts, corrections).

  ### INSERT OVERWRITE (partition-level)
  Replaces entire partitions. All existing data in the affected partitions
  is deleted, then new data is inserted.

  INSERT OVERWRITE warehouse.fct_orders
  PARTITION (order_date)
  SELECT * FROM staging.new_orders

  **Cost:** Only scans source data (cheaper than MERGE for full partition
  replacement).
  **Use when:** You can guarantee a full, correct snapshot of each
  partition (no partial updates).

  ### Decision Matrix
  ┌─────────────────────────┬─────────────────┬──────────────────┐
  │ Scenario                │ MERGE           │ INSERT OVERWRITE │
  ├─────────────────────────┼─────────────────┼──────────────────┤
  │ CDC / event updates     │ ✓ Best choice   │ ✗                │
  │ Full partition reload   │ Works but costly│ ✓ Best choice    │
  │ Late-arriving facts     │ ✓ Best choice   │ Risk of data loss│
  │ Append-only events      │ Overkill        │ ✓ or just INSERT │
  │ Cost sensitivity        │ Higher          │ Lower            │
  └─────────────────────────┴─────────────────┴──────────────────┘

  In your project, the dbt incremental models use MERGE by default
  (configured in dbt_project.yml). This is appropriate for orders and
  customers (which have updates), but you could save ~30% on the events
  models by switching to INSERT OVERWRITE since events are append-only.

  Want me to check which dbt models could benefit from switching?
```

---

## Scenario 6: Infrastructure as Code

```
cascade> create a Terraform config for a new "marketing" dataset in
         BigQuery with appropriate IAM for the marketing team

  ⠋ Checking existing Terraform structure...
  ⠋ Reading current IAM patterns from terraform/bigquery.tf...

  I see your Terraform follows a module-per-dataset pattern.
  I'll create the marketing dataset following the same pattern.

  terraform/modules/bigquery-marketing/main.tf
  ───────────────────────────────────────────────
  - Dataset: `marketing` in us-central1
  - Default table expiration: none
  - Labels: team=marketing, env=prod
  - IAM:
    - marketing-analysts@ → bigquery.dataViewer
    - marketing-engineers@ → bigquery.dataEditor
    - data-platform@ → bigquery.admin
    - cascade-sa@ → bigquery.jobUser (for Cascade access)

  terraform/modules/bigquery-marketing/variables.tf
  terraform/modules/bigquery-marketing/outputs.tf

  terraform/main.tf (addition)
  ─────────────────────────────
  + module "bigquery_marketing" {
  +   source = "./modules/bigquery-marketing"
  +   project_id = var.project_id
  +   ...
  + }

  Create these files and run terraform plan? [Y/n]
```
