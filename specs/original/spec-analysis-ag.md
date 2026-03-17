# Cascade CLI — Specs Analysis & Observations

I have reviewed the 10 specification files in the `specs/` directory. Below is my detailed analysis, categorized by **Strengths**, **Risks/Challenges**, **Technical Feasibility**, and **Missing Elements**.

## 1. High-Level Observations
The vision for **Cascade** is extremely compelling: "Claude Code for GCP Data Engineering." It correctly identifies a gap in the market (generic AI coders vs. platform-specific data context). The architecture mimics Claude Code's successful patterns (single binary, agent loop, tool-first) but adds necessary data engineering primitives (schema cache, cost awareness).

However, the **scope is massive**. You are effectively building:
1. A terminal UI (like Claude Code)
2. An agentic framework (like LangChain/ADK)
3. A local database for metadata (Schema/History)
4. A deeply integrated client for ~6 huge GCP services
5. A dbt runner/parser
6. A security sandbox

## 2. Strengths & "Killer Features"
- **Cost Gates & Estimations**: This is the strongest differentiator. Generic agents freely run `SELECT *` and burn money. The pre-execution dry-run + budget check is a "must-have" for enterprise adoption.
- **Schema Cache (Context Engine)**: This handles the context window problem elegantly. Fetching only relevant table schemas (Tier 2) vs. dumping everything is the right approach.
- **Data Engineering Primitives**: First-class support for `dbt` (manifest parsing), `Dataplex` (lineage), and `Composer` (logs) makes this usable for real work, not just toy examples.
- **Go + Unix Philosophy**: Sticking to a single binary with no Python runtime dependency is a huge UX win for deployment and speed.

## 3. Risks & Problematic Choices

### A. The "Google ADK Go" Dependency
- **Risk**: `google.golang.org/adk` (referenced in ARCHITECTURE.md) is not a widely known or stable public package as of early 2025. If this is an internal or alpha Google library, building a public open-source tool on it is risky.
- **Mitigation**: Ensure you can fallback to standard `langchain-go` or write a custom minimal agent loop if ADK changes/deprecates. The agent loop logic in `ARCHITECTURE.md` is actually simple enough to roll your own.

### B. Sandbox Complexity (SECURITY.md)
- **Problem**: Implementing OS-level sandboxing (`sandbox-exec` on macOS, `bubblewrap` on Linux) is **extremely hard** to get right cross-platform without breaking harmless tools (like `git`, `dbt`, `terraform`).
- **Observation**: Claude Code does this, but they have a dedicated team. For a V1, this might be overkill and a source of constant "permission denied" bugs.
- **Suggestion**: Consider "Logical Sandboxing" (Middleware that checks commands against a whitelist) for Phase 1, rather than OS-level kernel enforcement.

### C. Context Compaction (CONTEXT.md)
- **Challenge**: "Summarize this conversation preserving technical details." LLMs are notoriously bad at summarizing large JSON blobs or SQL results without losing the exact syntax needed later.
- **Reality Check**: You might need to store "Reference pointers" in the summary (e.g., "See Result ID #45") rather than re-summarizing the table data, so the agent can re-fetch the raw data if needed.

### D. The Roadmap (ROADMAP.md)
- **Observation**: Phase 0-6 in 28 weeks is aggressive.
- **Bottleneck**: The "Data Engineering Workflows" (Phase 2) alone usually takes months to get robust (handling every dbt project structure, every weird BigQuery error).
- **Suggestion**: Scope Phase 1 to **Read-Only** (investigation/debugging). Fixing pipelines (Write/Edit) is 10x harder than diagnosing them.

## 4. Technical Feasibility Gaps

### 1. dbt Integration
- Parsing `manifest.json` in Go is non-trivial if the schema changes (dbt changes schemas often). You'll need strictly typed structs that match the user's dbt version.
- **Better approach**: Use `dbt ls --output json` or `dbt compile` subprocess calls rather than trying to parse the massive raw `manifest.json` yourself unless necessary.

### 2. Schema Cache Scalability
- **Scenario**: A user has 10,000 tables. `RefreshFull` (looping through all datasets/tables) will take forever and hit API quotas.
- **Fix**: Needs `CreateTime` / `UpdateTime` filtering in the initial list call to only fetch active tables.

### 3. "Subagents" in a CLI
- Running multiple context windows (Main + Subagent) concurrently in a terminal UI is complex to render and debug.
- **Simpler Start**: One context window, simpler tools. `CostAnalyzer` doesn't need to be a subagent; it can just be a tool that returns a summary.

## 5. Missing Elements

1. **Authentication Flows**:
    - How does it handle MFA for `gcloud`?
    - How does it handle different projects for different sessions? (It's in config, but switching is common).

2. **Update Mechanism**:
    - Single binaries need a `cascade update` command.

3. **Error Recovery**:
    - If the LLM writes bad SQL (which it will), is there a loop that feeds the error back to the LLM automatically? (The Architecture hints at an Agent Loop, but explicit "Error Correction" patterns are key).

## 6. Recommendations

1. **Simplify the Agent Model**: Drop "Subagents" for V1. Use a single strong model (Gemini 2.5 Pro / Claude 3.5 Sonnet) with robust tools.
2. **Focus on "Read" first**: Make the "Debugger" scenario (Scenario 1 & 4) rock solid before tackling "Builder" (Scenario 3).
    - Value proposition: "Cascade tells you *why* it failed in 5 seconds" is stronger than "Cascade writes your code."
3. **Double down on Cost**: The cost estimation logic should be your bedrock. If that fails or is inaccurate, trust is lost.
4. **Mock the ADK**: If Google ADK is not stable, write an interface wrapper so you can swap it out.

## Summary
The specs are incredibly high quality and well thought out. The primary risk is **execution complexity** vs. **resources**. "Data Engineering primitives" are complex edge-case factories.

**Green Light** to proceed, but I strongly suggest cutting scope on **Sandboxing** and **Subagents** for the initial alpha.
