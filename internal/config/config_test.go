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
	if cfg.Model.Model != "gemini-2.5-pro" {
		t.Errorf("expected model %q, got %q", "gemini-2.5-pro", cfg.Model.Model)
	}
	if cfg.Agent.MaxToolCalls != 15 {
		t.Errorf("expected max_tool_calls %d, got %d", 15, cfg.Agent.MaxToolCalls)
	}
	if cfg.Agent.ToolTimeout != 120 {
		t.Errorf("expected tool_timeout %d, got %d", 120, cfg.Agent.ToolTimeout)
	}
	if cfg.Display.Theme != "auto" {
		t.Errorf("expected theme %q, got %q", "auto", cfg.Display.Theme)
	}
	if cfg.Security.DefaultMode != "confirm" {
		t.Errorf("expected default_mode %q, got %q", "confirm", cfg.Security.DefaultMode)
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
	if cfg.Agent.MaxToolCalls != 15 {
		t.Errorf("expected max_tool_calls default %d on parse error, got %d", 15, cfg.Agent.MaxToolCalls)
	}
}

// Helper to avoid unused import warning
var _ = strconv.Itoa
