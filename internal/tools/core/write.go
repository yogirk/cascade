package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/slokam-ai/cascade/internal/permission"
	"github.com/slokam-ai/cascade/internal/tools"
)

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// WriteTool writes content to a file.
type WriteTool struct{}

func NewWriteTool() *WriteTool { return &WriteTool{} }

func (t *WriteTool) Name() string       { return "write_file" }
func (t *WriteTool) Description() string { return "Write content to a file, creating it if it doesn't exist" }

func (t *WriteTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the file",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "Content to write",
			},
		},
		"required": []string{"file_path", "content"},
	}
}

func (t *WriteTool) RiskLevel() permission.RiskLevel { return permission.RiskDML }

func (t *WriteTool) Execute(_ context.Context, input json.RawMessage) (*tools.Result, error) {
	var params writeInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	// Create parent directories if they don't exist
	dir := filepath.Dir(params.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &tools.Result{Content: fmt.Sprintf("error creating directories: %v", err), IsError: true}, nil
	}

	if err := os.WriteFile(params.FilePath, []byte(params.Content), 0644); err != nil {
		return &tools.Result{Content: fmt.Sprintf("error writing file: %v", err), IsError: true}, nil
	}

	msg := fmt.Sprintf("Wrote %d bytes to %s", len(params.Content), params.FilePath)
	return &tools.Result{Content: msg, Display: msg}, nil
}
