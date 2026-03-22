package config

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model.Provider != "" {
		t.Errorf("expected provider %q, got %q", "", cfg.Model.Provider)
	}
	if cfg.Model.Model != "gemini-3-flash-preview" {
		t.Errorf("expected model %q, got %q", "gemini-3-flash-preview", cfg.Model.Model)
	}
	if cfg.Agent.MaxToolCalls != 200 {
		t.Errorf("expected max_tool_calls %d, got %d", 200, cfg.Agent.MaxToolCalls)
	}
	if cfg.Agent.ToolTimeout != 120 {
		t.Errorf("expected tool_timeout %d, got %d", 120, cfg.Agent.ToolTimeout)
	}
	if cfg.Display.Theme != "auto" {
		t.Errorf("expected theme %q, got %q", "auto", cfg.Display.Theme)
	}
	if cfg.Security.DefaultMode != "ask" {
		t.Errorf("expected default_mode %q, got %q", "ask", cfg.Security.DefaultMode)
	}
}

func TestLoadFromTOML(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(tomlPath, []byte(`
[model]
provider = "anthropic"
model = "claude-3-opus"

[agent]
max_tool_calls = 25
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOptions{ConfigPath: tomlPath})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Model.Provider != "anthropic" {
		t.Errorf("expected provider %q, got %q", "anthropic", cfg.Model.Provider)
	}
	if cfg.Model.Model != "claude-3-opus" {
		t.Errorf("expected model %q, got %q", "claude-3-opus", cfg.Model.Model)
	}
	if cfg.Agent.MaxToolCalls != 25 {
		t.Errorf("expected max_tool_calls %d, got %d", 25, cfg.Agent.MaxToolCalls)
	}
	// Unset fields should retain defaults
	if cfg.Agent.ToolTimeout != 120 {
		t.Errorf("expected tool_timeout default %d, got %d", 120, cfg.Agent.ToolTimeout)
	}
	if cfg.Display.Theme != "auto" {
		t.Errorf("expected theme default %q, got %q", "auto", cfg.Display.Theme)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(tomlPath, []byte(`
[model]
model = "gemini-2.0-flash"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CASCADE_MODEL", "gemini-2.5-flash")

	cfg, err := Load(LoadOptions{ConfigPath: tomlPath})
	if err != nil {
		t.Fatal(err)
	}

	// Env should override TOML
	if cfg.Model.Model != "gemini-2.5-flash" {
		t.Errorf("expected model %q (from env), got %q", "gemini-2.5-flash", cfg.Model.Model)
	}
}

func TestLoadFlagOverride(t *testing.T) {
	t.Setenv("CASCADE_MODEL", "gemini-2.5-flash")

	cfg, err := Load(LoadOptions{
		Flags: map[string]string{
			"model": "gemini-pro-experimental",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Flags should override env
	if cfg.Model.Model != "gemini-pro-experimental" {
		t.Errorf("expected model %q (from flags), got %q", "gemini-pro-experimental", cfg.Model.Model)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(LoadOptions{ConfigPath: "/nonexistent/config.toml"})
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}

	// Should return defaults
	if cfg.Model.Provider != "" {
		t.Errorf("expected default provider %q, got %q", "", cfg.Model.Provider)
	}
}

func TestLoadLayerOrder(t *testing.T) {
	// Set up all four layers
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(tomlPath, []byte(`
[model]
provider = "from-toml"
model = "from-toml"

[security]
default_mode = "from-toml"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("CASCADE_MODEL", "from-env")
	t.Setenv("CASCADE_DEFAULT_MODE", "from-env")

	cfg, err := Load(LoadOptions{
		ConfigPath: tomlPath,
		Flags: map[string]string{
			"model": "from-flag",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Flag > env > toml > default
	if cfg.Model.Model != "from-flag" {
		t.Errorf("model: expected %q (flag), got %q", "from-flag", cfg.Model.Model)
	}
	// provider: only set by toml (no env or flag for it in this test)
	if cfg.Model.Provider != "from-toml" {
		t.Errorf("provider: expected %q (toml), got %q", "from-toml", cfg.Model.Provider)
	}
	// default_mode: env overrides toml
	if cfg.Security.DefaultMode != "from-env" {
		t.Errorf("default_mode: expected %q (env), got %q", "from-env", cfg.Security.DefaultMode)
	}
}

func TestLoadEnvMaxToolCalls(t *testing.T) {
	t.Setenv("CASCADE_MAX_TOOL_CALLS", "30")

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Agent.MaxToolCalls != 30 {
		t.Errorf("expected max_tool_calls %d, got %d", 30, cfg.Agent.MaxToolCalls)
	}
}

func TestLoadEnvProvider(t *testing.T) {
	t.Setenv("CASCADE_PROVIDER", "openai")

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Model.Provider != "openai" {
		t.Errorf("expected provider %q, got %q", "openai", cfg.Model.Provider)
	}
}

func TestLoadFlagProvider(t *testing.T) {
	t.Setenv("CASCADE_PROVIDER", "openai")

	cfg, err := Load(LoadOptions{
		Flags: map[string]string{
			"provider": "anthropic",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Model.Provider != "anthropic" {
		t.Errorf("expected provider %q (flag), got %q", "anthropic", cfg.Model.Provider)
	}
}

func TestLoadInvalidMaxToolCalls(t *testing.T) {
	t.Setenv("CASCADE_MAX_TOOL_CALLS", "not-a-number")

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Should keep default when parsing fails
	if cfg.Agent.MaxToolCalls != 200 {
		t.Errorf("expected max_tool_calls default %d on parse error, got %d", 200, cfg.Agent.MaxToolCalls)
	}
}

func TestLoadLegacyAuthMigration(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(tomlPath, []byte(`
[auth]
impersonate_service_account = "sa@project.iam.gserviceaccount.com"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOptions{ConfigPath: tomlPath})
	if err != nil {
		t.Fatal(err)
	}

	// Legacy [auth] should migrate to [gcp.auth]
	if cfg.GCP.Auth.ImpersonateServiceAccount != "sa@project.iam.gserviceaccount.com" {
		t.Errorf("expected legacy auth to migrate, got %q", cfg.GCP.Auth.ImpersonateServiceAccount)
	}
	if cfg.GCP.Auth.Mode != "impersonation" {
		t.Errorf("expected mode to auto-set to impersonation, got %q", cfg.GCP.Auth.Mode)
	}
}

func TestLoadNewGCPConfig(t *testing.T) {
	dir := t.TempDir()
	tomlPath := filepath.Join(dir, "config.toml")
	err := os.WriteFile(tomlPath, []byte(`
[gcp]
project = "my-project"
location = "us-central1"

[gcp.auth]
mode = "impersonation"
impersonate_service_account = "sa@project.iam.gserviceaccount.com"

[model]
provider = "gemini_api"

[model.gemini_api]
api_key_env = "MY_GEMINI_KEY"
`), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(LoadOptions{ConfigPath: tomlPath})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.GCP.Project != "my-project" {
		t.Errorf("expected gcp.project %q, got %q", "my-project", cfg.GCP.Project)
	}
	if cfg.GCP.Location != "us-central1" {
		t.Errorf("expected gcp.location %q, got %q", "us-central1", cfg.GCP.Location)
	}
	if cfg.GCP.Auth.Mode != "impersonation" {
		t.Errorf("expected gcp.auth.mode %q, got %q", "impersonation", cfg.GCP.Auth.Mode)
	}
	if cfg.Model.Provider != "gemini_api" {
		t.Errorf("expected model.provider %q, got %q", "gemini_api", cfg.Model.Provider)
	}
	if cfg.Model.GeminiAPI.APIKeyEnv != "MY_GEMINI_KEY" {
		t.Errorf("expected model.gemini_api.api_key_env %q, got %q", "MY_GEMINI_KEY", cfg.Model.GeminiAPI.APIKeyEnv)
	}
}

func TestLoadGCPProjectEnvOverride(t *testing.T) {
	t.Setenv("CASCADE_GCP_PROJECT", "env-project")

	cfg, err := Load(LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.GCP.Project != "env-project" {
		t.Errorf("expected gcp.project %q from env, got %q", "env-project", cfg.GCP.Project)
	}
}

func TestDefaultConfigAPIKeyEnvs(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model.GeminiAPI.APIKeyEnv != "GOOGLE_API_KEY" {
		t.Errorf("expected default gemini_api.api_key_env %q, got %q", "GOOGLE_API_KEY", cfg.Model.GeminiAPI.APIKeyEnv)
	}
	if cfg.Model.OpenAI.APIKeyEnv != "OPENAI_API_KEY" {
		t.Errorf("expected default openai.api_key_env %q, got %q", "OPENAI_API_KEY", cfg.Model.OpenAI.APIKeyEnv)
	}
	if cfg.Model.Anthropic.APIKeyEnv != "ANTHROPIC_API_KEY" {
		t.Errorf("expected default anthropic.api_key_env %q, got %q", "ANTHROPIC_API_KEY", cfg.Model.Anthropic.APIKeyEnv)
	}
	if cfg.GCP.Auth.Mode != "adc" {
		t.Errorf("expected default gcp.auth.mode %q, got %q", "adc", cfg.GCP.Auth.Mode)
	}
}

// Helper to avoid unused import warning
var _ = strconv.Itoa
