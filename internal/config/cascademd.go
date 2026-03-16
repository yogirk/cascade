package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// CascadeMD holds project-specific configuration from CASCADE.md.
// Discovered by walking up from the current directory to the git root,
// similar to how CLAUDE.md works in Claude Code.
type CascadeMD struct {
	// CriticalTables lists tables that matter most for blast radius and alerting.
	CriticalTables []CriticalTable `toml:"critical_tables"`

	// Schedules defines expected refresh cadences for tables.
	Schedules []TableSchedule `toml:"schedules"`

	// Thresholds overrides default alerting thresholds.
	Thresholds AlertThresholds `toml:"thresholds"`

	// Playbook contains custom hints injected into the system prompt.
	Playbook string `toml:"playbook"`
}

// CriticalTable marks a table as important for monitoring.
type CriticalTable struct {
	Project string `toml:"project"`
	Dataset string `toml:"dataset"`
	Table   string `toml:"table"`
	Owner   string `toml:"owner"` // Team or person responsible
}

// TableSchedule defines expected refresh timing for a table.
type TableSchedule struct {
	Project       string `toml:"project"`
	Dataset       string `toml:"dataset"`
	Table         string `toml:"table"`
	IntervalHours int    `toml:"interval_hours"` // Expected refresh interval
	Description   string `toml:"description"`    // "Daily ETL at 3am UTC"
}

// AlertThresholds configures when signals are generated.
type AlertThresholds struct {
	StaleHours     int     `toml:"stale_hours"`      // Hours before a table is considered stale (default: 24)
	CostSpikeRatio float64 `toml:"cost_spike_ratio"`  // Ratio above average to trigger cost spike (default: 3.0)
	MaxLogErrors   int     `toml:"max_log_errors"`    // Error count threshold for log_error signal (default: 10)
}

// LoadCascadeMD searches for CASCADE.md starting from the current directory,
// walking up to the git root. Returns nil (not an error) if not found.
func LoadCascadeMD() (*CascadeMD, error) {
	path := findCascadeMD()
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var md CascadeMD
	if err := toml.Unmarshal(data, &md); err != nil {
		return nil, err
	}

	// Apply defaults
	if md.Thresholds.StaleHours <= 0 {
		md.Thresholds.StaleHours = 24
	}
	if md.Thresholds.CostSpikeRatio <= 0 {
		md.Thresholds.CostSpikeRatio = 3.0
	}
	if md.Thresholds.MaxLogErrors <= 0 {
		md.Thresholds.MaxLogErrors = 10
	}

	return &md, nil
}

// findCascadeMD walks up from cwd looking for CASCADE.md, stopping at git root.
func findCascadeMD() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		candidate := filepath.Join(dir, "CASCADE.md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}

		// Stop at git root
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// Also check ~/.cascade/CASCADE.md for global defaults
	home, err := os.UserHomeDir()
	if err == nil {
		global := filepath.Join(home, ".cascade", "CASCADE.md")
		if _, err := os.Stat(global); err == nil {
			return global
		}
	}

	return ""
}
