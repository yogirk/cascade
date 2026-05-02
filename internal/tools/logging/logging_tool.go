// Package logging provides a Cloud Logging tool for querying GCP log entries.
package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/logging/logadmin"
	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/internal/tools"
	"google.golang.org/api/iterator"
)

type logInput struct {
	Action   string `json:"action"`   // query | tail
	Filter   string `json:"filter"`   // Cloud Logging filter string
	Duration string `json:"duration"` // e.g., "1h", "24h", "7d"
	Limit    int    `json:"limit"`    // max entries (default 50)
}

// LogEntry holds a parsed log entry for rendering.
type LogEntry struct {
	Timestamp time.Time
	Severity  string
	Resource  string
	LogName   string
	Message   string
}

// ClientProvider returns a logging client (may be nil if still initializing).
type ClientProvider func() *logadmin.Client

// LogTool queries Cloud Logging entries.
type LogTool struct {
	getClient  ClientProvider
	projectID  string
	maxEntries int
}

// NewLogTool creates a new Cloud Logging tool.
func NewLogTool(getClient ClientProvider, projectID string, maxEntries int) *LogTool {
	if maxEntries <= 0 {
		maxEntries = 50
	}
	return &LogTool{
		getClient:  getClient,
		projectID:  projectID,
		maxEntries: maxEntries,
	}
}

func (t *LogTool) Name() string { return "cloud_logging" }

func (t *LogTool) Description() string {
	return "Query Cloud Logging entries. Use action='query' with a filter string to search logs, or action='tail' for most recent entries. Filter uses Cloud Logging syntax (e.g., severity>=ERROR AND resource.type=\"bigquery_dataset\"). Duration limits the time range (e.g., \"1h\", \"24h\", \"7d\")."
}

func (t *LogTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"query", "tail"},
				"description": "query: search with filter; tail: most recent entries",
			},
			"filter": map[string]any{
				"type":        "string",
				"description": "Cloud Logging filter string (e.g., severity>=ERROR AND resource.type=\"bigquery_dataset\")",
			},
			"duration": map[string]any{
				"type":        "string",
				"description": "Time range to search (e.g., \"1h\", \"24h\", \"7d\"). Default: 1h",
			},
			"limit": map[string]any{
				"type":        "number",
				"description": "Maximum number of entries to return. Default: 50",
			},
		},
		"required": []string{"action"},
	}
}

func (t *LogTool) RiskLevel() permission.RiskLevel {
	return permission.RiskReadOnly
}

func (t *LogTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var params logInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	switch params.Action {
	case "query", "tail":
		return t.queryLogs(ctx, params)
	default:
		return &tools.Result{
			Content: fmt.Sprintf("unknown action %q: use 'query' or 'tail'", params.Action),
			IsError: true,
		}, nil
	}
}

func (t *LogTool) queryLogs(ctx context.Context, params logInput) (*tools.Result, error) {
	client := t.getClient()
	if client == nil {
		return &tools.Result{
			Content: "Cloud Logging not available. Check GCP credentials and permissions (roles/logging.viewer).",
			IsError: true,
		}, nil
	}

	// Parse duration
	duration := 1 * time.Hour
	if params.Duration != "" {
		d, err := parseDuration(params.Duration)
		if err != nil {
			return &tools.Result{Content: fmt.Sprintf("invalid duration %q: %v", params.Duration, err), IsError: true}, nil
		}
		duration = d
	}

	// Build filter
	var filterParts []string

	// Add timestamp filter
	since := time.Now().UTC().Add(-duration)
	filterParts = append(filterParts, fmt.Sprintf("timestamp >= %q", since.Format(time.RFC3339)))

	// For tail, default to severity >= INFO if no filter specified
	if params.Action == "tail" && params.Filter == "" {
		filterParts = append(filterParts, "severity >= DEFAULT")
	}

	// Append user filter
	if params.Filter != "" {
		filterParts = append(filterParts, params.Filter)
	}

	filter := strings.Join(filterParts, " AND ")

	// Determine limit
	limit := params.Limit
	if limit <= 0 {
		limit = t.maxEntries
	}
	if limit > t.maxEntries {
		limit = t.maxEntries
	}

	// Query entries
	it := client.Entries(ctx, logadmin.Filter(filter), logadmin.NewestFirst())

	var entries []LogEntry
	for i := 0; i < limit; i++ {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return &tools.Result{
				Content: fmt.Sprintf("Log query failed: %v", err),
				IsError: true,
			}, nil
		}

		// Extract message from payload
		msg := extractMessage(entry.Payload)

		// Extract resource type
		resource := ""
		if entry.Resource != nil {
			resource = entry.Resource.Type
		}

		// Shorten log name — extract just the log ID from the full path
		logName := entry.LogName
		if idx := strings.LastIndex(logName, "/"); idx >= 0 {
			logName = logName[idx+1:]
		}

		entries = append(entries, LogEntry{
			Timestamp: entry.Timestamp,
			Severity:  entry.Severity.String(),
			Resource:  resource,
			LogName:   logName,
			Message:   msg,
		})
	}

	display, content := RenderLogEntries(entries, filter, duration)
	return &tools.Result{Content: content, Display: display}, nil
}

// extractMessage pulls a readable summary from a log entry payload.
// Prefers human-readable fields over raw JSON dumps.
func extractMessage(payload interface{}) string {
	if payload == nil {
		return ""
	}

	switch p := payload.(type) {
	case string:
		return truncateMsg(p, 120)
	default:
		// Try to extract useful fields from struct payloads
		data, err := json.Marshal(p)
		if err != nil {
			return truncateMsg(fmt.Sprintf("%v", p), 120)
		}

		var m map[string]interface{}
		if json.Unmarshal(data, &m) != nil {
			return truncateMsg(string(data), 120)
		}

		// Walk common message locations (ordered by specificity)
		if msg := deepString(m, "message"); msg != "" {
			return truncateMsg(msg, 120)
		}
		if msg := deepString(m, "textPayload"); msg != "" {
			return truncateMsg(msg, 120)
		}
		// BQ audit logs: status.message inside the proto
		if msg := deepString(m, "status", "message"); msg != "" {
			return truncateMsg(msg, 120)
		}
		// Nested serviceData or metadata
		if msg := deepString(m, "serviceData", "jobCompletedEvent", "job", "jobStatus", "error", "message"); msg != "" {
			return truncateMsg(msg, 120)
		}
		if msg := deepString(m, "metadata", "jobChange", "job", "jobStatus", "error", "message"); msg != "" {
			return truncateMsg(msg, 120)
		}
		// methodName as summary
		if method := deepString(m, "methodName"); method != "" {
			svc, _ := m["serviceName"].(string)
			if svc == "" {
				return truncateMsg(method, 120)
			}
			return truncateMsg(fmt.Sprintf("%s → %s", svc, method), 120)
		}

		// Fallback: compact JSON, truncated
		return truncateMsg(string(data), 120)
	}
}

// deepString walks a nested map by keys and returns the string value at the end.
func deepString(m map[string]interface{}, keys ...string) string {
	current := interface{}(m)
	for _, key := range keys {
		cm, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = cm[key]
		if !ok {
			return ""
		}
	}
	if s, ok := current.(string); ok {
		return s
	}
	return ""
}

func truncateMsg(s string, maxLen int) string {
	// Replace newlines with spaces for single-line display
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// parseDuration parses human-friendly durations like "1h", "24h", "7d", "30m".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}
