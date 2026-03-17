---
phase: 1
slug: foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` stdlib + `go-cmp` v0.7.0 |
| **Config file** | None (Go convention: tests alongside source) |
| **Quick run command** | `go test ./internal/... -short -count=1` |
| **Full suite command** | `go test ./... -count=1 -race` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./internal/... -short -count=1`
- **After every plan wave:** Run `go test ./... -count=1 -race`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | AGNT-01 | unit | `go test ./internal/agent/ -run TestStreamingTokenDelivery -count=1` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | AGNT-02 | unit | `go test ./internal/agent/ -run TestLoopGovernor -count=1` | ❌ W0 | ⬜ pending |
| 01-01-03 | 01 | 1 | AGNT-03 | integration | `go test ./internal/tui/ -run TestMarkdownRendering -count=1` | ❌ W0 | ⬜ pending |
| 01-01-04 | 01 | 1 | AGNT-04 | unit | `go test ./internal/provider/gemini/ -run TestGeminiProvider -count=1` | ❌ W0 | ⬜ pending |
| 01-01-05 | 01 | 1 | AGNT-05 | integration | `go test ./internal/oneshot/ -run TestOneShotMode -count=1` | ❌ W0 | ⬜ pending |
| 01-01-06 | 01 | 1 | AGNT-07 | unit | `go test ./internal/tools/core/ -run TestCoreTool -count=1` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 1 | AUTH-01 | unit (mock) | `go test ./internal/auth/ -run TestADCTokenSource -count=1` | ❌ W0 | ⬜ pending |
| 01-02-02 | 02 | 1 | AUTH-02 | unit (mock) | `go test ./internal/auth/ -run TestImpersonation -count=1` | ❌ W0 | ⬜ pending |
| 01-02-03 | 02 | 1 | AUTH-03 | unit | `go test ./internal/permission/ -run TestRiskClassification -count=1` | ❌ W0 | ⬜ pending |
| 01-02-04 | 02 | 1 | AUTH-04 | unit | `go test ./internal/permission/ -run TestPermissionDecision -count=1` | ❌ W0 | ⬜ pending |
| 01-02-05 | 02 | 1 | AUTH-05 | unit | `go test ./internal/permission/ -run TestModeCycling -count=1` | ❌ W0 | ⬜ pending |
| 01-02-06 | 02 | 1 | AUTH-07 | unit (mock) | `go test ./internal/auth/ -run TestRetryOn401 -count=1` | ❌ W0 | ⬜ pending |
| 01-03-01 | 03 | 1 | UX-01 | integration | `go test ./internal/config/ -run TestConfigLoading -count=1` | ❌ W0 | ⬜ pending |
| 01-03-02 | 03 | 1 | UX-04 | unit | `go test ./internal/tui/ -run TestKeyboardShortcuts -count=1` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/agent/agent_test.go` — stubs for AGNT-01, AGNT-02 (streaming delivery, loop governor)
- [ ] `internal/tui/tui_test.go` — stubs for AGNT-03, UX-04 (markdown rendering, keyboard shortcuts)
- [ ] `internal/provider/gemini/gemini_test.go` — stubs for AGNT-04 (provider interface)
- [ ] `internal/tools/core/core_test.go` — stubs for AGNT-07 (core tool execution)
- [ ] `internal/auth/auth_test.go` — stubs for AUTH-01, AUTH-02, AUTH-07 (ADC, impersonation, retry)
- [ ] `internal/permission/permission_test.go` — stubs for AUTH-03, AUTH-04, AUTH-05 (risk classification, decisions, modes)
- [ ] `internal/config/config_test.go` — stubs for UX-01 (config loading)
- [ ] `go-cmp` v0.7.0 added to go.mod

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Streaming output visual rendering | AGNT-01 | Visual rendering quality requires human eye | Launch `cascade`, ask a question, verify streaming text appears character-by-character with markdown formatting |
| Permission prompt UX | AUTH-04 | Interactive prompt requires human interaction | Attempt a DML operation, verify warning badge and y/N prompt appear correctly |
| Shift+Tab mode cycling | AUTH-05 | Keyboard interaction in TUI | Press Shift+Tab, verify mode badge cycles CONFIRM → PLAN → BYPASS |
| One-shot mode exit behavior | AGNT-05 | CLI exit behavior | Run `cascade -p "hello"`, verify clean output and exit code 0 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
