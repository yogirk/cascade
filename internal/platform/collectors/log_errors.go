// Package collectors provides signal collectors for the Cascade platform intelligence layer.
package collectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"github.com/yogirk/cascade/internal/platform"
	"google.golang.org/api/iterator"
)

// LogErrorCollector collects error signals from Cloud Logging.
type LogErrorCollector struct {
	getClient ClientProviderLog
	projectID string
	maxErrors int // threshold: above this count, severity becomes Critical
}

// ClientProviderLog returns a logadmin client (may be nil if unavailable).
type ClientProviderLog func() *logadmin.Client

// NewLogErrorCollector creates a collector that queries Cloud Logging for error entries.
// maxErrors is the threshold above which a grouped error becomes Critical severity.
func NewLogErrorCollector(getClient ClientProviderLog, projectID string, maxErrors int) *LogErrorCollector {
	if maxErrors <= 0 {
		maxErrors = 10
	}
	return &LogErrorCollector{
		getClient: getClient,
		projectID: projectID,
		maxErrors: maxErrors,
	}
}

// Source returns the signal source identifier.
func (c *LogErrorCollector) Source() platform.SignalSource {
	return platform.SourceLogging
}

// Collect queries Cloud Logging for entries with severity >= ERROR in the given
// time window, groups them by resource type + log name, and converts each group
// into a Signal.
func (c *LogErrorCollector) Collect(ctx context.Context, since time.Duration) ([]platform.Signal, error) {
	client := c.getClient()
	if client == nil {
		return nil, nil
	}

	// Build filter for errors within the time window.
	cutoff := time.Now().UTC().Add(-since)
	filter := fmt.Sprintf("timestamp >= %q AND severity >= ERROR", cutoff.Format(time.RFC3339))

	it := client.Entries(ctx, logadmin.Filter(filter), logadmin.NewestFirst())

	// Group entries by resource_type/log_name.
	type errorGroup struct {
		count        int
		firstMessage string
		latestTime   time.Time
		related      []string // deduplicated resource references
	}
	groups := make(map[string]*errorGroup)
	relatedSeen := make(map[string]map[string]bool) // groupKey -> set of related refs

	const maxEntries = 500 // cap to avoid reading unbounded entries
	for i := 0; i < maxEntries; i++ {
		entry, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("log query failed: %w", err)
		}

		resourceType := ""
		if entry.Resource != nil {
			resourceType = entry.Resource.Type
		}

		logName := entry.LogName
		if idx := strings.LastIndex(logName, "/"); idx >= 0 {
			logName = logName[idx+1:]
		}

		key := resourceType + "/" + logName

		g, ok := groups[key]
		if !ok {
			g = &errorGroup{}
			groups[key] = g
			relatedSeen[key] = make(map[string]bool)
		}
		g.count++

		if g.firstMessage == "" {
			g.firstMessage = extractEntryMessage(entry)
		}
		if entry.Timestamp.After(g.latestTime) {
			g.latestTime = entry.Timestamp
		}

		// Extract related resource references from the entry.
		for _, ref := range extractResourceRefs(entry) {
			if !relatedSeen[key][ref] {
				relatedSeen[key][ref] = true
				g.related = append(g.related, ref)
			}
		}
	}

	// Convert groups into signals.
	var signals []platform.Signal
	for key, g := range groups {
		severity := platform.SeverityWarning
		if g.count > c.maxErrors {
			severity = platform.SeverityCritical
		}

		preview := g.firstMessage
		if len(preview) > 100 {
			preview = preview[:97] + "..."
		}

		signals = append(signals, platform.Signal{
			Type:      platform.SignalLogError,
			Severity:  severity,
			Source:    platform.SourceLogging,
			Timestamp: g.latestTime,
			Summary:   fmt.Sprintf("%d errors in %s: %s", g.count, key, preview),
			Details: map[string]any{
				"error_count":   g.count,
				"group_key":     key,
				"first_message": g.firstMessage,
			},
			Related: g.related,
		})
	}

	return signals, nil
}

// extractEntryMessage pulls a readable summary from a logging.Entry payload.
func extractEntryMessage(entry *logging.Entry) string {
	if entry.Payload == nil {
		return ""
	}

	switch p := entry.Payload.(type) {
	case string:
		return truncate(p, 200)
	default:
		data, err := json.Marshal(p)
		if err != nil {
			return truncate(fmt.Sprintf("%v", p), 200)
		}

		var m map[string]interface{}
		if json.Unmarshal(data, &m) != nil {
			return truncate(string(data), 200)
		}

		// Walk common message locations.
		for _, path := range [][]string{
			{"message"},
			{"textPayload"},
			{"status", "message"},
			{"serviceData", "jobCompletedEvent", "job", "jobStatus", "error", "message"},
			{"metadata", "jobChange", "job", "jobStatus", "error", "message"},
		} {
			if msg := walkMap(m, path); msg != "" {
				return truncate(msg, 200)
			}
		}

		return truncate(string(data), 200)
	}
}

// walkMap follows a key path into a nested map and returns the string at the end.
func walkMap(m map[string]interface{}, keys []string) string {
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

// extractResourceRefs looks for table or bucket references in a log entry.
func extractResourceRefs(entry *logging.Entry) []string {
	var refs []string

	// Check resource labels for dataset/table references.
	if entry.Resource != nil {
		labels := entry.Resource.Labels
		if dataset, ok := labels["dataset_id"]; ok {
			if table, ok := labels["table_id"]; ok {
				project := labels["project_id"]
				if project == "" {
					refs = append(refs, dataset+"."+table)
				} else {
					refs = append(refs, project+"."+dataset+"."+table)
				}
			}
		}
		if bucket, ok := labels["bucket_name"]; ok {
			refs = append(refs, "gs://"+bucket)
		}
	}

	// Scan payload text for gs:// references.
	if entry.Payload != nil {
		var text string
		switch p := entry.Payload.(type) {
		case string:
			text = p
		default:
			if data, err := json.Marshal(p); err == nil {
				text = string(data)
			}
		}
		refs = append(refs, extractGCSRefs(text)...)
	}

	return refs
}

// extractGCSRefs finds gs://bucket/path references in text.
func extractGCSRefs(text string) []string {
	var refs []string
	remaining := text
	for {
		idx := strings.Index(remaining, "gs://")
		if idx < 0 {
			break
		}
		remaining = remaining[idx:]
		end := strings.IndexAny(remaining, " \t\n\r\"',;)")
		if end < 0 {
			end = len(remaining)
		}
		ref := remaining[:end]
		if len(ref) > 5 { // more than just "gs://"
			refs = append(refs, ref)
		}
		remaining = remaining[end:]
	}
	return refs
}

// truncate shortens a string, replacing newlines with spaces.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
