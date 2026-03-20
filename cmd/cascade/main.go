package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/yogirk/cascade/internal/app"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/oneshot"
	"github.com/yogirk/cascade/internal/tui"
)

var version = "0.1.0-dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "cascade",
		Aliases: []string{"csc"},
		Short:   "AI-native terminal agent for GCP data engineering",
		Version: version,
		RunE:    run,
		// Silence usage/errors so we control output
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.Flags().StringP("prompt", "p", "", "One-shot mode: run prompt and exit")
	rootCmd.Flags().String("model", "", "LLM model to use (e.g., gemini-2.5-pro)")
	rootCmd.Flags().String("provider", "", "Backend: \"gemini\" (API key) or \"vertex\" (GCP)")
	rootCmd.Flags().String("project", "", "GCP Project ID for Vertex AI")
	rootCmd.Flags().String("config", "", "Path to config file")
	rootCmd.Flags().Bool("bypass", false, "Auto-approve all tool calls")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Load config with CLI flags
	flags := make(map[string]string)
	if m, _ := cmd.Flags().GetString("model"); m != "" {
		flags["model"] = m
	}
	if p, _ := cmd.Flags().GetString("provider"); p != "" {
		flags["provider"] = p
	}
	if p, _ := cmd.Flags().GetString("project"); p != "" {
		flags["project"] = p
	}
	configPath, _ := cmd.Flags().GetString("config")
	bypass, _ := cmd.Flags().GetBool("bypass")

	cfg, err := config.Load(config.LoadOptions{
		ConfigPath: configPath,
		Flags:      flags,
	})
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if bypass {
		cfg.Security.DefaultMode = "bypass"
	}

	// Check for stdin piping
	prompt, _ := cmd.Flags().GetString("prompt")
	stdinContext := ""
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// stdin is piped
		data, err := io.ReadAll(os.Stdin)
		if err == nil && len(data) > 0 {
			stdinContext = string(data)
		}
	}

	// One-shot mode
	if prompt != "" {
		if stdinContext != "" {
			prompt = fmt.Sprintf("Context:\n%s\n\nRequest: %s", stdinContext, prompt)
		}
		application, err := app.New(ctx, cfg)
		if err != nil {
			return err
		}
		return oneshot.Run(ctx, application, prompt, os.Stdout, os.Stderr)
	}

	// If stdin was piped but no -p flag, join args as prompt or go interactive
	if stdinContext != "" && len(args) > 0 {
		prompt := fmt.Sprintf("Context:\n%s\n\nRequest: %s", stdinContext, strings.Join(args, " "))
		application, err := app.New(ctx, cfg)
		if err != nil {
			return err
		}
		return oneshot.Run(ctx, application, prompt, os.Stdout, os.Stderr)
	}

	// Interactive mode (default)
	application, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}

	model := tui.NewModel(application)
	p := tea.NewProgram(model)
	_, err = p.Run()
	return err
}
