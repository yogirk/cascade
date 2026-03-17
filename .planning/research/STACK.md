# Technology Stack

**Project:** Cascade CLI -- AI-native terminal agent for GCP data engineering
**Researched:** 2026-03-16
**Overall Confidence:** HIGH (versions verified via `go list -m`, `gh release list`, and GitHub API)

## Decision Framework

Cascade is a single-binary AI terminal agent. Every dependency must satisfy: (1) pure Go or cgo-free for static cross-compilation, (2) actively maintained, (3) proven in production-grade CLI tools. OpenCode (11.4K stars, same domain) serves as the primary reference architecture for what works.

---

## Recommended Stack

### Language & Runtime

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.26+ (current: 1.26.1) | Language runtime | Sub-5ms startup, single binary, native concurrency, goroutines for subagents. Charm v2 requires Go 1.24.6+; Lip Gloss v2 requires Go 1.25+. Use latest stable. | HIGH |

### AI/LLM Framework

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Google ADK Go (`google.golang.org/adk`) | v0.6.0 | Agent framework: agent loop, tool dispatch, session management, MCP integration | Purpose-built agent framework from Google. Provides `model.LLM` interface, `runner.Runner` agent loop, `functiontool` for typed Go tool functions, `mcptoolset` for MCP servers, session/memory services, and tool confirmation (HITL). 7.2K stars, active monthly releases. Already includes MCP Go SDK integration. | HIGH |
| Google GenAI SDK (`google.golang.org/genai`) | v1.50.0 | Underlying Gemini API client (used by ADK) | ADK's dependency for Gemini model access. Supports both AI Studio and Vertex AI backends. Provides `genai.Content`, `genai.GenerateContentConfig` types that flow through the ADK `model.LLM` interface. | HIGH |

**Critical ADK Architecture Note:** ADK's `model.LLM` interface uses `genai.Content` and `genai.GenerateContentConfig` types from the Google GenAI SDK throughout. This means implementing non-Gemini providers (Claude, OpenAI) requires translating their native types to/from `genai.*` types. This is a deliberate tradeoff -- ADK provides the agent loop, tool dispatch, session management, and MCP integration for free, but non-Gemini support requires an adapter layer. OpenCode chose to build its own provider abstraction from scratch instead. For Cascade, ADK is the right choice because Gemini is the default model and the framework value (agent loop, tool confirmation, MCP) outweighs the adapter cost.

### LLM Provider SDKs (for multi-model support)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go`) | v1.26.0 | Claude model access | Official SDK, 900 stars. Needed for Claude provider adapter behind ADK's `model.LLM` interface. | HIGH |
| OpenAI Go SDK (`github.com/openai/openai-go`) | v3.28.0 | OpenAI/compatible model access | Official SDK, 3K stars. Covers OpenAI, Azure OpenAI, and any OpenAI-compatible endpoint (Ollama, vLLM, etc). | HIGH |

### TUI Framework (Charm v2 Stack)

| Technology | Version | Import Path | Purpose | Why | Confidence |
|------------|---------|-------------|---------|-----|------------|
| Bubble Tea | v2.0.2 | `charm.land/bubbletea/v2` | TUI framework (Elm architecture) | 40.7K stars. v2 ships the Cursed Renderer (ncurses-based), declarative view model, progressive keyboard enhancements, built-in color downsampling. Major release Feb 2026. | HIGH |
| Lip Gloss | v2.0.2 | `charm.land/lipgloss/v2` | Terminal styling | v2 is now pure (no I/O fighting with Bubble Tea), Bubble Tea manages I/O. Must use v2 with Bubble Tea v2. | HIGH |
| Glamour | v2.0.0 | `charm.land/glamour/v2` | Markdown rendering in terminal | Renders LLM markdown output. v2 released alongside Bubble Tea v2 for compatibility. | HIGH |
| Bubbles | v2.0.0 | `charm.land/bubbles/v2` | Pre-built TUI components (text input, viewport, spinner, list, table) | Standard components for input, scrollable output, loading states. v2 for Bubble Tea v2 compatibility. | HIGH |
| Huh | v2.0.3 | `charm.land/huh/v2` | Form/wizard framework | Powers the setup wizard, permission confirmation dialogs, config forms. v2 for ecosystem compatibility. | HIGH |

**Charm v2 Ecosystem Note:** The entire Charm stack moved to `charm.land/` vanity import paths in v2. All v2 packages must be used together -- do not mix v1 and v2 Charm packages. Bubble Tea v2 requires Go 1.24.6+; Lip Gloss/Bubbles/Huh require Go 1.25+.

### Database

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| modernc.org/sqlite | v1.46.1 | Schema cache, session storage, local state | Pure Go (C-to-Go transpiled), zero cgo. Enables static cross-compilation for all targets. FTS5 support for full-text schema search. ADK Go itself uses `glebarez/sqlite` (which wraps modernc.org/sqlite). Well-proven, actively maintained. | HIGH |

**Why not ncruces/go-sqlite3?** Also cgo-free (uses Wasm via wasm2go), 936 stars, active development, full FTS5 support. Viable alternative. However, modernc.org/sqlite is the ecosystem standard for pure-Go SQLite (used transitively by ADK Go itself), has broader community adoption, and eliminates the Wasm runtime overhead. Use modernc.org/sqlite unless you hit specific compilation issues.

**Why not mattn/go-sqlite3?** Requires cgo, which breaks static cross-compilation for the single-binary distribution goal.

### GCP Client Libraries

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `cloud.google.com/go/bigquery` | v1.74.0 | BigQuery queries, schema, jobs | Official Google client. Provides `bigquery.Client` for query execution, dry-run cost estimation, dataset/table listing, INFORMATION_SCHEMA access. | HIGH |
| `cloud.google.com/go/storage` | v1.61.3 | GCS operations (ls, head, read) | Official client for bucket listing, object metadata, content reading. | HIGH |
| `cloud.google.com/go/logging` | v1.13.2 | Cloud Logging queries | Official client for reading log entries, filtering by resource/severity/time. | HIGH |
| `cloud.google.com/go/orchestration` | v1.11.10 | Cloud Composer (managed Airflow) | Official client for Composer environments. Provides environment metadata, but DAG/task operations require Airflow REST API access through the Composer web server URL. | MEDIUM |
| `cloud.google.com/go/dataplex` | v1.28.0 | Dataplex metadata, data quality, tags | Official client for reading Dataplex tags (PII detection), data quality rules, catalog entries. | HIGH |
| `cloud.google.com/go/dataflow` | v0.11.1 | Dataflow job monitoring (Phase 2+) | Official client. Note v0.x version -- API is pre-GA. Out of scope for V1 per PROJECT.md but listed for future reference. | MEDIUM |
| `cloud.google.com/go/pubsub` | v1.50.1 | Pub/Sub topic/subscription info (Phase 2+) | Official client. Out of scope for V1 per PROJECT.md but listed for future reference. | HIGH |
| `google.golang.org/api` | v0.271.0 | Google API discovery-based clients | Fallback for services without dedicated Go clients. Also provides `option.WithCredentials` and auth utilities shared across all GCP clients. | HIGH |

**Composer/Airflow Integration Note:** The `cloud.google.com/go/orchestration` package handles Composer environment management (create, list, update), but DAG operations (list DAGs, trigger runs, get task logs, list failures) require hitting the Airflow REST API directly via the Composer web server URL. This means: (1) use orchestration SDK to discover the Composer environment's `airflowUri`, (2) use `net/http` with ADC-based auth to call the Airflow 2.x stable REST API. This is the standard pattern -- no SDK wraps Airflow's DAG API.

### CLI & Configuration

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Cobra (`github.com/spf13/cobra`) | v1.10.2 | CLI command framework | Industry standard. Used by ADK Go, kubectl, gh, docker. Handles subcommands, flags, completions, help generation. | HIGH |
| BurntSushi/toml (`github.com/BurntSushi/toml`) | v1.6.0 | TOML config parsing | Spec-compliant, zero dependencies, battle-tested. Parses `~/.cascade/config.toml` and `CASCADE.md` TOML frontmatter. Preferred over pelletier/go-toml for simplicity. | HIGH |

**Why not Viper?** OpenCode uses Viper (via spf13/viper), which adds environment variable binding, remote config, and watch support. Cascade's config model is simpler: one TOML file + CLI flags + env vars. BurntSushi/toml + manual env override is lighter and avoids Viper's transitive dependency tree. If config needs grow, migrate to Viper later.

### MCP (Model Context Protocol)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) | v1.4.1 | MCP server/client integration | Official SDK, maintained in collaboration with Google. ADK Go v0.6.0 already depends on this (uses `mcptoolset` package). Provides both server and client capabilities. | HIGH |

**Why not mark3labs/mcp-go?** This was the community SDK (OpenCode uses v0.17.0) before the official SDK existed. The official `modelcontextprotocol/go-sdk` is now the standard, actively maintained by the MCP project and Google, and already integrated into ADK Go. Use the official SDK.

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `testing` (stdlib) | - | Unit tests | Go standard library. No framework needed. | HIGH |
| `github.com/google/go-cmp` | v0.7.0 | Deep equality comparison | Used by ADK Go itself. Better diff output than `reflect.DeepEqual`. | HIGH |
| `github.com/stretchr/testify` | v1.10.0 | Assertions and mocking | Used by OpenCode. Provides `assert`, `require`, `mock` packages. Optional -- stdlib `testing` is sufficient for most cases. | HIGH |

### Distribution

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| GoReleaser | v2.14.3 | Build, package, release | Cross-compiles for linux/darwin/windows x amd64/arm64, creates Homebrew formula, uploads GitHub releases. Industry standard for Go CLI distribution. | HIGH |

### Supporting Libraries

| Library | Version | Purpose | When to Use | Confidence |
|---------|---------|---------|-------------|------------|
| `github.com/google/uuid` | v1.6.0 | UUID generation | Session IDs, request IDs, tool call IDs | HIGH |
| `golang.org/x/sync` | v0.19.0 | Concurrency primitives | `errgroup` for parallel GCP API calls, `singleflight` for deduplication | HIGH |
| `golang.org/x/oauth2` | v0.34.0 | OAuth2/ADC auth | GCP Application Default Credentials flow | HIGH |
| `github.com/charmbracelet/x/ansi` | v0.11.6 | ANSI sequence handling | Terminal width detection, escape sequence stripping | HIGH |
| `github.com/aymanbagabas/go-udiff` | v0.2.0 | Unified diff generation | File Edit tool diff display | HIGH |
| `github.com/bmatcuk/doublestar/v4` | v4.8.1 | Glob pattern matching | Glob tool for file matching | HIGH |
| `github.com/lithammer/fuzzysearch` | v1.1.8 | Fuzzy string matching | Schema search, command matching | MEDIUM |
| `github.com/alecthomas/chroma/v2` | v2.15.0 | Syntax highlighting | SQL and code highlighting in terminal output | MEDIUM |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Agent Framework | ADK Go | Build from scratch (OpenCode pattern) | ADK provides agent loop, tool dispatch, session management, MCP integration, tool confirmation for free. Building from scratch means reimplementing all of this. The genai type coupling is manageable via adapter pattern. |
| Agent Framework | ADK Go | LangChain Go | LangChain Go is less mature, Python-centric design ported to Go, heavier abstraction. ADK is Google-native, Gemini-optimized, and has official MCP integration. |
| TUI | Charm v2 stack | tview/tcell | Charm has the modern ecosystem (markdown rendering, forms, styling), Elm architecture is cleaner for complex UIs. tview is imperative, lacks Glamour/Huh equivalents. |
| TUI | Charm v2 stack | Charm v1 stack | v1 is being superseded. v2 has the Cursed Renderer, declarative views, progressive keyboard enhancements. Starting a new project on v1 would require migration later. |
| SQLite | modernc.org/sqlite | mattn/go-sqlite3 | Requires cgo, breaks static cross-compilation. |
| SQLite | modernc.org/sqlite | ncruces/go-sqlite3 | Viable but less ecosystem adoption. Wasm translation adds small overhead. modernc.org is the default choice. |
| Config | BurntSushi/toml | Viper | Viper is overkill for single-file TOML config. Adds heavy dependency tree (remote config, fsnotify, etc). |
| Config | TOML | YAML | TOML is better for configuration files -- explicit types, no indentation ambiguity, good for nested sections like `[composer]`, `[dbt]`, `[security]`. |
| CLI | Cobra | Kong/urfave/cli | Cobra is the Go standard, used by ADK Go itself. Best completion support, best documentation. |
| MCP | Official Go SDK | mark3labs/mcp-go | Official SDK is the standard going forward, already integrated with ADK Go. Community SDK will likely converge or deprecate. |
| LLM SDK | Direct SDKs (Anthropic, OpenAI) | LiteLLM proxy | Adding a proxy adds deployment complexity. Direct SDKs keep it a single binary. Adapter pattern behind ADK's model.LLM interface is cleaner. |

---

## Critical Architecture Decision: ADK Go + Custom Provider Adapters

The most consequential stack decision is using ADK Go as the agent framework despite its `genai.*` type coupling.

**What ADK gives you for free:**
- Agent loop with streaming (`runner.Runner`)
- Typed function tools with JSON schema inference (`functiontool.New`)
- Tool confirmation / Human-in-the-Loop (`toolconfirmation`)
- MCP server integration (`mcptoolset`)
- Session management (in-memory, database, Vertex AI)
- Multi-agent composition (`workflowagents`)
- A2A protocol support
- OpenTelemetry tracing

**What you must build:**
- `model.LLM` adapters for Claude and OpenAI that translate `genai.Content` <-> native types
- Custom TUI integration (ADK's runner emits events you render in Bubble Tea)
- All GCP tools (BigQuery, Composer, GCS, Logging, etc.) as ADK `functiontool` implementations
- Permission engine wrapping ADK's tool confirmation
- Schema cache and context injection

**The adapter pattern:** Implement `model.LLM` interface for each provider. The interface is small (just `Name()` and `GenerateContent()`). The translation layer maps `genai.Content` (which has `Parts` containing `Text`, `FunctionCall`, `FunctionResponse`) to each provider's native message format. This is ~200-400 lines per provider, one-time cost.

---

## Version Compatibility Matrix

| Component | Min Go Version | Notes |
|-----------|---------------|-------|
| Go runtime | 1.26.1 | Current stable |
| ADK Go v0.6.0 | 1.24.4 | |
| Bubble Tea v2.0.2 | 1.24.6 | |
| Lip Gloss v2.0.2 | 1.25.0 | Sets the floor |
| Glamour v2.0.0 | 1.25.8 | |
| Huh v2.0.3 | 1.25.8 | Highest Charm requirement |
| GCP client libs | ~1.21+ | |

**Go version floor: 1.25.8** (driven by Glamour v2 and Huh v2). Go 1.26.1 satisfies all requirements.

---

## Installation

```bash
# Initialize module
go mod init github.com/your-org/cascade

# Core: ADK Go agent framework
go get google.golang.org/adk@v0.6.0

# TUI: Charm v2 stack
go get charm.land/bubbletea/v2@v2.0.2
go get charm.land/lipgloss/v2@v2.0.2
go get charm.land/glamour/v2@v2.0.0
go get charm.land/bubbles/v2@v2.0.0
go get charm.land/huh/v2@v2.0.3

# LLM provider SDKs (for multi-model)
go get github.com/anthropics/anthropic-sdk-go@v1.26.0
go get github.com/openai/openai-go@v3.28.0

# GCP client libraries
go get cloud.google.com/go/bigquery@v1.74.0
go get cloud.google.com/go/storage@v1.61.3
go get cloud.google.com/go/logging@v1.13.2
go get cloud.google.com/go/orchestration@v1.11.10
go get cloud.google.com/go/dataplex@v1.28.0

# Database
go get modernc.org/sqlite@v1.46.1

# CLI & Config
go get github.com/spf13/cobra@v1.10.2
go get github.com/BurntSushi/toml@v1.6.0

# Supporting
go get github.com/google/uuid@v1.6.0
go get golang.org/x/sync@v0.19.0
go get github.com/aymanbagabas/go-udiff@v0.2.0
go get github.com/bmatcuk/doublestar/v4@v4.8.1
go get github.com/alecthomas/chroma/v2@v2.15.0
```

```bash
# Dev tools
go install github.com/goreleaser/goreleaser/v2@v2.14.3
```

---

## Risk Assessment

### ADK Go Maturity (MEDIUM RISK)
ADK Go is at v0.6.0 -- pre-1.0 with monthly breaking releases (v0.2 through v0.6 in 5 months). The API is stabilizing but not guaranteed. **Mitigation:** Pin version, wrap ADK types behind internal interfaces, limit ADK surface area to runner + tool + model packages. The `model.LLM` interface has been stable since v0.2.

### Charm v2 Freshness (LOW RISK)
Bubble Tea v2.0.0 released Feb 24, 2026 -- less than a month old. Import paths changed from `github.com/charmbracelet/*` to `charm.land/*/v2`. Some community examples and tutorials still reference v1. **Mitigation:** Charm v2 is a coordinated release across the whole stack, well-documented with an upgrade guide. OpenCode still uses Charm v1 (as of its latest go.mod), but starting fresh on v2 is correct for a new project.

### genai Type Coupling (LOW RISK)
ADK's `model.LLM` interface uses `genai.Content` types. Non-Gemini providers need translation. **Mitigation:** The translation is mechanical (text parts, function calls, function responses map cleanly across providers). ~200-400 LOC per adapter. This is a known, bounded cost.

### GCP SDK Stability (LOW RISK)
All GCP Go client libraries are stable (v1.x) except `cloud.google.com/go/dataflow` (v0.11.1, pre-GA). BigQuery, Storage, Logging, Dataplex are all well-established. **Mitigation:** Dataflow is Phase 2+ anyway.

---

## Sources

All versions verified via direct tooling on 2026-03-16:

- ADK Go: `gh release list --repo google/adk-go` -- v0.6.0, 7.2K stars
- ADK Go go.mod: `gh api repos/google/adk-go/contents/go.mod` -- dependencies verified
- ADK Go source: `gh api` for model/llm.go, tool/tool.go, runner/runner.go, model/gemini/gemini.go, tool/functiontool/function.go
- OpenCode: `gh api repos/opencode-ai/opencode/contents/go.mod` -- reference architecture, 11.4K stars
- Bubble Tea: `gh release list --repo charmbracelet/bubbletea` -- v2.0.2
- Bubble Tea v2 release notes: `gh release view v2.0.0 --repo charmbracelet/bubbletea`
- Lip Gloss: `gh release list --repo charmbracelet/lipgloss` -- v2.0.2
- Glamour: `gh release list --repo charmbracelet/glamour` -- v2.0.0
- Bubbles: `gh release list --repo charmbracelet/bubbles` -- v2.0.0
- Huh: `gh release list --repo charmbracelet/huh` -- v2.0.3
- Charm v2 go.mod files: verified `charm.land/` import paths and Go version requirements
- GCP SDKs: `go list -m [module]@latest` for all packages
- modernc.org/sqlite: `go list -m modernc.org/sqlite@latest` -- v1.46.1
- ncruces/go-sqlite3: `gh repo view` -- v0.32.0, 936 stars
- Anthropic SDK: `gh repo view anthropics/anthropic-sdk-go` -- v1.26.0, 900 stars
- OpenAI SDK: `gh repo view openai/openai-go` -- v3.28.0, 3K stars
- MCP Go SDK: `gh release list --repo modelcontextprotocol/go-sdk` -- v1.4.1
- GenAI SDK: `go list -m google.golang.org/genai@latest` -- v1.50.0
- Cobra: `go list -m github.com/spf13/cobra@latest` -- v1.10.2
- BurntSushi/toml: `go list -m` -- v1.6.0
- GoReleaser: `gh release view --repo goreleaser/goreleaser` -- v2.14.3
- Go runtime: `go version` -- go1.26.1 darwin/arm64

# Technology Stack

**Project:** Cascade CLI -- AI-native terminal agent for GCP data engineering
**Researched:** 2026-03-16
**Overall Confidence:** HIGH (versions verified via `go list -m`, `gh release list`, and GitHub API)

## Decision Framework

Cascade is a single-binary AI terminal agent. Every dependency must satisfy: (1) pure Go or cgo-free for static cross-compilation, (2) actively maintained, (3) proven in production-grade CLI tools. OpenCode (11.4K stars, same domain) serves as the primary reference architecture for what works.

---

## Recommended Stack

### Language & Runtime

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.26+ (current: 1.26.1) | Language runtime | Sub-5ms startup, single binary, native concurrency, goroutines for subagents. Charm v2 requires Go 1.24.6+; Lip Gloss v2 requires Go 1.25+. Use latest stable. | HIGH |

### AI/LLM Framework

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Google ADK Go (`google.golang.org/adk`) | v0.6.0 | Agent framework: agent loop, tool dispatch, session management, MCP integration | Purpose-built agent framework from Google. Provides `model.LLM` interface, `runner.Runner` agent loop, `functiontool` for typed Go tool functions, `mcptoolset` for MCP servers, session/memory services, and tool confirmation (HITL). 7.2K stars, active monthly releases. Already includes MCP Go SDK integration. | HIGH |
| Google GenAI SDK (`google.golang.org/genai`) | v1.50.0 | Underlying Gemini API client (used by ADK) | ADK's dependency for Gemini model access. Supports both AI Studio and Vertex AI backends. Provides `genai.Content`, `genai.GenerateContentConfig` types that flow through the ADK `model.LLM` interface. | HIGH |

**Critical ADK Architecture Note:** ADK's `model.LLM` interface uses `genai.Content` and `genai.GenerateContentConfig` types from the Google GenAI SDK throughout. This means implementing non-Gemini providers (Claude, OpenAI) requires translating their native types to/from `genai.*` types. This is a deliberate tradeoff -- ADK provides the agent loop, tool dispatch, session management, and MCP integration for free, but non-Gemini support requires an adapter layer. OpenCode chose to build its own provider abstraction from scratch instead. For Cascade, ADK is the right choice because Gemini is the default model and the framework value (agent loop, tool confirmation, MCP) outweighs the adapter cost.

### LLM Provider SDKs (for multi-model support)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go`) | v1.26.0 | Claude model access | Official SDK, 900 stars. Needed for Claude provider adapter behind ADK's `model.LLM` interface. | HIGH |
| OpenAI Go SDK (`github.com/openai/openai-go`) | v3.28.0 | OpenAI/compatible model access | Official SDK, 3K stars. Covers OpenAI, Azure OpenAI, and any OpenAI-compatible endpoint (Ollama, vLLM, etc). | HIGH |

### TUI Framework (Charm v2 Stack)

| Technology | Version | Import Path | Purpose | Why | Confidence |
|------------|---------|-------------|---------|-----|------------|
| Bubble Tea | v2.0.2 | `charm.land/bubbletea/v2` | TUI framework (Elm architecture) | 40.7K stars. v2 ships the Cursed Renderer (ncurses-based), declarative view model, progressive keyboard enhancements, built-in color downsampling. Major release Feb 2026. | HIGH |
| Lip Gloss | v2.0.2 | `charm.land/lipgloss/v2` | Terminal styling | v2 is now pure (no I/O fighting with Bubble Tea), Bubble Tea manages I/O. Must use v2 with Bubble Tea v2. | HIGH |
| Glamour | v2.0.0 | `charm.land/glamour/v2` | Markdown rendering in terminal | Renders LLM markdown output. v2 released alongside Bubble Tea v2 for compatibility. | HIGH |
| Bubbles | v2.0.0 | `charm.land/bubbles/v2` | Pre-built TUI components (text input, viewport, spinner, list, table) | Standard components for input, scrollable output, loading states. v2 for Bubble Tea v2 compatibility. | HIGH |
| Huh | v2.0.3 | `charm.land/huh/v2` | Form/wizard framework | Powers the setup wizard, permission confirmation dialogs, config forms. v2 for ecosystem compatibility. | HIGH |

**Charm v2 Ecosystem Note:** The entire Charm stack moved to `charm.land/` vanity import paths in v2. All v2 packages must be used together -- do not mix v1 and v2 Charm packages. Bubble Tea v2 requires Go 1.24.6+; Lip Gloss/Bubbles/Huh require Go 1.25+.

### Database

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| modernc.org/sqlite | v1.46.1 | Schema cache, session storage, local state | Pure Go (C-to-Go transpiled), zero cgo. Enables static cross-compilation for all targets. FTS5 support for full-text schema search. ADK Go itself uses `glebarez/sqlite` (which wraps modernc.org/sqlite). Well-proven, actively maintained. | HIGH |

**Why not ncruces/go-sqlite3?** Also cgo-free (uses Wasm via wasm2go), 936 stars, active development, full FTS5 support. Viable alternative. However, modernc.org/sqlite is the ecosystem standard for pure-Go SQLite (used transitively by ADK Go itself), has broader community adoption, and eliminates the Wasm runtime overhead. Use modernc.org/sqlite unless you hit specific compilation issues.

**Why not mattn/go-sqlite3?** Requires cgo, which breaks static cross-compilation for the single-binary distribution goal.

### GCP Client Libraries

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `cloud.google.com/go/bigquery` | v1.74.0 | BigQuery queries, schema, jobs | Official Google client. Provides `bigquery.Client` for query execution, dry-run cost estimation, dataset/table listing, INFORMATION_SCHEMA access. | HIGH |
| `cloud.google.com/go/storage` | v1.61.3 | GCS operations (ls, head, read) | Official client for bucket listing, object metadata, content reading. | HIGH |
| `cloud.google.com/go/logging` | v1.13.2 | Cloud Logging queries | Official client for reading log entries, filtering by resource/severity/time. | HIGH |
| `cloud.google.com/go/orchestration` | v1.11.10 | Cloud Composer (managed Airflow) | Official client for Composer environments. Provides environment metadata, but DAG/task operations require Airflow REST API access through the Composer web server URL. | MEDIUM |
| `cloud.google.com/go/dataplex` | v1.28.0 | Dataplex metadata, data quality, tags | Official client for reading Dataplex tags (PII detection), data quality rules, catalog entries. | HIGH |
| `cloud.google.com/go/dataflow` | v0.11.1 | Dataflow job monitoring (Phase 2+) | Official client. Note v0.x version -- API is pre-GA. Out of scope for V1 per PROJECT.md but listed for future reference. | MEDIUM |
| `cloud.google.com/go/pubsub` | v1.50.1 | Pub/Sub topic/subscription info (Phase 2+) | Official client. Out of scope for V1 per PROJECT.md but listed for future reference. | HIGH |
| `google.golang.org/api` | v0.271.0 | Google API discovery-based clients | Fallback for services without dedicated Go clients. Also provides `option.WithCredentials` and auth utilities shared across all GCP clients. | HIGH |

**Composer/Airflow Integration Note:** The `cloud.google.com/go/orchestration` package handles Composer environment management (create, list, update), but DAG operations (list DAGs, trigger runs, get task logs, list failures) require hitting the Airflow REST API directly via the Composer web server URL. This means: (1) use orchestration SDK to discover the Composer environment's `airflowUri`, (2) use `net/http` with ADC-based auth to call the Airflow 2.x stable REST API. This is the standard pattern -- no SDK wraps Airflow's DAG API.

### CLI & Configuration

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Cobra (`github.com/spf13/cobra`) | v1.10.2 | CLI command framework | Industry standard. Used by ADK Go, kubectl, gh, docker. Handles subcommands, flags, completions, help generation. | HIGH |
| BurntSushi/toml (`github.com/BurntSushi/toml`) | v1.6.0 | TOML config parsing | Spec-compliant, zero dependencies, battle-tested. Parses `~/.cascade/config.toml` and `CASCADE.md` TOML frontmatter. Preferred over pelletier/go-toml for simplicity. | HIGH |

**Why not Viper?** OpenCode uses Viper (via spf13/viper), which adds environment variable binding, remote config, and watch support. Cascade's config model is simpler: one TOML file + CLI flags + env vars. BurntSushi/toml + manual env override is lighter and avoids Viper's transitive dependency tree. If config needs grow, migrate to Viper later.

### MCP (Model Context Protocol)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) | v1.4.1 | MCP server/client integration | Official SDK, maintained in collaboration with Google. ADK Go v0.6.0 already depends on this (uses `mcptoolset` package). Provides both server and client capabilities. | HIGH |

**Why not mark3labs/mcp-go?** This was the community SDK (OpenCode uses v0.17.0) before the official SDK existed. The official `modelcontextprotocol/go-sdk` is now the standard, actively maintained by the MCP project and Google, and already integrated into ADK Go. Use the official SDK.

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `testing` (stdlib) | - | Unit tests | Go standard library. No framework needed. | HIGH |
| `github.com/google/go-cmp` | v0.7.0 | Deep equality comparison | Used by ADK Go itself. Better diff output than `reflect.DeepEqual`. | HIGH |
| `github.com/stretchr/testify` | v1.10.0 | Assertions and mocking | Used by OpenCode. Provides `assert`, `require`, `mock` packages. Optional -- stdlib `testing` is sufficient for most cases. | HIGH |

### Distribution

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| GoReleaser | v2.14.3 | Build, package, release | Cross-compiles for linux/darwin/windows x amd64/arm64, creates Homebrew formula, uploads GitHub releases. Industry standard for Go CLI distribution. | HIGH |

### Supporting Libraries

| Library | Version | Purpose | When to Use | Confidence |
|---------|---------|---------|-------------|------------|
| `github.com/google/uuid` | v1.6.0 | UUID generation | Session IDs, request IDs, tool call IDs | HIGH |
| `golang.org/x/sync` | v0.19.0 | Concurrency primitives | `errgroup` for parallel GCP API calls, `singleflight` for deduplication | HIGH |
| `golang.org/x/oauth2` | v0.34.0 | OAuth2/ADC auth | GCP Application Default Credentials flow | HIGH |
| `github.com/charmbracelet/x/ansi` | v0.11.6 | ANSI sequence handling | Terminal width detection, escape sequence stripping | HIGH |
| `github.com/aymanbagabas/go-udiff` | v0.2.0 | Unified diff generation | File Edit tool diff display | HIGH |
| `github.com/bmatcuk/doublestar/v4` | v4.8.1 | Glob pattern matching | Glob tool for file matching | HIGH |
| `github.com/lithammer/fuzzysearch` | v1.1.8 | Fuzzy string matching | Schema search, command matching | MEDIUM |
| `github.com/alecthomas/chroma/v2` | v2.15.0 | Syntax highlighting | SQL and code highlighting in terminal output | MEDIUM |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Agent Framework | ADK Go | Build from scratch (OpenCode pattern) | ADK provides agent loop, tool dispatch, session management, MCP integration, tool confirmation for free. Building from scratch means reimplementing all of this. The genai type coupling is manageable via adapter pattern. |
| Agent Framework | ADK Go | LangChain Go | LangChain Go is less mature, Python-centric design ported to Go, heavier abstraction. ADK is Google-native, Gemini-optimized, and has official MCP integration. |
| TUI | Charm v2 stack | tview/tcell | Charm has the modern ecosystem (markdown rendering, forms, styling), Elm architecture is cleaner for complex UIs. tview is imperative, lacks Glamour/Huh equivalents. |
| TUI | Charm v2 stack | Charm v1 stack | v1 is being superseded. v2 has the Cursed Renderer, declarative views, progressive keyboard enhancements. Starting a new project on v1 would require migration later. |
| SQLite | modernc.org/sqlite | mattn/go-sqlite3 | Requires cgo, breaks static cross-compilation. |
| SQLite | modernc.org/sqlite | ncruces/go-sqlite3 | Viable but less ecosystem adoption. Wasm translation adds small overhead. modernc.org is the default choice. |
| Config | BurntSushi/toml | Viper | Viper is overkill for single-file TOML config. Adds heavy dependency tree (remote config, fsnotify, etc). |
| Config | TOML | YAML | TOML is better for configuration files -- explicit types, no indentation ambiguity, good for nested sections like `[composer]`, `[dbt]`, `[security]`. |
| CLI | Cobra | Kong/urfave/cli | Cobra is the Go standard, used by ADK Go itself. Best completion support, best documentation. |
| MCP | Official Go SDK | mark3labs/mcp-go | Official SDK is the standard going forward, already integrated with ADK Go. Community SDK will likely converge or deprecate. |
| LLM SDK | Direct SDKs (Anthropic, OpenAI) | LiteLLM proxy | Adding a proxy adds deployment complexity. Direct SDKs keep it a single binary. Adapter pattern behind ADK's model.LLM interface is cleaner. |

---

## Critical Architecture Decision: ADK Go + Custom Provider Adapters

The most consequential stack decision is using ADK Go as the agent framework despite its `genai.*` type coupling.

**What ADK gives you for free:**
- Agent loop with streaming (`runner.Runner`)
- Typed function tools with JSON schema inference (`functiontool.New`)
- Tool confirmation / Human-in-the-Loop (`toolconfirmation`)
- MCP server integration (`mcptoolset`)
- Session management (in-memory, database, Vertex AI)
- Multi-agent composition (`workflowagents`)
- A2A protocol support
- OpenTelemetry tracing

**What you must build:**
- `model.LLM` adapters for Claude and OpenAI that translate `genai.Content` <-> native types
- Custom TUI integration (ADK's runner emits events you render in Bubble Tea)
- All GCP tools (BigQuery, Composer, GCS, Logging, etc.) as ADK `functiontool` implementations
- Permission engine wrapping ADK's tool confirmation
- Schema cache and context injection

**The adapter pattern:** Implement `model.LLM` interface for each provider. The interface is small (just `Name()` and `GenerateContent()`). The translation layer maps `genai.Content` (which has `Parts` containing `Text`, `FunctionCall`, `FunctionResponse`) to each provider's native message format. This is ~200-400 lines per provider, one-time cost.

---

## Version Compatibility Matrix

| Component | Min Go Version | Notes |
|-----------|---------------|-------|
| Go runtime | 1.26.1 | Current stable |
| ADK Go v0.6.0 | 1.24.4 | |
| Bubble Tea v2.0.2 | 1.24.6 | |
| Lip Gloss v2.0.2 | 1.25.0 | Sets the floor |
| Glamour v2.0.0 | 1.25.8 | |
| Huh v2.0.3 | 1.25.8 | Highest Charm requirement |
| GCP client libs | ~1.21+ | |

**Go version floor: 1.25.8** (driven by Glamour v2 and Huh v2). Go 1.26.1 satisfies all requirements.

---

## Installation

```bash
# Initialize module
go mod init github.com/your-org/cascade

# Core: ADK Go agent framework
go get google.golang.org/adk@v0.6.0

# TUI: Charm v2 stack
go get charm.land/bubbletea/v2@v2.0.2
go get charm.land/lipgloss/v2@v2.0.2
go get charm.land/glamour/v2@v2.0.0
go get charm.land/bubbles/v2@v2.0.0
go get charm.land/huh/v2@v2.0.3

# LLM provider SDKs (for multi-model)
go get github.com/anthropics/anthropic-sdk-go@v1.26.0
go get github.com/openai/openai-go@v3.28.0

# GCP client libraries
go get cloud.google.com/go/bigquery@v1.74.0
go get cloud.google.com/go/storage@v1.61.3
go get cloud.google.com/go/logging@v1.13.2
go get cloud.google.com/go/orchestration@v1.11.10
go get cloud.google.com/go/dataplex@v1.28.0

# Database
go get modernc.org/sqlite@v1.46.1

# CLI & Config
go get github.com/spf13/cobra@v1.10.2
go get github.com/BurntSushi/toml@v1.6.0

# Supporting
go get github.com/google/uuid@v1.6.0
go get golang.org/x/sync@v0.19.0
go get github.com/aymanbagabas/go-udiff@v0.2.0
go get github.com/bmatcuk/doublestar/v4@v4.8.1
go get github.com/alecthomas/chroma/v2@v2.15.0
```

```bash
# Dev tools
go install github.com/goreleaser/goreleaser/v2@v2.14.3
```

---

## Risk Assessment

### ADK Go Maturity (MEDIUM RISK)
ADK Go is at v0.6.0 -- pre-1.0 with monthly breaking releases (v0.2 through v0.6 in 5 months). The API is stabilizing but not guaranteed. **Mitigation:** Pin version, wrap ADK types behind internal interfaces, limit ADK surface area to runner + tool + model packages. The `model.LLM` interface has been stable since v0.2.

### Charm v2 Freshness (LOW RISK)
Bubble Tea v2.0.0 released Feb 24, 2026 -- less than a month old. Import paths changed from `github.com/charmbracelet/*` to `charm.land/*/v2`. Some community examples and tutorials still reference v1. **Mitigation:** Charm v2 is a coordinated release across the whole stack, well-documented with an upgrade guide. OpenCode still uses Charm v1 (as of its latest go.mod), but starting fresh on v2 is correct for a new project.

### genai Type Coupling (LOW RISK)
ADK's `model.LLM` interface uses `genai.Content` types. Non-Gemini providers need translation. **Mitigation:** The translation is mechanical (text parts, function calls, function responses map cleanly across providers). ~200-400 LOC per adapter. This is a known, bounded cost.

### GCP SDK Stability (LOW RISK)
All GCP Go client libraries are stable (v1.x) except `cloud.google.com/go/dataflow` (v0.11.1, pre-GA). BigQuery, Storage, Logging, Dataplex are all well-established. **Mitigation:** Dataflow is Phase 2+ anyway.

---

## Sources

All versions verified via direct tooling on 2026-03-16:

- ADK Go: `gh release list --repo google/adk-go` -- v0.6.0, 7.2K stars
- ADK Go go.mod: `gh api repos/google/adk-go/contents/go.mod` -- dependencies verified
- ADK Go source: `gh api` for model/llm.go, tool/tool.go, runner/runner.go, model/gemini/gemini.go, tool/functiontool/function.go
- OpenCode: `gh api repos/opencode-ai/opencode/contents/go.mod` -- reference architecture, 11.4K stars
- Bubble Tea: `gh release list --repo charmbracelet/bubbletea` -- v2.0.2
- Bubble Tea v2 release notes: `gh release view v2.0.0 --repo charmbracelet/bubbletea`
- Lip Gloss: `gh release list --repo charmbracelet/lipgloss` -- v2.0.2
- Glamour: `gh release list --repo charmbracelet/glamour` -- v2.0.0
- Bubbles: `gh release list --repo charmbracelet/bubbles` -- v2.0.0
- Huh: `gh release list --repo charmbracelet/huh` -- v2.0.3
- Charm v2 go.mod files: verified `charm.land/` import paths and Go version requirements
- GCP SDKs: `go list -m [module]@latest` for all packages
- modernc.org/sqlite: `go list -m modernc.org/sqlite@latest` -- v1.46.1
- ncruces/go-sqlite3: `gh repo view` -- v0.32.0, 936 stars
- Anthropic SDK: `gh repo view anthropics/anthropic-sdk-go` -- v1.26.0, 900 stars
- OpenAI SDK: `gh repo view openai/openai-go` -- v3.28.0, 3K stars
- MCP Go SDK: `gh release list --repo modelcontextprotocol/go-sdk` -- v1.4.1
- GenAI SDK: `go list -m google.golang.org/genai@latest` -- v1.50.0
- Cobra: `go list -m github.com/spf13/cobra@latest` -- v1.10.2
- BurntSushi/toml: `go list -m` -- v1.6.0
- GoReleaser: `gh release view --repo goreleaser/goreleaser` -- v2.14.3
- Go runtime: `go version` -- go1.26.1 darwin/arm64

# Technology Stack

**Project:** Cascade CLI -- AI-native terminal agent for GCP data engineering
**Researched:** 2026-03-16
**Overall Confidence:** HIGH (versions verified via `go list -m`, `gh release list`, and GitHub API)

## Decision Framework

Cascade is a single-binary AI terminal agent. Every dependency must satisfy: (1) pure Go or cgo-free for static cross-compilation, (2) actively maintained, (3) proven in production-grade CLI tools. OpenCode (11.4K stars, same domain) serves as the primary reference architecture for what works.

---

## Recommended Stack

### Language & Runtime

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.26+ (current: 1.26.1) | Language runtime | Sub-5ms startup, single binary, native concurrency, goroutines for subagents. Charm v2 requires Go 1.24.6+; Lip Gloss v2 requires Go 1.25+. Use latest stable. | HIGH |

### AI/LLM Framework

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Google ADK Go (`google.golang.org/adk`) | v0.6.0 | Agent framework: agent loop, tool dispatch, session management, MCP integration | Purpose-built agent framework from Google. Provides `model.LLM` interface, `runner.Runner` agent loop, `functiontool` for typed Go tool functions, `mcptoolset` for MCP servers, session/memory services, and tool confirmation (HITL). 7.2K stars, active monthly releases. Already includes MCP Go SDK integration. | HIGH |
| Google GenAI SDK (`google.golang.org/genai`) | v1.50.0 | Underlying Gemini API client (used by ADK) | ADK's dependency for Gemini model access. Supports both AI Studio and Vertex AI backends. Provides `genai.Content`, `genai.GenerateContentConfig` types that flow through the ADK `model.LLM` interface. | HIGH |

**Critical ADK Architecture Note:** ADK's `model.LLM` interface uses `genai.Content` and `genai.GenerateContentConfig` types from the Google GenAI SDK throughout. This means implementing non-Gemini providers (Claude, OpenAI) requires translating their native types to/from `genai.*` types. This is a deliberate tradeoff -- ADK provides the agent loop, tool dispatch, session management, and MCP integration for free, but non-Gemini support requires an adapter layer. OpenCode chose to build its own provider abstraction from scratch instead. For Cascade, ADK is the right choice because Gemini is the default model and the framework value (agent loop, tool confirmation, MCP) outweighs the adapter cost.

### LLM Provider SDKs (for multi-model support)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go`) | v1.26.0 | Claude model access | Official SDK, 900 stars. Needed for Claude provider adapter behind ADK's `model.LLM` interface. | HIGH |
| OpenAI Go SDK (`github.com/openai/openai-go`) | v3.28.0 | OpenAI/compatible model access | Official SDK, 3K stars. Covers OpenAI, Azure OpenAI, and any OpenAI-compatible endpoint (Ollama, vLLM, etc). | HIGH |

### TUI Framework (Charm v2 Stack)

| Technology | Version | Import Path | Purpose | Why | Confidence |
|------------|---------|-------------|---------|-----|------------|
| Bubble Tea | v2.0.2 | `charm.land/bubbletea/v2` | TUI framework (Elm architecture) | 40.7K stars. v2 ships the Cursed Renderer (ncurses-based), declarative view model, progressive keyboard enhancements, built-in color downsampling. Major release Feb 2026. | HIGH |
| Lip Gloss | v2.0.2 | `charm.land/lipgloss/v2` | Terminal styling | v2 is now pure (no I/O fighting with Bubble Tea), Bubble Tea manages I/O. Must use v2 with Bubble Tea v2. | HIGH |
| Glamour | v2.0.0 | `charm.land/glamour/v2` | Markdown rendering in terminal | Renders LLM markdown output. v2 released alongside Bubble Tea v2 for compatibility. | HIGH |
| Bubbles | v2.0.0 | `charm.land/bubbles/v2` | Pre-built TUI components (text input, viewport, spinner, list, table) | Standard components for input, scrollable output, loading states. v2 for Bubble Tea v2 compatibility. | HIGH |
| Huh | v2.0.3 | `charm.land/huh/v2` | Form/wizard framework | Powers the setup wizard, permission confirmation dialogs, config forms. v2 for ecosystem compatibility. | HIGH |

**Charm v2 Ecosystem Note:** The entire Charm stack moved to `charm.land/` vanity import paths in v2. All v2 packages must be used together -- do not mix v1 and v2 Charm packages. Bubble Tea v2 requires Go 1.24.6+; Lip Gloss/Bubbles/Huh require Go 1.25+.

### Database

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| modernc.org/sqlite | v1.46.1 | Schema cache, session storage, local state | Pure Go (C-to-Go transpiled), zero cgo. Enables static cross-compilation for all targets. FTS5 support for full-text schema search. ADK Go itself uses `glebarez/sqlite` (which wraps modernc.org/sqlite). Well-proven, actively maintained. | HIGH |

**Why not ncruces/go-sqlite3?** Also cgo-free (uses Wasm via wasm2go), 936 stars, active development, full FTS5 support. Viable alternative. However, modernc.org/sqlite is the ecosystem standard for pure-Go SQLite (used transitively by ADK Go itself), has broader community adoption, and eliminates the Wasm runtime overhead. Use modernc.org/sqlite unless you hit specific compilation issues.

**Why not mattn/go-sqlite3?** Requires cgo, which breaks static cross-compilation for the single-binary distribution goal.

### GCP Client Libraries

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `cloud.google.com/go/bigquery` | v1.74.0 | BigQuery queries, schema, jobs | Official Google client. Provides `bigquery.Client` for query execution, dry-run cost estimation, dataset/table listing, INFORMATION_SCHEMA access. | HIGH |
| `cloud.google.com/go/storage` | v1.61.3 | GCS operations (ls, head, read) | Official client for bucket listing, object metadata, content reading. | HIGH |
| `cloud.google.com/go/logging` | v1.13.2 | Cloud Logging queries | Official client for reading log entries, filtering by resource/severity/time. | HIGH |
| `cloud.google.com/go/orchestration` | v1.11.10 | Cloud Composer (managed Airflow) | Official client for Composer environments. Provides environment metadata, but DAG/task operations require Airflow REST API access through the Composer web server URL. | MEDIUM |
| `cloud.google.com/go/dataplex` | v1.28.0 | Dataplex metadata, data quality, tags | Official client for reading Dataplex tags (PII detection), data quality rules, catalog entries. | HIGH |
| `cloud.google.com/go/dataflow` | v0.11.1 | Dataflow job monitoring (Phase 2+) | Official client. Note v0.x version -- API is pre-GA. Out of scope for V1 per PROJECT.md but listed for future reference. | MEDIUM |
| `cloud.google.com/go/pubsub` | v1.50.1 | Pub/Sub topic/subscription info (Phase 2+) | Official client. Out of scope for V1 per PROJECT.md but listed for future reference. | HIGH |
| `google.golang.org/api` | v0.271.0 | Google API discovery-based clients | Fallback for services without dedicated Go clients. Also provides `option.WithCredentials` and auth utilities shared across all GCP clients. | HIGH |

**Composer/Airflow Integration Note:** The `cloud.google.com/go/orchestration` package handles Composer environment management (create, list, update), but DAG operations (list DAGs, trigger runs, get task logs, list failures) require hitting the Airflow REST API directly via the Composer web server URL. This means: (1) use orchestration SDK to discover the Composer environment's `airflowUri`, (2) use `net/http` with ADC-based auth to call the Airflow 2.x stable REST API. This is the standard pattern -- no SDK wraps Airflow's DAG API.

### CLI & Configuration

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Cobra (`github.com/spf13/cobra`) | v1.10.2 | CLI command framework | Industry standard. Used by ADK Go, kubectl, gh, docker. Handles subcommands, flags, completions, help generation. | HIGH |
| BurntSushi/toml (`github.com/BurntSushi/toml`) | v1.6.0 | TOML config parsing | Spec-compliant, zero dependencies, battle-tested. Parses `~/.cascade/config.toml` and `CASCADE.md` TOML frontmatter. Preferred over pelletier/go-toml for simplicity. | HIGH |

**Why not Viper?** OpenCode uses Viper (via spf13/viper), which adds environment variable binding, remote config, and watch support. Cascade's config model is simpler: one TOML file + CLI flags + env vars. BurntSushi/toml + manual env override is lighter and avoids Viper's transitive dependency tree. If config needs grow, migrate to Viper later.

### MCP (Model Context Protocol)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) | v1.4.1 | MCP server/client integration | Official SDK, maintained in collaboration with Google. ADK Go v0.6.0 already depends on this (uses `mcptoolset` package). Provides both server and client capabilities. | HIGH |

**Why not mark3labs/mcp-go?** This was the community SDK (OpenCode uses v0.17.0) before the official SDK existed. The official `modelcontextprotocol/go-sdk` is now the standard, actively maintained by the MCP project and Google, and already integrated into ADK Go. Use the official SDK.

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| `testing` (stdlib) | - | Unit tests | Go standard library. No framework needed. | HIGH |
| `github.com/google/go-cmp` | v0.7.0 | Deep equality comparison | Used by ADK Go itself. Better diff output than `reflect.DeepEqual`. | HIGH |
| `github.com/stretchr/testify` | v1.10.0 | Assertions and mocking | Used by OpenCode. Provides `assert`, `require`, `mock` packages. Optional -- stdlib `testing` is sufficient for most cases. | HIGH |

### Distribution

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| GoReleaser | v2.14.3 | Build, package, release | Cross-compiles for linux/darwin/windows x amd64/arm64, creates Homebrew formula, uploads GitHub releases. Industry standard for Go CLI distribution. | HIGH |

### Supporting Libraries

| Library | Version | Purpose | When to Use | Confidence |
|---------|---------|---------|-------------|------------|
| `github.com/google/uuid` | v1.6.0 | UUID generation | Session IDs, request IDs, tool call IDs | HIGH |
| `golang.org/x/sync` | v0.19.0 | Concurrency primitives | `errgroup` for parallel GCP API calls, `singleflight` for deduplication | HIGH |
| `golang.org/x/oauth2` | v0.34.0 | OAuth2/ADC auth | GCP Application Default Credentials flow | HIGH |
| `github.com/charmbracelet/x/ansi` | v0.11.6 | ANSI sequence handling | Terminal width detection, escape sequence stripping | HIGH |
| `github.com/aymanbagabas/go-udiff` | v0.2.0 | Unified diff generation | File Edit tool diff display | HIGH |
| `github.com/bmatcuk/doublestar/v4` | v4.8.1 | Glob pattern matching | Glob tool for file matching | HIGH |
| `github.com/lithammer/fuzzysearch` | v1.1.8 | Fuzzy string matching | Schema search, command matching | MEDIUM |
| `github.com/alecthomas/chroma/v2` | v2.15.0 | Syntax highlighting | SQL and code highlighting in terminal output | MEDIUM |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Agent Framework | ADK Go | Build from scratch (OpenCode pattern) | ADK provides agent loop, tool dispatch, session management, MCP integration, tool confirmation for free. Building from scratch means reimplementing all of this. The genai type coupling is manageable via adapter pattern. |
| Agent Framework | ADK Go | LangChain Go | LangChain Go is less mature, Python-centric design ported to Go, heavier abstraction. ADK is Google-native, Gemini-optimized, and has official MCP integration. |
| TUI | Charm v2 stack | tview/tcell | Charm has the modern ecosystem (markdown rendering, forms, styling), Elm architecture is cleaner for complex UIs. tview is imperative, lacks Glamour/Huh equivalents. |
| TUI | Charm v2 stack | Charm v1 stack | v1 is being superseded. v2 has the Cursed Renderer, declarative views, progressive keyboard enhancements. Starting a new project on v1 would require migration later. |
| SQLite | modernc.org/sqlite | mattn/go-sqlite3 | Requires cgo, breaks static cross-compilation. |
| SQLite | modernc.org/sqlite | ncruces/go-sqlite3 | Viable but less ecosystem adoption. Wasm translation adds small overhead. modernc.org is the default choice. |
| Config | BurntSushi/toml | Viper | Viper is overkill for single-file TOML config. Adds heavy dependency tree (remote config, fsnotify, etc). |
| Config | TOML | YAML | TOML is better for configuration files -- explicit types, no indentation ambiguity, good for nested sections like `[composer]`, `[dbt]`, `[security]`. |
| CLI | Cobra | Kong/urfave/cli | Cobra is the Go standard, used by ADK Go itself. Best completion support, best documentation. |
| MCP | Official Go SDK | mark3labs/mcp-go | Official SDK is the standard going forward, already integrated with ADK Go. Community SDK will likely converge or deprecate. |
| LLM SDK | Direct SDKs (Anthropic, OpenAI) | LiteLLM proxy | Adding a proxy adds deployment complexity. Direct SDKs keep it a single binary. Adapter pattern behind ADK's model.LLM interface is cleaner. |

---

## Critical Architecture Decision: ADK Go + Custom Provider Adapters

The most consequential stack decision is using ADK Go as the agent framework despite its `genai.*` type coupling.

**What ADK gives you for free:**
- Agent loop with streaming (`runner.Runner`)
- Typed function tools with JSON schema inference (`functiontool.New`)
- Tool confirmation / Human-in-the-Loop (`toolconfirmation`)
- MCP server integration (`mcptoolset`)
- Session management (in-memory, database, Vertex AI)
- Multi-agent composition (`workflowagents`)
- A2A protocol support
- OpenTelemetry tracing

**What you must build:**
- `model.LLM` adapters for Claude and OpenAI that translate `genai.Content` <-> native types
- Custom TUI integration (ADK's runner emits events you render in Bubble Tea)
- All GCP tools (BigQuery, Composer, GCS, Logging, etc.) as ADK `functiontool` implementations
- Permission engine wrapping ADK's tool confirmation
- Schema cache and context injection

**The adapter pattern:** Implement `model.LLM` interface for each provider. The interface is small (just `Name()` and `GenerateContent()`). The translation layer maps `genai.Content` (which has `Parts` containing `Text`, `FunctionCall`, `FunctionResponse`) to each provider's native message format. This is ~200-400 lines per provider, one-time cost.

---

## Version Compatibility Matrix

| Component | Min Go Version | Notes |
|-----------|---------------|-------|
| Go runtime | 1.26.1 | Current stable |
| ADK Go v0.6.0 | 1.24.4 | |
| Bubble Tea v2.0.2 | 1.24.6 | |
| Lip Gloss v2.0.2 | 1.25.0 | Sets the floor |
| Glamour v2.0.0 | 1.25.8 | |
| Huh v2.0.3 | 1.25.8 | Highest Charm requirement |
| GCP client libs | ~1.21+ | |

**Go version floor: 1.25.8** (driven by Glamour v2 and Huh v2). Go 1.26.1 satisfies all requirements.

---

## Installation

```bash
# Initialize module
go mod init github.com/your-org/cascade

# Core: ADK Go agent framework
go get google.golang.org/adk@v0.6.0

# TUI: Charm v2 stack
go get charm.land/bubbletea/v2@v2.0.2
go get charm.land/lipgloss/v2@v2.0.2
go get charm.land/glamour/v2@v2.0.0
go get charm.land/bubbles/v2@v2.0.0
go get charm.land/huh/v2@v2.0.3

# LLM provider SDKs (for multi-model)
go get github.com/anthropics/anthropic-sdk-go@v1.26.0
go get github.com/openai/openai-go@v3.28.0

# GCP client libraries
go get cloud.google.com/go/bigquery@v1.74.0
go get cloud.google.com/go/storage@v1.61.3
go get cloud.google.com/go/logging@v1.13.2
go get cloud.google.com/go/orchestration@v1.11.10
go get cloud.google.com/go/dataplex@v1.28.0

# Database
go get modernc.org/sqlite@v1.46.1

# CLI & Config
go get github.com/spf13/cobra@v1.10.2
go get github.com/BurntSushi/toml@v1.6.0

# Supporting
go get github.com/google/uuid@v1.6.0
go get golang.org/x/sync@v0.19.0
go get github.com/aymanbagabas/go-udiff@v0.2.0
go get github.com/bmatcuk/doublestar/v4@v4.8.1
go get github.com/alecthomas/chroma/v2@v2.15.0
```

```bash
# Dev tools
go install github.com/goreleaser/goreleaser/v2@v2.14.3
```

---

## Risk Assessment

### ADK Go Maturity (MEDIUM RISK)
ADK Go is at v0.6.0 -- pre-1.0 with monthly breaking releases (v0.2 through v0.6 in 5 months). The API is stabilizing but not guaranteed. **Mitigation:** Pin version, wrap ADK types behind internal interfaces, limit ADK surface area to runner + tool + model packages. The `model.LLM` interface has been stable since v0.2.

### Charm v2 Freshness (LOW RISK)
Bubble Tea v2.0.0 released Feb 24, 2026 -- less than a month old. Import paths changed from `github.com/charmbracelet/*` to `charm.land/*/v2`. Some community examples and tutorials still reference v1. **Mitigation:** Charm v2 is a coordinated release across the whole stack, well-documented with an upgrade guide. OpenCode still uses Charm v1 (as of its latest go.mod), but starting fresh on v2 is correct for a new project.

### genai Type Coupling (LOW RISK)
ADK's `model.LLM` interface uses `genai.Content` types. Non-Gemini providers need translation. **Mitigation:** The translation is mechanical (text parts, function calls, function responses map cleanly across providers). ~200-400 LOC per adapter. This is a known, bounded cost.

### GCP SDK Stability (LOW RISK)
All GCP Go client libraries are stable (v1.x) except `cloud.google.com/go/dataflow` (v0.11.1, pre-GA). BigQuery, Storage, Logging, Dataplex are all well-established. **Mitigation:** Dataflow is Phase 2+ anyway.

---

## Sources

All versions verified via direct tooling on 2026-03-16:

- ADK Go: `gh release list --repo google/adk-go` -- v0.6.0, 7.2K stars
- ADK Go go.mod: `gh api repos/google/adk-go/contents/go.mod` -- dependencies verified
- ADK Go source: `gh api` for model/llm.go, tool/tool.go, runner/runner.go, model/gemini/gemini.go, tool/functiontool/function.go
- OpenCode: `gh api repos/opencode-ai/opencode/contents/go.mod` -- reference architecture, 11.4K stars
- Bubble Tea: `gh release list --repo charmbracelet/bubbletea` -- v2.0.2
- Bubble Tea v2 release notes: `gh release view v2.0.0 --repo charmbracelet/bubbletea`
- Lip Gloss: `gh release list --repo charmbracelet/lipgloss` -- v2.0.2
- Glamour: `gh release list --repo charmbracelet/glamour` -- v2.0.0
- Bubbles: `gh release list --repo charmbracelet/bubbles` -- v2.0.0
- Huh: `gh release list --repo charmbracelet/huh` -- v2.0.3
- Charm v2 go.mod files: verified `charm.land/` import paths and Go version requirements
- GCP SDKs: `go list -m [module]@latest` for all packages
- modernc.org/sqlite: `go list -m modernc.org/sqlite@latest` -- v1.46.1
- ncruces/go-sqlite3: `gh repo view` -- v0.32.0, 936 stars
- Anthropic SDK: `gh repo view anthropics/anthropic-sdk-go` -- v1.26.0, 900 stars
- OpenAI SDK: `gh repo view openai/openai-go` -- v3.28.0, 3K stars
- MCP Go SDK: `gh release list --repo modelcontextprotocol/go-sdk` -- v1.4.1
- GenAI SDK: `go list -m google.golang.org/genai@latest` -- v1.50.0
- Cobra: `go list -m github.com/spf13/cobra@latest` -- v1.10.2
- BurntSushi/toml: `go list -m` -- v1.6.0
- GoReleaser: `gh release view --repo goreleaser/goreleaser` -- v2.14.3
- Go runtime: `go version` -- go1.26.1 darwin/arm64

