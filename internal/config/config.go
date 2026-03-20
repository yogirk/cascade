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
	}
}
