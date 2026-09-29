package screenshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"bob/internal/tools/registry"
)

// ScreenshotTool captures the Mac display.
type ScreenshotTool struct {
	screenshotsDir string
}

func NewScreenshotTool(screenshotsDir string) *ScreenshotTool {
	if screenshotsDir == "" {
		home, _ := os.UserHomeDir()
		screenshotsDir = filepath.Join(home, ".bob", "screenshots")
	}
	_ = os.MkdirAll(screenshotsDir, 0755)
	return &ScreenshotTool{screenshotsDir: screenshotsDir}
}

func (t *ScreenshotTool) Name() string {
	return "take_screenshot"
}

func (t *ScreenshotTool) Description() string {
	return "Captures a full screenshot of the macOS display and saves it for agent inspection and remote viewing."
}

func (t *ScreenshotTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"label": map[string]any{
				"type":        "string",
				"description": "Optional label or description for this capture.",
			},
		},
	}
}

type ScreenshotInput struct {
	Label string `json:"label,omitempty"`
}

type ScreenshotData struct {
	FilePath     string `json:"file_path"`
	Filename     string `json:"filename"`
	Timestamp    string `json:"timestamp"`
	SizeBytes    int64  `json:"size_bytes"`
	Base64Data   string `json:"base64_data,omitempty"`
	URL          string `json:"url"`
}

func (t *ScreenshotTool) Execute(ctx context.Context, rawInput json.RawMessage) (registry.ToolResult, error) {
	var in ScreenshotInput
	_ = json.Unmarshal(rawInput, &in)

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("screenshot_%s.png", timestamp)
	targetPath := filepath.Join(t.screenshotsDir, filename)

	// Invoke macOS screencapture
	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-t", "png", targetPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("screencapture failed (check Screen Recording permissions): %v - %s", err, string(out)),
		}, fmt.Errorf("screencapture failed: %w", err)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return registry.ToolResult{
			Success: false,
			Error:   fmt.Sprintf("failed to stat screenshot file: %v", err),
		}, err
	}

	// Read small preview if file exists
	imgBytes, _ := os.ReadFile(targetPath)
	var b64Preview string
	if len(imgBytes) > 0 {
		b64Preview = base64.StdEncoding.EncodeToString(imgBytes)
	}

	data := ScreenshotData{
		FilePath:   targetPath,
		Filename:   filename,
		Timestamp:  time.Now().Format(time.RFC3339),
		SizeBytes:  info.Size(),
		Base64Data: b64Preview,
		URL:        fmt.Sprintf("/api/screenshots/%s", filename),
	}

	return registry.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Screenshot captured successfully (%d bytes): %s", info.Size(), filename),
		Data:    data,
	}, nil
}
