# Cascade CLI — Context & Memory System

## The Context Challenge

Data engineering context is richer than code context. A coding agent needs to understand files in a repo. Cascade needs to understand:

1. **Schema**: 100s of tables, 1000s of columns, types, relationships
2. **Pipeline state**: DAG runs, failures, schedules, dependencies
3. **Cost profile**: Query costs, slot usage, storage growth
4. **Governance**: Tags, policies, masking rules, ownership
5. **dbt state**: Manifest, run results, test outcomes, lineage
6. **Infrastructure**: Terraform state, Composer config, Dataflow jobs
7. **Plus** the actual code: DAG files, SQL models, Python scripts

This cannot all fit in a single context window. Cascade uses a tiered context strategy.

---

## Context Tiers

### Tier 1: Always Present (system prompt + injected context)

These are included in every LLM call:

```
System prompt (~2K tokens)
├── Core identity and capabilities
├── Tool descriptions
├── Permission rules
├── Current project config

CASCADE.md (~1-3K tokens)
├── Project architecture
├── Conventions and rules
├── Team context

Platform summary (~1-2K tokens)
├── Project ID, region, datasets (names only)
├── Active alerts / recent failures (last 24h)
├── Cost status (today's spend vs budget)
├── Session context (what we've been working on)
```

**Total always-present: ~5-8K tokens** — leaving 90%+ of the context window for conversation and tool results.

### Tier 2: On-Demand (loaded when relevant)

Fetched by tools when the agent needs them:

```
Schema details       → BigQuerySchema tool (loads specific table schemas)
DAG details          → ComposerTool (loads specific DAG info)
Logs                 → LoggingTool (fetches relevant log entries)
dbt manifest         → DbtTool (parses manifest for specific models)
Terraform state      → Bash + Read (loads relevant state sections)
File contents        → Read tool (loads specific files)
Query history        → BigQueryCost tool (loads relevant query stats)
Lineage             → DataplexTool (loads specific lineage paths)
```

### Tier 3: Cached Locally (fast lookup, not in context)

Stored in SQLite, queried by tools:

```
Full schema cache    → ~/.cascade/cache/schema.db
Query cost history   → ~/.cascade/cache/costs.db
Session history      → ~/.cascade/cache/sessions.db
dbt manifest cache   → ~/.cascade/cache/dbt_manifest.json
Autocomplete index   → ~/.cascade/cache/completions.db
```

---

## Schema Cache (Deep Dive)

The schema cache is Cascade's most important context artifact. It enables the agent to write correct SQL, estimate costs, and understand data relationships without querying BigQuery metadata on every turn.

### Structure

```sql
-- ~/.cascade/cache/schema.db (SQLite)

CREATE TABLE datasets (
    dataset_id TEXT PRIMARY KEY,
    project_id TEXT,
    location TEXT,
    description TEXT,
    labels JSON,
    last_refreshed TIMESTAMP
);

CREATE TABLE tables (
    table_id TEXT,
    dataset_id TEXT,
    table_type TEXT,  -- TABLE, VIEW, MATERIALIZED_VIEW, EXTERNAL
    description TEXT,
    row_count INTEGER,
    size_bytes INTEGER,
    partition_field TEXT,
    partition_type TEXT,
    clustering_fields JSON,
    labels JSON,
    tags JSON,          -- From Dataplex
    last_modified TIMESTAMP,
    last_refreshed TIMESTAMP,
    PRIMARY KEY (dataset_id, table_id)
);

CREATE TABLE columns (
    table_id TEXT,
    dataset_id TEXT,
    column_name TEXT,
    data_type TEXT,
    is_nullable BOOLEAN,
    description TEXT,
    is_pii BOOLEAN,
    policy_tags JSON,
    ordinal_position INTEGER,
    PRIMARY KEY (dataset_id, table_id, column_name)
);

-- Full-text search index for natural language schema search
CREATE VIRTUAL TABLE schema_fts USING fts5(
    dataset_id, table_id, column_name, description, tags
);
```

### Refresh Strategy

```go
// RefreshFull runs on first setup or manual /sync.
// ~30s for 100 tables, ~5min for 1000 tables.
func (sc *SchemaCache) RefreshFull(ctx context.Context) error {
    datasets := sc.bqClient.Datasets(ctx)
    for {
        ds, err := datasets.Next()
        if err == iterator.Done { break }
        tables := ds.Tables(ctx)
        // Refresh tables concurrently with a worker pool
        g, gctx := errgroup.WithContext(ctx)
        g.SetLimit(10) // 10 concurrent metadata fetches
        for {
            tbl, err := tables.Next()
            if err == iterator.Done { break }
            g.Go(func() error {
                meta, _ := tbl.Metadata(gctx)
                return sc.upsert(meta)
            })
        }
        g.Wait()
    }
    return nil
}

// RefreshIncremental runs periodically in a background goroutine.
func (sc *SchemaCache) RefreshIncremental(ctx context.Context) error {
    stale := sc.getTablesModifiedSince(sc.lastRefresh)
    for _, ref := range stale {
        meta, _ := sc.bqClient.Dataset(ref.DatasetID).Table(ref.TableID).Metadata(ctx)
        sc.upsert(meta)
    }
    return nil
}

// RefreshOnDemand refreshes a single table if stale (>5 min old).
func (sc *SchemaCache) RefreshOnDemand(ctx context.Context, ref TableRef) (*TableMeta, error) {
    cached := sc.get(ref)
    if cached != nil && time.Since(cached.LastRefreshed) < 5*time.Minute {
        return cached, nil
    }
    meta, err := sc.bqClient.Dataset(ref.DatasetID).Table(ref.TableID).Metadata(ctx)
    if err != nil { return nil, err }
    sc.upsert(meta)
    return meta, nil
}
```

### Schema-Aware Prompt Construction

When the agent needs to write SQL, the relevant schema is injected into context:

```go
// BuildSchemaContext builds a concise schema context string for the LLM prompt.
func (sc *SchemaCache) BuildSchemaContext(tables []string) string {
    var b strings.Builder
    b.WriteString("## Available Schema\n\n")
    for _, ref := range tables {
        t := sc.get(parseTableRef(ref))
        fmt.Fprintf(&b, "### %s.%s\n", t.DatasetID, t.TableID)
        fmt.Fprintf(&b, "Rows: %s | Size: %s\n", humanize(t.RowCount), humanizeBytes(t.SizeBytes))
        if t.PartitionField != "" {
            fmt.Fprintf(&b, "Partitioned by: %s (%s)\n", t.PartitionField, t.PartitionType)
        }
        if len(t.ClusteringFields) > 0 {
            fmt.Fprintf(&b, "Clustered by: %s\n", strings.Join(t.ClusteringFields, ", "))
        }
        b.WriteString("Columns:\n")
        for _, col := range t.Columns {
            pii := ""
            if col.IsPII { pii = " [PII]" }
            fmt.Fprintf(&b, "  - %s (%s)%s", col.Name, col.DataType, pii)
            if col.Description != "" {
                fmt.Fprintf(&b, " -- %s", col.Description)
            }
            b.WriteString("\n")
        }
        b.WriteString("\n")
    }
    return b.String()
}
```

---

## Context Compaction

Like Claude Code, Cascade auto-compacts when approaching context limits.

### Compaction Strategy

```go
// Compact compresses conversation history while preserving key context.
func (a *Agent) Compact(ctx context.Context, focus string) (string, error) {
    prompt := `Summarize this conversation, preserving:
1. What the user is trying to accomplish (overall goal)
2. Key findings and decisions made
3. Schema details referenced (table names, columns, relationships)
4. Pipeline state discovered (failures, root causes, fixes applied)
5. Cost information discussed
6. Any pending actions or next steps`

    if focus != "" {
        prompt += "\n7. Specifically preserve context about: " + focus
    }
    prompt += `

Be concise but do not lose technical details (table names, column names,
SQL snippets, error messages, cost figures).`

    return a.llm.Summarize(ctx, a.sessionHistory, prompt)
}
```

### What's Preserved vs. Dropped

| Preserved | Dropped |
|-----------|---------|
| Schema details referenced | Full schema dumps (re-fetchable) |
| SQL queries written/run | Raw tool output (logs, large result sets) |
| Decisions and reasoning | Intermediate exploration steps |
| Error messages and root causes | Verbose log output |
| Cost figures | Full cost breakdowns |
| Pending actions | Completed/cancelled actions |
| CASCADE.md content | (never dropped — always re-injected) |

---

## Session Management

### Session Storage

```
~/.cascade/sessions/
├── my-analytics-prod/
│   ├── 2026-02-05_debug-orders-pipeline.json
│   ├── 2026-02-05_cost-optimization.json
│   └── 2026-02-04_dbt-model-refactor.json
└── my-analytics-dev/
    └── 2026-02-05_schema-migration.json
```

### Session Operations

```bash
# Resume last session
cascade --continue

# Interactive session picker
cascade --resume

# Resume by name
cascade --resume "debug-orders-pipeline"

# List sessions
cascade --list-sessions

# Name current session
/rename orders-debug-feb5
```

### Session Metadata

```json
{
    "session_id": "abc123",
    "name": "debug-orders-pipeline",
    "project": "my-analytics-prod",
    "created": "2026-02-05T10:00:00Z",
    "last_active": "2026-02-05T10:47:00Z",
    "tables_accessed": ["warehouse.raw_orders", "warehouse.fct_orders"],
    "dags_inspected": ["orders_daily"],
    "queries_run": 7,
    "total_cost_usd": 3.47,
    "status": "active",
    "summary": "Debugging schema mismatch in orders pipeline. Root cause identified: new discount_type column from Shopify. Fix applied to BQ schema and dbt model."
}
```

---

## Memory System

### Short-Term: Session Context
- Current conversation history
- Tool results from this session
- Schema details loaded this session

### Medium-Term: CASCADE.md
- Project-specific knowledge checked into repo
- Team conventions, architecture decisions, cost rules
- Updated by the team, read by Cascade on every session

### Long-Term: Cascade Memory
- Persistent notes across sessions (like Claude Code's auto memory)
- Stored in `~/.cascade/memory/` or `.cascade/memory/`

```markdown
<!-- ~/.cascade/memory/MEMORY.md -->

## Project: my-analytics-prod

### Known Issues
- raw_events table has inconsistent timezone handling before 2025-06
- Shopify webhook schema changes ~monthly, always check raw schema first
- Cloud Composer env update takes 5+ min, don't wait for it in session

### Optimization Notes
- fct_orders JOIN on customer_id benefits from clustering
- raw_events: always filter by event_date first, then event_type
- APPROX_COUNT_DISTINCT saves ~40% on events table

### Team Conventions Learned
- PR descriptions should include dbt model lineage impact
- #data-alerts Slack channel for pipeline failures
- Jake owns the payments domain, Sarah owns customer domain
```

### Subagent Context Isolation

Like Claude Code, subagents run in their own context windows to prevent the main conversation from being overwhelmed by verbose tool output:

```
Main Agent (full context)
├── Subagent: SchemaExplorer (read-only, schema-focused)
├── Subagent: LogAnalyzer (read-only, log-focused)
├── Subagent: CostAnalyzer (read-only, cost-focused)
└── Subagent: CodeWriter (full tools, code-focused)
```

Each subagent returns a concise summary to the main agent, not the raw output.
