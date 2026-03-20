---
phase: 02-bigquery-core
plan: 05
subsystem: integration
tags: [bigquery, schema-cache, cost-tracking, system-prompt, sql-risk, wiring]

# Dependency graph
requires:
  - phase: 02-01
    provides: "BQ client, schema cache, cost tracker, SQL classifier"
  - phase: 02-02
    provides: "Session compaction for long BQ sessions"
  - phase: 02-03
    provides: "BQ tools (query, schema, cost) with render layer"
  - phase: 02-04
    provides: "TUI slash commands (/sync, /cost, /compact), cost tracker view interface"
provides:
  - "End-to-end BigQuery integration: tools registered, cache wired, prompts dynamic"
  - "initBigQuery helper for app assembly with graceful degradation"
  - "BuildSystemPrompt with conditional BQ context injection"
  - "Dynamic SQL risk classification for bigquery_query in agent loop"
  - "/sync wired to actual schema cache PopulateAll with prompt refresh"
  - "CostTracker adapter bridging bigquery types to TUI CostTrackerView"
  - "Lazy cache build on startup when datasets configured"
affects: [03-platform-tools, 04-data-engineering]

# Tech tracking
tech-stack:
  added: []
  patterns: ["BQQuerier adapter for interface compliance", "CostTrackerView adapter pattern for package boundary"]

key-files:
  created:
    - internal/app/bigquery.go
    - internal/app/prompt.go
    - internal/tui/cost_adapter.go
  modified:
    - internal/app/app.go
    - internal/agent/loop.go
    - internal/tui/model.go

key-decisions:
  - "Refactored buildClientConfig to return oauth2.TokenSource for BQ reuse (avoids duplicate credentials)"
  - "Created bqClientAdapter to bridge bq.Client.RunQuery concrete return to schema.RowIterator interface"
  - "CostTracker wiring uses adapter pattern (cost_adapter.go) to avoid importing internal/bigquery from TUI"
  - "Lazy cache build fires on app startup via background goroutine when datasets are configured"

patterns-established:
  - "Interface adapter: bqClientAdapter wraps concrete BQ client for schema.BQQuerier interface"
  - "Cross-package adapter: costTrackerAdapter bridges bigquery.CostTracker to tui.CostTrackerView"
  - "Graceful degradation: BQ init failure logs warning, does not block app startup"

requirements-completed: [BQ-01, BQ-04, BQ-05, BQ-06, BQ-07, BQ-08]

# Metrics
duration: 4min
completed: 2026-03-20
---

# Phase 2 Plan 5: BigQuery Integration Wiring Summary

**End-to-end BigQuery wiring: app assembly with BQ tools, dynamic SQL risk, schema-aware system prompt, /sync cache refresh, and cost tracker bridge**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-20T17:44:48Z
- **Completed:** 2026-03-20T17:49:06Z
- **Tasks:** 2 (1 auto + 1 checkpoint auto-approved)
- **Files modified:** 6

## Accomplishments
- Wired all BigQuery components into app assembly with graceful degradation when BQ is not configured
- Dynamic SQL risk classification for bigquery_query tool calls (SELECT=read-only, INSERT=DML, DROP=destructive)
- System prompt dynamically includes BigQuery dataset summary when schema cache is populated
- /sync command now triggers actual cache refresh via PopulateAll and updates system prompt
- Cost tracker adapter bridges bigquery.CostTracker to TUI CostTrackerView interface
- Lazy cache build on startup when datasets are configured

## Task Commits

Each task was committed atomically:

1. **Task 1: App assembly, dynamic SQL risk, system prompt injection, and /sync wiring** - `e2b520e` (feat)
2. **Task 2: Verify complete BigQuery integration end-to-end** - auto-approved checkpoint

**Plan metadata:** `ec7f536` (docs: complete plan)

## Files Created/Modified
- `internal/app/bigquery.go` - BigQueryComponents struct, initBigQuery, registerBQTools, EnsureCachePopulated, bqClientAdapter
- `internal/app/prompt.go` - BuildSystemPrompt with conditional BQ context injection
- `internal/app/app.go` - BQ field on App struct, wired init/register in New(), refactored buildClientConfig to return TokenSource
- `internal/agent/loop.go` - Dynamic SQL risk classification for bigquery_query using ClassifySQLRisk
- `internal/tui/model.go` - /sync wired to PopulateAll, cost tracker wired in NewModel
- `internal/tui/cost_adapter.go` - costTrackerAdapter bridging bigquery.CostTracker to CostTrackerView

## Decisions Made
- Refactored buildClientConfig to return (clientConfig, tokenSource, error) so BQ can reuse the same oauth2.TokenSource as the Vertex provider, avoiding duplicate credential creation
- Created bqClientAdapter in app package because bq.Client.RunQuery returns concrete *bigquery.RowIterator while schema.BQQuerier expects schema.RowIterator interface
- Cost tracker wiring uses adapter pattern (cost_adapter.go in tui package) to maintain clean package boundaries without importing internal/bigquery from TUI
- Lazy cache build fires as background goroutine on app startup when datasets are configured, non-blocking

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created bqClientAdapter for BQQuerier interface**
- **Found during:** Task 1 (App assembly)
- **Issue:** bq.Client.RunQuery returns *bigquery.RowIterator (concrete) but schema.BQQuerier interface expects schema.RowIterator (interface) -- compile error
- **Fix:** Created bqClientAdapter struct in bigquery.go that wraps *bq.Client and returns the interface type
- **Files modified:** internal/app/bigquery.go
- **Verification:** go build ./... passes
- **Committed in:** e2b520e (Task 1 commit)

**2. [Rule 3 - Blocking] Created costTrackerAdapter for CostTrackerView**
- **Found during:** Task 1 (TUI wiring)
- **Issue:** bigquery.CostTracker.Entries() returns []bigquery.QueryCostEntry but CostTrackerView.Entries() expects []tui.CostEntry -- type mismatch across package boundary
- **Fix:** Created cost_adapter.go in tui package with costTrackerAdapter that converts between types
- **Files modified:** internal/tui/cost_adapter.go, internal/tui/model.go
- **Verification:** go build ./... passes, go test ./... passes
- **Committed in:** e2b520e (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both adapters are necessary for interface compliance across package boundaries. Standard Go pattern, no scope creep.

## Issues Encountered
None - straightforward wiring with two expected interface adaptation needs.

## User Setup Required
None - no external service configuration required. BigQuery access uses existing GCP credentials.

## Next Phase Readiness
- Phase 2 (BigQuery Core) is now complete: all 5 plans executed
- Full end-to-end BigQuery integration available: schema cache, query execution, cost tracking, dynamic risk classification
- Ready for Phase 3 (Platform Tools) which builds on this foundation

---
*Phase: 02-bigquery-core*
*Completed: 2026-03-20*
