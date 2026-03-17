package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cascade-cli/cascade/internal/permission"
	"github.com/cascade-cli/cascade/internal/tools"
)

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Include string `json:"include"`
}

// GrepTool searches file contents using regex.
type GrepTool struct{}

func NewGrepTool() *GrepTool { return &GrepTool{} }

func (t *GrepTool) Name() string       { return "grep" }
func (t *GrepTool) Description() string { return "Search file contents using regex pattern" }

func (t *GrepTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regex pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "File or directory to search in (defaults to cwd)",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "Glob pattern to filter files (e.g., *.go)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) RiskLevel() permission.RiskLevel { return permission.RiskReadOnly }

func (t *GrepTool) Execute(_ context.Context, input json.RawMessage) (*tools.Result, error) {
	var params grepInput
	if err := json.Unmarshal(input, &params); err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid input: %v", err), IsError: true}, nil
	}

	re, err := regexp.Compile(params.Pattern)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("invalid regex: %v", err), IsError: true}, nil
	}

	searchPath := params.Path
	if searchPath == "" {
		searchPath, err = os.Getwd()
		if err != nil {
			return &tools.Result{Content: fmt.Sprintf("error getting cwd: %v", err), IsError: true}, nil
		}
	}

	const maxMatches = 500
	var matches []string

	info, err := os.Stat(searchPath)
	if err != nil {
		return &tools.Result{Content: fmt.Sprintf("error: %v", err), IsError: true}, nil
	}

	if !info.IsDir() {
		// Search single file
		fileMatches, err := grepFile(searchPath, re, maxMatches)
		if err != nil {
			return &tools.Result{Content: fmt.Sprintf("error: %v", err), IsError: true}, nil
		}
		matches = fileMatches
	} else {
		// Walk directory
		err = filepath.Walk(searchPath, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil // skip errors
			}
			if fi.IsDir() {
				// Skip hidden directories
				if strings.HasPrefix(fi.Name(), ".") && fi.Name() != "." {
					return filepath.SkipDir
				}
				return nil
			}

			// Apply include filter
			if params.Include != "" {
				matched, _ := filepath.Match(params.Include, fi.Name())
				if !matched {
					return nil
				}
			}

			// Skip binary files (simple heuristic: skip files > 1MB or with common binary extensions)
			if fi.Size() > 1<<20 {
				return nil
			}

			fileMatches, err := grepFile(path, re, maxMatches-len(matches))
			if err != nil {
				return nil // skip files that can't be read
			}
			matches = append(matches, fileMatches...)

			if len(matches) >= maxMatches {
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return &tools.Result{Content: fmt.Sprintf("error walking directory: %v", err), IsError: true}, nil
		}
	}

	content := strings.Join(matches, "\n")
	return &tools.Result{Content: content, Display: content}, nil
}

func grepFile(path string, re *regexp.Regexp, maxResults int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) {
			matches = append(matches, fmt.Sprintf("%s:%d:%s", path, lineNum, line))
			if len(matches) >= maxResults {
				break
			}
		}
	}
	return matches, scanner.Err()
}
