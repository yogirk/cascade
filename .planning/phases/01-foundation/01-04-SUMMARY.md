---
phase: 01-foundation
plan: 04
subsystem: tui
tags: [bubbletea, lipgloss, glamour, cobra, tui, streaming, oneshot, cli]

# Dependency graph
requires:
  - phase: 01-foundation/03
    provides: "Agent loop with event emission, App struct with Events channel"
provides:
  - "Bubble Tea TUI with streaming token rendering at 30fps"
  - "Ring buffer + render tick architecture for deadlock-free streaming"
  - "Inline permission confirmation with risk badges"
  - "Keyboard shortcuts: Ctrl+C, Ctrl+D, Ctrl+L, Shift+Tab"
  - "Permission mode cycling via Shift+Tab (CONFIRM/PLAN/BYPASS)"
  - "One-shot runner for -p flag and stdin piping"
  - "Cobra CLI entry point with cascade/csc commands"
  - "Working cascade binary with interactive and one-shot modes"
affects: [phase-02, phase-06]

# Tech tracking
tech-stack:
  added: [bubbletea-v2, lipgloss-v2, glamour-v2, bubbles-v2, cobra]
  patterns: [ring-buffer-streaming, event-driven-tui, altscreen-via-view]

key-files:
  created:
    - internal/tui/model.go
    - internal/tui/chat.go
    - internal/tui/input.go
    - internal/tui/status.go
    - internal/tui/confirm.go
    - internal/tui/spinner.go
    - internal/tui/styles.go
    - internal/tui/keys.go
    - internal/tui/renderer.go
    - internal/tui/tui_test.go
    - internal/oneshot/runner.go
    - internal/oneshot/runner_test.go
  modified:
    - cmd/cascade/main.go

key-decisions:
  - "Bubble Tea v2 removed KeyBinding type; used custom KeyDef with string matching"
  - "Lip Gloss v2 replaced AdaptiveColor with plain Color values"
  - "BT v2 View() returns tea.View not string; AltScreen set on View struct"
  - "One-shot processEvents extracted as testable function to avoid mocking full App"
  - "Glamour v2 uses WithEnvironmentConfig() instead of WithAutoStyle()"

patterns-established:
  - "Ring buffer + 30fps tick: StreamRenderer.Push() non-blocking, DrainAll() on tick"
  - "Event-driven TUI: agentEventMsg wraps types.Event from channel polling"
  - "UIState enum: StateIdle/StateStreaming/StateToolExecuting/StateConfirming"
  - "Component model: ChatModel, InputModel, StatusModel, SpinnerModel, ConfirmModel"

requirements-completed: [AGNT-03, AGNT-05, UX-04]

# Metrics
duration: 14min
completed: 2026-03-17
---

# Phase 1 Plan 4: TUI and CLI Summary

**Bubble Tea v2 TUI with 30fps streaming rendering, inline permission prompts, Cobra CLI with one-shot mode via -p flag**

## Performance

- **Duration:** 14 min
- **Started:** 2026-03-17T17:57:34Z
- **Completed:** 2026-03-17T18:11:40Z
- **Tasks:** 3 (2 auto + 1 checkpoint auto-approved)
- **Files modified:** 13

## Accomplishments
- Bubble Tea TUI renders streaming LLM tokens via ring buffer at 30fps without deadlock
- One-shot mode streams tokens to stdout, auto-denies DML in non-bypass mode
- Cobra CLI with cascade/csc alias, -p flag, --model, --bypass, --config flags
- Inline permission confirmation with risk badges and y/N prompt
- Permission mode cycling via Shift+Tab with status bar badge
- Glamour markdown rendering on completed messages
- 19 tests total across tui and oneshot packages, all passing with race detector

## Task Commits

Each task was committed atomically:

1. **Task 1: Bubble Tea TUI with streaming, permissions, and keyboard shortcuts** - `ae2138b` (feat)
2. **Task 2: One-shot runner and Cobra CLI entry point** - `4a179c8` (feat)
3. **Task 3: Verify Cascade launches and works end-to-end** - auto-approved checkpoint (no code changes)

## Files Created/Modified
- `internal/tui/model.go` - Root Bubble Tea model with Init/Update/View, event polling, tick loop
- `internal/tui/chat.go` - Chat viewport with Glamour markdown rendering
- `internal/tui/input.go` - Multi-line textarea input with placeholder
- `internal/tui/status.go` - Status bar showing model name, permission mode badge, version
- `internal/tui/confirm.go` - Inline permission confirmation with risk badges
- `internal/tui/spinner.go` - Tool execution spinner with tool name
- `internal/tui/styles.go` - Lip Gloss v2 styles for all TUI components
- `internal/tui/keys.go` - Keyboard shortcuts: Ctrl+C, Ctrl+D, Ctrl+L, Shift+Tab, Enter
- `internal/tui/renderer.go` - Ring buffer (cap 256) + DrainAll for 30fps streaming
- `internal/tui/tui_test.go` - 14 tests for renderer, keys, chat, confirm
- `internal/oneshot/runner.go` - One-shot event consumer writing to stdout/stderr
- `internal/oneshot/runner_test.go` - 5 tests for token output, permission deny/bypass, errors
- `cmd/cascade/main.go` - Cobra root command with -p, --model, --bypass, --config flags

## Decisions Made
- Bubble Tea v2 API differs significantly from v1: no KeyBinding type, View() returns tea.View, AltScreen via View struct field, viewport uses functional options. Adapted all code to v2 API.
- Lip Gloss v2 replaced AdaptiveColor with plain lipgloss.Color() calls; LightDark is available but not needed for our color scheme.
- Glamour v2 replaced WithAutoStyle() with WithEnvironmentConfig() for terminal-aware markdown rendering.
- Extracted processEvents as a package-level function in oneshot package for direct testability without mocking the full App.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adapted to Bubble Tea v2 API changes**
- **Found during:** Task 1 (TUI implementation)
- **Issue:** Plan assumed v1 API patterns (KeyBinding, AdaptiveColor, View() string, viewport.New(w,h)). BT v2 removed KeyBinding, changed View to tea.View, viewport uses functional options.
- **Fix:** Created custom KeyDef type with string matching, used tea.NewView(), viewport.New(viewport.WithWidth(), viewport.WithHeight()), set AltScreen on View struct
- **Files modified:** keys.go, model.go, chat.go, styles.go, spinner.go
- **Verification:** go build ./... exits 0, all tests pass
- **Committed in:** ae2138b (Task 1 commit)

**2. [Rule 3 - Blocking] Missing go.sum entries for Charm v2 transitive dependencies**
- **Found during:** Task 1 (initial build)
- **Issue:** clipboard package missing from go.sum
- **Fix:** Ran go get for missing module, go mod tidy
- **Files modified:** go.mod, go.sum
- **Verification:** go build ./... exits 0
- **Committed in:** ae2138b (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 blocking)
**Impact on plan:** Both fixes necessary to adapt to Charm v2 API. No scope creep.

## Issues Encountered
- Charm v2 stack has significant API differences from v1 that aren't well-documented yet. Required reading actual source files to discover correct types and function signatures.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 1 Foundation is now complete: all 4 plans delivered
- Working `cascade` binary with interactive TUI and one-shot mode
- Ready for Phase 2: BigQuery tools, schema cache, and extended agent capabilities
- Prerequisite for interactive testing: GOOGLE_API_KEY or `gcloud auth application-default login`

## Self-Check: PASSED

All 13 created/modified files verified present. Both task commits (ae2138b, 4a179c8) verified in git log.

---
*Phase: 01-foundation*
*Completed: 2026-03-17*
