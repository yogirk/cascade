package duckdb

import (
	"testing"

	"github.com/slokam-ai/cascade/internal/permission"
)

func TestClassifySQLRisk(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want permission.RiskLevel
	}{
		// read-only
		{"select", "SELECT 1", permission.RiskReadOnly},
		{"select-lower", "select * from t", permission.RiskReadOnly},
		{"from-shorthand", "FROM hn_2025 LIMIT 10", permission.RiskReadOnly},
		{"with-select", "WITH x AS (SELECT 1) SELECT * FROM x", permission.RiskReadOnly},
		{"explain", "EXPLAIN SELECT * FROM t", permission.RiskReadOnly},
		{"summarize", "SUMMARIZE t", permission.RiskReadOnly},
		{"describe", "DESCRIBE t", permission.RiskReadOnly},
		{"show", "SHOW TABLES", permission.RiskReadOnly},
		{"values", "VALUES (1), (2)", permission.RiskReadOnly},

		// CTE-wrapped DML
		{"with-insert", "WITH x AS (SELECT 1 AS v) INSERT INTO t SELECT v FROM x", permission.RiskDML},
		{"with-delete", "WITH x AS (SELECT id FROM t WHERE v=1) DELETE FROM t WHERE id IN (SELECT id FROM x)", permission.RiskDML},

		// DML / write-bearing
		{"insert", "INSERT INTO t VALUES (1)", permission.RiskDML},
		{"update", "UPDATE t SET v=2", permission.RiskDML},
		{"delete", "DELETE FROM t", permission.RiskDML},
		{"copy-from", "COPY t FROM 'data.parquet' (FORMAT PARQUET)", permission.RiskDML},
		{"copy-to", "COPY t TO 'out.parquet' (FORMAT PARQUET)", permission.RiskDML},

		// DDL / state-modifying
		{"create", "CREATE TABLE t AS SELECT * FROM s", permission.RiskDDL},
		{"create-or-replace", "CREATE OR REPLACE TABLE t (id INT)", permission.RiskDDL},
		{"alter", "ALTER TABLE t ADD COLUMN v INT", permission.RiskDDL},
		{"attach", "ATTACH 'foo.db' AS f", permission.RiskDDL},
		{"install", "INSTALL httpfs", permission.RiskDDL},
		{"load", "LOAD httpfs", permission.RiskDDL},
		{"set", "SET memory_limit='2GB'", permission.RiskDDL},
		{"pragma", "PRAGMA database_list", permission.RiskDDL},

		// destructive
		{"drop", "DROP TABLE t", permission.RiskDestructive},
		{"truncate", "TRUNCATE t", permission.RiskDestructive},
		{"detach", "DETACH f", permission.RiskDestructive},
		{"export", "EXPORT DATABASE 'out'", permission.RiskDestructive},
		{"vacuum", "VACUUM", permission.RiskDestructive},

		// admin
		{"grant", "GRANT SELECT ON t TO u", permission.RiskAdmin},
		{"revoke", "REVOKE SELECT ON t FROM u", permission.RiskAdmin},

		// edge cases
		{"empty", "", permission.RiskDestructive},
		{"comment-only", "-- nothing\n/* still nothing */", permission.RiskDestructive},
		{"unknown-verb", "EXECUTE foo()", permission.RiskDestructive},
		{"select-with-comment", "/* hint */ SELECT 1", permission.RiskReadOnly},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ClassifySQLRisk(c.sql)
			if got != c.want {
				t.Errorf("ClassifySQLRisk(%q) = %v, want %v", c.sql, got, c.want)
			}
		})
	}
}
