package filesystem

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bob/internal/security"
	"bob/internal/tools/registry"
)

// ReadFileTool reads content from an authorized file.
type ReadFileTool struct {
	validator   *security.PathValidator
	maxReadSize int64
}

func NewReadFileTool(validator *security.PathValidator, maxReadSize int64) *ReadFileTool {
	if maxReadSize <= 0 {
		maxReadSize = 5 * 1024 * 1024
	}
	return &ReadFileTool{validator: validator, maxReadSize: maxReadSize}
}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) Description() string {
	return "Reads text content from a file within authorized workspace directories. Optional offset and limit lines."
}

func (t *ReadFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to read (relative to workspace or absolute).",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional 1-based start line.",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional 1-based end line.",
			},
		},
		"required": []string{"path"},
	}
}

type ReadFileInput struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}

func (t *ReadFileTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in ReadFileInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	validPath, err := t.validator.ValidatePath(in.Path)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	file, err := os.Open(validPath)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	if stat.IsDir() {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("path is a directory: %s", validPath),
		}, fmt.Errorf("is a directory")
	}

	reader := io.LimitReader(file, t.maxReadSize)
	contentBytes, err := io.ReadAll(reader)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	content := string(contentBytes)
	lines := strings.Split(content, "\n")

	if in.StartLine > 0 || in.EndLine > 0 {
		start := 1
		end := len(lines)
		if in.StartLine > 0 {
			start = in.StartLine
		}
		if in.EndLine > 0 && in.EndLine < end {
			end = in.EndLine
		}
		if start > len(lines) {
			return registry.ToolResult{Success: true, Output: "[Empty: start_line is past EOF]"}, nil
		}
		if start < 1 {
			start = 1
		}
		sliced := lines[start-1 : end]
		content = strings.Join(sliced, "\n")
	}

	return registry.ToolResult{
		Success: true,
		Output:  content,
		Data: map[string]any{
			"path":       validPath,
			"size_bytes": stat.Size(),
			"line_count": len(lines),
		},
	}, nil
}

// WriteFileTool creates or replaces file contents.
type WriteFileTool struct {
	validator *security.PathValidator
}

func NewWriteFileTool(validator *security.PathValidator) *WriteFileTool {
	return &WriteFileTool{validator: validator}
}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) Description() string {
	return "Writes or updates file contents within authorized workspace directories. Creates parent directories automatically."
}

func (t *WriteFileTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to create or overwrite.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The complete text content to write.",
			},
		},
		"required": []string{"path", "content"},
	}
}

type WriteFileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteFileTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in WriteFileInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	validPath, err := t.validator.ValidatePath(in.Path)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	// Create parent dir
	dir := filepath.Dir(validPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return registry.ToolResult{Success: false, Error: fmt.Sprintf("failed to create directory: %v", err)}, err
	}

	if err := os.WriteFile(validPath, []byte(in.Content), 0644); err != nil {
		return registry.ToolResult{Success: false, Error: fmt.Sprintf("failed to write file: %v", err)}, err
	}

	return registry.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Successfully wrote %d bytes to %s", len(in.Content), validPath),
		Data: map[string]any{
			"path":        validPath,
			"bytes_wrote": len(in.Content),
		},
	}, nil
}

// ListDirectoryTool enumerates directory contents.
type ListDirectoryTool struct {
	validator *security.PathValidator
}

func NewListDirectoryTool(validator *security.PathValidator) *ListDirectoryTool {
	return &ListDirectoryTool{validator: validator}
}

func (t *ListDirectoryTool) Name() string {
	return "list_directory"
}

func (t *ListDirectoryTool) Description() string {
	return "Lists files and subdirectories within an authorized directory path."
}

func (t *ListDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to directory to list.",
			},
		},
		"required": []string{"path"},
	}
}

type ListDirectoryInput struct {
	Path string `json:"path"`
}

type FileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

func (t *ListDirectoryTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in ListDirectoryInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	validPath, err := t.validator.ValidatePath(in.Path)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	entries, err := os.ReadDir(validPath)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	var results []FileEntry
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory listing for %s (%d items):\n", validPath, len(entries)))

	for _, entry := range entries {
		info, err := entry.Info()
		size := int64(0)
		modTime := ""
		if err == nil {
			size = info.Size()
			modTime = info.ModTime().Format(time.RFC3339)
		}

		fe := FileEntry{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    size,
			ModTime: modTime,
		}
		results = append(results, fe)

		typeMarker := "FILE"
		if entry.IsDir() {
			typeMarker = "DIR "
		}
		sb.WriteString(fmt.Sprintf("[%s] %-30s %10d bytes  %s\n", typeMarker, entry.Name(), size, modTime))
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data:    results,
	}, nil
}

// SearchFilesTool finds files matching a name or content query.
type SearchFilesTool struct {
	validator *security.PathValidator
}

func NewSearchFilesTool(validator *security.PathValidator) *SearchFilesTool {
	return &SearchFilesTool{validator: validator}
}

func (t *SearchFilesTool) Name() string {
	return "search_files"
}

func (t *SearchFilesTool) Description() string {
	return "Searches files within an authorized directory matching a name pattern or text content query."
}

func (t *SearchFilesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"directory": map[string]any{
				"type":        "string",
				"description": "Root directory for the search.",
			},
			"name_pattern": map[string]any{
				"type":        "string",
				"description": "Filename pattern to match (e.g. '*.go', 'test_*.py').",
			},
			"text_query": map[string]any{
				"type":        "string",
				"description": "Text substring to search for inside files.",
			},
		},
		"required": []string{"directory"},
	}
}

type SearchFilesInput struct {
	Directory   string `json:"directory"`
	NamePattern string `json:"name_pattern,omitempty"`
	TextQuery   string `json:"text_query,omitempty"`
}

func (t *SearchFilesTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in SearchFilesInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	validDir, err := t.validator.ValidatePath(in.Directory)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	var matches []string
	err = filepath.WalkDir(validDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		if in.NamePattern != "" {
			matched, _ := filepath.Match(in.NamePattern, d.Name())
			if !matched {
				return nil
			}
		}

		if in.TextQuery != "" {
			data, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(data), in.TextQuery) {
				return nil
			}
		}

		rel, _ := filepath.Rel(validDir, path)
		matches = append(matches, rel)
		if len(matches) >= 100 {
			return io.EOF // Stop at 100 matches
		}
		return nil
	})

	if err != nil && err != io.EOF {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Found %d matching files in %s:\n", len(matches), validDir))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("- %s\n", m))
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data:    matches,
	}, nil
}
