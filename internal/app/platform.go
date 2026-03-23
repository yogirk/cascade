package app

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/logging/logadmin"
	"cloud.google.com/go/storage"
	"github.com/yogirk/cascade/internal/auth"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/tools"
	gcstool "github.com/yogirk/cascade/internal/tools/gcs"
	logtool "github.com/yogirk/cascade/internal/tools/logging"
	"google.golang.org/api/option"
)

// PlatformComponents holds non-BQ GCP platform clients.
type PlatformComponents struct {
	mu            sync.Mutex
	LogClient     *logadmin.Client
	StorageClient *storage.Client
	ProjectID     string
}

// initPlatform creates Cloud Logging and GCS clients if GCP auth is available.
// Failures are non-fatal — each client is independently optional.
func initPlatform(ctx context.Context, cfg *config.Config, resource *auth.ResourceAuth) *PlatformComponents {
	if resource == nil || !resource.Available {
		return nil
	}

	project := cfg.GCP.Project
	if project == "" {
		project = resource.Project
	}
	if project == "" {
		return nil
	}

	comp := &PlatformComponents{ProjectID: project}
	opt := option.WithTokenSource(resource.TokenSource)

	// Init clients async — don't block startup.
	// Tools check for nil client at call time.
	go func() {
		logClient, err := logadmin.NewClient(ctx, project, opt)
		if err == nil {
			comp.mu.Lock()
			comp.LogClient = logClient
			comp.mu.Unlock()
		}
	}()

	go func() {
		storageClient, err := storage.NewClient(ctx, opt)
		if err == nil {
			comp.mu.Lock()
			comp.StorageClient = storageClient
			comp.mu.Unlock()
		}
	}()

	return comp
}

// registerPlatformTools registers Cloud Logging and GCS tools if clients are available.
func registerPlatformTools(registry *tools.Registry, comp *PlatformComponents, cfg *config.Config) {
	if comp == nil {
		return
	}

	// Register with lazy client providers — clients init async at startup
	maxEntries := cfg.Logging.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 50
	}
	lt := logtool.NewLogTool(comp.GetLogClient, comp.ProjectID, maxEntries)
	registry.Register(lt)

	maxLines := cfg.GCS.MaxReadLines
	if maxLines <= 0 {
		maxLines = 100
	}
	gt := gcstool.NewGCSTool(comp.GetStorageClient, comp.ProjectID, maxLines)
	registry.Register(gt)
}

// GetLogClient returns the logging client (thread-safe, may return nil if still initializing).
func (c *PlatformComponents) GetLogClient() *logadmin.Client {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.LogClient
}

// GetStorageClient returns the storage client (thread-safe, may return nil if still initializing).
func (c *PlatformComponents) GetStorageClient() *storage.Client {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.StorageClient
}

// Close cleans up platform clients.
func (c *PlatformComponents) Close() {
	if c == nil {
		return
	}
	if c.LogClient != nil {
		c.LogClient.Close()
	}
	if c.StorageClient != nil {
		c.StorageClient.Close()
	}
}

// platformStatus returns a summary of available platform tools.
func platformStatus(comp *PlatformComponents) string {
	if comp == nil {
		return ""
	}
	var parts []string
	if comp.LogClient != nil {
		parts = append(parts, "Logging")
	}
	if comp.StorageClient != nil {
		parts = append(parts, "GCS")
	}
	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf("Platform: %s", joinAnd(parts))
}

func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return parts[0] + " + " + parts[1]
	}
}
