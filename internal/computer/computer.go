package computer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	ErrNotImplemented = errors.New("computer action not implemented in MVP (planned for future release)")
)

// ScreenshotResult encapsulates captured screen data.
type ScreenshotResult struct {
	FilePath   string    `json:"file_path"`
	CapturedAt time.Time `json:"captured_at"`
	Width      int       `json:"width,omitempty"`
	Height     int       `json:"height,omitempty"`
}

// Computer is the high-level abstraction for host GUI control and automation.
type Computer interface {
	Screenshot(ctx context.Context) (ScreenshotResult, error)
	MoveMouse(ctx context.Context, x, y int) error
	Click(ctx context.Context, x, y int) error
	Type(ctx context.Context, text string) error
	KeyPress(ctx context.Context, key string) error
	Scroll(ctx context.Context, x, y int) error
}

// MacOSComputer implements the Computer interface on Apple Silicon / macOS.
type MacOSComputer struct {
	screenshotDir string
}

func NewMacOSComputer(screenshotDir string) *MacOSComputer {
	return &MacOSComputer{screenshotDir: screenshotDir}
}

// Screenshot captures the macOS screen.
func (c *MacOSComputer) Screenshot(ctx context.Context) (ScreenshotResult, error) {
	filename := fmt.Sprintf("mac_screen_%d.png", time.Now().Unix())
	targetPath := filepath.Join(c.screenshotDir, filename)

	cmd := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-t", "png", targetPath)
	if err := cmd.Run(); err != nil {
		return ScreenshotResult{}, fmt.Errorf("screencapture failed: %w", err)
	}

	return ScreenshotResult{
		FilePath:   targetPath,
		CapturedAt: time.Now(),
	}, nil
}

// Future GUI operations - cleanly marked as not implemented for MVP
func (c *MacOSComputer) MoveMouse(ctx context.Context, x, y int) error {
	return fmt.Errorf("%w: MoveMouse(%d, %d)", ErrNotImplemented, x, y)
}

func (c *MacOSComputer) Click(ctx context.Context, x, y int) error {
	return fmt.Errorf("%w: Click(%d, %d)", ErrNotImplemented, x, y)
}

func (c *MacOSComputer) Type(ctx context.Context, text string) error {
	return fmt.Errorf("%w: Type(%q)", ErrNotImplemented, text)
}

func (c *MacOSComputer) KeyPress(ctx context.Context, key string) error {
	return fmt.Errorf("%w: KeyPress(%q)", ErrNotImplemented, key)
}

func (c *MacOSComputer) Scroll(ctx context.Context, x, y int) error {
	return fmt.Errorf("%w: Scroll(%d, %d)", ErrNotImplemented, x, y)
}
