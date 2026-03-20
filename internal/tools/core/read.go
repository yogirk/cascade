package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/tools"
)

type readInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"` // 1-based line number to start from
	Limit    int    `json:"limit"`  // Number of lines to read
}

// ReadTool reads file contents.
type ReadTool struct{}

func NewReadTool() *ReadTool { return &ReadTool{} }

func (t *ReadTool) Name() string       { return "read_file" }
func (t *ReadTool) Description() string { return "Read the contents of a file at the given path" }

func (t *ReadTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the file",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "Line number to start reading from (1-based)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Number of lines to read",
			},
		},
		"required": []string{"file_path"},
	}
}

func (t *ReadTool) RiskLevel() permission.RiskLevel { return permission.RiskReadOnly }

func (t *ReadTool) Execute(_ context.Context, input json.RawMessage) (*tools.Result, error) {
	var params readInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	data, err := os.ReadFile(params.FilePath)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("error reading file: %v", err), IsError: true}, nil
	}

	lines := strings.Split(string(data), "\n")

	// Apply offset and limit
	startLine := 1
	if params.Offset > 0 {
		startLine = params.Offset
	}
	endLine := len(lines)
	if params.Limit > 0 {
		endLine = startLine - 1 + params.Limit
		if endLine > len(lines) {
			endLine = len(lines)
		}
	}

	// Build output with line numbers (cat -n style)
	var sb strings.Builder
	for i := startLine - 1; i < endLine; i++ {
		if i >= len(lines) {
			break
		}
		fmt.Fprintf(&sb, "%6d\t%s\n", i+1, lines[i])
	}

	content := sb.String()
	return &tools.Result{Content: content, Display: content}, nil
}
