// Package config handles layered configuration loading for Cascade.
// Config merges from: defaults < TOML file < environment variables < CLI flags.
package config

// Config is the root configuration structure for Cascade.
type Config struct {
	Model    ModelConfig    `toml:"model"`
	Auth     AuthConfig     `toml:"auth"`
	Agent    AgentConfig    `toml:"agent"`
	Display  DisplayConfig  `toml:"display"`
	Security SecurityConfig `toml:"security"`
	BigQuery BigQueryConfig `toml:"bigquery"`
	Cost     CostConfig     `toml:"cost"`
}

// BigQueryConfig configures the BigQuery connection.
type BigQueryConfig struct {
	Project  string   `toml:"project"`  // GCP project ID (auto-detected if empty)
	Location string   `toml:"location"` // BQ dataset location (default: "US")
	Datasets []string `toml:"datasets"` // Dataset IDs to cache (required for schema cache)
}

// CostConfig configures cost estimation and budget alerts.
type CostConfig struct {
	PricePerTB     float64 `toml:"price_per_tb"`     // Default: 6.25 (on-demand US)
	WarnThreshold  float64 `toml:"warn_threshold"`   // Dollar amount to warn (default: 1.0)
	MaxQueryCost   float64 `toml:"max_query_cost"`   // Dollar amount to block (default: 10.0)
	DailyBudget    float64 `toml:"daily_budget_usd"` // Daily budget for alerts (default: 100.0)
	MaxDisplayRows int     `toml:"max_display_rows"` // Max rows in result table (default: 50)
}

// ModelConfig configures the LLM provider and model.
type ModelConfig struct {
	Provider string `toml:"provider"` // default: "vertex"
	Model    string `toml:"model"`    // default: "gemini-2.5-pro"
	Project  string `toml:"project"`  // GCP Project ID for Vertex AI
	Location string `toml:"location"` // GCP Location for Vertex AI (e.g., "us-central1")
}

// AuthConfig configures GCP authentication.
type AuthConfig struct {
	ImpersonateServiceAccount string `toml:"impersonate_service_account"`
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
	DefaultMode string `toml:"default_mode"` // default: "confirm"
}

// DefaultConfig returns a Config with all default values set.
func DefaultConfig() Config {
	return Config{
		Model: ModelConfig{
			Provider: "", // auto-detect: GOOGLE_API_KEY → "gemini", else "vertex"
			Model:    "gemini-2.5-pro",
			Location: "us-central1",
		},
		Agent: AgentConfig{
			MaxToolCalls: 15,
			ToolTimeout:  120,
		},
		Display: DisplayConfig{
			Theme: "auto",
		},
		Security: SecurityConfig{
			DefaultMode: "confirm",
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
