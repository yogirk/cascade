// Package gcs provides a Cloud Storage tool for browsing buckets and reading objects.
package gcs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/tools"
	"google.golang.org/api/iterator"
)

type gcsInput struct {
	Action   string `json:"action"`    // list_buckets | list_objects | read_object | object_info
	Bucket   string `json:"bucket"`    // bucket name
	Prefix   string `json:"prefix"`    // path prefix for listing
	Object   string `json:"object"`    // full object path
	MaxLines int    `json:"max_lines"` // max lines to read (default 100)
}

// BucketInfo holds bucket metadata for rendering.
type BucketInfo struct {
	Name         string
	Location     string
	StorageClass string
	Created      time.Time
}

// ObjectInfo holds object metadata for rendering.
type ObjectInfo struct {
	Name        string
	Bucket      string
	Size        int64
	ContentType string
	Updated     time.Time
	IsDir       bool // pseudo-directory from delimiter listing
}

// ClientProvider returns a storage client (may be nil if still initializing).
type ClientProvider func() *storage.Client

// GCSTool browses and reads Cloud Storage objects.
type GCSTool struct {
	getClient    ClientProvider
	projectID    string
	maxReadLines int
}

// NewGCSTool creates a new GCS tool.
func NewGCSTool(getClient ClientProvider, projectID string, maxReadLines int) *GCSTool {
	if maxReadLines <= 0 {
		maxReadLines = 100
	}
	return &GCSTool{
		getClient:    getClient,
		projectID:    projectID,
		maxReadLines: maxReadLines,
	}
}

func (t *GCSTool) Name() string { return "gcs" }

func (t *GCSTool) Description() string {
	return "Browse and read Google Cloud Storage objects. Actions: list_buckets (all buckets in project), list_objects (objects in a bucket with optional prefix for directory browsing), read_object (first N lines of a text file), object_info (metadata for a specific object)."
}

func (t *GCSTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"list_buckets", "list_objects", "read_object", "object_info"},
				"description": "The GCS action to perform",
			},
			"bucket": map[string]any{
				"type":        "string",
				"description": "Bucket name (required for list_objects, read_object, object_info)",
			},
			"prefix": map[string]any{
				"type":        "string",
				"description": "Path prefix for list_objects (e.g., 'data/2026/03/')",
			},
			"object": map[string]any{
				"type":        "string",
				"description": "Object path for read_object and object_info",
			},
			"max_lines": map[string]any{
				"type":        "number",
				"description": "Max lines to read for read_object (default 100)",
			},
		},
		"required": []string{"action"},
	}
}

func (t *GCSTool) RiskLevel() permission.RiskLevel {
	return permission.RiskReadOnly
}

func (t *GCSTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var params gcsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	if t.getClient() == nil {
		return &tools.Result{
			Content: "Cloud Storage not available. Check GCP credentials and permissions (roles/storage.objectViewer).",
			IsError: true,
		}, nil
	}

	switch params.Action {
	case "list_buckets":
		return t.listBuckets(ctx)
	case "list_objects":
		return t.listObjects(ctx, params)
	case "read_object":
		return t.readObject(ctx, params)
	case "object_info":
		return t.objectInfo(ctx, params)
	default:
		return &tools.Result{
			Content: fmt.Sprintf("unknown action %q: use list_buckets, list_objects, read_object, or object_info", params.Action),
			IsError: true,
		}, nil
	}
}

func (t *GCSTool) listBuckets(ctx context.Context) (*tools.Result, error) {
	it := t.getClient().Buckets(ctx, t.projectID)

	var buckets []BucketInfo
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return &tools.Result{Content: fmt.Sprintf("Failed to list buckets: %v", err), IsError: true}, nil
		}
		buckets = append(buckets, BucketInfo{
			Name:         attrs.Name,
			Location:     attrs.Location,
			StorageClass: attrs.StorageClass,
			Created:      attrs.Created,
		})
	}

	display, content := RenderBucketList(buckets, t.projectID)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *GCSTool) listObjects(ctx context.Context, params gcsInput) (*tools.Result, error) {
	if params.Bucket == "" {
		return &tools.Result{Content: "bucket parameter is required for list_objects", IsError: true}, nil
	}

	query := &storage.Query{
		Prefix:    params.Prefix,
		Delimiter: "/",
	}

	it := t.getClient().Bucket(params.Bucket).Objects(ctx, query)

	var objects []ObjectInfo
	count := 0
	maxItems := 100 // cap listing
	for count < maxItems {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return &tools.Result{Content: fmt.Sprintf("Failed to list objects: %v", err), IsError: true}, nil
		}

		if attrs.Prefix != "" {
			// Pseudo-directory
			objects = append(objects, ObjectInfo{
				Name:  attrs.Prefix,
				IsDir: true,
			})
		} else {
			objects = append(objects, ObjectInfo{
				Name:        attrs.Name,
				Bucket:      params.Bucket,
				Size:        attrs.Size,
				ContentType: attrs.ContentType,
				Updated:     attrs.Updated,
			})
		}
		count++
	}

	display, content := RenderObjectList(objects, params.Bucket, params.Prefix, count >= maxItems)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *GCSTool) readObject(ctx context.Context, params gcsInput) (*tools.Result, error) {
	if params.Bucket == "" || params.Object == "" {
		return &tools.Result{Content: "bucket and object parameters are required for read_object", IsError: true}, nil
	}

	// Check metadata first
	obj := t.getClient().Bucket(params.Bucket).Object(params.Object)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("Object not found: %v", err), IsError: true}, nil
	}

	// Binary detection
	if !isTextContent(attrs.ContentType) {
		display, content := RenderObjectMeta(attrs, params.Bucket, true)
		return &tools.Result{Content: content, Display: display}, nil
	}

	// Size warning (>10MB)
	if attrs.Size > 10*1024*1024 {
		return &tools.Result{
			Content: fmt.Sprintf("File is large (%s). Use object_info to check metadata, or specify max_lines to read a portion.",
				formatSize(attrs.Size)),
		}, nil
	}

	// Read lines
	maxLines := params.MaxLines
	if maxLines <= 0 {
		maxLines = t.maxReadLines
	}
	if maxLines > t.maxReadLines {
		maxLines = t.maxReadLines
	}

	rc, err := obj.NewReader(ctx)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("Failed to read object: %v", err), IsError: true}, nil
	}
	defer rc.Close()

	scanner := bufio.NewScanner(rc)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) >= maxLines {
			break
		}
	}

	truncated := len(lines) >= maxLines
	display, content := RenderFileContent(lines, params.Bucket, params.Object, attrs, truncated, maxLines)
	return &tools.Result{Content: content, Display: display}, nil
}

func (t *GCSTool) objectInfo(ctx context.Context, params gcsInput) (*tools.Result, error) {
	if params.Bucket == "" || params.Object == "" {
		return &tools.Result{Content: "bucket and object parameters are required for object_info", IsError: true}, nil
	}

	attrs, err := t.getClient().Bucket(params.Bucket).Object(params.Object).Attrs(ctx)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("Object not found: %v", err), IsError: true}, nil
	}

	display, content := RenderObjectMeta(attrs, params.Bucket, false)
	return &tools.Result{Content: content, Display: display}, nil
}

// isTextContent returns true for text-readable content types.
func isTextContent(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "text/") ||
		ct == "application/json" ||
		ct == "application/xml" ||
		ct == "application/csv" ||
		ct == "application/x-ndjson" ||
		strings.Contains(ct, "yaml") ||
		strings.Contains(ct, "sql") ||
		ct == "application/octet-stream" // many text files uploaded without proper type
}

// formatSize formats bytes as human-readable.
func formatSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// Satisfy io.Closer for cleanup tracking.
var _ io.Closer = (*GCSTool)(nil)

// Close is a no-op — client lifecycle managed by PlatformComponents.
func (t *GCSTool) Close() error { return nil }
