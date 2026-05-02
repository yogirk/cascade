package app

import (
	"github.com/slokam-ai/cascade/internal/config"
	plat "github.com/slokam-ai/cascade/internal/platform"
	"github.com/slokam-ai/cascade/internal/platform/collectors"
)

// buildMorningCollector assembles signal collectors from available platform components.
// Each collector is optional — if a component is nil, its collector is skipped.
func buildMorningCollector(bq *BigQueryComponents, platform *PlatformComponents, cfg *config.Config, cascadeMD *config.CascadeMD) *plat.PlatformCollector {
	var colls []plat.SignalCollector

	// Extract critical table refs from CASCADE.md
	var criticalTables []string
	staleHours := 24
	if cascadeMD != nil {
		for _, ct := range cascadeMD.CriticalTables {
			ref := ct.Dataset + "." + ct.Table
			if ct.Project != "" {
				ref = ct.Project + "." + ref
			}
			criticalTables = append(criticalTables, ref)
		}
		if cascadeMD.Thresholds.StaleHours > 0 {
			staleHours = cascadeMD.Thresholds.StaleHours
		}
	}

	// BQ job failure collector
	if bq != nil && bq.Client != nil {
		location := cfg.BigQuery.Location
		if location == "" {
			location = "US"
		}
		colls = append(colls, collectors.NewBQJobCollector(
			bq.Client, bq.Client.ProjectID(), location, criticalTables,
		))
	}

	// Schema staleness collector
	if bq != nil && bq.Cache != nil {
		colls = append(colls, collectors.NewSchemaStaleCollector(
			bq.Cache, staleHours, criticalTables,
		))
	}

	// Log error collector
	if platform != nil {
		maxErrors := 10
		if cascadeMD != nil && cascadeMD.Thresholds.MaxLogErrors > 0 {
			maxErrors = cascadeMD.Thresholds.MaxLogErrors
		}
		colls = append(colls, collectors.NewLogErrorCollector(
			platform.GetLogClient, cfg.GCP.Project, maxErrors,
		))
	}

	// GCS freshness collector (uses expectations from CASCADE.md if available)
	// V1: GCS expectations are empty unless CASCADE.md defines GCS paths in future.
	if platform != nil {
		colls = append(colls, collectors.NewGCSFreshnessCollector(
			platform.GetStorageClient, cfg.GCP.Project, nil,
		))
	}

	if len(colls) == 0 {
		return nil
	}

	return plat.NewPlatformCollector(colls...)
}
