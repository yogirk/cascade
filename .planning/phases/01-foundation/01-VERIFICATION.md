---
phase: 01-foundation
verified: 2026-03-18T00:00:00Z
status: gaps_found
score: 4/5 success criteria verified
re_verification: false
gaps:
  - truth: "go.mod contains google.golang.org/adk as declared in plan 01-01 must_haves artifact"
    status: failed
    reason: "go.mod does not contain google.golang.org/adk. The implementation used google.golang.org/genai v1.50.0 directly. The plan explicitly allowed this fallback (Task 3 action note: 'If ADK Go v0.6.0's gemini package constructor signature differs, fall back to using google.golang.org/genai client directly'), and the 01-01-SUMMARY.md documents this decision. However, AGNT-04 requires 'Google ADK Go' by name, and the plan artifact contract states go.mod must contain the adk import. The functional requirement (Provider interface + Gemini 2.5 Pro as default) IS fully satisfied. This gap is a traceability discrepancy between the requirement text and the delivered implementation."
    artifacts:
      - path: "go.mod"
        issue: "Contains google.golang.org/genai v1.50.0 but not google.golang.org/adk. ADK is absent from both go.mod and go.sum."
    missing:
      - "Either: add google.golang.org/adk to go.mod (even as a direct dependency wrapping genai) to satisfy AGNT-04 literal text, OR update AGNT-04 to reflect the approved deviation (use genai SDK directly, abstracted behind Provider interface). The functional implementation is complete -- this is a requirements traceability issue, not a code gap."
human_verification:
  - test: "Launch `./bin/cascade` interactively, type a question, observe streaming output and permission flow"
    expected: "TUI appears with input area, chat area, and status bar. Streaming LLM tokens appear character-by-character. Shift+Tab cycles mode badge. DML tool triggers inline permission prompt with [y/N]. Ctrl+D exits cleanly."
    why_human: "Terminal UI behavior, streaming render quality, and keyboard interaction cannot be verified programmatically."
  - test: "Run `./bin/cascade -p 'list files in current directory'`"
    expected: "Streaming text output to stdout. Tool executes silently. Process exits with code 0."
    why_human: "One-shot mode output quality and clean exit behavior require runtime observation."
  - test: "Run with ADC credentials: `gcloud auth application-default login` then `./bin/cascade -p 'say hello'`"
    expected: "Auth succeeds with zero additional setup. Session continues past 1-hour token window without error."
    why_human: "Live GCP auth flow and token refresh require real credentials and time observation."
---

# Phase 1: Foundation Verification Report

**Phase Goal:** User can have a streaming conversation with an AI agent in the terminal that authenticates to GCP, executes file tools, and enforces risk-based permissions
**Verified:** 2026-03-18
**Status:** gaps_found (1 traceability gap; all functional goals achieved)
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (from ROADMAP.md Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | User can launch `cascade`, type a question, and see streaming LLM output rendered with markdown formatting and syntax highlighting | VERIFIED | `internal/tui/model.go` drives token streaming via ring buffer at 30fps; `chat.go:97` applies `renderMarkdown()` (Glamour) on assistant messages at completion; `internal/tui/renderer.go` implements non-blocking push with cap-256 channel |
| 2 | User can execute core file operations (read, write, edit, glob, grep, bash) through conversational requests | VERIFIED | All 6 tools exist in `internal/tools/core/`; `register.go` wires all into registry; agent loop resolves tools via `a.registry.Get()` and executes via `tool.Execute()`; `go test ./internal/tools/core/` passes |
| 3 | User can authenticate via `gcloud auth application-default login` with zero additional setup, and sessions survive the 1-hour ADC token refresh | VERIFIED | `internal/auth/gcp.go:47` calls `google.FindDefaultCredentials` with error message "Run: gcloud auth application-default login"; `oauth2.ReuseTokenSource` wraps result; `internal/auth/retry.go` implements `RetryOn401` + `RetryTransport`; `go test ./internal/auth/` passes |
| 4 | READ_ONLY operations auto-approve; DML and above require explicit user confirmation | VERIFIED | `internal/permission/engine.go`: ModeConfirm returns Allow for `<=RiskReadOnly`, Confirm for DML+; `internal/tui/confirm.go` renders inline prompt and sends on `PermissionRequestEvent.Response` channel; `go test ./internal/permission/` passes |
| 5 | User can run `cascade -p "..."` for one-shot scripting, or launch interactive mode by default | VERIFIED | `cmd/cascade/main.go:33` defines `-p`/`--prompt` flag; one-shot path calls `oneshot.Run()`; interactive path calls `tui.NewModel()` + `tea.NewProgram()`; `./bin/cascade --version` prints "cascade version 0.1.0-dev"; binary builds clean |

**Score:** 4/5 truths fully verified (Truth 3 needs human confirmation for live GCP; automated wiring verified)

### Required Artifacts

#### Plan 01-01: Types, Config, Auth, Provider

| Artifact | Status | Evidence |
|----------|--------|---------|
| `go.mod` (with google.golang.org/adk) | PARTIAL | Module `github.com/cascade-cli/cascade` present; `google.golang.org/genai v1.50.0` present; **`google.golang.org/adk` absent** (see gaps) |
| `pkg/types/message.go` | VERIFIED | `type Message struct`, `type ToolCall struct`, `func UserMessage(` — all present; tests pass |
| `pkg/types/event.go` | VERIFIED | `type Event interface`, `TokenEvent`, `ToolStartEvent`, `ToolEndEvent`, `PermissionRequestEvent`, `ErrorEvent`, `DoneEvent` — all present |
| `internal/config/config.go` | VERIFIED | `type Config struct`, `func DefaultConfig()` present with gemini-2.5-pro default |
| `internal/config/loader.go` | VERIFIED | `func Load(`, `toml.DecodeFile` at line 60, `CASCADE_MODEL` at line 66 |
| `internal/auth/gcp.go` | VERIFIED | `func NewTokenSource(`, `google.FindDefaultCredentials`, `impersonate.CredentialsTokenSource`, `oauth2.ReuseTokenSource`, error string with "gcloud auth application-default login" |
| `internal/auth/retry.go` | VERIFIED | `func RetryOn401(`, `type RetryTransport struct`, checks HTTP 401 |
| `internal/provider/provider.go` | VERIFIED | `type Provider interface`, `GenerateStream`, `type Stream struct`, `type Declaration struct`, `func NewStream(` |
| `internal/provider/gemini/gemini.go` | VERIFIED | `type GeminiProvider struct`, `func New(`, `func GenerateStream(`, `func convertToGenAI(`, `func convertFromGenAI(` — 244 lines, substantive |
| `internal/testutil/provider.go` | VERIFIED | `var _ provider.Provider = (*MockProvider)(nil)` compile-time check present |

#### Plan 01-02: Tools and Permission Engine

| Artifact | Status | Evidence |
|----------|--------|---------|
| `internal/tools/tool.go` | VERIFIED | `type Tool interface` with `Name()`, `RiskLevel() permission.RiskLevel`, `Execute(ctx context.Context, input json.RawMessage)` |
| `internal/tools/registry.go` | VERIFIED | `type Registry struct`, `func (r *Registry) Get(`, `func (r *Registry) Declarations()` |
| `internal/tools/result.go` | VERIFIED | `type Result struct` |
| `internal/tools/core/read.go` | VERIFIED | `type ReadTool struct`, Execute method present |
| `internal/tools/core/write.go` | VERIFIED | `type WriteTool struct` |
| `internal/tools/core/edit.go` | VERIFIED | `type EditTool struct` |
| `internal/tools/core/glob.go` | VERIFIED | `type GlobTool struct` |
| `internal/tools/core/grep.go` | VERIFIED | `type GrepTool struct` |
| `internal/tools/core/bash.go` | VERIFIED | `type BashTool struct`, `func ClassifyBashRisk(`, `readOnlyCommands` map |
| `internal/permission/risk.go` | VERIFIED | `type RiskLevel int`, `RiskReadOnly`, `RiskDestructive`, `func RequiresConfirmation(` |
| `internal/permission/mode.go` | VERIFIED | `type Mode int`, `ModeConfirm`, `ModePlan`, `ModeBypass`, `func CycleMode(` |
| `internal/permission/engine.go` | VERIFIED | `type Engine struct`, `type ToolRiskProvider interface`, `func (e *Engine) Check(`, `func (e *Engine) CycleMode()`, `func (e *Engine) CacheDecision(` — no circular import with tools |

#### Plan 01-03: Agent Loop

| Artifact | Status | Evidence |
|----------|--------|---------|
| `internal/agent/loop.go` | VERIFIED | `type Agent struct`, `func New(cfg AgentConfig)`, `func (a *Agent) RunTurn(`, `func (a *Agent) executeWithPermission(` — 214 lines |
| `internal/agent/governor.go` | VERIFIED | `type Governor struct`, `func (g *Governor) IsDuplicate(`, `func (g *Governor) CheckLimit(`, `func (g *Governor) ShouldNudge(` |
| `internal/agent/events.go` | VERIFIED | `type EventHandler interface`, `func (a *Agent) emit(` |
| `internal/agent/session.go` | VERIFIED | `type Session struct`, `func (s *Session) Append(`, `func (s *Session) Messages()` |
| `internal/app/app.go` | VERIFIED | `type App struct`, `func New(ctx context.Context, cfg *config.Config)` — 80 lines |

#### Plan 01-04: TUI and CLI

| Artifact | Status | Evidence |
|----------|--------|---------|
| `internal/tui/model.go` | VERIFIED | `type Model struct`, `Init() tea.Cmd`, `Update(msg tea.Msg)`, `View() string`, `tickMsg`, `agentEventMsg` — 356 lines |
| `internal/tui/renderer.go` | VERIFIED | `type StreamRenderer struct`, `func (r *StreamRenderer) Push(`, `func (r *StreamRenderer) DrainAll()`, `make(chan string, 256)` |
| `internal/tui/keys.go` | VERIFIED | All 6 required shortcuts registered: ctrl+c, ctrl+d, ctrl+l, shift+tab, ctrl+b, ctrl+r |
| `internal/tui/styles.go` | VERIFIED | Imports `charm.land/lipgloss/v2`, defines color palette and role styles |
| `internal/tui/chat.go` | VERIFIED | Imports `charm.land/bubbles/v2/viewport`, `charm.land/glamour/v2`; `renderMarkdown()` called on assistant messages |
| `internal/tui/confirm.go` | VERIFIED | `type ConfirmModel struct`, `sendResponse()` sends on response channel (line 77: `c.response <- approved`) |
| `internal/tui/spinner.go` | VERIFIED | Imports `charm.land/bubbles/v2/spinner` |
| `internal/oneshot/runner.go` | VERIFIED | `func Run(`, handles `*types.TokenEvent`, `*types.DoneEvent`, `*types.PermissionRequestEvent`, `fmt.Fprint(stdout` |
| `cmd/cascade/main.go` | VERIFIED | `cobra.Command`, `"cascade"`, `"csc"` alias, `"prompt", "p"` flag, `tea.NewProgram`, `oneshot.Run`, `os.ModeCharDevice` |

### Key Link Verification

| From | To | Via | Status | Evidence |
|------|-----|-----|--------|---------|
| `gemini/gemini.go` | `provider/provider.go` | implements Provider interface | VERIFIED | `func (g *GeminiProvider) GenerateStream(` signature matches interface |
| `auth/gcp.go` | `golang.org/x/oauth2/google` | ADC credentials | VERIFIED | `google.FindDefaultCredentials` at line 47 |
| `config/loader.go` | `github.com/BurntSushi/toml` | TOML parsing | VERIFIED | `toml.DecodeFile` at line 60 |
| `agent/loop.go` | `provider/provider.go` | LLM streaming call | VERIFIED | `a.provider.GenerateStream` at line 74 |
| `agent/loop.go` | `tools/registry.go` | tool lookup | VERIFIED | `a.registry.Get(` at line 128 |
| `agent/loop.go` | `permission/engine.go` | permission gating | VERIFIED | `a.permissions.Check(` at line 150 |
| `agent/loop.go` | `agent/governor.go` | limit + duplicate check | VERIFIED | `a.governor.IsDuplicate(` line 108, `a.governor.CheckLimit(` line 64 |
| `agent/loop.go` | `pkg/types/event.go` | event emission | VERIFIED | `a.emit(&types.TokenEvent{`, `a.emit(&types.ToolStartEvent{`, `a.emit(&types.DoneEvent{` |
| `tui/model.go` | `agent/events.go` | consumes event channel | VERIFIED | `types.TokenEvent`, `types.ToolStartEvent`, `types.DoneEvent`, `types.PermissionRequestEvent` handled in `processAgentEvent()` |
| `tui/renderer.go` | `tui/model.go` | tick-based drain | VERIFIED | `tickMsg` handler calls `m.renderer.DrainAll()` at line 91 |
| `tui/confirm.go` | `pkg/types/event.go` | sends on Response channel | VERIFIED | `sendResponse()` sends on `c.response` channel (the `PermissionRequestEvent.Response chan<- bool`) |
| `oneshot/runner.go` | `agent/events.go` | consumes event stream | VERIFIED | reads from `application.Events` channel, handles `*types.TokenEvent` and prints via `fmt.Fprint` |
| `cmd/cascade/main.go` | `internal/app/app.go` | creates App, launches TUI/oneshot | VERIFIED | `app.New(ctx, cfg)` + `tui.NewModel(application)` + `tea.NewProgram(model)` |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| AGNT-01 | 01-03 | Multi-turn streaming conversation | VERIFIED | Agent session accumulates `[]types.Message`; `RunTurn` loops until no tool calls; `TokenEvent` stream reaches TUI |
| AGNT-02 | 01-03 | Observe-reason-act loop with 15-20 tool call limit | VERIFIED | `governor.CheckLimit(toolCallCount)` enforced; default max=15 in config; duplicate detection via `IsDuplicate()` |
| AGNT-03 | 01-04 | Bubble Tea TUI with Lip Gloss, Glamour, syntax highlighting | VERIFIED | All three libraries imported and used: lipgloss in styles.go, glamour in chat.go, bubbletea/bubbles throughout tui/ |
| AGNT-04 | 01-01 | Google ADK Go with Gemini 2.5 Pro, Provider interface | PARTIAL | Provider interface present and fully abstracted; Gemini 2.5 Pro is default; **google.golang.org/adk NOT in go.mod** (uses genai directly per approved fallback) |
| AGNT-05 | 01-04 | Interactive mode and one-shot `-p` mode | VERIFIED | `-p` flag in Cobra; `oneshot.Run()` wired; `tui.NewModel()` for interactive; binary confirmed working |
| AGNT-07 | 01-02 | 6 core file tools: Read, Write, Edit, Glob, Grep, Bash | VERIFIED | All 6 tools in `internal/tools/core/`; all registered via `RegisterAll()`; tests pass |
| AUTH-01 | 01-01 | GCP ADC authentication | VERIFIED | `google.FindDefaultCredentials` with "gcloud auth application-default login" error message |
| AUTH-02 | 01-01 | Service account impersonation | VERIFIED | `impersonate.CredentialsTokenSource` called when `cfg.ImpersonateServiceAccount != ""` |
| AUTH-03 | 01-02 | Risk-level classification per tool call | VERIFIED | Every tool implements `RiskLevel() permission.RiskLevel`; `ClassifyBashRisk()` for dynamic bash classification |
| AUTH-04 | 01-02 | READ_ONLY auto-approves; DML+ requires confirmation | VERIFIED | `Engine.Check()`: `risk <= RiskReadOnly` returns Allow; above returns Confirm; `ConfirmModel` handles prompt |
| AUTH-05 | 01-02 | CONFIRM/PLAN/BYPASS mode cycling | VERIFIED | `CycleMode()` cycles CONFIRM→PLAN→BYPASS→CONFIRM; Shift+Tab triggers `m.app.Permissions.CycleMode()` in TUI |
| AUTH-07 | 01-01 | Retry-on-401 transparent token refresh | VERIFIED | `RetryOn401()` function and `RetryTransport` struct in `internal/auth/retry.go`; checks HTTP 401 |
| UX-01 | 01-01 | Config via `~/.cascade/config.toml` | VERIFIED | `loader.go` reads from `~/.cascade/config.toml`; TOML decoded via BurntSushi; 4-layer merge confirmed |
| UX-04 | 01-04 | Keyboard shortcuts: Ctrl+C, Ctrl+B, Shift+Tab, Ctrl+R, Ctrl+L, Ctrl+D | VERIFIED | All 6 shortcuts registered in `keys.go`; Ctrl+B and Ctrl+R are stubs (per plan: "register binding but show not implemented") — this is intentional for Phase 1 |

### Anti-Patterns Found

| File | Lines | Pattern | Severity | Impact |
|------|-------|---------|----------|--------|
| `internal/tui/keys.go` | 63, 67 | `Help: "background (not implemented)"`, `Help: "refresh cache (not implemented)"` | INFO | Intentional per plan 01-04 Task 1 note: "Ctrl+B and Ctrl+R are stubs for Phase 1" — AGNT-06 (background) and UX-05 (cache refresh) are deferred to later phases |
| `internal/tui/model.go` | 209, 213 | `SetMessage("Background mode not implemented yet")`, `SetMessage("Cache refresh not implemented yet")` | INFO | Same as above — intentional Phase 1 stubs |

No blocker anti-patterns found. No unintended `return null`/`return {}` stubs. No placeholder components.

### Human Verification Required

#### 1. Interactive TUI End-to-End

**Test:** Run `./bin/cascade` (requires GCP ADC configured or GOOGLE_API_KEY env var)
**Expected:** TUI appears with three-panel layout. Type a question and see streaming text. Tool calls show spinner. DML operations show inline `[DML] Execute? [y/N]` prompt. Shift+Tab cycles badge CONFIRM→PLAN→BYPASS. Ctrl+D exits cleanly.
**Why human:** Terminal UI rendering, streaming visual quality, and keyboard event handling cannot be verified by grep or test runner.

#### 2. One-Shot Mode Output

**Test:** Run `./bin/cascade -p "what is 2+2?"` (requires API credentials)
**Expected:** Streaming text printed to stdout, clean newline, process exits with code 0.
**Why human:** Live LLM call + stdout streaming behavior requires runtime observation.

#### 3. GCP ADC Token Refresh

**Test:** Run `gcloud auth application-default login`, then `./bin/cascade` for a session spanning the 1-hour ADC refresh boundary.
**Expected:** Session continues without auth error after token refresh; user sees no interruption.
**Why human:** Requires real GCP credentials and waiting for token expiry — cannot test programmatically.

### Gaps Summary

**One gap identified:** The plan 01-01 must_haves artifact for `go.mod` specifies `contains: "google.golang.org/adk"`, but `google.golang.org/adk` is absent from `go.mod` and `go.sum`. The implementation uses `google.golang.org/genai v1.50.0` directly.

**Context:** The plan explicitly permitted this: *"If ADK Go v0.6.0's gemini package constructor signature differs, fall back to using google.golang.org/genai client directly."* The 01-01-SUMMARY.md documents the decision: *"Used GenAI SDK directly instead of ADK Go model.LLM for Gemini provider — cleaner streaming API via iter.Seq2."* AGNT-04's functional requirement (Provider interface + Gemini 2.5 Pro default) is fully satisfied.

**Resolution options:**
1. Update AGNT-04 and plan 01-01 must_haves to reflect the approved implementation (genai SDK, not ADK Go)
2. Add `google.golang.org/adk` as a dependency even if the gemini implementation doesn't use it — this satisfies the literal artifact check but adds dead weight

Option 1 is recommended: update the requirement traceability to reflect the approved technical decision.

---

_Verified: 2026-03-18_
_Verifier: Claude (gsd-verifier)_
