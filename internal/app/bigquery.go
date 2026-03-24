package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cloud.google.com/go/bigquery"
	"github.com/yogirk/cascade/internal/auth"
	bq "github.com/yogirk/cascade/internal/bigquery"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/schema"
	"github.com/yogirk/cascade/internal/tools"
	bqtools "github.com/yogirk/cascade/internal/tools/bigquery"
	"github.com/yogirk/cascade/pkg/types"
	"google.golang.org/api/iterator"
)

// BigQueryComponents holds all BQ-related runtime objects.
type BigQueryComponents struct {
	Client      *bq.Client
	Cache       *schema.Cache
	Populators  map[string]*populatorEntry // keyed by project ID
	CostTracker *bq.CostTracker
	clients     map[string]*bq.Client // all clients for cleanup
}

// populatorEntry pairs a populator with the datasets it should populate.
type populatorEntry struct {
	Populator *schema.Populator
	Datasets  []string
}

// initBigQuery creates BQ components if configuration is present.
// Uses Resource Plane credentials, independent of LLM provider choice.
// Returns nil if GCP auth is unavailable or no project is configured.
func initBigQuery(ctx context.Context, cfg *config.Config, resource *auth.ResourceAuth) (*BigQueryComponents, error) {
	if resource == nil || !resource.Available {
		return nil, nil // No GCP auth — graceful degradation
	}

	// BQ project: bigquery.project > gcp.project (via resource)
	project := cfg.BigQuery.Project
	if project == "" {
		project = resource.Project
	}
	if project == "" {
		return nil, nil // No project = no BQ
	}

	// Location: bigquery.location, default "US"
	location := cfg.BigQuery.Location
	if location == "" {
		location = "US"
	}

	// Create main BQ client using Resource Plane credentials
	client, err := bq.NewClient(ctx, project, location, resource.TokenSource, cfg.Cost.PricePerTB)
	if err != nil {
		return nil, fmt.Errorf("BigQuery client failed: %w", err)
	}

	// Create unified schema cache
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	cacheDir := filepath.Join(home, ".cascade", "cache")

	cache := schema.NewCache(cacheDir)
	if err := cache.Open(); err != nil {
		client.Close()
		return nil, fmt.Errorf("schema cache failed: %w", err)
	}

	// Build populators map
	populators := make(map[string]*populatorEntry)
	clients := map[string]*bq.Client{project: client}

	// Main project populator
	if len(cfg.BigQuery.Datasets) > 0 {
		mainPopulator := schema.NewPopulator(cache, &bqClientAdapter{client: client})
		datasets := cfg.BigQuery.Datasets

		// If billing is in the same project, add billing dataset to main list
		billingProject := cfg.Cost.BillingProject
		billingDataset := cfg.Cost.BillingDataset
		if billingDataset != "" && (billingProject == "" || billingProject == project) {
			datasets = appendUnique(datasets, billingDataset)
		}

		populators[project] = &populatorEntry{
			Populator: mainPopulator,
			Datasets:  datasets,
		}
	}

	// Cross-project billing populator (if different project)
	billingProject := cfg.Cost.BillingProject
	billingDataset := cfg.Cost.BillingDataset
	if billingProject != "" && billingDataset != "" && billingProject != project {
		billingClient, err := bq.NewClient(ctx, billingProject, location, resource.TokenSource, cfg.Cost.PricePerTB)
		if err == nil {
			clients[billingProject] = billingClient
			billingPopulator := schema.NewPopulator(cache, &bqClientAdapter{client: billingClient})
			populators[billingProject] = &populatorEntry{
				Populator: billingPopulator,
				Datasets:  []string{billingDataset},
			}
		}
		// Non-fatal: billing cache is optional
	}

	costTracker := bq.NewCostTracker(cfg.Cost.DailyBudget)

	return &BigQueryComponents{
		Client:      client,
		Cache:       cache,
		Populators:  populators,
		CostTracker: costTracker,
		clients:     clients,
	}, nil
}

// registerBQTools creates and registers BQ tools if components are available.
func registerBQTools(registry *tools.Registry, comp *BigQueryComponents, costCfg *config.CostConfig, events chan types.Event) {
	if comp == nil {
		return
	}
	queryTool := bqtools.NewQueryTool(comp.Client, comp.Cache, comp.Client.ProjectID(), comp.CostTracker, costCfg, events)
	schemaTool := bqtools.NewSchemaTool(comp.Cache, comp.Client.ProjectID())
	bqtools.RegisterAll(registry, queryTool, schemaTool)
}

// EnsureCachePopulated triggers lazy cache build for all configured projects/datasets.
// Runs in a background goroutine so it never blocks the caller.
// If onCacheReady is non-nil, it is called after successful population.
func (c *BigQueryComponents) EnsureCachePopulated(ctx context.Context, events chan types.Event, onCacheReady func()) {
	if c == nil || len(c.Populators) == 0 {
		return
	}

	// Skip if already populated
	if c.Cache.IsPopulated() {
		return
	}

	go func() {
		if events != nil {
			events <- &types.StatusEvent{Message: "Building schema cache..."}
		}

		var totalTables int
		for projectID, entry := range c.Populators {
			err := entry.Populator.PopulateAll(ctx, entry.Datasets, func(completed, total int) {
				if events != nil {
					events <- &types.StatusEvent{
						Message: fmt.Sprintf("Building schema cache... %s %d/%d tables", projectID, completed, total),
					}
				}
			})
			if err != nil && events != nil {
				events <- &types.ErrorEvent{Err: fmt.Errorf("cache build failed for %s: %w", projectID, err)}
			}
			totalTables += len(entry.Datasets)
		}

		if events != nil {
			events <- &types.StatusEvent{Message: "Schema cache ready"}
		}
		if onCacheReady != nil {
			onCacheReady()
		}
	}()
}

// Close cleans up BigQuery resources.
func (c *BigQueryComponents) Close() {
	if c == nil {
		return
	}
	if c.Cache != nil {
		c.Cache.Close()
	}
	for _, client := range c.clients {
		if client != nil {
			client.Close()
		}
	}
}

// appendUnique appends s to slice if not already present.
func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}

// bqClientAdapter adapts *bq.Client to the schema.BQQuerier interface.
type bqClientAdapter struct {
	client *bq.Client
}

func (a *bqClientAdapter) RunQuery(ctx context.Context, sql string) (schema.RowIterator, error) {
	it, err := a.client.RunQuery(ctx, sql)
	if err != nil {
		return nil, err
	}
	return &bqRowIteratorAdapter{it: it}, nil
}

func (a *bqClientAdapter) ProjectID() string {
	return a.client.ProjectID()
}

// bqRowIteratorAdapter translates between bigquery.Value and interface{}
type bqRowIteratorAdapter struct {
	it *bigquery.RowIterator
}

func (a *bqRowIteratorAdapter) Next(dst interface{}) error {
	var row []bigquery.Value
	if err := a.it.Next(&row); err != nil {
		if err == iterator.Done {
			return fmt.Errorf("iterator done")
		}
		return err
	}
	if ptr, ok := dst.(*[]interface{}); ok {
		result := make([]interface{}, len(row))
		for i, v := range row {
			result[i] = v
		}
		*ptr = result
	}
	return nil
}
