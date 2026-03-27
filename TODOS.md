# TODOS

## Config Surface Audit

Audit all config fields to ensure every declared field is consumed by the code.
Known gaps: `cost.auth` (unused), `agent.tool_timeout` (never reaches execution),
custom `api_key_env` for OpenAI/Anthropic (may be validated but ignored).

**Why:** Config lies erode trust. Users set fields expecting behavior that doesn't happen.
**Depends on:** Nothing. Can be done anytime.
**Source:** Codex outside voice, eng review 2026-03-24.

## app/ Package Refactor (Pattern B)

When app/ exceeds 7 files or 1800 LOC, extract init logic into service packages.
E.g., `bigquery.NewClient(cfg, resource)`, `platform.NewClients(cfg, resource)`.
Keep `app.New()` as thin orchestrator that calls constructors and wires the graph.

**Why:** Both Claude and Codex independently validated Pattern B (push init into service
packages) over Pattern A (sub-packages). Init knowledge belongs near the code it configures.
**Depends on:** Next integration landing (Composer or dbt).
**Smell test:** "How does BigQuery get built?" -> belongs in bigquery/.
"When and why do we build BigQuery?" -> belongs in app/.
**Source:** Eng review 2026-03-24, architecture issue A1.

---

## Lessons from OpenCode/Crush Comparison (2026-03-24)

The following items emerged from a deep architectural comparison of Cascade vs
OpenCode/Crush (Charm's 41K-star AI coding agent, same Go + Bubble Tea stack).
Run `/office-hours` on these to decide scope and approach.

### Session Persistence (SQLite)

Conversations are lost on exit. Crush stores sessions in SQLite (via sqlc + Goose
migrations) so users can resume where they left off. Cascade already has SQLite
expertise from the schema cache — the pattern is proven in this codebase.

**Why:** Power users have multi-hour investigation sessions. Losing context on
accidental exit or terminal crash is painful. Also enables: session history,
conversation search, cross-session learning.
**Effort:** S-M (human: ~3 days / CC: ~30min)
**Depends on:** Nothing. Independent of all other work.
**Source:** Crush comparison 2026-03-24.

### MCP Server Support

Crush supports MCP (Model Context Protocol) with 3 transports: stdio, http,
streamable HTTP. This lets users connect external tools (Composer, dbt, custom
internal tools) without Cascade building every integration.

**Why:** Instead of building a Composer tool, a dbt tool, a Dataflow tool — let
users bring their own via MCP. This is the extensibility moat. Also unlocks
community-contributed tools without modifying Cascade core.
**Effort:** M-L (human: ~2 weeks / CC: ~2-3 hours)
**Depends on:** Nothing. But dramatically changes the Phase 4+ roadmap — many
planned tools could become MCP servers instead of built-in tools.
**Source:** Crush comparison 2026-03-24.

### Multi-Model Slots (Large + Small)

Crush has two model slots: "large" (reasoning) and "small" (simple tasks),
switchable mid-session. For Cascade, this could mean: Gemini 2.5 Pro for complex
SQL generation and cross-service debugging, Flash for schema lookups, formatting,
and simple queries.

**Why:** Cost optimization + speed. Simple tasks don't need the most expensive model.
Schema lookups and formatting waste money on Pro when Flash handles them fine.
**Effort:** S (human: ~2 days / CC: ~20min)
**Depends on:** Nothing. Provider interface already supports model switching.
**Source:** Crush comparison 2026-03-24.

### TUI Architecture: Alt Screen Trade-off

Root cause of recurring scroll + text selection conflicts: alt screen (xterm mode)
disables terminal-native scroll/selection. No wheel-only mouse mode exists in the
xterm protocol. Claude Code avoids this by not using alt screen at all.

**Options explored:**
- A) Drop alt screen — terminal handles scroll + selection natively (Claude Code model)
- B) Keep alt screen + keyboard-only scroll (lazygit/k9s model)
- C) Drop Bubble Tea entirely — custom render loop + Lip Gloss (simplest)

**Current decision:** Keep alt screen + mouse mode (1 line/tick for smoother scroll).
Revisit when/if UX model shifts toward conversational (non-dashboard) paradigm.
**Full analysis:** `Projects/notes/Atomic Notes/TIL - Why Terminal TUI Apps Can't
Have Both Scroll and Text Selection.md`
**Source:** TUI investigation 2026-03-24, confirmed by Codex.

---

## Color-Blind Tool Bullet Differentiation

Add glyph shape differentiation to tool bullets alongside color coding.
Currently: all bullets use `~` with color only (green/amber/red/cyan/indigo).
Proposed: `○` read, `◇` write, `●` exec, `△` query, `□` data.

**Why:** Tool risk categories rely on color alone. Deuteranopia (red-green, ~8% of men)
makes green (read/safe) and red (exec/dangerous) indistinguishable. The two most
security-critical categories are the ones that collide.
**Effort:** XS (human: ~2 hours / CC: ~5 min)
**Depends on:** Nothing. Independent work. Shapes defined in DESIGN.md accessibility section.
**Source:** Design review 2026-03-27, Codex + Claude subagent outside voices.

---

## Charm Ecosystem Libraries to Explore

Evaluate these charmbracelet libraries for adoption into Cascade's TUI stack.
All three are MIT-licensed, actively maintained, and compatible with our Charm v2 ecosystem.

### huh — Terminal Forms (6.7k stars)

Replace hand-rolled interactive components (model picker, confirmation dialog) with
huh's form primitives. Also unlocks richer flows: config setup wizard, project init,
multi-step prompts. Embeds directly as a Bubble Tea component.

**Candidates for migration:** `models.go` (model picker → huh select),
`confirm.go` (permission prompt → huh confirm).
**New opportunities:** `/config` interactive editor, first-run setup wizard.
**Repo:** https://github.com/charmbracelet/huh

### harmonica — Physics-Based Animations (1.5k stars)

Replace manual sine-wave interpolation in spinner pulse effects with proper
spring/damping curves. Also enables smooth scroll easing and cursor animations.
Small library, no dependencies.

**Candidates:** `spinner.go` pulse animation, viewport scroll transitions.
**Repo:** https://github.com/charmbracelet/harmonica

### log — Structured Colorful Logging (3.2k stars)

Lip Gloss-styled structured logging for `--verbose`/`--debug` output. Drop-in
replacement for stdlib log with leveled output and key-value fields that match
Cascade's visual style.

**Candidates:** Debug/verbose mode output, agent loop tracing, tool execution logs.
**Repo:** https://github.com/charmbracelet/log

**Source:** Charm ecosystem evaluation 2026-03-25.
