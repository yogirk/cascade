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
