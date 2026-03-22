// Package config handles layered configuration loading for Cascade.
// Config merges from: defaults < TOML file < environment variables < CLI flags.
package config

// Config is the root configuration structure for Cascade.
type Config struct {
	GCP      GCPConfig      `toml:"gcp"`
	Model    ModelConfig    `toml:"model"`
	Agent    AgentConfig    `toml:"agent"`
	Display  DisplayConfig  `toml:"display"`
	Security SecurityConfig `toml:"security"`
	BigQuery BigQueryConfig `toml:"bigquery"`
	Cost     CostConfig     `toml:"cost"`

	// Deprecated: use GCP.Auth instead. Kept for backward compatibility.
	Auth LegacyAuthConfig `toml:"auth"`
}

// GCPConfig configures access to GCP resources (BigQuery, GCS, Logging, Composer).
type GCPConfig struct {
	Project  string        `toml:"project"`  // GCP project ID (auto-detected if empty)
	Location string        `toml:"location"` // Default region (e.g. "us-central1")
	Auth     GCPAuthConfig `toml:"auth"`
}

// GCPAuthConfig configures how Cascade authenticates with GCP.
type GCPAuthConfig struct {
	Mode                      string `toml:"mode"`                        // adc | impersonation | service_account_key
	ImpersonateServiceAccount string `toml:"impersonate_service_account"` // Target SA for impersonation mode
	CredentialsFile           string `toml:"credentials_file"`            // Path to SA key file (discouraged)
}

// ModelConfig configures the LLM provider and model.
type ModelConfig struct {
	Provider string `toml:"provider"` // vertex | gemini_api | openai | anthropic
	Model    string `toml:"model"`    // Model name (e.g. "gemini-2.5-pro")

	Vertex    VertexModelConfig `toml:"vertex"`
	GeminiAPI APIKeyConfig      `toml:"gemini_api"`
	OpenAI    APIKeyConfig      `toml:"openai"`
	Anthropic APIKeyConfig      `toml:"anthropic"`
}

// VertexModelConfig configures the Vertex AI provider.
// Defaults to inheriting GCP project/location from [gcp].
type VertexModelConfig struct {
	Project  string `toml:"project"`  // Overrides gcp.project for Vertex
	Location string `toml:"location"` // Overrides gcp.location for Vertex
}

// APIKeyConfig configures an API-key-based provider.
type APIKeyConfig struct {
	APIKeyEnv string `toml:"api_key_env"` // Env var name holding the API key
}

// BigQueryConfig configures the BigQuery connection.
type BigQueryConfig struct {
	Project  string   `toml:"project"`  // Overrides gcp.project for BQ
	Location string   `toml:"location"` // BQ dataset location (default: "US")
	Datasets []string `toml:"datasets"` // Dataset IDs to cache
}

// CostConfig configures cost estimation and budget alerts.
type CostConfig struct {
	PricePerTB     float64 `toml:"price_per_tb"`     // Default: 6.25 (on-demand US)
	WarnThreshold  float64 `toml:"warn_threshold"`   // Dollar amount to warn (default: 1.0)
	MaxQueryCost   float64 `toml:"max_query_cost"`   // Dollar amount to block (default: 10.0)
	DailyBudget    float64 `toml:"daily_budget_usd"` // Daily budget for alerts (default: 100.0)
	MaxDisplayRows int     `toml:"max_display_rows"` // Max rows in result table (default: 50)
}

// AgentConfig configures the agent loop behavior.
type AgentConfig struct {
	MaxToolCalls int `toml:"max_tool_calls"` // default: 15
	ToolTimeout  int `toml:"tool_timeout"`   // default: 120 (seconds)
}

// DisplayConfig configures the TUI display.
type DisplayConfig struct {
	Theme string `toml:"theme"` // default: "auto"
}

// SecurityConfig configures the permission engine.
type SecurityConfig struct {
	DefaultMode string `toml:"default_mode"` // default: "ask"
}

// LegacyAuthConfig is the deprecated [auth] section. Migrate to [gcp.auth].
type LegacyAuthConfig struct {
	ImpersonateServiceAccount string `toml:"impersonate_service_account"`
}

// DefaultConfig returns a Config with all default values set.
func DefaultConfig() Config {
	return Config{
		GCP: GCPConfig{
			Auth: GCPAuthConfig{
				Mode: "adc",
			},
		},
		Model: ModelConfig{
			Provider: "", // auto-detect
			Model:    "gemini-3-flash-preview",
			GeminiAPI: APIKeyConfig{
				APIKeyEnv: "GOOGLE_API_KEY",
			},
			OpenAI: APIKeyConfig{
				APIKeyEnv: "OPENAI_API_KEY",
			},
			Anthropic: APIKeyConfig{
				APIKeyEnv: "ANTHROPIC_API_KEY",
			},
		},
		Agent: AgentConfig{
			MaxToolCalls: 200,
			ToolTimeout:  120,
		},
		Display: DisplayConfig{
			Theme: "auto",
		},
		Security: SecurityConfig{
			DefaultMode: "ask",
		},
		BigQuery: BigQueryConfig{
			Location: "US",
		},
		Cost: CostConfig{
			PricePerTB:     6.25,
			WarnThreshold:  1.0,
			MaxQueryCost:   10.0,
			DailyBudget:    100.0,
			MaxDisplayRows: 50,
		},
	}
}

// MigrateLegacy promotes deprecated [auth] fields to the new [gcp.auth] section.
// Called after loading to support old config files.
func (c *Config) MigrateLegacy() {
	if c.Auth.ImpersonateServiceAccount != "" && c.GCP.Auth.ImpersonateServiceAccount == "" {
		c.GCP.Auth.ImpersonateServiceAccount = c.Auth.ImpersonateServiceAccount
		if c.GCP.Auth.Mode == "adc" {
			c.GCP.Auth.Mode = "impersonation"
		}
	}
}
