package core_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cascade-cli/cascade/internal/permission"
	"github.com/cascade-cli/cascade/internal/tools"
	"github.com/cascade-cli/cascade/internal/tools/core"
)

// Compile-time interface checks: all 6 core tools implement tools.Tool.
var (
	_ tools.Tool = (*core.ReadTool)(nil)
	_ tools.Tool = (*core.WriteTool)(nil)
	_ tools.Tool = (*core.EditTool)(nil)
	_ tools.Tool = (*core.GlobTool)(nil)
	_ tools.Tool = (*core.GrepTool)(nil)
	_ tools.Tool = (*core.BashTool)(nil)
)

// --- Registry Tests ---

func TestRegistry_Get_ReadTool(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(core.NewReadTool())
	tool := reg.Get("read_file")
	if tool == nil {
		t.Fatal("Registry.Get('read_file') returned nil, expected ReadTool")
	}
	if tool.Name() != "read_file" {
		t.Errorf("got name %q, want %q", tool.Name(), "read_file")
	}
}

func TestRegistry_Get_Nonexistent(t *testing.T) {
	reg := tools.NewRegistry()
	tool := reg.Get("nonexistent")
	if tool != nil {
		t.Errorf("Registry.Get('nonexistent') returned %v, expected nil", tool)
	}
}

func TestRegistry_Declarations(t *testing.T) {
	reg := tools.NewRegistry()
	reg.Register(core.NewReadTool())
	reg.Register(core.NewWriteTool())
	reg.Register(core.NewEditTool())
	reg.Register(core.NewGlobTool())
	reg.Register(core.NewGrepTool())
	reg.Register(core.NewBashTool())

	decls := reg.Declarations()
	if len(decls) != 6 {
		t.Fatalf("got %d declarations, want 6", len(decls))
	}

	// Check all names are present
	names := make(map[string]bool)
	for _, d := range decls {
		names[d.Name] = true
		if d.Description == "" {
			t.Errorf("declaration %q has empty description", d.Name)
		}
		if d.Schema == nil {
			t.Errorf("declaration %q has nil schema", d.Name)
		}
	}

	expectedNames := []string{"read_file", "write_file", "edit_file", "glob", "grep", "bash"}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing declaration for %q", name)
		}
	}
}

// --- Read Tool Tests ---

func TestReadTool_Execute_ValidFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := core.NewReadTool()
	input, _ := json.Marshal(map[string]any{"file_path": filePath})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "line1") {
		t.Errorf("result missing 'line1', got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "line2") {
		t.Errorf("result missing 'line2', got: %s", result.Content)
	}
}

func TestReadTool_Execute_NonexistentFile(t *testing.T) {
	tool := core.NewReadTool()
	input, _ := json.Marshal(map[string]any{"file_path": "/nonexistent/file.txt"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for nonexistent file")
	}
}

func TestReadTool_Execute_WithOffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tool := core.NewReadTool()
	input, _ := json.Marshal(map[string]any{"file_path": filePath, "offset": 2, "limit": 2})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "line2") {
		t.Errorf("result missing 'line2', got: %s", result.Content)
	}
	if !strings.Contains(result.Content, "line3") {
		t.Errorf("result missing 'line3', got: %s", result.Content)
	}
	if strings.Contains(result.Content, "line1") {
		t.Errorf("result should not contain 'line1' with offset=2, got: %s", result.Content)
	}
	if strings.Contains(result.Content, "line4") {
		t.Errorf("result should not contain 'line4' with limit=2, got: %s", result.Content)
	}
}

// --- Write Tool Tests ---

func TestWriteTool_Execute_CreateFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "output.txt")

	tool := core.NewWriteTool()
	input, _ := json.Marshal(map[string]any{"file_path": filePath, "content": "hello world"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}
}

func TestWriteTool_Execute_CreateParentDirectories(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a", "b", "c", "deep.txt")

	tool := core.NewWriteTool()
	input, _ := json.Marshal(map[string]any{"file_path": filePath, "content": "deep content"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file in nested dir: %v", err)
	}
	if string(data) != "deep content" {
		t.Errorf("file content = %q, want %q", string(data), "deep content")
	}
}

// --- Edit Tool Tests ---

func TestEditTool_Execute_ReplaceText(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(filePath, []byte("hello world foo bar"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := core.NewEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path": filePath,
		"old_text":  "hello world",
		"new_text":  "goodbye world",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "goodbye world foo bar" {
		t.Errorf("file content = %q, want %q", string(data), "goodbye world foo bar")
	}

	// Display should contain diff
	if result.Display == "" {
		t.Error("expected non-empty Display with diff output")
	}
}

func TestEditTool_Execute_OldTextNotFound(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := core.NewEditTool()
	input, _ := json.Marshal(map[string]any{
		"file_path": filePath,
		"old_text":  "nonexistent text",
		"new_text":  "replacement",
	})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true when old_text not found")
	}
}

// --- Glob Tool Tests ---

func TestGlobTool_Execute_SimplePattern(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tool := core.NewGlobTool()
	input, _ := json.Marshal(map[string]any{"pattern": "*.go", "path": dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}

	lines := strings.Split(strings.TrimSpace(result.Content), "\n")
	sort.Strings(lines)
	if len(lines) != 2 {
		t.Fatalf("got %d matches, want 2: %v", len(lines), lines)
	}
	if !strings.HasSuffix(lines[0], "a.go") || !strings.HasSuffix(lines[1], "b.go") {
		t.Errorf("unexpected matches: %v", lines)
	}
}

func TestGlobTool_Execute_RecursivePattern(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"top.go", filepath.Join("sub", "nested.go")} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	tool := core.NewGlobTool()
	input, _ := json.Marshal(map[string]any{"pattern": "**/*.go", "path": dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}

	lines := strings.Split(strings.TrimSpace(result.Content), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d matches, want 2: %v", len(lines), lines)
	}
}

// --- Grep Tool Tests ---

func TestGrepTool_Execute_FindsMatchingLines(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := core.NewGrepTool()
	input, _ := json.Marshal(map[string]any{"pattern": "func main", "path": dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}
	if !strings.Contains(result.Content, "func main") {
		t.Errorf("result should contain 'func main', got: %s", result.Content)
	}
	// Should have line numbers
	if !strings.Contains(result.Content, ":3:") {
		t.Errorf("result should contain line number ':3:', got: %s", result.Content)
	}
}

func TestGrepTool_Execute_NoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	tool := core.NewGrepTool()
	input, _ := json.Marshal(map[string]any{"pattern": "nonexistent_pattern", "path": dir})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result should not be error for no matches, got: %s", result.Content)
	}
	if strings.TrimSpace(result.Content) != "" {
		t.Errorf("expected empty result for no matches, got: %q", result.Content)
	}
}

// --- Bash Tool Tests ---

func TestBashTool_Execute_EchoHello(t *testing.T) {
	tool := core.NewBashTool()
	input, _ := json.Marshal(map[string]any{"command": "echo hello"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("result is error: %s", result.Content)
	}
	if strings.TrimSpace(result.Content) != "hello" {
		t.Errorf("got %q, want %q", strings.TrimSpace(result.Content), "hello")
	}
}

func TestBashTool_Execute_FailingCommand(t *testing.T) {
	tool := core.NewBashTool()
	input, _ := json.Marshal(map[string]any{"command": "false"})
	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Error("expected IsError=true for failing command")
	}
}

// --- Bash Risk Classification Tests ---

func TestClassifyBashRisk_ReadOnlyCommands(t *testing.T) {
	cases := []struct {
		command string
		want    permission.RiskLevel
	}{
		{"ls -la", permission.RiskReadOnly},
		{"cat file.txt", permission.RiskReadOnly},
		{"git status", permission.RiskReadOnly},
		{"git log --oneline", permission.RiskReadOnly},
		{"git diff HEAD", permission.RiskReadOnly},
		{"head -n 10 file.txt", permission.RiskReadOnly},
		{"tail -f log.txt", permission.RiskReadOnly},
		{"pwd", permission.RiskReadOnly},
		{"whoami", permission.RiskReadOnly},
		{"wc -l file.txt", permission.RiskReadOnly},
		{"find . -name '*.go'", permission.RiskReadOnly},
		{"which go", permission.RiskReadOnly},
		{"echo hello", permission.RiskReadOnly},
		{"env", permission.RiskReadOnly},
		{"date", permission.RiskReadOnly},
		{"file test.txt", permission.RiskReadOnly},
		{"stat test.txt", permission.RiskReadOnly},
		{"df -h", permission.RiskReadOnly},
		{"du -sh .", permission.RiskReadOnly},
		{"uname -a", permission.RiskReadOnly},
		{"hostname", permission.RiskReadOnly},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			got := core.ClassifyBashRisk(tc.command)
			if got != tc.want {
				t.Errorf("ClassifyBashRisk(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestClassifyBashRisk_DMLCommands(t *testing.T) {
	cases := []struct {
		command string
		want    permission.RiskLevel
	}{
		{"git commit -m 'x'", permission.RiskDML},
		{"git push origin main", permission.RiskDML},
		{"git add .", permission.RiskDML},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			got := core.ClassifyBashRisk(tc.command)
			if got != tc.want {
				t.Errorf("ClassifyBashRisk(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

func TestClassifyBashRisk_DestructiveCommands(t *testing.T) {
	cases := []struct {
		command string
		want    permission.RiskLevel
	}{
		{"rm -rf /tmp/foo", permission.RiskDestructive},
		{"echo hello > file", permission.RiskDestructive},
		{"cat data >> log", permission.RiskDestructive},
		{"unknown_command", permission.RiskDestructive},
	}

	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			got := core.ClassifyBashRisk(tc.command)
			if got != tc.want {
				t.Errorf("ClassifyBashRisk(%q) = %v, want %v", tc.command, got, tc.want)
			}
		})
	}
}

// --- Risk Level Tests ---

func TestToolRiskLevels(t *testing.T) {
	cases := []struct {
		tool tools.Tool
		want permission.RiskLevel
	}{
		{core.NewReadTool(), permission.RiskReadOnly},
		{core.NewGlobTool(), permission.RiskReadOnly},
		{core.NewGrepTool(), permission.RiskReadOnly},
		{core.NewWriteTool(), permission.RiskDML},
		{core.NewEditTool(), permission.RiskDML},
		{core.NewBashTool(), permission.RiskDestructive},
	}

	for _, tc := range cases {
		t.Run(tc.tool.Name(), func(t *testing.T) {
			got := tc.tool.RiskLevel()
			if got != tc.want {
				t.Errorf("%s.RiskLevel() = %v, want %v", tc.tool.Name(), got, tc.want)
			}
		})
	}
}
