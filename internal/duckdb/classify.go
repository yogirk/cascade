package duckdb

import (
	"strings"

	"github.com/slokam-ai/cascade/internal/permission"
)

// ClassifySQLRisk classifies a DuckDB SQL statement by its leading keyword
// after stripping comments. CTE-prefixed statements (WITH … <stmt>) are
// classified by the actual statement keyword that follows the CTE block,
// because DuckDB allows WITH … INSERT/UPDATE/DELETE just like BigQuery.
// Unknown statements default to RiskDestructive (assume worst).
//
// Mirrors internal/bigquery/classify.go in shape, but the keyword tables
// reflect DuckDB-specific verbs: ATTACH/DETACH, INSTALL/LOAD, EXPORT/IMPORT,
// PRAGMA, SUMMARIZE, COPY, plus DuckDB's bare-FROM shorthand for SELECT.
func ClassifySQLRisk(sql string) permission.RiskLevel {
	normalized := strings.TrimSpace(stripSQLComments(strings.TrimSpace(sql)))
	if normalized == "" {
		return permission.RiskDestructive
	}

	upper := strings.ToUpper(normalized)

	if hasAnyPrefix(upper, "WITH") {
		if keyword := findStatementAfterCTE(upper); keyword != "" {
			upper = keyword
		}
		// If we can't parse the CTE structure, fall through and classify
		// "WITH" as read-only — the most common case is WITH … SELECT.
	}

	return classifyKeyword(upper)
}

// classifyKeyword maps the leading keyword to a risk level.
//
// Anything not enumerated falls through to RiskDestructive on purpose:
// CALL, BEGIN/END blocks, EXECUTE-style statements, and unknown DuckDB
// extension verbs may have side effects we cannot statically prove safe.
// Do not add a verb to the read-only branch without confirming it cannot
// mutate state — that would silently bypass the permission gate.
func classifyKeyword(upper string) permission.RiskLevel {
	switch {
	case hasAnyPrefix(upper,
		"SELECT", "FROM", "WITH",
		"SHOW", "DESCRIBE", "DESC",
		"EXPLAIN", "SUMMARIZE",
		"VALUES", "TABLE",
	):
		return permission.RiskReadOnly

	case hasAnyPrefix(upper, "INSERT", "UPDATE", "DELETE", "MERGE", "COPY"):
		return permission.RiskDML

	case hasAnyPrefix(upper,
		"CREATE", "ALTER",
		"ATTACH", "INSTALL", "LOAD", "USE",
		"PRAGMA", "SET", "RESET",
		"CHECKPOINT", "BEGIN", "COMMIT", "ROLLBACK",
		"IMPORT",
	):
		return permission.RiskDDL

	case hasAnyPrefix(upper,
		"DROP", "TRUNCATE", "DETACH",
		"EXPORT", "VACUUM",
	):
		return permission.RiskDestructive

	case hasAnyPrefix(upper, "GRANT", "REVOKE"):
		return permission.RiskAdmin

	default:
		return permission.RiskDestructive
	}
}

// findStatementAfterCTE skips past WITH … AS (…) blocks (including
// multiple comma-separated CTEs) and returns the remaining SQL starting
// from the actual statement keyword. Returns "" if it cannot parse the
// CTE structure cleanly.
func findStatementAfterCTE(upper string) string {
	i := 0
	n := len(upper)

	i = skipWord(upper, i) // skip "WITH"
	i = skipWhitespace(upper, i)

	for i < n {
		i = skipWord(upper, i) // CTE name
		i = skipWhitespace(upper, i)

		if i+2 <= n && upper[i:i+2] == "AS" && (i+2 == n || isWhitespaceOrParen(upper[i+2])) {
			i += 2
			i = skipWhitespace(upper, i)
		} else {
			return ""
		}

		if i >= n || upper[i] != '(' {
			return ""
		}
		depth := 0
		for i < n {
			if upper[i] == '(' {
				depth++
			} else if upper[i] == ')' {
				depth--
				if depth == 0 {
					i++
					break
				}
			}
			i++
		}
		if depth != 0 {
			return ""
		}

		i = skipWhitespace(upper, i)
		if i < n && upper[i] == ',' {
			i++
			i = skipWhitespace(upper, i)
			continue
		}
		break
	}

	if i >= n {
		return ""
	}
	return upper[i:]
}

func skipWhitespace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	return i
}

func skipWord(s string, i int) int {
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' && s[i] != '(' && s[i] != ',' {
		i++
	}
	return i
}

func isWhitespaceOrParen(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '('
}

// stripSQLComments removes line comments (--) and block comments (/* */).
func stripSQLComments(sql string) string {
	var b strings.Builder
	b.Grow(len(sql))

	i := 0
	for i < len(sql) {
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			if i < len(sql) {
				b.WriteByte('\n')
				i++
			}
			continue
		}
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			found := false
			for i+1 < len(sql) {
				if sql[i] == '*' && sql[i+1] == '/' {
					i += 2
					found = true
					break
				}
				i++
			}
			if !found {
				break
			}
			b.WriteByte(' ')
			continue
		}
		b.WriteByte(sql[i])
		i++
	}
	return b.String()
}

// hasAnyPrefix returns true if upper starts with any prefix as a full word.
func hasAnyPrefix(upper string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if !strings.HasPrefix(upper, prefix) {
			continue
		}
		if len(upper) == len(prefix) {
			return true
		}
		next := upper[len(prefix)]
		if next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '(' || next == ';' {
			return true
		}
	}
	return false
}
