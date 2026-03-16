package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	udiff "github.com/aymanbagabas/go-udiff"
	"github.com/yogirk/cascade/internal/permission"
	"github.com/yogirk/cascade/internal/tools"
)

type editInput struct {
	FilePath string `json:"file_path"`
	OldText  string `json:"old_text"`
	NewText  string `json:"new_text"`
}

// EditTool replaces text in a file.
type EditTool struct{}

func NewEditTool() *EditTool { return &EditTool{} }

func (t *EditTool) Name() string       { return "edit_file" }
func (t *EditTool) Description() string { return "Replace text in a file using string replacement" }

func (t *EditTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the file",
			},
			"old_text": map[string]any{
				"type":        "string",
				"description": "Text to find and replace",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "Replacement text",
			},
		},
		"required": []string{"file_path", "old_text", "new_text"},
	}
}

func (t *EditTool) RiskLevel() permission.RiskLevel { return permission.RiskDML }

func (t *EditTool) Execute(_ context.Context, input json.RawMessage) (*tools.Result, error) {
	var params editInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	data, err := os.ReadFile(params.FilePath)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("error reading file: %v", err), IsError: true}, nil
	}

	original := string(data)
	if !strings.Contains(original, params.OldText) {
		return &tools.Result{Content: "old_text not found in file", IsError: true}, nil
	}

	// Replace first occurrence
	modified := strings.Replace(original, params.OldText, params.NewText, 1)

	if err := os.WriteFile(params.FilePath, []byte(modified), 0644); err != nil {
		return &tools.Result{Content: fmt.Sprintf("error writing file: %v", err), IsError: true}, nil
	}

	// Generate unified diff
	diff := udiff.Unified(params.FilePath, params.FilePath, original, modified)

	msg := fmt.Sprintf("Edited %s: replaced 1 occurrence", params.FilePath)
	return &tools.Result{Content: msg, Display: diff}, nil
}
