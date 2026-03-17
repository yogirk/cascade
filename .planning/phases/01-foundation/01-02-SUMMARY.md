---
phase: 01-foundation
plan: 02
subsystem: tools
tags: [go, tools, permission, risk, registry, bash, glob, grep, udiff, doublestar]

# Dependency graph
requires:
  - phase: 01-foundation-01
    provides: "Go module, Provider interface with Declaration type, permission.RiskLevel type reference"
provides:
  - "Tool interface with Name, Description, InputSchema, RiskLevel, Execute"
  - "Tool registry with lookup and LLM declaration generation"
  - "6 core file tools: read, write, edit, glob, grep, bash"
  - "Bash risk classification (read-only/DML/destructive)"
  - "Permission engine with CONFIRM/PLAN/BYPASS modes"
  - "Session-cached permission decisions"
  - "Mode cycling (Shift+Tab support)"
affects: [01-03, 01-04]

# Tech tracking
tech-stack:
  added: [github.com/bmatcuk/doublestar/v4 v4.8.1, github.com/aymanbagabas/go-udiff v0.4.1]
  patterns: [Tool interface with self-describing schema, registry pattern for tool discovery, ToolRiskProvider interface to avoid circular imports, dynamic bash risk classification]

key-files:
  created: [internal/tools/tool.go, internal/tools/registry.go, internal/tools/result.go, internal/tools/core/read.go, internal/tools/core/write.go, internal/tools/core/edit.go, internal/tools/core/glob.go, internal/tools/core/grep.go, internal/tools/core/bash.go, internal/tools/core/tools_test.go, internal/permission/risk.go, internal/permission/mode.go, internal/permission/engine.go, internal/permission/engine_test.go]
  modified: [go.mod, go.sum]

key-decisions:
  - "Used udiff.Unified() helper instead of lower-level ToUnifiedDiff for simpler diff generation in edit tool"
  - "ToolRiskProvider interface in permission package avoids circular dependency -- permission never imports tools"
  - "Bash tool defaults to RiskDestructive; ClassifyBashRisk exported for agent loop to call before permission check"
  - "Glob tool returns absolute paths to avoid ambiguity in tool results"

patterns-established:
  - "Tool interface pattern: each tool self-describes via Name/Description/InputSchema/RiskLevel"
  - "Registry pattern: tools registered by name, Declarations() generates LLM-compatible schema list"
  - "ToolRiskProvider: narrow interface (Name+RiskLevel) lets permission package check tools without importing tools package"
  - "Dynamic risk classification: bash commands parsed to determine risk level at runtime"
  - "Permission engine modes: CONFIRM (auto-approve reads), PLAN (deny writes), BYPASS (allow all)"

requirements-completed: [AGNT-07, AUTH-03, AUTH-04, AUTH-05]

# Metrics
duration: 7min
completed: 2026-03-17
---

# Phase 1 Plan 02: Tool System and Permission Engine Summary

**6 core file tools with registry-based discovery, bash risk classification, and 3-mode permission engine with session caching**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-17T17:34:05Z
- **Completed:** 2026-03-17T17:41:33Z
- **Tasks:** 2
- **Files modified:** 16

## Accomplishments
- Implemented Tool interface and Registry with LLM declaration generation for all 6 core tools
- Built 6 file tools (read/write/edit/glob/grep/bash) with full JSON input parsing, error handling, and structured results
- Created permission engine enforcing CONFIRM/PLAN/BYPASS modes with session-cached decisions and mode cycling
- Dynamic bash risk classification: read-only commands auto-approved, git write ops = DML, destructive patterns caught

## Task Commits

Each task was committed atomically:

1. **Task 1: Tool interface, registry, and 6 core file tools** - `8113650` (feat)
2. **Task 2: Permission engine with risk classification and mode cycling** - `9026b57` (feat)

_Both tasks followed TDD: tests written first (RED), implementation second (GREEN)._

## Files Created/Modified
- `internal/tools/tool.go` - Tool interface (Name, Description, InputSchema, RiskLevel, Execute)
- `internal/tools/registry.go` - Registry with Get, All, Declarations for LLM tool discovery
- `internal/tools/result.go` - Result struct (Content, Display, IsError)
- `internal/tools/core/read.go` - ReadTool: file reading with offset/limit and cat -n line numbers
- `internal/tools/core/write.go` - WriteTool: file creation with auto-mkdir parents
- `internal/tools/core/edit.go` - EditTool: string replacement with unified diff output
- `internal/tools/core/glob.go` - GlobTool: doublestar pattern matching with 1000 result limit
- `internal/tools/core/grep.go` - GrepTool: regex search with line numbers, include filter, 500 match limit
- `internal/tools/core/bash.go` - BashTool: shell execution + ClassifyBashRisk dynamic classification
- `internal/tools/core/tools_test.go` - 20 tests: compile-time checks, all tool Execute paths, bash risk cases
- `internal/permission/risk.go` - RiskLevel enum with String, Badge, RequiresConfirmation
- `internal/permission/mode.go` - Mode enum (CONFIRM/PLAN/BYPASS) with CycleMode
- `internal/permission/engine.go` - Engine with Check, CacheDecision, CycleMode, ToolRiskProvider interface
- `internal/permission/engine_test.go` - 17 tests: all risk/mode/engine combinations, caching, cycling

## Decisions Made
- Used `udiff.Unified()` for edit tool diffs -- simpler API than lower-level `ToUnifiedDiff` which requires context line count
- Created `ToolRiskProvider` interface in permission package instead of importing tools.Tool -- avoids circular dependency
- Bash tool's `RiskLevel()` method returns RiskDestructive as default; agent loop calls `ClassifyBashRisk(cmd)` for dynamic classification
- Glob tool returns absolute paths for unambiguous tool results
- Grep tool skips hidden directories and files >1MB (binary heuristic)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed go-udiff API call for v0.4.1**
- **Found during:** Task 1 (Edit tool implementation)
- **Issue:** Plan referenced `udiff.ToUnifiedDiff` and `myers.ComputeEdits` which have different signatures in v0.4.1 (requires context lines param, returns tuple)
- **Fix:** Used `udiff.Unified()` helper which takes old/new strings directly and returns a diff string
- **Files modified:** `internal/tools/core/edit.go`
- **Verification:** `go build ./...` succeeds, edit tool test passes
- **Committed in:** 8113650 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor API adjustment. Simpler code than planned. No functional impact.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Tool interface and registry ready for agent loop integration (Plan 03)
- Permission engine ready for agent loop permission gating (Plan 03)
- All tools self-describe via InputSchema() for LLM declaration generation
- ClassifyBashRisk exported for agent loop to call before permission check
- All tests green with race detector (`go test ./... -count=1 -race`)

## Self-Check: PASSED

- All 14 key files: FOUND
- All 2 task commits: FOUND (8113650, 9026b57)
- `go build ./...`: exits 0
- `go test ./internal/tools/... ./internal/permission/ -count=1`: all pass
- No circular imports (permission does not import tools)

---
*Phase: 01-foundation*
*Completed: 2026-03-17*
