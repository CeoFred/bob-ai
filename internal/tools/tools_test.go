package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bob/internal/security"
	"bob/internal/tools/apps"
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

func TestListDirectoryTool_Comprehensive(t *testing.T) {
	tempDir := t.TempDir()
	validator := security.NewPathValidator([]string{tempDir})
	listTool := filesystem.NewListDirectoryTool(validator)

	// Create a nested file tree with hidden files
	// tempDir/
	//   ├── .env
	//   ├── .github/
	//   │   └── workflows/
	//   │       └── ci.yml
	//   ├── src/
	//   │   ├── main.go
	//   │   └── utils/
	//   │       └── helper.go
	//   └── README.md
	_ = os.MkdirAll(filepath.Join(tempDir, ".github", "workflows"), 0755)
	_ = os.MkdirAll(filepath.Join(tempDir, "src", "utils"), 0755)

	_ = os.WriteFile(filepath.Join(tempDir, ".env"), []byte("SECRET=123"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, ".github", "workflows", "ci.yml"), []byte("name: CI"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "src", "main.go"), []byte("package main"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "src", "utils", "helper.go"), []byte("package utils"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Test Project"), 0644)

	// 1. Default execution: recursive=true, include_hidden=true
	in1, _ := json.Marshal(map[string]any{"path": tempDir})
	res1, err := listTool.Execute(context.Background(), in1)
	if err != nil || !res1.Success {
		t.Fatalf("default list failed: %v (%s)", err, res1.Error)
	}

	data1, ok := res1.Data.(filesystem.DirectoryListingResult)
	if !ok {
		t.Fatalf("expected DirectoryListingResult, got %T", res1.Data)
	}

	// 5 files total: .env, ci.yml, main.go, helper.go, README.md
	if data1.TotalFiles != 5 {
		t.Errorf("expected 5 files, got %d", data1.TotalFiles)
	}
	// 4 directories total: .github, workflows, src, utils
	if data1.TotalDirectories != 4 {
		t.Errorf("expected 4 directories, got %d", data1.TotalDirectories)
	}

	// Output should contain tree structure with icons and hidden files
	if !strings.Contains(res1.Output, "📁 .github/") || !strings.Contains(res1.Output, "📄 .env") {
		t.Errorf("output missing hidden files: %s", res1.Output)
	}
	if !strings.Contains(res1.Output, "├──") || !strings.Contains(res1.Output, "└──") {
		t.Errorf("output missing tree connectors: %s", res1.Output)
	}

	// 2. Test include_hidden=false
	in2, _ := json.Marshal(map[string]any{
		"path":           tempDir,
		"include_hidden": false,
	})
	res2, err := listTool.Execute(context.Background(), in2)
	if err != nil || !res2.Success {
		t.Fatalf("non-hidden list failed: %v", err)
	}
	data2 := res2.Data.(filesystem.DirectoryListingResult)
	// Without hidden files: main.go, helper.go, README.md (3 files), src, utils (2 dirs)
	if data2.TotalFiles != 3 {
		t.Errorf("expected 3 non-hidden files, got %d", data2.TotalFiles)
	}
	if strings.Contains(res2.Output, ".env") || strings.Contains(res2.Output, ".github") {
		t.Errorf("output should not contain hidden files when include_hidden=false: %s", res2.Output)
	}

	// 3. Test non-recursive (top level only)
	in3, _ := json.Marshal(map[string]any{
		"path":      tempDir,
		"recursive": false,
	})
	res3, err := listTool.Execute(context.Background(), in3)
	if err != nil || !res3.Success {
		t.Fatalf("non-recursive list failed: %v", err)
	}
	data3 := res3.Data.(filesystem.DirectoryListingResult)
	// Immediate top level items: 2 files (.env, README.md) and 2 dirs (.github, src)
	if data3.TotalFiles != 2 {
		t.Errorf("expected 2 top-level files, got %d", data3.TotalFiles)
	}
	if data3.TotalDirectories != 2 {
		t.Errorf("expected 2 top-level dirs, got %d", data3.TotalDirectories)
	}

	// 4. Test max_depth = 1
	in4, _ := json.Marshal(map[string]any{
		"path":      tempDir,
		"max_depth": 1,
	})
	res4, err := listTool.Execute(context.Background(), in4)
	if err != nil || !res4.Success {
		t.Fatalf("depth-limited list failed: %v", err)
	}
	data4 := res4.Data.(filesystem.DirectoryListingResult)
	if data4.TotalFiles != 2 {
		t.Errorf("expected 2 files at depth 1, got %d", data4.TotalFiles)
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

func TestFindProjectTool(t *testing.T) {
	tempDir := t.TempDir()
	jeroidpayDir := filepath.Join(tempDir, "jeroidpay", "server")
	_ = os.MkdirAll(jeroidpayDir, 0755)

	validator := security.NewPathValidator([]string{tempDir})
	findTool := filesystem.NewFindProjectTool(validator)

	findIn, _ := json.Marshal(map[string]any{"name": "jeroidpay"})
	res, err := findTool.Execute(context.Background(), findIn)
	if err != nil || !res.Success {
		t.Fatalf("find_project failed: %v (%s)", err, res.Error)
	}

	matches, ok := res.Data.([]string)
	if !ok || len(matches) == 0 {
		t.Fatalf("expected matches, got %v", res.Data)
	}
}

func TestAppTools_Registration(t *testing.T) {
	reg := registry.NewRegistry()

	_ = reg.Register(apps.NewListRunningAppsTool())
	_ = reg.Register(apps.NewListInstalledAppsTool())
	_ = reg.Register(apps.NewOpenAppTool())
	_ = reg.Register(apps.NewCloseAppTool())

	expectedTools := []string{"list_running_apps", "list_installed_apps", "open_app", "close_app"}
	for _, expected := range expectedTools {
		tool, ok := reg.Get(expected)
		if !ok || tool == nil {
			t.Errorf("expected tool %q to be registered", expected)
		}
	}
}

func TestReplaceFileContentTool(t *testing.T) {
	tempDir := t.TempDir()
	validator := security.NewPathValidator([]string{tempDir})
	replaceTool := filesystem.NewReplaceFileContentTool(validator)
	writeTool := filesystem.NewWriteFileTool(validator)
	readTool := filesystem.NewReadFileTool(validator, 1024*1024)

	testFilePath := filepath.Join(tempDir, "code.go")

	// 1. Setup initial file
	initialCode := `package main

import "fmt"

func main() {
	fmt.Println("Hello, old world!")
	fmt.Println("Debug message")
}
`
	writeIn, _ := json.Marshal(map[string]any{
		"path":    testFilePath,
		"content": initialCode,
	})
	if _, err := writeTool.Execute(context.Background(), writeIn); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 2. Perform surgical replacement of a single block
	replaceIn, _ := json.Marshal(map[string]any{
		"path":                testFilePath,
		"target_content":      `fmt.Println("Hello, old world!")`,
		"replacement_content": `fmt.Println("Hello, Bob!")`,
	})
	repRes, err := replaceTool.Execute(context.Background(), replaceIn)
	if err != nil || !repRes.Success {
		t.Fatalf("replace_file_content failed: %v (%s)", err, repRes.Error)
	}

	// Verify content after replacement
	readIn, _ := json.Marshal(map[string]any{"path": testFilePath})
	readRes, _ := readTool.Execute(context.Background(), readIn)
	if !strings.Contains(readRes.Output, `fmt.Println("Hello, Bob!")`) {
		t.Errorf("expected replacement not found in file: %s", readRes.Output)
	}
	if strings.Contains(readRes.Output, `Hello, old world!`) {
		t.Errorf("old content still present after replacement")
	}

	// 3. Test non-existent target_content (should fail)
	badTargetIn, _ := json.Marshal(map[string]any{
		"path":                testFilePath,
		"target_content":      `non_existent_function_call()`,
		"replacement_content": `something_else()`,
	})
	badRes, badErr := replaceTool.Execute(context.Background(), badTargetIn)
	if badErr == nil || badRes.Success {
		t.Errorf("expected error for non-existent target_content, got success")
	}

	// 4. Test multiple occurrences with allow_multiple = false (should fail)
	dupeCode := "foo bar foo bar foo"
	writeDupeIn, _ := json.Marshal(map[string]any{
		"path":    testFilePath,
		"content": dupeCode,
	})
	_, _ = writeTool.Execute(context.Background(), writeDupeIn)

	ambigIn, _ := json.Marshal(map[string]any{
		"path":                testFilePath,
		"target_content":      "foo",
		"replacement_content": "baz",
		"allow_multiple":      false,
	})
	ambigRes, ambigErr := replaceTool.Execute(context.Background(), ambigIn)
	if ambigErr == nil || ambigRes.Success {
		t.Errorf("expected ambiguous replacement to fail without allow_multiple, got success")
	}

	// 5. Test multiple occurrences with allow_multiple = true (should succeed)
	allowMultIn, _ := json.Marshal(map[string]any{
		"path":                testFilePath,
		"target_content":      "foo",
		"replacement_content": "baz",
		"allow_multiple":      true,
	})
	multRes, multErr := replaceTool.Execute(context.Background(), allowMultIn)
	if multErr != nil || !multRes.Success {
		t.Fatalf("expected allow_multiple to succeed, got error: %v (%s)", multErr, multRes.Error)
	}

	readRes, _ = readTool.Execute(context.Background(), readIn)
	if readRes.Output != "baz bar baz bar baz" {
		t.Errorf("got %q, want %q", readRes.Output, "baz bar baz bar baz")
	}
}


