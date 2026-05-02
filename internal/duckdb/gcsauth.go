package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/iterator"
)

// GCSAuth turns Cascade's existing oauth2.TokenSource into the bits the
// duckdb httpfs extension needs: a SET extra_http_headers init prelude
// carrying a fresh Authorization Bearer, plus glob expansion that lists
// matching gs://… objects via the GCS API and rewrites them into the
// https://storage.googleapis.com/… URLs httpfs actually fetches.
//
// Auth-mode-agnostic by design: ResourceAuth produces an OAuth bearer
// from ADC, impersonation, or a service-account key. All three carry
// the cloud-platform scope GCS XML API accepts.
// StorageClientProvider returns a *storage.Client (or nil if it is
// still initializing). Mirrors the lazy-provider pattern used by the
// Logging and GCS tools — the storage client is created in a goroutine
// at app startup so it doesn't block the TUI from rendering.
type StorageClientProvider func() *storage.Client

type GCSAuth struct {
	ts          oauth2.TokenSource
	getGCSClient StorageClientProvider
}

// NewGCSAuth wires the bearer source and a storage-client provider.
// The provider is consulted lazily on each ExpandGlob call so it can
// race with async init — by the time the agent calls a DuckDB tool,
// the client is almost always ready.
func NewGCSAuth(ts oauth2.TokenSource, getGCSClient StorageClientProvider) *GCSAuth {
	return &GCSAuth{ts: ts, getGCSClient: getGCSClient}
}

// BuildInitPrelude returns init-file SQL that pins the current bearer
// token onto the duckdb session. Must run before any read_parquet against
// https://storage.googleapis.com/…
//
// Token freshness: oauth2.ReuseTokenSource (already wrapped upstream by
// internal/auth) auto-refreshes inside the 1-hour TTL window, so calling
// .Token() per Cascade tool invocation is the right cadence — we get a
// new bearer roughly hourly, no busier than that.
//
// JSON-escaping the token defends against quote-poisoning if a future
// token format ever included one.
func (g *GCSAuth) BuildInitPrelude(ctx context.Context) ([]string, error) {
	if g == nil || g.ts == nil {
		return nil, fmt.Errorf("duckdb: no GCS token source configured")
	}
	tok, err := g.ts.Token()
	if err != nil {
		return nil, fmt.Errorf("duckdb: fetch GCS bearer token: %w", err)
	}
	headers := map[string]string{
		"Authorization": "Bearer " + tok.AccessToken,
	}
	hdrJSON, err := json.Marshal(headers)
	if err != nil {
		return nil, err
	}
	// Escape single quotes for DuckDB's string literal form. The JSON
	// itself uses double quotes; single quotes shouldn't appear, but
	// belt-and-braces.
	literal := strings.ReplaceAll(string(hdrJSON), "'", "''")
	stmt := fmt.Sprintf("SET extra_http_headers = '%s'", literal)

	// Help duckdb resolve the httpfs extension if it isn't auto-loaded.
	// LOAD is idempotent and cheap.
	return []string{
		"INSTALL httpfs",
		"LOAD httpfs",
		stmt,
	}, nil
}

// RewriteGCSURL converts a gs://bucket/object URL into the
// https://storage.googleapis.com/bucket/object form that the GCS XML
// API serves and httpfs can GET. Returns the rewritten URL plus the
// parsed bucket/object pair so callers (notably ExpandGlob) can avoid
// re-parsing.
//
// Returns an error for malformed inputs (no gs:// prefix, missing
// object). The bucket-only case (gs://bucket) is rejected: httpfs needs
// to fetch an object, not a bucket listing.
func RewriteGCSURL(gsURL string) (httpsURL, bucket, object string, err error) {
	if !strings.HasPrefix(gsURL, "gs://") {
		return "", "", "", fmt.Errorf("not a gs:// URL: %q", gsURL)
	}
	rest := strings.TrimPrefix(gsURL, "gs://")
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "", "", "", fmt.Errorf("missing object path: %q", gsURL)
	}
	bucket = rest[:slash]
	object = rest[slash+1:]
	if bucket == "" || object == "" {
		return "", "", "", fmt.Errorf("empty bucket or object in %q", gsURL)
	}
	httpsURL = "https://storage.googleapis.com/" + bucket + "/" + object
	return httpsURL, bucket, object, nil
}

// ExpandGlob lists matching objects for a gs:// URL that may contain
// glob meta-characters (`*`, `?`) or hive-partition wildcards (`year=*`).
// Returns an explicit list of https URLs for read_parquet to consume.
//
// Why we expand server-side instead of asking httpfs to do it: the
// reviewer concern was textual gs:// rewriting inside arbitrary user
// SQL is fragile (CTEs, subqueries, string literals look the same).
// Building the URL list here, in Go, and passing it as a parameterized
// `read_parquet([url1, url2, ...])` call keeps the user's SQL clean.
//
// A URL with no meta-characters short-circuits to a single-element
// list, so callers can use ExpandGlob unconditionally.
//
// Hidden objects (path components starting with `_` or `.`) are
// excluded — that's BigQuery's EXPORT convention and what users
// expect when iterating on `gs://…/cascade-bq-export/{session}/`.
func (g *GCSAuth) ExpandGlob(ctx context.Context, gsURL string) ([]string, error) {
	if !strings.HasPrefix(gsURL, "gs://") {
		return nil, fmt.Errorf("not a gs:// URL: %q", gsURL)
	}
	bucket, pattern, err := splitGSPath(gsURL)
	if err != nil {
		return nil, err
	}

	// Fast path: no meta-characters → single URL.
	if !hasGlobChars(pattern) {
		return []string{"https://storage.googleapis.com/" + bucket + "/" + pattern}, nil
	}

	if g == nil || g.getGCSClient == nil {
		return nil, fmt.Errorf("duckdb: GCS client not configured for glob expansion")
	}
	client := g.getGCSClient()
	if client == nil {
		return nil, fmt.Errorf("duckdb: GCS client not yet initialized")
	}

	// Fixed prefix is everything before the first meta-character. We
	// list with that prefix server-side and filter the rest in Go.
	prefix := globPrefix(pattern)
	it := client.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})

	var matches []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list gs://%s/%s: %w", bucket, prefix, err)
		}
		if isHidden(attrs.Name) {
			continue
		}
		if !globMatch(pattern, attrs.Name) {
			continue
		}
		matches = append(matches, "https://storage.googleapis.com/"+bucket+"/"+attrs.Name)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no objects matched gs://%s/%s", bucket, pattern)
	}
	return matches, nil
}

// splitGSPath parses gs://bucket/path/with/maybe/* into (bucket, path).
func splitGSPath(gsURL string) (bucket, objectPattern string, err error) {
	rest := strings.TrimPrefix(gsURL, "gs://")
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		return "", "", fmt.Errorf("missing object path: %q", gsURL)
	}
	bucket = rest[:slash]
	objectPattern = rest[slash+1:]
	if bucket == "" || objectPattern == "" {
		return "", "", fmt.Errorf("empty bucket or object in %q", gsURL)
	}
	return bucket, objectPattern, nil
}

func hasGlobChars(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// globPrefix returns the longest prefix of pattern that has no
// meta-characters — what we send to GCS list as a server-side filter.
func globPrefix(pattern string) string {
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*', '?', '[':
			return pattern[:i]
		}
	}
	return pattern
}

// globMatch reports whether a full object name matches a possibly
// hive-partitioned glob. We use path.Match per `/`-separated segment so
// `year=*/month=*/file.parquet` works the way users expect — `*` does
// not span path separators.
func globMatch(pattern, name string) bool {
	patSegs := strings.Split(pattern, "/")
	nameSegs := strings.Split(name, "/")
	if len(patSegs) != len(nameSegs) {
		return false
	}
	for i, ps := range patSegs {
		ok, err := path.Match(ps, nameSegs[i])
		if err != nil || !ok {
			return false
		}
	}
	return true
}

// isHidden mirrors the BigQuery EXPORT convention: any path component
// starting with `_` or `.` is treated as metadata, not data.
func isHidden(name string) bool {
	for _, seg := range strings.Split(name, "/") {
		if seg == "" {
			continue
		}
		if seg[0] == '_' || seg[0] == '.' {
			return true
		}
	}
	return false
}

// FormatReadParquetCall builds a parameterized read_parquet([…]) call
// from a list of https URLs. Each URL is single-quoted and any embedded
// single quote is doubled, matching DuckDB's string-literal escaping.
//
// This is the central piece that keeps Cascade off the textual-rewrite
// fragility path: the user's SQL is built with a placeholder (e.g.
// `__GCS_URLS__`) and we substitute the formatted list here.
func FormatReadParquetCall(urls []string) string {
	parts := make([]string, len(urls))
	for i, u := range urls {
		parts[i] = "'" + strings.ReplaceAll(u, "'", "''") + "'"
	}
	return "read_parquet([" + strings.Join(parts, ", ") + "])"
}
