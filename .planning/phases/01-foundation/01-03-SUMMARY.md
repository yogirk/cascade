---
phase: 01-foundation
plan: 03
subsystem: agent
tags: [go, agent-loop, governor, session, events, permission-gating, streaming, app-wiring]

# Dependency graph
requires:
  - phase: 01-foundation-01
    provides: "Provider interface, Stream abstraction, MockProvider, Message/Event types"
  - phase: 01-foundation-02
    provides: "Tool interface, Registry, 6 core tools, Permission engine with modes"
provides:
  - "Agent loop with observe-reason-act cycle and streaming token forwarding"
  - "Loop governor with tool call limits, duplicate detection, progress nudges"
  - "Session tracking for multi-turn conversation history"
  - "EventHandler interface decoupling agent from TUI/one-shot consumers"
  - "Permission gating with dynamic bash risk classification in the loop"
  - "RegisterAll wiring all 6 core tools into the registry"
  - "App struct assembling config, auth, provider, tools, permissions, agent"
affects: [01-04]

# Tech tracking
tech-stack:
  added: []
  patterns: [observe-reason-act loop, governor pattern for loop safety, channel-based event handler, dynamic risk classification at call site, multi-response mock provider for testing]

key-files:
  created: [internal/agent/loop.go, internal/agent/governor.go, internal/agent/events.go, internal/agent/session.go, internal/agent/agent_test.go, internal/app/app.go, internal/tools/core/register.go]
  modified: []

key-decisions:
  - "Event types emitted as pointers to match sealed interface with pointer receivers"
  - "collectEvents test helper uses timeout-based drain instead of channel close to avoid race with async token goroutines"
  - "Governor reset happens at start of each turn, not at end, to ensure clean state"
  - "Default MaxToolCalls set to 15 when config provides 0 or negative value"

patterns-established:
  - "Agent loop pattern: stream tokens in goroutine, wait for Result(), process tool calls, loop"
  - "Governor pattern: CheckLimit before LLM call, IsDuplicate per tool call, ShouldNudge for progress"
  - "EventHandler interface: channel-based (EventChan) for production, slice-based for tests"
  - "Multi-response mock provider: slice of responses consumed sequentially for multi-round tests"
  - "Dynamic risk tool wrapper: dynamicRiskTool overrides RiskLevel for bash command classification"

requirements-completed: [AGNT-01, AGNT-02]

# Metrics
duration: 7min
completed: 2026-03-17
---

# Phase 1 Plan 03: Agent Loop and App Wiring Summary

**Observe-reason-act agent loop with governor safety (limits, duplicates, nudges), permission-gated tool execution, streaming event emission, and App struct wiring all components**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-17T17:46:00Z
- **Completed:** 2026-03-17T17:53:25Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Implemented core agent loop driving conversation: streams LLM tokens, executes tools through registry with permission checks, loops until text-only response
- Built loop governor enforcing MaxToolCalls limit, detecting duplicate tool calls (same name+args hash), and injecting progress nudges every 5 calls
- Created Session for multi-turn conversation history with system prompt support
- Created EventHandler interface with EventChan implementation for TUI/one-shot decoupling
- Built App struct that wires config, auth, provider, tools, permissions, and agent into a single assembly point
- Added RegisterAll to wire all 6 core tools into the registry
- 19 tests covering session, governor, and full agent loop scenarios (text-only, tool call, unknown tool, permission confirm, permission deny, governor limit, duplicate detection, tool error)

## Task Commits

Each task was committed atomically:

1. **Task 1: Agent loop, governor, session, events, and app wiring** - `4b879c7` (feat)
2. **Task 2: RegisterAll to wire core tools into registry** - `1390585` (feat)

## Files Created/Modified
- `internal/agent/loop.go` - Core observe-reason-act loop with Agent struct, RunTurn, executeWithPermission
- `internal/agent/governor.go` - Governor with CheckLimit, IsDuplicate, ShouldNudge, Reset
- `internal/agent/events.go` - EventHandler interface and EventChan channel implementation
- `internal/agent/session.go` - Session with Append, AppendSystem, Messages, NewSession
- `internal/agent/agent_test.go` - 19 tests: session, governor, and agent loop scenarios
- `internal/app/app.go` - App struct assembling all components with New constructor
- `internal/tools/core/register.go` - RegisterAll registering 6 core tools

## Decisions Made
- Event types emitted as pointers (`&types.TokenEvent{}`) to match the sealed interface's pointer receivers
- Test helper uses timeout-based drain (200ms) instead of closing the event channel, avoiding race conditions with async token-forwarding goroutines
- Governor resets at the start of each turn (not end) to ensure clean duplicate tracking state
- Default MaxToolCalls is 15 when config provides 0 or negative -- matches the spec's 15-20 range

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed event channel race condition in test helper**
- **Found during:** Task 1 (agent loop tests)
- **Issue:** Original `collectEvents` helper closed the event channel while async token-forwarding goroutines were still sending, causing "send on closed channel" panic
- **Fix:** Replaced channel-close pattern with timeout-based drain (200ms deadline after RunTurn returns)
- **Files modified:** `internal/agent/agent_test.go`
- **Verification:** All 19 tests pass, including with race detector
- **Committed in:** 4b879c7 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Test infrastructure fix only. No functional code changes from plan.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Agent loop ready for TUI integration (Plan 04) -- EventHandler interface is the contract
- App struct ready to be called from CLI entry point
- RegisterAll provides single-call tool registration for App.New
- All tests green with race detector (`go test ./... -count=1 -race`)
- `go vet ./...` clean, binary builds

## Self-Check: PASSED

- All 7 key files: FOUND
- All 2 task commits: FOUND (4b879c7, 1390585)
- `go build ./...`: exits 0
- `go test ./... -count=1 -race`: all pass
- `go vet ./...`: no warnings
- No genai imports outside internal/provider/gemini/

---
*Phase: 01-foundation*
*Completed: 2026-03-17*
