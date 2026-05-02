package duckdb

import (
	"strings"
	"testing"
)

func TestRewriteGCSURL(t *testing.T) {
	cases := []struct {
		in       string
		https    string
		bucket   string
		object   string
		wantErr  bool
	}{
		{"gs://hn-data/2025/data.parquet", "https://storage.googleapis.com/hn-data/2025/data.parquet", "hn-data", "2025/data.parquet", false},
		{"gs://b/o", "https://storage.googleapis.com/b/o", "b", "o", false},
		{"gs://only-bucket", "", "", "", true},
		{"gs://", "", "", "", true},
		{"https://example.com/x", "", "", "", true},
		{"gs://b/", "", "", "", true}, // empty object
	}
	for _, c := range cases {
		gotURL, gotBucket, gotObject, err := RewriteGCSURL(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("RewriteGCSURL(%q) err = %v, wantErr %v", c.in, err, c.wantErr)
			continue
		}
		if c.wantErr {
			continue
		}
		if gotURL != c.https || gotBucket != c.bucket || gotObject != c.object {
			t.Errorf("RewriteGCSURL(%q) = (%q, %q, %q), want (%q, %q, %q)",
				c.in, gotURL, gotBucket, gotObject, c.https, c.bucket, c.object)
		}
	}
}

func TestGlobPrefix(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"path/to/file.parquet", "path/to/file.parquet"},
		{"path/to/*.parquet", "path/to/"},
		{"year=*/file.parquet", "year="},
		{"a/b/c?", "a/b/c"},
		{"[abc]/x", ""},
	}
	for _, c := range cases {
		if got := globPrefix(c.in); got != c.want {
			t.Errorf("globPrefix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		// Hive partitions: `*` per segment, no spanning of /
		{"year=*/month=*/file.parquet", "year=2025/month=01/file.parquet", true},
		{"year=*/month=*/file.parquet", "year=2025/file.parquet", false},
		{"year=*/month=*/file.parquet", "year=2025/month=01/02/file.parquet", false},

		// `*.parquet` should not span `/`
		{"data/*.parquet", "data/a.parquet", true},
		{"data/*.parquet", "data/sub/a.parquet", false},

		// Question mark = single char, but only in one segment
		{"day=?", "day=1", true},
		{"day=?", "day=12", false},
	}
	for _, c := range cases {
		if got := globMatch(c.pattern, c.name); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}

func TestIsHidden(t *testing.T) {
	cases := []struct {
		name string
		hidden bool
	}{
		{"data/file.parquet", false},
		{"data/_metadata", true},
		{"data/.tmp/file.parquet", true},
		{"_SUCCESS", true},
		{"path/with/_internal/file.parquet", true},
	}
	for _, c := range cases {
		if got := isHidden(c.name); got != c.hidden {
			t.Errorf("isHidden(%q) = %v, want %v", c.name, got, c.hidden)
		}
	}
}

func TestFormatReadParquetCall(t *testing.T) {
	out := FormatReadParquetCall([]string{
		"https://storage.googleapis.com/b/o1.parquet",
		"https://storage.googleapis.com/b/o2.parquet",
	})
	want := "read_parquet(['https://storage.googleapis.com/b/o1.parquet', 'https://storage.googleapis.com/b/o2.parquet'])"
	if out != want {
		t.Errorf("FormatReadParquetCall = %q\nwant: %q", out, want)
	}

	// Single-quote escaping
	tricky := FormatReadParquetCall([]string{"https://x/it's.parquet"})
	if !strings.Contains(tricky, "it''s") {
		t.Errorf("expected single-quote escaping, got %q", tricky)
	}
}

func TestBuildInitPrelude_NoTokenSource(t *testing.T) {
	g := &GCSAuth{}
	if _, err := g.BuildInitPrelude(t.Context()); err == nil {
		t.Error("expected error when no TokenSource configured")
	}
}
