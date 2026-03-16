// Package platform provides cross-service platform intelligence for Cascade.
// It collects signals from BigQuery, Cloud Logging, GCS, and optional services,
// correlates them into incidents, and renders a morning briefing.
package platform

import "time"

// SignalType classifies the kind of platform signal.
type SignalType string

const (
	SignalJobFailed     SignalType = "job_failed"
	SignalTableStale    SignalType = "table_stale"
	SignalObjectMissing SignalType = "object_missing"
	SignalLogError      SignalType = "log_error"
	SignalCostSpike     SignalType = "cost_spike"
	SignalSecurityIssue SignalType = "security_issue"
)

// Severity indicates how critical a signal is.
type Severity int

const (
	SeverityInfo     Severity = iota // Informational, no action needed
	SeverityWarning                  // Worth investigating
	SeverityCritical                 // Needs immediate attention
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// SignalSource identifies which GCP service produced the signal.
type SignalSource string

const (
	SourceBigQuery  SignalSource = "bigquery"
	SourceLogging   SignalSource = "logging"
	SourceGCS       SignalSource = "gcs"
	SourceComposer  SignalSource = "composer"
	SourceScheduler SignalSource = "scheduler"
	SourceSchema    SignalSource = "schema"
)

// Signal represents a single platform observation from any GCP service.
type Signal struct {
	Type        SignalType
	Severity    Severity
	Source      SignalSource
	Timestamp   time.Time
	Summary     string         // 1-line human description
	Details     map[string]any // Service-specific data
	Related     []string       // Fully-qualified resource refs (e.g., "project.dataset.table", "gs://bucket/path")
	BlastRadius int            // Count of distinct downstream tables that read from the affected resource (0 = leaf/unknown)
}

// Incident groups correlated signals that share resources.
type Incident struct {
	Signals         []Signal // Grouped by shared Related entries
	TopSignal       Signal   // Highest severity in the group
	Resources       []string // Union of all Related across signals
	BlastRadius     int      // Max of constituent signals
	SuggestedAction string   // One-line next command
}

// MorningReport holds the complete /morning output.
type MorningReport struct {
	Incidents   []Incident
	Signals     []Signal // All signals before correlation
	SourceNotes []string // Per-source status notes ("BigQuery: not available", etc.)
	Since       time.Duration
	CollectedAt time.Time
}

// SourceResult holds signals and status from a single collector.
type SourceResult struct {
	Source  SignalSource
	Signals []Signal
	Err     error // Non-nil if the source failed entirely
	Note    string // Status note (e.g., "BigQuery: not available")
}
