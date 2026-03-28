# Cascade CLI — Development Guide

## Quick Start

```bash
make build       # Build binary to bin/cascade
make test        # Run all tests with race detector
make test-short  # Run internal tests (no race detector, faster)
make lint        # Run golangci-lint (falls back to go vet)
make clean       # Remove build artifacts
```

## Architecture

```
cmd/cascade/main.go          Entry point — CLI flags, interactive vs one-shot mode
internal/
  app/                        Composition root — wires all components
  agent/                      Agent loop (observe → reason → act), session, compaction
  provider/                   LLM abstraction (Provider interface)
    gemini/                   Google Gemini (default)
    openai/                   OpenAI (GPT-4o, o3-mini)
    anthropic/                Anthropic (Claude)
  platform/                   Platform intelligence: signals, correlation, /morning
    collectors/               Signal collectors (BQ jobs, logging, GCS, schema)
  tools/                      Tool interface + registry
    core/                     File tools: Read, Write, Edit, Glob, Grep, Bash
    bigquery/                 BigQuery query, schema, charts, render
    logging/                  Cloud Logging queries
    gcs/                      Cloud Storage browse/read
  permission/                 Permission engine: ASK / READ_ONLY / FULL_ACCESS modes
  bigquery/                   BQ client wrapper, SQL classification, cost tracking
  schema/                     SQLite + FTS5 schema cache from INFORMATION_SCHEMA
  persist/                    Session persistence (SQLite, auto-save, resume)
  config/                     TOML config loading with env/flag overrides
  auth/                       GCP resource auth + LLM provider auth
  tui/                        Bubble Tea TUI (model, chat, input, status, spinner, confirm)
  oneshot/                    One-shot mode runner
  testutil/                   MockProvider for tests
pkg/types/                    Provider-agnostic message, event, approval types
```

## Key Interfaces

- `provider.Provider` — `GenerateStream(ctx, messages, tools)` + `Model()`
- `tools.Tool` — `Name()`, `Description()`, `InputSchema()`, `RiskLevel()`, `Execute()`
- `tools.PermissionPlanner` — Optional input-aware risk gating
- `permission.ToolRiskProvider` — `Name()` + `RiskLevel()` (avoids circular dep)

## Adding a New Tool

1. Create package under `internal/tools/<name>/`
2. Implement the `tools.Tool` interface
3. Set appropriate `permission.RiskLevel` (ReadOnly, DML, DDL, Destructive, Admin)
4. If the tool needs input-aware risk classification, implement `tools.PermissionPlanner`
5. Add `RegisterAll()` function that registers tools with the registry
6. Wire in `internal/app/` — call `RegisterAll()` in `app.New()`
7. Add tests following patterns in `tools/bigquery/*_test.go`
8. Update system prompt in `internal/app/prompt.go` if the LLM needs guidance

## Testing

- Standard Go testing — no external frameworks
- Use `t.TempDir()` for file-based tests
- Mock providers via `internal/testutil/MockProvider`
- Mock tools via local `mockTool` structs in test files
- Run `go test ./... -count=1 -race` before committing

## Conventions

- Go 1.26+ with Charm v2 ecosystem (Bubble Tea, Lip Gloss, Glamour, Bubbles, Huh)
- Pure Go SQLite via `modernc.org/sqlite` (no cgo)
- Provider-agnostic types in `pkg/types/` — genai types never leak outside `provider/gemini/`
- Config layered: defaults < TOML < env vars < CLI flags
- Permission modes cycle: ASK → READ_ONLY → FULL_ACCESS → ASK
- Risk levels: ReadOnly < DML < DDL < Destructive < Admin
- SQL classification must handle CTE-wrapped statements (scan past WITH blocks)
- Schema cache functions require projectID parameter for multi-project correctness
- FTS5 rows must be DELETEd before re-inserting on sync (no ON CONFLICT support)

## CASCADE.md (Project Config)

Per-project config file discovered by walking up from cwd to git root (like CLAUDE.md).
Falls back to `~/.cascade/CASCADE.md` for global defaults. TOML format.

Sections: `[[critical_tables]]`, `[[schedules]]`, `[thresholds]`, `playbook`

Used by `/morning` to determine which tables are critical, expected refresh intervals,
and custom alert thresholds.

## Design System

Always read DESIGN.md before making any visual or UI decisions.
All colors, spacing, tool bullet semantics, and interaction patterns are defined there.
Do not deviate without explicit user approval.
In QA mode, flag any code that doesn't match DESIGN.md.

## Config

Config file: `~/.cascade/config.toml`

Key sections: `[gcp]`, `[model]`, `[agent]`, `[security]`, `[bigquery]`, `[cost]`, `[logging]`, `[gcs]`, `[display]`
