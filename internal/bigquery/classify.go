package bigquery

import (
	"strings"

	"github.com/slokam-ai/cascade/internal/permission"
)

// ClassifySQLRisk classifies the risk level of a SQL statement by parsing
// the leading keyword after stripping comments and whitespace.
// CTE-prefixed statements (WITH ... AS (...) <stmt>) are classified by the
// actual statement keyword after the CTE, not by WITH itself — because
// BigQuery allows WITH ... INSERT/UPDATE/DELETE/MERGE.
// Unknown statements default to RiskDestructive (assume worst).
func ClassifySQLRisk(sql string) permission.RiskLevel {
	normalized := stripSQLComments(strings.TrimSpace(sql))
	normalized = strings.TrimSpace(normalized)

	if normalized == "" {
		return permission.RiskDestructive
	}

	upper := strings.ToUpper(normalized)

	// If statement starts with WITH, skip past CTE blocks to find the real keyword.
	if hasAnyPrefix(upper, "WITH") {
		if keyword := findStatementAfterCTE(upper); keyword != "" {
			upper = keyword
		}
		// If we can't parse past the CTE, fall through to classify "WITH" as read-only
		// (likely a plain WITH ... SELECT).
	}

	return classifyKeyword(upper)
}

// classifyKeyword classifies the risk from the leading SQL keyword.
//
// Statements not enumerated below intentionally fall through to
// RiskDestructive. This covers BigQuery scripting constructs (CALL,
// BEGIN ... END, EXECUTE IMMEDIATE) whose effects depend on the body of
// the procedure or script — we cannot statically prove they are read-only
// without a full SQL parser, so the safe default is to force confirmation.
// Do NOT add any of these to the read-only branch without a parser that
// can analyse the procedure body — that would open a permission-bypass
// path.
func classifyKeyword(upper string) permission.RiskLevel {
	switch {
	case hasAnyPrefix(upper, "SELECT", "SHOW", "DESCRIBE", "EXPLAIN", "WITH"):
		return permission.RiskReadOnly
	case hasAnyPrefix(upper, "INSERT", "UPDATE", "DELETE", "MERGE"):
		return permission.RiskDML
	case hasAnyPrefix(upper, "CREATE", "ALTER"):
		return permission.RiskDDL
	case hasAnyPrefix(upper, "DROP", "TRUNCATE"):
		return permission.RiskDestructive
	case hasAnyPrefix(upper, "GRANT", "REVOKE"):
		return permission.RiskAdmin
	default:
		return permission.RiskDestructive
	}
}

// findStatementAfterCTE skips past WITH ... AS (...) blocks (including
// multiple comma-separated CTEs) and returns the remaining SQL starting
// from the actual statement keyword (SELECT, INSERT, UPDATE, DELETE, MERGE).
// Returns "" if it cannot parse the CTE structure.
func findStatementAfterCTE(upper string) string {
	i := 0
	n := len(upper)

	// Skip "WITH" keyword
	i = skipWord(upper, i)
	i = skipWhitespace(upper, i)

	for i < n {
		// Skip CTE name
		i = skipWord(upper, i)
		i = skipWhitespace(upper, i)

		// Expect "AS"
		if i+2 <= n && upper[i:i+2] == "AS" && (i+2 == n || isWhitespaceOrParen(upper[i+2])) {
			i += 2
			i = skipWhitespace(upper, i)
		} else {
			return ""
		}

		// Skip the parenthesized CTE body, handling nested parens
		if i < n && upper[i] == '(' {
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
				return "" // unbalanced parens
			}
		} else {
			return ""
		}

		i = skipWhitespace(upper, i)

		// If there's a comma, another CTE follows
		if i < n && upper[i] == ',' {
			i++
			i = skipWhitespace(upper, i)
			continue
		}

		// Otherwise we've reached the actual statement
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

// stripSQLComments removes line comments (--) and block comments (/* */) from SQL.
func stripSQLComments(sql string) string {
	var b strings.Builder
	b.Grow(len(sql))

	i := 0
	for i < len(sql) {
		// Line comment: -- until end of line.
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			if i < len(sql) {
				b.WriteByte('\n')
				i++ // skip the newline
			}
			continue
		}

		// Block comment: /* ... */.
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2 // skip /*
			found := false
			for i+1 < len(sql) {
				if sql[i] == '*' && sql[i+1] == '/' {
					i += 2 // skip */
					found = true
					break
				}
				i++
			}
			if !found {
				// Unterminated block comment -- discard rest of input.
				break
			}
			b.WriteByte(' ') // replace comment with space
			continue
		}

		b.WriteByte(sql[i])
		i++
	}

	return b.String()
}

// hasAnyPrefix returns true if the uppercase string starts with any of the given prefixes.
// The prefix must be followed by a space, newline, tab, or be the entire string.
func hasAnyPrefix(upper string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(upper, prefix) {
			// Ensure it's a full word match (not just a prefix of a longer word).
			if len(upper) == len(prefix) {
				return true
			}
			next := upper[len(prefix)]
			if next == ' ' || next == '\t' || next == '\n' || next == '\r' || next == '(' {
				return true
			}
		}
	}
	return false
}
