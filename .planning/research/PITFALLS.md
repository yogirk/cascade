# Pitfalls Research

**Domain:** AI-native GCP data engineering terminal agent (Go + Bubble Tea + LLM + BigQuery/GCS/Composer)
**Researched:** 2026-03-16
**Confidence:** MEDIUM (based on training data across Go, Bubble Tea, BigQuery, GCP, and LLM agent architectures; no live verification available)

## Critical Pitfalls

### Pitfall 1: Bubble Tea Streaming Deadlock — Blocking the Event Loop with LLM Output

**What goes wrong:**
Bubble Tea runs a single-threaded event loop via its `Update` function. LLM streaming responses generate tokens continuously for seconds or minutes. If token delivery blocks the Bubble Tea event loop — or if too many `Cmd` messages pile up from a streaming goroutine — the TUI freezes, drops keystrokes, or deadlocks entirely. The user presses Ctrl+C and nothing happens. This is the single most common failure mode in Bubble Tea apps that integrate with streaming APIs.

**Why it happens:**
Developers treat Bubble Tea like a web framework where you can just push updates. But Bubble Tea is an Elm-architecture: `Update` returns a `(Model, Cmd)` and the runtime processes commands sequentially. If your streaming goroutine sends messages via a channel that fills up (because `Update` is slow processing a large render), the goroutine blocks, which can cascade into the LLM client blocking on its HTTP read, which holds a connection open indefinitely. Conversely, batching too aggressively causes visible stutter.

**How to avoid:**
- Use a dedicated goroutine for LLM streaming that writes to a ring buffer or channel with a generous buffer size (100+), never blocking on send.
- In `Update`, drain the channel non-blockingly per tick. Batch multiple tokens into a single render cycle using `tea.Tick` at 16-33ms intervals (30-60fps), not per-token messages.
- Separate the "token arrived" message from the "render viewport" cycle. Accumulate tokens in the model, re-render on a tick.
- Implement a cancellation channel that the streaming goroutine checks, so Ctrl+C can abort mid-stream.
- Never call `p.Send()` from a goroutine — always return messages via `tea.Cmd` or use `p.Send()` only from a `tea.Cmd`-spawned goroutine that properly handles backpressure.

**Warning signs:**
- TUI freezes for 1-2 seconds during long LLM responses
- Ctrl+C takes noticeable time to respond during streaming
- Token rendering appears "bursty" — nothing, then a wall of text
- High CPU usage during streaming with no visible output

**Phase to address:**
Phase 1 (Core Agent Loop + TUI). This is foundational. If the streaming/rendering pipeline is wrong from the start, retrofitting is a near-rewrite of the TUI layer.

---

### Pitfall 2: LLM Tool Call Parse Failures Treated as Unrecoverable Errors

**What goes wrong:**
LLMs generate malformed tool calls — wrong JSON, hallucinated parameter names, invalid argument types, partially-streamed JSON that fails to parse. If the agent loop treats any tool call parse failure as a hard error (crash, abort, or confused state), the tool becomes unreliable. Users see "error: invalid tool call" 10-20% of the time and lose trust.

**Why it happens:**
Developers test with clean examples and assume LLM output is well-formed. In production, models produce: (1) JSON with trailing commas, (2) tool names that are close-but-wrong (e.g., `bigquery_query` vs `BigQueryQuery`), (3) arguments that are strings when numbers are expected, (4) multiple tool calls when one was expected, (5) tool calls embedded in natural language rather than proper function_call format. The failure rate increases with complex schemas and less-capable models (Ollama local models being worst).

**How to avoid:**
- Implement lenient JSON parsing: strip markdown code fences, handle trailing commas, try `json.Unmarshal` then fall back to regex extraction.
- Fuzzy-match tool names (Levenshtein distance or prefix match) before rejecting.
- Coerce argument types where safe (string "42" to int 42, "true" to bool).
- On parse failure, send the error back to the LLM as a tool result with a clear error message and let it retry. Budget 2-3 retries before surfacing to user.
- Validate tool call schemas server-side before execution — never trust the LLM's output structure.
- Log all malformed tool calls for analysis (what model, what prompt, what went wrong).

**Warning signs:**
- Error rates above 5% for tool calls across any model provider
- Users report "it keeps saying it will do X but then errors"
- Different models have wildly different success rates
- Tool calls work in testing but fail with real user prompts

**Phase to address:**
Phase 1 (Core Agent Loop). The tool dispatch mechanism must be resilient from day one. This is the inner loop of the entire product.

---

### Pitfall 3: INFORMATION_SCHEMA Queries Scanning Entire Organization Instead of Target Project/Dataset

**What goes wrong:**
BigQuery's INFORMATION_SCHEMA has region-scoped and project-scoped views. Querying `region-us.INFORMATION_SCHEMA.COLUMNS` without dataset qualification scans across ALL datasets in the project. For large enterprises with 500+ datasets and 50K+ tables, this query takes 30-60 seconds, processes gigabytes, and costs real money. The schema cache build becomes a blocking, expensive operation that runs on first startup.

**Why it happens:**
The INFORMATION_SCHEMA documentation is confusing about scoping. There are dataset-level views (`dataset.INFORMATION_SCHEMA.COLUMNS` — scoped to one dataset), project-level views (`region-us.INFORMATION_SCHEMA.COLUMNS` — all datasets in project), and organization-level views. Developers often start with the project-level view because it seems like the "get everything" approach. But the project context already specifies dataset-scoped caching — the pitfall is accidentally using project-scoped views "for convenience" during implementation, or during the initial setup wizard scan.

**How to avoid:**
- Always use dataset-scoped INFORMATION_SCHEMA: `project.dataset.INFORMATION_SCHEMA.COLUMNS`. Never use the region-scoped view for cache building.
- In the setup wizard, enumerate datasets first (`INFORMATION_SCHEMA.SCHEMATA`), let the user select which to cache (default: all, but with a count/cost preview).
- Implement per-dataset cache building with progress reporting. If a dataset has 10K+ tables, warn and offer to skip.
- Set a query timeout (30s) on cache-building queries. If a dataset is too large, fall back to sampling or API-based metadata.
- Track bytes_processed from dry-run before executing cache queries.

**Warning signs:**
- Schema cache build takes more than 10 seconds
- Users report high BigQuery costs from Cascade itself
- First-run wizard hangs or appears frozen
- Cache queries appear in BigQuery audit logs scanning TB of metadata

**Phase to address:**
Phase 2 (Schema Cache + BigQuery tools). Must be correct at the point schema caching is implemented. Get the query scoping right from the first implementation.

---

### Pitfall 4: GCP Auth Token Expiry Crashing Mid-Session

**What goes wrong:**
Application Default Credentials (ADC) tokens expire after 1 hour (3600 seconds). If the token refresh fails silently or the HTTP client doesn't handle 401s with automatic retry, every GCP API call starts failing mid-session. The user is 45 minutes into a debugging session, asks "what failed in Composer last night?", and gets a cryptic auth error. Worse: some GCP client libraries cache the token and don't refresh, while others refresh transparently — behavior varies by library.

**Why it happens:**
During development and testing, sessions are short. The 1-hour token expiry is never hit. In production, data engineers sit in Cascade for hours. The Go GCP client libraries (`cloud.google.com/go`) handle token refresh automatically IF you use the standard `google.DefaultClient()` flow. But if you've created a custom HTTP client, wrapped the transport, or are using service account impersonation with a two-hop token chain, the refresh logic may not trigger correctly. Service account impersonation is particularly tricky: the impersonated token has its own expiry independent of the base credential.

**How to avoid:**
- Use `google.DefaultClient()` or `google.DefaultTokenSource()` and let the standard library handle refresh. Do not cache tokens manually.
- For service account impersonation, use `google.golang.org/api/impersonate` package which handles the token chain refresh.
- Wrap all GCP API calls with a retry-on-401 middleware that forces a token refresh and retries once.
- Test with artificially short token lifetimes (set `GOOGLE_AUTH_TOKEN_LIFETIME=60` equivalent if available, or mock).
- Display a subtle "re-authenticating..." indicator when a token refresh occurs rather than letting it be invisible.
- On auth failure after retry, provide actionable error: "Run `gcloud auth application-default login` to refresh credentials."

**Warning signs:**
- Any GCP call failing with 401/403 after the session has been open for 45+ minutes
- Tests pass but real usage fails after extended periods
- Service account impersonation works initially but breaks later
- Inconsistent auth failures (works sometimes, fails others — race condition in refresh)

**Phase to address:**
Phase 1 (GCP Auth foundation). Auth must be robust from the start since every subsequent tool depends on it. Test with long-running sessions in Phase 1.

---

### Pitfall 5: Context Window Exhaustion from Schema + Conversation History

**What goes wrong:**
The agent injects cached schema into the system prompt for SQL generation context. A warehouse with 500 tables, each with 20 columns, produces ~200KB of schema text. Combined with conversation history, tool call results (especially query outputs with many rows), and the platform summary — the context window fills up within 5-10 turns. The LLM starts hallucinating table names, forgetting earlier conversation, or hitting API token limits causing hard failures.

**Why it happens:**
Schema injection is greedy by default — "give the LLM everything it might need." Early testing uses small schemas (5-10 tables) where this works perfectly. With real warehouses, the schema alone can consume 30-50% of the context window. Add in a few query results with 100+ rows, some error logs from Composer, and the conversation is at 80% capacity before the user's actual question.

**How to avoid:**
- Never inject full schema. Use the FTS5 index to inject only relevant tables/columns based on the user's current query or topic. 10-20 relevant tables, not 500.
- Implement aggressive context compaction: summarize old conversation turns, drop raw query results after they've been discussed, replace verbose error logs with summaries.
- Set hard limits on tool result sizes: truncate query results to 50 rows with a "showing 50 of 1,234 rows" indicator. Summarize log output.
- Track token usage per message and warn when approaching 70% of context window. Auto-compact at 80%.
- Use Gemini's 2M context as a safety net, not a crutch. Even with 2M tokens, irrelevant context degrades response quality ("lost in the middle" problem).
- Consider a two-pass approach: first pass identifies relevant tables (cheap, small context), second pass generates SQL with only relevant schema.

**Warning signs:**
- LLM references tables that don't exist (hallucinated from partial schema)
- Response quality degrades noticeably after 10+ turns
- API calls failing with token limit errors
- SQL generation works for simple queries but fails when the user has been in a long session

**Phase to address:**
Phase 2 (Schema Cache) for schema injection strategy, Phase 3 (Context Management/Compaction) for conversation history management. Both must be designed together even if implemented in different phases.

---

### Pitfall 6: Cost Gate Bypass Through LLM-Generated SQL Variations

**What goes wrong:**
The cost estimation system does a dry-run of the SQL the LLM generates. But the LLM can generate semantically equivalent queries with vastly different costs. It might use `SELECT *` instead of selecting specific columns. It might scan a non-partitioned table when a partitioned equivalent exists. It might use a subquery that forces a full table scan when a `WHERE` clause on the partition key would limit it. The dry-run catches the cost correctly, but the LLM keeps generating expensive queries that get rejected, frustrating the user.

**Why it happens:**
LLMs optimize for correctness, not cost. Without explicit instruction about partitioning, clustering, and column selection, they write the simplest correct query. The schema context tells them what tables exist but not which are partitioned, what the partition key is, or what the clustering columns are. The cost gate then rejects the query, but the LLM doesn't know WHY it was rejected or how to write a cheaper version.

**How to avoid:**
- Include partition and clustering metadata in schema context. For each table, inject: partition column, partition type (day/month/hour), clustering columns, approximate row count, approximate size.
- When a query exceeds the cost budget, feed back specific guidance: "Query scans 2.3TB. Table `events` is partitioned by `event_date`. Add a `WHERE event_date >= '2026-03-01'` clause to reduce scan."
- Implement a SQL rewriter that automatically adds partition filters when they're missing (before dry-run, suggest to user).
- Default to `SELECT column1, column2` prompting rather than `SELECT *` in the system prompt.
- Set per-query cost limits AND per-session cumulative limits.

**Warning signs:**
- Dry-run rejections exceeding 30% of generated queries
- Users complaining "it keeps trying to run expensive queries"
- LLM entering a loop of generating rejected queries
- Cost estimates showing TB-scale scans for simple questions

**Phase to address:**
Phase 2 (BigQuery tools + cost estimation). The schema cache must store partition/clustering metadata from INFORMATION_SCHEMA.TABLE_OPTIONS and COLUMNS, and the cost gate must provide actionable feedback.

---

### Pitfall 7: SQLite FTS5 Index Corruption Under Concurrent Access from Subagents

**What goes wrong:**
The schema cache uses SQLite with FTS5 for full-text search. Subagents are fire-and-forget goroutines. If the main agent loop and a subagent (e.g., background schema refresh, background cost analysis querying cached schema) both write to or read from SQLite simultaneously, and the connection isn't properly configured for concurrent access, you get `SQLITE_BUSY` errors, corrupted FTS5 indexes, or silent data loss. `modernc.org/sqlite` (pure Go SQLite) has different concurrency characteristics than CGo SQLite.

**Why it happens:**
SQLite's concurrency model is "multiple readers, single writer" with WAL mode, but this requires correct configuration. `modernc.org/sqlite` supports this but defaults may differ. Developers open multiple `*sql.DB` connections or share one without proper mutex protection. FTS5 indexes are particularly fragile during writes — a failed write mid-FTS-update can leave the index inconsistent, causing future searches to return wrong results or panic.

**How to avoid:**
- Use a single `*sql.DB` instance for all SQLite access, with `_journal_mode=WAL&_busy_timeout=5000` pragmas set at connection open.
- Set `SetMaxOpenConns(1)` for write operations. Use a separate read-only connection pool for concurrent reads if needed.
- Wrap all FTS5 write operations (index rebuild, incremental update) in explicit transactions.
- Never write to the FTS5 index from a subagent goroutine. Schema cache writes should go through a serialized channel to a single writer goroutine.
- Implement FTS5 index integrity checks (`INSERT INTO fts_table(fts_table) VALUES('integrity-check')`) on startup.
- If FTS5 index is corrupted, rebuild it automatically from the base tables rather than crashing.

**Warning signs:**
- Intermittent `SQLITE_BUSY` errors in logs
- Schema search returning stale or missing results
- FTS5 queries that worked before suddenly returning empty
- Subagent goroutines logging database errors silently

**Phase to address:**
Phase 2 (Schema Cache with SQLite/FTS5). Design the concurrency model for the schema cache from the outset. Phase 3 (Subagents) must respect the established access patterns.

---

### Pitfall 8: Permission Model That Blocks Legitimate Workflows

**What goes wrong:**
The 5-level permission model (READ_ONLY through ADMIN) with CONFIRM-by-default seems safe. In practice, it produces "permission fatigue" — the user confirms 15 read-only operations in a row just to debug a pipeline, gets frustrated, switches to BYPASS mode, and then runs unguarded when a genuinely dangerous operation comes through. The permission system either annoys users into disabling it or fails to protect when it matters.

**Why it happens:**
Permission classification is designed around operation type, not user intent. A debugging workflow might involve: list DAGs (read), get DAG status (read), get task logs (read), query BigQuery logs (read), read GCS file (read), query INFORMATION_SCHEMA (read). Each triggers a confirmation. By the time the user finds the issue and wants to re-run a DAG task (write), they've already mentally checked out of the confirmation flow.

**How to avoid:**
- Auto-approve READ_ONLY operations in CONFIRM mode. Only prompt for WRITE and above.
- Implement "trust escalation" within a session: after confirming 3 operations in the same risk category, auto-approve the rest of that category for the session.
- Group related operations: "I'm going to run 3 read queries to investigate — approve all?" rather than one-by-one.
- Show a clear risk indicator in the TUI (green/yellow/red) rather than a blocking modal for low-risk operations.
- The PLAN mode (read-only) should be truly non-blocking — no confirmations needed.
- Reserve blocking confirmation dialogs for DML, DDL, DESTRUCTIVE, and ADMIN only.

**Warning signs:**
- Users immediately switching to BYPASS mode during demos or first use
- Feedback like "too many prompts" or "I just want to look at my data"
- Session logs showing 20+ confirmations with 0 rejections
- Users avoiding tools that trigger confirmations

**Phase to address:**
Phase 1 (Permission Engine). Design with read-auto-approve from the start. Phase 3 (Polish) for trust escalation and grouping.

---

### Pitfall 9: Agent Loop Infinite Cycling on Ambiguous User Requests

**What goes wrong:**
The user says "fix the pipeline." The LLM calls a tool to check DAG status. Gets results. Calls another tool to check logs. Gets results. Calls another tool to check the BigQuery table. Then calls DAG status again because it "wants more information." This cycles 10-20 times, burning tokens and time, without ever producing an answer or asking the user for clarification. The agent appears busy but accomplishes nothing.

**Why it happens:**
The agent loop is observe-reason-select-execute with no governor. The LLM's reasoning step says "I need more information" repeatedly because the request is genuinely ambiguous (which pipeline? which failure? what timeframe?). Without explicit loop limits, escalation triggers, or a "stop and ask" heuristic, the loop continues indefinitely. This is worse with weaker models that have less self-awareness about when to stop.

**How to avoid:**
- Hard limit of 15-20 tool calls per turn (configurable). After the limit, force the agent to summarize findings and respond.
- Implement a "diminishing returns" detector: if the last 3 tool calls returned similar information or the same tool is called twice with the same arguments, stop and synthesize.
- Add explicit "ask for clarification" as a tool. Teach the LLM (via system prompt) to use it when the request is ambiguous.
- Track tool call history within a turn and include it in the LLM context so it can see what it's already done.
- Implement cost tracking per turn — if a single turn has consumed $X or N tokens, warn and suggest compaction.
- After 5 tool calls with no user-facing output, inject a nudge: "You've made 5 tool calls. Provide a progress update or ask the user for clarification."

**Warning signs:**
- Average tool calls per turn exceeding 8-10
- Same tool being called multiple times with identical or near-identical arguments
- Users reporting "it's thinking for a long time"
- Token costs per conversation turn higher than expected

**Phase to address:**
Phase 1 (Core Agent Loop). The loop governor must be built into the agent loop from the start. Retrofitting it is surprisingly hard because the system prompt and tool design assume unlimited iteration.

---

### Pitfall 10: Treating Google ADK Go as Production-Stable When It Is Early-Stage

**What goes wrong:**
Google ADK Go was announced in early 2025 and is under active development. The API surface changes between versions. Documentation is sparse compared to the Python ADK. Features that exist in the Python ADK (structured output, automatic function calling, agent-to-agent communication) may not be available or may work differently in Go. Building the entire agent loop on ADK Go means absorbing its instability into Cascade's core.

**Why it happens:**
The ADK Go is appealing because it provides native GCP auth, a built-in agent loop, and Vertex AI integration. But "native agent loop" may not match Cascade's requirements (custom tool dispatch, streaming to Bubble Tea, permission checks before tool execution). The developer ends up fighting the framework rather than being helped by it, or discovers a missing feature 3 months into development.

**How to avoid:**
- Use ADK Go as a LLM client library (model calls, streaming, function declarations) but NOT as the agent loop orchestrator. Build Cascade's agent loop independently.
- Abstract ADK Go behind a Provider interface from day one. The interface should be: `Stream(ctx, messages, tools) -> TokenStream` and `Call(ctx, messages, tools) -> Response`. Nothing ADK-specific leaks beyond this boundary.
- Pin to a specific ADK Go version and test against it. Do not auto-update.
- Have a fallback plan: the Provider interface should allow dropping in a raw Gemini REST client, the Anthropic Go SDK, or `sashabaranov/go-openai` if ADK Go proves too immature.
- Monitor the ADK Go GitHub repo for breaking changes and release cadence.

**Warning signs:**
- ADK Go requiring workarounds for basic features (custom headers, timeout control, streaming cancellation)
- Breaking changes in minor version bumps
- Agent loop features that don't map to Cascade's permission model
- Difficulty implementing tool-call interception (for permission checks) within ADK Go's execution model

**Phase to address:**
Phase 1 (LLM Provider abstraction). The Provider interface must be designed and implemented before any ADK Go integration, so ADK Go is pluggable rather than foundational.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcoding Gemini-specific function calling format | Faster initial development | Every new provider needs format translation; bugs when formats diverge | Never — define a canonical tool call format in Phase 1 |
| Storing schema cache as flat JSON files | No SQLite dependency to set up | No FTS, no incremental updates, file locking issues, cache invalidation nightmare | Never — SQLite/FTS5 from the start |
| Inlining SQL strings for INFORMATION_SCHEMA queries | Quick to write | Unmaintainable, untestable, version-dependent on BigQuery schema changes | Only in prototyping; extract to templates before Phase 2 ends |
| Using `fmt.Sprintf` for BigQuery SQL construction | Fast, obvious | SQL injection via LLM-generated values passed to templates | Never — use parameterized queries for any user/LLM-sourced values |
| Global mutable state for current GCP project/dataset | Simple context passing | Race conditions with subagents, impossible to test, hard to add multi-project support | Only in Phase 1 prototype; refactor to injected config by Phase 2 |
| Skipping dry-run cost estimation in development | Faster iteration | Developers hit real costs; muscle memory of skipping transfers to production code | Never — dry-run is cheap and should always run |
| Single-file `main.go` agent loop | Rapid iteration | Untestable, unmockable, impossible to add features without merge conflicts | Phase 1 only; decompose into packages before Phase 2 |
| Dumping full query results into LLM context | LLM sees everything | Context exhaustion, slow responses, high token costs | Never — always truncate and summarize |

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| BigQuery | Using legacy SQL dialect by default | Always set `UseLegacySQL: false` explicitly in query config. Legacy SQL is still the default in some client library versions. |
| BigQuery | Not setting query timeout | Set `JobTimeout` and `QueryTimeout` separately. A query can be queued (job timeout) vs executing (query timeout). Default: 30s job creation, 120s execution. |
| BigQuery | Trusting dry-run cost as exact | Dry-run returns upper bound bytes processed. Actual cost can be lower due to caching, partition pruning at execution time. Communicate as "up to X" not "will cost X". |
| Cloud Composer | Assuming Airflow REST API is always available | Composer 2 exposes the Airflow REST API, but it requires the environment to be running. Stopped/updating environments return 503. Check environment status first. |
| Cloud Composer | Using the Airflow webserver URL directly | Use the Composer API to get the Airflow web UI URL and the DAG GCS bucket. These change between environment recreations. |
| Cloud Logging | Unbounded log queries | Cloud Logging queries without time bounds scan all retained logs (default 30 days). Always add `timestamp >= "time"` filter. Cost and latency scale linearly with time range. |
| Cloud Logging | Not handling pagination | Log queries return paginated results. A query for "all errors in the last hour" might return 10K entries across many pages. Set a reasonable limit (100-500) and offer "load more". |
| GCS | Reading large files into memory | `gsutil cat` on a 5GB Parquet file will OOM. Always check object size first (`storage.ObjectAttrs`), stream reads, and set a size limit (e.g., 10MB for `head` operations). |
| dbt | Assuming manifest.json is always current | `manifest.json` is generated by `dbt compile` or `dbt run`. If the user hasn't run dbt recently, the manifest is stale. Check modification time and warn. |
| dbt | Parsing manifest without version checking | dbt manifest format changes between versions (v7, v8, v9, etc.). Check `metadata.dbt_schema_version` before parsing. |
| ADC Auth | Assuming `GOOGLE_APPLICATION_CREDENTIALS` is set | ADC works without this env var (uses `gcloud auth application-default login` credentials). Check both paths. Common in containers vs local dev. |
| Service Account Impersonation | Not checking `iam.serviceAccountTokenCreator` role | Impersonation requires the caller to have this role on the target SA. Fail with a clear message, not a generic 403. |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full schema injection into every LLM call | Slow responses, high token costs | Use FTS5 to inject only relevant tables (10-20 max per call) | 100+ tables in schema cache |
| Synchronous schema cache refresh on startup | Startup takes 30s+ for large warehouses | Build cache asynchronously; use stale cache immediately, refresh in background | 500+ tables across configured datasets |
| Unbatched INFORMATION_SCHEMA queries (one per dataset) | N sequential queries for N datasets | Use concurrent goroutines with rate limiting (5 concurrent max) | 10+ datasets configured |
| Re-parsing dbt manifest on every dbt-related command | Noticeable delay (1-2s) for large projects | Parse once at startup/on-change, hold in memory. Watch `manifest.json` mtime. | 500+ dbt models |
| Storing full query results in conversation history | Context window exhaustion, slow compaction | Store only: query SQL, row count, first 10 rows, summary stats | Any query returning 100+ rows |
| Rendering full markdown in TUI on every keystroke | TUI lag, dropped frames | Debounce markdown rendering to 100ms minimum. Cache rendered output until content changes. | Long LLM responses (1000+ tokens), complex markdown with tables/code blocks |
| Creating new GCP client instances per tool call | Connection overhead, auth token churn | Create clients once at startup, reuse via dependency injection | Any session with 20+ tool calls |
| SQLite FTS5 `MATCH` with very common terms | Slow queries (100ms+) on large indexes | Use prefix queries and column-scoped FTS. Exclude common words (id, name, type) from FTS index or use custom tokenizer. | 50K+ columns indexed |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Passing LLM-generated SQL directly to BigQuery without any sanitization | LLM could generate DDL/DML even when user expects read-only query; prompt injection via data values | Parse SQL with a Go SQL parser before execution; reject DDL/DML in read-only mode; use BigQuery `dryRun` to check query type before execution |
| Logging full query results that may contain PII | Sensitive data persisted to disk in session logs or SQLite | Never log query result data to files. Log only: query SQL, row count, column names. PII detection should flag before display, not after logging. |
| Storing GCP credentials or tokens in config files | Credential theft if dotfiles are committed to git or shared | Never store credentials. Use ADC exclusively. Add `.cascade/` to `.gitignore` template. Warn if `config.toml` contains anything resembling a key or token. |
| Service account impersonation without scope restriction | Impersonated SA may have broader permissions than intended | Always request minimum scopes. Use `--scopes` to limit to BigQuery, Storage, Logging, Composer. |
| Executing bash commands from LLM without path restriction | LLM could `rm -rf`, access secrets, exfiltrate data | Bash tool must have: working directory restriction, blocked command list (`rm -rf /`, `curl` to external hosts, credential file reads), timeout, output size limit |
| Trusting LLM-suggested GCS paths without validation | Path traversal: `gs://bucket/../other-bucket/sensitive` | Validate GCS paths against allowed bucket list from config. Reject paths with `..` or paths outside configured buckets. |
| Session history containing sensitive data accessible to future sessions | Cross-session data leakage | Sessions are isolated. Old session data is not injected into new sessions. Session files encrypted at rest or stored in memory only. |
| Cost budget bypass via many small queries | 1000 queries at $0.01 each = $10, bypassing per-query $5 limit | Track cumulative session cost. Alert at configurable threshold. Implement per-session budget, not just per-query. |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing raw BigQuery job errors | Users see "invalidQuery: Syntax error at [1:45]" with no context about what the LLM tried | Show the generated SQL with syntax highlighting, the error with the problematic location highlighted, and let the LLM auto-retry with the error context |
| No progress indication during long operations | User thinks the tool crashed during schema cache build or complex queries | Show a spinner with context: "Building schema cache... dataset_1 (234 tables)" or "Running query... (estimated 2.3s, processing 450MB)" |
| Displaying 10,000 rows in the terminal | Terminal hangs, scrollback destroyed | Default to 25 rows with pagination. Show row count, offer `/more` command. Offer CSV/JSON export for full results. |
| Making first-run setup mandatory and blocking | User just wants to try the tool, gets a 2-minute setup wizard | Setup wizard should be skippable. Auto-detect what's available (project, datasets, Composer) and work with what's found. Cache builds in background. |
| Dense information in query cost warnings | User ignores cost warnings because they're a wall of text | Simple traffic light: green (< $0.01), yellow ($0.01-$1.00), red (> $1.00) with one-line summary. Detail available via expand. |
| Not showing what the LLM is doing | User sees blank screen for 5-10 seconds while LLM reasons | Stream thinking/reasoning tokens. Show "Analyzing pipeline..." or tool call annotations in real-time. |
| Forcing keyboard-only interaction | Power users love it; new users are lost | Support both slash commands AND natural language for common operations. `/failures` and "show me recent failures" should both work. |
| Inconsistent date/time handling | "Show failures from yesterday" interpreted differently depending on timezone | Always display timestamps in the user's local timezone. Use relative time for recent events ("2 hours ago"). Let config set timezone explicitly. |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **BigQuery tool:** Often missing handling of `REPEATED` (array) and `RECORD` (struct) column types in schema cache -- these need nested representation, not flat columns
- [ ] **Schema cache:** Often missing view definitions and materialized view refresh schedules -- users ask "what does this view do?" and the cache has no answer
- [ ] **Cost estimation:** Often missing slot-based pricing calculation -- dry-run returns bytes, but some orgs use flat-rate/editions pricing where bytes don't map to dollars
- [ ] **Composer integration:** Often missing support for Composer 1 vs Composer 2 API differences -- the Airflow REST API endpoint format differs significantly
- [ ] **dbt integration:** Often missing handling of dbt packages (dependencies) -- manifest includes package models that aren't "yours" but appear in lineage
- [ ] **Permission model:** Often missing handling of tool chains -- a "safe" read tool that triggers an "unsafe" write tool via LLM reasoning isn't caught by per-tool classification
- [ ] **Session management:** Often missing graceful recovery from crash -- if Cascade is killed mid-session, the next startup should offer to resume or show what was lost
- [ ] **Config file:** Often missing validation of GCP project/dataset existence at config load time -- user typos a dataset name, gets cryptic errors 5 minutes later
- [ ] **Streaming output:** Often missing handling of LLM provider rate limits (429) -- user hits rate limit mid-stream, sees partial output with no explanation
- [ ] **Error retry:** Often missing exponential backoff -- retrying immediately on transient GCP errors (503, quota exceeded) makes the problem worse

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Streaming deadlock in TUI | MEDIUM | Requires rearchitecting the message/render pipeline. Cannot patch — must redesign the token accumulation and render tick system. Estimate: 2-3 days for an experienced developer. |
| Tool call parse failures | LOW | Add lenient parsing as a middleware layer. Does not require agent loop changes. Can be done incrementally per failure mode observed. |
| INFORMATION_SCHEMA over-scanning | LOW | Change query scope from project-level to dataset-level. One-line fix per query, but must audit all INFORMATION_SCHEMA queries. |
| Auth token expiry crashes | LOW | Add retry middleware around GCP client calls. 1-2 days of work if the GCP client is properly abstracted. |
| Context window exhaustion | HIGH | Requires redesigning schema injection, adding truncation to all tool results, implementing compaction. Touches every tool. Estimate: 1-2 weeks. |
| Cost gate bypass | MEDIUM | Add partition metadata to schema cache (schema change), update system prompt with cost-aware instructions, add SQL rewriting layer. Estimate: 3-5 days. |
| SQLite FTS5 corruption | MEDIUM | Implement automatic rebuild from base tables. Add integrity check on startup. Requires schema cache to have non-FTS base tables as source of truth. |
| Permission fatigue causing BYPASS overuse | MEDIUM | Redesign permission UX: auto-approve reads, add trust escalation. Requires changes to permission engine and TUI confirmation flow. |
| Agent loop infinite cycling | LOW | Add loop counter and governor. System prompt changes + 1 day of agent loop code. |
| ADK Go immaturity causing rework | HIGH | If ADK Go is deeply embedded, switching providers requires rewriting the LLM integration layer. With proper Provider interface, it's a 2-3 day swap. Without it, 2-3 weeks. |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Streaming deadlock | Phase 1: Core TUI + Agent Loop | Stress test with 5000-token streaming response while rapidly pressing keys. No dropped frames, Ctrl+C responds within 500ms. |
| Tool call parse failures | Phase 1: Agent Loop + Tool Dispatch | Fuzz test tool calls with malformed JSON, wrong types, missing fields. Recovery rate > 95%. |
| INFORMATION_SCHEMA over-scanning | Phase 2: Schema Cache | Verify every INFORMATION_SCHEMA query uses dataset-scoped view. Check bytes_processed in test against known dataset. |
| Auth token expiry | Phase 1: GCP Auth Layer | Integration test with 70-minute session (or mock token with 60s expiry). All API calls succeed after refresh. |
| Context window exhaustion | Phase 2-3: Schema Injection + Compaction | Run 30-turn conversation with 500-table warehouse. Token usage stays below 60% of context window. Response quality doesn't degrade. |
| Cost gate bypass | Phase 2: BigQuery Tools + Cost Gate | Generate queries against partitioned tables. Verify partition filter guidance is provided on rejection. Rejection rate below 20% for common queries. |
| SQLite FTS5 corruption | Phase 2: Schema Cache | Concurrent read/write stress test. Kill process mid-write 10 times. FTS5 integrity check passes after each restart. |
| Permission fatigue | Phase 1: Permission Engine | UX test: complete a 10-step debugging workflow. Count confirmations. Should be 0 for read-only steps, 1-2 for write steps. |
| Agent loop cycling | Phase 1: Agent Loop | Test with intentionally ambiguous prompts ("fix it", "check everything"). Loop terminates within 15 tool calls. Agent asks for clarification. |
| ADK Go immaturity | Phase 1: Provider Interface | Implement two providers (Gemini via ADK Go + one other). Swap providers in config. Both pass the same integration test suite. |

## Sources

- Training data knowledge of Bubble Tea architecture (Elm architecture pattern, message-passing model, common issues reported in charmbracelet/bubbletea GitHub issues and discussions)
- Training data knowledge of BigQuery INFORMATION_SCHEMA scoping (project-level vs dataset-level views, bytes_processed behavior, partition metadata tables)
- Training data knowledge of GCP Application Default Credentials lifecycle (token refresh, service account impersonation chain, `google.DefaultTokenSource` behavior)
- Training data knowledge of SQLite WAL mode concurrency and FTS5 index integrity (modernc.org/sqlite behavior vs CGo sqlite3)
- Training data knowledge of LLM agent loop patterns from Claude Code, OpenCode, Aider, and similar tools (tool call failure modes, context window management, loop governors)
- Training data knowledge of Go concurrency patterns (goroutine lifecycle, channel buffering, context cancellation)
- BigQuery pricing documentation (on-demand vs flat-rate, dry-run semantics)
- Cloud Composer API documentation (Composer 1 vs 2 differences, Airflow REST API availability)

**Confidence note:** All findings are based on training data (cutoff ~May 2025). Google ADK Go findings are MEDIUM confidence as the framework was very new at training cutoff and may have matured significantly. All other findings are MEDIUM-HIGH confidence as they cover well-established technologies with known failure modes.

---
*Pitfalls research for: AI-native GCP data engineering terminal agent*
*Researched: 2026-03-16*

# Pitfalls Research

**Domain:** AI-native GCP data engineering terminal agent (Go + Bubble Tea + LLM + BigQuery/GCS/Composer)
**Researched:** 2026-03-16
**Confidence:** MEDIUM (based on training data across Go, Bubble Tea, BigQuery, GCP, and LLM agent architectures; no live verification available)

## Critical Pitfalls

### Pitfall 1: Bubble Tea Streaming Deadlock — Blocking the Event Loop with LLM Output

**What goes wrong:**
Bubble Tea runs a single-threaded event loop via its `Update` function. LLM streaming responses generate tokens continuously for seconds or minutes. If token delivery blocks the Bubble Tea event loop — or if too many `Cmd` messages pile up from a streaming goroutine — the TUI freezes, drops keystrokes, or deadlocks entirely. The user presses Ctrl+C and nothing happens. This is the single most common failure mode in Bubble Tea apps that integrate with streaming APIs.

**Why it happens:**
Developers treat Bubble Tea like a web framework where you can just push updates. But Bubble Tea is an Elm-architecture: `Update` returns a `(Model, Cmd)` and the runtime processes commands sequentially. If your streaming goroutine sends messages via a channel that fills up (because `Update` is slow processing a large render), the goroutine blocks, which can cascade into the LLM client blocking on its HTTP read, which holds a connection open indefinitely. Conversely, batching too aggressively causes visible stutter.

**How to avoid:**
- Use a dedicated goroutine for LLM streaming that writes to a ring buffer or channel with a generous buffer size (100+), never blocking on send.
- In `Update`, drain the channel non-blockingly per tick. Batch multiple tokens into a single render cycle using `tea.Tick` at 16-33ms intervals (30-60fps), not per-token messages.
- Separate the "token arrived" message from the "render viewport" cycle. Accumulate tokens in the model, re-render on a tick.
- Implement a cancellation channel that the streaming goroutine checks, so Ctrl+C can abort mid-stream.
- Never call `p.Send()` from a goroutine — always return messages via `tea.Cmd` or use `p.Send()` only from a `tea.Cmd`-spawned goroutine that properly handles backpressure.

**Warning signs:**
- TUI freezes for 1-2 seconds during long LLM responses
- Ctrl+C takes noticeable time to respond during streaming
- Token rendering appears "bursty" — nothing, then a wall of text
- High CPU usage during streaming with no visible output

**Phase to address:**
Phase 1 (Core Agent Loop + TUI). This is foundational. If the streaming/rendering pipeline is wrong from the start, retrofitting is a near-rewrite of the TUI layer.

---

### Pitfall 2: LLM Tool Call Parse Failures Treated as Unrecoverable Errors

**What goes wrong:**
LLMs generate malformed tool calls — wrong JSON, hallucinated parameter names, invalid argument types, partially-streamed JSON that fails to parse. If the agent loop treats any tool call parse failure as a hard error (crash, abort, or confused state), the tool becomes unreliable. Users see "error: invalid tool call" 10-20% of the time and lose trust.

**Why it happens:**
Developers test with clean examples and assume LLM output is well-formed. In production, models produce: (1) JSON with trailing commas, (2) tool names that are close-but-wrong (e.g., `bigquery_query` vs `BigQueryQuery`), (3) arguments that are strings when numbers are expected, (4) multiple tool calls when one was expected, (5) tool calls embedded in natural language rather than proper function_call format. The failure rate increases with complex schemas and less-capable models (Ollama local models being worst).

**How to avoid:**
- Implement lenient JSON parsing: strip markdown code fences, handle trailing commas, try `json.Unmarshal` then fall back to regex extraction.
- Fuzzy-match tool names (Levenshtein distance or prefix match) before rejecting.
- Coerce argument types where safe (string "42" to int 42, "true" to bool).
- On parse failure, send the error back to the LLM as a tool result with a clear error message and let it retry. Budget 2-3 retries before surfacing to user.
- Validate tool call schemas server-side before execution — never trust the LLM's output structure.
- Log all malformed tool calls for analysis (what model, what prompt, what went wrong).

**Warning signs:**
- Error rates above 5% for tool calls across any model provider
- Users report "it keeps saying it will do X but then errors"
- Different models have wildly different success rates
- Tool calls work in testing but fail with real user prompts

**Phase to address:**
Phase 1 (Core Agent Loop). The tool dispatch mechanism must be resilient from day one. This is the inner loop of the entire product.

---

### Pitfall 3: INFORMATION_SCHEMA Queries Scanning Entire Organization Instead of Target Project/Dataset

**What goes wrong:**
BigQuery's INFORMATION_SCHEMA has region-scoped and project-scoped views. Querying `region-us.INFORMATION_SCHEMA.COLUMNS` without dataset qualification scans across ALL datasets in the project. For large enterprises with 500+ datasets and 50K+ tables, this query takes 30-60 seconds, processes gigabytes, and costs real money. The schema cache build becomes a blocking, expensive operation that runs on first startup.

**Why it happens:**
The INFORMATION_SCHEMA documentation is confusing about scoping. There are dataset-level views (`dataset.INFORMATION_SCHEMA.COLUMNS` -- scoped to one dataset), project-level views (`region-us.INFORMATION_SCHEMA.COLUMNS` -- all datasets in project), and organization-level views. Developers often start with the project-level view because it seems like the "get everything" approach. But the project context already specifies dataset-scoped caching -- the pitfall is accidentally using project-scoped views "for convenience" during implementation, or during the initial setup wizard scan.

**How to avoid:**
- Always use dataset-scoped INFORMATION_SCHEMA: `project.dataset.INFORMATION_SCHEMA.COLUMNS`. Never use the region-scoped view for cache building.
- In the setup wizard, enumerate datasets first (`INFORMATION_SCHEMA.SCHEMATA`), let the user select which to cache (default: all, but with a count/cost preview).
- Implement per-dataset cache building with progress reporting. If a dataset has 10K+ tables, warn and offer to skip.
- Set a query timeout (30s) on cache-building queries. If a dataset is too large, fall back to sampling or API-based metadata.
- Track bytes_processed from dry-run before executing cache queries.

**Warning signs:**
- Schema cache build takes more than 10 seconds
- Users report high BigQuery costs from Cascade itself
- First-run wizard hangs or appears frozen
- Cache queries appear in BigQuery audit logs scanning TB of metadata

**Phase to address:**
Phase 2 (Schema Cache + BigQuery tools). Must be correct at the point schema caching is implemented. Get the query scoping right from the first implementation.

---

### Pitfall 4: GCP Auth Token Expiry Crashing Mid-Session

**What goes wrong:**
Application Default Credentials (ADC) tokens expire after 1 hour (3600 seconds). If the token refresh fails silently or the HTTP client doesn't handle 401s with automatic retry, every GCP API call starts failing mid-session. The user is 45 minutes into a debugging session, asks "what failed in Composer last night?", and gets a cryptic auth error. Worse: some GCP client libraries cache the token and don't refresh, while others refresh transparently -- behavior varies by library.

**Why it happens:**
During development and testing, sessions are short. The 1-hour token expiry is never hit. In production, data engineers sit in Cascade for hours. The Go GCP client libraries (`cloud.google.com/go`) handle token refresh automatically IF you use the standard `google.DefaultClient()` flow. But if you've created a custom HTTP client, wrapped the transport, or are using service account impersonation with a two-hop token chain, the refresh logic may not trigger correctly. Service account impersonation is particularly tricky: the impersonated token has its own expiry independent of the base credential.

**How to avoid:**
- Use `google.DefaultClient()` or `google.DefaultTokenSource()` and let the standard library handle refresh. Do not cache tokens manually.
- For service account impersonation, use `google.golang.org/api/impersonate` package which handles the token chain refresh.
- Wrap all GCP API calls with a retry-on-401 middleware that forces a token refresh and retries once.
- Test with artificially short token lifetimes or mock token expiry scenarios.
- Display a subtle "re-authenticating..." indicator when a token refresh occurs rather than letting it be invisible.
- On auth failure after retry, provide actionable error: "Run `gcloud auth application-default login` to refresh credentials."

**Warning signs:**
- Any GCP call failing with 401/403 after the session has been open for 45+ minutes
- Tests pass but real usage fails after extended periods
- Service account impersonation works initially but breaks later
- Inconsistent auth failures (works sometimes, fails others -- race condition in refresh)

**Phase to address:**
Phase 1 (GCP Auth foundation). Auth must be robust from the start since every subsequent tool depends on it. Test with long-running sessions in Phase 1.

---

### Pitfall 5: Context Window Exhaustion from Schema + Conversation History

**What goes wrong:**
The agent injects cached schema into the system prompt for SQL generation context. A warehouse with 500 tables, each with 20 columns, produces ~200KB of schema text. Combined with conversation history, tool call results (especially query outputs with many rows), and the platform summary -- the context window fills up within 5-10 turns. The LLM starts hallucinating table names, forgetting earlier conversation, or hitting API token limits causing hard failures.

**Why it happens:**
Schema injection is greedy by default -- "give the LLM everything it might need." Early testing uses small schemas (5-10 tables) where this works perfectly. With real warehouses, the schema alone can consume 30-50% of the context window. Add in a few query results with 100+ rows, some error logs from Composer, and the conversation is at 80% capacity before the user's actual question.

**How to avoid:**
- Never inject full schema. Use the FTS5 index to inject only relevant tables/columns based on the user's current query or topic. 10-20 relevant tables, not 500.
- Implement aggressive context compaction: summarize old conversation turns, drop raw query results after they've been discussed, replace verbose error logs with summaries.
- Set hard limits on tool result sizes: truncate query results to 50 rows with a "showing 50 of 1,234 rows" indicator. Summarize log output.
- Track token usage per message and warn when approaching 70% of context window. Auto-compact at 80%.
- Use Gemini's 2M context as a safety net, not a crutch. Even with 2M tokens, irrelevant context degrades response quality ("lost in the middle" problem).
- Consider a two-pass approach: first pass identifies relevant tables (cheap, small context), second pass generates SQL with only relevant schema.

**Warning signs:**
- LLM references tables that don't exist (hallucinated from partial schema)
- Response quality degrades noticeably after 10+ turns
- API calls failing with token limit errors
- SQL generation works for simple queries but fails when the user has been in a long session

**Phase to address:**
Phase 2 (Schema Cache) for schema injection strategy, Phase 3 (Context Management/Compaction) for conversation history management. Both must be designed together even if implemented in different phases.

---

### Pitfall 6: Cost Gate Bypass Through LLM-Generated SQL Variations

**What goes wrong:**
The cost estimation system does a dry-run of the SQL the LLM generates. But the LLM can generate semantically equivalent queries with vastly different costs. It might use `SELECT *` instead of selecting specific columns. It might scan a non-partitioned table when a partitioned equivalent exists. It might use a subquery that forces a full table scan when a `WHERE` clause on the partition key would limit it. The dry-run catches the cost correctly, but the LLM keeps generating expensive queries that get rejected, frustrating the user.

**Why it happens:**
LLMs optimize for correctness, not cost. Without explicit instruction about partitioning, clustering, and column selection, they write the simplest correct query. The schema context tells them what tables exist but not which are partitioned, what the partition key is, or what the clustering columns are. The cost gate then rejects the query, but the LLM doesn't know WHY it was rejected or how to write a cheaper version.

**How to avoid:**
- Include partition and clustering metadata in schema context. For each table, inject: partition column, partition type (day/month/hour), clustering columns, approximate row count, approximate size.
- When a query exceeds the cost budget, feed back specific guidance: "Query scans 2.3TB. Table `events` is partitioned by `event_date`. Add a `WHERE event_date >= '2026-03-01'` clause to reduce scan."
- Implement a SQL rewriter that automatically adds partition filters when they're missing (before dry-run, suggest to user).
- Default to `SELECT column1, column2` prompting rather than `SELECT *` in the system prompt.
- Set per-query cost limits AND per-session cumulative limits.

**Warning signs:**
- Dry-run rejections exceeding 30% of generated queries
- Users complaining "it keeps trying to run expensive queries"
- LLM entering a loop of generating rejected queries
- Cost estimates showing TB-scale scans for simple questions

**Phase to address:**
Phase 2 (BigQuery tools + cost estimation). The schema cache must store partition/clustering metadata from INFORMATION_SCHEMA.TABLE_OPTIONS and COLUMNS, and the cost gate must provide actionable feedback.

---

### Pitfall 7: SQLite FTS5 Index Corruption Under Concurrent Access from Subagents

**What goes wrong:**
The schema cache uses SQLite with FTS5 for full-text search. Subagents are fire-and-forget goroutines. If the main agent loop and a subagent (e.g., background schema refresh, background cost analysis querying cached schema) both write to or read from SQLite simultaneously, and the connection isn't properly configured for concurrent access, you get `SQLITE_BUSY` errors, corrupted FTS5 indexes, or silent data loss. `modernc.org/sqlite` (pure Go SQLite) has different concurrency characteristics than CGo SQLite.

**Why it happens:**
SQLite's concurrency model is "multiple readers, single writer" with WAL mode, but this requires correct configuration. `modernc.org/sqlite` supports this but defaults may differ. Developers open multiple `*sql.DB` connections or share one without proper mutex protection. FTS5 indexes are particularly fragile during writes -- a failed write mid-FTS-update can leave the index inconsistent, causing future searches to return wrong results or panic.

**How to avoid:**
- Use a single `*sql.DB` instance for all SQLite access, with `_journal_mode=WAL&_busy_timeout=5000` pragmas set at connection open.
- Set `SetMaxOpenConns(1)` for write operations. Use a separate read-only connection pool for concurrent reads if needed.
- Wrap all FTS5 write operations (index rebuild, incremental update) in explicit transactions.
- Never write to the FTS5 index from a subagent goroutine. Schema cache writes should go through a serialized channel to a single writer goroutine.
- Implement FTS5 index integrity checks (`INSERT INTO fts_table(fts_table) VALUES('integrity-check')`) on startup.
- If FTS5 index is corrupted, rebuild it automatically from the base tables rather than crashing.

**Warning signs:**
- Intermittent `SQLITE_BUSY` errors in logs
- Schema search returning stale or missing results
- FTS5 queries that worked before suddenly returning empty
- Subagent goroutines logging database errors silently

**Phase to address:**
Phase 2 (Schema Cache with SQLite/FTS5). Design the concurrency model for the schema cache from the outset. Phase 3 (Subagents) must respect the established access patterns.

---

### Pitfall 8: Permission Model That Blocks Legitimate Workflows

**What goes wrong:**
The 5-level permission model (READ_ONLY through ADMIN) with CONFIRM-by-default seems safe. In practice, it produces "permission fatigue" -- the user confirms 15 read-only operations in a row just to debug a pipeline, gets frustrated, switches to BYPASS mode, and then runs unguarded when a genuinely dangerous operation comes through. The permission system either annoys users into disabling it or fails to protect when it matters.

**Why it happens:**
Permission classification is designed around operation type, not user intent. A debugging workflow might involve: list DAGs (read), get DAG status (read), get task logs (read), query BigQuery logs (read), read GCS file (read), query INFORMATION_SCHEMA (read). Each triggers a confirmation. By the time the user finds the issue and wants to re-run a DAG task (write), they've already mentally checked out of the confirmation flow.

**How to avoid:**
- Auto-approve READ_ONLY operations in CONFIRM mode. Only prompt for WRITE and above.
- Implement "trust escalation" within a session: after confirming 3 operations in the same risk category, auto-approve the rest of that category for the session.
- Group related operations: "I'm going to run 3 read queries to investigate -- approve all?" rather than one-by-one.
- Show a clear risk indicator in the TUI (green/yellow/red) rather than a blocking modal for low-risk operations.
- The PLAN mode (read-only) should be truly non-blocking -- no confirmations needed.
- Reserve blocking confirmation dialogs for DML, DDL, DESTRUCTIVE, and ADMIN only.

**Warning signs:**
- Users immediately switching to BYPASS mode during demos or first use
- Feedback like "too many prompts" or "I just want to look at my data"
- Session logs showing 20+ confirmations with 0 rejections
- Users avoiding tools that trigger confirmations

**Phase to address:**
Phase 1 (Permission Engine). Design with read-auto-approve from the start. Phase 3 (Polish) for trust escalation and grouping.

---

### Pitfall 9: Agent Loop Infinite Cycling on Ambiguous User Requests

**What goes wrong:**
The user says "fix the pipeline." The LLM calls a tool to check DAG status. Gets results. Calls another tool to check logs. Gets results. Calls another tool to check the BigQuery table. Then calls DAG status again because it "wants more information." This cycles 10-20 times, burning tokens and time, without ever producing an answer or asking the user for clarification. The agent appears busy but accomplishes nothing.

**Why it happens:**
The agent loop is observe-reason-select-execute with no governor. The LLM's reasoning step says "I need more information" repeatedly because the request is genuinely ambiguous (which pipeline? which failure? what timeframe?). Without explicit loop limits, escalation triggers, or a "stop and ask" heuristic, the loop continues indefinitely. This is worse with weaker models that have less self-awareness about when to stop.

**How to avoid:**
- Hard limit of 15-20 tool calls per turn (configurable). After the limit, force the agent to summarize findings and respond.
- Implement a "diminishing returns" detector: if the last 3 tool calls returned similar information or the same tool is called twice with the same arguments, stop and synthesize.
- Add explicit "ask for clarification" as a tool. Teach the LLM (via system prompt) to use it when the request is ambiguous.
- Track tool call history within a turn and include it in the LLM context so it can see what it's already done.
- Implement cost tracking per turn -- if a single turn has consumed $X or N tokens, warn and suggest compaction.
- After 5 tool calls with no user-facing output, inject a nudge: "You've made 5 tool calls. Provide a progress update or ask the user for clarification."

**Warning signs:**
- Average tool calls per turn exceeding 8-10
- Same tool being called multiple times with identical or near-identical arguments
- Users reporting "it's thinking for a long time"
- Token costs per conversation turn higher than expected

**Phase to address:**
Phase 1 (Core Agent Loop). The loop governor must be built into the agent loop from the start. Retrofitting it is surprisingly hard because the system prompt and tool design assume unlimited iteration.

---

### Pitfall 10: Treating Google ADK Go as Production-Stable When It Is Early-Stage

**What goes wrong:**
Google ADK Go was announced in early 2025 and is under active development. The API surface changes between versions. Documentation is sparse compared to the Python ADK. Features that exist in the Python ADK (structured output, automatic function calling, agent-to-agent communication) may not be available or may work differently in Go. Building the entire agent loop on ADK Go means absorbing its instability into Cascade's core.

**Why it happens:**
The ADK Go is appealing because it provides native GCP auth, a built-in agent loop, and Vertex AI integration. But "native agent loop" may not match Cascade's requirements (custom tool dispatch, streaming to Bubble Tea, permission checks before tool execution). The developer ends up fighting the framework rather than being helped by it, or discovers a missing feature 3 months into development.

**How to avoid:**
- Use ADK Go as a LLM client library (model calls, streaming, function declarations) but NOT as the agent loop orchestrator. Build Cascade's agent loop independently.
- Abstract ADK Go behind a Provider interface from day one. The interface should be: `Stream(ctx, messages, tools) -> TokenStream` and `Call(ctx, messages, tools) -> Response`. Nothing ADK-specific leaks beyond this boundary.
- Pin to a specific ADK Go version and test against it. Do not auto-update.
- Have a fallback plan: the Provider interface should allow dropping in a raw Gemini REST client, the Anthropic Go SDK, or `sashabaranov/go-openai` if ADK Go proves too immature.
- Monitor the ADK Go GitHub repo for breaking changes and release cadence.

**Warning signs:**
- ADK Go requiring workarounds for basic features (custom headers, timeout control, streaming cancellation)
- Breaking changes in minor version bumps
- Agent loop features that don't map to Cascade's permission model
- Difficulty implementing tool-call interception (for permission checks) within ADK Go's execution model

**Phase to address:**
Phase 1 (LLM Provider abstraction). The Provider interface must be designed and implemented before any ADK Go integration, so ADK Go is pluggable rather than foundational.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcoding Gemini-specific function calling format | Faster initial development | Every new provider needs format translation; bugs when formats diverge | Never -- define a canonical tool call format in Phase 1 |
| Storing schema cache as flat JSON files | No SQLite dependency to set up | No FTS, no incremental updates, file locking issues, cache invalidation nightmare | Never -- SQLite/FTS5 from the start |
| Inlining SQL strings for INFORMATION_SCHEMA queries | Quick to write | Unmaintainable, untestable, version-dependent on BigQuery schema changes | Only in prototyping; extract to templates before Phase 2 ends |
| Using `fmt.Sprintf` for BigQuery SQL construction | Fast, obvious | SQL injection via LLM-generated values passed to templates | Never -- use parameterized queries for any user/LLM-sourced values |
| Global mutable state for current GCP project/dataset | Simple context passing | Race conditions with subagents, impossible to test, hard to add multi-project support | Only in Phase 1 prototype; refactor to injected config by Phase 2 |
| Skipping dry-run cost estimation in development | Faster iteration | Developers hit real costs; muscle memory of skipping transfers to production code | Never -- dry-run is cheap and should always run |
| Single-file `main.go` agent loop | Rapid iteration | Untestable, unmockable, impossible to add features without merge conflicts | Phase 1 only; decompose into packages before Phase 2 |
| Dumping full query results into LLM context | LLM sees everything | Context exhaustion, slow responses, high token costs | Never -- always truncate and summarize |

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| BigQuery | Using legacy SQL dialect by default | Always set `UseLegacySQL: false` explicitly in query config. Legacy SQL is still the default in some client library versions. |
| BigQuery | Not setting query timeout | Set `JobTimeout` and `QueryTimeout` separately. A query can be queued (job timeout) vs executing (query timeout). Default: 30s job creation, 120s execution. |
| BigQuery | Trusting dry-run cost as exact | Dry-run returns upper bound bytes processed. Actual cost can be lower due to caching, partition pruning at execution time. Communicate as "up to X" not "will cost X". |
| Cloud Composer | Assuming Airflow REST API is always available | Composer 2 exposes the Airflow REST API, but it requires the environment to be running. Stopped/updating environments return 503. Check environment status first. |
| Cloud Composer | Using the Airflow webserver URL directly | Use the Composer API to get the Airflow web UI URL and the DAG GCS bucket. These change between environment recreations. |
| Cloud Logging | Unbounded log queries | Cloud Logging queries without time bounds scan all retained logs (default 30 days). Always add `timestamp >= "time"` filter. Cost and latency scale linearly with time range. |
| Cloud Logging | Not handling pagination | Log queries return paginated results. A query for "all errors in the last hour" might return 10K entries across many pages. Set a reasonable limit (100-500) and offer "load more". |
| GCS | Reading large files into memory | `gsutil cat` on a 5GB Parquet file will OOM. Always check object size first (`storage.ObjectAttrs`), stream reads, and set a size limit (e.g., 10MB for `head` operations). |
| dbt | Assuming manifest.json is always current | `manifest.json` is generated by `dbt compile` or `dbt run`. If the user hasn't run dbt recently, the manifest is stale. Check modification time and warn. |
| dbt | Parsing manifest without version checking | dbt manifest format changes between versions (v7, v8, v9, etc.). Check `metadata.dbt_schema_version` before parsing. |
| ADC Auth | Assuming `GOOGLE_APPLICATION_CREDENTIALS` is set | ADC works without this env var (uses `gcloud auth application-default login` credentials). Check both paths. Common in containers vs local dev. |
| Service Account Impersonation | Not checking `iam.serviceAccountTokenCreator` role | Impersonation requires the caller to have this role on the target SA. Fail with a clear message, not a generic 403. |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full schema injection into every LLM call | Slow responses, high token costs | Use FTS5 to inject only relevant tables (10-20 max per call) | 100+ tables in schema cache |
| Synchronous schema cache refresh on startup | Startup takes 30s+ for large warehouses | Build cache asynchronously; use stale cache immediately, refresh in background | 500+ tables across configured datasets |
| Unbatched INFORMATION_SCHEMA queries (one per dataset) | N sequential queries for N datasets | Use concurrent goroutines with rate limiting (5 concurrent max) | 10+ datasets configured |
| Re-parsing dbt manifest on every dbt-related command | Noticeable delay (1-2s) for large projects | Parse once at startup/on-change, hold in memory. Watch `manifest.json` mtime. | 500+ dbt models |
| Storing full query results in conversation history | Context window exhaustion, slow compaction | Store only: query SQL, row count, first 10 rows, summary stats | Any query returning 100+ rows |
| Rendering full markdown in TUI on every keystroke | TUI lag, dropped frames | Debounce markdown rendering to 100ms minimum. Cache rendered output until content changes. | Long LLM responses (1000+ tokens), complex markdown with tables/code blocks |
| Creating new GCP client instances per tool call | Connection overhead, auth token churn | Create clients once at startup, reuse via dependency injection | Any session with 20+ tool calls |
| SQLite FTS5 `MATCH` with very common terms | Slow queries (100ms+) on large indexes | Use prefix queries and column-scoped FTS. Exclude common words (id, name, type) from FTS index or use custom tokenizer. | 50K+ columns indexed |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Passing LLM-generated SQL directly to BigQuery without any sanitization | LLM could generate DDL/DML even when user expects read-only query; prompt injection via data values | Parse SQL with a Go SQL parser before execution; reject DDL/DML in read-only mode; use BigQuery `dryRun` to check query type before execution |
| Logging full query results that may contain PII | Sensitive data persisted to disk in session logs or SQLite | Never log query result data to files. Log only: query SQL, row count, column names. PII detection should flag before display, not after logging. |
| Storing GCP credentials or tokens in config files | Credential theft if dotfiles are committed to git or shared | Never store credentials. Use ADC exclusively. Add `.cascade/` to `.gitignore` template. Warn if `config.toml` contains anything resembling a key or token. |
| Service account impersonation without scope restriction | Impersonated SA may have broader permissions than intended | Always request minimum scopes. Limit to BigQuery, Storage, Logging, Composer scopes only. |
| Executing bash commands from LLM without path restriction | LLM could `rm -rf`, access secrets, exfiltrate data | Bash tool must have: working directory restriction, blocked command list (`rm -rf /`, `curl` to external hosts, credential file reads), timeout, output size limit |
| Trusting LLM-suggested GCS paths without validation | Path traversal: `gs://bucket/../other-bucket/sensitive` | Validate GCS paths against allowed bucket list from config. Reject paths with `..` or paths outside configured buckets. |
| Session history containing sensitive data accessible to future sessions | Cross-session data leakage | Sessions are isolated. Old session data is not injected into new sessions. Session files encrypted at rest or stored in memory only. |
| Cost budget bypass via many small queries | 1000 queries at $0.01 each = $10, bypassing per-query $5 limit | Track cumulative session cost. Alert at configurable threshold. Implement per-session budget, not just per-query. |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing raw BigQuery job errors | Users see `invalidQuery: Syntax error at [1:45]` with no context about what the LLM tried | Show the generated SQL with syntax highlighting, the error with the problematic location highlighted, and let the LLM auto-retry with the error context |
| No progress indication during long operations | User thinks the tool crashed during schema cache build or complex queries | Show a spinner with context: "Building schema cache... dataset_1 (234 tables)" or "Running query... (estimated 2.3s, processing 450MB)" |
| Displaying thousands of rows in the terminal | Terminal hangs, scrollback destroyed | Default to 25 rows with pagination. Show row count, offer `/more` command. Offer CSV/JSON export for full results. |
| Making first-run setup mandatory and blocking | User just wants to try the tool, gets a 2-minute setup wizard | Setup wizard should be skippable. Auto-detect what's available (project, datasets, Composer) and work with what's found. Cache builds in background. |
| Dense information in query cost warnings | User ignores cost warnings because they're a wall of text | Simple traffic light: green (< $0.01), yellow ($0.01-$1.00), red (> $1.00) with one-line summary. Detail available via expand. |
| Not showing what the LLM is doing | User sees blank screen for 5-10 seconds while LLM reasons | Stream thinking/reasoning tokens. Show "Analyzing pipeline..." or tool call annotations in real-time. |
| Forcing keyboard-only interaction | Power users love it; new users are lost | Support both slash commands AND natural language for common operations. `/failures` and "show me recent failures" should both work. |
| Inconsistent date/time handling | "Show failures from yesterday" interpreted differently depending on timezone | Always display timestamps in the user's local timezone. Use relative time for recent events ("2 hours ago"). Let config set timezone explicitly. |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **BigQuery tool:** Often missing handling of `REPEATED` (array) and `RECORD` (struct) column types in schema cache -- these need nested representation, not flat columns
- [ ] **Schema cache:** Often missing view definitions and materialized view refresh schedules -- users ask "what does this view do?" and the cache has no answer
- [ ] **Cost estimation:** Often missing slot-based pricing calculation -- dry-run returns bytes, but some orgs use flat-rate/editions pricing where bytes don't map to dollars
- [ ] **Composer integration:** Often missing support for Composer 1 vs Composer 2 API differences -- the Airflow REST API endpoint format differs significantly
- [ ] **dbt integration:** Often missing handling of dbt packages (dependencies) -- manifest includes package models that aren't "yours" but appear in lineage
- [ ] **Permission model:** Often missing handling of tool chains -- a "safe" read tool that triggers an "unsafe" write tool via LLM reasoning isn't caught by per-tool classification
- [ ] **Session management:** Often missing graceful recovery from crash -- if Cascade is killed mid-session, the next startup should offer to resume or show what was lost
- [ ] **Config file:** Often missing validation of GCP project/dataset existence at config load time -- user typos a dataset name, gets cryptic errors 5 minutes later
- [ ] **Streaming output:** Often missing handling of LLM provider rate limits (429) -- user hits rate limit mid-stream, sees partial output with no explanation
- [ ] **Error retry:** Often missing exponential backoff -- retrying immediately on transient GCP errors (503, quota exceeded) makes the problem worse

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Streaming deadlock | MEDIUM | Requires rearchitecting the message/render pipeline. Cannot patch -- must redesign the token accumulation and render tick system. Estimate: 2-3 days. |
| Tool call parse failures | LOW | Add lenient parsing as a middleware layer. Does not require agent loop changes. Can be done incrementally per failure mode observed. |
| INFORMATION_SCHEMA over-scanning | LOW | Change query scope from project-level to dataset-level. One-line fix per query, but must audit all INFORMATION_SCHEMA queries. |
| Auth token expiry crashes | LOW | Add retry middleware around GCP client calls. 1-2 days of work if the GCP client is properly abstracted. |
| Context window exhaustion | HIGH | Requires redesigning schema injection, adding truncation to all tool results, implementing compaction. Touches every tool. Estimate: 1-2 weeks. |
| Cost gate bypass | MEDIUM | Add partition metadata to schema cache (schema change), update system prompt with cost-aware instructions, add SQL rewriting layer. Estimate: 3-5 days. |
| SQLite FTS5 corruption | MEDIUM | Implement automatic rebuild from base tables. Add integrity check on startup. Requires schema cache to have non-FTS base tables as source of truth. |
| Permission fatigue | MEDIUM | Redesign permission UX: auto-approve reads, add trust escalation. Requires changes to permission engine and TUI confirmation flow. |
| Agent loop cycling | LOW | Add loop counter and governor. System prompt changes + 1 day of agent loop code. |
| ADK Go immaturity | HIGH | If ADK Go is deeply embedded, switching providers requires rewriting the LLM integration layer. With proper Provider interface, it's a 2-3 day swap. Without it, 2-3 weeks. |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Streaming deadlock | Phase 1: Core TUI + Agent Loop | Stress test with 5000-token streaming response while rapidly pressing keys. No dropped frames, Ctrl+C responds within 500ms. |
| Tool call parse failures | Phase 1: Agent Loop + Tool Dispatch | Fuzz test tool calls with malformed JSON, wrong types, missing fields. Recovery rate > 95%. |
| INFORMATION_SCHEMA over-scanning | Phase 2: Schema Cache | Verify every INFORMATION_SCHEMA query uses dataset-scoped view. Check bytes_processed in test against known dataset. |
| Auth token expiry | Phase 1: GCP Auth Layer | Integration test with 70-minute session (or mock token with 60s expiry). All API calls succeed after refresh. |
| Context window exhaustion | Phase 2-3: Schema Injection + Compaction | Run 30-turn conversation with 500-table warehouse. Token usage stays below 60% of context window. Response quality doesn't degrade. |
| Cost gate bypass | Phase 2: BigQuery Tools + Cost Gate | Generate queries against partitioned tables. Verify partition filter guidance is provided on rejection. Rejection rate below 20% for common queries. |
| SQLite FTS5 corruption | Phase 2: Schema Cache | Concurrent read/write stress test. Kill process mid-write 10 times. FTS5 integrity check passes after each restart. |
| Permission fatigue | Phase 1: Permission Engine | UX test: complete a 10-step debugging workflow. Count confirmations. Should be 0 for read-only steps, 1-2 for write steps. |
| Agent loop cycling | Phase 1: Agent Loop | Test with intentionally ambiguous prompts ("fix it", "check everything"). Loop terminates within 15 tool calls. Agent asks for clarification. |
| ADK Go immaturity | Phase 1: Provider Interface | Implement two providers (Gemini via ADK Go + one other). Swap providers in config. Both pass the same integration test suite. |

## Sources

- Training data knowledge of Bubble Tea architecture (Elm architecture pattern, message-passing model, common issues in charmbracelet/bubbletea GitHub issues and discussions)
- Training data knowledge of BigQuery INFORMATION_SCHEMA scoping (project-level vs dataset-level views, bytes_processed behavior, partition metadata tables)
- Training data knowledge of GCP Application Default Credentials lifecycle (token refresh, service account impersonation chain, `google.DefaultTokenSource` behavior)
- Training data knowledge of SQLite WAL mode concurrency and FTS5 index integrity (modernc.org/sqlite behavior vs CGo sqlite3)
- Training data knowledge of LLM agent loop patterns from Claude Code, OpenCode, Aider, and similar tools (tool call failure modes, context window management, loop governors)
- Training data knowledge of Go concurrency patterns (goroutine lifecycle, channel buffering, context cancellation)
- BigQuery pricing documentation (on-demand vs flat-rate, dry-run semantics)
- Cloud Composer API documentation (Composer 1 vs 2 differences, Airflow REST API availability)

**Confidence note:** All findings are based on training data (cutoff ~May 2025). Google ADK Go findings are MEDIUM confidence as the framework was very new at training cutoff and may have matured significantly. All other findings are MEDIUM-HIGH confidence as they cover well-established technologies with known failure modes.

---
*Pitfalls research for: AI-native GCP data engineering terminal agent*
*Researched: 2026-03-16*

# Pitfalls Research

**Domain:** AI-native GCP data engineering terminal agent (Go + Bubble Tea + LLM + BigQuery/GCS/Composer)
**Researched:** 2026-03-16
**Confidence:** MEDIUM (based on training data across Go, Bubble Tea, BigQuery, GCP, and LLM agent architectures; no live verification available)

## Critical Pitfalls

### Pitfall 1: Bubble Tea Streaming Deadlock — Blocking the Event Loop with LLM Output

**What goes wrong:**
Bubble Tea runs a single-threaded event loop via its `Update` function. LLM streaming responses generate tokens continuously for seconds or minutes. If token delivery blocks the Bubble Tea event loop — or if too many `Cmd` messages pile up from a streaming goroutine — the TUI freezes, drops keystrokes, or deadlocks entirely. The user presses Ctrl+C and nothing happens. This is the single most common failure mode in Bubble Tea apps that integrate with streaming APIs.

**Why it happens:**
Developers treat Bubble Tea like a web framework where you can just push updates. But Bubble Tea is an Elm-architecture: `Update` returns a `(Model, Cmd)` and the runtime processes commands sequentially. If your streaming goroutine sends messages via a channel that fills up (because `Update` is slow processing a large render), the goroutine blocks, which can cascade into the LLM client blocking on its HTTP read, which holds a connection open indefinitely. Conversely, batching too aggressively causes visible stutter.

**How to avoid:**
- Use a dedicated goroutine for LLM streaming that writes to a ring buffer or channel with a generous buffer size (100+), never blocking on send.
- In `Update`, drain the channel non-blockingly per tick. Batch multiple tokens into a single render cycle using `tea.Tick` at 16-33ms intervals (30-60fps), not per-token messages.
- Separate the "token arrived" message from the "render viewport" cycle. Accumulate tokens in the model, re-render on a tick.
- Implement a cancellation channel that the streaming goroutine checks, so Ctrl+C can abort mid-stream.
- Never call `p.Send()` from a goroutine — always return messages via `tea.Cmd` or use `p.Send()` only from a `tea.Cmd`-spawned goroutine that properly handles backpressure.

**Warning signs:**
- TUI freezes for 1-2 seconds during long LLM responses
- Ctrl+C takes noticeable time to respond during streaming
- Token rendering appears "bursty" — nothing, then a wall of text
- High CPU usage during streaming with no visible output

**Phase to address:**
Phase 1 (Core Agent Loop + TUI). This is foundational. If the streaming/rendering pipeline is wrong from the start, retrofitting is a near-rewrite of the TUI layer.

---

### Pitfall 2: LLM Tool Call Parse Failures Treated as Unrecoverable Errors

**What goes wrong:**
LLMs generate malformed tool calls — wrong JSON, hallucinated parameter names, invalid argument types, partially-streamed JSON that fails to parse. If the agent loop treats any tool call parse failure as a hard error (crash, abort, or confused state), the tool becomes unreliable. Users see "error: invalid tool call" 10-20% of the time and lose trust.

**Why it happens:**
Developers test with clean examples and assume LLM output is well-formed. In production, models produce: (1) JSON with trailing commas, (2) tool names that are close-but-wrong (e.g., `bigquery_query` vs `BigQueryQuery`), (3) arguments that are strings when numbers are expected, (4) multiple tool calls when one was expected, (5) tool calls embedded in natural language rather than proper function_call format. The failure rate increases with complex schemas and less-capable models (Ollama local models being worst).

**How to avoid:**
- Implement lenient JSON parsing: strip markdown code fences, handle trailing commas, try `json.Unmarshal` then fall back to regex extraction.
- Fuzzy-match tool names (Levenshtein distance or prefix match) before rejecting.
- Coerce argument types where safe (string "42" to int 42, "true" to bool).
- On parse failure, send the error back to the LLM as a tool result with a clear error message and let it retry. Budget 2-3 retries before surfacing to user.
- Validate tool call schemas server-side before execution — never trust the LLM's output structure.
- Log all malformed tool calls for analysis (what model, what prompt, what went wrong).

**Warning signs:**
- Error rates above 5% for tool calls across any model provider
- Users report "it keeps saying it will do X but then errors"
- Different models have wildly different success rates
- Tool calls work in testing but fail with real user prompts

**Phase to address:**
Phase 1 (Core Agent Loop). The tool dispatch mechanism must be resilient from day one. This is the inner loop of the entire product.

---

### Pitfall 3: INFORMATION_SCHEMA Queries Scanning Entire Organization Instead of Target Project/Dataset

**What goes wrong:**
BigQuery's INFORMATION_SCHEMA has region-scoped and project-scoped views. Querying `region-us.INFORMATION_SCHEMA.COLUMNS` without dataset qualification scans across ALL datasets in the project. For large enterprises with 500+ datasets and 50K+ tables, this query takes 30-60 seconds, processes gigabytes, and costs real money. The schema cache build becomes a blocking, expensive operation that runs on first startup.

**Why it happens:**
The INFORMATION_SCHEMA documentation is confusing about scoping. There are dataset-level views (`dataset.INFORMATION_SCHEMA.COLUMNS` -- scoped to one dataset), project-level views (`region-us.INFORMATION_SCHEMA.COLUMNS` -- all datasets in project), and organization-level views. Developers often start with the project-level view because it seems like the "get everything" approach. But the project context already specifies dataset-scoped caching -- the pitfall is accidentally using project-scoped views "for convenience" during implementation, or during the initial setup wizard scan.

**How to avoid:**
- Always use dataset-scoped INFORMATION_SCHEMA: `project.dataset.INFORMATION_SCHEMA.COLUMNS`. Never use the region-scoped view for cache building.
- In the setup wizard, enumerate datasets first (`INFORMATION_SCHEMA.SCHEMATA`), let the user select which to cache (default: all, but with a count/cost preview).
- Implement per-dataset cache building with progress reporting. If a dataset has 10K+ tables, warn and offer to skip.
- Set a query timeout (30s) on cache-building queries. If a dataset is too large, fall back to sampling or API-based metadata.
- Track bytes_processed from dry-run before executing cache queries.

**Warning signs:**
- Schema cache build takes more than 10 seconds
- Users report high BigQuery costs from Cascade itself
- First-run wizard hangs or appears frozen
- Cache queries appear in BigQuery audit logs scanning TB of metadata

**Phase to address:**
Phase 2 (Schema Cache + BigQuery tools). Must be correct at the point schema caching is implemented. Get the query scoping right from the first implementation.

---

### Pitfall 4: GCP Auth Token Expiry Crashing Mid-Session

**What goes wrong:**
Application Default Credentials (ADC) tokens expire after 1 hour (3600 seconds). If the token refresh fails silently or the HTTP client doesn't handle 401s with automatic retry, every GCP API call starts failing mid-session. The user is 45 minutes into a debugging session, asks "what failed in Composer last night?", and gets a cryptic auth error. Worse: some GCP client libraries cache the token and don't refresh, while others refresh transparently -- behavior varies by library.

**Why it happens:**
During development and testing, sessions are short. The 1-hour token expiry is never hit. In production, data engineers sit in Cascade for hours. The Go GCP client libraries (`cloud.google.com/go`) handle token refresh automatically IF you use the standard `google.DefaultClient()` flow. But if you've created a custom HTTP client, wrapped the transport, or are using service account impersonation with a two-hop token chain, the refresh logic may not trigger correctly. Service account impersonation is particularly tricky: the impersonated token has its own expiry independent of the base credential.

**How to avoid:**
- Use `google.DefaultClient()` or `google.DefaultTokenSource()` and let the standard library handle refresh. Do not cache tokens manually.
- For service account impersonation, use `google.golang.org/api/impersonate` package which handles the token chain refresh.
- Wrap all GCP API calls with a retry-on-401 middleware that forces a token refresh and retries once.
- Test with artificially short token lifetimes or mock token expiry scenarios.
- Display a subtle "re-authenticating..." indicator when a token refresh occurs rather than letting it be invisible.
- On auth failure after retry, provide actionable error: "Run `gcloud auth application-default login` to refresh credentials."

**Warning signs:**
- Any GCP call failing with 401/403 after the session has been open for 45+ minutes
- Tests pass but real usage fails after extended periods
- Service account impersonation works initially but breaks later
- Inconsistent auth failures (works sometimes, fails others -- race condition in refresh)

**Phase to address:**
Phase 1 (GCP Auth foundation). Auth must be robust from the start since every subsequent tool depends on it. Test with long-running sessions in Phase 1.

---

### Pitfall 5: Context Window Exhaustion from Schema + Conversation History

**What goes wrong:**
The agent injects cached schema into the system prompt for SQL generation context. A warehouse with 500 tables, each with 20 columns, produces ~200KB of schema text. Combined with conversation history, tool call results (especially query outputs with many rows), and the platform summary -- the context window fills up within 5-10 turns. The LLM starts hallucinating table names, forgetting earlier conversation, or hitting API token limits causing hard failures.

**Why it happens:**
Schema injection is greedy by default -- "give the LLM everything it might need." Early testing uses small schemas (5-10 tables) where this works perfectly. With real warehouses, the schema alone can consume 30-50% of the context window. Add in a few query results with 100+ rows, some error logs from Composer, and the conversation is at 80% capacity before the user's actual question.

**How to avoid:**
- Never inject full schema. Use the FTS5 index to inject only relevant tables/columns based on the user's current query or topic. 10-20 relevant tables, not 500.
- Implement aggressive context compaction: summarize old conversation turns, drop raw query results after they've been discussed, replace verbose error logs with summaries.
- Set hard limits on tool result sizes: truncate query results to 50 rows with a "showing 50 of 1,234 rows" indicator. Summarize log output.
- Track token usage per message and warn when approaching 70% of context window. Auto-compact at 80%.
- Use Gemini's 2M context as a safety net, not a crutch. Even with 2M tokens, irrelevant context degrades response quality ("lost in the middle" problem).
- Consider a two-pass approach: first pass identifies relevant tables (cheap, small context), second pass generates SQL with only relevant schema.

**Warning signs:**
- LLM references tables that don't exist (hallucinated from partial schema)
- Response quality degrades noticeably after 10+ turns
- API calls failing with token limit errors
- SQL generation works for simple queries but fails when the user has been in a long session

**Phase to address:**
Phase 2 (Schema Cache) for schema injection strategy, Phase 3 (Context Management/Compaction) for conversation history management. Both must be designed together even if implemented in different phases.

---

### Pitfall 6: Cost Gate Bypass Through LLM-Generated SQL Variations

**What goes wrong:**
The cost estimation system does a dry-run of the SQL the LLM generates. But the LLM can generate semantically equivalent queries with vastly different costs. It might use `SELECT *` instead of selecting specific columns. It might scan a non-partitioned table when a partitioned equivalent exists. It might use a subquery that forces a full table scan when a `WHERE` clause on the partition key would limit it. The dry-run catches the cost correctly, but the LLM keeps generating expensive queries that get rejected, frustrating the user.

**Why it happens:**
LLMs optimize for correctness, not cost. Without explicit instruction about partitioning, clustering, and column selection, they write the simplest correct query. The schema context tells them what tables exist but not which are partitioned, what the partition key is, or what the clustering columns are. The cost gate then rejects the query, but the LLM doesn't know WHY it was rejected or how to write a cheaper version.

**How to avoid:**
- Include partition and clustering metadata in schema context. For each table, inject: partition column, partition type (day/month/hour), clustering columns, approximate row count, approximate size.
- When a query exceeds the cost budget, feed back specific guidance: "Query scans 2.3TB. Table `events` is partitioned by `event_date`. Add a `WHERE event_date >= '2026-03-01'` clause to reduce scan."
- Implement a SQL rewriter that automatically adds partition filters when they're missing (before dry-run, suggest to user).
- Default to `SELECT column1, column2` prompting rather than `SELECT *` in the system prompt.
- Set per-query cost limits AND per-session cumulative limits.

**Warning signs:**
- Dry-run rejections exceeding 30% of generated queries
- Users complaining "it keeps trying to run expensive queries"
- LLM entering a loop of generating rejected queries
- Cost estimates showing TB-scale scans for simple questions

**Phase to address:**
Phase 2 (BigQuery tools + cost estimation). The schema cache must store partition/clustering metadata from INFORMATION_SCHEMA.TABLE_OPTIONS and COLUMNS, and the cost gate must provide actionable feedback.

---

### Pitfall 7: SQLite FTS5 Index Corruption Under Concurrent Access from Subagents

**What goes wrong:**
The schema cache uses SQLite with FTS5 for full-text search. Subagents are fire-and-forget goroutines. If the main agent loop and a subagent (e.g., background schema refresh, background cost analysis querying cached schema) both write to or read from SQLite simultaneously, and the connection isn't properly configured for concurrent access, you get `SQLITE_BUSY` errors, corrupted FTS5 indexes, or silent data loss. `modernc.org/sqlite` (pure Go SQLite) has different concurrency characteristics than CGo SQLite.

**Why it happens:**
SQLite's concurrency model is "multiple readers, single writer" with WAL mode, but this requires correct configuration. `modernc.org/sqlite` supports this but defaults may differ. Developers open multiple `*sql.DB` connections or share one without proper mutex protection. FTS5 indexes are particularly fragile during writes -- a failed write mid-FTS-update can leave the index inconsistent, causing future searches to return wrong results or panic.

**How to avoid:**
- Use a single `*sql.DB` instance for all SQLite access, with `_journal_mode=WAL&_busy_timeout=5000` pragmas set at connection open.
- Set `SetMaxOpenConns(1)` for write operations. Use a separate read-only connection pool for concurrent reads if needed.
- Wrap all FTS5 write operations (index rebuild, incremental update) in explicit transactions.
- Never write to the FTS5 index from a subagent goroutine. Schema cache writes should go through a serialized channel to a single writer goroutine.
- Implement FTS5 index integrity checks (`INSERT INTO fts_table(fts_table) VALUES('integrity-check')`) on startup.
- If FTS5 index is corrupted, rebuild it automatically from the base tables rather than crashing.

**Warning signs:**
- Intermittent `SQLITE_BUSY` errors in logs
- Schema search returning stale or missing results
- FTS5 queries that worked before suddenly returning empty
- Subagent goroutines logging database errors silently

**Phase to address:**
Phase 2 (Schema Cache with SQLite/FTS5). Design the concurrency model for the schema cache from the outset. Phase 3 (Subagents) must respect the established access patterns.

---

### Pitfall 8: Permission Model That Blocks Legitimate Workflows

**What goes wrong:**
The 5-level permission model (READ_ONLY through ADMIN) with CONFIRM-by-default seems safe. In practice, it produces "permission fatigue" -- the user confirms 15 read-only operations in a row just to debug a pipeline, gets frustrated, switches to BYPASS mode, and then runs unguarded when a genuinely dangerous operation comes through. The permission system either annoys users into disabling it or fails to protect when it matters.

**Why it happens:**
Permission classification is designed around operation type, not user intent. A debugging workflow might involve: list DAGs (read), get DAG status (read), get task logs (read), query BigQuery logs (read), read GCS file (read), query INFORMATION_SCHEMA (read). Each triggers a confirmation. By the time the user finds the issue and wants to re-run a DAG task (write), they've already mentally checked out of the confirmation flow.

**How to avoid:**
- Auto-approve READ_ONLY operations in CONFIRM mode. Only prompt for WRITE and above.
- Implement "trust escalation" within a session: after confirming 3 operations in the same risk category, auto-approve the rest of that category for the session.
- Group related operations: "I'm going to run 3 read queries to investigate -- approve all?" rather than one-by-one.
- Show a clear risk indicator in the TUI (green/yellow/red) rather than a blocking modal for low-risk operations.
- The PLAN mode (read-only) should be truly non-blocking -- no confirmations needed.
- Reserve blocking confirmation dialogs for DML, DDL, DESTRUCTIVE, and ADMIN only.

**Warning signs:**
- Users immediately switching to BYPASS mode during demos or first use
- Feedback like "too many prompts" or "I just want to look at my data"
- Session logs showing 20+ confirmations with 0 rejections
- Users avoiding tools that trigger confirmations

**Phase to address:**
Phase 1 (Permission Engine). Design with read-auto-approve from the start. Phase 3 (Polish) for trust escalation and grouping.

---

### Pitfall 9: Agent Loop Infinite Cycling on Ambiguous User Requests

**What goes wrong:**
The user says "fix the pipeline." The LLM calls a tool to check DAG status. Gets results. Calls another tool to check logs. Gets results. Calls another tool to check the BigQuery table. Then calls DAG status again because it "wants more information." This cycles 10-20 times, burning tokens and time, without ever producing an answer or asking the user for clarification. The agent appears busy but accomplishes nothing.

**Why it happens:**
The agent loop is observe-reason-select-execute with no governor. The LLM's reasoning step says "I need more information" repeatedly because the request is genuinely ambiguous (which pipeline? which failure? what timeframe?). Without explicit loop limits, escalation triggers, or a "stop and ask" heuristic, the loop continues indefinitely. This is worse with weaker models that have less self-awareness about when to stop.

**How to avoid:**
- Hard limit of 15-20 tool calls per turn (configurable). After the limit, force the agent to summarize findings and respond.
- Implement a "diminishing returns" detector: if the last 3 tool calls returned similar information or the same tool is called twice with the same arguments, stop and synthesize.
- Add explicit "ask for clarification" as a tool. Teach the LLM (via system prompt) to use it when the request is ambiguous.
- Track tool call history within a turn and include it in the LLM context so it can see what it's already done.
- Implement cost tracking per turn -- if a single turn has consumed $X or N tokens, warn and suggest compaction.
- After 5 tool calls with no user-facing output, inject a nudge: "You've made 5 tool calls. Provide a progress update or ask the user for clarification."

**Warning signs:**
- Average tool calls per turn exceeding 8-10
- Same tool being called multiple times with identical or near-identical arguments
- Users reporting "it's thinking for a long time"
- Token costs per conversation turn higher than expected

**Phase to address:**
Phase 1 (Core Agent Loop). The loop governor must be built into the agent loop from the start. Retrofitting it is surprisingly hard because the system prompt and tool design assume unlimited iteration.

---

### Pitfall 10: Treating Google ADK Go as Production-Stable When It Is Early-Stage

**What goes wrong:**
Google ADK Go was announced in early 2025 and is under active development. The API surface changes between versions. Documentation is sparse compared to the Python ADK. Features that exist in the Python ADK (structured output, automatic function calling, agent-to-agent communication) may not be available or may work differently in Go. Building the entire agent loop on ADK Go means absorbing its instability into Cascade's core.

**Why it happens:**
The ADK Go is appealing because it provides native GCP auth, a built-in agent loop, and Vertex AI integration. But "native agent loop" may not match Cascade's requirements (custom tool dispatch, streaming to Bubble Tea, permission checks before tool execution). The developer ends up fighting the framework rather than being helped by it, or discovers a missing feature 3 months into development.

**How to avoid:**
- Use ADK Go as a LLM client library (model calls, streaming, function declarations) but NOT as the agent loop orchestrator. Build Cascade's agent loop independently.
- Abstract ADK Go behind a Provider interface from day one. The interface should be: `Stream(ctx, messages, tools) -> TokenStream` and `Call(ctx, messages, tools) -> Response`. Nothing ADK-specific leaks beyond this boundary.
- Pin to a specific ADK Go version and test against it. Do not auto-update.
- Have a fallback plan: the Provider interface should allow dropping in a raw Gemini REST client, the Anthropic Go SDK, or `sashabaranov/go-openai` if ADK Go proves too immature.
- Monitor the ADK Go GitHub repo for breaking changes and release cadence.

**Warning signs:**
- ADK Go requiring workarounds for basic features (custom headers, timeout control, streaming cancellation)
- Breaking changes in minor version bumps
- Agent loop features that don't map to Cascade's permission model
- Difficulty implementing tool-call interception (for permission checks) within ADK Go's execution model

**Phase to address:**
Phase 1 (LLM Provider abstraction). The Provider interface must be designed and implemented before any ADK Go integration, so ADK Go is pluggable rather than foundational.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcoding Gemini-specific function calling format | Faster initial development | Every new provider needs format translation; bugs when formats diverge | Never -- define a canonical tool call format in Phase 1 |
| Storing schema cache as flat JSON files | No SQLite dependency to set up | No FTS, no incremental updates, file locking issues, cache invalidation nightmare | Never -- SQLite/FTS5 from the start |
| Inlining SQL strings for INFORMATION_SCHEMA queries | Quick to write | Unmaintainable, untestable, version-dependent on BigQuery schema changes | Only in prototyping; extract to templates before Phase 2 ends |
| Using `fmt.Sprintf` for BigQuery SQL construction | Fast, obvious | SQL injection via LLM-generated values passed to templates | Never -- use parameterized queries for any user/LLM-sourced values |
| Global mutable state for current GCP project/dataset | Simple context passing | Race conditions with subagents, impossible to test, hard to add multi-project support | Only in Phase 1 prototype; refactor to injected config by Phase 2 |
| Skipping dry-run cost estimation in development | Faster iteration | Developers hit real costs; muscle memory of skipping transfers to production code | Never -- dry-run is cheap and should always run |
| Single-file `main.go` agent loop | Rapid iteration | Untestable, unmockable, impossible to add features without merge conflicts | Phase 1 only; decompose into packages before Phase 2 |
| Dumping full query results into LLM context | LLM sees everything | Context exhaustion, slow responses, high token costs | Never -- always truncate and summarize |

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| BigQuery | Using legacy SQL dialect by default | Always set `UseLegacySQL: false` explicitly in query config. Legacy SQL is still the default in some client library versions. |
| BigQuery | Not setting query timeout | Set `JobTimeout` and `QueryTimeout` separately. A query can be queued (job timeout) vs executing (query timeout). Default: 30s job creation, 120s execution. |
| BigQuery | Trusting dry-run cost as exact | Dry-run returns upper bound bytes processed. Actual cost can be lower due to caching, partition pruning at execution time. Communicate as "up to X" not "will cost X". |
| Cloud Composer | Assuming Airflow REST API is always available | Composer 2 exposes the Airflow REST API, but it requires the environment to be running. Stopped/updating environments return 503. Check environment status first. |
| Cloud Composer | Using the Airflow webserver URL directly | Use the Composer API to get the Airflow web UI URL and the DAG GCS bucket. These change between environment recreations. |
| Cloud Logging | Unbounded log queries | Cloud Logging queries without time bounds scan all retained logs (default 30 days). Always add `timestamp >= "time"` filter. Cost and latency scale linearly with time range. |
| Cloud Logging | Not handling pagination | Log queries return paginated results. A query for "all errors in the last hour" might return 10K entries across many pages. Set a reasonable limit (100-500) and offer "load more". |
| GCS | Reading large files into memory | Calling read on a 5GB Parquet file will OOM. Always check object size first (`storage.ObjectAttrs`), stream reads, and set a size limit (e.g., 10MB for `head` operations). |
| dbt | Assuming manifest.json is always current | `manifest.json` is generated by `dbt compile` or `dbt run`. If the user hasn't run dbt recently, the manifest is stale. Check modification time and warn. |
| dbt | Parsing manifest without version checking | dbt manifest format changes between versions (v7, v8, v9, etc.). Check `metadata.dbt_schema_version` before parsing. |
| ADC Auth | Assuming `GOOGLE_APPLICATION_CREDENTIALS` is set | ADC works without this env var (uses `gcloud auth application-default login` credentials). Check both paths. Common in containers vs local dev. |
| Service Account Impersonation | Not checking `iam.serviceAccountTokenCreator` role | Impersonation requires the caller to have this role on the target SA. Fail with a clear message, not a generic 403. |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full schema injection into every LLM call | Slow responses, high token costs | Use FTS5 to inject only relevant tables (10-20 max per call) | 100+ tables in schema cache |
| Synchronous schema cache refresh on startup | Startup takes 30s+ for large warehouses | Build cache asynchronously; use stale cache immediately, refresh in background | 500+ tables across configured datasets |
| Unbatched INFORMATION_SCHEMA queries (one per dataset) | N sequential queries for N datasets | Use concurrent goroutines with rate limiting (5 concurrent max) | 10+ datasets configured |
| Re-parsing dbt manifest on every dbt-related command | Noticeable delay (1-2s) for large projects | Parse once at startup/on-change, hold in memory. Watch `manifest.json` mtime. | 500+ dbt models |
| Storing full query results in conversation history | Context window exhaustion, slow compaction | Store only: query SQL, row count, first 10 rows, summary stats | Any query returning 100+ rows |
| Rendering full markdown in TUI on every keystroke | TUI lag, dropped frames | Debounce markdown rendering to 100ms minimum. Cache rendered output until content changes. | Long LLM responses (1000+ tokens), complex markdown with tables/code blocks |
| Creating new GCP client instances per tool call | Connection overhead, auth token churn | Create clients once at startup, reuse via dependency injection | Any session with 20+ tool calls |
| SQLite FTS5 `MATCH` with very common terms | Slow queries (100ms+) on large indexes | Use prefix queries and column-scoped FTS. Exclude common words (id, name, type) from FTS index or use custom tokenizer. | 50K+ columns indexed |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Passing LLM-generated SQL directly to BigQuery without any sanitization | LLM could generate DDL/DML even when user expects read-only query; prompt injection via data values | Parse SQL with a Go SQL parser before execution; reject DDL/DML in read-only mode; use BigQuery `dryRun` to check query type before execution |
| Logging full query results that may contain PII | Sensitive data persisted to disk in session logs or SQLite | Never log query result data to files. Log only: query SQL, row count, column names. PII detection should flag before display, not after logging. |
| Storing GCP credentials or tokens in config files | Credential theft if dotfiles are committed to git or shared | Never store credentials. Use ADC exclusively. Add `.cascade/` to `.gitignore` template. Warn if `config.toml` contains anything resembling a key or token. |
| Service account impersonation without scope restriction | Impersonated SA may have broader permissions than intended | Always request minimum scopes. Limit to BigQuery, Storage, Logging, Composer scopes only. |
| Executing bash commands from LLM without path restriction | LLM could `rm -rf`, access secrets, exfiltrate data | Bash tool must have: working directory restriction, blocked command list (`rm -rf /`, `curl` to external hosts, credential file reads), timeout, output size limit |
| Trusting LLM-suggested GCS paths without validation | Path traversal: `gs://bucket/../other-bucket/sensitive` | Validate GCS paths against allowed bucket list from config. Reject paths with `..` or paths outside configured buckets. |
| Session history containing sensitive data accessible to future sessions | Cross-session data leakage | Sessions are isolated. Old session data is not injected into new sessions. Session files encrypted at rest or stored in memory only. |
| Cost budget bypass via many small queries | 1000 queries at $0.01 each = $10, bypassing per-query $5 limit | Track cumulative session cost. Alert at configurable threshold. Implement per-session budget, not just per-query. |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing raw BigQuery job errors | Users see `invalidQuery: Syntax error at [1:45]` with no context about what the LLM tried | Show the generated SQL with syntax highlighting, the error with the problematic location highlighted, and let the LLM auto-retry with the error context |
| No progress indication during long operations | User thinks the tool crashed during schema cache build or complex queries | Show a spinner with context: "Building schema cache... dataset_1 (234 tables)" or "Running query... (estimated 2.3s, processing 450MB)" |
| Displaying thousands of rows in the terminal | Terminal hangs, scrollback destroyed | Default to 25 rows with pagination. Show row count, offer `/more` command. Offer CSV/JSON export for full results. |
| Making first-run setup mandatory and blocking | User just wants to try the tool, gets a 2-minute setup wizard | Setup wizard should be skippable. Auto-detect what's available (project, datasets, Composer) and work with what's found. Cache builds in background. |
| Dense information in query cost warnings | User ignores cost warnings because they're a wall of text | Simple traffic light: green (< $0.01), yellow ($0.01-$1.00), red (> $1.00) with one-line summary. Detail available via expand. |
| Not showing what the LLM is doing | User sees blank screen for 5-10 seconds while LLM reasons | Stream thinking/reasoning tokens. Show "Analyzing pipeline..." or tool call annotations in real-time. |
| Forcing keyboard-only interaction | Power users love it; new users are lost | Support both slash commands AND natural language for common operations. `/failures` and "show me recent failures" should both work. |
| Inconsistent date/time handling | "Show failures from yesterday" interpreted differently depending on timezone | Always display timestamps in the user's local timezone. Use relative time for recent events ("2 hours ago"). Let config set timezone explicitly. |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **BigQuery tool:** Often missing handling of `REPEATED` (array) and `RECORD` (struct) column types in schema cache -- these need nested representation, not flat columns
- [ ] **Schema cache:** Often missing view definitions and materialized view refresh schedules -- users ask "what does this view do?" and the cache has no answer
- [ ] **Cost estimation:** Often missing slot-based pricing calculation -- dry-run returns bytes, but some orgs use flat-rate/editions pricing where bytes don't map to dollars
- [ ] **Composer integration:** Often missing support for Composer 1 vs Composer 2 API differences -- the Airflow REST API endpoint format differs significantly
- [ ] **dbt integration:** Often missing handling of dbt packages (dependencies) -- manifest includes package models that aren't "yours" but appear in lineage
- [ ] **Permission model:** Often missing handling of tool chains -- a "safe" read tool that triggers an "unsafe" write tool via LLM reasoning isn't caught by per-tool classification
- [ ] **Session management:** Often missing graceful recovery from crash -- if Cascade is killed mid-session, the next startup should offer to resume or show what was lost
- [ ] **Config file:** Often missing validation of GCP project/dataset existence at config load time -- user typos a dataset name, gets cryptic errors 5 minutes later
- [ ] **Streaming output:** Often missing handling of LLM provider rate limits (429) -- user hits rate limit mid-stream, sees partial output with no explanation
- [ ] **Error retry:** Often missing exponential backoff -- retrying immediately on transient GCP errors (503, quota exceeded) makes the problem worse

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Streaming deadlock | MEDIUM | Requires rearchitecting the message/render pipeline. Cannot patch -- must redesign the token accumulation and render tick system. Estimate: 2-3 days. |
| Tool call parse failures | LOW | Add lenient parsing as a middleware layer. Does not require agent loop changes. Can be done incrementally per failure mode observed. |
| INFORMATION_SCHEMA over-scanning | LOW | Change query scope from project-level to dataset-level. One-line fix per query, but must audit all INFORMATION_SCHEMA queries. |
| Auth token expiry crashes | LOW | Add retry middleware around GCP client calls. 1-2 days of work if the GCP client is properly abstracted. |
| Context window exhaustion | HIGH | Requires redesigning schema injection, adding truncation to all tool results, implementing compaction. Touches every tool. Estimate: 1-2 weeks. |
| Cost gate bypass | MEDIUM | Add partition metadata to schema cache (schema change), update system prompt with cost-aware instructions, add SQL rewriting layer. Estimate: 3-5 days. |
| SQLite FTS5 corruption | MEDIUM | Implement automatic rebuild from base tables. Add integrity check on startup. Requires schema cache to have non-FTS base tables as source of truth. |
| Permission fatigue | MEDIUM | Redesign permission UX: auto-approve reads, add trust escalation. Requires changes to permission engine and TUI confirmation flow. |
| Agent loop cycling | LOW | Add loop counter and governor. System prompt changes + 1 day of agent loop code. |
| ADK Go immaturity | HIGH | If ADK Go is deeply embedded, switching providers requires rewriting the LLM integration layer. With proper Provider interface, it's a 2-3 day swap. Without it, 2-3 weeks. |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Streaming deadlock | Phase 1: Core TUI + Agent Loop | Stress test with 5000-token streaming response while rapidly pressing keys. No dropped frames, Ctrl+C responds within 500ms. |
| Tool call parse failures | Phase 1: Agent Loop + Tool Dispatch | Fuzz test tool calls with malformed JSON, wrong types, missing fields. Recovery rate > 95%. |
| INFORMATION_SCHEMA over-scanning | Phase 2: Schema Cache | Verify every INFORMATION_SCHEMA query uses dataset-scoped view. Check bytes_processed in test against known dataset. |
| Auth token expiry | Phase 1: GCP Auth Layer | Integration test with 70-minute session (or mock token with 60s expiry). All API calls succeed after refresh. |
| Context window exhaustion | Phase 2-3: Schema Injection + Compaction | Run 30-turn conversation with 500-table warehouse. Token usage stays below 60% of context window. Response quality doesn't degrade. |
| Cost gate bypass | Phase 2: BigQuery Tools + Cost Gate | Generate queries against partitioned tables. Verify partition filter guidance is provided on rejection. Rejection rate below 20% for common queries. |
| SQLite FTS5 corruption | Phase 2: Schema Cache | Concurrent read/write stress test. Kill process mid-write 10 times. FTS5 integrity check passes after each restart. |
| Permission fatigue | Phase 1: Permission Engine | UX test: complete a 10-step debugging workflow. Count confirmations. Should be 0 for read-only steps, 1-2 for write steps. |
| Agent loop cycling | Phase 1: Agent Loop | Test with intentionally ambiguous prompts ("fix it", "check everything"). Loop terminates within 15 tool calls. Agent asks for clarification. |
| ADK Go immaturity | Phase 1: Provider Interface | Implement two providers (Gemini via ADK Go + one other). Swap providers in config. Both pass the same integration test suite. |

## Sources

- Training data knowledge of Bubble Tea architecture (Elm architecture pattern, message-passing model, common issues in charmbracelet/bubbletea GitHub issues and discussions)
- Training data knowledge of BigQuery INFORMATION_SCHEMA scoping (project-level vs dataset-level views, bytes_processed behavior, partition metadata tables)
- Training data knowledge of GCP Application Default Credentials lifecycle (token refresh, service account impersonation chain, `google.DefaultTokenSource` behavior)
- Training data knowledge of SQLite WAL mode concurrency and FTS5 index integrity (modernc.org/sqlite behavior vs CGo SQLite)
- Training data knowledge of LLM agent loop patterns from Claude Code, OpenCode, Aider, and similar tools (tool call failure modes, context window management, loop governors)
- Training data knowledge of Go concurrency patterns (goroutine lifecycle, channel buffering, context cancellation)
- BigQuery pricing documentation (on-demand vs flat-rate, dry-run semantics)
- Cloud Composer API documentation (Composer 1 vs 2 differences, Airflow REST API availability)

**Confidence note:** All findings are based on training data (cutoff approximately May 2025). Google ADK Go findings are MEDIUM confidence as the framework was very new at training cutoff and may have matured significantly. All other findings are MEDIUM-HIGH confidence as they cover well-established technologies with known failure modes.

---
*Pitfalls research for: AI-native GCP data engineering terminal agent*
*Researched: 2026-03-16*

# Pitfalls Research

**Domain:** AI-native GCP data engineering terminal agent (Go + Bubble Tea + LLM + BigQuery/GCS/Composer)
**Researched:** 2026-03-16
**Confidence:** MEDIUM (based on training data across Go, Bubble Tea, BigQuery, GCP, and LLM agent architectures; no live verification available)

## Critical Pitfalls

### Pitfall 1: Bubble Tea Streaming Deadlock — Blocking the Event Loop with LLM Output

**What goes wrong:**
Bubble Tea runs a single-threaded event loop via its `Update` function. LLM streaming responses generate tokens continuously for seconds or minutes. If token delivery blocks the Bubble Tea event loop — or if too many `Cmd` messages pile up from a streaming goroutine — the TUI freezes, drops keystrokes, or deadlocks entirely. The user presses Ctrl+C and nothing happens. This is the single most common failure mode in Bubble Tea apps that integrate with streaming APIs.

**Why it happens:**
Developers treat Bubble Tea like a web framework where you can just push updates. But Bubble Tea is an Elm-architecture: `Update` returns a `(Model, Cmd)` and the runtime processes commands sequentially. If your streaming goroutine sends messages via a channel that fills up (because `Update` is slow processing a large render), the goroutine blocks, which can cascade into the LLM client blocking on its HTTP read, which holds a connection open indefinitely. Conversely, batching too aggressively causes visible stutter.

**How to avoid:**
- Use a dedicated goroutine for LLM streaming that writes to a ring buffer or channel with a generous buffer size (100+), never blocking on send.
- In `Update`, drain the channel non-blockingly per tick. Batch multiple tokens into a single render cycle using `tea.Tick` at 16-33ms intervals (30-60fps), not per-token messages.
- Separate the "token arrived" message from the "render viewport" cycle. Accumulate tokens in the model, re-render on a tick.
- Implement a cancellation channel that the streaming goroutine checks, so Ctrl+C can abort mid-stream.
- Never call `p.Send()` from a goroutine — always return messages via `tea.Cmd` or use `p.Send()` only from a `tea.Cmd`-spawned goroutine that properly handles backpressure.

**Warning signs:**
- TUI freezes for 1-2 seconds during long LLM responses
- Ctrl+C takes noticeable time to respond during streaming
- Token rendering appears "bursty" — nothing, then a wall of text
- High CPU usage during streaming with no visible output

**Phase to address:**
Phase 1 (Core Agent Loop + TUI). This is foundational. If the streaming/rendering pipeline is wrong from the start, retrofitting is a near-rewrite of the TUI layer.

---

### Pitfall 2: LLM Tool Call Parse Failures Treated as Unrecoverable Errors

**What goes wrong:**
LLMs generate malformed tool calls — wrong JSON, hallucinated parameter names, invalid argument types, partially-streamed JSON that fails to parse. If the agent loop treats any tool call parse failure as a hard error (crash, abort, or confused state), the tool becomes unreliable. Users see "error: invalid tool call" 10-20% of the time and lose trust.

**Why it happens:**
Developers test with clean examples and assume LLM output is well-formed. In production, models produce: (1) JSON with trailing commas, (2) tool names that are close-but-wrong (e.g., `bigquery_query` vs `BigQueryQuery`), (3) arguments that are strings when numbers are expected, (4) multiple tool calls when one was expected, (5) tool calls embedded in natural language rather than proper function_call format. The failure rate increases with complex schemas and less-capable models (Ollama local models being worst).

**How to avoid:**
- Implement lenient JSON parsing: strip markdown code fences, handle trailing commas, try `json.Unmarshal` then fall back to regex extraction.
- Fuzzy-match tool names (Levenshtein distance or prefix match) before rejecting.
- Coerce argument types where safe (string "42" to int 42, "true" to bool).
- On parse failure, send the error back to the LLM as a tool result with a clear error message and let it retry. Budget 2-3 retries before surfacing to user.
- Validate tool call schemas server-side before execution — never trust the LLM's output structure.
- Log all malformed tool calls for analysis (what model, what prompt, what went wrong).

**Warning signs:**
- Error rates above 5% for tool calls across any model provider
- Users report "it keeps saying it will do X but then errors"
- Different models have wildly different success rates
- Tool calls work in testing but fail with real user prompts

**Phase to address:**
Phase 1 (Core Agent Loop). The tool dispatch mechanism must be resilient from day one. This is the inner loop of the entire product.

---

### Pitfall 3: INFORMATION_SCHEMA Queries Scanning Entire Organization Instead of Target Project/Dataset

**What goes wrong:**
BigQuery's INFORMATION_SCHEMA has region-scoped and project-scoped views. Querying `region-us.INFORMATION_SCHEMA.COLUMNS` without dataset qualification scans across ALL datasets in the project. For large enterprises with 500+ datasets and 50K+ tables, this query takes 30-60 seconds, processes gigabytes, and costs real money. The schema cache build becomes a blocking, expensive operation that runs on first startup.

**Why it happens:**
The INFORMATION_SCHEMA documentation is confusing about scoping. There are dataset-level views (`dataset.INFORMATION_SCHEMA.COLUMNS` -- scoped to one dataset), project-level views (`region-us.INFORMATION_SCHEMA.COLUMNS` -- all datasets in project), and organization-level views. Developers often start with the project-level view because it seems like the "get everything" approach. But the project context already specifies dataset-scoped caching -- the pitfall is accidentally using project-scoped views "for convenience" during implementation, or during the initial setup wizard scan.

**How to avoid:**
- Always use dataset-scoped INFORMATION_SCHEMA: `project.dataset.INFORMATION_SCHEMA.COLUMNS`. Never use the region-scoped view for cache building.
- In the setup wizard, enumerate datasets first (`INFORMATION_SCHEMA.SCHEMATA`), let the user select which to cache (default: all, but with a count/cost preview).
- Implement per-dataset cache building with progress reporting. If a dataset has 10K+ tables, warn and offer to skip.
- Set a query timeout (30s) on cache-building queries. If a dataset is too large, fall back to sampling or API-based metadata.
- Track bytes_processed from dry-run before executing cache queries.

**Warning signs:**
- Schema cache build takes more than 10 seconds
- Users report high BigQuery costs from Cascade itself
- First-run wizard hangs or appears frozen
- Cache queries appear in BigQuery audit logs scanning TB of metadata

**Phase to address:**
Phase 2 (Schema Cache + BigQuery tools). Must be correct at the point schema caching is implemented. Get the query scoping right from the first implementation.

---

### Pitfall 4: GCP Auth Token Expiry Crashing Mid-Session

**What goes wrong:**
Application Default Credentials (ADC) tokens expire after 1 hour (3600 seconds). If the token refresh fails silently or the HTTP client doesn't handle 401s with automatic retry, every GCP API call starts failing mid-session. The user is 45 minutes into a debugging session, asks "what failed in Composer last night?", and gets a cryptic auth error. Worse: some GCP client libraries cache the token and don't refresh, while others refresh transparently -- behavior varies by library.

**Why it happens:**
During development and testing, sessions are short. The 1-hour token expiry is never hit. In production, data engineers sit in Cascade for hours. The Go GCP client libraries (`cloud.google.com/go`) handle token refresh automatically IF you use the standard `google.DefaultClient()` flow. But if you've created a custom HTTP client, wrapped the transport, or are using service account impersonation with a two-hop token chain, the refresh logic may not trigger correctly. Service account impersonation is particularly tricky: the impersonated token has its own expiry independent of the base credential.

**How to avoid:**
- Use `google.DefaultClient()` or `google.DefaultTokenSource()` and let the standard library handle refresh. Do not cache tokens manually.
- For service account impersonation, use `google.golang.org/api/impersonate` package which handles the token chain refresh.
- Wrap all GCP API calls with a retry-on-401 middleware that forces a token refresh and retries once.
- Test with artificially short token lifetimes or mock token expiry scenarios.
- Display a subtle "re-authenticating..." indicator when a token refresh occurs rather than letting it be invisible.
- On auth failure after retry, provide actionable error: "Run `gcloud auth application-default login` to refresh credentials."

**Warning signs:**
- Any GCP call failing with 401/403 after the session has been open for 45+ minutes
- Tests pass but real usage fails after extended periods
- Service account impersonation works initially but breaks later
- Inconsistent auth failures (works sometimes, fails others -- race condition in refresh)

**Phase to address:**
Phase 1 (GCP Auth foundation). Auth must be robust from the start since every subsequent tool depends on it. Test with long-running sessions in Phase 1.

---

### Pitfall 5: Context Window Exhaustion from Schema + Conversation History

**What goes wrong:**
The agent injects cached schema into the system prompt for SQL generation context. A warehouse with 500 tables, each with 20 columns, produces ~200KB of schema text. Combined with conversation history, tool call results (especially query outputs with many rows), and the platform summary -- the context window fills up within 5-10 turns. The LLM starts hallucinating table names, forgetting earlier conversation, or hitting API token limits causing hard failures.

**Why it happens:**
Schema injection is greedy by default -- "give the LLM everything it might need." Early testing uses small schemas (5-10 tables) where this works perfectly. With real warehouses, the schema alone can consume 30-50% of the context window. Add in a few query results with 100+ rows, some error logs from Composer, and the conversation is at 80% capacity before the user's actual question.

**How to avoid:**
- Never inject full schema. Use the FTS5 index to inject only relevant tables/columns based on the user's current query or topic. 10-20 relevant tables, not 500.
- Implement aggressive context compaction: summarize old conversation turns, drop raw query results after they've been discussed, replace verbose error logs with summaries.
- Set hard limits on tool result sizes: truncate query results to 50 rows with a "showing 50 of 1,234 rows" indicator. Summarize log output.
- Track token usage per message and warn when approaching 70% of context window. Auto-compact at 80%.
- Use Gemini's 2M context as a safety net, not a crutch. Even with 2M tokens, irrelevant context degrades response quality ("lost in the middle" problem).
- Consider a two-pass approach: first pass identifies relevant tables (cheap, small context), second pass generates SQL with only relevant schema.

**Warning signs:**
- LLM references tables that don't exist (hallucinated from partial schema)
- Response quality degrades noticeably after 10+ turns
- API calls failing with token limit errors
- SQL generation works for simple queries but fails when the user has been in a long session

**Phase to address:**
Phase 2 (Schema Cache) for schema injection strategy, Phase 3 (Context Management/Compaction) for conversation history management. Both must be designed together even if implemented in different phases.

---

### Pitfall 6: Cost Gate Bypass Through LLM-Generated SQL Variations

**What goes wrong:**
The cost estimation system does a dry-run of the SQL the LLM generates. But the LLM can generate semantically equivalent queries with vastly different costs. It might use `SELECT *` instead of selecting specific columns. It might scan a non-partitioned table when a partitioned equivalent exists. It might use a subquery that forces a full table scan when a `WHERE` clause on the partition key would limit it. The dry-run catches the cost correctly, but the LLM keeps generating expensive queries that get rejected, frustrating the user.

**Why it happens:**
LLMs optimize for correctness, not cost. Without explicit instruction about partitioning, clustering, and column selection, they write the simplest correct query. The schema context tells them what tables exist but not which are partitioned, what the partition key is, or what the clustering columns are. The cost gate then rejects the query, but the LLM doesn't know WHY it was rejected or how to write a cheaper version.

**How to avoid:**
- Include partition and clustering metadata in schema context. For each table, inject: partition column, partition type (day/month/hour), clustering columns, approximate row count, approximate size.
- When a query exceeds the cost budget, feed back specific guidance: "Query scans 2.3TB. Table `events` is partitioned by `event_date`. Add a `WHERE event_date >= '2026-03-01'` clause to reduce scan."
- Implement a SQL rewriter that automatically adds partition filters when they're missing (before dry-run, suggest to user).
- Default to `SELECT column1, column2` prompting rather than `SELECT *` in the system prompt.
- Set per-query cost limits AND per-session cumulative limits.

**Warning signs:**
- Dry-run rejections exceeding 30% of generated queries
- Users complaining "it keeps trying to run expensive queries"
- LLM entering a loop of generating rejected queries
- Cost estimates showing TB-scale scans for simple questions

**Phase to address:**
Phase 2 (BigQuery tools + cost estimation). The schema cache must store partition/clustering metadata from INFORMATION_SCHEMA.TABLE_OPTIONS and COLUMNS, and the cost gate must provide actionable feedback.

---

### Pitfall 7: SQLite FTS5 Index Corruption Under Concurrent Access from Subagents

**What goes wrong:**
The schema cache uses SQLite with FTS5 for full-text search. Subagents are fire-and-forget goroutines. If the main agent loop and a subagent (e.g., background schema refresh, background cost analysis querying cached schema) both write to or read from SQLite simultaneously, and the connection isn't properly configured for concurrent access, you get `SQLITE_BUSY` errors, corrupted FTS5 indexes, or silent data loss. `modernc.org/sqlite` (pure Go SQLite) has different concurrency characteristics than CGo SQLite.

**Why it happens:**
SQLite's concurrency model is "multiple readers, single writer" with WAL mode, but this requires correct configuration. `modernc.org/sqlite` supports this but defaults may differ. Developers open multiple `*sql.DB` connections or share one without proper mutex protection. FTS5 indexes are particularly fragile during writes -- a failed write mid-FTS-update can leave the index inconsistent, causing future searches to return wrong results or panic.

**How to avoid:**
- Use a single `*sql.DB` instance for all SQLite access, with `_journal_mode=WAL&_busy_timeout=5000` pragmas set at connection open.
- Set `SetMaxOpenConns(1)` for write operations. Use a separate read-only connection pool for concurrent reads if needed.
- Wrap all FTS5 write operations (index rebuild, incremental update) in explicit transactions.
- Never write to the FTS5 index from a subagent goroutine. Schema cache writes should go through a serialized channel to a single writer goroutine.
- Implement FTS5 index integrity checks (`INSERT INTO fts_table(fts_table) VALUES('integrity-check')`) on startup.
- If FTS5 index is corrupted, rebuild it automatically from the base tables rather than crashing.

**Warning signs:**
- Intermittent `SQLITE_BUSY` errors in logs
- Schema search returning stale or missing results
- FTS5 queries that worked before suddenly returning empty
- Subagent goroutines logging database errors silently

**Phase to address:**
Phase 2 (Schema Cache with SQLite/FTS5). Design the concurrency model for the schema cache from the outset. Phase 3 (Subagents) must respect the established access patterns.

---

### Pitfall 8: Permission Model That Blocks Legitimate Workflows

**What goes wrong:**
The 5-level permission model (READ_ONLY through ADMIN) with CONFIRM-by-default seems safe. In practice, it produces "permission fatigue" -- the user confirms 15 read-only operations in a row just to debug a pipeline, gets frustrated, switches to BYPASS mode, and then runs unguarded when a genuinely dangerous operation comes through. The permission system either annoys users into disabling it or fails to protect when it matters.

**Why it happens:**
Permission classification is designed around operation type, not user intent. A debugging workflow might involve: list DAGs (read), get DAG status (read), get task logs (read), query BigQuery logs (read), read GCS file (read), query INFORMATION_SCHEMA (read). Each triggers a confirmation. By the time the user finds the issue and wants to re-run a DAG task (write), they've already mentally checked out of the confirmation flow.

**How to avoid:**
- Auto-approve READ_ONLY operations in CONFIRM mode. Only prompt for WRITE and above.
- Implement "trust escalation" within a session: after confirming 3 operations in the same risk category, auto-approve the rest of that category for the session.
- Group related operations: "I'm going to run 3 read queries to investigate -- approve all?" rather than one-by-one.
- Show a clear risk indicator in the TUI (green/yellow/red) rather than a blocking modal for low-risk operations.
- The PLAN mode (read-only) should be truly non-blocking -- no confirmations needed.
- Reserve blocking confirmation dialogs for DML, DDL, DESTRUCTIVE, and ADMIN only.

**Warning signs:**
- Users immediately switching to BYPASS mode during demos or first use
- Feedback like "too many prompts" or "I just want to look at my data"
- Session logs showing 20+ confirmations with 0 rejections
- Users avoiding tools that trigger confirmations

**Phase to address:**
Phase 1 (Permission Engine). Design with read-auto-approve from the start. Phase 3 (Polish) for trust escalation and grouping.

---

### Pitfall 9: Agent Loop Infinite Cycling on Ambiguous User Requests

**What goes wrong:**
The user says "fix the pipeline." The LLM calls a tool to check DAG status. Gets results. Calls another tool to check logs. Gets results. Calls another tool to check the BigQuery table. Then calls DAG status again because it "wants more information." This cycles 10-20 times, burning tokens and time, without ever producing an answer or asking the user for clarification. The agent appears busy but accomplishes nothing.

**Why it happens:**
The agent loop is observe-reason-select-execute with no governor. The LLM's reasoning step says "I need more information" repeatedly because the request is genuinely ambiguous (which pipeline? which failure? what timeframe?). Without explicit loop limits, escalation triggers, or a "stop and ask" heuristic, the loop continues indefinitely. This is worse with weaker models that have less self-awareness about when to stop.

**How to avoid:**
- Hard limit of 15-20 tool calls per turn (configurable). After the limit, force the agent to summarize findings and respond.
- Implement a "diminishing returns" detector: if the last 3 tool calls returned similar information or the same tool is called twice with the same arguments, stop and synthesize.
- Add explicit "ask for clarification" as a tool. Teach the LLM (via system prompt) to use it when the request is ambiguous.
- Track tool call history within a turn and include it in the LLM context so it can see what it's already done.
- Implement cost tracking per turn -- if a single turn has consumed $X or N tokens, warn and suggest compaction.
- After 5 tool calls with no user-facing output, inject a nudge: "You've made 5 tool calls. Provide a progress update or ask the user for clarification."

**Warning signs:**
- Average tool calls per turn exceeding 8-10
- Same tool being called multiple times with identical or near-identical arguments
- Users reporting "it's thinking for a long time"
- Token costs per conversation turn higher than expected

**Phase to address:**
Phase 1 (Core Agent Loop). The loop governor must be built into the agent loop from the start. Retrofitting it is surprisingly hard because the system prompt and tool design assume unlimited iteration.

---

### Pitfall 10: Treating Google ADK Go as Production-Stable When It Is Early-Stage

**What goes wrong:**
Google ADK Go was announced in early 2025 and is under active development. The API surface changes between versions. Documentation is sparse compared to the Python ADK. Features that exist in the Python ADK (structured output, automatic function calling, agent-to-agent communication) may not be available or may work differently in Go. Building the entire agent loop on ADK Go means absorbing its instability into Cascade's core.

**Why it happens:**
The ADK Go is appealing because it provides native GCP auth, a built-in agent loop, and Vertex AI integration. But "native agent loop" may not match Cascade's requirements (custom tool dispatch, streaming to Bubble Tea, permission checks before tool execution). The developer ends up fighting the framework rather than being helped by it, or discovers a missing feature 3 months into development.

**How to avoid:**
- Use ADK Go as a LLM client library (model calls, streaming, function declarations) but NOT as the agent loop orchestrator. Build Cascade's agent loop independently.
- Abstract ADK Go behind a Provider interface from day one. The interface should be: `Stream(ctx, messages, tools) -> TokenStream` and `Call(ctx, messages, tools) -> Response`. Nothing ADK-specific leaks beyond this boundary.
- Pin to a specific ADK Go version and test against it. Do not auto-update.
- Have a fallback plan: the Provider interface should allow dropping in a raw Gemini REST client, the Anthropic Go SDK, or `sashabaranov/go-openai` if ADK Go proves too immature.
- Monitor the ADK Go GitHub repo for breaking changes and release cadence.

**Warning signs:**
- ADK Go requiring workarounds for basic features (custom headers, timeout control, streaming cancellation)
- Breaking changes in minor version bumps
- Agent loop features that don't map to Cascade's permission model
- Difficulty implementing tool-call interception (for permission checks) within ADK Go's execution model

**Phase to address:**
Phase 1 (LLM Provider abstraction). The Provider interface must be designed and implemented before any ADK Go integration, so ADK Go is pluggable rather than foundational.

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Hardcoding Gemini-specific function calling format | Faster initial development | Every new provider needs format translation; bugs when formats diverge | Never -- define a canonical tool call format in Phase 1 |
| Storing schema cache as flat JSON files | No SQLite dependency to set up | No FTS, no incremental updates, file locking issues, cache invalidation nightmare | Never -- SQLite/FTS5 from the start |
| Inlining SQL strings for INFORMATION_SCHEMA queries | Quick to write | Unmaintainable, untestable, version-dependent on BigQuery schema changes | Only in prototyping; extract to templates before Phase 2 ends |
| Using `fmt.Sprintf` for BigQuery SQL construction | Fast, obvious | SQL injection via LLM-generated values passed to templates | Never -- use parameterized queries for any user/LLM-sourced values |
| Global mutable state for current GCP project/dataset | Simple context passing | Race conditions with subagents, impossible to test, hard to add multi-project support | Only in Phase 1 prototype; refactor to injected config by Phase 2 |
| Skipping dry-run cost estimation in development | Faster iteration | Developers hit real costs; muscle memory of skipping transfers to production code | Never -- dry-run is cheap and should always run |
| Single-file `main.go` agent loop | Rapid iteration | Untestable, unmockable, impossible to add features without merge conflicts | Phase 1 only; decompose into packages before Phase 2 |
| Dumping full query results into LLM context | LLM sees everything | Context exhaustion, slow responses, high token costs | Never -- always truncate and summarize |

## Integration Gotchas

Common mistakes when connecting to external services.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| BigQuery | Using legacy SQL dialect by default | Always set `UseLegacySQL: false` explicitly in query config. Legacy SQL is still the default in some client library versions. |
| BigQuery | Not setting query timeout | Set `JobTimeout` and `QueryTimeout` separately. A query can be queued (job timeout) vs executing (query timeout). Default: 30s job creation, 120s execution. |
| BigQuery | Trusting dry-run cost as exact | Dry-run returns upper bound bytes processed. Actual cost can be lower due to caching, partition pruning at execution time. Communicate as "up to X" not "will cost X". |
| Cloud Composer | Assuming Airflow REST API is always available | Composer 2 exposes the Airflow REST API, but it requires the environment to be running. Stopped/updating environments return 503. Check environment status first. |
| Cloud Composer | Using the Airflow webserver URL directly | Use the Composer API to get the Airflow web UI URL and the DAG GCS bucket. These change between environment recreations. |
| Cloud Logging | Unbounded log queries | Cloud Logging queries without time bounds scan all retained logs (default 30 days). Always add `timestamp >= "time"` filter. Cost and latency scale linearly with time range. |
| Cloud Logging | Not handling pagination | Log queries return paginated results. A query for "all errors in the last hour" might return 10K entries across many pages. Set a reasonable limit (100-500) and offer "load more". |
| GCS | Reading large files into memory | Calling read on a 5GB Parquet file will OOM. Always check object size first (`storage.ObjectAttrs`), stream reads, and set a size limit (e.g., 10MB for `head` operations). |
| dbt | Assuming manifest.json is always current | `manifest.json` is generated by `dbt compile` or `dbt run`. If the user hasn't run dbt recently, the manifest is stale. Check modification time and warn. |
| dbt | Parsing manifest without version checking | dbt manifest format changes between versions (v7, v8, v9, etc.). Check `metadata.dbt_schema_version` before parsing. |
| ADC Auth | Assuming `GOOGLE_APPLICATION_CREDENTIALS` is set | ADC works without this env var (uses `gcloud auth application-default login` credentials). Check both paths. Common in containers vs local dev. |
| Service Account Impersonation | Not checking `iam.serviceAccountTokenCreator` role | Impersonation requires the caller to have this role on the target SA. Fail with a clear message, not a generic 403. |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full schema injection into every LLM call | Slow responses, high token costs | Use FTS5 to inject only relevant tables (10-20 max per call) | 100+ tables in schema cache |
| Synchronous schema cache refresh on startup | Startup takes 30s+ for large warehouses | Build cache asynchronously; use stale cache immediately, refresh in background | 500+ tables across configured datasets |
| Unbatched INFORMATION_SCHEMA queries (one per dataset) | N sequential queries for N datasets | Use concurrent goroutines with rate limiting (5 concurrent max) | 10+ datasets configured |
| Re-parsing dbt manifest on every dbt-related command | Noticeable delay (1-2s) for large projects | Parse once at startup/on-change, hold in memory. Watch `manifest.json` mtime. | 500+ dbt models |
| Storing full query results in conversation history | Context window exhaustion, slow compaction | Store only: query SQL, row count, first 10 rows, summary stats | Any query returning 100+ rows |
| Rendering full markdown in TUI on every keystroke | TUI lag, dropped frames | Debounce markdown rendering to 100ms minimum. Cache rendered output until content changes. | Long LLM responses (1000+ tokens), complex markdown with tables/code blocks |
| Creating new GCP client instances per tool call | Connection overhead, auth token churn | Create clients once at startup, reuse via dependency injection | Any session with 20+ tool calls |
| SQLite FTS5 `MATCH` with very common terms | Slow queries (100ms+) on large indexes | Use prefix queries and column-scoped FTS. Exclude common words (id, name, type) from FTS index or use custom tokenizer. | 50K+ columns indexed |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Passing LLM-generated SQL directly to BigQuery without any sanitization | LLM could generate DDL/DML even when user expects read-only query; prompt injection via data values | Parse SQL with a Go SQL parser before execution; reject DDL/DML in read-only mode; use BigQuery `dryRun` to check query type before execution |
| Logging full query results that may contain PII | Sensitive data persisted to disk in session logs or SQLite | Never log query result data to files. Log only: query SQL, row count, column names. PII detection should flag before display, not after logging. |
| Storing GCP credentials or tokens in config files | Credential theft if dotfiles are committed to git or shared | Never store credentials. Use ADC exclusively. Add `.cascade/` to `.gitignore` template. Warn if `config.toml` contains anything resembling a key or token. |
| Service account impersonation without scope restriction | Impersonated SA may have broader permissions than intended | Always request minimum scopes. Limit to BigQuery, Storage, Logging, Composer scopes only. |
| Executing bash commands from LLM without path restriction | LLM could `rm -rf`, access secrets, exfiltrate data | Bash tool must have: working directory restriction, blocked command list (`rm -rf /`, `curl` to external hosts, credential file reads), timeout, output size limit |
| Trusting LLM-suggested GCS paths without validation | Path traversal: `gs://bucket/../other-bucket/sensitive` | Validate GCS paths against allowed bucket list from config. Reject paths with `..` or paths outside configured buckets. |
| Session history containing sensitive data accessible to future sessions | Cross-session data leakage | Sessions are isolated. Old session data is not injected into new sessions. Session files encrypted at rest or stored in memory only. |
| Cost budget bypass via many small queries | 1000 queries at $0.01 each = $10, bypassing per-query $5 limit | Track cumulative session cost. Alert at configurable threshold. Implement per-session budget, not just per-query. |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Showing raw BigQuery job errors | Users see `invalidQuery: Syntax error at [1:45]` with no context about what the LLM tried | Show the generated SQL with syntax highlighting, the error with the problematic location highlighted, and let the LLM auto-retry with the error context |
| No progress indication during long operations | User thinks the tool crashed during schema cache build or complex queries | Show a spinner with context: "Building schema cache... dataset_1 (234 tables)" or "Running query... (estimated 2.3s, processing 450MB)" |
| Displaying thousands of rows in the terminal | Terminal hangs, scrollback destroyed | Default to 25 rows with pagination. Show row count, offer `/more` command. Offer CSV/JSON export for full results. |
| Making first-run setup mandatory and blocking | User just wants to try the tool, gets a 2-minute setup wizard | Setup wizard should be skippable. Auto-detect what's available (project, datasets, Composer) and work with what's found. Cache builds in background. |
| Dense information in query cost warnings | User ignores cost warnings because they're a wall of text | Simple traffic light: green (< $0.01), yellow ($0.01-$1.00), red (> $1.00) with one-line summary. Detail available via expand. |
| Not showing what the LLM is doing | User sees blank screen for 5-10 seconds while LLM reasons | Stream thinking/reasoning tokens. Show "Analyzing pipeline..." or tool call annotations in real-time. |
| Forcing keyboard-only interaction | Power users love it; new users are lost | Support both slash commands AND natural language for common operations. `/failures` and "show me recent failures" should both work. |
| Inconsistent date/time handling | "Show failures from yesterday" interpreted differently depending on timezone | Always display timestamps in the user's local timezone. Use relative time for recent events ("2 hours ago"). Let config set timezone explicitly. |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **BigQuery tool:** Often missing handling of `REPEATED` (array) and `RECORD` (struct) column types in schema cache -- these need nested representation, not flat columns
- [ ] **Schema cache:** Often missing view definitions and materialized view refresh schedules -- users ask "what does this view do?" and the cache has no answer
- [ ] **Cost estimation:** Often missing slot-based pricing calculation -- dry-run returns bytes, but some orgs use flat-rate/editions pricing where bytes don't map to dollars
- [ ] **Composer integration:** Often missing support for Composer 1 vs Composer 2 API differences -- the Airflow REST API endpoint format differs significantly
- [ ] **dbt integration:** Often missing handling of dbt packages (dependencies) -- manifest includes package models that aren't "yours" but appear in lineage
- [ ] **Permission model:** Often missing handling of tool chains -- a "safe" read tool that triggers an "unsafe" write tool via LLM reasoning isn't caught by per-tool classification
- [ ] **Session management:** Often missing graceful recovery from crash -- if Cascade is killed mid-session, the next startup should offer to resume or show what was lost
- [ ] **Config file:** Often missing validation of GCP project/dataset existence at config load time -- user typos a dataset name, gets cryptic errors 5 minutes later
- [ ] **Streaming output:** Often missing handling of LLM provider rate limits (429) -- user hits rate limit mid-stream, sees partial output with no explanation
- [ ] **Error retry:** Often missing exponential backoff -- retrying immediately on transient GCP errors (503, quota exceeded) makes the problem worse

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Streaming deadlock | MEDIUM | Requires rearchitecting the message/render pipeline. Cannot patch -- must redesign the token accumulation and render tick system. Estimate: 2-3 days. |
| Tool call parse failures | LOW | Add lenient parsing as a middleware layer. Does not require agent loop changes. Can be done incrementally per failure mode observed. |
| INFORMATION_SCHEMA over-scanning | LOW | Change query scope from project-level to dataset-level. One-line fix per query, but must audit all INFORMATION_SCHEMA queries. |
| Auth token expiry crashes | LOW | Add retry middleware around GCP client calls. 1-2 days of work if the GCP client is properly abstracted. |
| Context window exhaustion | HIGH | Requires redesigning schema injection, adding truncation to all tool results, implementing compaction. Touches every tool. Estimate: 1-2 weeks. |
| Cost gate bypass | MEDIUM | Add partition metadata to schema cache (schema change), update system prompt with cost-aware instructions, add SQL rewriting layer. Estimate: 3-5 days. |
| SQLite FTS5 corruption | MEDIUM | Implement automatic rebuild from base tables. Add integrity check on startup. Requires schema cache to have non-FTS base tables as source of truth. |
| Permission fatigue | MEDIUM | Redesign permission UX: auto-approve reads, add trust escalation. Requires changes to permission engine and TUI confirmation flow. |
| Agent loop cycling | LOW | Add loop counter and governor. System prompt changes + 1 day of agent loop code. |
| ADK Go immaturity | HIGH | If ADK Go is deeply embedded, switching providers requires rewriting the LLM integration layer. With proper Provider interface, it's a 2-3 day swap. Without it, 2-3 weeks. |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Streaming deadlock | Phase 1: Core TUI + Agent Loop | Stress test with 5000-token streaming response while rapidly pressing keys. No dropped frames, Ctrl+C responds within 500ms. |
| Tool call parse failures | Phase 1: Agent Loop + Tool Dispatch | Fuzz test tool calls with malformed JSON, wrong types, missing fields. Recovery rate > 95%. |
| INFORMATION_SCHEMA over-scanning | Phase 2: Schema Cache | Verify every INFORMATION_SCHEMA query uses dataset-scoped view. Check bytes_processed in test against known dataset. |
| Auth token expiry | Phase 1: GCP Auth Layer | Integration test with 70-minute session (or mock token with 60s expiry). All API calls succeed after refresh. |
| Context window exhaustion | Phase 2-3: Schema Injection + Compaction | Run 30-turn conversation with 500-table warehouse. Token usage stays below 60% of context window. Response quality doesn't degrade. |
| Cost gate bypass | Phase 2: BigQuery Tools + Cost Gate | Generate queries against partitioned tables. Verify partition filter guidance is provided on rejection. Rejection rate below 20% for common queries. |
| SQLite FTS5 corruption | Phase 2: Schema Cache | Concurrent read/write stress test. Kill process mid-write 10 times. FTS5 integrity check passes after each restart. |
| Permission fatigue | Phase 1: Permission Engine | UX test: complete a 10-step debugging workflow. Count confirmations. Should be 0 for read-only steps, 1-2 for write steps. |
| Agent loop cycling | Phase 1: Agent Loop | Test with intentionally ambiguous prompts ("fix it", "check everything"). Loop terminates within 15 tool calls. Agent asks for clarification. |
| ADK Go immaturity | Phase 1: Provider Interface | Implement two providers (Gemini via ADK Go + one other). Swap providers in config. Both pass the same integration test suite. |

## Sources

- Training data knowledge of Bubble Tea architecture (Elm architecture pattern, message-passing model, common issues in charmbracelet/bubbletea GitHub issues and discussions)
- Training data knowledge of BigQuery INFORMATION_SCHEMA scoping (project-level vs dataset-level views, bytes_processed behavior, partition metadata tables)
- Training data knowledge of GCP Application Default Credentials lifecycle (token refresh, service account impersonation chain, `google.DefaultTokenSource` behavior)
- Training data knowledge of SQLite WAL mode concurrency and FTS5 index integrity (modernc.org/sqlite behavior vs CGo SQLite)
- Training data knowledge of LLM agent loop patterns from Claude Code, OpenCode, Aider, and similar tools (tool call failure modes, context window management, loop governors)
- Training data knowledge of Go concurrency patterns (goroutine lifecycle, channel buffering, context cancellation)
- BigQuery pricing documentation (on-demand vs flat-rate, dry-run semantics)
- Cloud Composer API documentation (Composer 1 vs 2 differences, Airflow REST API availability)

**Confidence note:** All findings are based on training data (cutoff approximately May 2025). Google ADK Go findings are MEDIUM confidence as the framework was very new at training cutoff and may have matured significantly. All other findings are MEDIUM-HIGH confidence as they cover well-established technologies with known failure modes.

---
*Pitfalls research for: AI-native GCP data engineering terminal agent*
*Researched: 2026-03-16*

