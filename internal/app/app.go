// Package app assembles all Cascade components into a runnable application.
package app

import (
	"context"
	"fmt"

	"github.com/cascade-cli/cascade/internal/agent"
	"github.com/cascade-cli/cascade/internal/auth"
	"github.com/cascade-cli/cascade/internal/config"
	"github.com/cascade-cli/cascade/internal/permission"
	"github.com/cascade-cli/cascade/internal/provider"
	"github.com/cascade-cli/cascade/internal/provider/gemini"
	"github.com/cascade-cli/cascade/internal/tools"
	"github.com/cascade-cli/cascade/internal/tools/core"
	"github.com/cascade-cli/cascade/pkg/types"
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
	// Auth
	_, err := auth.NewTokenSource(ctx, &auth.AuthConfig{
		ImpersonateServiceAccount: cfg.Auth.ImpersonateServiceAccount,
	})
	if err != nil {
		return nil, fmt.Errorf("auth setup failed: %w", err)
	}

	// Provider
	prov, err := gemini.New(ctx, cfg.Model.Model, nil)
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

const systemPrompt = `You are Cascade, an AI assistant for GCP data engineering. You help users diagnose pipeline failures, investigate costs, write queries, and manage their GCP data stack through conversational interaction.`
