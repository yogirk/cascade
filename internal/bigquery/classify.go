package bigquery

import (
	"strings"

	"github.com/yogirk/cascade/internal/permission"
)

// ClassifySQLRisk classifies the risk level of a SQL statement by parsing
// the first keyword after stripping comments and whitespace.
// Unknown statements default to RiskDestructive (assume worst).
func ClassifySQLRisk(sql string) permission.RiskLevel {
	normalized := stripSQLComments(strings.TrimSpace(sql))
	normalized = strings.TrimSpace(normalized)

	if normalized == "" {
		return permission.RiskDestructive
	}

	upper := strings.ToUpper(normalized)

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
