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
	Populator   *schema.Populator
	CostTracker *bq.CostTracker
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

	// Create BQ client using Resource Plane credentials
	client, err := bq.NewClient(ctx, project, location, resource.TokenSource, cfg.Cost.PricePerTB)
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
func registerBQTools(registry *tools.Registry, comp *BigQueryComponents, costCfg *config.CostConfig, events chan types.Event) {
	if comp == nil {
		return
	}
	queryTool := bqtools.NewQueryTool(comp.Client, comp.Cache, comp.CostTracker, costCfg, events)
	schemaTool := bqtools.NewSchemaTool(comp.Cache, comp.Client.ProjectID())
	bqtools.RegisterAll(registry, queryTool, schemaTool)
}

// EnsureCachePopulated triggers lazy cache build if not yet populated.
// Runs in a background goroutine so it never blocks the caller.
// If onCacheReady is non-nil, it is called after successful population.
func (c *BigQueryComponents) EnsureCachePopulated(ctx context.Context, datasets []string, events chan types.Event, onCacheReady func()) {
	if c == nil || c.Cache.IsPopulated() || len(datasets) == 0 {
		return
	}
	go func() {
		if events != nil {
			events <- &types.StatusEvent{Message: "Building schema cache..."}
		}
		err := c.Populator.PopulateAll(ctx, datasets, func(completed, total int) {
			if events != nil {
				events <- &types.StatusEvent{
					Message: fmt.Sprintf("Building schema cache... %d/%d tables", completed, total),
				}
			}
		})
		if err != nil && events != nil {
			events <- &types.ErrorEvent{Err: fmt.Errorf("lazy cache build failed: %w", err)}
			return
		}
		if events != nil {
			events <- &types.StatusEvent{Message: "Schema cache ready"}
		}
		if err == nil && onCacheReady != nil {
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
	it, err := a.client.RunQuery(ctx, sql)
	if err != nil {
		return nil, err
	}
	return &bqRowIteratorAdapter{it: it}, nil
}

// bqRowIteratorAdapter translates between bigquery.Value and interface{}
// so that populate.go can work without importing the bigquery package.
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
	// Convert []bigquery.Value to []interface{} and assign to dst.
	if ptr, ok := dst.(*[]interface{}); ok {
		result := make([]interface{}, len(row))
		for i, v := range row {
			result[i] = v
		}
		*ptr = result
	}
	return nil
}

func (a *bqClientAdapter) ProjectID() string {
	return a.client.ProjectID()
}
