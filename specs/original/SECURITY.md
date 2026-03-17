# Cascade CLI — Security & Permissions

## Defense-in-Depth Model

Cascade implements four layers of security, combining Claude Code's proven permission model with GCP-native governance.

```
Layer 1: GCP IAM (what your credentials can access)
    │
Layer 2: Cascade Permission Engine (what the agent can do)
    │
Layer 3: OS Sandbox (filesystem and network isolation)
    │
Layer 4: Cost Gates (financial guardrails)
```

---

## Layer 1: GCP IAM Integration

Cascade operates within the permissions of the authenticated GCP identity. It cannot escalate privileges.

### Authentication
```
Priority order:
1. Service account key (GOOGLE_APPLICATION_CREDENTIALS env var)
2. Application Default Credentials (gcloud auth application-default login)
3. gcloud CLI credentials (gcloud auth login)
4. Workload Identity (GKE/Cloud Run environments)
5. Compute Engine metadata (VM environments)
```

### Service Account Impersonation
For production use, Cascade supports impersonating a service account with limited permissions:

```toml
# ~/.cascade/config.toml
[auth]
impersonate_service_account = "cascade-agent@my-project.iam.gserviceaccount.com"
```

This way, even if the user has Owner permissions, Cascade operates with only the permissions granted to the service account.

### Recommended IAM Roles for Cascade

| Role | Purpose |
|------|---------|
| `roles/bigquery.dataViewer` | Read schemas and query data |
| `roles/bigquery.jobUser` | Execute queries |
| `roles/bigquery.dataEditor` | CREATE/INSERT/UPDATE (optional, for mutations) |
| `roles/composer.user` | View DAGs and trigger runs |
| `roles/dataflow.viewer` | View Dataflow jobs |
| `roles/storage.objectViewer` | Read GCS objects |
| `roles/logging.viewer` | Read Cloud Logging |
| `roles/dataplex.viewer` | Read Dataplex catalog and lineage |

---

## Layer 2: Cascade Permission Engine

### Risk Classification

Every tool call is classified before execution:

#### File Operations
| Risk | Operations | Default |
|------|-----------|---------|
| SAFE | Read, Glob, Grep | Auto-approve |
| LOW | Write new file | Auto-approve |
| MEDIUM | Edit existing file | Prompt in Confirm mode |
| HIGH | Delete file, overwrite | Always prompt |

#### Bash Commands
| Risk | Examples | Default |
|------|---------|---------|
| SAFE | `ls`, `cat`, `echo`, `git status`, `dbt compile` | Auto-approve |
| LOW | `git diff`, `dbt run --target dev`, `terraform plan` | Auto-approve |
| MEDIUM | `dbt run --target prod`, `gcloud ...` | Prompt |
| HIGH | `rm`, `terraform apply`, `gcloud ... delete` | Always prompt |
| CRITICAL | `rm -rf`, `terraform destroy`, `gsutil rm -r` | Double prompt |

#### SQL Operations
| Risk | SQL Types | Default |
|------|----------|---------|
| READ_ONLY | SELECT, SHOW, DESCRIBE, EXPLAIN | Auto-approve |
| DDL | CREATE, ALTER (additive) | Prompt |
| DML | INSERT, UPDATE, DELETE, MERGE | Prompt |
| DESTRUCTIVE | DROP, TRUNCATE, ALTER (destructive) | Always prompt |
| ADMIN | GRANT, REVOKE, CREATE ROLE | Always prompt |

#### GCP Operations
| Risk | Operations | Default |
|------|-----------|---------|
| READ | List, describe, status checks | Auto-approve |
| TRIGGER | Trigger DAG, start job | Prompt |
| MODIFY | Update config, deploy | Prompt |
| DELETE | Delete resource, drain job | Always prompt |

### Permission Caching

Approved permissions can be cached to reduce prompt fatigue:

```json
// ~/.cascade/permissions.json
{
    "rules": [
        {
            "scope": "project:my-analytics",
            "tool": "BigQueryQuery",
            "risk": "READ_ONLY",
            "decision": "allow",
            "expires": "session"
        },
        {
            "scope": "project:my-analytics",
            "tool": "DbtTool",
            "action": "run",
            "target": "dev",
            "decision": "allow",
            "expires": "never"
        },
        {
            "scope": "global",
            "tool": "Bash",
            "pattern": "terraform apply*",
            "decision": "ask",
            "expires": "never"
        }
    ]
}
```

### Blocklist (Never Allowed)

These operations are blocked regardless of permission mode:

```
- DROP DATABASE / DROP SCHEMA (without explicit --allow-drop flag)
- TRUNCATE on tables > 1M rows (without explicit approval)
- gsutil rm -r gs:// (recursive bucket delete)
- terraform destroy (without explicit --allow-destroy flag)
- Any command that modifies IAM policies
- Any command that disables audit logging
- Sending credentials/tokens to external services
```

---

## Layer 3: OS Sandbox

### macOS (sandbox-exec)
```
deny default
allow file-read* (project directory, GCP credentials, cache)
allow file-write* (project directory, cascade cache)
allow network* (*.googleapis.com, configured MCP servers)
deny process-exec* (except allowlisted: git, dbt, terraform, gcloud, bq, gsutil)
```

### Linux (bubblewrap)
```
Namespace isolation:
- Mount: read-only root, read-write project dir and cache
- Network: proxy through cascade for domain allowlisting
- PID: isolated process namespace
- User: unprivileged user mapping
```

### Network Allowlist (Default)

```
# GCP APIs
*.googleapis.com
*.google.com
accounts.google.com

# Package registries (for dbt/terraform plugins)
registry.terraform.io

# MCP servers (user-configured)
# Added per-server with user approval on first connection
```

---

## Layer 4: Cost Gates

Financial guardrails to prevent accidental expensive operations.

### Pre-Execution Cost Estimation

Every SQL query runs a dry-run first:

```go
// Automatic before every BigQuery query
func (t *BigQueryTool) estimateCost(ctx context.Context, sql string) (float64, error) {
    q := t.client.Query(sql)
    q.DryRun = true
    job, err := q.Run(ctx)
    if err != nil { return 0, err }
    stats := job.LastStatus().Statistics
    estimatedBytes := stats.TotalBytesProcessed
    estimatedCost := float64(estimatedBytes) / 1e12 * 6.25 // on-demand pricing

    switch {
    case estimatedCost > t.config.Cost.MaxQueryCostUSD:
        return estimatedCost, ErrCostLimitExceeded{estimatedCost, t.config.Cost.MaxQueryCostUSD}
    case estimatedCost > t.config.Cost.WarnQueryCostUSD:
        t.promptUserWithCostWarning(estimatedCost)
    }
    return estimatedCost, nil
}
```

### Session Cost Tracking

```
cascade> /cost session

  Session cost summary (this session):
  ┌──────────────────┬──────────┬──────────────┐
  │ Operation        │ Count    │ Cost (est.)  │
  ├──────────────────┼──────────┼──────────────┤
  │ BigQuery queries │ 12       │ $3.47        │
  │ LLM tokens       │ 145K     │ $2.18        │
  │ Total            │          │ $5.65        │
  └──────────────────┴──────────┴──────────────┘
```

### Daily Budget Enforcement

```toml
[cost]
daily_budget_usd = 100.0
budget_action = "warn"  # warn | soft_block | hard_block
```

- `warn`: Show warning but allow execution
- `soft_block`: Require explicit confirmation for each operation
- `hard_block`: Block all cost-incurring operations until next day

---

## PII Handling

### Detection
Cascade detects PII through:
1. Dataplex tags (authoritative — from your governance policies)
2. Column name heuristics (email, phone, ssn, address, etc.)
3. Data pattern matching (email regex, phone patterns, SSN format)

### Display Behavior

```toml
[security]
mask_pii_in_output = true
```

When enabled:
- PII-tagged columns show `[REDACTED]` in query result displays
- Data profiler shows statistics but not actual values for PII columns
- The agent warns before generating queries that SELECT PII columns
- Export formats (JSON, CSV) respect masking settings

### Audit Trail

All operations are logged locally for compliance:

```json
// ~/.cascade/audit.log (append-only)
{
    "timestamp": "2026-02-05T10:30:00Z",
    "session_id": "abc123",
    "tool": "BigQueryQuery",
    "action": "SELECT",
    "tables_accessed": ["warehouse.raw_orders"],
    "pii_columns_accessed": ["customer_email"],
    "estimated_cost_usd": 0.43,
    "actual_cost_usd": 0.41,
    "user": "rk@company.com",
    "approved_by": "user_prompt",
    "result_rows": 47
}
```
