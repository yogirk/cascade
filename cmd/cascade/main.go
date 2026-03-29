package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/yogirk/cascade/internal/app"
	"github.com/yogirk/cascade/internal/config"
	"github.com/yogirk/cascade/internal/oneshot"
	"github.com/yogirk/cascade/internal/persist"
	"github.com/yogirk/cascade/internal/tui"
)

var version = "dev" // set at build time via -ldflags from VERSION file

func init() {
	// Fallback: if ldflags weren't used, read VERSION file from repo root
	if version == "dev" {
		if data, err := os.ReadFile("VERSION"); err == nil {
			if v := strings.TrimSpace(string(data)); v != "" {
				version = v
			}
		}
	}
}

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
	rootCmd.Flags().Bool("bypass", false, "Enable full-access mode (legacy flag name)")
	rootCmd.Flags().Bool("resume", false, "Resume the most recent session")
	rootCmd.Flags().String("session", "", "Resume a specific session by ID")

	// Sessions subcommand
	sessionsCmd := &cobra.Command{
		Use:   "sessions",
		Short: "List saved sessions",
		RunE:  listSessions,
	}
	sessionsCmd.AddCommand(&cobra.Command{
		Use:   "rm [id]",
		Short: "Delete a session",
		Args:  cobra.ExactArgs(1),
		RunE:  deleteSession,
	})
	rootCmd.AddCommand(sessionsCmd)

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
		cfg.Security.DefaultMode = "full-access"
	}

	// Session resume options
	resume, _ := cmd.Flags().GetBool("resume")
	sessionID, _ := cmd.Flags().GetString("session")
	opts := app.Options{
		ResumeSession: resume,
		SessionID:     sessionID,
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
		application, err := app.New(ctx, cfg, opts)
		if err != nil {
			return err
		}
		defer application.Close()
		return oneshot.Run(ctx, application, prompt, os.Stdout, os.Stderr)
	}

	// If stdin was piped but no -p flag, join args as prompt or go interactive
	if stdinContext != "" && len(args) > 0 {
		prompt := fmt.Sprintf("Context:\n%s\n\nRequest: %s", stdinContext, strings.Join(args, " "))
		application, err := app.New(ctx, cfg, opts)
		if err != nil {
			return err
		}
		defer application.Close()
		return oneshot.Run(ctx, application, prompt, os.Stdout, os.Stderr)
	}

	// Interactive mode (default)
	application, err := app.New(ctx, cfg, opts)
	if err != nil {
		return err
	}
	defer application.Close()
	application.Version = version

	model := tui.NewModel(application)
	p := tea.NewProgram(model)
	_, err = p.Run()
	return err
}

func listSessions(cmd *cobra.Command, args []string) error {
	homeDir, _ := os.UserHomeDir()
	cascadeDir := filepath.Join(homeDir, ".cascade")
	store, err := persist.OpenSQLite(cascadeDir)
	if err != nil {
		return fmt.Errorf("open sessions: %w", err)
	}
	defer store.Close()

	sessions, err := store.ListSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions.")
		return nil
	}

	for _, s := range sessions {
		summary := s.Summary
		if len([]rune(summary)) > 60 {
			summary = string([]rune(summary)[:60]) + "..."
		}
		if summary == "" {
			summary = "(empty)"
		}
		fmt.Printf("  %s  %-20s  %s  %s\n",
			s.ID,
			s.Model,
			s.UpdatedAt.Format("2006-01-02 15:04"),
			summary,
		)
	}
	return nil
}

func deleteSession(cmd *cobra.Command, args []string) error {
	homeDir, _ := os.UserHomeDir()
	cascadeDir := filepath.Join(homeDir, ".cascade")
	store, err := persist.OpenSQLite(cascadeDir)
	if err != nil {
		return fmt.Errorf("open sessions: %w", err)
	}
	defer store.Close()

	if err := store.DeleteSession(args[0]); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	fmt.Printf("Deleted session %s\n", args[0])
	return nil
}
