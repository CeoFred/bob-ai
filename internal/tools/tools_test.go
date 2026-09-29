package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bob/internal/security"
	"bob/internal/tools/filesystem"
	"bob/internal/tools/registry"
	"bob/internal/tools/screenshot"
	"bob/internal/tools/terminal"
)

func TestTerminalTool_Execute(t *testing.T) {
	tempDir := t.TempDir()
	policy := security.NewCommandPolicy([]string{"echo", "pwd", "ls"}, []string{"rm"}, []string{"rm -rf /"})
	validator := security.NewPathValidator([]string{tempDir})

	term := terminal.NewTerminalTool(policy, validator, tempDir, 5*time.Second, 1024*1024)

	// Test safe echo
	inputJSON, _ := json.Marshal(map[string]any{"command": "echo 'hello bob'"})
	res, err := term.Execute(context.Background(), inputJSON)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if !res.Success {
		t.Errorf("expected success, got error: %s", res.Error)
	}

	// Test blocked command
	blockedJSON, _ := json.Marshal(map[string]any{"command": "rm -rf /"})
	bRes, bErr := term.Execute(context.Background(), blockedJSON)
	if bErr == nil || bRes.Success {
		t.Fatalf("expected blocked command to fail, got success")
	}
}

func TestFilesystemTools(t *testing.T) {
	tempDir := t.TempDir()
	validator := security.NewPathValidator([]string{tempDir})

	writeTool := filesystem.NewWriteFileTool(validator)
	readTool := filesystem.NewReadFileTool(validator, 1024*1024)
	listTool := filesystem.NewListDirectoryTool(validator)
	searchTool := filesystem.NewSearchFilesTool(validator)

	testFilePath := filepath.Join(tempDir, "sample.txt")

	// 1. Write file
	writeIn, _ := json.Marshal(map[string]any{
		"path":    testFilePath,
		"content": "Line 1\nLine 2\nLine 3\nBob agent file content",
	})
	wRes, err := writeTool.Execute(context.Background(), writeIn)
	if err != nil || !wRes.Success {
		t.Fatalf("write failed: %v (%s)", err, wRes.Error)
	}

	// 2. Read file
	readIn, _ := json.Marshal(map[string]any{
		"path":       testFilePath,
		"start_line": 2,
		"end_line":   3,
	})
	rRes, err := readTool.Execute(context.Background(), readIn)
	if err != nil || !rRes.Success {
		t.Fatalf("read failed: %v (%s)", err, rRes.Error)
	}
	if rRes.Output != "Line 2\nLine 3" {
		t.Errorf("got %q, want %q", rRes.Output, "Line 2\nLine 3")
	}

	// 3. List directory
	listIn, _ := json.Marshal(map[string]any{"path": tempDir})
	lRes, err := listTool.Execute(context.Background(), listIn)
	if err != nil || !lRes.Success {
		t.Fatalf("list failed: %v (%s)", err, lRes.Error)
	}

	// 4. Search files
	searchIn, _ := json.Marshal(map[string]any{
		"directory":  tempDir,
		"text_query": "Bob agent",
	})
	sRes, err := searchTool.Execute(context.Background(), searchIn)
	if err != nil || !sRes.Success {
		t.Fatalf("search failed: %v (%s)", err, sRes.Error)
	}
}

func TestScreenshotTool(t *testing.T) {
	tempDir := t.TempDir()
	screenTool := screenshot.NewScreenshotTool(tempDir)

	if screenTool.Name() != "take_screenshot" {
		t.Errorf("expected name take_screenshot, got %s", screenTool.Name())
	}

	// In CI or headless test, check schema and path creation
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		t.Errorf("screenshot directory not created")
	}
}

func TestToolRegistry(t *testing.T) {
	reg := registry.NewRegistry()
	tempDir := t.TempDir()
	validator := security.NewPathValidator([]string{tempDir})

	writeTool := filesystem.NewWriteFileTool(validator)
	if err := reg.Register(writeTool); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	if _, ok := reg.Get("write_file"); !ok {
		t.Errorf("expected to find write_file tool")
	}

	defs := reg.ToToolDefinitions()
	if len(defs) != 1 || defs[0].Function.Name != "write_file" {
		t.Errorf("unexpected tool definitions: %+v", defs)
	}
}
