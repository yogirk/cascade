// Package app assembles all Cascade components into a runnable application.
package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/genai"

	"github.com/yogirk/cascade/internal/agent"
	"github.com/yogirk/cascade/internal/auth"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/provider"
	"github.com/yogirk/cascade/internal/provider/gemini"
	"github.com/yogirk/cascade/internal/tools"
	"github.com/yogirk/cascade/internal/tools/core"
	"github.com/yogirk/cascade/pkg/types"
)

// App holds all assembled Cascade components.
type App struct {
	Config      *config.Config
	Agent       *agent.Agent
	Provider    provider.Provider
	Permissions *permission.Engine
	Events      chan types.Event
	BQ          *BigQueryComponents // nil if BQ not configured
}

// New creates a fully-wired App from the given configuration.
func New(ctx context.Context, cfg *config.Config) (*App, error) {
	// Auto-detect provider: if GOOGLE_API_KEY is set and provider isn't explicit, use gemini
	if cfg.Model.Provider == "" {
		if os.Getenv("GOOGLE_API_KEY") != "" {
			cfg.Model.Provider = "gemini"
		} else {
			cfg.Model.Provider = "vertex"
		}
	}

	clientConfig, ts, err := buildClientConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Provider
	prov, err := gemini.New(ctx, cfg.Model.Model, clientConfig)
	if err != nil {
		return nil, fmt.Errorf("provider setup failed: %w", err)
	}

	// Tools
	registry := tools.NewRegistry()
	core.RegisterAll(registry)

	// BigQuery (optional -- graceful if not configured)
	var bqComp *BigQueryComponents
	if ts != nil {
		// Vertex provider has a token source; reuse it for BQ
		bqComp, err = initBigQuery(ctx, cfg, ts)
		if err != nil {
			// Log warning but don't fail -- BQ is optional
			fmt.Fprintf(os.Stderr, "Warning: BigQuery init failed: %v\n", err)
			bqComp = nil
		}
	}

	// Register BQ tools alongside core tools
	registerBQTools(registry, bqComp, &cfg.Cost)

	// Permissions
	defaultMode := permission.ModeConfirm
	switch cfg.Security.DefaultMode {
	case "plan":
		defaultMode = permission.ModePlan
	case "bypass":
		defaultMode = permission.ModeBypass
	}
	perms := permission.NewEngine(defaultMode)

	// Events channel (buffered for TUI consumption)
	events := make(chan types.Event, 256)

	// Agent
	ag := agent.New(agent.AgentConfig{
		Provider:     prov,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: cfg.Agent.MaxToolCalls,
		SystemPrompt: BuildSystemPrompt(bqComp),
		Events:       agent.EventChan(events),
	})

	// Trigger lazy cache build if datasets are configured
	if bqComp != nil && len(cfg.BigQuery.Datasets) > 0 {
		bqComp.EnsureCachePopulated(ctx, cfg.BigQuery.Datasets, events)
	}

	return &App{
		Config:      cfg,
		Agent:       ag,
		Provider:    prov,
		Permissions: perms,
		Events:      events,
		BQ:          bqComp,
	}, nil
}

// buildClientConfig creates the GenAI client config based on the provider type.
// For the vertex provider, it also returns the oauth2.TokenSource so it can be
// reused for BigQuery (avoids creating duplicate credentials).
func buildClientConfig(ctx context.Context, cfg *config.Config) (*genai.ClientConfig, oauth2.TokenSource, error) {
	switch cfg.Model.Provider {
	case "gemini":
		// AI Studio: uses GOOGLE_API_KEY, no project/OAuth needed
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil, nil, fmt.Errorf("GOOGLE_API_KEY env var required for gemini provider")
		}
		return &genai.ClientConfig{
			Backend: genai.BackendGeminiAPI,
			APIKey:  apiKey,
		}, nil, nil

	case "vertex":
		// Vertex AI: uses OAuth + GCP project
		ts, err := auth.NewTokenSource(ctx, &auth.AuthConfig{
			ImpersonateServiceAccount: cfg.Auth.ImpersonateServiceAccount,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("auth setup failed: %w", err)
		}

		clientConfig := &genai.ClientConfig{
			Backend:    genai.BackendVertexAI,
			HTTPClient: oauth2.NewClient(ctx, ts),
			Project:    cfg.Model.Project,
		}

		// Auto-detect project if missing: ADC creds -> env var -> gcloud config
		if clientConfig.Project == "" {
			if creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform"); err == nil && creds.ProjectID != "" {
				clientConfig.Project = creds.ProjectID
			}
		}
		if clientConfig.Project == "" {
			clientConfig.Project = detectGCPProject()
		}
		if clientConfig.Project == "" {
			return nil, nil, fmt.Errorf("no GCP project found: set --project flag, GOOGLE_CLOUD_PROJECT env var, or run 'gcloud config set project <id>'")
		}

		if cfg.Model.Location != "" {
			clientConfig.Location = cfg.Model.Location
		}

		return clientConfig, ts, nil

	default:
		return nil, nil, fmt.Errorf("unknown provider %q: use \"gemini\" (API key) or \"vertex\" (GCP)", cfg.Model.Provider)
	}
}

// detectGCPProject tries GOOGLE_CLOUD_PROJECT env var, then gcloud CLI.
func detectGCPProject() string {
	if p := os.Getenv("GOOGLE_CLOUD_PROJECT"); p != "" {
		return p
	}
	if p := os.Getenv("GCLOUD_PROJECT"); p != "" {
		return p
	}
	out, err := exec.Command("gcloud", "config", "get-value", "project").Output()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" && p != "(unset)" {
			return p
		}
	}
	return ""
}

// systemPrompt moved to prompt.go as baseSystemPrompt; dynamic version built by BuildSystemPrompt.
