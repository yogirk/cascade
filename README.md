# Cascade

AI-native terminal agent for GCP data engineering. Think Claude Code, but for BigQuery, Airflow, dbt, and your GCP data platform.

## What It Does

Cascade is a conversational CLI that understands your data warehouse, pipelines, and GCP infrastructure. Ask questions, run queries, diagnose pipeline failures, and manage your data stack — all from the terminal.

## Status

**Early development** — foundational agent loop, TUI, and tool system are in place. GCP-specific tools (BigQuery, Composer, dbt) coming next.

## Quick Start

```bash
# Requires Go 1.26+
go install github.com/yogirk/cascade/cmd/cascade@latest

# Set up auth (Gemini API key or GCP ADC)
export GOOGLE_API_KEY="your-key"

# Interactive mode
cascade

# One-shot mode
cascade -p "explain what this query does"
```

## Features

- Streaming conversational interface (Bubble Tea TUI)
- Tool system: Read, Write, Edit, Glob, Grep, Bash
- Permission engine with risk classification (read-only / confirm / bypass)
- GCP auth via API key or Application Default Credentials
- One-shot mode for scripting (`cascade -p "..."`)
- Configurable via `~/.cascade/config.toml`

## Configuration

```toml
# ~/.cascade/config.toml
[model]
provider = "vertex"          # "gemini" or "vertex"
model = "gemini-2.5-pro"
project = "my-gcp-project"

[security]
default_mode = "confirm"     # "confirm", "plan", or "bypass"
```

## License

MIT
