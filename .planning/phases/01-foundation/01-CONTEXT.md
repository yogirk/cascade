# Phase 1: Foundation - Context

**Gathered:** 2026-03-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Deliver a working conversational agent in the terminal that authenticates to GCP, executes file tools, and enforces risk-based permissions. User can launch `cascade`, have a multi-turn streaming conversation with an AI, execute core file operations (read, write, edit, glob, grep, bash), authenticate via ADC, and see permission enforcement based on risk classification. Interactive mode is default; one-shot mode via `-p` flag is also available.

This phase does NOT include BigQuery tools, schema cache, platform tools, or any GCP data service integration — those are Phase 2+.

</domain>

<decisions>
## Implementation Decisions

### Streaming Output
- Character-by-character token rendering with markdown post-processing — matches Claude Code's proven UX pattern for AI agents
- Spinner with tool name during tool execution (e.g., "Querying BigQuery..." or "Reading file...")
- Tool results displayed inline with syntax highlighting — SQL as tables, code with highlighting, logs with color coding
- Long output handled via viewport with scroll — truncate at reasonable length, scrollable for full content
- Ring buffer for LLM token delivery, batch renders at 30fps tick — prevents Bubble Tea streaming deadlock (critical pitfall from research)

### Permission UX
- Inline prompt with risk level badge: [READ] auto-approves silently, [DML] shows warning + y/N prompt, [DESTRUCTIVE] shows red warning requiring explicit 'yes'
- Shift+Tab cycles permission modes (CONFIRM → PLAN → BYPASS) with persistent mode badge in TUI header
- Cost estimation shown inline with permission confirmation when applicable (e.g., "This query will scan ~4.2 GB (~$0.02). Execute? [y/N]")
- Permission caching per-session for same tool+args patterns to reduce prompt fatigue

### CLI Invocation
- Primary command: `cascade`, alias: `csc` (from VISION.md)
- Default interactive mode (Bubble Tea TUI), `-p` flag for one-shot mode (matches Claude Code pattern)
- Support stdin piping as context (e.g., `cat file.sql | cascade "optimize this"`)
- Config loading order: defaults → `~/.cascade/config.toml` → `CASCADE.md` → env vars → CLI flags

### Error Recovery
- LLM API failure: retry once with exponential backoff, then show error with retry prompt — never crash silently
- GCP auth failure: clear error with fix instructions ("GCP auth failed. Run: gcloud auth application-default login")
- Tool execution failure: feed error back to LLM for self-correction, max 2 retries per tool call
- Unrecoverable errors: clean exit with session state preservation, print error, suggest recovery steps
- Auth token expiry: use `google.DefaultTokenSource()` exclusively, retry-on-401 middleware — handle transparently during long sessions

### Agent Loop Design
- Single-threaded observe-reason-act cycle with hard limit of 15-20 tool calls per turn (configurable)
- Loop governor: detect same tool called twice with same args, inject progress nudge after 5 tool calls with no user-facing output
- ADK Go used as LLM client (via `model.LLM` interface), not as agent loop orchestrator — custom loop wraps ADK
- Provider interface from day one: ADK Go abstracted behind `Provider` so it remains replaceable
- Gemini 2.5 Pro as default, model configurable via config.toml

### Project Structure
- Go module: standard Go project layout with `cmd/cascade/`, `internal/` packages
- Key packages: `internal/agent/` (loop), `internal/tui/` (Bubble Tea), `internal/provider/` (LLM abstraction), `internal/tools/` (tool registry), `internal/permission/` (risk engine), `internal/config/` (config loading)
- TUI fully decoupled from agent logic via typed event channels — agent loop works identically in interactive and one-shot modes
- Tools are interface-based, self-describing (name, description, JSON schema, risk level, execute method)

### Claude's Discretion
- Exact Bubble Tea component structure and message types
- Specific Lip Gloss styling choices (colors, borders, spacing)
- Config TOML section naming and key structure
- Internal error types and error wrapping patterns
- Test structure and test helper patterns

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Architecture & Agent Loop
- `specs/original/ARCHITECTURE.md` — Agent loop design, LLM backend, platform context engine, technology stack decisions
- `specs/original/SPEC-REVIEW.md` — Pre-build decisions: read-first strategy, deferred sandboxing, simplified subagents, ADK Go commitment

### Tools & Permissions
- `specs/original/TOOLS.md` — Complete tool system: core tools, risk classification, parallel execution, timeout policies
- `specs/original/SECURITY.md` — Four-layer security model (Layer 2: Permission Engine is primary for Phase 1)

### UX & Terminal
- `specs/original/UX.md` — Terminal UI elements, keyboard shortcuts, permission modes, interactive vs one-shot mode, configuration
- `specs/original/VISION.md` — Design principles, command naming (`cascade` / `csc`), golden standard test

### Research
- `.planning/research/ARCHITECTURE.md` — Component boundaries, data flow, build order patterns from OpenCode/Claude Code
- `.planning/research/STACK.md` — Verified library versions: ADK Go v0.6.0, Charm v2 stack, Go 1.26+
- `.planning/research/PITFALLS.md` — Critical pitfalls: streaming deadlock, agent loop cycling, permission fatigue, auth token expiry

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None — greenfield project, no existing code

### Established Patterns
- None yet — Phase 1 establishes all foundational patterns

### Integration Points
- Go module initialization (`go mod init`)
- GCP Application Default Credentials (`golang.org/x/oauth2/google`)
- Google ADK Go v0.6.0 (`google.golang.org/adk`)
- Charm v2 stack: Bubble Tea v2.0.2, Lip Gloss v2.0.2, Glamour v2.0.0, Bubbles v2.0.0, Huh v2.0.3

</code_context>

<specifics>
## Specific Ideas

- Agent loop should feel like Claude Code — streaming output, tool calls visible inline, clean markdown rendering
- Permission prompts should be unobtrusive for reads but unmissable for writes — similar to Claude Code's ask/allow model
- One-shot mode (`-p`) must work cleanly in CI pipelines: proper exit codes, no interactive prompts, structured output option
- The "cascade" name and "csc" alias are from the spec and are non-negotiable

</specifics>

<deferred>
## Deferred Ideas

- BigQuery query tool and schema cache — Phase 2
- Platform tools (Composer, Logging, GCS) — Phase 3
- dbt integration — Phase 4
- Skills/hooks/subagents — Phase 5
- Setup wizard and Homebrew distribution — Phase 6

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-03-16*
