# Cascade CLI — Spec Review & Pre-Build Decisions

> Review of `spec-analysis-ag.md` feedback against all 10 specs.
> Written 2026-02-06, the night before build starts.

---

## Decisions Made

These are resolved. Don't revisit during build.

### 1. Read-First Strategy — ADOPTED

Reorder build priority so the first usable version delivers **investigation and debugging**, not code generation.

The "what failed last night?" scenario (SCENARIOS.md #1, #4) is:
- Read-only (low risk, no permission complexity)
- High frequency (every data engineer's morning routine)
- Highly differentiated (no existing tool does cross-service pipeline debugging)
- Trust-building (earns the right to later write/edit code on the user's behalf)

**Concrete impact on ROADMAP.md:**
- Phase 0 stays as-is (foundation, BQ query, file tools)
- Phase 1 priority order becomes: LoggingTool > ComposerTool > GCSTool > schema cache
- DbtTool `generate_model` and `Write`-heavy workflows move to Phase 2
- Phase 1 exit criteria changes to: "Can diagnose a pipeline failure end-to-end in under 60 seconds"

### 2. Defer OS-Level Sandboxing — ADOPTED

Drop `sandbox-exec` (macOS) and `bubblewrap` (Linux) from Phase 3. Revisit post-V1.

**Why:** OS sandboxing is a maintenance nightmare that breaks legitimate tools (`git`, `dbt`, `terraform`, `gcloud`). Claude Code has a dedicated team for this. The ROI for a V1 is negative — you'll spend more time debugging sandbox escapes and false-positive blocks than building features.

**What replaces it for V1:**
- Layer 1 (GCP IAM) already limits what credentials can do
- Layer 2 (Permission Engine) is your real security layer — invest here
- Layer 4 (Cost Gates) prevents the most common damage vector (expensive queries)
- Add: SQL command parsing to detect writes to external projects/buckets
- Add: Log all outbound network requests, alert on non-`*.googleapis.com` destinations

**SECURITY.md Layer 3 becomes:** "Planned for future release. V1 relies on Layers 1, 2, and 4."

### 3. Subagents — KEEP, but Simplify

Don't drop subagents. They solve a real problem: context isolation. When you fetch 500 lines of Cloud Logging output, you don't want it polluting the main conversation.

**V1 implementation:** Fire-and-forget goroutines. No parallel TUI rendering, no concurrent context windows, no orchestration complexity.

```
Main agent calls subagent:
1. Spawn goroutine with focused prompt + limited tool set
2. Subagent runs in its own context (separate message history)
3. Subagent returns a summary string (1-3 paragraphs)
4. Main agent receives summary, continues conversation
```

That's it. No background task UI, no `/tasks` command, no multi-panel rendering. The user sees a spinner ("Analyzing logs...") and then the summary. Implement the fancy stuff later.

### 4. ADK Go — KEEP, No Changes

The `spec-analysis-ag.md` concern about ADK Go instability was valid when written but is outdated. Google ADK Go is under `google.golang.org/adk`, actively maintained, and the official Go agent framework for Gemini. The `Provider` interface in ARCHITECTURE.md already wraps it behind an abstraction, which is the correct mitigation.

**No spec changes needed.** If ADK Go breaks, the interface lets you swap it out. But don't preemptively over-engineer for that scenario.

---

## Spec Amendments

Changes to make to specs before or during build.

### 5. Schema Cache — Use INFORMATION_SCHEMA, Not Per-Table API Calls

**Problem:** The `RefreshFull` code in CONTEXT.md iterates table-by-table with individual metadata API calls. For a 10K-table warehouse:
- 10K API calls at 10 concurrent workers = ~17 minutes minimum
- Hits BigQuery metadata API quota (1500 req/100s default)
- Individual `table.Metadata()` calls are slow and wasteful

**Fix in CONTEXT.md:** Replace per-table iteration with bulk SQL queries:

```sql
-- One query gets ALL tables in a dataset (fast, cheap, no API quota issues)
SELECT table_name, partition_column, clustering_columns, row_count, size_bytes, ...
FROM `project.dataset.INFORMATION_SCHEMA.TABLE_OPTIONS`

-- One query gets ALL columns
SELECT table_name, column_name, data_type, is_nullable, description, ...
FROM `project.dataset.INFORMATION_SCHEMA.COLUMNS`
```

**Also add dataset scoping.** Most users care about 2-3 datasets, not all 50. Config should support:

```toml
[cache]
# Only cache these datasets (empty = all, which is the dangerous default)
datasets = ["warehouse", "marts", "raw_import"]
```

**First-run UX should degrade gracefully:** If the user has 10K tables and didn't scope datasets, cache what they reference on-demand rather than blocking startup for 17 minutes.

### 6. Context Compaction — Add Reference Pointers

**Problem:** CONTEXT.md's compaction strategy summarizes everything into prose. LLMs can lose exact SQL syntax, error messages, and column names during summarization.

**Fix:** Hybrid approach — summarize reasoning, store artifacts as refetchable references.

```
Compacted summary includes:
- Reasoning and conclusions (prose, summarized by LLM)
- Reference pointers to raw artifacts stored in session:
  "Query result #7: 47 rows from warehouse.fct_orders (stored, re-fetchable)"
  "Log output #3: Cloud Logging errors for orders_daily (stored, re-fetchable)"
```

The agent can re-fetch raw artifacts from the session store if it needs exact details after compaction. This is the `Preserved vs. Dropped` table in CONTEXT.md but made explicit as an implementation pattern rather than just a conceptual list.

### 7. Add Model Routing Strategy to ARCHITECTURE.md

**Problem:** The specs say "model-agnostic" but don't specify when to use which model. This matters because Gemini and Claude have different strengths.

**Add a section to ARCHITECTURE.md:**

```toml
[model.routing]
# Default model for main agent loop
default = "gemini-2.5-pro"

# Override for specific tasks (optional)
sql_generation = "gemini-2.5-pro"     # Gemini is strong at structured data
complex_debugging = "claude-sonnet-4-5" # Claude is stronger at multi-step reasoning
schema_exploration = "gemini-2.0-flash"  # Fast model for quick lookups
```

For V1, just support `default` and let the user configure it. Model routing is a Phase 4+ feature. But having the config shape ready prevents future breaking changes.

**Also add:** Error retry with model fallback. If the primary model produces bad SQL (syntax error from BigQuery), feed the error back to the same model first. If it fails twice, try the fallback model. This is the "error correction loop" that `spec-analysis-ag.md` correctly identified as missing from ARCHITECTURE.md.

### 8. Add Degraded / Offline Mode

**Problem:** The specs assume all GCP APIs are always available. They're not. Rate limits, outages, and airplane mode are all real.

**Add to CONTEXT.md or ARCHITECTURE.md:**

```
Degraded Mode Behavior:
- If BigQuery API fails: Use cached schema. Warn: "Schema may be stale (last refresh: 2h ago)"
- If Composer API fails: Show last-known DAG state from cache. Warn: "Pipeline status may be stale"
- If LLM API fails: Graceful error with retry prompt, not a crash
- If all APIs fail (offline): Cascade still works for local file operations, cached schema browsing, and dbt compile (local)
```

The schema cache already enables this — the design is cache-first by nature. Just make the agent loop resilient to tool failures instead of crashing.

### 9. Autocomplete — Move to Phase 2 or Later

**Problem:** UX.md describes schema-aware tab completion. In Bubble Tea, this requires intercepting keystrokes, querying SQLite, rendering a dropdown overlay, and handling selection mid-input. This is a significant engineering effort that isn't accounted for in any roadmap phase.

**Fix:** Explicitly call out autocomplete as a Phase 2 deliverable at the earliest. Phase 0 and Phase 1 should use standard readline-style text input. The schema cache needs to exist and be reliable before you can build autocomplete on top of it.

---

## Build Order Checklist

Based on all the above. This is the sequence to follow when starting tomorrow.

### Phase 0 (Foundation) — What to Build First

```
1. Go module scaffold (go mod init, directory structure)
2. Bubble Tea TUI shell (text input, viewport, spinner, streaming output)
3. LLM integration (Gemini via ADK Go, streaming responses)
4. Agent loop (observe-reason-act, single-threaded, simple)
5. Core tools: Read, Write, Edit, Glob, Grep, Bash
6. GCP auth (ADC, test with `bq` equivalent calls)
7. BigQueryQuery tool (with dry-run cost estimation — this is your flagship)
8. BigQuerySchema tool (basic: list datasets, describe table)
9. Config loading (~/.cascade/config.toml)
10. CASCADE.md loading
11. Interactive mode + one-shot mode (-p flag)
12. Permission model (READ auto-approve, DML prompt, DESTRUCTIVE always prompt)
```

**Phase 0 exit test:** Open Cascade, ask "what's the most expensive table in the warehouse?", get a correct answer with cost estimate. Ask it to write a SQL file, see it create the file.

### Phase 1 (Platform Awareness) — Read-First Priority

```
1. Schema cache (INFORMATION_SCHEMA-based, scoped to configured datasets)
2. Schema-aware context injection (BuildSchemaContext for SQL generation)
3. LoggingTool (query, errors — this powers debugging)
4. ComposerTool (list_dags, dag_status, task_logs, list_failures)
5. GCSTool (ls, head, profile — read-only)
6. PipelineDebugger (ties LoggingTool + ComposerTool + GCSTool together)
7. Platform summary injection (alerts, failures, cost in system prompt)
8. /sync, /failures slash commands
9. Natural language schema search (FTS5)
```

**Phase 1 exit test:** Ask "what failed last night?", get the Scenario 1 experience — failed DAG identified, logs fetched, root cause diagnosed, fix suggested. All read-only, under 60 seconds.

---

## Risks to Watch During Build

Things that will bite you if you're not looking for them.

| Risk | Trigger | Mitigation |
|------|---------|------------|
| Schema cache takes too long on first run | User has >500 tables and didn't scope datasets | Default to on-demand caching. Full sync is opt-in via `/sync` |
| Cost estimation is wrong | Slot reservations, flat-rate pricing, BI Engine | Show "estimated" prominently. Add config for pricing model |
| LLM generates bad SQL | Complex joins, wrong column names, syntax errors | Feed BQ error back to LLM automatically (max 2 retries) |
| Bubble Tea rendering breaks with long output | Large query results, verbose logs | Truncate tool output before rendering. Use viewport with scroll |
| ADK Go API changes | Google ships breaking update | Provider interface isolates you. Pin version in go.mod |
| `dbt` subprocess is slow | dbt compile takes 10-30s depending on project | Cache compiled SQL. Use manifest.json for dependency info, not dbt CLI |
| Permission prompts are annoying | User gets asked 15 times in one session | Permission caching (per-session). Batch similar approvals |
| Gemini hallucinates table/column names | Schema not in context, or model ignores it | Always inject relevant schema before SQL generation. Validate generated SQL column names against cache before execution |

---

## What NOT to Build in V1

Explicitly out of scope. Resist the temptation.

- OS-level sandboxing (sandbox-exec, bubblewrap)
- Multi-panel TUI for concurrent subagent rendering
- IDE extensions (VS Code, JetBrains)
- Cascade Cloud (managed/team version)
- Multi-cloud support (AWS, Azure)
- Slack/Teams bot
- Mobile app
- Plugin marketplace (just support local skills/hooks)
- Telemetry reporting to a remote server (local-only stats are fine)
- Autocomplete (Phase 2+)
- Terraform apply integration (read-only `terraform plan` is fine)
- DataflowTool, PubSubTool (Phase 2+, streaming pipelines are complex)
