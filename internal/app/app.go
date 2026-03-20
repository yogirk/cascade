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

	clientConfig, err := buildClientConfig(ctx, cfg)
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
		SystemPrompt: systemPrompt,
		Events:       agent.EventChan(events),
	})

	return &App{
		Config:      cfg,
		Agent:       ag,
		Provider:    prov,
		Permissions: perms,
		Events:      events,
	}, nil
}

// buildClientConfig creates the GenAI client config based on the provider type.
func buildClientConfig(ctx context.Context, cfg *config.Config) (*genai.ClientConfig, error) {
	switch cfg.Model.Provider {
	case "gemini":
		// AI Studio: uses GOOGLE_API_KEY, no project/OAuth needed
		apiKey := os.Getenv("GOOGLE_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GOOGLE_API_KEY env var required for gemini provider")
		}
		return &genai.ClientConfig{
			Backend: genai.BackendGeminiAPI,
			APIKey:  apiKey,
		}, nil

	case "vertex":
		// Vertex AI: uses OAuth + GCP project
		ts, err := auth.NewTokenSource(ctx, &auth.AuthConfig{
			ImpersonateServiceAccount: cfg.Auth.ImpersonateServiceAccount,
		})
		if err != nil {
			return nil, fmt.Errorf("auth setup failed: %w", err)
		}

		clientConfig := &genai.ClientConfig{
			Backend:    genai.BackendVertexAI,
			HTTPClient: oauth2.NewClient(ctx, ts),
			Project:    cfg.Model.Project,
		}

		// Auto-detect project if missing: ADC creds → env var → gcloud config
		if clientConfig.Project == "" {
			if creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform"); err == nil && creds.ProjectID != "" {
				clientConfig.Project = creds.ProjectID
			}
		}
		if clientConfig.Project == "" {
			clientConfig.Project = detectGCPProject()
		}
		if clientConfig.Project == "" {
			return nil, fmt.Errorf("no GCP project found: set --project flag, GOOGLE_CLOUD_PROJECT env var, or run 'gcloud config set project <id>'")
		}

		if cfg.Model.Location != "" {
			clientConfig.Location = cfg.Model.Location
		}

		return clientConfig, nil

	default:
		return nil, fmt.Errorf("unknown provider %q: use \"gemini\" (API key) or \"vertex\" (GCP)", cfg.Model.Provider)
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

const systemPrompt = `You are Cascade, an AI assistant for GCP data engineering. You help users diagnose pipeline failures, investigate costs, write queries, and manage their GCP data stack through conversational interaction.`
