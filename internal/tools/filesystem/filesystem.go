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

	validPath, err := t.validator.ValidatePathWithContext(ctx, in.Path)
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

	// In conversation mode, file writes are strictly forbidden
	if sc, ok := security.SessionFromContext(ctx); ok && (sc.Type == "conversation" || sc.ReadOnly) {
		return registry.ToolResult{
			Success: false,
			Error:   "File modifications are disabled in General Conversation mode. Switch to or start a Project session to modify project files.",
		}, nil
	}

	validPath, err := t.validator.ValidatePathWithContext(ctx, in.Path)
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

// ListDirectoryTool enumerates directory contents and builds a visually pleasing tree.
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
	return "Lists directory contents and builds a visually pleasing, complete project directory tree. By default, recursively lists all files and directories including hidden files (dotfiles) without omitting any files, unless explicitly specified otherwise."
}

func (t *ListDirectoryTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to directory to list.",
			},
			"recursive": map[string]any{
				"type":        "boolean",
				"description": "Whether to recursively list all subdirectories and files in a visual tree (default: true).",
			},
			"include_hidden": map[string]any{
				"type":        "boolean",
				"description": "Whether to include hidden files and directories (dotfiles like .gitignore, .env, .github). Default is true unless explicitly set to false.",
			},
			"max_depth": map[string]any{
				"type":        "integer",
				"description": "Optional maximum recursion depth (0 or omitted = full depth).",
			},
		},
		"required": []string{"path"},
	}
}

type ListDirectoryInput struct {
	Path          string `json:"path"`
	Recursive     *bool  `json:"recursive,omitempty"`
	IncludeHidden *bool  `json:"include_hidden,omitempty"`
	MaxDepth      int    `json:"max_depth,omitempty"`
}

type FileEntry struct {
	Name          string       `json:"name"`
	Path          string       `json:"path"`
	RelativePath  string       `json:"relative_path"`
	IsDir         bool         `json:"is_dir"`
	IsHidden      bool         `json:"is_hidden"`
	Size          int64        `json:"size"`
	SizeFormatted string       `json:"size_formatted"`
	ModTime       string       `json:"mod_time"`
	Children      []*FileEntry `json:"children,omitempty"`
}

type DirectoryListingResult struct {
	RootPath           string       `json:"root_path"`
	TotalFiles         int          `json:"total_files"`
	TotalDirectories   int          `json:"total_directories"`
	TotalSizeBytes     int64        `json:"total_size_bytes"`
	TotalSizeFormatted string       `json:"total_size_formatted"`
	Entries            []*FileEntry `json:"entries"`
}

// FormatFileSize returns a human-readable file size string.
func FormatFileSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	val := float64(bytes)
	for _, u := range units {
		val /= 1024.0
		if val < 1024.0 || u == "TB" {
			if val < 10.0 {
				return fmt.Sprintf("%.1f %s", val, u)
			}
			return fmt.Sprintf("%.0f %s", val, u)
		}
	}
	return fmt.Sprintf("%d B", bytes)
}

func (t *ListDirectoryTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in ListDirectoryInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	targetPath := in.Path
	if strings.TrimSpace(targetPath) == "" || targetPath == "." {
		if sc, ok := security.SessionFromContext(ctx); ok && sc.ProjectPath != "" {
			targetPath = sc.ProjectPath
		}
	}

	validPath, err := t.validator.ValidatePathWithContext(ctx, targetPath)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	isRecursive := true
	if in.Recursive != nil {
		isRecursive = *in.Recursive
	}

	includeHidden := true
	if in.IncludeHidden != nil {
		includeHidden = *in.IncludeHidden
	}

	maxDepth := in.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 50 // Safe recursion depth limit
	}

	var totalFiles int
	var totalDirs int
	var totalSize int64
	visited := make(map[string]bool)

	entries, err := buildDirectoryTree(
		validPath,
		validPath,
		1,
		maxDepth,
		isRecursive,
		includeHidden,
		&totalFiles,
		&totalDirs,
		&totalSize,
		visited,
	)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📁 %s (%d files, %d directories • %s)\n", validPath, totalFiles, totalDirs, FormatFileSize(totalSize)))

	if len(entries) == 0 {
		sb.WriteString("└── (empty directory)\n")
	} else {
		renderTree(entries, "", &sb)
	}

	resultData := DirectoryListingResult{
		RootPath:           validPath,
		TotalFiles:         totalFiles,
		TotalDirectories:   totalDirs,
		TotalSizeBytes:     totalSize,
		TotalSizeFormatted: FormatFileSize(totalSize),
		Entries:            entries,
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data:    resultData,
	}, nil
}

func buildDirectoryTree(
	rootPath string,
	currentDir string,
	currentDepth int,
	maxDepth int,
	recursive bool,
	includeHidden bool,
	totalFiles *int,
	totalDirs *int,
	totalSize *int64,
	visited map[string]bool,
) ([]*FileEntry, error) {
	realPath, err := filepath.EvalSymlinks(currentDir)
	if err != nil {
		realPath = currentDir
	}
	if visited[realPath] {
		return nil, nil
	}
	visited[realPath] = true

	dirEntries, err := os.ReadDir(currentDir)
	if err != nil {
		return nil, err
	}

	var dirNodes []*FileEntry
	var fileNodes []*FileEntry

	for _, de := range dirEntries {
		name := de.Name()
		isHidden := strings.HasPrefix(name, ".")

		if !includeHidden && isHidden {
			continue
		}

		fullPath := filepath.Join(currentDir, name)
		relPath, _ := filepath.Rel(rootPath, fullPath)
		info, err := de.Info()

		size := int64(0)
		modTime := ""
		if err == nil {
			size = info.Size()
			modTime = info.ModTime().Format(time.RFC3339)
		}

		node := &FileEntry{
			Name:          name,
			Path:          fullPath,
			RelativePath:  relPath,
			IsDir:         de.IsDir(),
			IsHidden:      isHidden,
			Size:          size,
			SizeFormatted: FormatFileSize(size),
			ModTime:       modTime,
		}

		if de.IsDir() {
			*totalDirs++
			if recursive && currentDepth < maxDepth {
				subChildren, err := buildDirectoryTree(
					rootPath,
					fullPath,
					currentDepth+1,
					maxDepth,
					recursive,
					includeHidden,
					totalFiles,
					totalDirs,
					totalSize,
					visited,
				)
				if err == nil {
					node.Children = subChildren
				}
			}
			dirNodes = append(dirNodes, node)
		} else {
			*totalFiles++
			*totalSize += size
			fileNodes = append(fileNodes, node)
		}
	}

	// Sort directories first (case-insensitive), then files (case-insensitive)
	sortEntries(dirNodes)
	sortEntries(fileNodes)

	allEntries := append(dirNodes, fileNodes...)
	return allEntries, nil
}

func sortEntries(entries []*FileEntry) {
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if strings.ToLower(entries[i].Name) > strings.ToLower(entries[j].Name) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}

func renderTree(entries []*FileEntry, prefix string, sb *strings.Builder) {
	for i, entry := range entries {
		isLast := (i == len(entries)-1)
		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		if entry.IsDir {
			sb.WriteString(fmt.Sprintf("%s%s📁 %s/\n", prefix, connector, entry.Name))
			if len(entry.Children) > 0 {
				renderTree(entry.Children, childPrefix, sb)
			}
		} else {
			sb.WriteString(fmt.Sprintf("%s%s📄 %s (%s)\n", prefix, connector, entry.Name, entry.SizeFormatted))
		}
	}
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
	return "Searches files within an authorized directory matching a name pattern or text content query. Includes hidden files by default."
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
				"description": "Filename pattern to match (e.g. '*.go', 'test_*.py', '.env*').",
			},
			"text_query": map[string]any{
				"type":        "string",
				"description": "Text substring to search for inside files.",
			},
			"include_hidden": map[string]any{
				"type":        "boolean",
				"description": "Whether to search hidden files and directories (default: true).",
			},
			"max_results": map[string]any{
				"type":        "integer",
				"description": "Maximum number of search matches to return (default: 1000).",
			},
		},
		"required": []string{"directory"},
	}
}

type SearchFilesInput struct {
	Directory     string `json:"directory"`
	NamePattern   string `json:"name_pattern,omitempty"`
	TextQuery     string `json:"text_query,omitempty"`
	IncludeHidden *bool  `json:"include_hidden,omitempty"`
	MaxResults    int    `json:"max_results,omitempty"`
}

func (t *SearchFilesTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in SearchFilesInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	targetDir := in.Directory
	if strings.TrimSpace(targetDir) == "" || targetDir == "." {
		if sc, ok := security.SessionFromContext(ctx); ok && sc.ProjectPath != "" {
			targetDir = sc.ProjectPath
		}
	}

	validDir, err := t.validator.ValidatePathWithContext(ctx, targetDir)
	if err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	includeHidden := true
	if in.IncludeHidden != nil {
		includeHidden = *in.IncludeHidden
	}

	maxResults := in.MaxResults
	if maxResults <= 0 {
		maxResults = 1000
	}

	var matches []string
	err = filepath.WalkDir(validDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if !includeHidden && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}

		if !includeHidden && strings.HasPrefix(d.Name(), ".") {
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
		if len(matches) >= maxResults {
			return io.EOF // Stop at maxResults
		}
		return nil
	})

	if err != nil && err != io.EOF {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("Found %d matching file(s) in %s:\n", len(matches), validDir))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("- %s\n", m))
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data:    matches,
	}, nil
}

// FindProjectTool searches authorized workspaces for a project or folder by name.
type FindProjectTool struct {
	validator *security.PathValidator
}

func NewFindProjectTool(validator *security.PathValidator) *FindProjectTool {
	return &FindProjectTool{validator: validator}
}

func (t *FindProjectTool) Name() string {
	return "find_project"
}

func (t *FindProjectTool) Description() string {
	return "Locates a project, repository, or subdirectory by name across all authorized workspaces (e.g. 'jeroidpay', 'oxcart', 'server'). Returns exact absolute paths."
}

func (t *FindProjectTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Project or directory name to locate (e.g. 'jeroidpay', 'server', 'Bob-AI').",
			},
		},
		"required": []string{"name"},
	}
}

type FindProjectInput struct {
	Name string `json:"name"`
}

func (t *FindProjectTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in FindProjectInput
	if err := json.Unmarshal(rawInput, &in); err != nil {
		return registry.ToolResult{Success: false, Error: err.Error()}, err
	}

	trimmed := strings.TrimSpace(in.Name)
	if trimmed == "" {
		return registry.ToolResult{Success: false, Error: "project name cannot be empty"}, fmt.Errorf("empty name")
	}

	matches := t.validator.FindMatchingPathsWithContext(ctx, trimmed)
	if len(matches) == 0 {
		return registry.ToolResult{
			Success: true,
			Output:  fmt.Sprintf("No project or directory named %q found in authorized workspaces (%v).", trimmed, t.validator.AllowedRoots()),
			Data:    []string{},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d matching path(s) for %q:\n", len(matches), trimmed))
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("- %s\n", m))
	}

	return registry.ToolResult{
		Success: true,
		Output:  sb.String(),
		Data:    matches,
	}, nil
}
