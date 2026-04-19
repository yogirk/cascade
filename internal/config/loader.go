package config

import (
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
)

// LoadOptions controls the config loading behavior.
type LoadOptions struct {
	// ConfigPath is the explicit path to a TOML config file.
	// If empty, defaults to ~/.cascade/config.toml.
	ConfigPath string

	// Flags are CLI flag overrides. Keys: "model", "provider".
	Flags map[string]string
}

// Load reads configuration from all layers and returns a merged Config.
// Layer order: defaults < TOML file < env vars < CLI flags.
func Load(opts LoadOptions) (*Config, error) {
	// Layer 1: Defaults
	cfg := DefaultConfig()

	// Layer 2: TOML file
	configPath := opts.ConfigPath
	if configPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			configPath = home + "/.cascade/config.toml"
		}
	}
	if configPath != "" {
		if err := loadTOML(&cfg, configPath); err != nil {
			return nil, err
		}
	}

	// Layer 3: Environment variables
	applyEnv(&cfg)

	// Layer 4: CLI flags
	applyFlags(&cfg, opts.Flags)

	// Migrate deprecated [auth] → [gcp.auth]
	cfg.MigrateLegacy()

	return &cfg, nil
}

// loadTOML decodes a TOML file into cfg. If the file doesn't exist, it's
// silently skipped (not an error).
func loadTOML(cfg *Config, path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil // Missing config file is not an error
	}
	if err != nil {
		return err
	}

	_, err = toml.DecodeFile(path, cfg)
	return err
}

// applyEnv overrides config with environment variable values.
func applyEnv(cfg *Config) {
	if v := os.Getenv("CASCADE_MODEL"); v != "" {
		cfg.Model.Model = v
	}
	if v := os.Getenv("CASCADE_PROVIDER"); v != "" {
		cfg.Model.Provider = v
	}
	if v := os.Getenv("CASCADE_MAX_TOOL_CALLS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Agent.MaxToolCalls = n
		}
	}
	if v := os.Getenv("CASCADE_DEFAULT_MODE"); v != "" {
		cfg.Security.DefaultMode = v
	}
	if v := os.Getenv("CASCADE_GCP_PROJECT"); v != "" {
		cfg.GCP.Project = v
	}
}

// applyFlags overrides config with CLI flag values.
func applyFlags(cfg *Config, flags map[string]string) {
	if flags == nil {
		return
	}
	if v, ok := flags["model"]; ok && v != "" {
		cfg.Model.Model = v
	}
	if v, ok := flags["provider"]; ok && v != "" {
		cfg.Model.Provider = v
	}
	if v, ok := flags["project"]; ok && v != "" {
		cfg.GCP.Project = v
	}
	if v, ok := flags["theme"]; ok && v != "" {
		cfg.Display.Theme = v
	}
}
