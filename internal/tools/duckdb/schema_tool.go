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

type schemaInput struct {
	Action string `json:"action"`         // list_tables | describe | sample
	Table  string `json:"table,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// SchemaTool is the read-only counterpart to query_tool: it lists and
// describes tables in the local DuckDB session DB without the agent
// having to remember the SHOW/DESCRIBE/SELECT idioms.
//
// Scope: local session DB only. To inspect a remote Parquet schema,
// the agent uses query_tool with `DESCRIBE SELECT * FROM read_parquet(…)`
// — adding a second remote-aware schema tool would push the surface
// past the 15-tool guard without earning its keep.
type SchemaTool struct {
	runtime duck.Runtime
	session *duck.Session
}

// NewSchemaTool wires a schema_tool.
func NewSchemaTool(runtime duck.Runtime, session *duck.Session) *SchemaTool {
	return &SchemaTool{runtime: runtime, session: session}
}

func (t *SchemaTool) Name() string { return "duckdb_schema" }

func (t *SchemaTool) Description() string {
	return "Inspect the local DuckDB session: list tables, describe a table's columns, or sample rows from a table. Read-only. Use this after bq_to_duckdb to confirm a table landed and to peek at its shape before writing queries."
}

func (t *SchemaTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list_tables", "describe", "sample"},
				"description": "What to inspect.",
			},
			"table": map[string]any{
				"type":        "string",
				"description": "Table name (required for describe and sample).",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Row count for sample. Default 10.",
			},
		},
		"required": []string{"action"},
	}
}

func (t *SchemaTool) RiskLevel() permission.RiskLevel { return permission.RiskReadOnly }

func (t *SchemaTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var p schemaInput
	if err := json.Unmarshal(input, &p); err != nil {
		return errResult(fmt.Sprintf("invalid input: %v", err)), nil
	}
	if t.runtime == nil || t.session == nil {
		return errResult("DuckDB runtime not configured"), nil
	}

	switch p.Action {
	case "list_tables":
		return t.listTables(ctx)
	case "describe":
		return t.describe(ctx, p.Table)
	case "sample":
		limit := p.Limit
		if limit <= 0 {
			limit = 10
		}
		return t.sample(ctx, p.Table, limit)
	default:
		return errResult(fmt.Sprintf("unknown action %q (want list_tables, describe, or sample)", p.Action)), nil
	}
}

func (t *SchemaTool) listTables(ctx context.Context) (*tools.Result, error) {
	t.session.RLock()
	defer t.session.RUnlock()

	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database:     t.session.Path,
		SQL:          "SHOW TABLES",
		SkipDescribe: true, // SHOW is not DESCRIBE-able
	})
	if err != nil {
		return errResult(formatRuntimeErr("list tables", err)), nil
	}
	display, content := renderTableList(res.Rows)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *SchemaTool) describe(ctx context.Context, table string) (*tools.Result, error) {
	if err := validateIdentifier(table); err != nil {
		return errResult(err.Error()), nil
	}
	t.session.RLock()
	defer t.session.RUnlock()

	// DESCRIBE <table> returns the column list. We use Query rather than
	// stitching DESCRIBE-of-DESCRIBE — runtime.Query already DESCRIBEs
	// the SQL we hand it for type fidelity, which is fine here since
	// the inner DESCRIBE returns string columns.
	sql := fmt.Sprintf("DESCRIBE %s", quoteIdent(table))
	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database:     t.session.Path,
		SQL:          sql,
		SkipDescribe: true, // DESCRIBE-of-DESCRIBE is invalid
	})
	if err != nil {
		return errResult(formatRuntimeErr("describe", err)), nil
	}

	cols := make([]duck.Column, 0, len(res.Rows))
	// Output of `DESCRIBE` columns: column_name, column_type, null, key,
	// default, extra. We only surface name + type — the rest is
	// noise for v1.
	for _, row := range res.Rows {
		if len(row) < 2 {
			continue
		}
		cols = append(cols, duck.Column{Name: row[0], Type: row[1]})
	}
	display, content := renderColumnList(table, cols)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *SchemaTool) sample(ctx context.Context, table string, limit int) (*tools.Result, error) {
	if err := validateIdentifier(table); err != nil {
		return errResult(err.Error()), nil
	}
	t.session.RLock()
	defer t.session.RUnlock()

	sql := fmt.Sprintf("SELECT * FROM %s LIMIT %d", quoteIdent(table), limit)
	start := time.Now()
	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database: t.session.Path,
		SQL:      sql,
		MaxRows:  limit,
	})
	if err != nil {
		return errResult(formatRuntimeErr("sample", err)), nil
	}
	display, content := renderQueryResult(res, time.Since(start).Milliseconds())
	return &tools.Result{Content: content, Display: display}, nil
}

// validateIdentifier rejects names containing characters that would
// require a different escaping strategy than quoteIdent provides. We
// only allow ASCII letters/digits, underscore, and dot (for schema-
// qualified names like "main.foo"). Anything else gets rejected before
// it reaches the SQL builder.
func validateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("table is required")
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '.':
		default:
			return fmt.Errorf("invalid identifier %q (allowed: A-Z, a-z, 0-9, _, .)", s)
		}
	}
	return nil
}

// quoteIdent double-quotes an identifier, also splitting on `.` so that
// `schema.table` is rendered as `"schema"."table"`. validateIdentifier
// has already restricted the character set, so simple quoting suffices.
func quoteIdent(s string) string {
	parts := strings.Split(s, ".")
	for i, p := range parts {
		parts[i] = `"` + p + `"`
	}
	return strings.Join(parts, ".")
}

func errResult(msg string) *tools.Result {
	return &tools.Result{Content: msg, Display: msg, IsError: true}
}

func formatRuntimeErr(op string, err error) string {
	if subErr, ok := err.(*duck.SubprocessError); ok {
		return fmt.Sprintf("%s failed (exit %d): %s", op, subErr.ExitCode, subErr.Stderr)
	}
	return fmt.Sprintf("%s failed: %v", op, err)
}
