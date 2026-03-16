# BQ Core Testing Tasks

Manual testing checklist using the `hacker_news` dataset.
Track progress by marking items as they're tested.

## Prerequisites

```toml
# ~/.cascade/config.toml
[gcp]
project = "cloudside-academy"

[bigquery]
datasets = ["hacker_news"]

[cost]
warn_threshold = 1.0
max_query_cost = 10.0
daily_budget_usd = 50.0
```

The `hacker_news` dataset is a copy in your own project (`cloudside-academy`). Key table:
- `full` — ~47M rows, ~18.8 GB, non-partitioned (contains stories, comments, polls, etc.)
  - Columns: title, url, text, dead, by, score, time, timestamp, type, id, parent, descendants, ranking, deleted

> **Note:** Queries reference `hacker_news.full` (your project, not `bigquery-public-data`).

---

## 1. Schema Cache & Sync

### 1.1 First-run lazy cache build
- [x] **Test:** Launch Cascade fresh (delete `~/.cascade/cache/*.db` first). Ask "what tables are in hacker_news?"
- [x] **Expect:** Spinner shows "Building schema cache..." with progress. After build completes, answer includes table list.
- [x] **Watch for:** Does the TUI block during cache build? It should NOT — you should be able to type while it builds.
- **Result:** PASS — status bar shows progress, cache builds in background, table list returned correctly (full: 47M rows, 17.6 GB)

### 1.2 /sync command
- [x] **Test:** Run `/sync`
- [x] **Expect:** Progress feedback ("Syncing schema..."), completion message with table count.
- [x] **Watch for:** Does the spinner show? Does it report how many tables were cached?
- **Result:** PASS

### 1.3 /sync with dataset argument
- [x] **Test:** Run `/sync hacker_news`
- [x] **Expect:** Syncs only the specified dataset, shows completion with count.
- **Result:** PASS

### 1.4 Cache persistence
- [x] **Test:** Exit and relaunch Cascade. Ask "describe the full table"
- [x] **Expect:** Immediate response (no rebuild) — cache loaded from SQLite on disk.
- **Result:** PASS

---

## 2. Schema Exploration

### 2.1 List datasets
- [x] **Test:** "What datasets do I have?"
- [x] **Expect:** Shows `hacker_news` with table count and total size. Styled output, not raw text.
- **Result:** PASS

### 2.2 List tables
- [x] **Test:** "Show me the tables in hacker_news"
- [x] **Expect:** Table list with columns: Table, Type, Rows, Size, Partition.
- **Result:** PASS

### 2.3 Describe table — partitioned
- [x] **Test:** "Describe the full table"
- [x] **Expect:** Shows all columns with types, nullable status. Row count and size displayed.
- **Result:** PASS

### 2.4 Describe table — non-partitioned
- [x] **Test:** "Describe the full table in hacker_news"
- [x] **Expect:** Same format but no partition/clustering info shown.
- **Result:** PASS

### 2.5 Search columns
- [x] **Test:** "Find columns related to 'score' across all tables"
- [x] **Expect:** FTS5 search returns matching columns with dataset.table.column paths.
- **Result:** PASS

### 2.6 Search columns — no results
- [x] **Test:** "Search for columns named 'zzz_nonexistent'"
- [x] **Expect:** Clean "No columns matching" message, not an error.
- **Result:** PASS

---

## 3. Query Execution & Results

### 3.1 Simple SELECT
- [x] **Test:** "Show me the top 10 stories by score"
- [x] **Expect:** Results in styled Lipgloss table with cost/duration footer.
- **Result:** PASS

### 3.2 Aggregation query
- [x] **Test:** "How many stories were posted each year?"
- [x] **Expect:** Aggregation with EXTRACT(YEAR FROM timestamp), GROUP BY, ORDER BY. Clean table output.
- **Result:** PASS

### 3.3 Query with many rows
- [x] **Test:** Run a query returning >50 rows
- [x] **Expect:** Results truncated at 50 rows (default), "N more rows" indicator shown. Cost/duration footer present.
- **Result:** PASS

### 3.4 Query returning no rows
- [x] **Test:** Query with impossible filter
- [x] **Expect:** Clean empty result message, not a crash.
- **Result:** PASS

### 3.5 Invalid SQL
- [x] **Test:** Trigger a SQL syntax error
- [x] **Expect:** BQ error shown and fed back to LLM for self-correction.
- **Result:** PASS — LLM self-corrects (e.g., unquoted `by` column, TIMESTAMP_SUB with YEAR)

### 3.6 Multi-statement query
- [x] **Test:** CTE query (WITH ... SELECT)
- [x] **Expect:** CTE executes correctly, results displayed in table format.
- **Result:** PASS

---

## 4. Cost Estimation & Guards

### 4.1 Dry-run cost display
- [x] **Test:** "How much would it cost to query all of hacker_news.full?"
- [x] **Expect:** LLM uses bigquery_query with dry_run=true. Shows estimated cost and bytes to scan without executing.
- **Result:** PASS

### 4.2 Cost shown on every query
- [x] **Test:** Run any SELECT query and check the footer
- [x] **Expect:** Footer shows `$X.XX · X.Xs · X.X GB scanned` after every query execution.
- **Result:** PASS

### 4.3 Warn threshold trigger
- [x] **Test:** Set `warn_threshold = 0.001` in config. Run a full table scan.
- [x] **Expect:** Permission escalation — Cascade asks for confirmation before running the expensive query.
- **Result:** PASS

### 4.4 Max cost block
- [x] **Test:** Set `max_query_cost = 0.001` in config. Run a full table scan.
- [x] **Expect:** Query is BLOCKED with message about exceeding maximum cost.
- **Result:** PASS

### 4.5 DML cost estimation
- [x] **Test:** Ask it to dry-run an INSERT statement
- [x] **Expect:** "Cost: cannot estimate for DML (syntax valid)" message.
- **Result:** PASS

---

## 5. Status Bar & Session Cost

### 5.1 Cost appears in status bar
- [x] **Test:** Run 2-3 SELECT queries and watch the status bar
- [x] **Expect:** Running cost total updates after each query (e.g., "$0.02").
- **Result:** PASS

### 5.2 /cost command
- [x] **Test:** After running a few queries, type `/cost`
- [x] **Expect:** Detailed breakdown table: each query's SQL snippet, bytes scanned, cost, duration. Session total at bottom.
- **Result:** PASS

### 5.3 Budget warning
- [x] **Test:** Set `daily_budget_usd = 0.01` in config. Run queries until session total exceeds 80%.
- [x] **Expect:** Budget alert message appears.
- **Result:** PASS

---

## 6. Natural Language to SQL

### 6.1 Simple NL query
- [x] **Test:** "What are the most popular stories about Rust programming?"
- [x] **Expect:** Generates SQL filtering on title/text, orders by score.
- **Result:** PASS

### 6.2 NL with time filter
- [x] **Test:** "Show me the top authors from 2023"
- [x] **Expect:** SQL uses EXTRACT or date filter on timestamp.
- **Result:** PASS

### 6.3 NL with JOIN
- [x] **Test:** "Show me stories that have the most comments"
- [x] **Expect:** Generates valid SQL using subquery or aggregation on descendants column.
- **Result:** PASS

### 6.4 NL accuracy — correct column names
- [x] **Test:** "What's the average score of stories by the author 'pg'?"
- [x] **Expect:** Uses correct columns: `score`, `by` (backtick-quoted). No hallucinated columns.
- **Result:** PASS

### 6.5 NL — ambiguous request
- [x] **Test:** "Show me everything about HN"
- [x] **Expect:** LLM picks a reasonable approach, not a blind SELECT * on 47M rows.
- **Result:** PASS

### 6.6 Schema context injection
- [x] **Test:** After cache build, ask "what columns does the full table have?"
- [x] **Expect:** LLM answers from schema context or uses bigquery_schema tool.
- **Result:** PASS

---

## 7. SQL Optimization Hints

### 7.1 Missing partition filter
- [x] **Test:** Run a full scan on a non-partitioned table
- [x] **Expect:** No false partition hint (table isn't partitioned).
- **Result:** PASS

### 7.2 Partition filter present — no hint
- [x] **Test:** Query with timestamp filter
- [x] **Expect:** No optimization hint triggered.
- **Result:** PASS

### 7.3 Unused clustering key
- [x] **Test:** Query that doesn't use clustering columns
- [x] **Expect:** Info hint about unused clustering keys (if applicable).
- **Result:** PASS — N/A for `full` table (no clustering), no false hints

### 7.4 Non-partitioned table — no false hint
- [x] **Test:** `SELECT * FROM hacker_news.full LIMIT 10`
- [x] **Expect:** NO partition hint (table isn't partitioned).
- **Result:** PASS

---

## 8. SQL Risk Classification & Permissions

### 8.1 SELECT is read-only
- [x] **Test:** In "ask" permission mode, run a SELECT query
- [x] **Expect:** Executes without asking permission (read-only).
- **Result:** PASS

### 8.2 DML requires approval
- [x] **Test:** Ask Cascade to insert a row
- [x] **Expect:** Permission prompt appears (DML risk level).
- **Result:** PASS

### 8.3 DDL requires approval
- [x] **Test:** Ask to create a temporary table
- [x] **Expect:** Permission prompt with DDL risk level.
- **Result:** PASS

### 8.4 Destructive blocked/warned
- [x] **Test:** Ask to drop a table
- [x] **Expect:** Strong warning or block — destructive risk level.
- **Result:** PASS

### 8.5 CTE classified as read-only
- [x] **Test:** Run a WITH...SELECT query
- [x] **Expect:** No permission prompt (WITH is classified as READ_ONLY).
- **Result:** PASS

---

## 9. Context Compaction

### 9.1 /compact command
- [x] **Test:** After several interactions, type `/compact`
- [x] **Expect:** "Context compacted" message appears. Conversation continues working.
- **Result:** PASS

### 9.2 Auto-compaction
- [x] **Test:** Run many queries in a single session to fill context
- [x] **Expect:** At ~80% context usage, auto-compaction triggers.
- **Result:** PASS

---

## 10. Edge Cases & Error Handling

### 10.1 No GCP credentials
- [x] **Test:** Unset GCP credentials, launch Cascade, ask a BQ question
- [x] **Expect:** Clear error, not a panic/crash.
- **Result:** PASS

### 10.2 Wrong project ID
- [x] **Test:** Set `gcp.project = "nonexistent-project"` in config, try to query
- [x] **Expect:** Clear BQ error about project access. Not a crash.
- **Result:** PASS

### 10.3 Empty dataset config
- [x] **Test:** Set `datasets = []` in config, run `/sync`
- [x] **Expect:** Graceful message about no datasets configured.
- **Result:** PASS

### 10.4 Network interruption during query
- [x] **Test:** Start a long-running query, then disconnect wifi
- [x] **Expect:** Timeout or connection error message. TUI stays responsive.
- **Result:** PASS

### 10.5 Very wide results
- [x] **Test:** `SELECT * FROM hacker_news.full LIMIT 5`
- [x] **Expect:** Table renders without breaking the terminal. Columns truncated sensibly.
- **Result:** PASS

### 10.6 Query with NULL values
- [x] **Test:** Query rows with NULL fields
- [x] **Expect:** NULLs rendered cleanly as "NULL" text.
- **Result:** PASS

### 10.7 Unicode/special characters in results
- [x] **Test:** Query rows with unicode content
- [x] **Expect:** Unicode renders correctly, table alignment not broken.
- **Result:** PASS

---

## 11. Slash Commands

### 11.1 /sync
- [x] **Status:** Covered in section 1
- **Result:** PASS

### 11.2 /cost
- [x] **Status:** Covered in section 5
- **Result:** PASS

### 11.3 /compact
- [x] **Status:** Covered in section 9
- **Result:** PASS

### 11.4 Unknown slash command
- [x] **Test:** Type `/foobar`
- [x] **Expect:** Helpful error message listing available commands.
- **Result:** PASS

---

## 12. Rendering & UX Polish

### 12.1 Table styling
- [x] **Test:** Run any query and visually inspect the result table
- [x] **Expect:** Bold blue headers, alternating row colors, no borders (conversational style), padding between columns.
- **Result:** PASS

### 12.2 Cost footer styling
- [x] **Test:** Check footer after any query
- [x] **Expect:** Dimmed gray text: `$0.00 · 245ms · 1.2 GB scanned`
- **Result:** PASS

### 12.3 Overflow indicator
- [x] **Test:** Query returning >50 rows
- [x] **Expect:** Dimmed "N more rows" text below the table.
- **Result:** PASS

### 12.4 Schema detail formatting
- [x] **Test:** Describe a table with many columns
- [x] **Expect:** Clean column table with partition/clustering annotations in description column.
- **Result:** PASS

### 12.5 Optimization hints styling
- [x] **Test:** Trigger a partition filter warning
- [x] **Expect:** Amber "Optimization Suggestions" header, [WARN] and [INFO] prefixes on hints.
- **Result:** PASS

---

## Progress Summary

| Section | Tests | Passed | Failed | Blocked |
|---------|-------|--------|--------|---------|
| 1. Schema Cache & Sync | 4 | 4 | 0 | 0 |
| 2. Schema Exploration | 6 | 6 | 0 | 0 |
| 3. Query Execution | 6 | 6 | 0 | 0 |
| 4. Cost Estimation | 5 | 5 | 0 | 0 |
| 5. Status Bar & Cost | 3 | 3 | 0 | 0 |
| 6. NL to SQL | 6 | 6 | 0 | 0 |
| 7. Optimization Hints | 4 | 4 | 0 | 0 |
| 8. Risk & Permissions | 5 | 5 | 0 | 0 |
| 9. Context Compaction | 2 | 2 | 0 | 0 |
| 10. Edge Cases | 7 | 7 | 0 | 0 |
| 11. Slash Commands | 1 | 1 | 0 | 0 |
| 12. Rendering & UX | 5 | 5 | 0 | 0 |
| **Total** | **54** | **54** | **0** | **0** |
