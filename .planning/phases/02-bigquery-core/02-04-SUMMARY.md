---
phase: 02-bigquery-core
plan: 04
subsystem: ui
tags: [lipgloss, bubbletea, tui, bigquery, cost-display, slash-commands]

# Dependency graph
requires:
  - phase: 02-01
    provides: "CostTracker, QueryCostEntry, cost estimation types"
  - phase: 01-04
    provides: "TUI foundation: styles.go, status.go, model.go, confirm.go, spinner.go"
provides:
  - "BigQuery-specific TUI styles (QueryTableHeader, CostSafe/Warn/Danger, SchemaHeader, etc.)"
  - "CostUpdateEvent in event system for BQ cost display updates"
  - "DDL risk badge separate from DESTRUCTIVE"
  - "BigQuery tool bullets (green read, amber query)"
  - "SQL preview truncation in confirm prompt"
  - "BigQuery spinner messages"
  - "Status bar budget warning at 80% threshold"
  - "/cost slash command with per-query breakdown table"
  - "/sync slash command handler (wiring deferred to Plan 05)"
  - "CostTrackerView interface and SetCostTracker for Plan 05 wiring"
affects: [02-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "CostTrackerView interface decouples TUI from bigquery package"
    - "CostEntry TUI-local mirror avoids cross-package import"

key-files:
  created: []
  modified:
    - internal/tui/styles.go
    - internal/tui/status.go
    - internal/tui/model.go
    - internal/tui/confirm.go
    - internal/tui/spinner.go
    - pkg/types/event.go

key-decisions:
  - "CostTrackerView interface in tui package avoids importing internal/bigquery directly"
  - "CostEntry mirror type in tui package for display decoupling"
  - "DDL badge uses warningColor (amber) to differentiate from DESTRUCTIVE (red)"
  - "/sync command handler structure ready but actual cache refresh deferred to Plan 05 wiring"

patterns-established:
  - "BigQuery styles reuse existing color tokens only -- zero new colors"
  - "Tool bullets follow color convention: green=read-only, amber=write-capable"

requirements-completed: [BQ-07]

# Metrics
duration: 4min
completed: 2026-03-20
---

# Phase 2 Plan 4: BigQuery TUI Extensions Summary

**BigQuery TUI styles, cost-aware status bar with budget warnings, /cost and /sync slash commands, DDL risk badge, SQL preview in confirm prompts, and BQ spinner messages**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-20T17:36:14Z
- **Completed:** 2026-03-20T17:40:23Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Added 8 BigQuery-specific styles using existing color tokens (QueryTableHeader, CostSafe/Warn/Danger, SchemaHeader, SchemaAnnotation, QueryTableCell, QueryTableNull)
- Extended ToolBullet with BigQuery tools (green for schema/cost, amber for query) and separated DDL risk badge from DESTRUCTIVE
- Added CostUpdateEvent to event system and handler in TUI update loop
- Status bar shows budget warning in amber when cost reaches 80% of daily budget
- /cost shows detailed per-query breakdown table with bytes, cost, time, and budget line
- /sync command handler ready for Plan 05 wiring
- SQL preview truncated at 6 lines in confirm prompt for bigquery_query tool
- BigQuery-specific spinner messages for query/schema/cost tools

## Task Commits

Each task was committed atomically:

1. **Task 1: BigQuery styles, tool bullets, risk badge, spinner messages, and confirm SQL preview** - `6f486a4` (feat)
2. **Task 2: Status bar budget warning, /cost breakdown, /sync command, and event handling** - `1c044f3` (feat)

## Files Created/Modified
- `pkg/types/event.go` - Added CostUpdateEvent struct with QueryCost, SessionTotal, BytesScanned fields
- `internal/tui/styles.go` - Added 8 BigQuery styles, DDL badge, BigQuery tool bullet cases
- `internal/tui/confirm.go` - Added formatArgsSummary cases for bigquery_query (6-line SQL truncation), bigquery_schema, bigquery_cost
- `internal/tui/spinner.go` - Added BigQuery tool messages (Executing query, Looking up schema, Estimating cost)
- `internal/tui/status.go` - Added dailyBudget field with SetDailyBudget setter and 80% budget warning rendering
- `internal/tui/model.go` - Added CostTrackerView interface, CostUpdateEvent handler, /cost and /sync commands, helper functions, updated /help

## Decisions Made
- CostTrackerView interface defined in tui package to avoid importing internal/bigquery -- Plan 05 wires the concrete CostTracker
- CostEntry mirror type in tui package keeps display code decoupled from storage types
- DDL badge uses warningColor (amber) distinct from DESTRUCTIVE badge (dangerColor/red) per UI spec
- /sync command handler validates config but defers actual cache refresh to Plan 05 app assembly

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All BigQuery TUI elements ready for Plan 05 (app assembly) to wire CostTracker and schema cache
- CostTrackerView interface provides clean integration point
- /sync command needs cache populator wiring in Plan 05
- All 14 existing TUI tests pass

## Self-Check: PASSED

All 6 modified files verified on disk. Both task commits (6f486a4, 1c044f3) verified in git log.

---
*Phase: 02-bigquery-core*
*Completed: 2026-03-20*
