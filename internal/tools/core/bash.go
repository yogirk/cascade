package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/tools"
)

type bashInput struct {
	Command string `json:"command"`
}

// BashTool executes shell commands.
type BashTool struct{}

func NewBashTool() *BashTool { return &BashTool{} }

func (t *BashTool) Name() string       { return "bash" }
func (t *BashTool) Description() string { return "Execute a shell command" }

func (t *BashTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

// RiskLevel returns the default risk level for bash (DESTRUCTIVE).
// PlanPermission refines this based on the actual command.
func (t *BashTool) RiskLevel() permission.RiskLevel { return permission.RiskDestructive }

// PlanPermission classifies the actual bash command to determine the real risk level.
func (t *BashTool) PlanPermission(_ context.Context, input json.RawMessage, _ permission.RiskLevel) (*tools.PermissionPlan, error) {
	var params bashInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, nil // fall back to default risk
	}
	risk := ClassifyBashRisk(params.Command)
	return &tools.PermissionPlan{RiskOverride: &risk}, nil
}

func (t *BashTool) Execute(ctx context.Context, input json.RawMessage) (*tools.Result, error) {
	var params bashInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", params.Command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	content := stdout.String()
	if stderr.Len() > 0 {
		if content != "" {
			content += "\n"
		}
		content += stderr.String()
	}

	if err != nil {
		return &tools.Result{Content: content, Display: content, IsError: true}, nil
	}

	return &tools.Result{Content: content, Display: content}, nil
}

// readOnlyCommands lists commands that have no side effects.
var readOnlyCommands = map[string]bool{
	"ls": true, "cat": true, "head": true, "tail": true,
	"wc": true, "find": true, "which": true, "pwd": true,
	"whoami": true, "echo": true, "env": true, "date": true,
	"file": true, "stat": true, "df": true, "du": true,
	"uname": true, "hostname": true,
}

// readOnlyGitSubcommands lists git subcommands that have no side effects.
var readOnlyGitSubcommands = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true,
	"branch": true, "remote": true, "tag": true, "rev-parse": true,
}

// dangerousPatterns contains strings that indicate destructive operations
// anywhere in a command.
var dangerousPatterns = []string{
	"rm ", "rm\t", "> ", ">> ", ">|",
	"chmod ", "chown ",
}

// ClassifyBashRisk classifies the risk level of a bash command string.
// This is exported so the agent loop can call it before checking permissions.
func ClassifyBashRisk(command string) permission.RiskLevel {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return permission.RiskDestructive
	}

	base := filepath.Base(parts[0])

	// Check for dangerous patterns anywhere in the command
	if containsDangerousPattern(command) {
		return permission.RiskDestructive
	}

	// Check if the base command is read-only
	if base == "git" {
		if len(parts) > 1 && readOnlyGitSubcommands[parts[1]] {
			return permission.RiskReadOnly
		}
		// Non-read-only git subcommands are DML (commit, push, add, etc.)
		return permission.RiskDML
	}

	if readOnlyCommands[base] {
		return permission.RiskReadOnly
	}

	if base == "gcloud" && isReadOnlyGCloud(parts[1:]) {
		return permission.RiskReadOnly
	}

	if base == "bq" && isReadOnlyBQ(parts[1:]) {
		return permission.RiskReadOnly
	}

	// Unknown commands default to destructive
	return permission.RiskDestructive
}

func isReadOnlyGCloud(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "config":
		return len(args) >= 2 && (args[1] == "get-value" || args[1] == "list")
	case "projects":
		return len(args) >= 2 && (args[1] == "describe" || args[1] == "list")
	case "auth":
		return len(args) >= 2 && (args[1] == "list" || args[1] == "print-access-token")
	default:
		return false
	}
}

func isReadOnlyBQ(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "ls", "show", "head":
		return true
	case "query":
		for _, arg := range args[1:] {
			if strings.HasPrefix(arg, "--destination_table") ||
				strings.HasPrefix(arg, "--append_table") ||
				strings.HasPrefix(arg, "--replace") {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func containsDangerousPattern(command string) bool {
	for _, pat := range dangerousPatterns {
		if strings.Contains(command, pat) {
			return true
		}
	}
	// Also check for output redirection patterns: > or >> not preceded by 2 (stderr redirect only)
	// Simple check: look for > that isn't part of >>
	for i := 0; i < len(command); i++ {
		if command[i] == '>' {
			// Check if this is >> (append)
			if i+1 < len(command) && command[i+1] == '>' {
				return true // >> is dangerous
			}
			// Single > is dangerous unless it's 2> (stderr redirect in logging context)
			if i > 0 && command[i-1] == '2' {
				continue // 2> is okay (stderr redirect)
			}
			return true
		}
	}
	return false
}
