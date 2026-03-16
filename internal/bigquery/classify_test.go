package bigquery

import (
	"testing"

	"github.com/yogirk/cascade/internal/permission"
)

func TestClassifySQLRisk(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want permission.RiskLevel
	}{
		// ReadOnly
		{"select basic", "SELECT * FROM t", permission.RiskReadOnly},
		{"select leading whitespace", "  SELECT * FROM t", permission.RiskReadOnly},
		{"select line comment", "-- comment\nSELECT * FROM t", permission.RiskReadOnly},
		{"select block comment", "/* block */  SELECT * FROM t", permission.RiskReadOnly},
		{"with cte select", "WITH cte AS (SELECT 1) SELECT * FROM cte", permission.RiskReadOnly},
		{"with cte insert", "WITH data AS (SELECT 1 AS id) INSERT INTO t SELECT * FROM data", permission.RiskDML},
		{"with cte update", "WITH data AS (SELECT 1 AS id) UPDATE t SET x = 1 WHERE id IN (SELECT id FROM data)", permission.RiskDML},
		{"with cte delete", "WITH data AS (SELECT 1 AS id) DELETE FROM t WHERE id IN (SELECT id FROM data)", permission.RiskDML},
		{"with cte merge", "WITH data AS (SELECT 1 AS id) MERGE INTO t USING data ON t.id = data.id WHEN MATCHED THEN DELETE", permission.RiskDML},
		{"with multiple ctes select", "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a JOIN b", permission.RiskReadOnly},
		{"with multiple ctes insert", "WITH a AS (SELECT 1), b AS (SELECT 2) INSERT INTO t SELECT * FROM a", permission.RiskDML},
		{"with nested parens", "WITH cte AS (SELECT * FROM (SELECT 1)) SELECT * FROM cte", permission.RiskReadOnly},
		{"with cte lowercase", "with cte as (select 1) insert into t select 1", permission.RiskDML},
		{"show tables", "SHOW TABLES", permission.RiskReadOnly},
		{"explain", "EXPLAIN SELECT 1", permission.RiskReadOnly},
		{"describe", "DESCRIBE my_table", permission.RiskReadOnly},
		{"select lowercase", "select * from t", permission.RiskReadOnly},

		// DML
		{"insert", "INSERT INTO t VALUES (1)", permission.RiskDML},
		{"update", "UPDATE t SET x=1", permission.RiskDML},
		{"delete", "DELETE FROM t", permission.RiskDML},
		{"merge", "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET x=1", permission.RiskDML},

		// DDL
		{"create table", "CREATE TABLE t (id INT64)", permission.RiskDDL},
		{"alter table", "ALTER TABLE t ADD COLUMN x STRING", permission.RiskDDL},
		{"create or replace", "CREATE OR REPLACE TABLE t (id INT64)", permission.RiskDDL},

		// Destructive
		{"drop table", "DROP TABLE t", permission.RiskDestructive},
		{"truncate table", "TRUNCATE TABLE t", permission.RiskDestructive},

		// Admin
		{"grant", "GRANT SELECT ON t TO user", permission.RiskAdmin},
		{"revoke", "REVOKE SELECT ON t FROM user", permission.RiskAdmin},

		// Edge cases
		{"empty string", "", permission.RiskDestructive},
		{"unknown command", "SOMETHING UNKNOWN", permission.RiskDestructive},
		{"only whitespace", "   ", permission.RiskDestructive},
		{"only comments", "-- just a comment", permission.RiskDestructive},
		{"nested block comments", "/* outer /* inner */ SELECT * FROM t", permission.RiskReadOnly},
		{"multiple line comments", "-- first\n-- second\nSELECT 1", permission.RiskReadOnly},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifySQLRisk(tt.sql)
			if got != tt.want {
				t.Errorf("ClassifySQLRisk(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func TestStripSQLComments(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want string
	}{
		{"no comments", "SELECT 1", "SELECT 1"},
		{"line comment", "-- hi\nSELECT 1", "\nSELECT 1"},
		{"block comment", "/* hi */ SELECT 1", "  SELECT 1"},
		{"mixed", "-- line\n/* block */ SELECT 1", "\n  SELECT 1"},
		{"unterminated block", "/* never closes", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripSQLComments(tt.sql)
			if got != tt.want {
				t.Errorf("stripSQLComments(%q) = %q, want %q", tt.sql, got, tt.want)
			}
		})
	}
}
