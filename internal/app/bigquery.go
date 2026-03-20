package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"

	bq "github.com/yogirk/cascade/internal/bigquery"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/schema"
	"github.com/yogirk/cascade/internal/tools"
	bqtools "github.com/yogirk/cascade/internal/tools/bigquery"
	"github.com/yogirk/cascade/pkg/types"
)

// BigQueryComponents holds all BQ-related runtime objects.
type BigQueryComponents struct {
	Client      *bq.Client
	Cache       *schema.Cache
	Populator   *schema.Populator
	CostTracker *bq.CostTracker
}

// initBigQuery creates BQ components if configuration is present.
// Returns nil if no BQ project is configured (graceful degradation).
func initBigQuery(ctx context.Context, cfg *config.Config, ts oauth2.TokenSource) (*BigQueryComponents, error) {
	// BQ project: use bigquery.project config, fall back to model.project
	project := cfg.BigQuery.Project
	if project == "" {
		project = cfg.Model.Project
	}
	if project == "" {
		// No project = no BQ (user hasn't configured it)
		return nil, nil
	}

	// Location: use bigquery.location, fall back to "US"
	location := cfg.BigQuery.Location
	if location == "" {
		location = "US"
	}

	// Create BQ client
	client, err := bq.NewClient(ctx, project, location, ts, cfg.Cost.PricePerTB)
	if err != nil {
		return nil, fmt.Errorf("BigQuery client failed: %w", err)
	}

	// Create schema cache
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	cacheDir := filepath.Join(home, ".cascade", "cache")

	cache := schema.NewCache(cacheDir)
	if err := cache.Open(project); err != nil {
		client.Close()
		return nil, fmt.Errorf("schema cache failed: %w", err)
	}

	// Create populator and cost tracker
	populator := schema.NewPopulator(cache, &bqClientAdapter{client: client})
	costTracker := bq.NewCostTracker(cfg.Cost.DailyBudget)

	return &BigQueryComponents{
		Client:      client,
		Cache:       cache,
		Populator:   populator,
		CostTracker: costTracker,
	}, nil
}

// registerBQTools creates and registers BQ tools if components are available.
func registerBQTools(registry *tools.Registry, comp *BigQueryComponents, costCfg *config.CostConfig) {
	if comp == nil {
		return
	}
	queryTool := bqtools.NewQueryTool(comp.Client, comp.Cache, comp.CostTracker, costCfg)
	schemaTool := bqtools.NewSchemaTool(comp.Cache, comp.Client.ProjectID())
	costTool := bqtools.NewCostTool(comp.Client)
	bqtools.RegisterAll(registry, queryTool, schemaTool, costTool)
}

// EnsureCachePopulated triggers lazy cache build if not yet populated.
// Runs in a background goroutine so it never blocks the caller.
func (c *BigQueryComponents) EnsureCachePopulated(ctx context.Context, datasets []string, events chan types.Event) {
	if c == nil || c.Cache.IsPopulated() || len(datasets) == 0 {
		return
	}
	go func() {
		err := c.Populator.PopulateAll(ctx, datasets, nil)
		if err != nil && events != nil {
			events <- &types.ErrorEvent{Err: fmt.Errorf("lazy cache build failed: %w", err)}
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
	if c.Client != nil {
		c.Client.Close()
	}
}

// bqClientAdapter adapts *bq.Client to the schema.BQQuerier interface.
// The adapter is needed because bq.Client.RunQuery returns *bigquery.RowIterator
// (concrete type) while schema.BQQuerier expects schema.RowIterator (interface).
type bqClientAdapter struct {
	client *bq.Client
}

func (a *bqClientAdapter) RunQuery(ctx context.Context, sql string) (schema.RowIterator, error) {
	return a.client.RunQuery(ctx, sql)
}

func (a *bqClientAdapter) ProjectID() string {
	return a.client.ProjectID()
}
