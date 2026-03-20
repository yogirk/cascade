package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/tools"
)

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

func NewGlobTool() *GlobTool { return &GlobTool{} }

func (t *GlobTool) Name() string       { return "glob" }
func (t *GlobTool) Description() string { return "Find files matching a glob pattern" }

func (t *GlobTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern (supports **)",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in (defaults to cwd)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GlobTool) RiskLevel() permission.RiskLevel { return permission.RiskReadOnly }

func (t *GlobTool) Execute(_ context.Context, input json.RawMessage) (*tools.Result, error) {
	var params globInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	searchPath := params.Path
	if searchPath == "" {
		var err error
		searchPath, err = os.Getwd()
		if err != nil {
			return &tools.Result{Content: fmt.Sprintf("error getting cwd: %v", err), IsError: true}, nil
		}
	}

	fsys := os.DirFS(searchPath)
	matches, err := doublestar.Glob(fsys, params.Pattern)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("error matching pattern: %v", err), IsError: true}, nil
	}

	sort.Strings(matches)

	// Limit to 1000 results
	const maxResults = 1000
	truncated := false
	if len(matches) > maxResults {
		matches = matches[:maxResults]
		truncated = true
	}

	// Convert to absolute paths
	absMatches := make([]string, len(matches))
	for i, m := range matches {
		absMatches[i] = filepath.Join(searchPath, m)
	}

	content := strings.Join(absMatches, "\n")
	if truncated {
		content += fmt.Sprintf("\n... (truncated, showing %d of more results)", maxResults)
	}

	return &tools.Result{Content: content, Display: content}, nil
}
