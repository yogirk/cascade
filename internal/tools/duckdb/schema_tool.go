package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	duck "github.com/slokam-ai/cascade/internal/duckdb"
	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/internal/tools"
)

type schemaInput struct {
	Action   string `json:"action"`         // list_tables | describe | sample
	Database string `json:"database,omitempty"`
	Table    string `json:"table,omitempty"`
	Limit    int    `json:"limit,omitempty"`
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
	return "Inspect a DuckDB database: list tables, describe a table's columns, or sample rows. Read-only. " +
		"By default operates on the per-session DB; pass database='/path/to/your.duckdb' to inspect an existing file. " +
		"Use this after bq_to_duckdb to confirm a table landed and to peek at its shape before writing queries."
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
			"database": map[string]any{
				"type":        "string",
				"description": "Optional path to a .duckdb file. Default: the per-session DB.",
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

	dbPath, isExternal, err := t.resolveDatabase(p.Database)
	if err != nil {
		return errResult(err.Error()), nil
	}

	switch p.Action {
	case "list_tables":
		return t.listTables(ctx, dbPath, isExternal)
	case "describe":
		return t.describe(ctx, dbPath, isExternal, p.Table)
	case "sample":
		limit := p.Limit
		if limit <= 0 {
			limit = 10
		}
		return t.sample(ctx, dbPath, isExternal, p.Table, limit)
	default:
		return errResult(fmt.Sprintf("unknown action %q (want list_tables, describe, or sample)", p.Action)), nil
	}
}

// resolveDatabase mirrors QueryTool.resolveDatabase. Empty = session DB
// (we own its lifecycle); non-empty must exist on disk.
func (t *SchemaTool) resolveDatabase(database string) (string, bool, error) {
	if database == "" {
		return t.session.Path, false, nil
	}
	abs, err := filepath.Abs(database)
	if err != nil {
		return "", false, fmt.Errorf("resolve database path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", false, fmt.Errorf("database %q does not exist: %w", abs, err)
	}
	if info.IsDir() {
		return "", false, fmt.Errorf("database %q is a directory, expected a .duckdb file", abs)
	}
	return abs, abs != t.session.Path, nil
}

func (t *SchemaTool) listTables(ctx context.Context, dbPath string, isExternal bool) (*tools.Result, error) {
	if !isExternal {
		t.session.RLock()
		defer t.session.RUnlock()
	}

	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database:     dbPath,
		SQL:          "SHOW TABLES",
		SkipDescribe: true, // SHOW is not DESCRIBE-able
	})
	if err != nil {
		return errResult(formatRuntimeErr("list tables", err)), nil
	}
	display, content := renderTableList(res.Rows)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *SchemaTool) describe(ctx context.Context, dbPath string, isExternal bool, table string) (*tools.Result, error) {
	if err := validateIdentifier(table); err != nil {
		return errResult(err.Error()), nil
	}
	if !isExternal {
		t.session.RLock()
		defer t.session.RUnlock()
	}

	// information_schema.columns rather than DESCRIBE so the DESCRIBE
	// pre-pass in runtime.Query works (DESCRIBE-of-DESCRIBE is invalid)
	// and the column order is stable. The query supports schema-
	// qualified names like "main.foo" by splitting on the dot.
	schema, name := splitSchema(table)
	sql := fmt.Sprintf(
		`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = '%s'`+
			` AND table_schema = '%s' ORDER BY ordinal_position`,
		escapeLiteral(name), escapeLiteral(schema))

	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database: dbPath,
		SQL:      sql,
	})
	if err != nil {
		return errResult(formatRuntimeErr("describe", err)), nil
	}
	if len(res.Rows) == 0 {
		msg := fmt.Sprintf("Table %q not found.", table)
		return errResult(msg), nil
	}

	cols := make([]duck.Column, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) < 2 {
			continue
		}
		cols = append(cols, duck.Column{Name: row[0], Type: row[1]})
	}
	display, content := renderColumnList(table, cols)
	return &tools.Result{Content: content, Display: display}, nil
}

// splitSchema parses an optional schema qualifier. "foo" → ("main", "foo");
// "schema.foo" → ("schema", "foo"). validateIdentifier already rejected
// names with more than one dot or any non-allowlisted character.
func splitSchema(s string) (schema, name string) {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "main", s
}

// escapeLiteral doubles single quotes so an identifier can be embedded
// in a string literal. validateIdentifier has already restricted the
// character set, so this is belt-and-braces against future inputs.
func escapeLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func (t *SchemaTool) sample(ctx context.Context, dbPath string, isExternal bool, table string, limit int) (*tools.Result, error) {
	if err := validateIdentifier(table); err != nil {
		return errResult(err.Error()), nil
	}
	if !isExternal {
		t.session.RLock()
		defer t.session.RUnlock()
	}

	sql := fmt.Sprintf("SELECT * FROM %s LIMIT %d", quoteIdent(table), limit)
	start := time.Now()
	res, err := t.runtime.Query(ctx, duck.QueryOptions{
		Database: dbPath,
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
