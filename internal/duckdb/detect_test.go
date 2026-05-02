package duckdb

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"v1.2.1 abc1234\n", "1.2.1"},
		{"DuckDB v0.10.2", "0.10.2"},
		{"v1.0.0\n", "1.0.0"},
		{"junk", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := parseVersion(c.in); got != c.want {
			t.Errorf("parseVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.2.0", "1.2.0", 0},
		{"1.2.0", "1.2.1", -1},
		{"1.2.1", "1.2.0", 1},
		{"1.1.9", "1.2.0", -1},
		{"2.0.0", "1.99.99", 1},
		{"0.10.2", "0.9.99", 1}, // numeric, not lexicographic
		{"0.9.99", "0.10.2", -1},
	}
	for _, c := range cases {
		if got := compareSemver(c.a, c.b); got != c.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
