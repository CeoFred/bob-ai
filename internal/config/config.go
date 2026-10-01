package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the full configuration for Bob.
type Config struct {
	Server     ServerConfig     `yaml:"server" json:"server"`
	LLM        LLMConfig        `yaml:"llm" json:"llm"`
	Agent      AgentConfig      `yaml:"agent" json:"agent"`
	Security   SecurityConfig   `yaml:"security" json:"security"`
	Filesystem FilesystemConfig `yaml:"filesystem" json:"filesystem"`
	Tailscale  TailscaleConfig  `yaml:"tailscale" json:"tailscale"`
	Storage    StorageConfig    `yaml:"storage" json:"storage"`
	User       UserConfig       `yaml:"user" json:"user"`
}

type UserConfig struct {
	Name        string `yaml:"name" json:"name"`
	Alias       string `yaml:"alias" json:"alias"`
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
}

type ServerConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	APIToken string `yaml:"api_token" json:"api_token"`
}

type LLMConfig struct {
	Provider    string  `yaml:"provider" json:"provider"` // "ollama", "mock", etc.
	BaseURL     string  `yaml:"base_url" json:"base_url"`
	Model       string  `yaml:"model" json:"model"`
	Temperature float64 `yaml:"temperature" json:"temperature"`
	TimeoutSec  int     `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type AgentConfig struct {
	MaxSteps       int `yaml:"max_steps" json:"max_steps"`
	MaxToolCalls   int `yaml:"max_tool_calls" json:"max_tool_calls"`
	TimeoutSeconds int `yaml:"timeout_seconds" json:"timeout_seconds"`
}

type SecurityConfig struct {
	RequireApprovalForSensitive bool     `yaml:"require_approval_for_sensitive_tools" json:"require_approval_for_sensitive_tools"`
	AllowedCommands             []string `yaml:"allowed_commands" json:"allowed_commands"`
	ApprovalRequiredCommands    []string `yaml:"approval_required_commands" json:"approval_required_commands"`
	BlockedCommands             []string `yaml:"blocked_commands" json:"blocked_commands"`
}

type FilesystemConfig struct {
	AllowedPaths     []string `yaml:"allowed_paths" json:"allowed_paths"`
	MaxOutputBytes   int64    `yaml:"max_output_bytes" json:"max_output_bytes"`
	MaxFileSizeRead  int64    `yaml:"max_file_size_read" json:"max_file_size_read"`
}

type TailscaleConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Hostname  string `yaml:"hostname" json:"hostname"`
	AuthKey   string `yaml:"auth_key,omitempty" json:"auth_key,omitempty"`
}

type StorageConfig struct {
	DataDir        string `yaml:"data_dir" json:"data_dir"`
	AuditLogPath   string `yaml:"audit_log_path" json:"audit_log_path"`
	ScreenshotsDir string `yaml:"screenshots_dir" json:"screenshots_dir"`
}

// SystemInfo encapsulates detected Mac hardware, OS, and recommended models.
type SystemInfo struct {
	OS             string   `json:"os"`
	MacOSVersion   string   `json:"macos_version"`
	Architecture   string   `json:"architecture"`
	CPUModel       string   `json:"cpu_model"`
	RAMBytes       uint64   `json:"ram_bytes"`
	RAMFormatted   string   `json:"ram_formatted"`
	AvailableDisk  string   `json:"available_disk"`
	OllamaRunning  bool     `json:"ollama_running"`
	TailscaleIP    string   `json:"tailscale_ip"`
	RecommendedLLM []string `json:"recommended_models"`
}

// DefaultConfig returns safe and robust defaults.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".bob")

	return &Config{
		Server: ServerConfig{
			Host:     "0.0.0.0",
			Port:     8787,
			APIToken: "", // If empty, can be set via BOB_API_TOKEN
		},
		LLM: LLMConfig{
			Provider:    "ollama",
			BaseURL:     "http://127.0.0.1:11434",
			Model:       "qwen2.5-coder:7b",
			Temperature: 0.2,
			TimeoutSec:  120,
		},
		Agent: AgentConfig{
			MaxSteps:       20,
			MaxToolCalls:   20,
			TimeoutSeconds: 300,
		},
		Security: SecurityConfig{
			RequireApprovalForSensitive: true,
			AllowedCommands: []string{
				"pwd", "ls", "git status", "git diff", "git log", "git branch",
				"go test", "go build", "go vet", "go run", "echo", "cat",
				"uname", "whoami", "date", "which", "head", "tail", "wc", "grep", "find",
			},
			ApprovalRequiredCommands: []string{
				"rm", "mv", "chmod", "chown", "sudo", "launchctl", "diskutil",
				"brew install", "npm install", "pip install", "kill", "pkill", "reboot",
			},
			BlockedCommands: []string{
				"rm -rf /", "rm -rf /*", "mkfs", "dd if=/dev/zero",
				":(){ :|:& };:", "shutdown", "halt",
			},
		},
		Filesystem: FilesystemConfig{
			AllowedPaths: []string{
				filepath.Join(homeDir, "Projects"),
				filepath.Join(homeDir, "Documents"),
				filepath.Join(homeDir, "BobWorkspace"),
				filepath.Join(homeDir, "Documents/Bob-AI"),
			},
			MaxOutputBytes:  1024 * 1024,      // 1MB
			MaxFileSizeRead: 5 * 1024 * 1024,  // 5MB
		},
		Tailscale: TailscaleConfig{
			Enabled:  true,
			Hostname: "",
		},
		Storage: StorageConfig{
			DataDir:        dataDir,
			AuditLogPath:   filepath.Join(dataDir, "audit.jsonl"),
			ScreenshotsDir: filepath.Join(dataDir, "screenshots"),
		},
		User: UserConfig{
			Name:        "codemon",
			Alias:       "Alfred",
			Title:       "Creator",
			Description: "Bob's creator and master",
		},
	}
}

// LoadConfig loads configuration from path if exists, and overrides with env vars.
func LoadConfig(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Environment variable overrides
	if host := os.Getenv("BOB_SERVER_HOST"); host != "" {
		cfg.Server.Host = host
	}
	if portStr := os.Getenv("BOB_SERVER_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Server.Port = p
		}
	}
	if token := os.Getenv("BOB_API_TOKEN"); token != "" {
		cfg.Server.APIToken = token
	}
	if provider := os.Getenv("BOB_LLM_PROVIDER"); provider != "" {
		cfg.LLM.Provider = provider
	}
	if baseURL := os.Getenv("BOB_LLM_BASE_URL"); baseURL != "" {
		cfg.LLM.BaseURL = baseURL
	}
	if model := os.Getenv("BOB_LLM_MODEL"); model != "" {
		cfg.LLM.Model = model
	}
	if dataDir := os.Getenv("BOB_DATA_DIR"); dataDir != "" {
		cfg.Storage.DataDir = dataDir
		cfg.Storage.AuditLogPath = filepath.Join(dataDir, "audit.jsonl")
		cfg.Storage.ScreenshotsDir = filepath.Join(dataDir, "screenshots")
	}
	if userName := os.Getenv("BOB_USER_NAME"); userName != "" {
		cfg.User.Name = userName
	}
	if userAlias := os.Getenv("BOB_USER_ALIAS"); userAlias != "" {
		cfg.User.Alias = userAlias
	}
	if userTitle := os.Getenv("BOB_USER_TITLE"); userTitle != "" {
		cfg.User.Title = userTitle
	}

	// Expand ~ in all paths
	cfg.Storage.DataDir = ExpandPath(cfg.Storage.DataDir)
	cfg.Storage.AuditLogPath = ExpandPath(cfg.Storage.AuditLogPath)
	cfg.Storage.ScreenshotsDir = ExpandPath(cfg.Storage.ScreenshotsDir)
	for i, p := range cfg.Filesystem.AllowedPaths {
		cfg.Filesystem.AllowedPaths[i] = ExpandPath(p)
	}

	// Ensure directories exist with fallback
	if err := os.MkdirAll(cfg.Storage.DataDir, 0755); err != nil {
		// Fallback to local workspace data directory
		cwd, _ := os.Getwd()
		cfg.Storage.DataDir = filepath.Join(cwd, ".bob_data")
		cfg.Storage.AuditLogPath = filepath.Join(cfg.Storage.DataDir, "audit.jsonl")
		cfg.Storage.ScreenshotsDir = filepath.Join(cfg.Storage.DataDir, "screenshots")
		_ = os.MkdirAll(cfg.Storage.DataDir, 0755)
		_ = os.MkdirAll(cfg.Storage.ScreenshotsDir, 0755)
	} else {
		_ = os.MkdirAll(cfg.Storage.ScreenshotsDir, 0755)
	}

	return cfg, nil
}

// ExpandPath expands ~ to user home directory.
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				return home
			}
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// DetectSystemInfo probes the Mac host and recommends optimal models.
func DetectSystemInfo() *SystemInfo {
	info := &SystemInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	// macOS Version
	if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		info.MacOSVersion = strings.TrimSpace(string(out))
	}

	// CPU Model / Apple Silicon
	if out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil && len(out) > 0 {
		info.CPUModel = strings.TrimSpace(string(out))
	} else if out, err := exec.Command("sysctl", "-n", "hw.model").Output(); err == nil {
		info.CPUModel = strings.TrimSpace(string(out))
	}

	// RAM Bytes
	if out, err := exec.Command("sysctl", "-n", "hw.memsize").Output(); err == nil {
		if bytes, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64); err == nil {
			info.RAMBytes = bytes
			gb := float64(bytes) / (1024 * 1024 * 1024)
			info.RAMFormatted = fmt.Sprintf("%.1f GB", gb)
		}
	}

	// Disk Free
	if out, err := exec.Command("df", "-h", "/").Output(); err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(lines) > 1 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 4 {
				info.AvailableDisk = fields[3]
			}
		}
	}

	// Check Ollama running
	cmd := exec.Command("curl", "-s", "--max-time", "2", "http://127.0.0.1:11434/api/tags")
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		info.OllamaRunning = true
	}

	// Check Tailscale IP
	if out, err := exec.Command("tailscale", "ip", "-4").Output(); err == nil {
		info.TailscaleIP = strings.TrimSpace(string(out))
	}

	// Recommend models based on RAM
	gb := float64(info.RAMBytes) / (1024 * 1024 * 1024)
	if gb >= 32 {
		info.RecommendedLLM = []string{"qwen2.5-coder:14b", "qwen2.5-coder:32b", "llama3.1:70b-q4_k_m"}
	} else if gb >= 16 {
		info.RecommendedLLM = []string{"qwen2.5-coder:7b", "qwen2.5-coder:14b", "llama3.1:8b"}
	} else if gb >= 8 {
		info.RecommendedLLM = []string{"qwen2.5-coder:7b", "llama3.2:3b", "qwen2.5:3b"}
	} else {
		info.RecommendedLLM = []string{"qwen2.5-coder:1.5b", "llama3.2:1b"}
	}

	return info
}
