// Package app assembles all Cascade components into a runnable application.
package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/slokam-ai/cascade/internal/agent"
	"github.com/slokam-ai/cascade/internal/auth"
	"github.com/slokam-ai/cascade/internal/config"
	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/internal/persist"
	"github.com/slokam-ai/cascade/internal/provider"
	antprov "github.com/slokam-ai/cascade/internal/provider/anthropic"
	"github.com/slokam-ai/cascade/internal/provider/gemini"
	oaiprov "github.com/slokam-ai/cascade/internal/provider/openai"
	plat "github.com/slokam-ai/cascade/internal/platform"
	"github.com/slokam-ai/cascade/internal/tools"
	"github.com/slokam-ai/cascade/internal/tools/core"
	"github.com/slokam-ai/cascade/pkg/types"
)

// Options controls optional behaviors during app initialization.
type Options struct {
	ResumeSession bool   // resume the most recent session
	SessionID     string // resume a specific session by ID
}

// App holds all assembled Cascade components.
type App struct {
	Config      *config.Config
	Agent       *agent.Agent
	Provider    provider.Provider
	Registry    *tools.Registry
	Permissions *permission.Engine
	Events      chan types.Event
	Approvals   chan types.ApprovalRequest
	BQ          *BigQueryComponents  // nil if BQ not configured
	Platform    *PlatformComponents  // nil if GCP auth unavailable
	Resource    *auth.ResourceAuth   // GCP platform credentials
	Morning     *plat.PlatformCollector // nil if no sources available
	CascadeMD   *config.CascadeMD      // nil if no CASCADE.md found
	Sessions    *persist.SQLiteStore   // nil if session persistence unavailable
	SessionID   string                 // current session ID
	Version     string                 // app version (set by caller)
}

// New creates a fully-wired App from the given configuration.
func New(ctx context.Context, cfg *config.Config, opts ...Options) (*App, error) {
	var opt Options
	if len(opts) > 0 {
		opt = opts[0]
	}
	// ── 1. Resource Plane: GCP platform auth (always attempt) ──
	resource := auth.ResolveResourceAuth(ctx,
		cfg.GCP.Project,
		cfg.GCP.Location,
		cfg.GCP.Auth.Mode,
		cfg.GCP.Auth.ImpersonateServiceAccount,
		cfg.GCP.Auth.CredentialsFile,
	)

	// ── 2. Model Plane: LLM provider auth ──
	modelAuth, err := auth.ResolveModelAuth(
		cfg.Model.Provider,
		cfg.Model.Model,
		resource,
		cfg.Model.Vertex.Project,
		cfg.Model.Vertex.Location,
		cfg.Model.GeminiAPI.APIKeyEnv,
		cfg.Model.OpenAI.APIKeyEnv,
		cfg.Model.Anthropic.APIKeyEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("LLM provider setup failed: %w", err)
	}

	// ── 3. Build LLM provider ──
	prov, err := buildProvider(ctx, cfg.Model.Model, modelAuth)
	if err != nil {
		return nil, err
	}

	// ── 4. Tools ──
	registry := tools.NewRegistry()
	core.RegisterAll(registry)

	// ── 5. BigQuery (uses Resource Plane, independent of LLM choice) ──
	var bqComp *BigQueryComponents
	bqComp, err = initBigQuery(ctx, cfg, resource)
	if err != nil {
		// Log warning but don't fail — BQ is optional
		fmt.Fprintf(os.Stderr, "Warning: BigQuery init failed: %v\n", err)
		bqComp = nil
	}

	// Events channels
	events := make(chan types.Event, 256)
	approvals := make(chan types.ApprovalRequest, 8)

	// Register BQ tools alongside core tools
	registerBQTools(registry, bqComp, &cfg.Cost, events)

	// ── 5b. Platform tools (Logging, GCS) ──
	platform := initPlatform(ctx, cfg, resource)
	registerPlatformTools(registry, platform, cfg)

	// ── 6. Permissions ──
	defaultMode := permission.ParseMode(cfg.Security.DefaultMode)
	perms := permission.NewEngine(defaultMode)

	// ── 7. Agent ──
	ag := agent.New(agent.AgentConfig{
		Provider:     prov,
		Registry:     registry,
		Permissions:  perms,
		MaxToolCalls: cfg.Agent.MaxToolCalls,
		ToolTimeout:  cfg.Agent.ToolTimeout,
		SystemPrompt: BuildSystemPrompt(bqComp, cfg),
		Events:       agent.EventChan(events),
		Approvals:    approvals,
	})

	// Trigger lazy cache build for all configured projects/datasets
	if bqComp != nil {
		bqComp.EnsureCachePopulated(ctx, events, func() {
			ag.Session().SetSystemPrompt(BuildSystemPrompt(bqComp, cfg))
		})
	}

	// ── 8. CASCADE.md project config ──
	cascadeMD, err := config.LoadCascadeMD()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ⚠ CASCADE.md parse error: %v (using defaults)\n", err)
		cascadeMD = nil
	}

	// ── 9. Morning collector (platform intelligence) ──
	morning := buildMorningCollector(bqComp, platform, cfg, cascadeMD)

	// ── 10. Session persistence ──
	homeDir, _ := os.UserHomeDir()
	cascadeDir := filepath.Join(homeDir, ".cascade")
	var sessionStore *persist.SQLiteStore
	var sessionID string
	if store, err := persist.OpenSQLite(cascadeDir); err == nil {
		sessionStore = store
		sessionID = persist.GenerateSessionID()

		// Resume existing session if requested
		if opt.SessionID != "" {
			if meta, msgs, err := store.LoadSession(opt.SessionID); err == nil {
				ag.Session().Replace(stripSystemMessages(msgs))
				sessionID = meta.ID
			} else {
				fmt.Fprintf(os.Stderr, "  ⚠ Session %q not found: %v\n", opt.SessionID, err)
			}
		} else if opt.ResumeSession {
			if latestID, err := store.LatestSessionID(); err == nil && latestID != "" {
				if _, msgs, err := store.LoadSession(latestID); err == nil {
					ag.Session().Replace(stripSystemMessages(msgs))
					sessionID = latestID
				}
			}
		}

		// Wire auto-save callback
		sid := sessionID
		ag.Session().SetOnSave(func(messages []types.Message) {
			summary := ""
			for _, m := range messages {
				if m.Role == types.RoleUser && m.Content != "" {
					summary = m.Content
					if len(summary) > 100 {
						summary = string([]rune(summary)[:100])
					}
					break
				}
			}
			if err := store.SaveSession(persist.SessionMeta{
				ID:      sid,
				Model:   prov.Model(),
				Project: cfg.GCP.Project,
				Summary: summary,
			}, messages); err != nil {
				fmt.Fprintf(os.Stderr, "  ⚠ Session save failed: %v\n", err)
			}
		})
	}

	// ── 11. Startup report ──
	reportAuthStatus(os.Stderr, resource, modelAuth, bqComp, cfg)

	return &App{
		Config:      cfg,
		Agent:       ag,
		Provider:    prov,
		Registry:    registry,
		Permissions: perms,
		Events:      events,
		Approvals:   approvals,
		BQ:          bqComp,
		Platform:    platform,
		Resource:    resource,
		Morning:     morning,
		CascadeMD:   cascadeMD,
		Sessions:    sessionStore,
		SessionID:   sessionID,
	}, nil
}

// ReloadTools re-registers all tools into the existing registry.
// Existing tools are overwritten; new tools are appended.
func (a *App) ReloadTools() int {
	core.RegisterAll(a.Registry)
	if a.BQ != nil {
		registerBQTools(a.Registry, a.BQ, &a.Config.Cost, a.Events)
	}
	if a.Platform != nil {
		registerPlatformTools(a.Registry, a.Platform, a.Config)
	}
	return len(a.Registry.All())
}

// Close releases resources held by the App.
func (a *App) Close() {
	if a.BQ != nil {
		a.BQ.Close()
	}
	if a.Sessions != nil {
		a.Sessions.Close()
	}
}

// stripSystemMessages removes system-role messages from a loaded session
// to avoid duplicating the system prompt when Replace() prepends a fresh one.
func stripSystemMessages(msgs []types.Message) []types.Message {
	filtered := make([]types.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role != types.RoleSystem {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// buildProvider creates the LLM provider from resolved model auth.
func buildProvider(ctx context.Context, modelName string, m *auth.ModelAuth) (provider.Provider, error) {
	switch m.Provider {
	case "vertex", "gemini_api":
		prov, err := gemini.New(ctx, modelName, m.GenAIConfig)
		if err != nil {
			return nil, fmt.Errorf("provider setup failed: %w", err)
		}
		return prov, nil

	case "openai":
		prov, err := oaiprov.New(modelName, "")
		if err != nil {
			return nil, fmt.Errorf("OpenAI provider setup failed: %w", err)
		}
		return prov, nil

	case "anthropic":
		prov, err := antprov.New(modelName, "")
		if err != nil {
			return nil, fmt.Errorf("anthropic provider setup failed: %w", err)
		}
		return prov, nil

	default:
		return nil, fmt.Errorf("unknown provider %q", m.Provider)
	}
}

// reportAuthStatus prints a startup summary of auth status to w.
func reportAuthStatus(w *os.File, resource *auth.ResourceAuth, model *auth.ModelAuth, bq *BigQueryComponents, cfg *config.Config) {
	// Resource plane
	if resource.Available {
		project := resource.Project
		if project == "" {
			project = "(no project)"
		}
		fmt.Fprintf(w, "  ✓ GCP: %s (%s)\n", project, cfg.GCP.Auth.Mode)
	} else {
		fmt.Fprintf(w, "  ✗ GCP: not available — platform tools disabled\n")
	}

	// Model plane
	fmt.Fprintf(w, "  ✓ LLM: %s (%s)\n", cfg.Model.Model, model.Provider)

	// BigQuery
	if bq != nil {
		fmt.Fprintf(w, "  ✓ BigQuery: %d dataset(s) configured\n", len(cfg.BigQuery.Datasets))
	} else if !resource.Available {
		fmt.Fprintf(w, "  ✗ BigQuery: no GCP credentials\n")
	}

	// Print any warnings
	for _, warning := range resource.Warnings {
		fmt.Fprintf(w, "  ⚠ %s\n", warning)
	}
	for _, warning := range model.Warnings {
		fmt.Fprintf(w, "  ⚠ %s\n", warning)
	}
}
