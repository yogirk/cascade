package collectors

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/slokam-ai/cascade/internal/platform"
	"google.golang.org/api/iterator"
)

// GCSExpectation defines an expected GCS path and its freshness interval.
// If no objects under Bucket/Prefix have been updated within Interval,
// a SignalObjectMissing signal is emitted.
type GCSExpectation struct {
	Bucket   string
	Prefix   string
	Interval time.Duration
}

// GCSFreshnessCollector checks for expected GCS objects that may be missing
// or stale. It is configured with a set of expectations (bucket + prefix +
// interval) typically loaded from CASCADE.md project configuration.
type GCSFreshnessCollector struct {
	getClient    ClientProviderGCS
	projectID    string
	expectations []GCSExpectation
}

// ClientProviderGCS returns a storage client (may be nil if unavailable).
type ClientProviderGCS func() *storage.Client

// NewGCSFreshnessCollector creates a collector that checks configured GCS paths
// for freshness. If expectations is empty, the collector is a no-op.
func NewGCSFreshnessCollector(getClient ClientProviderGCS, projectID string, expectations []GCSExpectation) *GCSFreshnessCollector {
	return &GCSFreshnessCollector{
		getClient:    getClient,
		projectID:    projectID,
		expectations: expectations,
	}
}

// Source returns the signal source identifier.
func (c *GCSFreshnessCollector) Source() platform.SignalSource {
	return platform.SourceGCS
}

// Collect checks each configured expectation and returns signals for paths
// that have no recent objects within the expected interval. If no expectations
// are configured, returns empty (no-op).
func (c *GCSFreshnessCollector) Collect(ctx context.Context, since time.Duration) ([]platform.Signal, error) {
	// No-op if nothing is configured.
	if len(c.expectations) == 0 {
		return nil, nil
	}

	client := c.getClient()
	if client == nil {
		return nil, nil
	}

	var signals []platform.Signal

	for _, exp := range c.expectations {
		signal, err := c.checkExpectation(ctx, client, exp)
		if err != nil {
			// Individual expectation failures produce a signal rather than
			// aborting the entire collection, so other expectations still run.
			signals = append(signals, platform.Signal{
				Type:      platform.SignalObjectMissing,
				Severity:  platform.SeverityWarning,
				Source:    platform.SourceGCS,
				Timestamp: time.Now(),
				Summary:   fmt.Sprintf("Unable to check gs://%s/%s: %v", exp.Bucket, exp.Prefix, err),
				Details: map[string]any{
					"bucket": exp.Bucket,
					"prefix": exp.Prefix,
					"error":  err.Error(),
				},
				Related: []string{fmt.Sprintf("gs://%s/%s", exp.Bucket, exp.Prefix)},
			})
			continue
		}
		if signal != nil {
			signals = append(signals, *signal)
		}
	}

	return signals, nil
}

// checkExpectation lists recent objects under the expected path and returns
// a signal if nothing was updated within the expected interval.
func (c *GCSFreshnessCollector) checkExpectation(ctx context.Context, client *storage.Client, exp GCSExpectation) (*platform.Signal, error) {
	query := &storage.Query{
		Prefix: exp.Prefix,
	}
	// Only fetch the name and updated time to minimize data transfer.
	if err := query.SetAttrSelection([]string{"Name", "Updated"}); err != nil {
		return nil, fmt.Errorf("set attr selection: %w", err)
	}

	it := client.Bucket(exp.Bucket).Objects(ctx, query)

	cutoff := time.Now().Add(-exp.Interval)
	foundAny := false
	foundRecent := false

	// Scan up to a reasonable number of objects looking for a recent one.
	const maxScan = 200
	for i := 0; i < maxScan; i++ {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list objects in gs://%s/%s: %w", exp.Bucket, exp.Prefix, err)
		}

		foundAny = true
		if attrs.Updated.After(cutoff) {
			foundRecent = true
			break
		}
	}

	if foundRecent {
		return nil, nil // All good — recent object found.
	}

	// Construct summary based on whether we found any objects at all.
	var summary string
	path := fmt.Sprintf("gs://%s/%s", exp.Bucket, exp.Prefix)
	interval := formatInterval(exp.Interval)

	if !foundAny {
		summary = fmt.Sprintf("No objects found in %s (expected within %s)", path, interval)
	} else {
		summary = fmt.Sprintf("No recent objects in %s (expected within %s)", path, interval)
	}

	return &platform.Signal{
		Type:      platform.SignalObjectMissing,
		Severity:  platform.SeverityWarning,
		Source:    platform.SourceGCS,
		Timestamp: time.Now(),
		Summary:   summary,
		Details: map[string]any{
			"bucket":            exp.Bucket,
			"prefix":            exp.Prefix,
			"expected_interval": exp.Interval.String(),
			"found_objects":     foundAny,
		},
		Related: []string{path},
	}, nil
}

// formatInterval returns a human-friendly string for a duration.
func formatInterval(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if d >= time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	return d.String()
}
