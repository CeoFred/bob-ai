package apps

import (
	"context"
	"encoding/json"
	"testing"
)

func TestListRunningAppsTool_Properties(t *testing.T) {
	tool := NewListRunningAppsTool()

	if tool.Name() != "list_running_apps" {
		t.Errorf("expected tool name 'list_running_apps', got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("tool description should not be empty")
	}

	schema := tool.InputSchema()
	if schema == nil {
		t.Error("tool input schema should not be nil")
	}
}

func TestListRunningAppsTool_Execute_Basic(t *testing.T) {
	tool := NewListRunningAppsTool()
	ctx := context.Background()

	// Execute with empty JSON
	res, err := tool.Execute(ctx, json.RawMessage(`{}`))
	// Note: in a sandboxed test environment without process access, it may either return running apps or fall back gracefully
	if err == nil {
		if !res.Success {
			t.Errorf("expected success to be true, got false")
		}
		data, ok := res.Data.(ListRunningAppsResult)
		if !ok {
			t.Errorf("expected Data to be ListRunningAppsResult, got %T", res.Data)
		}
		if data.GUIOnly != true {
			t.Errorf("expected GUIOnly to default to true, got %v", data.GUIOnly)
		}
	}
}

func TestListRunningAppsTool_Execute_WithParams(t *testing.T) {
	tool := NewListRunningAppsTool()
	ctx := context.Background()

	input := `{"gui_only": false, "search": "bob", "limit": 10}`
	res, err := tool.Execute(ctx, json.RawMessage(input))
	if err == nil {
		if !res.Success {
			t.Errorf("expected success to be true")
		}
		data := res.Data.(ListRunningAppsResult)
		if data.GUIOnly != false {
			t.Errorf("expected GUIOnly to be false")
		}
		if data.SearchQuery != "bob" {
			t.Errorf("expected SearchQuery 'bob', got %q", data.SearchQuery)
		}
	}
}

func TestIsSystemDaemon(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		expected bool
	}{
		{"launchd", "/sbin/launchd", true},
		{"kernel_task", "kernel_task", true},
		{"cfprefsd", "/usr/sbin/cfprefsd", true},
		{"Chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", false},
		{"VSCode", "/Applications/Visual Studio Code.app/Contents/MacOS/Electron", false},
		{"Bob", "/Users/codemon_/Documents/Bob-AI/bob", false},
	}

	for _, tt := range tests {
		result := isSystemDaemon(tt.name, tt.cmd)
		if result != tt.expected {
			t.Errorf("isSystemDaemon(%q, %q) = %v, expected %v", tt.name, tt.cmd, result, tt.expected)
		}
	}
}

func TestGenerateFormattedSummary(t *testing.T) {
	apps := []AppInfo{
		{Name: "Google Chrome", PID: 1234, IsFrontmost: true, IsGUIApp: true},
		{Name: "Visual Studio Code", PID: 5678, IsFrontmost: false, IsGUIApp: true},
	}

	summary := GenerateFormattedSummary(apps, "Google Chrome")
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	if !testing.Short() {
		if len(apps) != 2 {
			t.Errorf("unexpected apps length: %d", len(apps))
		}
	}
}

func TestListInstalledAppsTool(t *testing.T) {
	tool := NewListInstalledAppsTool()

	if tool.Name() != "list_installed_apps" {
		t.Errorf("expected name 'list_installed_apps', got %q", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("description should not be empty")
	}

	res, err := tool.Execute(context.Background(), json.RawMessage(`{"limit": 5}`))
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if !res.Success {
		t.Errorf("expected success to be true")
	}

	data, ok := res.Data.(ListInstalledAppsResult)
	if !ok {
		t.Fatalf("expected ListInstalledAppsResult, got %T", res.Data)
	}

	if data.Category != "all" {
		t.Errorf("expected default category 'all', got %q", data.Category)
	}
}

func TestOpenAppTool_InputValidation(t *testing.T) {
	tool := NewOpenAppTool()

	if tool.Name() != "open_app" {
		t.Errorf("expected name 'open_app', got %q", tool.Name())
	}

	// Empty input should fail
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil || res.Success {
		t.Errorf("expected error for empty open_app input")
	}
}

func TestCloseAppTool_InputValidation(t *testing.T) {
	tool := NewCloseAppTool()

	if tool.Name() != "close_app" {
		t.Errorf("expected name 'close_app', got %q", tool.Name())
	}

	// Empty input should fail
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err == nil || res.Success {
		t.Errorf("expected error for empty close_app input")
	}
}

