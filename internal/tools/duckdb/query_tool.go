package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	duck "github.com/slokam-ai/cascade/internal/duckdb"
	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/internal/tools"
)

// querySource binds an alias to a list of gs://… URLs that the agent
// wants to query directly, without first staging into the local session
// DB. Globs (`*`, `?`, hive-partitioned `year=*`) are expanded server-
// side via the GCS list API; the resolved https URLs are passed to
// read_parquet([…]) which Cascade substitutes into the SQL where the
// alias appears as `${alias}`.
//
// We chose alias substitution over textual gs:// rewriting for the
// reason the design's reviewer flagged: scanning arbitrary SQL for
// `gs://` tokens is fragile inside CTEs, subqueries, and string
// literals. `${alias}` is unambiguous.
type querySource struct {
	Alias    string   `json:"alias"`
	GCSPaths []string `json:"gcs_paths"`
}

type queryInput struct {
	SQL     string        `json:"sql"`
	Sources []querySource `json:"sources,omitempty"`
	MaxRows int           `json:"max_rows,omitempty"`
}

// QueryTool runs a SQL query against the local DuckDB session DB and/or
// gs://*.parquet sources via httpfs. Permission risk is classified from
// the SQL's leading verb (read-only / DML / DDL / destructive / admin)
// just like the BigQuery tool — the difference is the volume gate
// instead of the cost gate, and the volume gate only fires on
// bq_to_duckdb (not here).
type QueryTool struct {
	runtime  duck.Runtime
	session  *duck.Session
	gcs      *duck.GCSAuth // nil = no GCS auth available; gs:// sources error
}

// NewQueryTool wires a query tool. gcs may be nil when GCP auth is
// unavailable; in that case any input that includes Sources is rejected
// with a clear message.
func NewQueryTool(runtime duck.Runtime, session *duck.Session, gcs *duck.GCSAuth) *QueryTool {
	return &QueryTool{runtime: runtime, session: session, gcs: gcs}
}

func (t *QueryTool) Name() string { return "duckdb_query" }

func (t *QueryTool) Description() string {
	return "Execute a SQL query against the local DuckDB session DB and/or gs://*.parquet sources via httpfs. " +
		"For local queries, just pass the sql. To query a Parquet path on GCS without staging, declare a source: " +
		`{"alias":"hn","gcs_paths":["gs://b/year=*/file.parquet"]} and reference it in the SQL as ${hn}. ` +
		"Globs are expanded by Cascade before the SQL runs. Read-only by default; DDL/DML escalate to a confirmation prompt."
}

func (t *QueryTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"sql": map[string]any{
				"type":        "string",
				"description": "The SQL statement. References to declared sources use ${alias}.",
			},
			"sources": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"alias":     map[string]any{"type": "string"},
						"gcs_paths": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
					"required": []string{"alias", "gcs_paths"},
				},
				"description": "Optional. gs:// sources bound to ${alias} placeholders in the SQL.",
			},
			"max_rows": map[string]any{
				"type":        "integer",
				"description": "Maximum rows to return. Default 1000, hard cap 10000 (the design's agent-context guard).",
			},
		},
		"required": []string{"sql"},
	}
}

// RiskLevel returns a worst-case stub. PlanPermission classifies the
// real SQL risk before the agent loop applies the gate.
func (t *QueryTool) RiskLevel() permission.RiskLevel { return permission.RiskDestructive }

// PlanPermission classifies the SQL using the DuckDB classifier and
// returns the resolved risk. Mirrors internal/tools/bigquery's
// PlanPermission shape — the BQ tool also classifies dynamically and
// downgrades read-only SQL from the destructive stub.
func (t *QueryTool) PlanPermission(ctx context.Context, input json.RawMessage, baseRisk permission.RiskLevel) (*tools.PermissionPlan, error) {
	var p queryInput
	if err := json.Unmarshal(input, &p); err != nil {
		return nil, nil
	}
	risk := baseRisk
	if p.SQL != "" {
		risk = duck.ClassifySQLRisk(p.SQL)
	}
	if risk > permission.RiskReadOnly {
		return &tools.PermissionPlan{RiskOverride: &risk}, nil
	}
	r := permission.RiskReadOnly
	return &tools.PermissionPlan{RiskOverride: &r}, nil
}

// MaxAgentRows is the design's agent-context guard. A query result
// landing in the LLM context above this size pushes out everything
// the agent already knows; the truncation marker prompts it to
// re-issue with an aggregation or sample.
const MaxAgentRows = 10000

func (t *QueryTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var p queryInput
	if err := json.Unmarshal(input, &p); err != nil {
		return errResult(fmt.Sprintf("invalid input: %v", err)), nil
	}
	if strings.TrimSpace(p.SQL) == "" {
		return errResult("sql is required and cannot be empty"), nil
	}
	if t.runtime == nil || t.session == nil {
		return errResult("DuckDB runtime not configured"), nil
	}

	maxRows := p.MaxRows
	if maxRows <= 0 {
		maxRows = 1000
	}
	if maxRows > MaxAgentRows {
		maxRows = MaxAgentRows
	}

	// Resolve sources (gs:// → expanded URL list → read_parquet call →
	// substitution into the SQL placeholder). When there are no sources,
	// the SQL runs against the local session DB unchanged.
	resolvedSQL, init, err := t.resolveSources(ctx, p.SQL, p.Sources)
	if err != nil {
		return errResult(err.Error()), nil
	}

	risk := duck.ClassifySQLRisk(resolvedSQL)

	// Lock policy: writes serialize, reads run in parallel. Same shape
	// as a typical SQL connection pool, scoped to this Cascade process.
	if risk > permission.RiskReadOnly {
		t.session.Lock()
		defer t.session.Unlock()
	} else {
		t.session.RLock()
		defer t.session.RUnlock()
	}

	start := time.Now()

	if risk > permission.RiskReadOnly {
		_, err := t.runtime.Exec(ctx, duck.ExecOptions{
			Database: t.session.Path,
			Init:     init,
			SQL:      resolvedSQL,
		})
		if err != nil {
			return errResult(formatRuntimeErr("query", err)), nil
		}
		ms := time.Since(start).Milliseconds()
		msg := fmt.Sprintf("Statement completed in %s.", durationStr(ms))
		return &tools.Result{Content: msg, Display: msg}, nil
	}

	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database: t.session.Path,
		Init:     init,
		SQL:      resolvedSQL,
		MaxRows:  maxRows,
	})
	if err != nil {
		return errResult(formatRuntimeErr("query", err)), nil
	}

	display, content := renderQueryResult(res, time.Since(start).Milliseconds())
	return &tools.Result{Content: content, Display: display}, nil
}

// resolveSources expands every declared gs:// source, substitutes the
// resulting read_parquet([…]) call into the SQL where ${alias} appears,
// and returns the rewritten SQL plus the GCS-auth init prelude.
func (t *QueryTool) resolveSources(ctx context.Context, sql string, sources []querySource) (string, []string, error) {
	if len(sources) == 0 {
		return sql, nil, nil
	}
	if t.gcs == nil {
		return "", nil, fmt.Errorf("gs:// sources require GCP auth, which is not configured")
	}

	resolved := sql
	seen := make(map[string]bool)
	for _, s := range sources {
		if s.Alias == "" {
			return "", nil, fmt.Errorf("source alias is required")
		}
		if !validAlias(s.Alias) {
			return "", nil, fmt.Errorf("invalid alias %q (allowed: A-Za-z0-9_)", s.Alias)
		}
		if seen[s.Alias] {
			return "", nil, fmt.Errorf("duplicate source alias %q", s.Alias)
		}
		seen[s.Alias] = true

		var urls []string
		for _, p := range s.GCSPaths {
			expanded, err := t.gcs.ExpandGlob(ctx, p)
			if err != nil {
				return "", nil, fmt.Errorf("expand %s: %w", p, err)
			}
			urls = append(urls, expanded...)
		}
		if len(urls) == 0 {
			return "", nil, fmt.Errorf("source %q matched no objects", s.Alias)
		}

		placeholder := "${" + s.Alias + "}"
		if !strings.Contains(resolved, placeholder) {
			return "", nil, fmt.Errorf("placeholder %s not found in sql", placeholder)
		}
		resolved = strings.ReplaceAll(resolved, placeholder, duck.FormatReadParquetCall(urls))
	}

	init, err := t.gcs.BuildInitPrelude(ctx)
	if err != nil {
		return "", nil, err
	}
	return resolved, init, nil
}

func validAlias(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_':
		default:
			return false
		}
	}
	return true
}

func durationStr(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}
