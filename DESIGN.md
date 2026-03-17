# Design System — Cascade

## Product Context
- **What this is:** AI-native terminal agent for GCP data engineering
- **Who it's for:** Data engineers who live in the terminal — BigQuery, Airflow, dbt, Cloud Logging, GCS
- **Space/industry:** Data infrastructure tooling (peers: dbt CLI, bq CLI, gcloud, Claude Code, Codex CLI)
- **Project type:** Interactive TUI (Bubble Tea) — conversational agent with tool execution

## Aesthetic Direction
- **Direction:** Industrial/Utilitarian
- **Decoration level:** Minimal — typographic weight and semantic color do all the work
- **Mood:** A well-designed instrument panel. Everything visible has a purpose, nothing decorative. The only decorative element is the cascade bar logo (stepped colored bars evoking pipeline stages).
- **Reference tools:** Claude Code (interaction gold standard), Codex CLI (item lifecycle), Warp (block-based output), aider (auto-lint loops)

## Typography (Terminal Weights)

Terminal apps don't pick fonts — the user's terminal font applies. The typographic tools are: bold, dim, italic, normal, and color.

| Weight | Usage | Example |
|--------|-------|---------|
| **Bold** | Names, headers, key values — the thing you're scanning for | `bigquery_query`, column headers |
| Normal | Body text, assistant responses, tool output | Response paragraphs, file contents |
| Dim | Metadata, timestamps, token counts, secondary info | `7s · ↑15.4k ↓2.1k` |
| *Italic* | System messages, hints, ephemeral content | `Context compacted`, transient status |
| Color + Bold | Status badges, risk levels — demands attention | `ASK`, `[DESTRUCTIVE]`, `[DML]` |

**Rules:**
- Never stack bold + italic (muddy in most terminal fonts)
- Never use underline (conflicts with OSC 8 hyperlink rendering in modern terminals)
- Dim is the workhorse — most metadata should be dim, not normal

## Color

- **Approach:** Restrained — semantic color carries meaning, not decoration
- **Adaptive:** All colors have light/dark variants via `lipgloss.LightDark()`. Detection at startup via `HasDarkBackground()`, overridable with `SetTheme("light"|"dark"|"auto")`.

### Core Palette

| Role | Dark | Light | Usage |
|------|------|-------|-------|
| Accent | `#6B9FFF` | `#2563EB` | Assistant bullet, input border, headings, schema annotations |
| Settled Accent | `#4A6FA5` | `#4A6FA5` | Submitted user message border (muted echo of live input) |
| Bright | `#F3F4F6` | `#111827` | Headings, key values, bold text — highest contrast |
| Text | `#D1D5DB` | `#374151` | Body text, normal weight content |
| Dim | `#64748B` | `#6B7280` | Metadata, timestamps, secondary info | Dark raised from `#4B5563` (was 2.35:1, now ~3.7:1) |
| Elevated | `#3A3B3F` | `#ECEEF2` | Input box background, user message background |
| Surface | `#111827` | `#F3F4F6` | Status bar background, terminal chrome |
| Border | `#374151` | `#D1D5DB` | Input border (focused state) |
| Border Dim | `#1F2937` | `#E5E7EB` | Separator rules, input border (blurred state) |

### Semantic Colors

All semantic text colors must clear WCAG AA contrast (4.5:1) against their background surface.

| Role | Dark | Light | Usage | Notes |
|------|------|-------|-------|-------|
| Success | `#34D399` | `#047857` | Read-only tool bullets, cost safe, ASK mode badge | Light darkened from `#059669` (was 3.42:1, now ~5.0:1) |
| Warning | `#FBBF24` | `#92400E` | Write tool bullets, cost warning, DML badges, approval prompt | Light darkened from `#D97706` (was 2.89:1, now ~6.5:1) |
| Danger | `#F87171` | `#B91C1C` | Exec tool bullets, cost danger, destructive badges, errors | Light darkened from `#DC2626` (was 4.39:1, now ~5.4:1) |
| Tool | `#22D3EE` | `#0E7490` | Query tool bullets (bigquery_query) — separates cost-bearing queries from write warnings | Light darkened from `#0891B2` (was 3.35:1, now ~4.8:1) |
| Data | `#818CF8` | `#4F46E5` | Platform data tools (cloud_logging, gcs_browse), BQ data display, plan color | Already 5.71:1, passes |

### Cascade Ocean (Spinner Palette)

| Step | Dark | Light | Role |
|------|------|-------|------|
| Dim | `#1E3A5F` | `#93C5FD` | Tilde at rest |
| Trail | `#0369A1` | `#3B82F6` | Fade trail |
| Bright | `#38BDF8` | `#2563EB` | Active pulse |
| Peak | `#7DD3FC` | `#1D4ED8` | Maximum brightness |

### Sweep Palette (Spinner Text)

| Step | Dark | Light |
|------|------|-------|
| Dim | `#4B5563` | `#B0B8C4` |
| Mid | `#9CA3AF` | `#6B7280` |
| Bright | `#F3F4F6` | `#111827` |

### Fixed Colors (not theme-dependent)

- Google Blue: `#4285F4`, Red: `#EA4335`, Yellow: `#FBBC05`, Green: `#34A853` — logo only

### Diff Colors

| Type | Dark BG | Dark FG | Light BG | Light FG |
|------|---------|---------|----------|----------|
| Add | `#022c22` | `#86efac` | `#DCFCE7` | `#166534` |
| Remove | `#2a0a0a` | `#fca5a5` | `#FEE2E2` | `#991B1B` |

### Inline Code

- Dark: `#7EB8DA` on `#223366` background
- Light: `#1E40AF` on `#EBF0FF` background
- Rationale: cyan/blue tones, NOT red (red means errors in this UI)

### Color Rules
- Tool bullets encode risk category, NOT severity — they tell you what KIND of action, not how dangerous
- Warning color (amber) is reserved for actual warnings and write operations — never use for query tools
- Cost display escalates green → amber → red based on configurable thresholds
- Google brand colors are fixed and used only in the welcome banner logo
- All semantic text colors must clear 4.5:1 contrast against their background surface (WCAG AA)
- Dim color targets ~3.5:1 — readable when focused, unobtrusive when scanning

### Accessibility
- **Color-blind safety:** Tool bullet semantics rely on color alone. To support deuteranopia (red-green, ~8% of men), tool bullets should also differ by glyph shape as a future enhancement: `○` read, `◇` write, `●` exec, `△` query, `□` data. This is NOT implemented yet but shapes are chosen to be visually distinct at terminal font sizes.
- **Contrast:** All semantic colors are calibrated to clear WCAG AA (4.5:1) on their respective backgrounds. Dim is intentionally below AA (~3.7:1 dark, ~4.4:1 light) because it represents secondary metadata, not primary content.
- **Motion sensitivity:** The sweep animation and tilde pulse are subtle. No rapid flashing. Tick rate of 12fps is well below seizure thresholds.

## Tool Bullet Semantics

| Bullet Color | Category | Tools | Meaning |
|-------------|----------|-------|---------|
| Green (success) | Read | read_file, grep, glob, bigquery_schema | Safe. No side effects. |
| Amber (warning) | Write | write_file, edit_file | Modifies files. Reversible. |
| Red (danger) | Execute | bash | Shell execution. Potentially destructive. |
| Cyan (tool) | Query | bigquery_query | Data query. Cost-bearing. |
| Indigo (data) | Data | cloud_logging, gcs | Platform data access. |

## Spacing
- **Base unit:** Line-based (terminal constraint — no sub-line spacing)
- **Density:** Compact-comfortable

| Element | Spacing |
|---------|---------|
| Between messages | 1 blank line (`\n\n` after each message) |
| Turn separator | Dim `─` horizontal rule before each `role="user"` message (except the first). Transcript is message-based — separators mark turn boundaries, not every message. |
| Content margin | 2 spaces left gutter (all content except status bar) |
| Tool output indent | 4 spaces (clear hierarchy: bullet+name at margin, body indented) |
| Diff indent | 4 spaces (aligned with tool output) |
| Input box padding | 1 line top, 1 line bottom, 2 spaces left/right inner |
| Status bar | Edge-to-edge (no margin — the only element that bleeds to edges) |
| Welcome banner | Vertically centered at ~1/3 from top of available space |

## Welcome Banner

Two-panel horizontal layout, vertically positioned at ~1/3 from top of available space.

**Left panel (20% width):** Cascade bar logo — three colored bars stepping right by 1 cell each, using `cascadeBg1/2/3` background colors. Pipeline-stages motif.

**Right panel (80% width):**
- `Project   <project-id>` — bright value, dim label (if GCP configured)
- `Datasets  <list>` — bright value, dim label (if BQ datasets configured)
- `Mode      [ASK]` — live mode badge
- Blank line
- `Type a message to get started` — dim hint
- `/help  all commands` — dim, 10-char label column

**Title line:** `── Cascade vX.X.X ──────` in `inputBorderColor`, title embedded mid-rule.

**Degraded states:**
- No GCP project: omit Project/Datasets rows. Show hint: `Run cascade --project <id> to connect` in dim.
- No auth: show `⚠ Not authenticated` in warning color above the hint.

**Note:** Full onboarding wizard deferred to end of development. Current welcome banner serves as placeholder first-run experience.

**On first submit:** Welcome banner is snapshotted into chat history as `role="welcome"` (first message), persists in scroll-back.

## Turn Anatomy

Each conversation turn composes these elements in order:

```
  ── turn separator ──────────────          ← dim ─ rule (border-dim color)

  ┃ User message text               ← elevated bg, settled-accent left border

  ~ tool_name  arg=value             ← color-coded bullet + bold name + dim args
      output line 1                  ← 4-space indent, dim
      output line 2
      ··· N lines omitted ···       ← dim, italic (if truncated)
      output last 3 lines
      [Space to expand]              ← accent color, dim (if truncated)

  ≋ Assistant response text          ← accent bullet + normal text
    **Bold** for emphasis, `code` for inline code

    Code blocks with syntax highlighting
    Diff blocks with add/remove coloring

    7s · ↑15.4k ↓2.1k               ← turn summary, dim italic, 2-space indent
```

**Reading order per turn:** separator → user message → tool calls (in execution order) → assistant response → turn summary.

## Layout

- **Approach:** Zone-based vertical stack

```
┌──────────────────────────────────────────┐
│  CHAT VIEWPORT  (dynamic height)         │
│  SPINNER LINE   (1 line, conditional)    │
│  CONFIRM PROMPT (N lines, conditional)   │
│  MODEL PICKER   (N lines, conditional)   │
│  INPUT BOX      (dynamic height)         │
│  STATUS BAR     (1 line, full width)     │
└──────────────────────────────────────────┘
```

- Chat viewport is the remaining height after subtracting all other zones
- Input box auto-expands with content; viewport shrinks to compensate
- Only one conditional zone (spinner/confirm/picker) is visible at a time
- Status bar is always visible, always full-width

### Terminal Width Breakpoints

Status bar elements collapse progressively as terminal narrows. Elements are removed right-to-left (least critical first).

| Width | Visible Elements | Dropped |
|-------|-----------------|---------|
| **≥100 cols** | model, mode, cost, middle slot, context bar, cwd, branch | (full layout) |
| **80-99 cols** | model, mode, cost, middle slot, context bar, cwd | branch |
| **60-79 cols** | model, mode, cost, middle slot | context bar, cwd, branch |
| **40-59 cols** | mode, cost | model, middle slot, context bar, cwd, branch |
| **<40 cols** | mode only | everything else (cost shown in chat via `/cost`) |

**Truncation rules:**
- Model name: truncate to first word at <70 cols (e.g., "Gemini" instead of "Gemini 2.5 Pro")
- cwd: truncate from left with `…/` prefix, max 30 chars
- Branch: truncate at 15 chars with `…` suffix
- Cost: abbreviate to `$0.04` (drop "BQ"/"LLM" labels) at <70 cols
- Middle slot tool name: truncate at 20 chars with `…`

**Input box max height:** `min(40% of terminal height, 10 lines)`. Prevents paste-bombing the viewport. Content beyond max height scrolls within the input box.

### Status Bar Layout

```
[model]  [mode badge]  [BQ $X.XX · LLM $X.XX]  [middle slot]  ···  [context bar XX%]  [cwd]  [branch]
```

- **Model name:** friendly-formatted, bold
- **Mode badge:** colored (ASK=green, READ_ONLY=indigo, FULL_ACCESS=red)
- **Cost (Risk #1):** BQ dollar cost + LLM token counts, always visible. Format: `BQ $0.01 · ↑15k ↓2k`. BQ cost color-coded: green < $1, amber < $5, red > $5 (configurable). LLM shows token counts only (not fabricated dollar estimates — providers have different pricing, trust requires accuracy)
- **Middle slot** (priority): `● awaiting approval` > `~ tool_name` > transient message > (empty)
- **Context bar:** 5-cell block fill, green→amber→red as context fills
- **Right side:** shortened cwd (≤30 chars, ≥80 cols), git branch (≥60 cols)

## Motion
- **Approach:** Intentional — every animation communicates state

### Spinner
- **Tilde character:** `≋` (thinking) / `~` (tool mode), pulses through 4-step ocean-blue palette via sine wave, 1.5s cycle
- **Text sweep:** Bright spotlight moves left-to-right across message characters, fading with distance, 1.5s per sweep
- **Thinking messages:** Rotate every 2s through curated set — personality is intentional
- **Tool messages:** Static, tool-specific: "Executing command...", "Reading file...", "Executing query...", etc.
- **Timer:** Appears after 1s, shows `Xs` or `Xm Ys`
- **Token counts:** `↑Nk ↓Nk` appear as soon as first usage arrives, dim
- **Tick rate:** 12fps for spinner animation, 30fps for streaming content drain

### Streaming
- Tokens accumulated thread-safe, drained in batch at 30fps
- Auto-scroll follows new content (tail mode)
- Manual scroll up disables follow; scroll to bottom re-enables
- New turn always resumes follow

## Interaction Patterns

### Input
- **Submit:** Enter
- **Newline:** Shift+Enter
- **History:** Up/Down when single-line and idle (session-only, 50 entries, deduplicated)
- **Auto-expand:** Box grows with content, viewport shrinks. Height synced every keystroke.
- **Focus:** Blurred during streaming/tool/confirm. Re-focused on turn end.
- **Placeholder:** "Ask anything..." in dim
- **Cursor:** Non-blinking (explicitly disabled)

### Approval Flow
- 3 options: Allow once, Allow tool for session, Deny
- Navigation: j/k, Up/Down, Tab/Shift+Tab, or direct keys 1/2/3, y/a/n
- Default cursor: option 1 (Allow once)
- Status bar shows `● awaiting approval` in amber
- Rendered inside `ConfirmBoxStyle` (warning-colored left border)
- Tool args displayed context-aware: bash shows `$ command`, write shows `file: path`, BQ shows SQL preview

### Scrolling
- Mouse wheel: 1 line per tick
- PgUp/PgDown: half-page
- Up/Down: 1 line (when not in input history mode)
- Auto-follow on new content; manual scroll disables follow; scroll-to-bottom re-enables
- New turn always resumes follow

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Enter | Submit message |
| Shift+Enter | Insert newline |
| Space | Expand/collapse truncated tool output |
| Up/Down | History (idle) or scroll |
| PgUp/PgDown | Half-page scroll |
| Ctrl+C | Cancel operation / quit |
| Ctrl+D | Exit |
| Ctrl+Y | Copy last assistant response |
| Shift+Tab | Cycle permission mode |

### Input Routing Precedence

Keys are routed to the highest-priority active handler. Only one handler processes each key.

| Priority | Mode | Owns | Notes |
|----------|------|------|-------|
| 1 | Confirm (approval) | j/k, Up/Down, Tab/Shift+Tab, 1/2/3, y/a/n, Enter, Esc | All keys routed here when approval is active |
| 2 | Model picker | j/k, Up/Down, Enter, Esc, number keys | Active during `/model` |
| 3 | Input focused (idle) | Enter (submit), Shift+Enter, Up/Down (history), all text input | Default when idle |
| 4 | Viewport (scroll) | Up/Down, PgUp/PgDown, Space (expand), mouse wheel | Active when input not focused OR multiline |
| 5 | Global | Ctrl+C, Ctrl+D, Ctrl+Y, Shift+Tab (permission cycle) | Always available |

**Key collision resolution:**
- Space: only triggers expand/collapse in viewport mode (priority 4). When input is focused, Space types a space character.
- Up/Down: history navigation in input mode (priority 3), scrolling in viewport mode (priority 4).
- Tab/Shift+Tab: approval navigation in confirm mode (priority 1), permission cycle as global (priority 5). Never collides because confirm takes priority.
- Enter: confirm selection in modes 1/2, submit in mode 3. Never ambiguous.

### Slash Commands

| Command | Action |
|---------|--------|
| `/help` | List commands and shortcuts |
| `/clear` | Clear all messages |
| `/copy` | Copy last assistant message |
| `/copy-code` | Copy last code block |
| `/model` | Interactive model picker |
| `/compact` | Trigger context compaction |
| `/cost` | BigQuery session cost breakdown |
| `/morning` | Platform health briefing |
| `/insights` | BigQuery cost dashboard |
| `/logs` | Cloud Logging queries |
| `/sync` | Refresh schema cache |

## Application States

Every state the app can be in, with render contracts for each.

| State | Viewport | Spinner | Input | Status Bar Middle | Exit Transition |
|-------|----------|---------|-------|-------------------|-----------------|
| **First-run** | Welcome banner (degraded if no GCP) | — | Focused, placeholder | (empty) | User submits first message |
| **Idle** | Chat history | — | Focused, placeholder or draft | Cost display (if > 0) | User submits message or slash command |
| **Streaming** | Chat + live streaming tokens | `≋ Thinking... Xs` | Blurred | (empty or cost) | Stream completes → tool or idle |
| **Tool running** | Chat history | `~ Reading file... Xs` | Blurred | `~ tool_name` | Tool completes → streaming or idle |
| **Awaiting approval** | Chat history | — | Blurred | `● awaiting approval` (amber) | User selects option → tool runs or denied |
| **Model picking** | Chat history | — | Blurred | (empty) | User selects model or presses Esc |
| **Tool error** | Chat + error message (red `!` prefix, 4-space indent) | — | Re-focused | Cost display | Automatic — error rendered, return to streaming/idle |
| **Auth error** | Chat + system message: `⚠ Authentication failed: <detail>` in warning | — | Focused | `⚠ auth error` (amber, 3s) | User re-authenticates externally, retries |
| **Network error** | Chat + error message: `! Connection failed: <detail>` in danger | — | Focused | `! error` (red, 3s) | User retries manually |
| **Compacting** | Chat history (unchanged) | `≋ Compacting context...` | Blurred | (empty) | Compaction completes → system message + idle |
| **Cost warning** | Chat + system message: `⚠ Budget alert: session cost $X.XX (N% of daily budget)` in warning | — | Focused (not blocked) | Cost in amber/red | Informational only — does not block |
| **Empty conversation** | Welcome banner | — | Focused, placeholder | (empty) | User submits first message |

**Transition rules:**
- Error states (tool-error, auth-error, network-error) always render the error into chat and return to idle. Errors never block the UI.
- Compacting is non-interactive — the user cannot type until compaction completes.
- Cost warning is informational — the user can keep working. The warning persists in chat history.
- Streaming → tool-running → streaming is the normal agent loop. The spinner changes between `≋` and `~` on each transition.

**Slash command feedback:**
- `/clear`: Clears viewport, resets to empty-conversation state. Cost does NOT reset (session-scoped).
- `/compact`: Transient status bar message "Context compacted" (3s) + system message in chat.
- `/sync`: Transient status bar message "Schema refreshed" (3s) on success, error message in chat on failure.
- `/copy`, `/copy-code`: Transient status bar message "Copied to clipboard" (3s). No chat message.
- `/model`: Enters model-picking state. Selection or Esc returns to idle.

## Design Risks (Differentiators)

### Risk 1: Cost as First-Class Citizen
Split BQ and LLM costs in the status bar, always visible, color-coded by threshold. Flash pre-scan estimates for queries >1TB before the spinner starts. No other terminal AI tool surfaces cost this prominently. Data engineers care about money — this builds trust.

### Risk 2: Progressive Tool Output
Truncated tool output (10 head + 3 tail lines) shows a `[Space to expand]` hint in dim accent. Space toggles between truncated and full output when the truncated message is visible. Implementation: `expandedSet map[int]bool` on chat model, toggled by Space key in viewport mode. Expand triggers a forced transcript rebuild (set a `forceRebuild` flag, not a fake resize — `SetSize` no-ops on unchanged dimensions). Expanded state persists until toggled again. On expand, viewport adjusts to keep content visible. **Note:** chat is append-only rendered strings, not addressable blocks. The expand target is the most recently visible truncated tool message. Removes the friction of re-asking. Saves turns.

### Risk 3: Contextual Spinner (Tool Mode)
Tool-mode spinner shows what's actually happening: "Reading file...", "Executing query...", "Searching code...". Thinking mode keeps rotating personality messages. The distinction: tools are transparent (show the verb), reasoning is opaque (show personality).

## Anti-Patterns

- Never use red for inline code (red = errors in this UI)
- Never use amber for query tools (amber = write warnings; queries get cyan)
- Never stack bold + italic
- Never use underline (conflicts with OSC 8 hyperlinks)
- Never show the model name in the spinner (it's already in the status bar)
- Never use color purely for decoration — every color carries semantic meaning
- No purple/violet gradients, no decorative borders, no rounded-corner aesthetic
- Turn summaries are dim italic, never competing with message content

## Implementation Notes (from eng review)

These notes capture implementation-level details discovered during the eng review that aren't design decisions but affect how the design is built.

- **Progressive output target:** Chat is append-only rendered strings. Space expand targets the most recently visible truncated tool message. Use `forceRebuild` flag to trigger transcript rebuild (not fake resize).
- **Turn separator rule:** Transcript is message-based, not turn-based. Separator renders before `role="user"` messages with `index > 0`. Not between every message.
- **LLM cost display:** Only token counts available from providers, not dollar costs. Do not fabricate dollar estimates. Show `↑Nk ↓Nk` for LLM, `$X.XX` for BQ only.
- **Error state classification:** `ErrorEvent` carries only `error` — no auth/network/tool type. Error rendering uses the error message text to display appropriately. Auth errors are GCP-specific (LLM auth failure is fatal pre-TUI).
- **Input max height:** `InputModel` needs terminal height passed in. The `resetViewport()` hack conflicts with scroll-inside-box. May need to disable the reset when height is capped.
- **Tool names in code:** `read_file`, `write_file`, `edit_file`, `gcs`, `cloud_logging` (not `read`, `write`, `edit`, `gcs_browse`).
- **Status bar breakpoints:** Implement as a priority-based segment fitter rather than hard-coded width tiers. The DESIGN.md matrix defines the contract (what appears at each width), but the fitter achieves it dynamically.
- **`/clear` state:** Currently flips `showWelcome` off permanently. To match the state table (empty-conversation shows welcome), `/clear` should reset `showWelcome = true` when message count reaches 0.

## Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-03-27 | Initial design system created | Created by /design-consultation based on TUI interaction audit + competitive research (Claude Code, Codex, Warp, aider, Copilot CLI) |
| 2026-03-27 | Separate tool color (cyan) from warning (amber) | Tool bullets encode action category, not severity. BQ queries are cost-bearing but not warnings. |
| 2026-03-27 | Add data color (indigo) for platform tools | Cloud Logging and GCS are distinct from file I/O — deserve their own visual category |
| 2026-03-27 | Cost split: BQ + LLM in status bar | Data engineers care about cost more than any other AI tool user. Dual cost display is the differentiator. |
| 2026-03-27 | Keep spinner personality messages | Rotating messages (Pondering, Connecting the dots, etc.) add warmth. Revisit later if feedback suggests otherwise. |
| 2026-03-27 | No model name in spinner | Already visible in status bar. Redundant information wastes spinner real estate. |
| 2026-03-27 | Enable turn separator | `turnSeparator()` exists but isn't rendered. Dim `─` rules between turns improve scroll-back navigation. |
| 2026-03-27 | Progressive tool output with Space expand | Fixed 10+3 truncation forces users to re-ask. Space toggle removes friction. Tab reserved for approval navigation. |
| 2026-03-27 | Add application state table | Design review: 12 states with render contracts, transition rules, slash command feedback. Closes critical gap. |
| 2026-03-27 | Fix light-theme contrast | Codex measured: Tool 3.35:1, Success 3.42:1, Warning 2.89:1. All darkened to clear WCAG AA 4.5:1. |
| 2026-03-27 | Raise dark-theme Dim from #4B5563 to #64748B | Was 2.35:1 contrast — nearly invisible. Now ~3.7:1. Still dim but readable. |
| 2026-03-27 | Add color-blind bullet differentiation note | Tool bullets rely on color alone. Future: add glyph shapes (○/◇/●/△/□) for deuteranopia safety. |
| 2026-03-27 | Add terminal width breakpoint matrix | Status bar collapses progressively: ≥100 (full) → 80 → 60 → 40 → <40. Input max height: min(40%, 10 lines). |
| 2026-03-27 | Add input routing precedence table | Resolves Tab/Space/Up/Down collisions across confirm, picker, input, and viewport modes. |
| 2026-03-27 | Document welcome banner content | Existing bar logo + content spec, degraded states for no-GCP/no-auth. Onboarding deferred. |
| 2026-03-27 | Add turn anatomy diagram | Explicit reading order and render spec for each message type in a turn. |
