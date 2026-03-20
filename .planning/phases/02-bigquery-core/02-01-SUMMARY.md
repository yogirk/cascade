---
phase: 02-bigquery-core
plan: 01
subsystem: database
tags: [bigquery, sqlite, fts5, schema-cache, cost-estimation, sql-classification]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: Config struct, auth.NewTokenSource, permission.RiskLevel constants
provides:
  - BigQueryConfig and CostConfig in Config struct
  - BQ client wrapper with query, dry-run, close
  - SQL risk classification via prefix parsing
  - Session cost tracker with budget alerts
  - SQLite schema cache with FTS5 search
  - INFORMATION_SCHEMA bulk population logic
  - Schema context builder for LLM prompt injection
affects: [02-02-PLAN, 02-03-PLAN, 02-04-PLAN, 02-05-PLAN]

# Tech tracking
tech-stack:
  added: [cloud.google.com/go/bigquery v1.74.0, modernc.org/sqlite v1.47.0]
  patterns: [BQ client wrapper, SQL prefix classification, SQLite FTS5 schema cache, schema context injection]

key-files:
  created:
    - internal/bigquery/client.go
    - internal/bigquery/classify.go
    - internal/bigquery/classify_test.go
    - internal/bigquery/cost.go
    - internal/bigquery/cost_test.go
    - internal/schema/migrations.go
    - internal/schema/cache.go
    - internal/schema/cache_test.go
    - internal/schema/populate.go
    - internal/schema/search.go
    - internal/schema/context.go
  modified:
    - internal/config/config.go
    - go.mod
    - go.sum

key-decisions:
  - "Used content-bearing FTS5 instead of contentless (content='') for direct column value access in search results"
  - "SQL prefix parsing for risk classification modeled on existing ClassifyBashRisk pattern"
  - "Cost tracker uses mutex-guarded accumulator; negative cost signals DML (cannot estimate)"

patterns-established:
  - "BQ client wrapper: thin wrapper over bigquery.Client with oauth2.TokenSource injection"
  - "SQL classification: strip comments, uppercase first keyword, match prefix"
  - "Schema cache: SQLite with WAL mode, FTS5 for search, cache per GCP project"
  - "Populator interface: BQQuerier abstraction for testability"

requirements-completed: [BQ-02, BQ-03, BQ-05, BQ-08, BQ-09]

# Metrics
duration: 10min
completed: 2026-03-20
---

# Phase 2 Plan 1: BigQuery Foundation Summary

**BQ client wrapper, SQL risk classifier (27 cases), cost tracker, SQLite schema cache with FTS5 search, and LLM context builder**

## Performance

- **Duration:** 10 min
- **Started:** 2026-03-20T17:22:33Z
- **Completed:** 2026-03-20T17:33:03Z
- **Tasks:** 2
- **Files modified:** 14

## Accomplishments
- Config extended with BigQuery and Cost sections with production defaults ($6.25/TB, $1 warn, $10 max, $100/day budget)
- BQ client wrapper with query execution, dry-run cost estimation, and raw RowIterator access
- SQL risk classification passing 27 test cases across all 5 risk levels (ReadOnly, DML, DDL, Destructive, Admin)
- Thread-safe cost tracker with budget percentage calculation and 80% warning threshold
- SQLite schema cache with WAL mode, FTS5 full-text search, and BM25 ranking
- INFORMATION_SCHEMA bulk population with progress callbacks for 3 system views (TABLES, COLUMNS, TABLE_STORAGE)
- Schema context builder producing formatted markdown for LLM system prompt injection
- 46 total tests passing (32 bigquery + 14 schema)

## Task Commits

Each task was committed atomically:

1. **Task 1: Config extensions, BQ client, SQL classifier, cost tracker** - `0783eba` (feat)
2. **Task 2: SQLite schema cache with FTS5, population, search, context builder** - `436aa6e` (feat)

## Files Created/Modified
- `internal/config/config.go` - Extended with BigQueryConfig and CostConfig structs
- `internal/bigquery/client.go` - BQ client wrapper with query, dry-run, close, value formatting
- `internal/bigquery/classify.go` - SQL risk classification via prefix parsing with comment stripping
- `internal/bigquery/classify_test.go` - 27 classification test cases + 5 comment stripping tests
- `internal/bigquery/cost.go` - Thread-safe session cost tracker with budget alerts
- `internal/bigquery/cost_test.go` - 8 cost tracker tests including concurrency
- `internal/schema/migrations.go` - SQLite DDL for datasets, tables, columns, FTS5, index
- `internal/schema/cache.go` - Cache manager with Open/Close, CRUD, InvalidateTable
- `internal/schema/cache_test.go` - 14 tests covering WAL, FTS5, CRUD, invalidation, context
- `internal/schema/populate.go` - INFORMATION_SCHEMA bulk population with BQQuerier interface
- `internal/schema/search.go` - FTS5 Search and SearchColumns with BM25 ranking
- `internal/schema/context.go` - BuildSchemaContext and BuildDatasetSummary for LLM injection
- `go.mod` - Added direct bigquery and sqlite dependencies
- `go.sum` - Updated checksums

## Decisions Made
- Used content-bearing FTS5 table instead of contentless (`content=''`). Contentless FTS5 returns NULL for stored columns in SELECT, making it impossible to retrieve dataset_id/table_id from search results without a separate rowid mapping. Content-bearing adds ~20% storage overhead but provides direct access to indexed values.
- SQL prefix parsing modeled on existing `ClassifyBashRisk` pattern in `internal/tools/core/bash.go`. Both use keyword-based classification with conservative defaults (unknown = destructive).
- Cost tracker treats negative cost values (-1) as "cannot estimate" signals for DML dry-runs where BigQuery returns 0 bytes processed. Only positive costs are accumulated in session totals.
- Populator uses `BQQuerier` interface (RunQuery + ProjectID) for testability, decoupling from concrete BigQuery client.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed unterminated block comment handling in stripSQLComments**
- **Found during:** Task 1 (SQL classifier tests)
- **Issue:** Unterminated block comments (e.g., `/* never closes`) left residual characters in output
- **Fix:** Added explicit `found` flag to block comment loop; break cleanly when comment is unterminated
- **Files modified:** internal/bigquery/classify.go
- **Verification:** TestStripSQLComments/unterminated_block passes
- **Committed in:** 0783eba (Task 1 commit)

**2. [Rule 1 - Bug] Changed FTS5 from contentless to content-bearing**
- **Found during:** Task 2 (FTS5 search tests)
- **Issue:** Contentless FTS5 (`content=''`) returns NULL for dataset_id and table_id columns, causing scan errors
- **Fix:** Removed `content=''` from FTS5 CREATE VIRTUAL TABLE; updated InvalidateTable to use direct DELETE instead of insert-with-delete pattern
- **Files modified:** internal/schema/migrations.go, internal/schema/cache.go, internal/schema/search.go
- **Verification:** TestFTS5Search, TestSearchColumns, TestBuildSchemaContext all pass
- **Committed in:** 436aa6e (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both fixes necessary for correctness. No scope creep. FTS5 content-bearing approach is slightly more storage but functionally correct.

## Issues Encountered
- `go mod tidy` reverted `modernc.org/sqlite` from v1.47.0 to v1.21.2 because indirect dependencies pulled the older version. Required explicit `go get modernc.org/sqlite@v1.47.0` followed by `go mod tidy` to pin correctly.
- Pre-existing staged change to `internal/tui/model.go` (compaction feature from prior session) was in the index. Unstaged it to keep commits focused on plan scope.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- BigQuery client wrapper ready for tool integration (Plan 02-02: BigQuery Query Tool)
- Schema cache ready for population via /sync command (Plan 02-04: TUI Integration)
- SQL classifier ready for agent loop dynamic risk assessment
- Cost tracker ready for status bar integration

---
*Phase: 02-bigquery-core, Plan: 01*
*Completed: 2026-03-20*
